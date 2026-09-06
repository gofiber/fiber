package proxy

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	clientpkg "github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/valyala/fasthttp"
)

// TestMain relaxes the proxy security policy for the whole suite so the
// existing loopback-based tests continue to work. Tests that exercise
// the secure defaults install their own policy in their own scope.
func TestMain(m *testing.M) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	WithSecurityPolicy(policy)
	m.Run()
}

func startServer(app *fiber.App, ln net.Listener) {
	go func() {
		err := app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		})
		if err != nil {
			panic(err)
		}
	}()
}

func createProxyTestServer(t *testing.T, handler fiber.Handler, network, address string) (target *fiber.App, addr string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned target app and address for readability
	t.Helper()

	target = fiber.New()
	target.Get("/", handler)

	ln, err := net.Listen(network, address)
	require.NoError(t, err)

	addr = ln.Addr().String()

	startServer(target, ln)

	return target, addr
}

func createProxyTestServerIPv4(t *testing.T, handler fiber.Handler) (target *fiber.App, addr string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned target app and address for readability
	t.Helper()
	return createProxyTestServer(t, handler, fiber.NetworkTCP4, "127.0.0.1:0")
}

func createProxyTestServerIPv6(t *testing.T, handler fiber.Handler) (target *fiber.App, addr string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming returned target app and address for readability
	t.Helper()

	// Skip instead of failing on hosts without IPv6 support (e.g. some CI containers).
	probe, err := net.Listen(fiber.NetworkTCP6, "[::1]:0")
	if err != nil {
		t.Skipf("skipping: IPv6 is not available: %v", err)
	}
	require.NoError(t, probe.Close())

	return createProxyTestServer(t, handler, fiber.NetworkTCP6, "[::1]:0")
}

func createRedirectServer(t *testing.T) string {
	t.Helper()
	app := fiber.New()

	var addr string
	app.Get("/", func(c fiber.Ctx) error {
		c.Location("http://" + addr + "/final")
		return c.Status(fiber.StatusMovedPermanently).SendString("redirect")
	})
	app.Get("/final", func(c fiber.Ctx) error {
		return c.SendString("final")
	})

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	addr = ln.Addr().String()

	startServer(app, ln)

	return addr
}

func restoreGlobalProxyClient(t *testing.T) {
	t.Helper()

	prev := client.Load()
	t.Cleanup(func() {
		WithClient(prev)
	})
}

// go test -run Test_Proxy_DefaultClient_MaxConnsPerHost
func Test_Proxy_DefaultClient_MaxConnsPerHost(t *testing.T) {
	require.Equal(t, defaultMaxConnsPerHost, client.Load().MaxConnsPerHost)
}

// go test -run Test_Proxy_ConfigDefault_MaxConnsPerHost
func Test_Proxy_ConfigDefault_MaxConnsPerHost(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{Servers: []string{"127.0.0.1"}})
	require.Equal(t, defaultMaxConnsPerHost, cfg.MaxConnsPerHost)
}

// go test -run Test_Proxy_ConfigDefault_MaxConnsPerHost_Override
func Test_Proxy_ConfigDefault_MaxConnsPerHost_Override(t *testing.T) {
	t.Parallel()

	cfg := configDefault(Config{
		Servers:         []string{"127.0.0.1"},
		MaxConnsPerHost: 2048,
	})
	require.Equal(t, 2048, cfg.MaxConnsPerHost)
}

// go test -run Test_Proxy_Empty_Host
func Test_Proxy_Empty_Upstream_Servers(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			if r != "Servers cannot be empty" {
				panic(r)
			}
		}
	}()
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{}}))
}

// go test -run Test_Proxy_Empty_Config
func Test_Proxy_Empty_Config(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			if r != "Servers cannot be empty" {
				panic(r)
			}
		}
	}()
	app := fiber.New()
	app.Use(Balancer(Config{}))
}

