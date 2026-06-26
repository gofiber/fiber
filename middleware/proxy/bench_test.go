package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// These benchmarks are the baseline cited by PERF_PLAN.md. Each one
// targets a specific hot path identified during the perf survey so
// subsequent changes can produce a benchstat-grade before/after.
//
// All benchmarks call b.ReportAllocs() so allocation regressions are
// caught even when ns/op is noise.

func BenchmarkCurrentSecurityPolicy(b *testing.B) {
	prev := WithSecurityPolicy(DefaultSecurityPolicy())
	b.Cleanup(func() { WithSecurityPolicy(prev) })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = currentSecurityPolicy()
	}
}

func BenchmarkResolvePolicy_Nil(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolvePolicy(nil)
	}
}

func BenchmarkResolvePolicy_Override(b *testing.B) {
	override := DefaultSecurityPolicy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resolvePolicy(&override)
	}
}

func BenchmarkSchemeAllowed_HTTPS(b *testing.B) {
	allowed := []string{schemeHTTP, schemeHTTPS}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schemeAllowed(schemeHTTPS, allowed)
	}
}

func BenchmarkSchemeAllowed_EmptyAllowlist(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = schemeAllowed(schemeHTTPS, nil)
	}
}

func BenchmarkValidateUpstream_IPLiteral(b *testing.B) {
	policy := DefaultSecurityPolicy()
	const addr = "http://203.0.113.5:8080/api/v1/widgets"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := validateUpstream(addr, policy); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateUpstreamForBalancer_IPLiteral(b *testing.B) {
	policy := DefaultSecurityPolicy()
	const addr = "http://203.0.113.5:8080"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolveRedirect("https://example.com/", location, policy) //nolint:errcheck // bench
	}
}

func BenchmarkResolveRedirect_AllowedAcrossOrigin(b *testing.B) {
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	location := []byte("https://other.example/landing")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := resolveRedirect("https://example.com/", location, policy); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStripHopByHop_NoConnection(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("User-Agent", "bench/1.0")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stripHopByHopRequestHeaders(req)
	}
}

func BenchmarkStripHopByHop_WithConnection(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.Set(fiber.HeaderConnection, "X-Custom-Hop, Keep-Alive")
	req.Header.Set("X-Custom-Hop", "drop")
	req.Header.Set(fiber.HeaderProxyAuthorization, "Basic Zm9vOmJhcg==")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stripHopByHopRequestHeaders(req)
	}
}

func BenchmarkJoinUpstreamPath_RootBase(b *testing.B) {
	base, err := url.Parse("http://upstream.example")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = joinUpstreamPath(base, "/api/v1/widgets?ids=1,2,3")
	}
}

func BenchmarkJoinUpstreamPath_PrefixBase(b *testing.B) {
	base, err := url.Parse("http://upstream.example/service")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = joinUpstreamPath(base, "/api/v1/widgets")
	}
}

func BenchmarkIsBlockedIP_PublicV4(b *testing.B) {
	ip := net.ParseIP("203.0.113.5")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isBlockedIP(ip)
	}
}

// BenchmarkDomainForward_HostMatchPath measures the per-request work
// that happens once DomainForward's host check matches: the handler
// closure executes from its first statement through the call into
// doActionWithPolicy. The action callback short-circuits so the
// downstream action work (cli.Do) is not measured — this isolates the
// constructor-vs-per-request split that P2 targets.
func BenchmarkDomainForward_HostMatchPath(b *testing.B) {
	// Construction-time validation requires a passing upstream.
	policy := DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	prev := WithSecurityPolicy(policy)
	b.Cleanup(func() { WithSecurityPolicy(prev) })

	// Stash & restore the global proxy client so the benchmark uses a
	// no-op transport instead of dialing.
	noopClient := &fasthttp.Client{
		Transport: noopRoundTripper{},
	}
	prevClient := client.Swap(noopClient)
	b.Cleanup(func() {
		if prevClient != nil {
			client.Store(prevClient)
		}
	})

	app := fiber.New()
	app.Use(DomainForward("api.example", "http://203.0.113.5:8080"))
	req := newReqWithHost("api.example", "/v1/widgets?q=1")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 1})
		if err != nil {
			b.Fatal(err)
		}
		_ = resp
	}
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
