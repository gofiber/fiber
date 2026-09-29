package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// rawRequest hands app a request line exactly as written, which net/http's
// request builders would reject or rewrite, and returns the response.
func rawRequest(t *testing.T, app *fiber.App, target, host string) *fasthttp.Response {
	t.Helper()

	var req fasthttp.Request
	raw := fiber.MethodGet + " " + target + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n"
	if host == "" {
		// HTTP/1.0 needs no Host header.
		raw = fiber.MethodGet + " " + target + " HTTP/1.0\r\n\r\n"
	}
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	var fctx fasthttp.RequestCtx
	fctx.Init(&req, nil, nil)
	app.Handler()(&fctx)

	resp := fasthttp.AcquireResponse()
	fctx.Response.CopyTo(resp)
	return resp
}

// echoTarget serves an upstream that answers every request with the target
// it was handed, exactly as it arrived on the wire.
func echoTarget(t *testing.T) string {
	t.Helper()

	target := fiber.New()
	target.Use(func(c fiber.Ctx) error {
		return c.SendString(c.OriginalURL())
	})

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	startServer(target, ln)

	return ln.Addr().String()
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return string(b)
}

func forbid(c fiber.Ctx) error {
	return c.SendStatus(fiber.StatusForbidden)
}

// go test -run Test_Proxy_Balancer_ForwardsRoutedPath
func Test_Proxy_Balancer_ForwardsRoutedPath(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	// Every mount and every expectation is the same for both apps: what
	// differs is which spellings the router resolves before matching.
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/admin", forbid)
		app.Use(Balancer(Config{Servers: []string{addr}}))

		for target, want := range map[string]string{
			"/public/%41?x=%2F&y=1": "/public/A?x=%2F&y=1",
			"/public/%zz":           "/public/%25zz",
			"/public/a%3Ab%20c":     "/public/a%3Ab%20c",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		for _, target := range []string{"/admin/secret", "/public/../admin/secret", "/%2e%2e/admin/secret", "/public/%2E./admin/secret"} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusForbidden, resp.StatusCode(), target)
		}
		// Kept as routed, an escaped slash or an empty segment would be read
		// as another path by an upstream that decodes or merges: refused.
		for _, target := range []string{"/public/..%2Fadmin/secret", "/public%2F..%2Fadmin", "//admin/secret", "/public//x"} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), target)
		}
	})

	t.Run("unescape path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Use("/admin", forbid)
		app.Use(Balancer(Config{Servers: []string{addr}}))

		for target, want := range map[string]string{
			"/public/a%2Fb":         "/public/a/b",
			"/public/%41%20b?x=%2F": "/public/A%20b?x=%2F",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		for _, target := range []string{"/public/..%2Fadmin/secret", "/public%2F..%2Fadmin/secret", "/%2e%2e/admin/secret"} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusForbidden, resp.StatusCode(), target)
		}
		// The decoded path keeps an empty segment, which is refused.
		require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, "//admin/secret", "example.com").StatusCode())
	})
}

// go test -run Test_Proxy_Balancer_ModifyRequestCanRetarget
func Test_Proxy_Balancer_ModifyRequestCanRetarget(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	app := fiber.New()
	app.Use("/set-path", Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(c fiber.Ctx) error {
			c.Request().URI().SetPath("/other")
			return nil
		},
	}))
	app.Use("/set-uri", Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(c fiber.Ctx) error {
			c.Request().SetRequestURI("/other?y=2")
			return nil
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/set-path?x=1", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "/other?x=1", readBody(t, resp))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/set-uri?x=1", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "/other?y=2", readBody(t, resp))
}

// go test -run Test_Proxy_Balancer_RestoresRequestLine
func Test_Proxy_Balancer_RestoresRequestLine(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		require.Equal(t, "/public/%41?x=%2F", c.OriginalURL())
		return err
	})
	app.Use(Balancer(Config{Servers: []string{addr}}))

	resp := rawRequest(t, app, "/public/%41?x=%2F", "example.com")
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "/public/A?x=%2F", string(resp.Body()))
}

// go test -run Test_Proxy_Balancer_RejectsHostUserinfo
func Test_Proxy_Balancer_RejectsHostUserinfo(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		hits.Add(1)
		return c.SendString(c.Get(fiber.HeaderAuthorization))
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))

	resp := rawRequest(t, app, "/", "svc:pw@"+addr)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode())
	require.Equal(t, int32(0), hits.Load(), "the upstream must not see the request")

	resp = rawRequest(t, app, "/", addr)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Empty(t, string(resp.Body()), "no Authorization header is forged")
	require.Equal(t, int32(1), hits.Load())
}

