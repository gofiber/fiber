// Package urlnorm applies the parts of the WHATWG URL parser's input handling a
// guard must apply before judging a URL Fiber is about to emit. Nothing
// normalizes an outbound URL; the parser deciding it runs in the client after.
//
// The callers need different policies:
//   - StripTabCRLF removes tab, CR and LF only.
//   - AsBrowserReads also trims surrounding C0 controls and spaces.
//   - Referer also folds backslashes before a query or fragment, retaining up
//     to two leading slashes so the caller can check the original authority.
//   - RootedPath forces exactly one leading slash for a generated local path;
//     it does not trim surrounding spaces or fold backslashes inside the path.
//
// These transformations do not authorize a redirect or check its origin.
package urlnorm

import (
	"strings"

	"github.com/gofiber/utils/v2"
)

type backslashPolicy uint8

const (
	keepBackslashes backslashPolicy = iota
	foldPathBackslashes
)

// StripTabCRLF removes every ASCII tab, LF and CR, as the WHATWG URL parser does
// before parsing. Leaving them in lets "\t/evil.com" hide behind a leading slash.
// Returns the input unchanged, without copying, when it holds none of them.
func StripTabCRLF(s string) string {
	return rewrite(s, keepBackslashes)
}

// rewrite removes tab/newline and optionally folds path backslashes, copying
// only after finding a byte that changes. Query and fragment backslashes are
// data, but tab/newline removal applies to the entire input.
func rewrite(s string, policy backslashPolicy) string {
	foldPath := policy == foldPathBackslashes
	start := utils.IndexAny3(s, '\t', '\r', '\n')
	if foldPath {
		pathEnd := utils.IndexAny2(s, '?', '#')
		path := s
		if pathEnd >= 0 {
			path = s[:pathEnd]
		}
		if slash := strings.IndexByte(path, '\\'); slash >= 0 && (start < 0 || slash < start) {
			start = slash
		}
		// The unchanged prefix may already contain the query/fragment marker.
		foldPath = pathEnd < 0 || start < pathEnd
	}
	if start < 0 {
		return s
	}

	b := make([]byte, start, len(s))
	copy(b, s[:start])
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\t' || c == '\n' || c == '\r':
			continue
		case foldPath && (c == '?' || c == '#'):
			foldPath = false
		case foldPath && c == '\\':
			c = '/'
		}
		b = append(b, c)
	}
	return string(b)
}

// RootedPath returns location as what a URL composed from a named route can only
// be: a path on the origin now being served. Parameter values are escaped before
// this check, except slashes inside greedy parameters; normalization still
// protects route author text and direct callers.
//
// Exactly one leading slash, never two: "//evil.com" is a network-path reference
// whose host is whatever follows, so it resolves against "https://good.example"
// as "https://evil.com". That is true of author text as much as of a value,
// which is why NamesAuthority refuses such a route rather than composing it.
func RootedPath(location string) string {
	location = StripTabCRLF(location)

	n := LeadingSlashes(location)
	if n == 1 && location[0] == '/' {
		return location
	}
	// Backslashes count as slashes here because the parser folds them.
	return "/" + location[n:]
}

// LeadingSlashes returns the length of the run of slashes starting s, counting
// backslashes among them because the parser folds the two.
func LeadingSlashes(s string) int {
	n := 0
	for n < len(s) && (s[n] == '/' || s[n] == '\\') {
		n++
	}
	return n
}

// AsBrowserReads applies the WHATWG parser's input cleanup: tab, LF and CR
// removed, then surrounding C0 controls and spaces trimmed. Interior spaces
// and percent escapes remain unchanged here.
func AsBrowserReads(location string) string {
	return trimC0Space(StripTabCRLF(location))
}

// Referer normalizes an input for a subsequent same-origin check. It trims
// surrounding C0 controls/spaces, removes tab/newline, and folds backslashes
// only before the first query or fragment delimiter. It preserves raw bytes
// and percent escapes rather than decoding or re-encoding them.
//
// A leading slash run is reduced to two to retain an authority; an input made
// entirely of slashes becomes "/". Unlike RootedPath, it must not turn a
// cross-origin reference into a local path before the caller checks it.
func Referer(location string) string {
	location = rewrite(trimC0Space(location), foldPathBackslashes)
	if strings.HasPrefix(location, "//") {
		n := LeadingSlashes(location)
		if n == len(location) {
			return "/"
		}
		return location[n-2:]
	}
	return location
}

func trimC0Space(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
}
