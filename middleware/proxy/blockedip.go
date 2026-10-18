package proxy

import (
	"net"
)

// isBlockedIP reports whether ip falls inside a range that proxy
// upstreams must not reach by default. An IPv4 address, or an IPv4-mapped one,
// is judged by isBlockedIPv4. For IPv6, loopback, unspecified, unique-local,
// link-local, multicast and the deprecated site-local fec0::/10 (RFC 3879)
// ranges are blocked.
//
// IPv6 transition addresses are handled in two ways so a blocked IPv4 cannot
// be smuggled past as an IPv6 literal. Deprecated tunneling and local-use
// ranges (6to4, Teredo, NAT64 local-use) are blocked in their entirety by
// isBlockedIPv6TransitionRange — their embedded IPv4 sits at an
// operator-dependent offset, and none has a legitimate use as a proxy
// upstream. The remaining wrappers whose embedded IPv4 is unambiguous and may
// legitimately be public (NAT64 well-known prefix, IPv4-compatible, ISATAP)
// are unwrapped by embeddedIPv4 and re-checked against the same blocklist.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return isBlockedIPv4(v4)
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsMulticast() || isIPv6SiteLocal(ip) {
		return true
	}
	// Block deprecated / local-use IPv6 transition ranges wholesale (see the
	// helper for the rationale). Done before the embeddedIPv4 unwrap because
	// their embedded IPv4 offset is not fixed.
	if isBlockedIPv6TransitionRange(ip) {
		return true
	}
	// Look through the unambiguous wrappers to the embedded IPv4 and apply the
	// same blocklist. Some stacks route these to the embedded address, so a
	// private target could otherwise be reached via such a literal (the
	// IPv4-mapped ::ffff:a.b.c.d form is already handled above because
	// net.IP.To4 exposes it).
	embedded := embeddedIPv4(ip)
	return embedded != nil && isBlockedIPv4(embedded)
}

// isBlockedIPv4 reports whether the four bytes of v4 are an address proxy upstreams must not
// reach by default:
//
//   - 0.0.0.0/8                                   "this network" (RFC 1122)
//   - 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16   private (RFC 1918)
//   - 100.64.0.0/10                               shared address space, CGNAT (RFC 6598)
//   - 127.0.0.0/8                                 loopback
//   - 169.254.0.0/16                              link-local, incl. the cloud-metadata 169.254.169.254
//   - 192.0.0.0/24                                IETF protocol assignments (RFC 6890), incl. Oracle Cloud's 192.0.0.192
//   - 198.18.0.0/15                               benchmarking (RFC 2544)
//   - 224.0.0.0/3                                 multicast and reserved (RFC 1112), incl. 255.255.255.255
//
// The documentation ranges (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24)
// are left alone: they route nowhere, and tests use them as public addresses.
func isBlockedIPv4(v4 net.IP) bool {
	switch v4[0] {
	case 0, 10, 127:
		return true
	case 100:
		return v4[1]&0xc0 == 64
	case 169:
		return v4[1] == 254
	case 172:
		return v4[1]&0xf0 == 16
	case 192:
		return v4[1] == 168 || (v4[1] == 0 && v4[2] == 0)
	case 198:
		return v4[1]&0xfe == 18
	default:
		return v4[0] >= 224
	}
}

// isIPv6SiteLocal reports whether ip is in fec0::/10, the site-local range
// RFC 3879 deprecated: some networks still route it to internal hosts.
func isIPv6SiteLocal(ip net.IP) bool {
	ip16 := ip.To16()
	return ip16 != nil && ip16[0] == 0xfe && ip16[1]&0xc0 == 0xc0
}

