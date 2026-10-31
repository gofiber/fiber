package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	clientpkg "github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/valyala/fasthttp"
)

// TestMain relaxes the proxy security policy for the whole suite so the
// existing loopback-based tests continue to work. Tests that exercise
// the secure defaults install their own policy in their own scope.
func TestMain(m *testing.M) {
	WithSecurityPolicy(proxyTestAllowPrivatePolicy())
	m.Run()
}

func startServer(app *fiber.App, ln net.Listener) {
	go func() {
		err := app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
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

// proxyTestListen returns a loopback TCP listener on a free port, for a server the test starts.
func proxyTestListen(t *testing.T) net.Listener {
	t.Helper()

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	return ln
}

func createRedirectServer(t *testing.T) string {
	t.Helper()
	app := fiber.New()

	var addr string
	app.Get("/", func(c fiber.Ctx) error {
		c.Location("http://" + addr + "/final")
		return c.Status(fiber.StatusMovedPermanently).SendString("redirect")
	})
	app.Get("/final", func(c fiber.Ctx) error { return c.SendString("final") })

	ln := proxyTestListen(t)
	t.Cleanup(func() {
		ln.Close() //nolint:errcheck // It is fine to ignore the error here
	})
	addr = ln.Addr().String()

	startServer(app, ln)

	return addr
}

func restoreGlobalProxyClient(tb testing.TB) {
	tb.Helper()

	prev := sharedClient.Load()
	tb.Cleanup(func() { WithClient(prev) })
}

// proxyTestAllowPrivatePolicy returns the default security policy with private
// upstream addresses allowed, which the loopback servers of the tests need.
func proxyTestAllowPrivatePolicy() SecurityPolicy {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	return policy
}

// proxyTestTLSListener returns a loopback TCP listener that speaks TLS with the
// test certificate, to serve an app behind.
func proxyTestTLSListener(t *testing.T) net.Listener {
	t.Helper()

	tlsConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)
	return tls.NewListener(proxyTestListen(t), tlsConf)
}

// proxyTestTLSClient returns a client that reaches a server presenting the test
// certificate, which it does not verify.
func proxyTestTLSClient() *clientpkg.Client {
	client := clientpkg.New()
	client.SetTimeout(5 * time.Second)
	client.TLSConfig().InsecureSkipVerify = true
	return client
}

// proxyTestExpectGet gets target with client and requires the status 200 and
// the given body.
func proxyTestExpectGet(t *testing.T, client *clientpkg.Client, target, body string) {
	t.Helper()

	resp, err := client.Get(target)
	require.NoError(t, err)
	defer resp.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode(), string(resp.Body()))
	require.Equal(t, body, string(resp.Body()))
}

// proxyTestExpectGetOverTLS serves app behind a TLS listener and requires a GET of it, from a client
// that trusts the test certificate, to answer 200 with the given body.
func proxyTestExpectGetOverTLS(t *testing.T, app *fiber.App, body string) {
	t.Helper()

	ln := proxyTestTLSListener(t)
	startServer(app, ln)
	proxyTestExpectGet(t, proxyTestTLSClient(), "https://"+ln.Addr().String(), body)
}

// proxyTestRedirectUpstream starts an upstream that answers every request with a 302 to location and
// returns its address.
func proxyTestRedirectUpstream(t *testing.T, location string) string {
	t.Helper()

	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		c.Location(location)
		return c.SendStatus(fiber.StatusFound)
	})
	return addr
}

// proxyTestFieldLinesUpstream starts an upstream that answers body and hands
// the field lines of the request that match names (see matchingFieldLines) to
// the returned channel, which holds the one request a test sends it.
func proxyTestFieldLinesUpstream(t *testing.T, body string, names ...string) (addr string, seen <-chan []string) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming the returned address and channel for readability
	t.Helper()

	lines := make(chan []string, 1)
	_, addr = createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		lines <- matchingFieldLines(c, names...)
		return c.SendString(body)
	})
	return addr, lines
}

// rawGet starts the request of a client that GETs "/" from example.com, which the header lines of a test follow.
const rawGet = "GET / HTTP/1.1\r\nHost: example.com\r\n"

// connectionNames returns a Connection value of n names.
func connectionNames(n int) string { return utils.TrimRight(strings.Repeat("a,", n), ',') }

func Test_Proxy_DefaultClient_MaxConnsPerHost(t *testing.T) {
	t.Parallel()
	require.Equal(t, defaultMaxConnsPerHost, sharedClient.Load().MaxConnsPerHost)
}

// Test_Proxy_Balancer_LeavesConfiguredClientAlone pins that Balancer does not write to a host client
// that already has DisablePathNormalizing: another goroutine may be reading it (run under -race).
func Test_Proxy_Balancer_LeavesConfiguredClientAlone(t *testing.T) {
	t.Parallel()
	hc := &fasthttp.HostClient{Addr: "127.0.0.1:1", DisablePathNormalizing: true}
	var reads atomic.Int64
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Go(func() {
		for !stop.Load() {
			if hc.DisablePathNormalizing {
				reads.Add(1)
			}
		}
	})
	t.Cleanup(func() {
		stop.Store(true)
		wg.Wait()
	})
	readsMoreThan := func(n int64) func() bool { return func() bool { return reads.Load() > n } }

	require.Eventually(t, readsMoreThan(0), 5*time.Second, time.Millisecond, "the reader is running before the build")

	Balancer(Config{Client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hc}}})

	require.Eventually(t, readsMoreThan(reads.Load()+1000), 5*time.Second, time.Millisecond, "and is still reading after it")
}

func Test_Proxy_ConfigDefault_MaxConnsPerHost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		set  int // Config.MaxConnsPerHost, 0 to leave it unset
		want int
	}{
		{name: "default", want: defaultMaxConnsPerHost},
		{name: "override", set: 2048, want: 2048},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := configDefault(Config{Servers: []string{"127.0.0.1"}, MaxConnsPerHost: tc.set})
			require.Equal(t, tc.want, cfg.MaxConnsPerHost)
		})
	}
}

// Without a server to forward to there is nothing a request could be sent to, so
// the constructors stop at startup.
func Test_Proxy_EmptyServersPanic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		build func()
		name  string
	}{
		{name: "Balancer with empty Servers", build: func() { Balancer(Config{Servers: []string{}}) }},
		{name: "Balancer with empty Config", build: func() { Balancer(Config{}) }},
		// No default upstream exists, so Balancer() stops at startup, as Balancer(Config{}) does.
		{name: "Balancer without arguments", build: func() { Balancer() }},
		{name: "BalancerForward with empty servers", build: func() { BalancerForward([]string{}) }},
		// Neither Servers nor a Client override is supplied.
		{name: "configDefault with neither Servers nor Client", build: func() { configDefault(Config{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, "Servers cannot be empty", tc.build)
		})
	}
}

func Test_Proxy_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{"127.0.0.1"}, Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

// Each row starts an upstream that answers 418, checks that it does, and has a
// Balancer reach it from an app of its own.
func Test_Proxy_Balancer_Upstream(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		server    func(*testing.T, fiber.Handler) (*fiber.App, string)
		client    func(addr string) *fasthttp.LBClient // the Balancer's Client, which replaces Servers, when set
		name      string
		appCfg    []fiber.Config
		status    int
		dualStack bool // Config.DialDualStack
	}{
		{name: "IPv4", server: createProxyTestServerIPv4, status: fiber.StatusTeapot},
		{name: "IPv6", server: createProxyTestServerIPv6, status: fiber.StatusInternalServerError},
		{
			name: "IPv6 with DialDualStack", server: createProxyTestServerIPv6, dualStack: true,
			status: fiber.StatusTeapot,
		},
		{
			name: "IPv4 with DialDualStack", server: createProxyTestServerIPv4, dualStack: true,
			status: fiber.StatusTeapot,
		},
		{
			name:   "custom LBClient",
			server: createProxyTestServerIPv4,
			client: func(addr string) *fasthttp.LBClient {
				return &fasthttp.LBClient{
					Clients: []fasthttp.BalancingClient{
						&fasthttp.HostClient{NoDefaultUserAgentHeader: true, DisablePathNormalizing: true, Addr: addr},
					},
					Timeout: time.Second,
				}
			},
			status: fiber.StatusTeapot,
		},
		{
			name: "immutable app", server: createProxyTestServerIPv4, appCfg: []fiber.Config{{Immutable: true}},
			status: fiber.StatusTeapot,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target, addr := tc.server(t, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

			resp := apptest.Do(t, target, "/", fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true})
			require.Equal(t, fiber.StatusTeapot, resp.StatusCode)

			cfg := Config{Servers: []string{addr}, DialDualStack: tc.dualStack}
			if tc.client != nil {
				cfg = Config{Client: tc.client(addr)}
			}
			app := fiber.New(tc.appCfg...)
			app.Use(Balancer(cfg))

			apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr)), tc.status)
		})
	}
}

// The balancer does not speak TLS itself, but the request must still reach the upstream as https.
func Test_Proxy_Balancer_WithTlsConfig(t *testing.T) {
	t.Parallel()
	ln := proxyTestTLSListener(t)

	upstream := fiber.New()
	upstream.Get("/tlsbalancer", func(c fiber.Ctx) error { return c.SendString("tls balancer") })
	startServer(upstream, ln)

	app := fiber.New()
	// disable certificate verification in Balancer
	app.Use(Balancer(Config{
		Servers:   []string{"https://" + ln.Addr().String()},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}))

	resp := apptest.Do(t, app, "/tlsbalancer", fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true})
	body := apptest.Body(t, resp)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, body)
	require.Equal(t, "tls balancer", body)
}

