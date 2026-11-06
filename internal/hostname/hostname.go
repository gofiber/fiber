// Package hostname splits host:port values and converts IDN host names.
package hostname

import (
	"strings"

	"github.com/gofiber/utils/v2"
	"golang.org/x/net/idna"
)

// SplitPort splits "host[:port]" into host and port without trimming. A
// bracketed IPv6 literal keeps its brackets and ends at the first ']', and
// anything after it but a ":port" is dropped. A value with several colons is
// returned whole as a bare IPv6 literal. An unclosed '[' is not a literal, so
// "[a:80" splits like any other host into "[a" and "80".
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

	// No colon leaves nothing to split, and a second one makes this an
	// unbracketed IPv6 literal, where a port is impossible.
	i := strings.IndexByte(authority, ':')
	if i < 0 || strings.IndexByte(authority[i+1:], ':') >= 0 {
		return authority, ""
	}
	return authority[:i], authority[i+1:]
}

// ToASCII converts a non-ASCII host to Punycode. IDNA has no concept of a port,
// so a caller holding "host:port" splits it first: a host with a colon is
// returned unchanged. So are an ASCII host and one idna.Lookup rejects, so an
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

// ToUnicode decodes the Punycode labels of host. "xn--" anywhere in host sends
// all of it through idna.Lookup, which also maps the other labels (width, dots,
// soft hyphens), and a single label it rejects leaves the whole host unchanged.
// The xn-- check is case-sensitive, so callers fold ASCII case first.
func ToUnicode(host string) string {
	if !strings.Contains(host, "xn--") {
		return host
	}
	if u, err := idna.Lookup.ToUnicode(host); err == nil {
		return u
	}
	return host
}