// isBlockedIPv6TransitionRange reports whether ip belongs to an IPv6
// transition range that must be blocked in its entirety:
//
//   - 6to4         2002::/16        (RFC 3056) — deprecated by RFC 7526.
//   - Teredo       2001:0000::/32   (RFC 4380) — a private IPv4 can hide in
//     either the un-obfuscated server field (bytes [4:8]) or the obfuscated
//     client field (bytes [12:16]).
//   - NAT64 local  64:ff9b:1::/48   (RFC 8215) — local-use only; RFC 8215 §3
//     forbids it on the public Internet, and RFC 6052 lets the embedded IPv4
//     sit anywhere from a /32 to a /96 boundary.
//
// Because the embedded IPv4 offset depends on operator configuration,
// unwrapping a single position would leave a bypass; blocking the whole range
// is both safer and simpler. This mirrors the CIDR-level handling in Gitea's
// and Coder's SSRF guards.
func isBlockedIPv6TransitionRange(ip net.IP) bool {
	ip16 := ip.To16()
	if ip16 == nil {
		return false
	}
	switch {
	case ip16[0] == 0x20 && ip16[1] == 0x02: // 6to4 2002::/16
		return true
	case ip16[0] == 0x20 && ip16[1] == 0x01 && ip16[2] == 0x00 && ip16[3] == 0x00: // Teredo 2001:0000::/32
		return true
	case ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b &&
		ip16[4] == 0x00 && ip16[5] == 0x01: // NAT64 local-use 64:ff9b:1::/48
		return true
	}
	return false
}

// embeddedIPv4 returns the IPv4 address embedded in an IPv6 transition
// address, as the last four bytes of ip itself, or nil for anything else. It
// only handles wrappers whose embedded IPv4 has a fixed, unambiguous offset
// and can legitimately be a public host:
//
//   - NAT64 well-known prefix 64:ff9b::/96 (RFC 6052) — defined only at /96,
//     so the embedded IPv4 is always the trailing 32 bits and is commonly used
//     to reach public IPv4 hosts from IPv6-only networks.
//   - ISATAP (RFC 5214) — interface identifier 00-00-5E-FE or 02-00-5E-FE
//     followed by the embedded IPv4 in the trailing 32 bits. The prefix is an
//     ordinary global one, so there is no dedicated range to block wholesale.
//   - SIIT IPv4-translated ::ffff:0:0/96 (RFC 2765) — 0xffff marker in bytes
//     [8:10], zero in [10:12], embedded IPv4 in the trailing 32 bits.
//   - IPv4-compatible ::a.b.c.d (RFC 4291) — trailing 32 bits.
//
// Deprecated tunneling / local-use ranges with operator-dependent embedding
// offsets (6to4, Teredo, NAT64 local-use) are intentionally NOT unwrapped here;
// isBlockedIPv6TransitionRange blocks them wholesale instead. The IPv4-mapped
// form (::ffff:a.b.c.d, 0xffff marker in bytes [10:12]) is also excluded:
// net.IP.To4 already surfaces its IPv4, so isBlockedIP checks it directly.
func embeddedIPv4(ip net.IP) net.IP {
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	// NAT64 well-known prefix 64:ff9b::/96 (RFC 6052).
	if ip16[0] == 0x00 && ip16[1] == 0x64 && ip16[2] == 0xff && ip16[3] == 0x9b &&
		allZero(ip16[4:12]) {
		return ip16[12:]
	}
	// ISATAP (RFC 5214): interface identifier ::0:5efe:a.b.c.d or
	// ::200:5efe:a.b.c.d — the 5e:fe marker (with the IANA OUI and the
	// universal/local bit in ip16[8]) precedes the embedded IPv4.
	if ip16[9] == 0x00 && ip16[10] == 0x5e && ip16[11] == 0xfe &&
		(ip16[8] == 0x00 || ip16[8] == 0x02) {
		return ip16[12:]
	}
	// SIIT IPv4-translated ::ffff:0:a.b.c.d (RFC 2765, prefix ::ffff:0:0/96):
	// the 0xffff marker sits in bytes [8:10] with [10:12] zero, and the
	// embedded IPv4 is the trailing 32 bits. This is distinct from the
	// IPv4-mapped form (::ffff:a.b.c.d, marker in bytes [10:12]) that
	// net.IP.To4 already exposes, so it is unwrapped here.
	if allZero(ip16[:8]) && ip16[8] == 0xff && ip16[9] == 0xff &&
		ip16[10] == 0x00 && ip16[11] == 0x00 {
		return ip16[12:]
	}
	// IPv4-compatible ::a.b.c.d (first 12 bytes zero). :: and ::1 have already
	// been classified above, so treat any remaining all-zero-prefix address as
	// a wrapper around its trailing IPv4.
	if allZero(ip16[:12]) {
		return ip16[12:]
	}
	return nil
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}
