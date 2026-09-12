package extractors

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// A JWT-sized token68 credential.
const benchToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P"

// benchSink keeps the results alive so nothing measured is optimized away.
var benchSink struct {
	err    error
	value  string
	result Result
	source Source
	ok     bool
}

func newBenchCtx(b *testing.B, cfg ...fiber.Config) fiber.Ctx {
	b.Helper()
	app := fiber.New(cfg...)
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	b.Cleanup(func() { app.ReleaseCtx(c) })
	return c
}

func benchExtract(b *testing.B, e Extractor, c fiber.Ctx) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		benchSink.value, benchSink.err = e.Extract(c)
	}
}

func benchResolve(b *testing.B, e Extractor, c fiber.Ctx) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		benchSink.result, benchSink.err = Resolve(e, c)
	}
}

// benchExtractFresh resets the request's user values after every extraction,
// as fasthttp does between requests, so what an extractor keeps in Locals is
// paid for per request rather than amortized over a reused context.
func benchExtractFresh(b *testing.B, e Extractor, c fiber.Ctx) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		benchSink.value, benchSink.err = e.Extract(c)
		c.RequestCtx().ResetUserValues()
	}
}

// benchResolveFresh is benchExtractFresh for the source-aware entry point.
func benchResolveFresh(b *testing.B, e Extractor, c fiber.Ctx) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		benchSink.result, benchSink.err = Resolve(e, c)
		c.RequestCtx().ResetUserValues()
	}
}

func benchChain3() Extractor {
	return Chain(FromHeader("X-API-Key"), FromCookie("api_key"), FromQuery("api_key"))
}

// --- leaves -----------------------------------------------------------------

