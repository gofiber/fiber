package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// These benchmarks are the performance baseline: each targets a hot path.
//
// All benchmarks call b.ReportAllocs() so allocation regressions are
// caught even when ns/op is noise. They use the testing.B.Loop form
// (https://go.dev/blog/testing-b-loop): the first b.Loop() call resets
// the timer, so per-benchmark setup before the loop is excluded from
// timing without an explicit b.ResetTimer, and the loop body is kept
// safe from dead-code elimination.

func BenchmarkCurrentSecurityPolicy(b *testing.B) {
	prev := WithSecurityPolicy(DefaultSecurityPolicy())
	b.Cleanup(func() { WithSecurityPolicy(prev) })
	b.ReportAllocs()
	for b.Loop() {
		_ = currentSecurityPolicy()
	}
}

func BenchmarkResolvePolicy_Nil(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = resolvePolicy(nil)
	}
}

func BenchmarkResolvePolicy_Override(b *testing.B) {
	override := DefaultSecurityPolicy()
	b.ReportAllocs()
	for b.Loop() {
		_ = resolvePolicy(&override)
	}
}

func BenchmarkSchemeAllowed_HTTPS(b *testing.B) {
	allowed := []string{schemeHTTP, schemeHTTPS}
	b.ReportAllocs()
	for b.Loop() {
		_ = schemeAllowed(schemeHTTPS, allowed)
	}
}

func BenchmarkSchemeAllowed_EmptyAllowlist(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = schemeAllowed(schemeHTTPS, nil)
	}
}

func BenchmarkValidateUpstream_IPLiteral(b *testing.B) {
	policy := DefaultSecurityPolicy()
	const addr = "http://203.0.113.5:8080/api/v1/widgets"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := validateUpstream(addr, policy); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateUpstreamForBalancer_IPLiteral(b *testing.B) {
	policy := DefaultSecurityPolicy()
	const addr = "http://203.0.113.5:8080"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := validateUpstreamForBalancer(addr, policy); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolveRedirect_HTTPSDowngradeBlocked(b *testing.B) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	location := []byte("http://example.com/landing")
	b.ReportAllocs()
	var err error
	for b.Loop() {
		_, err = resolveRedirect("https://example.com/", location, policy)
	}
	// Anchor: the loop must have measured the blocked downgrade path.
	if !errors.Is(err, ErrRedirectDowngrade) {
		b.Fatalf("expected ErrRedirectDowngrade, got %v", err)
	}
}

func BenchmarkResolveRedirect_AllowedAcrossOrigin(b *testing.B) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	location := []byte("https://other.example/landing")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := resolveRedirect("https://example.com/", location, policy); err != nil {
			b.Fatal(err)
		}
	}
}

// seedNoConnectionRequest installs the headers exercised by
// BenchmarkStripHopByHop_NoConnection. Kept as a tiny helper so the
// re-seed step inside the timed loop is a single call.
func seedNoConnectionRequest(req *fasthttp.Request) {
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("User-Agent", "bench/1.0")
}

// seedWithConnectionRequest installs the headers exercised by
// BenchmarkStripHopByHop_WithConnection, including a Connection field
// that lists a non-standard hop header so the stripping loop has real
// work to do.
func seedWithConnectionRequest(req *fasthttp.Request) {
	req.Header.Set(fiber.HeaderConnection, "X-Custom-Hop, Keep-Alive")
	req.Header.Set("X-Custom-Hop", "drop")
	req.Header.Set(fiber.HeaderProxyAuthorization, "Basic Zm9vOmJhcg==")
}

func BenchmarkStripHopByHop_NoConnection(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	b.ReportAllocs()
	for b.Loop() {
		// Re-seed outside the timed section every iteration: without
		// this, the first call removes the hop-by-hop headers and the
		// rest of the loop measures a near no-op.
		b.StopTimer()
		req.Reset()
		seedNoConnectionRequest(req)
		b.StartTimer()
		stripHopByHopRequestHeaders(req, true)
	}
}

func BenchmarkStripHopByHop_WithConnection(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		req.Reset()
		seedWithConnectionRequest(req)
		b.StartTimer()
		stripHopByHopRequestHeaders(req, true)
	}
}

func BenchmarkJoinUpstreamPath_RootBase(b *testing.B) {
	base, err := url.Parse("http://upstream.example")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = joinUpstreamPath(base, "/api/v1/widgets?ids=1,2,3")
	}
}

func BenchmarkJoinUpstreamPath_PrefixBase(b *testing.B) {
	base, err := url.Parse("http://upstream.example/service")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = joinUpstreamPath(base, "/api/v1/widgets")
	}
}

func BenchmarkIsBlockedIP_PublicV4(b *testing.B) {
	ip := net.ParseIP("203.0.113.5")
	b.ReportAllocs()
	for b.Loop() {
		_ = isBlockedIP(ip)
	}
}