// The other way round: the balancer sits behind a TLS listener and its upstream is plain HTTP.
func Test_Proxy_Balancer_PlainUpstreamBehindTLS(t *testing.T) {
	t.Parallel()
	_, upstreamAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("plain upstream") })

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{"http://" + upstreamAddr}}))

	proxyTestExpectGetOverTLS(t, app, "plain upstream")
}

// Each upstream of a mixed list is reached with its own scheme, whichever one the request arrived on.
func Test_Proxy_Balancer_MixedSchemes(t *testing.T) {
	t.Parallel()
	_, plainAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("plain") })
	ln := proxyTestTLSListener(t)
	secure := fiber.New()
	secure.Get("/", func(c fiber.Ctx) error { return c.SendString("secure") })
	startServer(secure, ln)

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers:   []string{"http://" + plainAddr, "https://" + ln.Addr().String()},
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	}))

	seen := map[string]int{}
	for range 6 {
		resp := apptest.Do(t, app, "/", fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true})
		body := apptest.Body(t, resp)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, body)
		seen[body]++
	}
	require.Equal(t, map[string]int{"plain": 3, "secure": 3}, seen)
}

func Test_Proxy_Balancer_EmptyClientPanicsAtStartup(t *testing.T) {
	t.Parallel()

	// fasthttp would panic on the first request, and nothing above the handler recovers it.
	require.PanicsWithValue(t, "Client.Clients cannot be empty", func() {
		Balancer(Config{Client: &fasthttp.LBClient{}})
	})
}

func Test_Proxy_Forward_WithTlsConfig_To_Http(t *testing.T) {
	t.Parallel()
	_, targetAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("hello from target") })

	app := fiber.New()
	app.Use(Forward("http://" + targetAddr))

	proxyTestExpectGetOverTLS(t, app, "hello from target")
}

// Test_Proxy_DoRedirects_AppliesSchemePerHopBehindTLS pins that every hop is sent with the scheme of
// its own URL: a copied hop would keep the TLS mark of the gateway's request and speak TLS to plain HTTP.
func Test_Proxy_DoRedirects_AppliesSchemePerHopBehindTLS(t *testing.T) {
	t.Parallel()
	_, finalAddr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("final") })
	redirectAddr := proxyTestRedirectUpstream(t, "http://"+finalAddr+"/")

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return DoRedirects(c, "http://"+redirectAddr+"/", 3) })

	proxyTestExpectGetOverTLS(t, app, "final")
}

// Test_Proxy_DoRedirects_StreamedRequestBody pins that a method-keeping redirect of a streamed
// request (StreamRequestBody) is followed when the first hop left the whole body in memory.
func Test_Proxy_DoRedirects_StreamedRequestBody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		body       string
		wantBody   string
		wantStatus int
		wantFinals int32
	}{
		{name: "empty", wantStatus: fiber.StatusOK, wantBody: "POST:0", wantFinals: 1},
		{name: "small", body: "payload", wantStatus: fiber.StatusOK, wantBody: "POST:7", wantFinals: 1},
		{name: "more than is read ahead", body: strings.Repeat("a", 8*1024+1), wantStatus: fiber.StatusInternalServerError, wantBody: fasthttp.ErrRedirectBodyStream.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var finals atomic.Int32 // requests that reached the target of the redirect
			upstream := fiber.New()
			ln := proxyTestListen(t)
			addr := ln.Addr().String()
			upstream.Post("/start", func(c fiber.Ctx) error {
				return c.Redirect().Status(fiber.StatusTemporaryRedirect).To("http://" + addr + "/final")
			})
			upstream.Post("/final", func(c fiber.Ctx) error {
				finals.Add(1)
				return c.SendString(c.Method() + ":" + strconv.Itoa(len(c.Body())))
			})
			startServer(upstream, ln)

			app := fiber.New(fiber.Config{StreamRequestBody: true})
			app.Post("/", func(c fiber.Ctx) error { return DoRedirects(c, "http://"+addr+"/start", 3) })

			req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set(fiber.HeaderContentLength, strconv.Itoa(len(tc.body)))
			resp := apptest.Do(t, app, req, fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true})
			defer func() { require.NoError(t, resp.Body.Close()) }()

			body := apptest.Body(t, resp)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t, tc.wantBody, body)
			require.Equal(t, tc.wantFinals, finals.Load(), "requests that reached the target of the redirect")
		})
	}
}

func Test_Proxy_Forward(t *testing.T) {
	t.Parallel()
	for _, tc := range []realIPHandler{
		{name: "Forward", build: func(addr string) fiber.Handler { return Forward("http://" + addr) }},
		{name: "BalancerForward", build: func(addr string) fiber.Handler { return BalancerForward([]string{addr}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("forwarded") })
			app.Use(tc.build(addr))

			apptest.ExpectBody(t, app, "/", fiber.StatusOK, "forwarded")
		})
	}
}

// A refused request replays the Use chain; forwarded, the upstream ran it with
// an empty body while the client was told 413.
func Test_Proxy_RefusedRequestIsNotForwarded(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var executed atomic.Int32
			upstream := fiber.New()
			upstream.Use(func(c fiber.Ctx) error {
				executed.Add(1)
				return c.SendString("executed")
			})
			app := fiber.New(fiber.Config{BodyLimit: 8})
			app.Use(tc.build(listenUpstream(t, upstream)))

			refused := httptest.NewRequest(fiber.MethodPost, "http://example.com/transfer", strings.NewReader(strings.Repeat("x", 64)))
			_, err := app.Test(refused)
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
			require.Zero(t, executed.Load())

			accepted := httptest.NewRequest(fiber.MethodPost, "http://example.com/transfer", strings.NewReader("ok"))
			apptest.Expect(t, app, accepted, fiber.StatusOK)
			require.Equal(t, int32(1), executed.Load())
		})
	}
}

// Test_Proxy_RealIPOverwritesClientClaims checks that every helper that writes
// X-Real-IP leaves the upstream exactly one line, the address Fiber saw, however
// many lines the client sent.
//
// The forwarders overwrite X-Real-IP so the upstream can attribute the request to
// the peer Fiber actually saw. Header.Set replaces the first field line with a
// given name and leaves any others in place, so a client that sends the header
// twice used to keep one of its own values on the wire — and the upstream, which
// may read the last line or join the pair per RFC 9110 Section 5.2, would
// attribute the request to an address the client chose.
func Test_Proxy_RealIPOverwritesClientClaims(t *testing.T) {
	t.Parallel()
	for _, handler := range realIPHandlers() {
		for _, claim := range []struct {
			name       string
			connection string // the Connection header, not sent when empty
			values     []string
		}{
			{name: "one spoofed line", values: []string{"10.0.0.1"}},
			{name: "two spoofed lines", values: []string{"7.7.7.7", "8.8.8.8"}},
			// A client naming X-Real-IP in Connection does not have the line this
			// hop writes removed: the listing is applied to the request as
			// received, and X-Real-IP is this hop's to write.
			{name: "named in Connection", connection: "X-Real-IP", values: []string{"7.7.7.7"}},
		} {
			t.Run(handler.name+" "+claim.name, func(t *testing.T) {
				t.Parallel()
				addr, seen := proxyTestFieldLinesUpstream(t, "OK", realIPHeader)

				app := fiber.New()
				app.Use(handler.build(addr))

				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"))
				for _, value := range claim.values {
					req.Header.Add("X-Real-IP", value)
				}
				if claim.connection != "" {
					req.Header.Set(fiber.HeaderConnection, claim.connection)
				}

				apptest.Expect(t, app, req, fiber.StatusOK)
				// app.Test injects 0.0.0.0 as the remote address.
				require.Equal(t, []string{"X-Real-Ip=0.0.0.0"}, <-seen)
			})
		}
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

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Real-IP", "203.0.113.9"))

	apptest.ExpectBody(t, app, req, fiber.StatusOK, "203.0.113.9")
}

// realIPHandler names one of the helpers that write X-Real-IP, with a
// constructor taking the upstream address.
type realIPHandler struct {
	build func(addr string) fiber.Handler
	name  string
}

// realIPHandlers returns every helper that writes X-Real-IP. DomainForward is
// built for example.com, so requests carry that Host.
func realIPHandlers() []realIPHandler {
	return []realIPHandler{
		{name: "Forward", build: func(addr string) fiber.Handler { return Forward("http://" + addr) }},
		{name: "DomainForward", build: func(addr string) fiber.Handler { return DomainForward("example.com", "http://"+addr) }},
		{name: "BalancerForward", build: func(addr string) fiber.Handler { return BalancerForward([]string{"http://" + addr}) }},
		{name: "Balancer", build: func(addr string) fiber.Handler { return Balancer(Config{Servers: []string{addr}}) }},
	}
}

// Test_Proxy_ConnectionCannotRemoveRealIPWhateverTheCase is the same under
// DisableHeaderNormalizing, where the listing and the field arrive spelled as
// the client sent them.
func Test_Proxy_ConnectionCannotRemoveRealIPWhateverTheCase(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr, seen := proxyTestFieldLinesUpstream(t, "upstream", realIPHeader)

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
			app.Use(tc.build(addr))

			body := sendRawUnnormalized(t, app, "GET / HTTP/1.1\r\nHost: example.com\r\n"+
				"connection: x-real-ip\r\n"+
				"x-real-ip: 7.7.7.7\r\n\r\n")
			require.Equal(t, "upstream", body)

			lines := <-seen
			require.Len(t, lines, 1, "exactly one X-Real-IP reaches the upstream: %v", lines)
			require.NotContains(t, lines[0], "7.7.7.7")
		})
	}
}

