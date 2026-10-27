package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
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
	method := fiber.MethodGet
	if target == "*" {
		method = fiber.MethodOptions // the only method asterisk-form is for
	}
	raw := method + " " + target + " HTTP/1.1\r\nHost: " + host + "\r\n\r\n"
	if host == "" {
		// HTTP/1.0 needs no Host header.
		raw = method + " " + target + " HTTP/1.0\r\n\r\n"
	}
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	var fctx fasthttp.RequestCtx
	fctx.Init(&req, nil, nil)
	app.Handler()(&fctx)

	resp := fasthttp.AcquireResponse()
	fctx.Response.CopyTo(resp)
	return resp
}

// listenUpstream serves upstream on a loopback port until the test ends and
// returns its address.
func listenUpstream(t *testing.T, upstream *fiber.App) string {
	t.Helper()

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	startServer(upstream, ln)

	return ln.Addr().String()
}

// echoTarget serves an upstream that answers every request with the target
// it was handed, exactly as it arrived on the wire.
func echoTarget(t *testing.T) string {
	t.Helper()

	target := fiber.New()
	target.Use(func(c fiber.Ctx) error {
		return c.SendString(c.OriginalURL())
	})
	return listenUpstream(t, target)
}

// forwardingHandlers returns the helpers that forward the routed path: those
// of realIPHandlers but Forward, which sends to the address it was given.
func forwardingHandlers() []realIPHandler {
	return slices.DeleteFunc(realIPHandlers(), func(h realIPHandler) bool { return h.name == "Forward" })
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

// Test_Proxy_Balancer_AbsoluteFormHost pins that the upstream is addressed by
// the host c.Host() and the host checks went by: an absolute-form line naming
// another host than the Host field is refused (RFC 9112 Sections 3.2, 3.2.2).
func Test_Proxy_Balancer_AbsoluteFormHost(t *testing.T) {
	t.Parallel()
	addr := echoIdentity(t)

	var routed string
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		routed = strings.Clone(c.Host())
		return c.Next()
	})
	app.Use(Balancer(Config{Servers: []string{addr}}))

	for _, tc := range []struct{ target, host, want string }{
		{"http://public.example/x", "internal.example", ""},
		{"http://public.example:8080/x", "public.example", ""},
		{"http://public.example:443/x", "public.example", ""},
		{"https://u:pw@public.example/x", "internal.example", ""},
		{"//public.example/a?x=://", "internal.example", ""},
		{"http://public.example/x", "svc:pw@public.example", ""},
		{"http:///x", "internal.example", ""},
		{"http://public.example/x", "", ""},
		// c.Host() keeps the escape, which fasthttp decodes in the Host it sends upstream.
		{"http://public%25c3%25a9.example/x", "public%c3%a9.example", ""},
		{"http://public.example/x", "public.example/evil", ""},
		{"http://PUBLIC.example:8080/x?q=1", "Public.Example:8080", "/x?q=1|public.example:8080|"},
		{"https://u:pw@public.example/x", "public.example", "/x|public.example|"},
		{"//public.example/a?x=://", "public.example", "/a?x=://|public.example|"},
		{"/x", "internal.example", "/x|internal.example|"},
	} {
		resp := rawRequest(t, app, tc.target, tc.host)
		if tc.want == "" {
			require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), tc.target)
			continue
		}
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), tc.target)
		require.Equal(t, tc.want, string(resp.Body()), tc.target)
		require.Contains(t, tc.want, "|"+routed+"|", tc.target)
	}

	// A port that is empty or the scheme's default names the same host (RFC 9110 Section 4.2.3).
	for target, host := range map[string]string{
		"http://public.example:80/x":   "public.example",
		"http://public.example:/x":     "public.example",
		"https://public.example:443/x": "public.example",
		"http://public.example/x":      "public.example:80",
		"http://[::1]:80/x":            "[::1]",
		"http://my_svc:80/x":           "my_svc",
	} {
		resp := rawRequest(t, app, target, host)
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
		require.Equal(t, "/x|"+host+"|", string(resp.Body()), target)
	}

	// A Host set before the handler is kept, and no request line replaces it.
	pinned := fiber.New()
	pinned.Use(func(c fiber.Ctx) error {
		c.Request().Header.SetHost("early.internal")
		return c.Next()
	})
	pinned.Use(Balancer(Config{Servers: []string{addr}}))
	require.Equal(t, "/x|early.internal|", string(rawRequest(t, pinned, "/x", "public.example").Body()))
	require.Equal(t, "/*|early.internal|", string(rawRequest(t, pinned, "*", "public.example").Body()))
	require.Equal(t, fiber.StatusBadRequest, rawRequest(t, pinned, "http://other.internal/x", "public.example").StatusCode())
	require.Equal(t, fiber.StatusBadRequest, rawRequest(t, pinned, "http://public.example/x", "public.example").StatusCode())

	// ModifyRequest runs after the check, so a Host pinned there serves absolute-form requests too.
	late := fiber.New()
	late.Use(Balancer(Config{Servers: []string{addr}, ModifyRequest: pinIdentity}))
	require.Equal(t, "/x|backend.internal|Bearer pinned", string(rawRequest(t, late, "http://public.example/x", "public.example").Body()))
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