// go test -run Test_Proxy_Balancer_RejectsNonOriginServer
func Test_Proxy_Balancer_RejectsNonOriginServer(t *testing.T) {
	t.Parallel()

	for _, server := range []string{
		"http://127.0.0.1:1/api",
		"http://127.0.0.1:1/api/",
		"127.0.0.1:1/api",
		"http://svc:pw@127.0.0.1:1",
		"http://127.0.0.1:1?x=1",
		"http://127.0.0.1:1?",
		"http://127.0.0.1:1#frag",
	} {
		require.PanicsWithError(t, ErrUpstreamNotOrigin.Error()+": "+strconv.Quote(server), func() {
			Balancer(Config{Servers: []string{server}})
		}, server)
	}

	require.NotPanics(t, func() {
		Balancer(Config{Servers: []string{"http://127.0.0.1:1", "http://127.0.0.1:1/", "127.0.0.1:1", "https://[::1]:1"}})
	})
}

// go test -run Test_Proxy_DomainForward_ForwardsRoutedPath
func Test_Proxy_DomainForward_ForwardsRoutedPath(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	t.Run("upstream with a path prefix", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/admin", forbid)
		app.Use(DomainForward("example.com", "http://"+addr+"/api/"))

		for target, want := range map[string]string{
			"/../internal":     "/api/internal",
			"/%2e%2e/internal": "/api/internal",
			"/x/%41?q=%2F&y=1": "/api/x/A?q=%2F&y=1",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		resp := rawRequest(t, app, "/%2e%2e/admin/secret", "example.com")
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
		for _, target := range []string{"//internal", "/..%2Finternal", "/public/..%2Fadmin/secret"} {
			require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), target)
		}
	})

	t.Run("plain upstream", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/admin", forbid)
		app.Use(DomainForward("example.com", "http://"+addr))

		for target, want := range map[string]string{
			"/a/../public/%zz?x=1": "/public/%25zz?x=1",
			"/public/a%3Ab":        "/public/a%3Ab",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		resp := rawRequest(t, app, "/public/../admin/secret", "example.com")
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
		for _, target := range []string{"//admin/secret", "/public/..%2Fadmin/secret"} {
			require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), target)
		}
	})
}

// go test -run Test_Proxy_BalancerForward_ForwardsRoutedPath
func Test_Proxy_BalancerForward_ForwardsRoutedPath(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	app := fiber.New()
	app.Use("/admin", forbid)
	app.Use(BalancerForward([]string{"http://" + addr + "/api/"}))

	for target, want := range map[string]string{
		"/../internal":  "/api/internal",
		"/x/%41?q=%2F":  "/api/x/A?q=%2F",
		"/public/a%3Ab": "/api/public/a%3Ab",
	} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
		require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
	}
	resp := rawRequest(t, app, "/%2e%2e/admin/secret", "example.com")
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
	for _, target := range []string{"//internal", "/public/..%2Fadmin/secret"} {
		require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), target)
	}
}

// Test_Proxy_RejectsEncodedSeparator pins that a routed path an upstream could
// read as another path is refused by every forwarding handler rather than
// forwarded: an upstream that decodes "%2F" into a separator or merges "//"
// before it matches routes, as a net/http file server or a fasthttp server
// does, would serve "/admin/secret.txt" for each refused target although no
// middleware on "/admin" ran. An escape of another reserved character is
// forwarded as sent.
func Test_Proxy_RejectsEncodedSeparator(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	handlers := map[string]func() fiber.Handler{
		"Balancer":        func() fiber.Handler { return Balancer(Config{Servers: []string{addr}}) },
		"DomainForward":   func() fiber.Handler { return DomainForward("example.com", "http://"+addr) },
		"BalancerForward": func() fiber.Handler { return BalancerForward([]string{"http://" + addr}) },
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/admin", forbid)
			app.Use(handler())

			for _, target := range []string{
				"/public/..%2Fadmin/secret.txt",
				"/public/..%2fadmin/secret.txt",
				"/admin%2Fsecret.txt",
				"//admin/secret.txt",
			} {
				require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), target)
			}

			// A separator forged by a stray "%": routed as "..%2fadmin" when
			// the router keeps the stray "%" as sent, and refused; routed as
			// "..%252fadmin" when it writes the stray "%" as "%25", and
			// forwarded as that literal name.
			resp := rawRequest(t, app, "/public/..%%32fadmin/secret.txt", "example.com")
			switch resp.StatusCode() {
			case fiber.StatusBadRequest:
			case fiber.StatusOK:
				require.Equal(t, "/public/..%252fadmin/secret.txt", string(resp.Body()))
			default:
				t.Fatalf("unexpected status %d", resp.StatusCode())
			}

			resp = rawRequest(t, app, "/files/a%20b%3Fc?x=%2F", "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "/files/a%20b%3Fc?x=%2F", string(resp.Body()))
		})
	}
}

