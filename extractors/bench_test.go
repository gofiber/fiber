package extractors

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/valyala/fasthttp"
)

// A JWT-sized token68 credential.
const benchToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9P"

// benchSink keeps the results alive so nothing measured is optimized away.
var benchSink struct {
	err    error
	value  string
	result Result
	ok     bool
}

func newBenchCtx(b *testing.B, cfg ...fiber.Config) fiber.Ctx {
	b.Helper()
	return apptest.Ctx(b, fiber.New(cfg...))
}

// newBenchHeaderCtx returns a context whose request carries the header name: value.
func newBenchHeaderCtx(b *testing.B, name, value string) fiber.Ctx {
	b.Helper()
	c := newBenchCtx(b)
	c.Request().Header.Set(name, value)
	return c
}

// newBenchURICtx returns a context whose request is for uri.
func newBenchURICtx(b *testing.B, uri string) fiber.Ctx {
	b.Helper()
	c := newBenchCtx(b)
	c.Request().SetRequestURI(uri)
	return c
}

// newBenchKeyCtx returns a context whose request carries benchToken in X-API-Key.
func newBenchKeyCtx(b *testing.B) fiber.Ctx {
	b.Helper()
	return newBenchHeaderCtx(b, "X-API-Key", benchToken)
}

// newBenchQueryCtx returns a context whose request carries api_key=abc123 in its
// query string.
func newBenchQueryCtx(b *testing.B) fiber.Ctx {
	b.Helper()
	return newBenchURICtx(b, "/api?api_key=abc123")
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
	benchExtract(b, FromHeader("X-API-Key"), newBenchKeyCtx(b))
}

func Benchmark_Extractor_FromHeader_Miss(b *testing.B) {
	benchExtract(b, FromHeader("X-API-Key"), newBenchHeaderCtx(b, "X-Other", "x"))
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
	benchExtract(b, FromAuthHeader("Bearer"), newBenchHeaderCtx(b, fiber.HeaderAuthorization, "Bearer "+benchToken))
}

func Benchmark_Extractor_FromAuthHeader_Basic_Hit(b *testing.B) {
	benchExtract(b, FromAuthHeader("Basic"),
		newBenchHeaderCtx(b, fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA=="))
}

func Benchmark_Extractor_FromAuthHeader_Miss(b *testing.B) {
	benchExtract(b, FromAuthHeader("Bearer"), newBenchCtx(b))
}

func Benchmark_Extractor_FromAuthHeader_WrongScheme(b *testing.B) {
	benchExtract(b, FromAuthHeader("Bearer"),
		newBenchHeaderCtx(b, fiber.HeaderAuthorization, "Basic dXNlcjpwYXNzd29yZA=="))
}

func Benchmark_Extractor_FromAuthHeader_NoScheme(b *testing.B) {
	benchExtract(b, FromAuthHeader(""), newBenchHeaderCtx(b, fiber.HeaderAuthorization, "Bearer "+benchToken))
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
	benchExtract(b, FromQuery("token"), newBenchURICtx(b, "/api?format=json&token=abc123"))
}

func Benchmark_Extractor_FromQuery_Miss(b *testing.B) {
	benchExtract(b, FromQuery("token"), newBenchURICtx(b, "/api?format=json"))
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
	benchExtract(b, FromCustom("k", func(fiber.Ctx) (string, error) { return "v", nil }), newBenchCtx(b))
}

// --- chains -----------------------------------------------------------------

func Benchmark_Extractor_Chain1_Hit(b *testing.B) {
	benchExtract(b, Chain(FromHeader("X-API-Key")), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Chain3_HitFirst(b *testing.B) {
	benchExtract(b, benchChain3(), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Chain3_HitLast(b *testing.B) {
	benchExtract(b, benchChain3(), newBenchQueryCtx(b))
}

func Benchmark_Extractor_Chain3_Miss(b *testing.B) {
	benchExtract(b, benchChain3(), newBenchCtx(b))
}

func Benchmark_Extractor_ChainNested_HitInner(b *testing.B) {
	outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
	benchExtract(b, outer, newBenchQueryCtx(b))
}

// The locals a real request accumulates before the extractor runs; every
// request-local lookup scans past them.
func Benchmark_Extractor_Chain3_HitFirst_WithLocals(b *testing.B) {
	c := newBenchKeyCtx(b)
	for i := range 5 {
		c.Locals(i, "value")
	}
	benchExtract(b, benchChain3(), c)
}

// --- source-aware -------------------------------------------------------------

func Benchmark_Extractor_Resolve_Leaf(b *testing.B) {
	benchResolve(b, FromHeader("X-API-Key"), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Resolve_Chain3_HitFirst(b *testing.B) {
	benchResolve(b, benchChain3(), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Resolve_Chain3_HitLast(b *testing.B) {
	benchResolve(b, benchChain3(), newBenchQueryCtx(b))
}

func Benchmark_Extractor_Resolve_Chain3_Miss(b *testing.B) {
	benchResolve(b, benchChain3(), newBenchCtx(b))
}

func Benchmark_Extractor_Resolve_Nested_HitInner(b *testing.B) {
	outer := Chain(FromHeader("X-API-Key"), Chain(FromCookie("api_key"), FromQuery("api_key")))
	benchResolve(b, outer, newBenchQueryCtx(b))
}

func Benchmark_Extractor_Resolve_BareChain_HitLast(b *testing.B) {
	bare := Extractor{
		Chain:  []Extractor{FromHeader("X-API-Key"), FromCookie("api_key"), FromQuery("api_key")},
		Source: SourceHeader,
	}
	benchResolve(b, bare, newBenchQueryCtx(b))
}

// --- fresh request -----------------------------------------------------------

func Benchmark_Extractor_FromHeader_Hit_FreshRequest(b *testing.B) {
	benchExtractFresh(b, FromHeader("X-API-Key"), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Chain1_Hit_FreshRequest(b *testing.B) {
	benchExtractFresh(b, Chain(FromHeader("X-API-Key")), newBenchKeyCtx(b))
}

func Benchmark_Extractor_Chain3_HitLast_FreshRequest(b *testing.B) {
	benchExtractFresh(b, benchChain3(), newBenchQueryCtx(b))
}

func Benchmark_Extractor_Resolve_Chain3_HitFirst_FreshRequest(b *testing.B) {
	benchResolveFresh(b, benchChain3(), newBenchKeyCtx(b))
}

// --- token68 ------------------------------------------------------------------

func Benchmark_Extractor_Token68_JWT(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSink.ok = isValidToken68(benchToken)
	}
}
