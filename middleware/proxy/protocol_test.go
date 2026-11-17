package proxy

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_Proxy_LeavesTheProtocolAsReceived pins that the request keeps its protocol after Do and Balancer spoke
// HTTP/1.1 to the upstream: c.SendEarlyHints (RFC 9110 Section 15.2 forbids a 1xx for an HTTP/1.0 client)
// and the logger read it after Next returns.
func Test_Proxy_LeavesTheProtocolAsReceived(t *testing.T) {
	t.Parallel()
	upstream := fiber.New()
	upstream.All("/*", func(c fiber.Ctx) error { return c.SendString(c.Protocol()) })
	addr := listenUpstream(t, upstream)
	for _, proto := range []string{"HTTP/2", "HTTP/1.0", "HTTP/1.1"} {
		for name, forward := range map[string]fiber.Handler{
			"Do":       func(c fiber.Ctx) error { return Do(c, "http://"+addr+"/") },
			"Balancer": Balancer(Config{Servers: []string{"http://" + addr}}),
		} {
			t.Run(name+"/"+proto, func(t *testing.T) {
				t.Parallel()
				app := fiber.New()
				var before, after string
				app.Use(func(c fiber.Ctx) error {
					c.Request().Header.SetProtocol(proto)
					before = strings.Clone(c.Protocol())
					err := c.Next()
					after = c.Protocol()
					return err
				})
				app.Use(forward)
				// the upstream is always spoken to in HTTP/1.1
				apptest.ExpectBody(t, app, "/", fiber.StatusOK, "HTTP/1.1")
				require.Equal(t, proto, before)
				require.Equal(t, proto, after)
			})
		}
	}
}

// Test_Proxy_Do_LeavesTheUserAgentAsReceived pins that the User-Agent fasthttp gives a request that has none,
// the name of the client or "fasthttp", does not stay on the request for a later hop or for the middleware
// that runs after Next.
func Test_Proxy_Do_LeavesTheUserAgentAsReceived(t *testing.T) {
	t.Parallel()
	echo := func() string {
		upstream := fiber.New()
		upstream.All("/*", func(c fiber.Ctx) error { return c.SendString("ua=" + c.Get(fiber.HeaderUserAgent)) })
		return listenUpstream(t, upstream)
	}
	checkAddr, nextAddr := echo(), echo()
	for _, tc := range []struct {
		client  *fasthttp.Client
		name    string
		inbound string
		doSees  string
	}{
		{name: "default fasthttp client", client: &fasthttp.Client{}, doSees: "ua=fasthttp"},
		{name: "named client", client: &fasthttp.Client{Name: "svc"}, doSees: "ua=svc"},
		{name: "client without a default", client: &fasthttp.Client{NoDefaultUserAgentHeader: true}, doSees: "ua="},
		{name: "User-Agent of the client kept", client: &fasthttp.Client{}, inbound: "curl/8", doSees: "ua=curl/8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			var seen, after string
			app.Use(func(c fiber.Ctx) error {
				if err := Do(c, "http://"+checkAddr+"/", tc.client); err != nil {
					return err
				}
				seen, after = string(c.Response().Body()), c.Get(fiber.HeaderUserAgent)
				return c.Next()
			})
			app.Use(Forward("http://" + nextAddr))
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderUserAgent, tc.inbound))
			resp := apptest.Expect(t, app, req, fiber.StatusOK)
			require.Equal(t, tc.doSees, seen)
			require.Equal(t, tc.inbound, after)
			require.Equal(t, "ua="+tc.inbound, apptest.Body(t, resp), "the next hop got what the client sent")
		})
	}
}

// Test_Proxy_Balancer_LeavesTheUserAgentAndHostAsReceived pins what Do's tests above pin, for Balancer behind a host client of
// the caller's: such a client gives a request that has no User-Agent its name, or "fasthttp", and writes the host of the
// URI it parsed, in lower case, into the Host field. Neither stays on the request for the middleware that runs after Next
// or for a later hop.
func Test_Proxy_Balancer_LeavesTheUserAgentAndHostAsReceived(t *testing.T) {
	t.Parallel()
	upstream := fiber.New()
	upstream.All("/*", func(c fiber.Ctx) error { return c.SendString("ua=" + c.Get(fiber.HeaderUserAgent)) })
	addr := listenUpstream(t, upstream)
	for _, tc := range []struct {
		client  *fasthttp.HostClient
		name    string
		inbound string
		sees    string
	}{
		{name: "default host client", client: &fasthttp.HostClient{Addr: addr}, sees: "ua=fasthttp"},
		{name: "named host client", client: &fasthttp.HostClient{Addr: addr, Name: "svc"}, sees: "ua=svc"},
		{name: "host client without a default", client: &fasthttp.HostClient{Addr: addr, NoDefaultUserAgentHeader: true}, sees: "ua="},
		{name: "User-Agent of the client kept", client: &fasthttp.HostClient{Addr: addr}, inbound: "curl/8", sees: "ua=curl/8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			var userAgent, host string
			app.Use(func(c fiber.Ctx) error {
				err := c.Next()
				userAgent, host = c.Get(fiber.HeaderUserAgent), string(c.Request().Header.Host())
				return err
			})
			app.Use(Balancer(Config{Client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{tc.client}}}))
			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderUserAgent, tc.inbound), apptest.WithHost("Example.COM"))
			require.Equal(t, tc.sees, apptest.Body(t, apptest.Expect(t, app, req, fiber.StatusOK)))
			require.Equal(t, tc.inbound, userAgent)
			require.Equal(t, "Example.COM", host)
		})
	}
}
