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
//	segment ending "." or " "    404       -         (6) Windows only
//	drive letter or volume       404       -         (7) OS root only
//	fs.ValidPath                 404       -         (8) fs.FS root only
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
//     merges "//".
//  6. Windows strips a trailing dot or space from a path component when it
//     opens a file, so "private./x" would open "private/x".
//  7. fasthttp refuses ":" only on Windows and only for its OS file system,
//     with a 403; the check here runs on every OS.
//  8. An fs.FS name must also be valid UTF-8.
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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofiber/utils/v2"
)

// windowsOS is runtime.GOOS on Windows, where a path component loses its
// trailing dots and spaces when a file is opened.
const windowsOS = "windows"

// Valid reports whether the routed path p, with the route's prefix stripped,
// can name a file inside the root, which is fsys when set and the OS file
// system otherwise. The file server then opens p unchanged. It is false when
// p holds an escape of a slash ("private%2Fsecret.txt" never reaches the
// "private/secret.txt" the router did not match), an escape of an unreserved
// character (a stray '%' the router kept that would otherwise double-decode
// past the route and middleware), a malformed escape, a backslash, a control
// character, a ".", ".." or empty segment, a drive letter and, on Windows, a
// segment ending in a dot or a space that the OS would strip. escaped is off
// under UnescapePath, which already decoded everything.
func Valid(p []byte, fsys fs.FS, escaped bool) bool { //nolint:revive // the flag mirrors UnescapePath
	if !validName(p, escaped) {
		return false
	}
	if hasUnsafeSegment(p) || (runtime.GOOS == windowsOS && hasTrailingDotOrSpaceSegment(p)) {
		return false
	}

	name := strings.TrimPrefix(utils.UnsafeString(p), "/")
	if fsys != nil {
		name = strings.TrimSuffix(name, "/")
		return name == "" || fs.ValidPath(name)
	}
	return filepath.VolumeName(name) == "" && !hasDriveLetter(name)
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

// hasUnsafeSegment reports whether p holds a "." or ".." segment, or an empty
// one other than the root's leading and a directory's trailing slash. The
// router removes dot segments and keeps empty ones as sent, so none of them
// is part of a path it matched to a file.
func hasUnsafeSegment(p []byte) bool {
	start := 0
	for i := 0; i <= len(p); i++ {
		if i < len(p) && p[i] != '/' {
			continue
		}
		switch n := i - start; {
		case n == 0:
			if start > 0 && i < len(p) {
				return true
			}
		case n == 1 && p[start] == '.', n == 2 && p[start] == '.' && p[start+1] == '.':
			return true
		}
		start = i + 1
	}
	return false
}

// hasDriveLetter reports whether name starts with a Windows drive letter and a colon.
func hasDriveLetter(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	drive := name[0]
	return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
}

// hasTrailingDotOrSpaceSegment reports whether a segment of p ends in a dot or
// a space. Windows drops both from a path component when it opens a file, so
// "private./secret.txt" and "private /secret.txt" would open
// "private/secret.txt": a path the router kept apart from "private", and one
// that middleware guarding "private" never saw. Elsewhere such a name is an
// ordinary file, so Valid applies this only on Windows, and there to every
// root: an fs.FS may be OS-backed (os.DirFS) and cannot be told apart from a
// virtual one such as embed.FS, so a virtual name ending in a dot or a space
// is refused on Windows rather than leave the alias open for the rest.
func hasTrailingDotOrSpaceSegment(p []byte) bool {
	for i, c := range p {
		if (c == '.' || c == ' ') && (i+1 == len(p) || p[i+1] == '/') {
			return true
		}
	}
	return false
}
