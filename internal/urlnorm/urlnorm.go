// Package urlnorm applies the parts of the WHATWG URL parser's input handling
// that a guard has to apply before it can judge a URL Fiber is about to emit.
//
// Nothing normalizes an outbound URL. A Location header is written as composed
// and a href is rendered as composed, and the parser that decides where either
// one points runs in the client afterwards. It strips ASCII tab, LF and CR from
// anywhere in the input and folds backslashes to slashes before it reads the
// host, so a guard reading the bytes as written is reading a different URL than
// the one the client will fetch.
package urlnorm

import (
	"strings"
)

// StripTabCRLF removes every ASCII tab, LF and CR, which the WHATWG URL parser
// removes from anywhere in its input before parsing.
//
// Removing them changes no location a client could tell apart, since it drops
// them itself. Leaving them in is what lets "\t/evil.com" sit behind a leading
// slash as "/\t/evil.com", whose leading slash run is two by the time anything
// navigates.
//
// The input is returned unchanged, without copying, when it holds none of them
// — which is every ordinary URL.
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
// Every route is registered under "/", so the path a route builds starts there
// too — but only the constant parts of it are the author's. A parameter value
// is whatever the caller passed, and it is spliced in raw, so where the route is
// "/*" that single leading slash is all that separates the value from the start
// of the URL. A value of "/evil.com" then composes "//evil.com", a network-path
// reference the browser resolves to another origin, and "\evil.com" composes
// "/\evil.com", which the parser folds to the same thing. Whoever asked for the
// URL of a route in this application asked for neither.
func RootedPath(location string) string {
	location = StripTabCRLF(location)

	n := 0
	for n < len(location) && (location[n] == '/' || location[n] == '\\') {
		n++
	}
	if n == 1 && location[0] == '/' {
		return location
	}
	// Collapse the run to the single slash the route is rooted at. Backslashes
	// count as slashes here because the parser folds them in this position.
	return "/" + location[n:]
}

// AsBrowserReads returns location with the input handling a client applies
// before it parses: tab, LF and CR removed, then leading and trailing C0
// controls and spaces trimmed.
//
// A recipient strips leading and trailing whitespace from a header value
// (RFC 9110 Section 5.5) as well. Skipping either leaves the normalization
// mismatch a guard exists to close: with UnescapePath enabled, "/r/%20//evil.com"
// under the rule "$1" composes " //evil.com", whose leading slash run the guard
// never sees, and "/api/%09/evil.com" under "/$1" composes "/\t/evil.com", which
// the parser turns back into "//evil.com". Both reach evil.com.
//
// Interior spaces are left alone — the parser percent-encodes them rather than
// removing them, so one cannot form an authority.
func AsBrowserReads(location string) string {
	location = StripTabCRLF(location)
	return strings.TrimFunc(location, func(r rune) bool { return r <= ' ' })
}
