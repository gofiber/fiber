// Package hostname holds the host helpers shared by Hostname, Subdomains, the
// listener banner, DomainForward and hostauthorization.
package hostname

import (
	"strings"

	"github.com/gofiber/utils/v2"
	"golang.org/x/net/idna"
)

// SplitPort splits "host[:port]" into host and port without trimming. A
// bracketed IPv6 literal keeps its brackets and ends at the first ']'; a bare
// IPv6 literal is returned whole.
func SplitPort(authority string) (host, port string) { //nolint:nonamedreturns // gocritic unnamedResult requires naming host and port parts for clarity
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

// ToASCII converts a non-ASCII host to Punycode. A host with a colon, an
// ASCII host and one idna.Lookup rejects are returned unchanged, so an
// invalid host fails by not matching.
func ToASCII(host string) string {
	if host == "" || strings.IndexByte(host, ':') >= 0 || utils.IsASCII(host) {
		return host
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		return ascii
	}
	return host
}

// ToUnicode decodes the Punycode labels of host; one idna.Lookup rejects is
// returned unchanged. The xn-- check is case-sensitive, so callers fold ASCII
// case first.
func ToUnicode(host string) string {
	if !strings.Contains(host, "xn--") {
		return host
	}
	if u, err := idna.Lookup.ToUnicode(host); err == nil {
		return u
	}
	return host
}
