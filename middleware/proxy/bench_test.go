package proxy

import (
	"net"
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
