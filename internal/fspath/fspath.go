// Package fspath holds the checks that stand between a request and the
// fasthttp file handler, for the static middleware and SendFile.
//
// Each check exists because of something fasthttp does, or leaves to us:
//
//	check                        static    SendFile  why
//	control byte                 404       404       (1)
//	backslash, raw or "%5C"      404       -         (2)
//	escape the router kept       opened    -         (3)
//	stray "%", forged escape     404       literal   (4)
//	".", ".." or empty segment   404       cleaned   (5)
//	segment ending "." or " "    404       -         (6)
//	drive letter or volume       404       -         (7) OS root only
//	fs.ValidPath                 404       -         (8) fs.FS root only
//	ASCII case alias, "~" digit  404       -         (9) Aliased
//
// The reasons, by number:
//
//  1. The file handler refuses only NUL, and with a 400; other C0 bytes and
//     DEL reach the OS. None of them is part of a name a client can ask for.
//  2. fasthttp treats "\" as a separator only on Windows, so on Unix "..\"
//     passes its ".." check and the same path names different files per OS.
//  3. A PathRewrite path is opened as is, still encoded, so the file opened
//     is the one the router and path-scoped middleware matched.
//  4. See the stray "%" policy below.
//  5. A PathRewrite path skips Path() normalization, and the handler's own
//     ".." check answers 500 and lets "." and "//" through. SendFile hands
//     fasthttp a request URI instead, whose Path() resolves dot segments and
//     merges "//"; FSName and OSName give the name that comes out.
//  6. Windows strips a trailing dot or space from a path component when it
//     opens a file, so "private./x" would open "private/x"; so does a FAT
//     volume on any OS.
//  7. fasthttp refuses ":" only on Windows and only for its OS file system,
//     with a 403; the check here runs on every OS.
//  8. An fs.FS name must also be valid UTF-8.
//  9. The router folds ASCII case only. A case-folding file system opens
//     "secret" for "ſecret" (U+017F), "session" for "seßion", "fish" for
//     "ﬁsh", drops a soft hyphen or zero-width space, and an 8.3 short name
//     such as "PRIVAT~1" opens a long one. Another normalization or case of a
//     non-ASCII name still opens the same file.
//
// Stray "%" policy: a "%" is a name byte wherever nothing decodes the name
// after it, that is under UnescapePath, where the router already decoded every
// escape, and in SendFile, which escapes the name so fasthttp's one decode
// gives it back. In the static middleware's default mode the routed path is
// still a percent-encoded spelling, and a "%" there must begin an escape the
// router kept. One that begins none, or an escape of an unreserved character
// the router would have decoded, is no spelling the router produced, so the
// name is refused rather than guessed. The router itself keeps a stray "%"
// (as "%25" where it would forge an escape), the extractors' url.PathUnescape
// rejects it and the client's cookie jar encodes every "%"; refusing is the
// strictest of those readings.
package fspath

import (
	"bytes"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/utils/v2"
)

// Valid reports whether the routed path p, with the route's prefix stripped,
// can name a file inside the root, which is fsys when set and the OS file
// system otherwise. The file server then opens p unchanged. It is false when
// p holds an escape of a slash ("private%2Fsecret.txt" never reaches the
// "private/secret.txt" the router did not match), an escape of an unreserved
// character (a stray '%' the router kept that would otherwise double-decode
// past the route and middleware), a malformed escape, a backslash, a control
// character, a ".", ".." or empty segment, a drive letter and a segment ending
// in a dot or a space that the file system may strip. escaped is off under
// UnescapePath, which already decoded everything.
func Valid(p []byte, fsys fs.FS, escaped bool) bool { //nolint:revive // the flag mirrors UnescapePath
	if !validName(p, escaped) {
		return false
	}
	if hasUnsafeSegment(p) {
		return false
	}

	name := strings.TrimPrefix(utils.UnsafeString(p), "/")
	if fsys != nil {
		name = strings.TrimSuffix(name, "/")
		return name == "" || fs.ValidPath(name)
	}
	return filepath.VolumeName(name) == "" && !hasDriveLetter(name)
}

// FSName returns the name the fasthttp file handler opens in an fs.FS for the
// slash-separated name SendFile hands it: rooted and cleaned as Path()
// normalization cleans a request path, then without the leading slash.
func FSName(name string) string {
	return path.Clean("/" + name)[1:]
}

// OSName is FSName for the OS file system, where the name keeps its root and
// takes the OS separator.
func OSName(name string) string {
	return filepath.Clean(filepath.FromSlash(name))
}