// Test_Proxy_ConnectionCannotRemoveForwardingHeaders checks that the fields an
// application writes about the client before the proxy runs are not the
// client's to remove by naming them in Connection, while any other field it
// names there still is, as RFC 9110 §7.6.1 asks.
func Test_Proxy_ConnectionCannotRemoveForwardingHeaders(t *testing.T) {
	t.Parallel()
	const (
		clientIP  = "203.0.113.9"
		forwarded = "for=203.0.113.9;proto=https"
	)

	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr, seen := proxyTestFieldLinesUpstream(t, "OK", fiber.HeaderXForwardedFor, fiber.HeaderXForwardedHost,
				fiber.HeaderXForwardedProto, fiber.HeaderForwarded, "X-Gateway-Policy")

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				h := &c.Request().Header
				h.Set(fiber.HeaderXForwardedFor, clientIP)
				h.Set(fiber.HeaderXForwardedHost, "app.example")
				h.Set(fiber.HeaderXForwardedProto, "https")
				h.Set(fiber.HeaderForwarded, forwarded)
				h.Set("X-Gateway-Policy", "read-only")
				return c.Next()
			})
			app.Use(tc.build(addr))

			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"),
				apptest.WithHeader(fiber.HeaderConnection,
					"X-Forwarded-For, X-Forwarded-Host, X-Forwarded-Proto, Forwarded, X-Gateway-Policy"))

			apptest.Expect(t, app, req, fiber.StatusOK)
			require.ElementsMatch(t, []string{
				"X-Forwarded-For=" + clientIP,
				"X-Forwarded-Host=app.example",
				"X-Forwarded-Proto=https",
				"Forwarded=" + forwarded,
			}, <-seen, "the forwarding fields reach the upstream; X-Gateway-Policy, which the listing may remove, does not")
		})
	}
}

// Test_Proxy_ConnectionListLimit checks that a request listing more names in
// Connection than the proxy removes one by one is refused, not forwarded.
func Test_Proxy_ConnectionListLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var forwarded atomic.Int32
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				forwarded.Add(1)
				return c.SendStatus(fiber.StatusOK)
			})

			app := fiber.New()
			app.Use(tc.build(addr))

			const tooLarge = fiber.StatusRequestHeaderFieldsTooLarge
			for _, tt := range []struct {
				lines  []string
				status int
			}{
				{lines: []string{connectionNames(maxConnectionOptions)}, status: fiber.StatusOK},
				{lines: []string{connectionNames(maxConnectionOptions + 1)}, status: tooLarge},
				{lines: []string{connectionNames(maxConnectionOptions), "a"}, status: tooLarge},
			} {
				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"))
				for _, line := range tt.lines {
					req.Header.Add(fiber.HeaderConnection, line)
				}
				apptest.Expect(t, app, req, tt.status)
			}
			require.Equal(t, int32(1), forwarded.Load())
		})
	}
}

// caseSpellings returns n spellings of name, a lower-case token, that differ
// from it and from each other only in case.
func caseSpellings(name string, n int) []string {
	var out []string
	for mask := 1; len(out) < n && mask < 1<<len(name); mask++ {
		b := []byte(name)
		for i := range b {
			if mask&(1<<i) != 0 && b[i] >= 'a' && b[i] <= 'z' {
				b[i] -= 'a' - 'A'
			}
		}
		if s := string(b); s != name && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Test_Proxy_ConnectionListLimitCountsSpellings checks that under
// DisableHeaderNormalizing each other spelling of a removed name counts against
// the limit like a listed name, as each costs another scan, while a name
// repeated in one spelling costs once.
func Test_Proxy_ConnectionListLimitCountsSpellings(t *testing.T) {
	t.Parallel()
	addr, seen := proxyTestFieldLinesUpstream(t, "", "X-Abcdef", fiber.HeaderUpgrade)

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(Forward("http://" + addr))

	repeated := slices.Repeat([]string{"X-ABCDEF"}, 200)
	for _, tt := range []struct {
		name       string
		connection string
		lines      []string
		status     int
	}{
		// The listed name takes one of the budget.
		{name: "within", connection: "x-abcdef", lines: caseSpellings("x-abcdef", maxConnectionOptions-1), status: fiber.StatusOK},
		{name: "beyond", connection: "x-abcdef", lines: caseSpellings("x-abcdef", maxConnectionOptions), status: fiber.StatusRequestHeaderFieldsTooLarge},
		{name: "repeated", connection: "x-abcdef", lines: repeated, status: fiber.StatusOK},
		{name: "hop-by-hop", lines: caseSpellings("upgrade", 2*maxConnectionOptions), status: fiber.StatusRequestHeaderFieldsTooLarge},
	} {
		req := apptest.Req(fiber.MethodGet, "/")
		if tt.connection != "" {
			req.Header[fiber.HeaderConnection] = []string{tt.connection}
		}
		for _, k := range tt.lines {
			// Assigned rather than added, which would canonicalize the key.
			req.Header[k] = append(req.Header[k], "v")
		}
		resp := apptest.Do(t, app, req)
		require.Equal(t, tt.status, resp.StatusCode, tt.name)
		if tt.status == fiber.StatusOK {
			require.Empty(t, <-seen, tt.name)
		}
	}
}

// Test_Proxy_RealIPSpellingLimit checks that under DisableHeaderNormalizing a
// request spelling X-Real-IP in more ways than the proxy removes one by one is
// refused, not forwarded.
func Test_Proxy_RealIPSpellingLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr, seen := proxyTestFieldLinesUpstream(t, "", realIPHeader)

			app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
			app.Use(tc.build(addr))

			for n, status := range map[int]int{
				maxConnectionOptions:     fiber.StatusOK,
				maxConnectionOptions + 1: fiber.StatusRequestHeaderFieldsTooLarge,
			} {
				spellings := caseSpellings("x-real-ip", n)
				require.NotContains(t, spellings, realIPHeader, "every spelling counts")
				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"))
				for _, k := range spellings {
					// Assigned rather than added, which would canonicalize the key.
					req.Header[k] = []string{"6.6.6.6"}
				}
				apptest.Expect(t, app, req, status)
				if status == fiber.StatusOK {
					require.Len(t, <-seen, 1)
				}
			}
		})
	}
}

// Test_Proxy_ResponseConnectionListLimit checks that a response listing more
// names in Connection than the request may is answered with 502, and that none
// of the upstream's fields go out with it.
func Test_Proxy_ResponseConnectionListLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				n, err := strconv.Atoi(c.Get("X-Count"))
				if err != nil {
					return fiber.ErrBadRequest
				}
				c.Set(fiber.HeaderConnection, connectionNames(n))
				c.Set("X-Upstream", "1")
				return c.SendString("upstream")
			})

			app := fiber.New()
			app.Use(tc.build(addr))

			for n, status := range map[int]int{
				maxConnectionOptions:     fiber.StatusOK,
				maxConnectionOptions + 1: fiber.StatusBadGateway,
			} {
				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost("example.com"),
					apptest.WithHeader("X-Count", strconv.Itoa(n)))
				resp := apptest.Expect(t, app, req, status)
				require.Equal(t, status == fiber.StatusOK, resp.Header.Get("X-Upstream") == "1")
			}
		})
	}
}

// Test_Proxy_ResponseSpellingLimit checks the same budget against an upstream
// whose fields the client keeps as sent: each other spelling of a hop-by-hop
// name costs a scan, a name repeated in one spelling costs once.
func Test_Proxy_ResponseSpellingLimit(t *testing.T) {
	t.Parallel()
	upstream := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	upstream.Get("/", func(c fiber.Ctx) error {
		n, err := strconv.Atoi(c.Get("X-Count"))
		if err != nil {
			return fiber.ErrBadRequest
		}
		for _, k := range caseSpellings("upgrade", n) {
			c.Response().Header.Add(k, "v")
		}
		for range 100 {
			c.Response().Header.Add("keep-alive", "v")
		}
		return c.SendString("upstream")
	})

	app := fiber.New()
	app.Use(Forward("http://"+listenUpstream(t, upstream), &fasthttp.Client{DisableHeaderNamesNormalizing: true}))

	for n, status := range map[int]int{3: fiber.StatusOK, 2*maxConnectionOptions + 1: fiber.StatusBadGateway} {
		req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Count", strconv.Itoa(n)))
		resp := apptest.Expect(t, app, req, status)
		require.Empty(t, resp.Header.Values(fiber.HeaderUpgrade))
		require.Empty(t, resp.Header.Values(fiber.HeaderKeepAlive))
	}
}