// Test_Proxy_Forward_EscapedUpstreamPathKeepsTarget pins that an escape in the
// upstream URL's path does not cost the routed target its escapes: with "%3B"
// sent as ";", a servlet container serves "/admin/secret" for
// "/public/..%3B/admin/secret" although no middleware on "/admin" ran.
func Test_Proxy_Forward_EscapedUpstreamPathKeepsTarget(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	for name, handler := range map[string]fiber.Handler{
		"DomainForward":   DomainForward("example.com", "http://"+addr+"/my%20app"),
		"BalancerForward": BalancerForward([]string{"http://" + addr + "/my%20app"}),
	} {
		app := fiber.New()
		app.Use("/admin", forbid)
		app.Use(handler)

		for target, want := range map[string]string{
			"/public/x":                  "/my%20app/public/x",
			"/public/..%3B/admin/secret": "/my%20app/public/..%3B/admin/secret",
			"/..%3B/other/a%40b?q=%3B":   "/my%20app/..%3B/other/a%40b?q=%3B",
		} {
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode(), "%s %s", name, target)
			require.Equal(t, want, string(resp.Body()), "%s upstream target for %q", name, target)
		}
		require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, "/public/..;/admin/secret", "example.com").StatusCode(), name)
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

	for _, h := range forwardingHandlers() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use("/admin", forbid)
			app.Use(h.build(addr))

			for _, target := range []string{
				"/public/..%2Fadmin/secret.txt",
				"/public/..%2fadmin/secret.txt",
				"/admin%2Fsecret.txt",
				"//admin/secret.txt",
				// a separator to WHATWG URL parsers and IIS
				"/admin\\secret.txt",
				"/public/..\\admin/secret.txt",
				"/public/..%5Cadmin/secret.txt",
				"/public/..%5cadmin/secret.txt",
				// a dot segment once a servlet container strips its parameters
				"/public/..;/admin/secret.txt",
				"/public/.;x/../..;jsessionid=1/admin/secret.txt",
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

// Test_Proxy_RejectsPathParameters pins that a path parameter, a ";" in any
// segment, is refused by every forwarding handler by default. A servlet
// container strips the parameters from every segment before it matches routes,
// so "/admin;x/secret", "/admin;/secret" and "/;/admin/secret" are
// "/admin/secret" there, although no middleware on "/admin" ran for them.
func Test_Proxy_RejectsPathParameters(t *testing.T) {
	t.Parallel()

	for _, h := range forwardingHandlers() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()
			// an upstream that counts what it is handed and echoes it
			var hits atomic.Int32
			upstream := fiber.New()
			upstream.Use(func(c fiber.Ctx) error {
				hits.Add(1)
				return c.SendString(c.OriginalURL())
			})
			addr := listenUpstream(t, upstream)
			app := fiber.New()
			app.Use("/admin", forbid)
			app.Use(h.build(addr))

			// the guard still answers for the path it is mounted on
			require.Equal(t, fiber.StatusForbidden, rawRequest(t, app, "/admin/secret", "example.com").StatusCode())
			resp := rawRequest(t, app, "/public/page?x=1", "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "/public/page?x=1", string(resp.Body()))
			require.Equal(t, int32(1), hits.Load())

			for _, target := range []string{
				"/admin;x/secret",
				"/admin;/secret",
				"/;/admin/secret",
				"/;x/admin/secret",
				"/public/;/admin/secret",
				"/public/..;/admin/secret",
				"/public/.;x/../..;jsessionid=1/admin/secret",
				// a parameter on an ordinary segment, whether or not it is the last
				"/public;x/page",
				"/public/page;jsessionid=1",
				"/public/page;jsessionid=1?x=1",
			} {
				refused := rawRequest(t, app, target, "example.com")
				require.Equal(t, fiber.StatusBadRequest, refused.StatusCode(), "%s: %s", target, refused.Body())
			}
			require.Equal(t, int32(1), hits.Load(), "the upstream must not see a path parameter")

			// A ";" in the query is no path parameter.
			resp = rawRequest(t, app, "/public/page?a=1;b=2", "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "/public/page?a=1;b=2", string(resp.Body()))

			// With UnescapePath the router takes a ";" for part of the name
			// it matches, "admin;x", which is no path under "/admin". It is
			// forwarded as an escape, so the upstream reads that name too.
			unescaping := fiber.New(fiber.Config{UnescapePath: true})
			unescaping.Use("/admin", forbid)
			unescaping.Use(h.build(addr))
			for target, want := range map[string]string{
				"/admin;x/secret":   "/admin%3Bx/secret",
				"/admin%3Bx/secret": "/admin%3Bx/secret",
				"/admin;/secret":    "/admin%3B/secret",
				"/;/admin/secret":   "/%3B/admin/secret",
			} {
				resp := rawRequest(t, unescaping, target, "example.com")
				require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
				require.Equal(t, want, string(resp.Body()), "path read by the upstream for %q", target)
			}
		})
	}
}

