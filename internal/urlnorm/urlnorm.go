// Package urlnorm applies the parts of the WHATWG URL parser's input handling
// that a guard must apply before it can judge a URL Fiber is about to emit.
//
// Nothing normalizes an outbound URL: a Location is written as composed, and
// the parser deciding where it points runs in the client afterwards. It strips
// tab, LF and CR from anywhere in the input and folds backslashes to slashes
// before reading the host.
package urlnorm

import (
	"strings"
)

// StripTabCRLF removes every ASCII tab, LF and CR, as the WHATWG URL parser
// does before parsing. Removing them changes no location a client could tell
// apart; leaving them in lets "\t/evil.com" hide behind a leading slash as
// "/\t/evil.com", whose slash run is two by the time anything navigates.
//
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

// RootedPath returns location as what a URL composed from a named route can
// only be: a path on the origin now being served.
//
// Only the constant parts of a route are the author's; a parameter is spliced
// in raw, so on "/*" one slash separates it from the start of the URL. A value
// of "/evil.com" composes "//evil.com", and "\evil.com" composes "/\evil.com",
// which the parser folds to the same thing.
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
// parsing: tab, LF and CR removed, then leading and trailing C0 controls and
// spaces trimmed, as a recipient does to a field value (RFC 9110 Section 5.5).
//
// Interior spaces are left alone — the parser percent-encodes those rather than
// removing them, so one cannot form an authority.
func AsBrowserReads(location string) string {
	return strings.TrimFunc(StripTabCRLF(location), func(r rune) bool { return r <= ' ' })
}