// Test_Proxy_RealIPOutlivesTheTrailerReturn checks that the address written
// into X-Real-IP is the one the request carried when the field it was read
// from is deleted before the write. Removing Trailer returned the received
// trailer fields to the header section, where fasthttp reuses the buffers of
// the lines deleted before them, so an address still aliasing such a buffer
// read as whatever trailer landed on it.
func Test_Proxy_RealIPOutlivesTheTrailerReturn(t *testing.T) {
	t.Parallel()
	for _, tc := range realIPHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr, seen := proxyTestFieldLinesUpstream(t, "upstream", realIPHeader)

			app := fiber.New(fiber.Config{
				TrustProxy:       true,
				TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0/0"}},
				ProxyHeader:      "CF-Connecting-IP",
			})
			app.Use(tc.build(addr))

			// The peer names the proxy header in Connection, so that field is
			// deleted before Trailer is, and sends enough trailer fields for
			// one of them to land on its buffer. A GET, since that is the
			// route the upstream serves; a body on it is framed all the same.
			body := sendRaw(t, app, "GET / HTTP/1.1\r\nHost: example.com\r\n"+
				"CF-Connecting-IP: 203.0.113.5\r\n"+
				"Connection: CF-Connecting-IP\r\n"+
				"Transfer-Encoding: chunked\r\n\r\n"+
				"5\r\nhello\r\n0\r\n"+
				"X-A: 192.168.1.1\r\nX-B: 192.168.1.1\r\nX-C: 192.168.1.1\r\n\r\n")
			require.Equal(t, "upstream", body)
			require.Equal(t, []string{"X-Real-Ip=203.0.113.5"}, <-seen)
		})
	}
}

// trailerHandlers returns every helper that strips the hop-by-hop fields.
func trailerHandlers() []realIPHandler {
	return append(realIPHandlers(), realIPHandler{name: "Do", build: func(addr string) fiber.Handler {
		return func(c fiber.Ctx) error { return Do(c, "http://"+addr) }
	}})
}

// Test_Proxy_TrailerDoesNotBecomeHeader checks that a field sent after the last
// chunk does not reach the upstream as a header line, which removing Trailer
// alone made it.
func Test_Proxy_TrailerDoesNotBecomeHeader(t *testing.T) {
	t.Parallel()
	const owned = "X-Remote-User"

	for _, tc := range trailerHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen := make(chan []string, 1)
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				lines := matchingFieldLines(c, owned, fiber.HeaderUserAgent, fiber.HeaderTrailer, "X-Kept")
				for _, key := range c.Request().Header.PeekTrailerKeys() {
					lines = append(lines, "trailer "+string(key))
				}
				seen <- append(lines, string(c.Body()))
				return c.SendString("upstream")
			})

			app := fiber.New()
			app.Use(tc.build(addr))

			for _, tt := range []struct {
				send    func(*testing.T, *fiber.App, string) string
				name    string
				head    string
				trailer string
			}{
				{name: "unannounced", trailer: owned + ": admin"},
				{name: "announced", head: "Trailer: " + owned + "\r\n", trailer: owned + ": admin"},
				{name: "listed", head: "Connection: Trailer\r\n", trailer: owned + ": admin"},
				{name: "repeated", trailer: owned + ": admin\r\n" + owned + ": root"},
				{name: "own slot", trailer: "User-Agent: late"},
				{name: "as spelled", send: sendRawUnnormalized, head: "trailer: x-remote-user\r\n", trailer: "x-remote-user: admin"},
			} {
				send := tt.send
				if send == nil {
					send = sendRaw
				}
				body := send(t, app, rawGet+"X-Kept: early\r\n"+tt.head+
					"Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n"+tt.trailer+"\r\n\r\n")
				require.Equal(t, "upstream", body, tt.name)
				require.ElementsMatch(t, []string{"X-Kept=early", "hello"}, <-seen, tt.name)
			}
		})
	}
}

// Test_Proxy_TrailerDropKeepsHeaderLines checks that dropping the trailer section
// keeps the header lines that share its names, and a header the application
// writes under a name the client announced.
func Test_Proxy_TrailerDropKeepsHeaderLines(t *testing.T) {
	t.Parallel()
	for _, tc := range trailerHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				lines := matchingFieldLines(c, fiber.HeaderForwarded, "X-Remote-User", fiber.HeaderUserAgent)
				slices.Sort(lines)
				return c.SendString(strings.Join(lines, ","))
			})

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Request().Header.Set(fiber.HeaderForwarded, "for=192.0.2.1")
				return c.Next()
			})
			app.Use(tc.build(addr))

			const (
				chunked = "Transfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n"
				written = "Forwarded=for=192.0.2.1"
			)
			for raw, want := range map[string]string{
				rawGet + "\r\n":                                       written,
				rawGet + "Trailer: Forwarded\r\n\r\n":                 written,
				rawGet + chunked + "Forwarded: for=127.0.0.1\r\n\r\n": written,
				rawGet + "X-Remote-User: alice\r\n" + chunked +
					"X-Remote-User: admin\r\n\r\n": written + ",X-Remote-User=alice",
				rawGet + "User-Agent: early\r\n" + chunked +
					"User-Agent: late\r\n\r\n": written + ",User-Agent=early",
			} {
				require.Equal(t, want, sendRaw(t, app, raw), raw)
			}
		})
	}
}

// Test_Proxy_TrailerSectionWhateverThePolicy checks that a trailer section the request still holds is dropped
// before it goes upstream, or refused above fieldname.MaxTrailers names, with the hop-by-hop fields kept too.
func Test_Proxy_TrailerSectionWhateverThePolicy(t *testing.T) {
	t.Parallel()
	// Not a Fiber app, which would drop the section or refuse the request itself.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "header=%q trailer=%d", r.Header.Values(fiber.HeaderTrailer), len(r.Trailer))
	}))
	t.Cleanup(upstream.Close)
	addr := upstream.Listener.Addr().String()
	announced := make([]string, fieldname.MaxTrailers+1)
	for i := range announced {
		announced[i] = "X-" + strconv.Itoa(i)
	}
	names := strings.Join(announced, ", ")

	for _, keep := range []bool{false, true} {
		t.Run(fmt.Sprintf("keep=%v", keep), func(t *testing.T) {
			t.Parallel()
			policy := proxyTestAllowPrivatePolicy()
			policy.KeepHopByHopHeaders = keep
			for name, handler := range map[string]fiber.Handler{
				"Balancer": Balancer(Config{Servers: []string{addr}, SecurityPolicy: &policy}),
				"Forward": func(c fiber.Ctx) error {
					return forwardWithRealIP(c, "http://"+addr, normalizePolicy(policy))
				},
			} {
				app := fiber.New()
				app.Use(func(c fiber.Ctx) error {
					// Announced after the router dropped the client's, as only the application can.
					if err := c.Request().Header.AddTrailer(c.Get("X-Announce")); err != nil {
						return err
					}
					return c.Next()
				})
				app.Get("/", handler)

				for announce, want := range map[string]int{"X-T": fiber.StatusOK, names: fiber.StatusRequestHeaderFieldsTooLarge} {
					req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Announce", announce))
					resp := apptest.Do(t, app, req)
					body := apptest.Body(t, resp)
					require.Equal(t, want, resp.StatusCode, name+" "+body)
					if want == fiber.StatusOK {
						require.Equal(t, "header=[] trailer=0", body, name)
					}
				}
			}
		})
	}
}

// Test_Proxy_StreamedBodyTrailerSection checks that the trailer section of a streamed body
// reaches the upstream neither as header lines nor as trailers, and does not take the
// place of a header written once the body has been read, whatever the hop-by-hop policy.
func Test_Proxy_StreamedBodyTrailerSection(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "body=%s header=%q trailer=%q", body, r.Header.Values("X-User"), r.Trailer.Values("X-User"))
	}))
	t.Cleanup(upstream.Close)

	var many strings.Builder
	for i := range fieldname.MaxTrailers - 1 {
		fmt.Fprintf(&many, "X-T%d: v\r\n", i)
	}

	for _, keep := range []bool{false, true} {
		t.Run(fmt.Sprintf("keep=%v", keep), func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{StreamRequestBody: true})
			balancer := Balancer(Config{
				Servers:        []string{upstream.Listener.Addr().String()},
				SecurityPolicy: &SecurityPolicy{AllowPrivateIPs: true, KeepHopByHopHeaders: keep},
			})
			app.Post("/forward", balancer)
			app.Post("/read", func(c fiber.Ctx) error {
				c.Body()
				c.Request().Header.Set("X-User", "alice")
				return c.Next()
			}, balancer)
			appAddr := listenUpstream(t, app)

			exchange := func(path, head, trailers string) (int, string) {
				conn, err := net.Dial(fiber.NetworkTCP4, appAddr)
				require.NoError(t, err)
				require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
				_, err = conn.Write([]byte("POST " + path + " HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n" + head +
					"Transfer-Encoding: chunked\r\n\r\n2\r\nhi\r\n0\r\n" + trailers + "X-User: x\r\n\r\n"))
				require.NoError(t, err)
				resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
				require.NoError(t, err)
				body := apptest.Body(t, resp)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, conn.Close())
				return resp.StatusCode, body
			}

			for _, tc := range []struct{ head, forwarded, trailers string }{
				{head: "", forwarded: `[]`},
				{head: "X-User: admin\r\n", forwarded: `["admin"]`},
				{head: "X-User: admin\r\nTrailer: X-User\r\n", forwarded: `["admin"]`},
				{head: "X-User: admin\r\n", forwarded: `["admin"]`, trailers: many.String()},
			} {
				for path, want := range map[string]string{
					"/forward": "body=hi header=" + tc.forwarded + " trailer=[]",
					"/read":    `body=hi header=["alice"] trailer=[]`,
				} {
					_, body := exchange(path, tc.head, tc.trailers)
					require.Equal(t, want, body, path+" "+tc.head)
				}
			}

			// One name past the cap fails the body read and drops the section all the same: the proxy answers 431,
			// also after a handler that read the body and went on, since the failed read sticks to the stream.
			over := many.String() + "X-T15: v\r\n"
			for _, path := range []string{"/forward", "/read"} {
				status, body := exchange(path, "X-User: admin\r\n", over)
				require.Equal(t, fiber.StatusRequestHeaderFieldsTooLarge, status, path+" "+body)
			}
		})
	}
}

