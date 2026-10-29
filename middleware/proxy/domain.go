package proxy

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/errorreplay"
	"github.com/gofiber/fiber/v3/internal/idnafold"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
)

// foldHostnameLabel returns hostname with its host label folded to lowercase
// Punycode, preserving an explicit port unchanged. It is the construction-time
// counterpart to hostWithoutPort: that strips a port from the per-request wire
// value, this normalizes the configured value once so the two sides compare
// correctly regardless of how the operator spelled the domain.
func foldHostnameLabel(hostname string) string {
	if host, port, ok := utils.SplitHostPort(hostname); ok {
		return net.JoinHostPort(idnafold.ToASCII(utilsstrings.ToLower(host)), port)
	}
	return idnafold.ToASCII(utilsstrings.ToLower(hostname))
}

// hostWithoutPort strips the port from a Host header value, keeping a bracketed IPv6 literal.
func hostWithoutPort(host string) string {
	if strings.HasPrefix(host, "[") {
		if end := strings.IndexByte(host, ']'); end != -1 {
			return host[:end+1]
		}
		return host
	}
	if i := strings.LastIndexByte(host, ':'); i != -1 && strings.IndexByte(host[:i], ':') == -1 {
		return host[:i]
	}
	return host
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
		// Host names are case-insensitive (RFC 9110 §4.2.3) and fasthttp
		// does not case-fold the raw Host header, so compare with
		// EqualFold — otherwise "API.Example.com" would slip past a
		// DomainForward("api.example.com", ...) gate and be passed
		// through unproxied.
		// Match the host with or without its port; another host is passed on to the next handler.
		host := utils.UnsafeString(c.Request().Host())
		if !utils.EqualFold(host, hostname) && !utils.EqualFold(hostWithoutPort(host), hostname) {
			return c.Next()
		}
		// See Balancer: a refused request is not forwarded.
		if errorreplay.Active(c.RequestCtx()) {
			return c.Next()
		}
		return forwardRouted(c, base, clients...)
	}
}