// Test_Proxy_EscapesRawBytes pins that a byte a path may not carry raw (RFC
// 3986 Section 3.3) reaches the upstream as its escape, as fasthttp's
// normalizing writer sent it before the proxy forwarded the routed path: an
// upstream such as Tomcat answers 400 to a raw "|" or "{", and a raw byte of
// UTF-8 is not a valid request line. The sub-delims, ":" and "@" stay raw, but
// for the ";" that begins a path parameter (see Test_Proxy_RejectsPathParameters).
func Test_Proxy_EscapesRawBytes(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	for _, h := range forwardingHandlers() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(h.build(addr))
			for target, want := range map[string]string{
				"/a|b{c}d":              "/a%7Cb%7Bc%7Dd",
				"/q\"^<>`":              "/q%22%5E%3C%3E%60",
				"/\xc3\xbcber?x=1":      "/%C3%BCber?x=1",
				"/keep!$&'()*+,=:@/end": "/keep!$&'()*+,=:@/end",
			} {
				resp := rawRequest(t, app, target, "example.com")
				require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
				require.Equal(t, want, string(resp.Body()), "upstream target for %q", target)
			}
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
	addr := listenUpstream(t, target)

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
	return listenUpstream(t, target)
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
// normalization would change is refused rather than sent as another path,
// while one it leaves alone, including the escapes the proxy writes for raw
// bytes, is forwarded.
func Test_Proxy_Balancer_OpaqueClientFailsClosed(t *testing.T) {
	t.Parallel()

	addr := echoTarget(t)
	app := fiber.New()
	app.Use(Balancer(Config{Client: &fasthttp.LBClient{
		Clients: []fasthttp.BalancingClient{&opaqueClient{hc: &fasthttp.HostClient{Addr: addr}}},
		Timeout: time.Second,
	}}))

	// Normalization would decode "%2F", "%3B" and "%40" into the bytes they
	// name and merge "//".
	for _, target := range []string{"/public/..%2Fadmin/secret", "//admin/secret", "/public/..%3B/admin", "/a%40b"} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusBadRequest, resp.StatusCode(), target)
	}
	for target, want := range map[string]string{
		"/plain/path?q=%2F": "/plain/path?q=%2F",
		"/a%20b":            "/a%20b",
		"/caf\xc3\xa9":      "/caf%C3%A9",
		"/a|b%25":           "/a%7Cb%25",
	} {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), target)
		require.Equal(t, want, string(resp.Body()), target)
	}
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

// allowanceCase is one opt-in of SecurityPolicy and what the forwarding
// handlers do under it: forwarded maps a target to what the upstream reads,
// refused lists the targets answered 400 Bad Request. A forged escape is
// refused, or forwarded as a literal name, under every allowance (see
// requireForgedEscapeRefused).
type allowanceCase struct {
	allow     func(*SecurityPolicy)
	forwarded map[string]string
	name      string
	refused   []string
}