// Test_Proxy_ForwardingLinesKeepReceivedOrder checks that a client's forwarding line and the one a front
// proxy appended reach the upstream in the order received: after fasthttp removed a line in front of them,
// after the application added a forwarding line before the proxy, or removed a line in ModifyRequest
// (fasthttp moves the last line into a removed one's slot). A request with more forwarding lines than the
// order is kept for is refused rather than forwarded with them in whatever order fasthttp left.
func Test_Proxy_ForwardingLinesKeepReceivedOrder(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString(forwardingLinesByName(c)) })
	// The edits of the application are tried with the first two heads.
	heads := []struct{ name, head string }{
		{"empty chunked body", "Transfer-Encoding: chunked\r\n" + forwardingLines + "\r\n0\r\n\r\n"},
		{"connection listed", "X-Junk: 1\r\nConnection: X-Junk\r\n" + forwardingLines + "\r\n"},
		{"hop-by-hop", "Keep-Alive: timeout=5\r\n" + forwardingLines + "\r\n"},
		{"real ip", "X-Real-IP: 192.0.2.1\r\n" + forwardingLines + "\r\n"},
	}
	const added = "X-Forwarded-For=6.6.6.6,X-Forwarded-For=203.0.113.7,X-Forwarded-For=127.0.0.1," +
		"X-Forwarded-Proto=http,X-Forwarded-Proto=https,X-Url-Scheme=http,X-Url-Scheme=https"
	var tooMany strings.Builder
	for i := range fieldname.MaxOrderedLines + 1 {
		fmt.Fprintf(&tooMany, "X-Forwarded-%d: v\r\n", i)
	}

	for _, tc := range trailerHandlers() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(tc.build(addr))
			for _, h := range heads {
				require.Equal(t, forwardingLinesWant, sendRaw(t, app, rawGet+h.head), h.name)
			}
		})

		t.Run("added before "+tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Request().Header.Add(fiber.HeaderXForwardedFor, "127.0.0.1")
				return c.Next()
			}, tc.build(addr))
			for _, h := range heads[:2] {
				require.Equal(t, added, sendRaw(t, app, rawGet+h.head), h.name)
			}
		})

		t.Run("too many lines "+tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(tc.build(addr))
			require.Equal(t, utils.StatusMessage(fiber.StatusRequestHeaderFieldsTooLarge),
				sendRaw(t, app, rawGet+tooMany.String()+"\r\n"))
		})
	}

	t.Run("removed in ModifyRequest", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		app.Use(Balancer(Config{Servers: []string{addr}, ModifyRequest: func(c fiber.Ctx) error {
			c.Request().Header.Del(fiber.HeaderAuthorization)
			c.Request().Header.Del("X-Api-Key")
			return nil
		}}))
		for _, h := range heads[:2] {
			head := "Authorization: Bearer x\r\nX-Api-Key: k\r\n" + h.head
			require.Equal(t, forwardingLinesWant, sendRaw(t, app, rawGet+head), h.name)
		}
	})
}

// A client's forwarding lines followed by the ones a front proxy appended.
const (
	forwardingLines = "X-Forwarded-For: 6.6.6.6\r\nX-Forwarded-Proto: http\r\nX-Url-Scheme: http\r\n" +
		"X-Forwarded-For: 203.0.113.7\r\nX-Forwarded-Proto: https\r\nX-Url-Scheme: https\r\n"
	forwardingLinesWant = "X-Forwarded-For=6.6.6.6,X-Forwarded-For=203.0.113.7,X-Forwarded-Proto=http," +
		"X-Forwarded-Proto=https,X-Url-Scheme=http,X-Url-Scheme=https"
)

// forwardingLinesByName reports the forwarding lines of c grouped by name, since only the order within a name
// counts (RFC 9110 Section 5.3).
func forwardingLinesByName(c fiber.Ctx) string {
	lines := matchingFieldLines(c, fiber.HeaderXForwardedFor, fiber.HeaderXForwardedProto, fiber.HeaderXUrlScheme)
	slices.SortStableFunc(lines, func(a, b string) int {
		return strings.Compare(strings.Split(a, "=")[0], strings.Split(b, "=")[0])
	})
	return strings.Join(lines, ",")
}

// Test_Proxy_ForwardingLinesKeepReceivedOrderBehindAdaptor checks the order for a request net/http read, for
// which fasthttp holds no received block: the strip's Del can move a forwarding line all the same.
func Test_Proxy_ForwardingLinesKeepReceivedOrderBehindAdaptor(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString(forwardingLinesByName(c)) })
	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}}))
	h := adaptor.FiberApp(app)

	// r.Header's map order decides which line is stored last, so several rounds. HTTP/2 keeps a value's edge
	// whitespace, which a received line drops.
	for _, client := range []string{"6.6.6.6", "6.6.6.6 "} {
		for range 32 {
			for name, value := range map[string]string{"X-Junk": "1", fiber.HeaderKeepAlive: "timeout=5"} {
				r := apptest.Req(http.MethodGet, "/")
				r.Header.Set(name, value)
				r.Header.Set(fiber.HeaderConnection, "X-Junk")
				r.Header.Add(fiber.HeaderXForwardedFor, client)
				r.Header.Add(fiber.HeaderXForwardedFor, "203.0.113.7")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				require.Equal(t, "X-Forwarded-For=6.6.6.6,X-Forwarded-For=203.0.113.7", w.Body.String(), "%s %q", name, client)
			}
		}
	}
}

// Test_Proxy_DoRedirects_ForwardingLinesKeepReceivedOrder checks a redirect hop, from which fields are removed
// after the first hop went out: the body fields when the method changes, the credentials off the origin.
func Test_Proxy_DoRedirects_ForwardingLinesKeepReceivedOrder(t *testing.T) {
	t.Parallel()
	target := fiber.New()
	target.Post("/303", func(c fiber.Ctx) error {
		c.Location("/final")
		return c.SendStatus(fiber.StatusSeeOther)
	})
	target.Post("/307", func(c fiber.Ctx) error {
		c.Location("http://" + loopbackAlias(t, string(c.Request().Host())) + "/final")
		return c.SendStatus(fiber.StatusTemporaryRedirect)
	})
	target.All("/final", func(c fiber.Ctx) error { return c.SendString(forwardingLinesByName(c)) })
	addr := listenUpstream(t, target)

	for path, first := range map[string]string{
		"/303": "Content-Language: de\r\n",
		"/307": "Authorization: Bearer x\r\n",
	} {
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error { return DoRedirects(c, "http://"+addr+path, 1) })
		body := sendRaw(t, app, "POST / HTTP/1.1\r\nHost: example.com\r\n"+first+forwardingLines+"Content-Length: 2\r\n\r\nhi")
		require.Equal(t, forwardingLinesWant, body, path)
	}
}

// Test_Proxy_ForwardingOrderInOrderDoesNotAllocate pins that the order check adds no allocation for the common
// shape, a client's line and the one a front proxy appended, already in received order.
// Not parallel: AllocsPerRun panics in a parallel test.
func Test_Proxy_ForwardingOrderInOrderDoesNotAllocate(t *testing.T) {
	const raw = "GET / HTTP/1.1\r\nHost: example.com\r\nX-Forwarded-For: 6.6.6.6\r\nAccept: */*\r\n" +
		"X-Forwarded-For: 203.0.113.7\r\nX-Forwarded-Proto: https\r\n\r\n"
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	proxyTestReadRequest(t, req, raw)

	strip := testing.AllocsPerRun(100, func() { stripHopByHopRequestHeaders(req, true) })
	both := testing.AllocsPerRun(100, func() {
		stripHopByHopRequestHeaders(req, true)
		if !keepForwardingOrder(req) {
			t.Fatal("refused")
		}
	})
	require.InDelta(t, strip, both, 0)
}

// Test_Proxy_Do_ConnectionCannotRemoveRealIP checks that the X-Real-IP written
// by hand before Do, as the docs show, is not the client's to remove either.
func Test_Proxy_Do_ConnectionCannotRemoveRealIP(t *testing.T) {
	t.Parallel()
	addr, seen := proxyTestFieldLinesUpstream(t, "OK", realIPHeader)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		ip := c.IP()
		c.Request().Header.Del(realIPHeader)
		c.Request().Header.Add(realIPHeader, ip)
		return Do(c, "http://"+addr)
	})

	req := apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderConnection, "X-Real-IP"),
		apptest.WithHeader(realIPHeader, "7.7.7.7"))

	apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, []string{"X-Real-Ip=0.0.0.0"}, <-seen)
}

func Test_Proxy_Forward_WithClient_TLSConfig(t *testing.T) {
	// Intentionally not parallel: WithClient replaces the process-wide shared client.
	restoreGlobalProxyClient(t)

	ln := proxyTestTLSListener(t)

	app := fiber.New()

	app.Get("/tlsfwd", func(c fiber.Ctx) error { return c.SendString("tls forward") })

	addr := ln.Addr().String()
	clientTLSConf := &tls.Config{InsecureSkipVerify: true}

	// disable certificate verification
	WithClient(&fasthttp.Client{TLSConfig: clientTLSConf})
	app.Use(Forward("https://" + addr + "/tlsfwd"))

	startServer(app, ln)

	client := clientpkg.New()
	client.SetTLSConfig(clientTLSConf)

	proxyTestExpectGet(t, client, "https://"+addr, "tls forward")
}