// validName validates the escapes the router left in p. The router already
// produced the canonical resource name (it decoded every escape of an
// unreserved character and kept the rest encoded), so file lookup opens
// exactly the bytes the route and any path-scoped middleware matched. It
// refuses what cannot name such a file: a backslash or control character, a
// malformed or truncated escape, an escape of a separator or control
// character, and an escape of an unreserved character. That last one cannot
// survive the router's own decoding, so its only source is a stray '%' the
// router kept literally that lines up with the following bytes into a fresh
// escape ("%%370rivate" and "%7%30rivate" both normalize to "%70rivate");
// decoding it a second time would reach a name ("private") the router never
// matched and the middleware never guarded, so it is rejected. When escaped is
// off, UnescapePath already decoded everything and no escape remains to
// validate.
func validName(p []byte, escaped bool) bool { //nolint:revive // the flag mirrors UnescapePath; see Valid
	if utils.IndexControl(p) >= 0 || bytes.IndexByte(p, '\\') >= 0 {
		return false
	}
	i := bytes.IndexByte(p, '%')
	if !escaped || i < 0 {
		return true
	}
	for ; i < len(p); i++ {
		if p[i] != '%' {
			continue
		}
		if i+2 >= len(p) {
			return false
		}
		hi, lo := unhex(p[i+1]), unhex(p[i+2])
		if hi < 0 || lo < 0 {
			return false
		}
		c := byte(hi<<4 | lo) //nolint:gosec // G115: both nibbles are 0-15
		if c == '/' || c == '\\' || c < 0x20 || c == 0x7f || isUnreserved(c) {
			return false
		}
		i += 2
	}
	return true
}

func isUnreserved(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

// unhex returns the value of a hexadecimal digit, or -1 for any other byte.
func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// hasUnsafeSegment reports whether p holds an empty segment other than the
// root's leading and a directory's trailing slash, or a segment ending in a dot
// or a space, "." and ".." included. The router removes dot segments and keeps
// empty ones as sent, so none of them is part of a path it matched to a file.
// Windows and FAT volumes drop a trailing dot or space when they open a file,
// so "private./x" would open "private/x", which middleware guarding "private"
// never saw; the volume, or what backs an fs.FS, is not known here, so such a
// name is refused for every root.
func hasUnsafeSegment(p []byte) bool {
	start := 0
	for i, c := range p {
		if c != '/' {
			continue
		}
		if i == start {
			if start > 0 {
				return true
			}
		} else if last := p[i-1]; last == '.' || last == ' ' {
			return true
		}
		start = i + 1
	}
	// the last segment, empty after a directory's trailing slash
	return start < len(p) && (p[len(p)-1] == '.' || p[len(p)-1] == ' ')
}

// hasDriveLetter reports whether name starts with a Windows drive letter and a colon.
func hasDriveLetter(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	drive := name[0]
	return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
}

// Aliased reports whether p, a path Valid accepted, holds a spelling a file
// system may open under another name than the router matched: a rune of
// aliasRunes, or a "~" and a digit, the tail of an 8.3 short name such as "PRIVAT~1".
func Aliased(p []byte) bool {
	if i := bytes.IndexByte(p, '~'); i >= 0 && hasShortNameTail(p[i:]) {
		return true
	}
	if utils.IsASCII(p) {
		return false
	}
	for _, r := range utils.UnsafeString(p) {
		if r >= utf8.RuneSelf && unicode.Is(aliasRunes, r) {
			return true
		}
	}
	return false
}

// aliasRunes are the non-ASCII runes a case-folding or normalizing file system can open as ASCII or drop:
// those whose case, or full case fold and canonical decomposition, is ASCII, and the default ignorables.
// Derived from golang.org/x/text v0.44.0 cases.Fold and norm.NFD over every code point.
var aliasRunes = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x00ad, 0x00ad, 1},
		{0x00df, 0x00df, 1},
		{0x0130, 0x0131, 1},
		{0x017f, 0x017f, 1},
		{0x034f, 0x034f, 1},
		{0x037e, 0x037e, 1},
		{0x0600, 0x0605, 1},
		{0x061c, 0x061c, 1},
		{0x06dd, 0x06dd, 1},
		{0x070f, 0x070f, 1},
		{0x0890, 0x0891, 1},
		{0x08e2, 0x08e2, 1},
		{0x115f, 0x1160, 1},
		{0x17b4, 0x17b5, 1},
		{0x180b, 0x180f, 1},
		{0x1e9e, 0x1e9e, 1},
		{0x1fef, 0x1fef, 1},
		{0x200b, 0x200f, 1},
		{0x202a, 0x202e, 1},
		{0x2060, 0x206f, 1},
		{0x212a, 0x212a, 1},
		{0x3164, 0x3164, 1},
		{0xfb00, 0xfb06, 1},
		{0xfe00, 0xfe0f, 1},
		{0xfeff, 0xfeff, 1},
		{0xffa0, 0xffa0, 1},
		{0xfff0, 0xfffb, 1},
	},
	R32: []unicode.Range32{
		{0x110bd, 0x110bd, 1},
		{0x110cd, 0x110cd, 1},
		{0x13430, 0x1343f, 1},
		{0x1bca0, 0x1bca3, 1},
		{0x1d173, 0x1d17a, 1},
		{0xe0000, 0xe0fff, 1},
	},
	LatinOffset: 2,
}

// hasShortNameTail reports whether a "~" in p is followed by a digit.
func hasShortNameTail(p []byte) bool {
	for i := 0; i+1 < len(p); i++ {
		if p[i] == '~' && p[i+1] >= '0' && p[i+1] <= '9' {
			return true
		}
	}
	return false
}