func Benchmark_Extractor_FromHeader_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Miss(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-Other", "x")
	benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_ManyHeaders(b *testing.B) {
	c := newBenchCtx(b)
	for _, h := range []string{
		"Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Referer",
		"Sec-Fetch-Site", "Sec-Fetch-Mode", "Origin", "X-Request-Id", "X-Forwarded-For",
	} {
		c.Request().Header.Set(h, "value")
	}
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_NoNormalize(b *testing.B) {
	c := newBenchCtx(b, fiber.Config{DisableHeaderNormalizing: true})
	c.Request().Header.DisableNormalizing()
	c.Request().Header.Set("x-api-key", benchToken)
	benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromHeader_Hit_Immutable(b *testing.B) {
	c := newBenchCtx(b, fiber.Config{Immutable: true})
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtract(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_FromAuthHeader_Bearer_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+benchToken)
	benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_Basic_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA==")
	benchExtract(b, FromAuthHeader("Basic"), c)
}

func Benchmark_Extractor_FromAuthHeader_Miss(b *testing.B) {
	c := newBenchCtx(b)
	benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_WrongScheme(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set(fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA==")
	benchExtract(b, FromAuthHeader("Bearer"), c)
}

func Benchmark_Extractor_FromAuthHeader_NoScheme(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+benchToken)
	benchExtract(b, FromAuthHeader(""), c)
}

func Benchmark_Extractor_FromCookie_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.SetCookie("session_id", "abc123")
	c.Request().Header.SetCookie("other", "x")
	benchExtract(b, FromCookie("session_id"), c)
}

func Benchmark_Extractor_FromCookie_Miss(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.SetCookie("other", "x")
	benchExtract(b, FromCookie("session_id"), c)
}

func Benchmark_Extractor_FromQuery_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?format=json&token=abc123")
	benchExtract(b, FromQuery("token"), c)
}

func Benchmark_Extractor_FromQuery_Miss(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?format=json")
	benchExtract(b, FromQuery("token"), c)
}

func Benchmark_Extractor_FromForm_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.SetMethod(fiber.MethodPost)
	c.Request().Header.SetContentType(fiber.MIMEApplicationForm)
	c.Request().SetBodyString("username=john&token=abc123")
	benchExtract(b, FromForm("token"), c)
}

func Benchmark_Extractor_FromForm_Miss(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.SetMethod(fiber.MethodPost)
	c.Request().Header.SetContentType(fiber.MIMEApplicationForm)
	c.Request().SetBodyString("username=john")
	benchExtract(b, FromForm("token"), c)
}

// benchParam runs the loop inside a matched route, where route parameters are
// the only place they exist.
func benchParam(b *testing.B, uri string) {
	b.Helper()
	app := fiber.New()
	ext := FromParam("id")
	ran := false
	app.Get("/users/:id", func(c fiber.Ctx) error {
		ran = true
		b.ReportAllocs()
		for b.Loop() {
			benchSink.value, benchSink.err = ext.Extract(c)
		}
		return nil
	})
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI(uri)
	app.Handler()(fctx)
	if !ran {
		b.Fatal("route did not match")
	}
}

func Benchmark_Extractor_FromParam_Hit(b *testing.B) {
	benchParam(b, "/users/abc123")
}

func Benchmark_Extractor_FromParam_Hit_Escaped(b *testing.B) {
	benchParam(b, "/users/abc%20123")
}

func Benchmark_Extractor_FromCustom_Hit(b *testing.B) {
	c := newBenchCtx(b)
	benchExtract(b, FromCustom("k", func(fiber.Ctx) (string, error) { return "v", nil }), c)
}

// --- chains -----------------------------------------------------------------

func Benchmark_Extractor_Chain1_Hit(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtract(b, Chain(FromHeader("X-API-Key")), c)
}

func Benchmark_Extractor_Chain3_HitFirst(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_Chain3_HitLast(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_Chain3_Miss(b *testing.B) {
	c := newBenchCtx(b)
	benchExtract(b, benchChain3(), c)
}

func Benchmark_Extractor_ChainNested_HitInner(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
	benchExtract(b, outer, c)
}

// The locals a real request accumulates before the extractor runs; every
// request-local lookup scans past them.
func Benchmark_Extractor_Chain3_HitFirst_WithLocals(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	for i := range 5 {
		c.Locals(i, "value")
	}
	benchExtract(b, benchChain3(), c)
}

// --- source-aware -------------------------------------------------------------

func Benchmark_Extractor_Resolve_Leaf(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchResolve(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_Resolve_Chain3_HitFirst(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchResolve(b, benchChain3(), c)
}

func Benchmark_Extractor_Resolve_Chain3_HitLast(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	benchResolve(b, benchChain3(), c)
}

func Benchmark_Extractor_Resolve_Chain3_Miss(b *testing.B) {
	c := newBenchCtx(b)
	benchResolve(b, benchChain3(), c)
}

func Benchmark_Extractor_Resolve_Nested_HitInner(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
	benchResolve(b, outer, c)
}

func Benchmark_Extractor_Resolve_BareChain_HitLast(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	bare := Extractor{
		Chain:  []Extractor{FromHeader("X-API-Key"), FromCookie("api_key"), FromQuery("api_key")},
		Source: SourceHeader,
	}
	benchResolve(b, bare, c)
}

// --- fresh request -----------------------------------------------------------

func Benchmark_Extractor_FromHeader_Hit_FreshRequest(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtractFresh(b, FromHeader("X-API-Key"), c)
}

func Benchmark_Extractor_Chain1_Hit_FreshRequest(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchExtractFresh(b, Chain(FromHeader("X-API-Key")), c)
}

func Benchmark_Extractor_Chain3_HitLast_FreshRequest(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().SetRequestURI("/api?api_key=abc123")
	benchExtractFresh(b, benchChain3(), c)
}

func Benchmark_Extractor_Resolve_Chain3_HitFirst_FreshRequest(b *testing.B) {
	c := newBenchCtx(b)
	c.Request().Header.Set("X-API-Key", benchToken)
	benchResolveFresh(b, benchChain3(), c)
}

// --- token68 ------------------------------------------------------------------

func Benchmark_Extractor_Token68_JWT(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSink.ok = isValidToken68(benchToken)
	}
}