// Test_Proxy_Balancer_Modify runs the ModifyRequest and ModifyResponse hooks of a Balancer: what they change
// reaches the upstream or the client, and an error either returns ends in a 500.
func Test_Proxy_Balancer_Modify(t *testing.T) {
	t.Parallel()
	failed := errors.New("modify failed")
	for _, tc := range []struct {
		upstream fiber.Handler
		name     string
		body     string // the body the client gets back, unchecked when empty
		cfg      Config
		status   int
	}{
		{
			name:     "ModifyResponse",
			upstream: func(c fiber.Ctx) error { return c.Status(500).SendString("not modified") },
			cfg: Config{ModifyResponse: func(c fiber.Ctx) error {
				c.Response().SetStatusCode(fiber.StatusOK)
				return c.SendString("modified response")
			}},
			status: fiber.StatusOK, body: "modified response",
		},
		{
			name:     "ModifyRequest",
			upstream: func(c fiber.Ctx) error { return c.SendString(string(c.Request().Body())) },
			cfg: Config{ModifyRequest: func(c fiber.Ctx) error {
				c.Request().SetBody([]byte("modified request"))
				return nil
			}},
			status: fiber.StatusOK, body: "modified request",
		},
		{
			name:     "ModifyRequest error",
			upstream: func(c fiber.Ctx) error { return c.SendString("ok") },
			cfg:      Config{ModifyRequest: func(fiber.Ctx) error { return failed }},
			status:   fiber.StatusInternalServerError,
		},
		{
			name:     "ModifyResponse error",
			upstream: func(c fiber.Ctx) error { return c.SendString("ok") },
			cfg:      Config{ModifyResponse: func(fiber.Ctx) error { return failed }},
			status:   fiber.StatusInternalServerError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, tc.upstream)

			cfg := tc.cfg
			cfg.Servers = []string{addr}
			app := fiber.New()
			app.Use(Balancer(cfg))

			if tc.body == "" {
				apptest.Expect(t, app, "/", tc.status)
				return
			}
			apptest.ExpectBody(t, app, "/", tc.status, tc.body)
		})
	}
}

func Test_Proxy_Timeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		body    string
		delay   time.Duration // how long the upstream takes to answer
		timeout time.Duration // Config.Timeout
		status  int
	}{
		{name: "slow server within the timeout", delay: 300 * time.Millisecond, timeout: 600 * time.Millisecond, status: fiber.StatusOK, body: "fiber is awesome"},
		{name: "server slower than the timeout", delay: time.Second, timeout: 100 * time.Millisecond, status: fiber.StatusInternalServerError, body: fasthttp.ErrTimeout.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				time.Sleep(tc.delay)
				return c.SendString("fiber is awesome")
			})

			app := fiber.New()
			app.Use(Balancer(Config{Servers: []string{addr}, Timeout: tc.timeout}))

			resp := apptest.Do(t, app, "/", fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true})
			require.Equal(t, tc.status, resp.StatusCode)

			require.Equal(t, tc.body, apptest.Body(t, resp))
		})
	}
}

func Test_Proxy_Buffer_Size_Response(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		long := strings.Join(make([]string, 5000), "-")
		c.Set("Very-Long-Header", long)
		return c.SendString("ok")
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, KeepConnectionHeader: true}))

	apptest.Expect(t, app, "/", fiber.StatusInternalServerError)

	app = fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, ReadBufferSize: 1024 * 8}))

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

// Every Do helper answers with what its upstream sent, or the error it ended in,
// and leaves the request URL of the app's own request as it was.
func Test_Proxy_DoHelpers(t *testing.T) {
	t.Parallel()
	do := func(c fiber.Ctx, target string) error { return Do(c, target) }
	doRedirects := func(maxRedirects int) func(c fiber.Ctx, target string) error {
		return func(c fiber.Ctx, target string) error { return DoRedirects(c, target, maxRedirects) }
	}

	for _, tc := range []struct {
		do       func(c fiber.Ctx, target string) error
		name     string
		body     string // what the helper has to answer with, which the upstream says too unless it redirects
		status   int
		redirect bool // whether the upstream redirects to a page of its own instead of answering
	}{
		{name: "Do", do: do, status: fiber.StatusOK, body: "proxied"},
		{name: "Do with the real URL", do: do, status: fiber.StatusOK, body: "real url"},
		{
			name: "Do does not follow a redirect", do: do, redirect: true, status: fiber.StatusMovedPermanently,
			body: "redirect",
		},
		{name: "DoRedirects", do: doRedirects(1), redirect: true, status: fiber.StatusOK, body: "final"},
		{
			name: "DoRedirects with too many redirects", do: doRedirects(0), redirect: true,
			status: fiber.StatusInternalServerError, body: fasthttp.ErrTooManyRedirects.Error(),
		},
		{
			name: "DoTimeout", do: func(c fiber.Ctx, target string) error { return DoTimeout(c, target, time.Second) },
			status: fiber.StatusOK, body: "proxied",
		},
		{
			name:   "DoDeadline",
			do:     func(c fiber.Ctx, target string) error { return DoDeadline(c, target, time.Now().Add(time.Second)) },
			status: fiber.StatusOK,
			body:   "proxied",
		},
		{
			name: "Do with a client of its own",
			do: func(c fiber.Ctx, target string) error {
				return Do(c, target, &fasthttp.Client{NoDefaultUserAgentHeader: true, DisablePathNormalizing: true})
			},
			status: fiber.StatusOK,
			body:   "proxied",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var addr string
			if tc.redirect {
				addr = createRedirectServer(t)
			} else {
				_, addr = createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString(tc.body) })
			}

			app := fiber.New()
			app.Get("/test", func(c fiber.Ctx) error { return tc.do(c, "http://"+addr) })

			resp := apptest.Do(t, app, "/test", fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true})
			require.Equal(t, tc.body, apptest.Body(t, resp))
			require.Equal(t, tc.status, resp.StatusCode)
			require.Equal(t, "/test", resp.Request.URL.String())
		})
	}
}

// Test_Proxy_DoRedirects_AliasedAddr covers an addr that aliases the request buffer (c.OriginalURL()):
// doActionWithPolicy clones it once, or SetRequestURI would corrupt a *url.URL still pointing into it.
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

	resp := apptest.Do(t, app, "/http://"+addr, fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true})
	require.Equal(t, "final", apptest.Body(t, resp))
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Proxy_DoTimeout_Timeout(t *testing.T) {
	// Intentionally not parallel: it measures a timeout, so it must not compete for CPU.
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(time.Second * 5)
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error { return DoTimeout(c, "http://"+addr, time.Second) })

	resp := apptest.Do(t, app, "/test", fiber.TestConfig{Timeout: 2 * time.Second, FailOnTimeout: true})
	require.Equal(t, fasthttp.ErrTimeout.Error(), apptest.Body(t, resp))
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "/test", resp.Request.URL.String())
}

func Test_Proxy_DoDeadline_PastDeadline(t *testing.T) {
	// Intentionally not parallel: it measures a deadline, so it must not compete for CPU.
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		time.Sleep(time.Second * 5)
		return c.SendString("proxied")
	})

	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error { return DoDeadline(c, "http://"+addr, time.Now().Add(2*time.Second)) })

	cfg := fiber.TestConfig{Timeout: time.Second, FailOnTimeout: true}
	_, err := app.Test(apptest.Req(fiber.MethodGet, "/test"), cfg)
	require.Equal(t, os.ErrDeadlineExceeded, err)
}

func Test_Proxy_Do_HTTP_Prefix_URL(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("hello world") })

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

	apptest.ExpectBody(t, app, "/http://"+addr, fiber.StatusOK, "hello world")
}

func Test_Proxy_Forward_Global_Client(t *testing.T) {
	// Intentionally not parallel: WithClient replaces the process-wide shared client.
	restoreGlobalProxyClient(t)
	ln := proxyTestListen(t)
	WithClient(&fasthttp.Client{NoDefaultUserAgentHeader: true, DisablePathNormalizing: true, MaxConnsPerHost: 123})
	loadedClient := sharedClient.Load()
	require.NotNil(t, loadedClient)
	require.Equal(t, 123, loadedClient.MaxConnsPerHost)

	app := fiber.New()
	app.Get("/test_global_client", func(c fiber.Ctx) error { return c.SendString("test_global_client") })

	addr := ln.Addr().String()
	app.Use(Forward("http://" + addr + "/test_global_client"))
	startServer(app, ln)

	proxyTestExpectGet(t, clientpkg.New(), "http://"+addr, "test_global_client")
}

func Test_Proxy_Forward_Local_Client(t *testing.T) {
	t.Parallel()
	ln := proxyTestListen(t)
	app := fiber.New()
	app.Get("/test_local_client", func(c fiber.Ctx) error { return c.SendString("test_local_client") })

	addr := ln.Addr().String()
	app.Use(Forward("http://"+addr+"/test_local_client", &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		Dial:                     fasthttp.Dial,
	}))
	startServer(app, ln)

	proxyTestExpectGet(t, clientpkg.New(), "http://"+addr, "test_local_client")
}

func Test_Proxy_WithClient_Nil_Panics(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "proxy: WithClient requires a non-nil *fasthttp.Client", func() { WithClient(nil) })
}

func Test_Proxy_SelectClient_NilGlobal(t *testing.T) {
	t.Parallel()
	selectedClient, err := selectClient(nil)
	require.ErrorIs(t, err, errNilGlobalProxyClient)
	require.Nil(t, selectedClient)
}