// allowanceCases lists the opt-ins and their combination.
func allowanceCases() []allowanceCase {
	return []allowanceCase{
		{
			// an upstream that keeps "%2F" and "//" as sent, such as one
			// addressing GitLab-style ids or object keys
			name:  "AllowAmbiguousSlashes",
			allow: func(p *SecurityPolicy) { p.AllowAmbiguousSlashes = true },
			forwarded: map[string]string{
				"/api/projects/group%2Fname": "/api/projects/group%2Fname",
				"/bucket/key//with/empty":    "/bucket/key//with/empty",
				"/a%2fb":                     "/a%2Fb",
			},
			// a path parameter still needs AllowPathParameters
			refused: []string{"/admin\\secret", "/a/..%5Cb", "/a/..;/b", "/cart;jsessionid=abc", "/admin;x/secret"},
		},
		{
			// an upstream that relies on ";jsessionid"
			name:  "AllowPathParameters",
			allow: func(p *SecurityPolicy) { p.AllowPathParameters = true },
			forwarded: map[string]string{
				"/cart;jsessionid=abc":   "/cart;jsessionid=abc",
				"/shop;x/cart;y=1?q=1":   "/shop;x/cart;y=1?q=1",
				"/admin;x/secret":        "/admin;x/secret",
				"/;jsessionid=abc":       "/;jsessionid=abc",
				"/app/;jsessionid=abc":   "/app/;jsessionid=abc",
				"/keep!$&'()*+,;=:@/end": "/keep!$&'()*+,;=:@/end",
			},
			// a dot segment with parameters, a backslash and an escaped slash
			// stay refused, and a segment of parameters alone before another
			// segment is an empty one
			refused: []string{
				"/public/..;/admin", "/public/.;x/admin", "/..;jsessionid=1/a", "/a;x/..;/b",
				"/admin\\secret", "/a/..%5Cb", "/a/..%2Fb", "/a%2Fb;c",
				"/;/admin/secret", "/;x/admin", "/public/;/admin", "/a/;x/b",
			},
		},
		{
			name: "both",
			allow: func(p *SecurityPolicy) {
				p.AllowAmbiguousSlashes = true
				p.AllowPathParameters = true
			},
			forwarded: map[string]string{
				"/;/admin/secret":  "/;/admin/secret",
				"/public/;x/admin": "/public/;x/admin",
				"/a%2Fb;c":         "/a%2Fb;c",
				"/group%2Fname;x":  "/group%2Fname;x",
			},
			// a dot segment carrying parameters is refused behind an escaped
			// slash as behind a raw one
			refused: []string{"/public/..;/admin", "/;/..;/admin", "/a%2F..;/b", "/%2F..;/b", "/a%2F.;x/b", "/admin\\secret"},
		},
	}
}

// check sends every target of the case to app, a Fiber app with a forwarding
// handler built under the case's policy, and asserts the answers.
func (tc allowanceCase) check(t *testing.T, app *fiber.App, label string) {
	t.Helper()

	for target, want := range tc.forwarded {
		resp := rawRequest(t, app, target, "example.com")
		require.Equal(t, fiber.StatusOK, resp.StatusCode(), "%s %s", label, target)
		require.Equal(t, want, string(resp.Body()), "%s %s", label, target)
	}
	for _, target := range tc.refused {
		require.Equal(t, fiber.StatusBadRequest, rawRequest(t, app, target, "example.com").StatusCode(), "%s %s", label, target)
	}
	requireForgedEscapeRefused(t, rawRequest(t, app, "/static/%%370rivate/secret.txt", "example.com"), "")
}

// Test_Proxy_Balancer_Allowances pins each opt-in of SecurityPolicy on
// Balancer, which reads it from Config.SecurityPolicy.
func Test_Proxy_Balancer_Allowances(t *testing.T) {
	t.Parallel()
	addr := echoTarget(t)

	for _, tc := range allowanceCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := DefaultSecurityPolicy()
			policy.AllowPrivateIPs = true
			tc.allow(&policy)
			app := fiber.New()
			app.Use(Balancer(Config{Servers: []string{addr}, SecurityPolicy: &policy}))
			tc.check(t, app, "Balancer")
		})
	}
}

// Test_Proxy_Forward_Allowances pins the same opt-ins on DomainForward and
// BalancerForward, which read the policy installed with WithSecurityPolicy.
func Test_Proxy_Forward_Allowances(t *testing.T) {
	// Not parallel: installs the global policy, which every other test in
	// the package reads.
	addr := echoTarget(t)

	for _, tc := range allowanceCases() {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultSecurityPolicy()
			policy.AllowPrivateIPs = true
			tc.allow(&policy)
			withSecurityPolicyForTest(t, policy)
			for _, h := range forwardingHandlers() {
				if h.name == "Balancer" {
					continue // reads Config.SecurityPolicy, not the installed policy
				}
				app := fiber.New()
				app.Use(h.build(addr))
				tc.check(t, app, h.name)
			}
		})
	}
}