// go test -run Test_Proxy_Next
func Test_Proxy_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{"127.0.0.1"},
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Proxy
func Test_Proxy(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Balancer_WithTlsConfig
func Test_Proxy_Balancer_WithTlsConfig(t *testing.T) {
	t.Parallel()

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()

	app.Get("/tlsbalancer", func(c fiber.Ctx) error {
		return c.SendString("tls balancer")
	})

	addr := ln.Addr().String()
	clientTLSConf := &tls.Config{InsecureSkipVerify: true}

	// disable certificate verification in Balancer
	app.Use(Balancer(Config{
		Servers:   []string{addr},
		TLSConfig: clientTLSConf,
	}))

	startServer(app, ln)

	client := clientpkg.New()
	client.SetTLSConfig(clientTLSConf)

	resp, err := client.Get("https://" + addr + "/tlsbalancer")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "tls balancer", string(resp.Body()))
	resp.Close()
}

// go test -run Test_Proxy_Balancer_IPv6_Upstream
func Test_Proxy_Balancer_IPv6_Upstream(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv6(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// go test -run Test_Proxy_Balancer_IPv6_Upstream
func Test_Proxy_Balancer_IPv6_Upstream_With_DialDualStack(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv6(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(Balancer(Config{
		Servers:       []string{addr},
		DialDualStack: true,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Balancer_IPv6_Upstream
func Test_Proxy_Balancer_IPv4_Upstream_With_DialDualStack(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(Balancer(Config{
		Servers:       []string{addr},
		DialDualStack: true,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Forward_WithTlsConfig_To_Http
func Test_Proxy_Forward_WithTlsConfig_To_Http(t *testing.T) {
	t.Parallel()

	_, targetAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("hello from target")
	})

	proxyServerTLSConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	proxyServerLn, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	proxyServerLn = tls.NewListener(proxyServerLn, proxyServerTLSConf)
	proxyAddr := proxyServerLn.Addr().String()

	app := fiber.New()
	app.Use(Forward("http://" + targetAddr))
	startServer(app, proxyServerLn)

	client := clientpkg.New()
	client.SetTimeout(5 * time.Second)
	client.TLSConfig().InsecureSkipVerify = true

	resp, err := client.Get("https://" + proxyAddr)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "hello from target", string(resp.Body()))
	resp.Close()
}

// go test -run Test_Proxy_Forward
func Test_Proxy_Forward(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app.Use(Forward("http://" + addr))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "forwarded", string(b))
}

// go test -run Test_Proxy_Forward_ReplacesClientSuppliedRealIP
func Test_Proxy_Forward_ReplacesClientSuppliedRealIP(t *testing.T) {
	t.Parallel()

	// The forwarders overwrite X-Real-IP so the upstream can attribute the
	// request to the peer Fiber actually saw. Header.Set replaces the first
	// field line with a given name and leaves any others in place, so a client
	// that sends the header twice used to keep one of its own values on the
	// wire — and the upstream, which may read the last line or join the pair
	// per RFC 9110 Section 5.2, would attribute the request to an address the
	// client chose.
	for _, tc := range []struct {
		handler func(addr string) fiber.Handler
		name    string
	}{
		{
			name:    "Forward",
			handler: func(addr string) fiber.Handler { return Forward("http://" + addr) },
		},
		{
			name: "DomainForward",
			handler: func(addr string) fiber.Handler {
				return DomainForward("example.com", "http://"+addr)
			},
		},
		{
			name: "BalancerForward",
			handler: func(addr string) fiber.Handler {
				return BalancerForward([]string{"http://" + addr})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				seen := c.Request().Header.PeekAll("X-Real-IP")
				out := make([]string, 0, len(seen))
				for _, v := range seen {
					out = append(out, string(v))
				}
				return c.SendString(strings.Join(out, "|"))
			})

			app := fiber.New()
			app.Use(tc.handler(addr))

			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Host = "example.com"
			req.Header.Add("X-Real-IP", "7.7.7.7")
			req.Header.Add("X-Real-IP", "8.8.8.8")

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			got := string(b)
			require.NotContains(t, got, "7.7.7.7", "client-supplied X-Real-IP reached the upstream")
			require.NotContains(t, got, "8.8.8.8", "client-supplied X-Real-IP reached the upstream")
			require.NotContains(t, got, "|", "more than one X-Real-IP field line reached the upstream")
			require.NotEmpty(t, got)
		})
	}
}

// Test_Proxy_Forward_RealIPFromProxyHeader pins that replacing X-Real-IP does
// not destroy the value it is derived from.
//
// With Config.ProxyHeader set to "X-Real-IP", c.IP() reads that very header, so
// deleting it before resolving the address left the upstream with an empty
// X-Real-IP instead of the client's.
func Test_Proxy_Forward_RealIPFromProxyHeader(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen := c.Request().Header.PeekAll("X-Real-IP")
		out := make([]string, 0, len(seen))
		for _, v := range seen {
			out = append(out, string(v))
		}
		return c.SendString(strings.Join(out, "|"))
	})

	app := fiber.New(fiber.Config{
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0/0"}},
		ProxyHeader:      "X-Real-IP",
	})
	app.Use(Forward("http://" + addr))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Real-IP", "203.0.113.9")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.9", string(b))
}

// go test -run Test_Proxy_Forward_WithClient_TLSConfig
func Test_Proxy_Forward_WithClient_TLSConfig(t *testing.T) {
	restoreGlobalProxyClient(t)

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()

	app.Get("/tlsfwd", func(c fiber.Ctx) error {
		return c.SendString("tls forward")
	})

	addr := ln.Addr().String()
	clientTLSConf := &tls.Config{InsecureSkipVerify: true}

	// disable certificate verification
	WithClient(&fasthttp.Client{
		TLSConfig: clientTLSConf,
	})
	app.Use(Forward("https://" + addr + "/tlsfwd"))

	startServer(app, ln)

	client := clientpkg.New()
	client.SetTLSConfig(clientTLSConf)

	resp, err := client.Get("https://" + addr)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "tls forward", string(resp.Body()))
	resp.Close()
}

// go test -run Test_Proxy_Modify_Response
func Test_Proxy_Modify_Response(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.Status(500).SendString("not modified")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyResponse: func(c fiber.Ctx) error {
			c.Response().SetStatusCode(fiber.StatusOK)
			return c.SendString("modified response")
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "modified response", string(b))
}

// go test -run Test_Proxy_Modify_Request
func Test_Proxy_Modify_Request(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		b := c.Request().Body()
		return c.SendString(string(b))
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(c fiber.Ctx) error {
			c.Request().SetBody([]byte("modified request"))
			return nil
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "modified request", string(b))
}

// go test -run Test_Proxy_Timeout_Slow_Server
func Test_Proxy_Timeout_Slow_Server(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(300 * time.Millisecond)
		return c.SendString("fiber is awesome")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		Timeout: 600 * time.Millisecond,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "fiber is awesome", string(b))
}

