// Package idnafold converts a Unicode hostname label to the Punycode form a
// browser puts on the wire (RFC 5891), so a hostname literal written in
// source still matches the requests that name actually generates.
package idnafold

import (
	"strings"

	"github.com/gofiber/utils/v2"
	"golang.org/x/net/idna"
)

// ToASCII converts host to its Punycode ("xn--...") form when it carries
// non-ASCII characters, and returns it unchanged otherwise — including when
// it is already Punycode, which is itself ASCII.
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