// go test -run Test_Proxy_Do_UserClientForwardsTargetAsGiven
func Test_Proxy_Do_UserClientForwardsTargetAsGiven(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	// A client left with path normalization on would decode "%2F", merge
	// "//" and resolve ".." on the way out, and the upstream would see
	// "/a/b/d?q=%2F" for a target the handler never wrote.
	target := "http://" + addr + "/a%2Fb//c/..%2Fd?q=%2F"
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return Do(c, target, &fasthttp.Client{})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/a%2Fb//c/..%2Fd?q=%2F", readBody(t, resp))
}

// Test_Proxy_Balancer_RejectsAuthorityTarget pins that a routed target
// fasthttp would read as an authority never reaches the upstream: one that
// begins with "//" and holds "://", here built by ".." resolution so that the
// raw request line never held it and the server never split it, or one that
// arrives without a Host header. Set as the request URI, fasthttp would take
// the upstream's Host and a Basic credential from it, over the ones the
// application pinned.
func Test_Proxy_Balancer_RejectsAuthorityTarget(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	target := fiber.New()
	target.Use(func(c fiber.Ctx) error {
		hits.Add(1)
		return c.SendString(c.OriginalURL() + "|" + string(c.Request().Header.Host()) + "|" + c.Get(fiber.HeaderAuthorization))
	})
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	startServer(target, ln)
	addr := ln.Addr().String()

	for _, unescape := range []bool{false, true} {
		app := fiber.New(fiber.Config{UnescapePath: unescape})
		app.Use(Balancer(Config{
			Servers: []string{addr},
			ModifyRequest: func(c fiber.Ctx) error {
				c.Request().Header.SetHost("backend.internal")
				c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer pinned")
				return nil
			},
		}))

		before := hits.Load()
		// Neither raw line holds "://": fasthttp's server would read one that
		// did as the absolute form and split the host off before routing.
		rejected := []string{
			"//u:pw@evil.example/a:/x/..//b",
			"//u:pw@evil.example/a:/x/%2E%2E//b",
		}
		if unescape {
			rejected = append(rejected, "//u:pw@evil.example/a%3A%2F%2Fb")
		}
		for _, tgt := range rejected {
			resp := rawRequest(t, app, tgt, "example.com")
			require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), "UnescapePath=%v %s", unescape, tgt)
		}
		// Without a Host header fasthttp reads the authority out of any
		// target that begins with "//"; the ".." keeps the server from
		// reading it out of the raw line first.
		resp := rawRequest(t, app, "/x/..//u:pw@evil.example/y", "")
		require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), "UnescapePath=%v, no Host", unescape)
		require.Equal(t, before, hits.Load(), "UnescapePath=%v: the upstream must not see the request", unescape)

		// The same characters in a path with no empty segment are forwarded
		// as the router matched them, under the identity the application
		// pinned.
		resp = rawRequest(t, app, "/u:pw@evil.example/a:/b", "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), "UnescapePath=%v", unescape)
		require.Equal(t, "/u:pw@evil.example/a:/b|backend.internal|Bearer pinned", string(resp.Body()), "UnescapePath=%v", unescape)
	}
}

// echoIdentity starts an upstream that answers with the request line it
// received, the Host it was addressed to and the Authorization it carried,
// and returns its address.
func echoIdentity(t *testing.T) string {
	t.Helper()

	target := fiber.New()
	target.Use(func(c fiber.Ctx) error {
		return c.SendString(c.OriginalURL() + "|" + string(c.Request().Header.Host()) + "|" + c.Get(fiber.HeaderAuthorization))
	})

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	startServer(target, ln)

	return ln.Addr().String()
}

// pinIdentity is a ModifyRequest that sets the Host and Authorization the
// upstream must see whatever the request carried.
func pinIdentity(c fiber.Ctx) error {
	c.Request().Header.SetHost("backend.internal")
	c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer pinned")
	return nil
}