// go test -run Test_Proxy_With_Timeout
func Test_Proxy_With_Timeout(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(1 * time.Second)
		return c.SendString("fiber is awesome")
	})

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		Timeout: 100 * time.Millisecond,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, fasthttp.ErrTimeout.Error(), string(b))
}

// go test -run Test_Proxy_Buffer_Size_Response
func Test_Proxy_Buffer_Size_Response(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		long := strings.Join(make([]string, 5000), "-")
		c.Set("Very-Long-Header", long)
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, KeepConnectionHeader: true}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	app = fiber.New()
	app.Use(Balancer(Config{
		Servers:        []string{addr},
		ReadBufferSize: 1024 * 8,
	}))

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -race -run Test_Proxy_Do_RestoreOriginalURL
func Test_Proxy_Do_RestoreOriginalURL(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "http://"+addr)
	})
	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
	require.NoError(t, err1)
	require.Equal(t, "/test", resp.Request.URL.String())
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxied", string(body))
}

// go test -race -run Test_Proxy_Do_WithRealURL
func Test_Proxy_Do_WithRealURL(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("real url")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "http://"+addr)
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err1)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "real url", string(body))
}

// go test -race -run Test_Proxy_Do_WithRedirect
func Test_Proxy_Do_WithRedirect(t *testing.T) {
	t.Parallel()

	addr := createRedirectServer(t)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "http://"+addr)
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err1)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "redirect", string(body))
	require.Equal(t, fiber.StatusMovedPermanently, resp.StatusCode)
}

