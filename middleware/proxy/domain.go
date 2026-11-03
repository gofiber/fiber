package proxy

import (
	"net"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/errorreplay"
	hostnamepkg "github.com/gofiber/fiber/v3/internal/hostname"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
)

// foldHostnameLabel returns hostname with its host label folded to lowercase
// Punycode, preserving an explicit port unchanged. It is the construction-time
// counterpart to the per-request hostnamepkg.SplitPort in DomainForward: that
// strips a port from the wire value, this normalizes the configured value once
// so the two sides compare correctly regardless of how the operator spelled
// the domain.
func foldHostnameLabel(hostname string) string {
	if host, port, ok := utils.SplitHostPort(hostname); ok {
		return net.JoinHostPort(hostnamepkg.ToASCII(utilsstrings.ToLower(host)), port)
	}
	return hostnamepkg.ToASCII(utilsstrings.ToLower(hostname))
}

// DomainForward performs an http request based on the given domain and populates the given http response.
// This method will return a fiber.Handler.
//
// The upstream addr is validated at handler construction: scheme allowlist,
// SSRF block, and URL parsing all run once. Unlike Balancer (which defers
// DNS to dial time to survive transient resolver failures at startup), this
// construction check resolves the host, so a misconfigured or unresolvable
// addr panics at startup instead of failing per request. Each request also
// re-validates the host against the current policy before dispatch.
//
// SSRF note: the per-request validation is an up-front host check, and
// when AllowPrivateIPs is false the dispatching client's dial-time guard
// re-validates the resolved IP at connect time — so a rebinding-capable
// resolver cannot reach a private address through this handler.
func DomainForward(hostname, addr string, clients ...*fasthttp.Client) fiber.Handler {
	base, err := validateUpstream(addr, currentSecurityPolicy(), nil)
	if err != nil {
		panic(err)
	}
	// A Unicode hostname literal — entirely natural to write for a non-ASCII
	// domain — never appears on the wire: a conforming client always sends
	// the Punycode form (RFC 5891), so folding once here, rather than on
	// every request, is what makes the two sides comparable at all.
	hostname = foldHostnameLabel(hostname)
	return func(c fiber.Ctx) error {
		// Request.Host is URI().Host, which fasthttp's URI.parse validates
		// (parseHost) and then ASCII-lowercases; the server answers 400 when
		// that parse fails, so a served request always reaches here with a
		// lowercase, well-formed host and SplitPort's lenient fallbacks are
		// never taken. Host names are case-insensitive (RFC 9110 §4.2.3);
		// EqualFold stays so the gate does not depend on that guarantee for
		// a request built by hand rather than read off the wire.
		// Match the host with or without its port; another host is passed on to the next handler.
		host := utils.UnsafeString(c.Request().Host())
		if !utils.EqualFold(host, hostname) {
			if bare, _ := hostnamepkg.SplitPort(host); !utils.EqualFold(bare, hostname) {
				return c.Next()
			}
		}
		// See Balancer: a refused request is not forwarded.
		if errorreplay.Active(c.RequestCtx()) {
			return c.Next()
		}
		return forwardRouted(c, base, clients...)
	}
}