// Test_Proxy_Balancer_RootsUnrootedPath pins that a path override without a
// leading slash, as a rewrite of "/go/*" to "$1" produces, is handled as a
// path and not as the absolute URL a request line reads it as, which would
// have replaced the pinned Host and credential: rooted, it is forwarded under
// the pinned identity, or refused when the rooted path holds an empty
// segment, as "/http://u:pw@evil.example/x" does.
func Test_Proxy_Balancer_RootsUnrootedPath(t *testing.T) {
	t.Parallel()

	addr := echoIdentity(t)
	balancer := Balancer(Config{Servers: []string{addr}, ModifyRequest: pinIdentity})
	app := fiber.New()
	app.Get("/go/*", func(c fiber.Ctx) error {
		c.Path(c.Params("*"))
		return balancer(c)
	})

	resp := rawRequest(t, app, "/go/u:pw@evil.example/x", "example.com")
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "/u:pw@evil.example/x|backend.internal|Bearer pinned", string(resp.Body()))

	require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, "/go/http://u:pw@evil.example/x", "example.com").StatusCode())
}

// Test_Proxy_Balancer_CustomClientKeepsTarget pins that a host client the
// caller built is told to keep the request line as given: left normalizing it
// would decode "%2F" into a separator and merge "//" on the way out, and the
// upstream would see a path no middleware had matched.
func Test_Proxy_Balancer_CustomClientKeepsTarget(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	hc := &fasthttp.HostClient{Addr: addr, NoDefaultUserAgentHeader: true}
	app := fiber.New()
	app.Use("/admin", forbid)
	app.Use(Balancer(Config{Client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hc}, Timeout: time.Second}}))

	require.True(t, hc.DisablePathNormalizing)
	for _, target := range []string{"/public/a%3Ab", "/a%20b?q=%2F"} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
		require.Equal(t, target, string(resp.Body()), target)
	}
	for _, target := range []string{"/public/..%2Fadmin/secret", "//admin/secret"} {
		require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), target)
	}
}

// opaqueClient is a BalancingClient the proxy cannot configure. It forwards
// through a host client left normalizing, as a caller's own implementation
// might.
type opaqueClient struct {
	hc *fasthttp.HostClient
}

func (o *opaqueClient) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return o.hc.DoDeadline(req, resp, deadline)
}

func (o *opaqueClient) PendingRequests() int {
	return o.hc.PendingRequests()
}

// Test_Proxy_Balancer_OpaqueClientFailsClosed pins that behind a
// BalancingClient the proxy cannot keep from normalizing, a target that
// normalization would change is refused rather than sent as another path.
func Test_Proxy_Balancer_OpaqueClientFailsClosed(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	app := fiber.New()
	app.Use(Balancer(Config{Client: &fasthttp.LBClient{
		Clients: []fasthttp.BalancingClient{&opaqueClient{hc: &fasthttp.HostClient{Addr: addr}}},
		Timeout: time.Second,
	}}))

	for _, target := range []string{"/public/..%2Fadmin/secret", "//admin/secret", "/a%20b"} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), target)
	}
	resp := rawRequest(t, app, "/plain/path?q=%2F", "example.com")
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "/plain/path?q=%2F", string(resp.Body()))
}

// requireForgedEscapeRefused checks a response to
// "/static/%%370rivate/secret.txt", an escape of an unreserved character
// forged by a stray "%", which must never reach the upstream as something it
// would decode into the guarded name. What the router hands the proxy depends
// on how it spells a stray "%": kept as sent, the request is routed as
// "/static/%70rivate/secret.txt" and refused; written as "%25", it is routed
// as "/static/%2570rivate/secret.txt", holds no escape of an unreserved
// character, and is forwarded as that literal name, prefix included.
func requireForgedEscapeRefused(t *testing.T, resp *fasthttp.Response, prefix string) {
	t.Helper()
	switch resp.StatusCode() {
	case fiber.StatusBadRequest:
	case fiber.StatusOK:
		require.Equal(t, prefix+"/static/%2570rivate/secret.txt", string(resp.Body()))
	default:
		t.Fatalf("unexpected status %d", resp.StatusCode())
	}
}

func Test_Proxy_Balancer_RejectsForgedEscape(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	app := fiber.New()
	app.Use("/static/private", forbid)
	app.Use(Balancer(Config{Servers: []string{addr}}))

	requireForgedEscapeRefused(t, rawRequest(t, app, "/static/%%370rivate/secret.txt", "example.com"), "")
}

func Test_Proxy_DomainForward_RejectsForgedEscape(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	app := fiber.New()
	app.Use("/static/private", forbid)
	app.Use(DomainForward("example.com", "http://"+addr+"/api/"))

	requireForgedEscapeRefused(t, rawRequest(t, app, "/static/%%370rivate/secret.txt", "example.com"), "/api")
}

func Test_Proxy_BalancerForward_RejectsForgedEscape(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	app := fiber.New()
	app.Use("/static/private", forbid)
	app.Use(BalancerForward([]string{"http://" + addr}))

	requireForgedEscapeRefused(t, rawRequest(t, app, "/static/%%370rivate/secret.txt", "example.com"), "")
}