// go test -race -run Test_Proxy_DoRedirects_RestoreOriginalURL
// Test_Proxy_DoRedirects_AliasedAddr regression-tests P3+P6: the addr
// string is derived from c.OriginalURL() and therefore aliases the
// fasthttp request's internal requestURI buffer. After P6, the *url.URL
// returned by validateUpstream is passed into followRedirects, which
// reads u.Host on the cross-host strip check and u.String() to feed
// SetRequestURI. If u's field slices were ever aliased back to the
// request buffer, SetRequestURI's overwrite would corrupt them mid-loop
// and the redirect target would be garbled. P3 fixes this by cloning
// addr once at the parse boundary inside doActionWithPolicy.
func Test_Proxy_DoRedirects_AliasedAddr(t *testing.T) {
	t.Parallel()

	addr := createRedirectServer(t)
	app := fiber.New()
	// Routing the upstream through the request path forces addr to alias
	// req.Header.requestURI, exercising the aliasing case directly.
	app.Get("/*", func(c fiber.Ctx) error {
		aliased := strings.TrimPrefix(c.OriginalURL(), "/")
		return DoRedirects(c, aliased, 1)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/http://"+addr, http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "final", string(body))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Proxy_DoRedirects_RestoreOriginalURL(t *testing.T) {
	t.Parallel()

	addr := createRedirectServer(t)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoRedirects(c, "http://"+addr, 1)
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err1)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "final", string(body))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

// go test -race -run Test_Proxy_DoRedirects_TooManyRedirects
func Test_Proxy_DoRedirects_TooManyRedirects(t *testing.T) {
	t.Parallel()

	addr := createRedirectServer(t)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoRedirects(c, "http://"+addr, 0)
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err1)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, fasthttp.ErrTooManyRedirects.Error(), string(body))
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

// go test -race -run Test_Proxy_DoTimeout_RestoreOriginalURL
func Test_Proxy_DoTimeout_RestoreOriginalURL(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoTimeout(c, "http://"+addr, time.Second)
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err1)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxied", string(body))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

// go test -race -run Test_Proxy_DoTimeout_Timeout
func Test_Proxy_DoTimeout_Timeout(t *testing.T) {
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(time.Second * 5)
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoTimeout(c, "http://"+addr, time.Second)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, fasthttp.ErrTimeout.Error(), string(body))
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

// go test -race -run Test_Proxy_DoDeadline_RestoreOriginalURL
func Test_Proxy_DoDeadline_RestoreOriginalURL(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoDeadline(c, "http://"+addr, time.Now().Add(time.Second))
	})

	resp, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
	require.NoError(t, err1)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxied", string(body))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

// go test -race -run Test_Proxy_DoDeadline_PastDeadline
func Test_Proxy_DoDeadline_PastDeadline(t *testing.T) {
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(time.Second * 5)
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return DoDeadline(c, "http://"+addr, time.Now().Add(2*time.Second))
	})

	_, err1 := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       1 * time.Second,
		FailOnTimeout: true,
	})
	require.Equal(t, os.ErrDeadlineExceeded, err1)
}

// go test -race -run Test_Proxy_Do_HTTP_Prefix_URL
func Test_Proxy_Do_HTTP_Prefix_URL(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	app := fiber.New()
	app.Get("/*", func(c fiber.Ctx) error {
		path := c.OriginalURL()
		url := strings.TrimPrefix(path, "/")

		require.Equal(t, "http://"+addr, url)
		if err := Do(c, url); err != nil {
			return err
		}
		c.Response().Header.Del(fiber.HeaderServer)
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/http://"+addr, http.NoBody))
	require.NoError(t, err)
	s, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(s))
}

// go test -race -run Test_Proxy_Forward_Global_Client
func Test_Proxy_Forward_Global_Client(t *testing.T) {
	restoreGlobalProxyClient(t)
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	WithClient(&fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		MaxConnsPerHost:          123,
	})
	loadedClient := client.Load()
	require.NotNil(t, loadedClient)
	require.Equal(t, 123, loadedClient.MaxConnsPerHost)

	app := fiber.New()
	app.Get("/test_global_client", func(c fiber.Ctx) error {
		return c.SendString("test_global_client")
	})

	addr := ln.Addr().String()
	app.Use(Forward("http://" + addr + "/test_global_client"))
	startServer(app, ln)

	client := clientpkg.New()
	resp, err := client.Get("http://" + addr)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "test_global_client", string(resp.Body()))
	resp.Close()
}