// BenchmarkFollowRedirects_NoRedirect isolates the followRedirects entry
// cost — initial URL handling, request setup, and one cli.Do call that
// returns 204 (no Location). The URL arrives validated, so no validateUpstream is measured.
func BenchmarkFollowRedirects_NoRedirect(b *testing.B) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	prev := WithSecurityPolicy(policy)
	b.Cleanup(func() { WithSecurityPolicy(prev) })

	cli := &fasthttp.Client{
		Transport: noopRoundTripper{},
	}

	initialURL, err := url.Parse("http://203.0.113.5:8080/api/v1/widgets")
	if err != nil {
		b.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	b.ReportAllocs()
	for b.Loop() {
		req.Reset()
		req.SetRequestURI(initialURL.String())
		req.Header.SetMethod(fasthttp.MethodGet)
		if err := followRedirects(cli, req, resp, 3, initialURL, policy); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDomainForward_HostMatchPath measures the per-request work
// that happens once DomainForward's host check matches: the handler
// closure executes from its first statement through the call into
// doActionWithPolicy. The action callback short-circuits so the
// downstream action work (cli.Do) is not measured — this isolates the
// per-request work from the one-time construction work.
func BenchmarkDomainForward_HostMatchPath(b *testing.B) {
	benchNoopUpstream(b)

	app := fiber.New()
	app.Use(DomainForward("api.example", "http://203.0.113.5:8080"))
	req := newReqWithHost("api.example", "/v1/widgets?q=1")
	b.ReportAllocs()
	for b.Loop() {
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 1})
		if err != nil {
			b.Fatal(err)
		}
		// Close the body with the timer stopped: every iteration
		// allocates an *http.Response, and leaking the body retains
		// buffers that would skew B/op the benchmark is trying to
		// track.
		b.StopTimer()
		if cerr := resp.Body.Close(); cerr != nil {
			b.Fatal(cerr)
		}
		b.StartTimer()
	}
}

// benchNoopUpstream points the runtime helpers at an upstream that answers 204 without dialing.
func benchNoopUpstream(b *testing.B) {
	b.Helper()
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	withSecurityPolicyForTest(b, policy)

	restoreGlobalProxyClient(b)
	sharedClient.Store(&fasthttp.Client{Transport: noopRoundTripper{}})
}

// benchProxyPath sends a GET through app per iteration on a bare fasthttp context; the upstream must answer 204.
func benchProxyPath(b *testing.B, app *fiber.App, host, target string) {
	b.Helper()
	var req fasthttp.Request
	req.SetRequestURI(target)
	req.Header.SetHost(host)
	handler := app.Handler()
	remote := &net.TCPAddr{IP: net.IPv4(203, 0, 113, 9), Port: 40000}

	var fctx fasthttp.RequestCtx
	b.ReportAllocs()
	for b.Loop() {
		fctx.Init(&req, remote, nil)
		handler(&fctx)
		if got := fctx.Response.StatusCode(); got != fasthttp.StatusNoContent {
			b.Fatalf("status = %d, want %d", got, fasthttp.StatusNoContent)
		}
	}
}

// BenchmarkBalancer_ProxyPath measures a request through Balancer to an upstream that never dials.
func BenchmarkBalancer_ProxyPath(b *testing.B) {
	lbc := &fasthttp.LBClient{
		Timeout: time.Second,
		Clients: []fasthttp.BalancingClient{
			&fasthttp.HostClient{Addr: "203.0.113.5:8080", Transport: noopRoundTripper{}},
		},
	}
	app := fiber.New()
	app.Use(Balancer(Config{Client: lbc}))
	benchProxyPath(b, app, "api.example", "/v1/widgets?q=1")
}

// BenchmarkForward_ProxyPath measures a request through Forward.
func BenchmarkForward_ProxyPath(b *testing.B) {
	benchNoopUpstream(b)

	app := fiber.New()
	app.Use(Forward("http://203.0.113.5:8080/v1/widgets"))
	benchProxyPath(b, app, "api.example", "/anything")
}

// BenchmarkBalancerForward_ProxyPath measures a request through BalancerForward.
func BenchmarkBalancerForward_ProxyPath(b *testing.B) {
	benchNoopUpstream(b)

	app := fiber.New()
	app.Use(BalancerForward([]string{"http://203.0.113.5:8080", "http://203.0.113.6:8080"}))
	benchProxyPath(b, app, "api.example", "/v1/widgets?q=1")
}

func BenchmarkURLRoundrobinGet(b *testing.B) {
	pool := []*url.URL{
		{Scheme: schemeHTTP, Host: "a.example"},
		{Scheme: schemeHTTP, Host: "b.example"},
		{Scheme: schemeHTTP, Host: "c.example"},
	}
	b.Run("atomic", func(b *testing.B) {
		r := &urlRoundrobin{pool: pool}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = r.get()
			}
		})
	})
	b.Run("mutex", func(b *testing.B) {
		r := &mutexURLRoundrobin{pool: pool}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = r.get()
			}
		})
	})
}

type mutexURLRoundrobin struct {
	pool []*url.URL

	current int
	sync.Mutex
}

func (r *mutexURLRoundrobin) get() *url.URL {
	r.Lock()
	defer r.Unlock()

	if r.current >= len(r.pool) {
		r.current %= len(r.pool)
	}
	result := r.pool[r.current]
	r.current++
	return result
}

// noopRoundTripper is the minimum surface for fasthttp.RoundTripper used
// by BenchmarkDomainForward_HostMatchPath. It returns 204 No Content
// without touching the network.
type noopRoundTripper struct{}

func (noopRoundTripper) RoundTrip(_ *fasthttp.HostClient, _ *fasthttp.Request, resp *fasthttp.Response) (bool, error) {
	resp.Reset()
	resp.Header.SetStatusCode(fasthttp.StatusNoContent)
	return false, nil
}

// newReqWithHost builds an http.Request with the given Host header set
// directly so DomainForward's host-match branch fires.
func newReqWithHost(host, target string) *http.Request {
	req := httptest.NewRequest(fiber.MethodGet, target, http.NoBody)
	req.Host = host
	return req
}
