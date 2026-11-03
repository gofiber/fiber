// Package hostname splits and folds the host part of an authority
// ("host[:port]"). It is shared by the core request helpers (Hostname,
// Subdomains), the listener banner, the proxy DomainForward handler and the
// hostauthorization middleware, so each of those reads a host the same way
// instead of carrying its own copy.
//
// The helpers are deliberately small and composable: a caller picks the
// steps its contract needs (split off the port, fold to Punycode or decode
// it) rather than one entry point guessing for all of them. None of
// them validates; a caller that must reject malformed input does so before
// or after, as hostauthorization does.
package hostname

import (
	"strings"

	"github.com/gofiber/utils/v2"
	"golang.org/x/net/idna"
)

// SplitPort splits authority into its host and port. Surrounding whitespace
// is trimmed first. A bracketed IPv6 literal keeps its brackets ("[::1]:80"
// gives "[::1]" and "80"); an unbracketed value with more than one colon is
// a bare IPv6 literal, which cannot carry a port, so it is returned whole.
// A value without a colon has no port.
//
// SplitPort is lenient by design: it never fails, and input that is not a
// valid authority still splits somewhere. It reads up to the first ']', so
// "[::1]]:80" gives "[::1]" and no port; fasthttp's parseHost accepts that
// spelling, so a caller that must tell it apart from "[::1]" cannot rely on
// either of them to reject it.
func SplitPort(authority string) (host, port string) { //nolint:nonamedreturns // gocritic unnamedResult requires naming host and port parts for clarity
	if authority == "" {
		return "", ""
	}

	authority = utils.TrimSpace(authority)

	// An IPv6 literal enclosed in brackets, as defined by RFC 3986.
	if strings.HasPrefix(authority, "[") {
		if end := strings.IndexByte(authority, ']'); end != -1 {
			host = authority[:end+1] // keep the closing ]
			if len(authority) > end+1 && authority[end+1] == ':' {
				return host, authority[end+2:]
			}
			return host, ""
		}
	}

	// Everything else with a colon.
	if i := strings.LastIndexByte(authority, ':'); i != -1 {
		host, port = authority[:i], authority[i+1:]

		// If host still contains ':', this is an unbracketed IPv6 literal. In
		// that form a port is impossible, so the whole value is the host.
		if strings.IndexByte(host, ':') >= 0 {
			return authority, ""
		}
		return host, port
	}

	// No colon, nothing to split.
	return authority, ""
}

// ToASCII converts host to its Punycode ("xn--...") form when it carries
// non-ASCII characters, and returns it unchanged otherwise — including when
// it is already Punycode, which is itself ASCII. This is the form a browser
// puts on the wire (RFC 5891), so a hostname literal written in source still
// matches the requests that name actually generates.
//
// A value containing a colon is returned unchanged: IDNA has no concept of a
// port, and a bracketed IPv6 literal is never subject to IDNA. A caller with
// a "host:port" or "[ipv6]:port" value splits it first and folds the host
// part alone.
//
// Input idna.Lookup's stricter validation rejects is also returned
// unchanged rather than as an error: it will not match any Punycode-encoded
// value, which is the correct security default — reject by non-match, not by
// a second error path every caller would have to remember to handle.
func ToASCII(host string) string {
	if host == "" || strings.IndexByte(host, ':') >= 0 || utils.IsASCII(host) {
		return host
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		return ascii
	}
	return host
}

// ToUnicode decodes the Punycode ("xn--...") labels of host back to Unicode
// with the same idna.Lookup profile ToASCII encodes with. A host without an
// "xn--" label is returned unchanged without consulting IDNA, as is a host
// idna.Lookup rejects, for the same reason ToASCII does not surface errors.
//
// The caller folds ASCII case first: decoding keeps a label's basic code
// points as they are, so a lowercase input yields a lowercase output.
func ToUnicode(host string) string {
	if !strings.Contains(host, "xn--") {
		return host
	}
	if u, err := idna.Lookup.ToUnicode(host); err == nil {
		return u
	}
	return host
}