// go test -race -run Test_Proxy_Forward_Local_Client
func Test_Proxy_Forward_Local_Client(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	app := fiber.New()
	app.Get("/test_local_client", func(c fiber.Ctx) error {
		return c.SendString("test_local_client")
	})

	addr := ln.Addr().String()
	app.Use(Forward("http://"+addr+"/test_local_client", &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,

		Dial: fasthttp.Dial,
	}))
	startServer(app, ln)

	client := clientpkg.New()
	resp, err := client.Get("http://" + addr)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "test_local_client", string(resp.Body()))
	resp.Close()
}

// go test -run Test_Proxy_WithClient_Nil_Panics
func Test_Proxy_WithClient_Nil_Panics(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "proxy: WithClient requires a non-nil *fasthttp.Client", func() {
		WithClient(nil)
	})
}

// go test -run Test_Proxy_Do_NilClientOverride
func Test_Proxy_Do_NilClientOverride(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "http://"+addr, nil)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, errNilProxyClientOverride.Error(), string(body))
}

// go test -run Test_Proxy_Do_NonNilClientOverride
func Test_Proxy_Do_NonNilClientOverride(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		return Do(c, "http://"+addr, &fasthttp.Client{
			NoDefaultUserAgentHeader: true,
			DisablePathNormalizing:   true,
		})
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxied", string(body))
}

// go test -run Test_Proxy_SelectClient_NilGlobal
func Test_Proxy_SelectClient_NilGlobal(t *testing.T) {
	t.Parallel()

	selectedClient, err := selectClient(nil)
	require.ErrorIs(t, err, errNilGlobalProxyClient)
	require.Nil(t, selectedClient)
}

// go test -run Test_Proxy_NilClientOverride_AcrossHelpers
func Test_Proxy_NilClientOverride_AcrossHelpers(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	tests := map[string]func(c fiber.Ctx) error{
		"DoRedirects": func(c fiber.Ctx) error {
			return DoRedirects(c, "http://"+addr, 1, nil)
		},
		"DoDeadline": func(c fiber.Ctx) error {
			return DoDeadline(c, "http://"+addr, time.Now().Add(time.Second), nil)
		},
		"DoTimeout": func(c fiber.Ctx) error {
			return DoTimeout(c, "http://"+addr, time.Second, nil)
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			app.Get("/test", run)

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, errNilProxyClientOverride.Error(), string(body))
		})
	}
}

// go test -run Test_ProxyBalancer_Custom_Client
func Test_ProxyBalancer_Custom_Client(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New()

	app.Use(Balancer(Config{Client: &fasthttp.LBClient{
		Clients: []fasthttp.BalancingClient{
			&fasthttp.HostClient{
				NoDefaultUserAgentHeader: true,
				DisablePathNormalizing:   true,
				Addr:                     addr,
			},
		},
		Timeout: time.Second,
	}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

// go test -run Test_Proxy_Domain_Forward_Local
func Test_Proxy_Domain_Forward_Local(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	app := fiber.New()

	// target server
	ln1, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	app1 := fiber.New()

	app1.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("test_local_client:" + c.Query("query_test"))
	})

	proxyAddr := ln.Addr().String()
	targetAddr := ln1.Addr().String()
	localDomain := strings.Replace(proxyAddr, "127.0.0.1", "localhost", 1)
	app.Use(DomainForward(localDomain, "http://"+targetAddr, &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,

		Dial: fasthttp.Dial,
	}))
	startServer(app, ln)
	startServer(app1, ln1)

	client := clientpkg.New()
	resp, err := client.Get("http://" + localDomain + "/test?query_test=true")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "test_local_client:true", string(resp.Body()))
	resp.Close()
}

// go test -run Test_Proxy_Balancer_Forward_Local
func Test_Proxy_Balancer_Forward_Local(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app.Use(BalancerForward([]string{addr}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, "forwarded", string(b))
}

// go test -run Test_Proxy_Balancer_Forward_Empty_Servers
func Test_Proxy_Balancer_Forward_Empty_Servers(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "Servers cannot be empty", func() {
		BalancerForward([]string{})
	})
}

