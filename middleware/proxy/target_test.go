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
			"/public/..%2Fadmin/secret": "/public/..%2Fadmin/secret",
			"/public%2F..%2Fadmin":      "/public%2F..%2Fadmin",
			"//admin/secret":            "//admin/secret",
			"/public//x":                "/public//x",
			"/public/%41?x=%2F&y=1":     "/public/A?x=%2F&y=1",
			"/public/%zz":               "/public/%25zz",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		for _, target := range []string{"/admin/secret", "/public/../admin/secret", "/%2e%2e/admin/secret", "/public/%2E./admin/secret"} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusForbidden, resp.StatusCode(), target)
		}
	})

	t.Run("unescape path", func(t *testing.T) {
		t.Parallel()
		app := fiber.New(fiber.Config{UnescapePath: true})
		app.Use("/admin", forbid)
		app.Use(Balancer(Config{Servers: []string{addr}}))

		for target, want := range map[string]string{
			"/public/a%2Fb":         "/public/a/b",
			"//admin/secret":        "//admin/secret",
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
			"/../internal":              "/api/internal",
			"/%2e%2e/internal":          "/api/internal",
			"//internal":                "/api//internal",
			"/..%2Finternal":            "/api/..%2Finternal",
			"/x/%41?q=%2F&y=1":          "/api/x/A?q=%2F&y=1",
			"/public/..%2Fadmin/secret": "/api/public/..%2Fadmin/secret",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		resp := rawRequest(t, app, "/%2e%2e/admin/secret", "example.com")
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
	})

	t.Run("plain upstream", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use("/admin", forbid)
		app.Use(DomainForward("example.com", "http://"+addr))

		for target, want := range map[string]string{
			"//admin/secret":            "//admin/secret",
			"/public/..%2Fadmin/secret": "/public/..%2Fadmin/secret",
			"/a/../public/%zz?x=1":      "/public/%25zz?x=1",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
			require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
		}
		resp := rawRequest(t, app, "/public/../admin/secret", "example.com")
		require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
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
		"/../internal":              "/api/internal",
		"//internal":                "/api//internal",
		"/public/..%2Fadmin/secret": "/api/public/..%2Fadmin/secret",
	} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
		require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
	}
	resp := rawRequest(t, app, "/%2e%2e/admin/secret", "example.com")
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode())
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
