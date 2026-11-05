// Package ipv6host writes IPv6 hosts in the one form browsers serialize them
// in, so that every spelling of an address (RFC 4291 Section 2.2) compares equal.
package ipv6host

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
)

// Parse parses an IPv6 address written without brackets. A zone is refused:
// netip copies it, and Unmap would drop it.
func Parse(s string) (netip.Addr, bool) {
	if strings.IndexByte(s, '%') >= 0 {
		return netip.Addr{}, false
	}
	return utils.ParseIPv6(s)
}

// String writes addr as the WHATWG URL serializer does: RFC 5952 Section 4, and
// an IPv4-mapped tail in hex ("::ffff:7f00:1"). raw is returned when it already
// is that form, so the common case does not allocate.
func String(addr netip.Addr, raw string) string {
	var buf [len("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")]byte
	b := buf[:0]
	if addr.Is4In6() {
		a := addr.As16()
		b = append(b, "::ffff:"...)
		b = strconv.AppendUint(b, uint64(a[12])<<8|uint64(a[13]), 16)
		b = append(b, ':')
		b = strconv.AppendUint(b, uint64(a[14])<<8|uint64(a[15]), 16)
	} else {
		b = addr.AppendTo(b)
	}
	if string(b) == raw {
		return raw
	}
	return string(b)
}

// Canonical rewrites the bracketed IPv6 address that host starts with, as in
// "[0:0:0:0:0:0:0:1]:3000", and returns anything else unchanged.
func Canonical(host string) string {
	if host == "" || host[0] != '[' {
		return host
	}
	end := strings.IndexByte(host, ']')
	if end < 0 {
		return host
	}
	addr, ok := Parse(host[1:end])
	if !ok {
		return host
	}
	if canonical := String(addr, host[1:end]); canonical != host[1:end] {
		return "[" + canonical + host[end:]
	}
	return host
}