func Test_Proxy_Balancer_Forward_OverwritesXRealIP(t *testing.T) {
	t.Parallel()

	const (
		spoofedIP       = "10.0.0.1"
		appTestClientIP = "0.0.0.0"
	)

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		value := c.Get("X-Real-IP")
		require.Equal(t, appTestClientIP, value)
		require.NotEqual(t, spoofedIP, value)
		return c.SendStatus(fiber.StatusOK)
	})

	app := fiber.New()
	app.Use(BalancerForward([]string{addr}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Real-IP", spoofedIP)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Proxy_Immutable(t *testing.T) {
	t.Parallel()

	target, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	resp, err := target.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody), fiber.TestConfig{
		Timeout:       2 * time.Second,
		FailOnTimeout: true,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

	app := fiber.New(fiber.Config{Immutable: true})

	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}

func Test_Proxy_KeepConnectionHeader(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderConnection, "backend")
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, KeepConnectionHeader: true}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	req.Header.Set(fiber.HeaderConnection, "keep-alive")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "backend", resp.Header.Get(fiber.HeaderConnection))
}

func Test_Proxy_DropConnectionHeader(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderConnection, "backend")
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = addr
	req.Header.Set(fiber.HeaderConnection, "keep-alive")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Empty(t, resp.Header.Get(fiber.HeaderConnection))
}

func Test_Proxy_Forward_OverwritesXRealIP(t *testing.T) {
	t.Parallel()

	const spoofedIP = "10.0.0.1"
	// app.Test injects 0.0.0.0 as the remote address, so c.IP() returns IPv4zero.
	appTestClientIP := net.IPv4zero.String()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		require.Equal(t, appTestClientIP, c.Get("X-Real-IP"))
		return c.SendStatus(fiber.StatusOK)
	})

	app := fiber.New()
	app.Use(Forward("http://" + addr))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Real-IP", spoofedIP)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Proxy_DomainForward_OverwritesXRealIP(t *testing.T) {
	t.Parallel()

	const (
		spoofedIP    = "10.0.0.1"
		testHostname = "example.com"
	)
	// app.Test injects 0.0.0.0 as the remote address, so c.IP() returns IPv4zero.
	appTestClientIP := net.IPv4zero.String()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		require.Equal(t, appTestClientIP, c.Get("X-Real-IP"))
		return c.SendStatus(fiber.StatusOK)
	})

	app := fiber.New()
	app.Use(DomainForward(testHostname, "http://"+addr))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = testHostname
	req.Header.Set("X-Real-IP", spoofedIP)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_Proxy_DomainForward_HostMatchIsCaseInsensitive verifies that the
// host gate folds case per RFC 9110 §4.2.3 — an inbound Host header with
// different casing than the configured hostname must still be proxied,
// not silently passed through.
func Test_Proxy_DomainForward_HostMatchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("proxied")
	})

	app := fiber.New()
	// Handler configured with a lowercase hostname...
	app.Use(DomainForward("api.example.com", "http://"+addr))

	// ...but the request arrives with mixed-case Host.
	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "API.Example.com"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "proxied", string(body), "mixed-case Host must still be proxied")
}

// sendRawUnnormalized drives one request whose header names are kept exactly as
// written, the way a front end translating HTTP/2 down to HTTP/1.1 leaves them,
// and returns the response body.
func sendRawUnnormalized(t *testing.T, app *fiber.App, raw string) string {
	t.Helper()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.DisableNormalizing()
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, nil, nil)
	app.Handler()(fctx)

	return string(fctx.Response.Body())
}

// loopbackAlias returns addr with its host rewritten to "localhost", naming the
// same listener under a different authority.
//
// A cross-host redirect is decided by comparing the authority as written, not
// by where it resolves, so this is enough to make the hop cross an origin
// without binding a second loopback address.
func loopbackAlias(t *testing.T, addr string) string {
	t.Helper()

	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	return net.JoinHostPort("localhost", port)
}