func Test_Proxy_NilClientOverride_AcrossHelpers(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("proxied") })

	tests := map[string]func(c fiber.Ctx) error{
		"Do":          func(c fiber.Ctx) error { return Do(c, "http://"+addr, nil) },
		"DoRedirects": func(c fiber.Ctx) error { return DoRedirects(c, "http://"+addr, 1, nil) },
		"DoDeadline": func(c fiber.Ctx) error {
			return DoDeadline(c, "http://"+addr, time.Now().Add(time.Second), nil)
		},
		"DoTimeout": func(c fiber.Ctx) error { return DoTimeout(c, "http://"+addr, time.Second, nil) },
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Get("/test", run)

			apptest.ExpectBody(t, app, "/test", fiber.StatusInternalServerError, errNilProxyClientOverride.Error())
		})
	}
}

func Test_Proxy_Domain_Forward_Local(t *testing.T) {
	t.Parallel()
	ln := proxyTestListen(t)
	app := fiber.New()

	ln1 := proxyTestListen(t) // the target server
	app1 := fiber.New()

	app1.Get("/test", func(c fiber.Ctx) error { return c.SendString("test_local_client:" + c.Query("query_test")) })

	proxyAddr := ln.Addr().String()
	targetAddr := ln1.Addr().String()
	localDomain := strings.Replace(proxyAddr, "127.0.0.1", "localhost", 1)
	app.Use(DomainForward(localDomain, "http://"+targetAddr, &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		Dial:                     fasthttp.Dial,
	}))
	startServer(app, ln)
	startServer(app1, ln1)

	proxyTestExpectGet(t, clientpkg.New(), "http://"+localDomain+"/test?query_test=true", "test_local_client:true")
}

func Test_Proxy_Balancer_ModifyRequestCanSetXRealIP(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString(c.Get("X-Real-IP")) })

	app := fiber.New()
	app.Use(Balancer(Config{
		Servers: []string{addr},
		ModifyRequest: func(c fiber.Ctx) error {
			c.Request().Header.Set("X-Real-IP", "203.0.113.7")
			return nil
		},
	}))

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr), apptest.WithHeader("X-Real-IP", "6.6.6.6"))

	apptest.ExpectBody(t, app, req, fiber.StatusOK, "203.0.113.7")
}

func Test_Proxy_ConnectionHeader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want string // the Connection header the client gets back
		keep bool   // Config.KeepConnectionHeader
	}{
		{name: "KeepConnectionHeader passes the upstream's through", want: "backend", keep: true},
		{name: "dropped by default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
				c.Set(fiber.HeaderConnection, "backend")
				return c.SendString("ok")
			})

			app := fiber.New()
			app.Use(Balancer(Config{Servers: []string{addr}, KeepConnectionHeader: tc.keep}))

			resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(addr),
				apptest.WithHeader(fiber.HeaderConnection, "keep-alive")))
			require.Equal(t, tc.want, resp.Header.Get(fiber.HeaderConnection))
		})
	}
}

// Test_Proxy_DomainForward_HostMatch pins which Host headers a DomainForward
// proxies and which it leaves to the handlers behind it. The upstream answers
// "proxied"; the route behind, when a row has one, answers its fallback.
func Test_Proxy_DomainForward_HostMatch(t *testing.T) {
	t.Parallel()
	type request struct {
		host   string
		body   string // not checked when empty
		status int
	}

	for _, tc := range []struct {
		name     string
		domain   string // the hostname the handler is configured with
		fallback string // what the route behind the handler answers; none is registered when empty
		requests []request
	}{
		// The host gate folds case per RFC 9110 §4.2.3 — an inbound Host header
		// with different casing than the configured hostname must still be
		// proxied, not silently passed through.
		{
			name:     "mixed-case Host is still proxied",
			domain:   "api.example.com",
			requests: []request{{host: "API.Example.com", status: fiber.StatusOK, body: "proxied"}},
		},
		// A Unicode hostname literal — the natural way to write a non-ASCII
		// domain in Go source — still matches the request a real client sends,
		// which is always the Punycode form (RFC 5891). Before this was fixed,
		// the rule silently never matched: every real request for the configured
		// domain fell through unproxied.
		{
			name:     "a Unicode-literal hostname matches the Punycode form on the wire",
			domain:   "münchen.example.com",
			requests: []request{{host: "xn--mnchen-3ya.example.com", status: fiber.StatusOK, body: "proxied"}},
		},
		// Folding preserves an explicit port on the configured hostname, so
		// DomainForward("münchen.example.com:8080", ...) keeps requiring that port
		// rather than silently dropping the restriction.
		{
			name:   "a folded hostname keeps its port",
			domain: "münchen.example.com:8080",
			requests: []request{
				{host: "xn--mnchen-3ya.example.com:8080", status: fiber.StatusOK, body: "proxied"},
				// a different port must still fall through unproxied
				{host: "xn--mnchen-3ya.example.com:9090", status: fiber.StatusNotFound},
			},
		},
		// Construction-time hostname folding keeps the brackets required around an
		// IPv6 literal when it has an explicit port (RFC 3986, section 3.2.2).
		{
			name:     "an IPv6 literal keeps its brackets",
			domain:   "[::1]:8080",
			fallback: "fallthrough",
			requests: []request{{host: "[::1]:8080", status: fiber.StatusOK, body: "proxied"}},
		},
		{
			name:     "a Host that does not match continues to the next handler",
			domain:   "api.example.com",
			fallback: "local",
			requests: []request{
				{host: "api.example.com", status: fiber.StatusOK, body: "proxied"},
				{host: "API.Example.com:8080", status: fiber.StatusOK, body: "proxied"},
				{host: "www.example.com", status: fiber.StatusOK, body: "local"},
				{host: "www.example.com:8080", status: fiber.StatusOK, body: "local"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("proxied") })

			app := fiber.New()
			app.Use(DomainForward(tc.domain, "http://"+addr))
			if tc.fallback != "" {
				app.Use(func(c fiber.Ctx) error { return c.SendString(tc.fallback) })
			}

			for _, r := range tc.requests {
				resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost(r.host)))
				require.Equal(t, r.status, resp.StatusCode, "Host %s", r.host)
				if r.body != "" {
					require.Equal(t, r.body, apptest.Body(t, resp), "Host %s", r.host)
				}
			}
		})
	}
}

// sendRaw parses raw as the request a client put on the wire and runs app's
// handler on it, returning the response body. Reading from the wire is what
// lets a test send a chunked body with a trailer section, which the request
// setters cannot express.
func sendRaw(t *testing.T, app *fiber.App, raw string) string {
	t.Helper()

	return string(serveParsed(t, app, proxyTestRequest(t), raw).Body())
}

// sendRawUnnormalized is sendRaw with the header names kept exactly as
// written, the way a front end translating HTTP/2 down to HTTP/1.1 leaves them.
func sendRawUnnormalized(t *testing.T, app *fiber.App, raw string) string {
	t.Helper()

	req := proxyTestRequest(t)
	req.Header.DisableNormalizing()
	return string(serveParsed(t, app, req, raw).Body())
}

// serveParsed reads raw into req and runs app's handler on the result, returning the response.
func serveParsed(t *testing.T, app *fiber.App, req *fasthttp.Request, raw string) *fasthttp.Response {
	t.Helper()

	proxyTestReadRequest(t, req, raw)

	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, nil, nil)
	app.Handler()(fctx)

	return &fctx.Response
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

// Test_Proxy_HopByHopHeadersStrippedWhateverTheirCase checks that the RFC 9110 §7.6.1 strip holds
// for names in lower case, as HTTP/2 and HTTP/3 require on the wire.
//
// Del matches the stored key byte for byte, so under DisableHeaderNormalizing
// it removed nothing: the hop-by-hop headers reached the upstream, and so did a
// field named in Connection, which is a peer smuggling a connection-scoped
// header through an intermediary required to drop it.
func Test_Proxy_HopByHopHeadersStrippedWhateverTheirCase(t *testing.T) {
	t.Parallel()
	addr, seen := proxyTestFieldLinesUpstream(t, "upstream", "Upgrade", "TE", "Keep-Alive", "Connection", "X-Smuggled")

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
	addr, seen := proxyTestFieldLinesUpstream(t, "upstream", realIPHeader)

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
	finalAddr, seen := proxyTestFieldLinesUpstream(t, "final", fiber.HeaderAuthorization, fiber.HeaderCookie)

	// The redirect has to cross to a different host, and that is decided by
	// comparing the authority as written rather than where it resolves — so
	// name the same listener "localhost" instead of binding a second loopback
	// address. 127.0.0.2 is bound by default only on Linux.
	redirectAddr := proxyTestRedirectUpstream(t, "http://"+loopbackAlias(t, finalAddr)+"/")

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(func(c fiber.Ctx) error { return DoRedirects(c, "http://"+redirectAddr, 3) })

	body := sendRawUnnormalized(t, app, "GET / HTTP/1.1\r\nHost: front\r\n"+
		"authorization: Bearer caller-token\r\n"+
		"cookie: session=caller\r\n\r\n")
	require.Equal(t, "final", body)

	require.Empty(t, <-seen, "credentials must not follow a redirect off the host they were sent to")
}

