// Package urlnorm applies the parts of the WHATWG URL parser's input handling a
// guard must apply before judging a URL Fiber is about to emit. Nothing
// normalizes an outbound URL; the parser deciding it runs in the client after.
package urlnorm

import (
	"strings"
)

// StripTabCRLF removes every ASCII tab, LF and CR, as the WHATWG URL parser does
// before parsing. Leaving them in lets "\t/evil.com" hide behind a leading slash.
// Returns the input unchanged, without copying, when it holds none of them.
func StripTabCRLF(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\t', '\n', '\r':
			if b == nil {
				b = append(make([]byte, 0, len(s)), s[:i]...)
			}
		default:
			if b != nil {
				b = append(b, c)
			}
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

// RootedPath returns location as what a URL composed from a named route can only
// be: a path on the origin now being served. A parameter is spliced in raw, so
// on "/*" "/evil.com" composed "//evil.com" and "\evil.com" folded to the same.
func RootedPath(location string) string {
	location = StripTabCRLF(location)

	n := 0
	for n < len(location) && (location[n] == '/' || location[n] == '\\') {
		n++
	}
	if n == 1 && location[0] == '/' {
		return location
	}
	// Backslashes count as slashes here because the parser folds them.
	return "/" + location[n:]
}

// AsBrowserReads returns location with the handling a client applies before
// parsing: tab, LF and CR removed, then surrounding C0 controls and spaces
// trimmed (RFC 9110 §5.5). Interior spaces are percent-encoded, not removed.
func AsBrowserReads(location string) string {
	return strings.TrimFunc(StripTabCRLF(location), func(r rune) bool { return r <= ' ' })
}