// matchingFieldLines reports every field line whose name matches one of names,
// in the spelling the header store holds.
func matchingFieldLines(c fiber.Ctx, names ...string) []string {
	var found []string
	for k, v := range c.Request().Header.All() {
		for _, name := range names {
			if utils.EqualFold(utils.UnsafeString(k), name) {
				found = append(found, string(k)+"="+string(v))
			}
		}
	}
	return found
}

// Test_Proxy_HopByHopHeadersStrippedWhateverTheirCase checks that the RFC 7230
// Section 6.1 strip holds for a peer that spells the names in lower case, which
// is what HTTP/2 and HTTP/3 require on the wire.
//
// Del matches the stored key byte for byte, so under DisableHeaderNormalizing
// it removed nothing: the hop-by-hop headers reached the upstream, and so did a
// field named in Connection, which is a peer smuggling a connection-scoped
// header through an intermediary required to drop it.
func Test_Proxy_HopByHopHeadersStrippedWhateverTheirCase(t *testing.T) {
	t.Parallel()

	seen := make(chan []string, 1)
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen <- matchingFieldLines(c, "Upgrade", "TE", "Keep-Alive", "Connection", "X-Smuggled")
		return c.SendString("upstream")
	})

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(Forward("http://" + addr))

	body := sendRawUnnormalized(t, app, "GET / HTTP/1.1\r\nHost: front\r\n"+
		"connection: x-smuggled\r\n"+
		"x-smuggled: through-the-proxy\r\n"+
		"upgrade: websocket\r\n"+
		"te: trailers\r\n"+
		"keep-alive: timeout=5\r\n\r\n")
	require.Equal(t, "upstream", body)

	require.Empty(t, <-seen, "no hop-by-hop or Connection-listed header may reach the upstream")
}

// Test_Proxy_RealIPReplacedWhateverTheirCase checks that the client cannot keep
// a claim of its own beside the address this hop writes.
func Test_Proxy_RealIPReplacedWhateverTheirCase(t *testing.T) {
	t.Parallel()

	seen := make(chan []string, 1)
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen <- matchingFieldLines(c, realIPHeader)
		return c.SendString("upstream")
	})

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(Forward("http://" + addr))

	body := sendRawUnnormalized(t, app, "GET / HTTP/1.1\r\nHost: front\r\n"+
		"x-real-ip: 6.6.6.6\r\n"+
		"X-Real-IP: 7.7.7.7\r\n\r\n")
	require.Equal(t, "upstream", body)

	lines := <-seen
	require.Len(t, lines, 1, "exactly one X-Real-IP reaches the upstream: %v", lines)
	require.NotContains(t, lines[0], "6.6.6.6")
	require.NotContains(t, lines[0], "7.7.7.7")
}

// Test_Proxy_CrossHostCredentialsStrippedWhateverTheirCase checks that a
// redirect leaving the host the caller addressed does not carry the caller's
// credentials to wherever it points.
func Test_Proxy_CrossHostCredentialsStrippedWhateverTheirCase(t *testing.T) {
	t.Parallel()

	seen := make(chan []string, 1)
	_, finalAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen <- matchingFieldLines(c, fiber.HeaderAuthorization, fiber.HeaderCookie)
		return c.SendString("final")
	})

	// The redirect has to cross to a different host, and that is decided by
	// comparing the authority as written rather than where it resolves — so
	// name the same listener "localhost" instead of binding a second loopback
	// address. 127.0.0.2 is bound by default only on Linux.
	_, redirectAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Location("http://" + loopbackAlias(t, finalAddr) + "/")
		return c.SendStatus(fiber.StatusFound)
	})

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(func(c fiber.Ctx) error {
		return DoRedirects(c, "http://"+redirectAddr, 3)
	})

	body := sendRawUnnormalized(t, app, "GET / HTTP/1.1\r\nHost: front\r\n"+
		"authorization: Bearer caller-token\r\n"+
		"cookie: session=caller\r\n\r\n")
	require.Equal(t, "final", body)

	require.Empty(t, <-seen, "credentials must not follow a redirect off the host they were sent to")
}