// Test_Proxy_CrossHostStripsEveryCredentialHeader checks that the set dropped on a redirect off the addressed
// host is the one the client package drops: Cookie2 carries a session exactly as Cookie does, and a list that
// covers one but not the other leaks through the gap. A header outside the set goes on.
func Test_Proxy_CrossHostStripsEveryCredentialHeader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		headers map[string]string // what the caller sends
		name    string
		want    []string // what the target reports of them
	}{
		{
			name: "every credential header",
			headers: map[string]string{
				fiber.HeaderAuthorization: "Bearer caller-token", fiber.HeaderProxyAuthorization: "Basic proxy",
				fiber.HeaderProxyAuthenticate: "Basic realm=x", fiber.HeaderWWWAuthenticate: "Basic realm=y",
				fiber.HeaderCookie: "session=caller", "Cookie2": `$Version="1"`,
			},
		},
		{
			name:    "a header outside the set",
			headers: map[string]string{fiber.HeaderAuthorization: "Bearer caller-token", "X-Api-Key": "key"},
			want:    []string{"X-Api-Key=key"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			finalAddr, seen := proxyTestFieldLinesUpstream(t, "final",
				fiber.HeaderAuthorization, fiber.HeaderProxyAuthorization, fiber.HeaderProxyAuthenticate,
				fiber.HeaderWWWAuthenticate, fiber.HeaderCookie, "Cookie2", "X-Api-Key")
			redirectAddr := proxyTestRedirectUpstream(t, "http://"+loopbackAlias(t, finalAddr)+"/")

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error { return DoRedirects(c, "http://"+redirectAddr, 3) })

			req := apptest.Req(fiber.MethodGet, "/")
			for name, value := range tc.headers {
				req.Header.Set(name, value)
			}
			apptest.Expect(t, app, req, fiber.StatusOK)

			require.ElementsMatch(t, tc.want, <-seen, "only the headers outside the set follow the redirect")
		})
	}
}

func Test_Proxy_DoRedirects_DropsListedHeadersOffTheOrigin(t *testing.T) {
	// Intentionally not parallel: it sets a process-wide security policy.
	policy := proxyTestAllowPrivatePolicy() // replaces TestMain's relaxed policy
	policy.RedirectSensitiveHeaders = []string{"x-api-key"}
	withSecurityPolicyForTest(t, policy)

	finalAddr, seen := proxyTestFieldLinesUpstream(t, "final", "X-Api-Key", "X-Auth-Token", fiber.HeaderAuthorization)
	redirectAddr := proxyTestRedirectUpstream(t, "http://"+loopbackAlias(t, finalAddr)+"/")

	var own string
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		err := DoRedirects(c, "http://"+redirectAddr, 3)
		own = c.Get("X-Api-Key") + "|" + c.Get("X-Auth-Token") + "|" + c.Get(fiber.HeaderAuthorization)
		return err
	})

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAuthorization, "Bearer caller-token"),
		apptest.WithHeader("X-Api-Key", "key"), apptest.WithHeader("X-Auth-Token", "tok"))
	apptest.Expect(t, app, req, fiber.StatusOK)

	require.Equal(t, []string{"X-Auth-Token=tok"}, <-seen, "only the header nobody named reaches the target")
	require.Equal(t, "key|tok|Bearer caller-token", own, "the handler's request is untouched")
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
	addr := listenUpstream(t, upstream)

	cli := &fasthttp.Client{DisableHeaderNamesNormalizing: true}

	// ...while the app takes the default, so the two settings disagree.
	app := fiber.New()
	app.Use(Forward("http://"+addr, cli))

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)

	for _, name := range []string{fiber.HeaderKeepAlive, fiber.HeaderUpgrade, fiber.HeaderTE} {
		require.Empty(t, resp.Header.Values(name),
			"%s must not reach the client whatever case the upstream wrote it in", name)
	}
}

func Test_Proxy_Forward_RestoresHost(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString("forwarded") })

	app := fiber.New()
	seen := make(chan string, 1)
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		seen <- c.Hostname()
		return err
	})
	app.Use(Forward("http://" + addr))

	apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", apptest.WithHost("public.example.com")), fiber.StatusOK)
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

func Test_Proxy_NewHostClient(t *testing.T) {
	t.Parallel()
	cfg := Config{
		MaxConnsPerHost:     7,
		ReadBufferSize:      2048,
		WriteBufferSize:     3072,
		MaxResponseBodySize: 4096,
		DialDualStack:       true,
	}
	strict := DefaultSecurityPolicy()
	relaxed := DefaultSecurityPolicy()
	relaxed.AllowPrivateIPs = true

	t.Run("carries the config", func(t *testing.T) {
		t.Parallel()
		hc := newHostClient(mustParseTestURL(t, "http://upstream.example:8080"), &cfg, strict)
		require.Equal(t, "upstream.example:8080", hc.Addr)
		require.False(t, hc.IsTLS)
		require.Equal(t, 7, hc.MaxConns)
		require.Equal(t, 2048, hc.ReadBufferSize)
		require.Equal(t, 3072, hc.WriteBufferSize)
		require.Equal(t, 4096, hc.MaxResponseBodySize)
		require.True(t, hc.DialDualStack)
		require.True(t, hc.DisablePathNormalizing)
		require.True(t, hc.NoDefaultUserAgentHeader)
	})

	t.Run("https gets TLS with a floor", func(t *testing.T) {
		t.Parallel()
		hc := newHostClient(mustParseTestURL(t, "https://upstream.example"), &cfg, strict)
		require.True(t, hc.IsTLS)
		require.NotNil(t, hc.TLSConfig)
		require.Equal(t, uint16(tls.VersionTLS12), hc.TLSConfig.MinVersion)
	})

	t.Run("guards the dial unless private IPs are allowed", func(t *testing.T) {
		t.Parallel()
		guarded := newHostClient(mustParseTestURL(t, "http://upstream.example"), &cfg, strict)
		require.NotNil(t, guarded.Dial)
		_, err := guarded.Dial("127.0.0.1:80")
		require.ErrorIs(t, err, ErrUpstreamHostBlocked)

		open := newHostClient(mustParseTestURL(t, "http://upstream.example"), &cfg, relaxed)
		require.Nil(t, open.Dial)
		require.Nil(t, open.DialTimeout)
	})

	// fasthttp hands the timeout a request leaves for the connect only to a DialTimeout.
	t.Run("guards the dial timeout too", func(t *testing.T) {
		t.Parallel()
		guarded := newHostClient(mustParseTestURL(t, "http://upstream.example"), &cfg, strict)
		require.NotNil(t, guarded.DialTimeout)
		for _, timeout := range []time.Duration{0, time.Second} {
			_, err := guarded.DialTimeout("127.0.0.1:80", timeout)
			require.ErrorIs(t, err, ErrUpstreamHostBlocked, "timeout %v", timeout)
		}
	})

	// Only fasthttp's own dialer adds the port, and the guard's does not.
	t.Run("adds the port its scheme defaults to", func(t *testing.T) {
		t.Parallel()
		for raw, want := range map[string]string{
			"http://upstream.example":           "upstream.example:80",
			"https://upstream.example":          "upstream.example:443",
			"http://upstream.example:8080":      "upstream.example:8080",
			"https://upstream.example:8443":     "upstream.example:8443",
			"http://[2001:db8::1]":              "[2001:db8::1]:80",
			"https://[2001:db8::1]":             "[2001:db8::1]:443",
			"http://[2001:db8::1]:8080":         "[2001:db8::1]:8080",
			"http://203.0.113.5":                "203.0.113.5:80",
			"https://upstream.example:443":      "upstream.example:443",
			"http://upstream.example:80":        "upstream.example:80",
			"https://[2001:db8::1]:8443":        "[2001:db8::1]:8443",
			"http://upstream.example.:8080":     "upstream.example.:8080",
			"http://xn--bcher-kva.example/":     "xn--bcher-kva.example:80",
			"https://xn--bcher-kva.example:444": "xn--bcher-kva.example:444",
		} {
			hc := newHostClient(mustParseTestURL(t, raw), &cfg, strict)
			require.Equal(t, want, hc.Addr, raw)
		}
	})
}

func Test_Proxy_Balancer_MaxResponseBodySize(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error { return c.SendString(strings.Repeat("x", 64)) })

	for name, tc := range map[string]struct {
		limit   int
		wantErr bool
	}{
		"over the limit":   {limit: 16, wantErr: true},
		"within the limit": {limit: 128},
		"unbounded":        {limit: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var handled error
			app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
				handled = err
				return c.SendStatus(fiber.StatusBadGateway)
			}})
			app.Use(Balancer(Config{Servers: []string{addr}, MaxResponseBodySize: tc.limit}))

			resp := apptest.Do(t, app, "/")
			if tc.wantErr {
				require.ErrorIs(t, handled, fasthttp.ErrBodyTooLarge)
				require.Equal(t, fiber.StatusBadGateway, resp.StatusCode)
				return
			}
			require.NoError(t, handled)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}

// Headers larger than WriteBufferSize must still arrive whole.
func Test_Proxy_Balancer_WriteBufferSize(t *testing.T) {
	t.Parallel()
	_, addr := createProxyTestServerIPv4(t, func(c fiber.Ctx) error {
		return c.SendString(strconv.Itoa(len(c.Get(fiber.HeaderCookie))))
	})

	app := fiber.New()
	app.Use(Balancer(Config{Servers: []string{addr}, WriteBufferSize: 256}))

	cookie := "session=" + strings.Repeat("a", 2000)
	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderCookie, cookie))

	apptest.ExpectBody(t, app, req, fiber.StatusOK, strconv.Itoa(len(cookie)))
}