// Test_Proxy_CrossHostStripsEveryCredentialHeader checks that the set dropped
// on a redirect off the addressed host is the one the client package drops:
// Cookie2 carries a session exactly as Cookie does, and a list that covers one
// but not the other leaks through the gap.
func Test_Proxy_CrossHostStripsEveryCredentialHeader(t *testing.T) {
	t.Parallel()

	seen := make(chan []string, 1)
	_, finalAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		seen <- matchingFieldLines(c,
			fiber.HeaderAuthorization,
			fiber.HeaderProxyAuthorization,
			fiber.HeaderProxyAuthenticate,
			fiber.HeaderWWWAuthenticate,
			fiber.HeaderCookie,
			"Cookie2",
		)
		return c.SendString("final")
	})

	_, redirectAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Location("http://" + loopbackAlias(t, finalAddr) + "/")
		return c.SendStatus(fiber.StatusFound)
	})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		return DoRedirects(c, "http://"+redirectAddr, 3)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer caller-token")
	req.Header.Set(fiber.HeaderProxyAuthorization, "Basic proxy")
	req.Header.Set(fiber.HeaderProxyAuthenticate, "Basic realm=x")
	req.Header.Set(fiber.HeaderWWWAuthenticate, "Basic realm=y")
	req.Header.Set(fiber.HeaderCookie, "session=caller")
	req.Header.Set("Cookie2", `$Version="1"`)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.Empty(t, <-seen, "no credential header may follow a redirect off the addressed host")
}

// Test_Proxy_ResponseStripIgnoresTheAppsNormalizationSetting checks the
// response-side hop-by-hop strip against a proxy whose outbound client
// preserves upstream header casing while the app itself normalizes.
//
// The two settings are independent — the client stamps its own onto the
// response header on every hop — so reading the app's told the strip nothing
// about the keys it was about to match, and it removed nothing: the upstream's
// connection-scoped headers reached the client unchanged.
func Test_Proxy_ResponseStripIgnoresTheAppsNormalizationSetting(t *testing.T) {
	t.Parallel()

	// The upstream preserves the casing it writes, and writes its hop-by-hop
	// headers in lower case.
	upstream := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	upstream.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Set("keep-alive", "timeout=5")
		c.Response().Header.Set("upgrade", "websocket")
		c.Response().Header.Set("te", "trailers")
		return c.SendString("upstream")
	})
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	startServer(upstream, ln)

	cli := &fasthttp.Client{DisableHeaderNamesNormalizing: true}

	// ...while the app takes the default, so the two settings disagree.
	app := fiber.New()
	app.Use(Forward("http://"+addr, cli))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	for _, name := range []string{fiber.HeaderKeepAlive, fiber.HeaderUpgrade, fiber.HeaderTE} {
		require.Empty(t, resp.Header.Values(name),
			"%s must not reach the client whatever case the upstream wrote it in", name)
	}
}

func Test_Proxy_DomainForward_NonMatchingHostContinues(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app := fiber.New()
	app.Use(DomainForward("api.example.com", "http://"+addr))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("local")
	})

	testCases := []struct {
		host string
		body string
	}{
		{host: "api.example.com", body: "forwarded"},
		{host: "API.Example.com:8080", body: "forwarded"},
		{host: "www.example.com", body: "local"},
		{host: "www.example.com:8080", body: "local"},
	}
	for _, tc := range testCases {
		req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
		req.Host = tc.host
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, "Host %s", tc.host)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, tc.body, string(body), "Host %s", tc.host)
	}
}

func Test_Proxy_Forward_RestoresHost(t *testing.T) {
	t.Parallel()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString("forwarded")
	})

	app := fiber.New()
	seen := make(chan string, 1)
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		seen <- c.Hostname()
		return err
	})
	app.Use(Forward("http://" + addr))

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Host = "public.example.com"
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "public.example.com", <-seen)
}

func Test_Proxy_HostWithoutPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, out string
	}{
		{in: "example.com", out: "example.com"},
		{in: "example.com:8080", out: "example.com"},
		{in: "[::1]:8080", out: "[::1]"},
		{in: "[::1]", out: "[::1]"},
		{in: "[::1", out: "[::1"},
		{in: "::1", out: "::1"},
	}

	for _, tc := range tests {
		require.Equal(t, tc.out, hostWithoutPort(tc.in), "in=%q", tc.in)
	}
}
