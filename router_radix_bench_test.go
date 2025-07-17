package fiber

import (
	"testing"

	"github.com/valyala/fasthttp"
)

// Benchmark_Router_Handler_Radix benchmarks route lookup using radix tree.
func Benchmark_Router_Handler_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	registerDummyRoutes(app)
	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("DELETE")
	ctx.URI().SetPath("/user/keys/1337")

	b.ResetTimer()
	for b.Loop() {
		handler(ctx)
	}
}

// Benchmark_Router_Next_Radix benchmarks next() using radix routing.
func Benchmark_Router_Next_Radix(b *testing.B) {
	app := New(Config{UseRadix: true})
	registerDummyRoutes(app)
	app.startupProcess()

	req := &fasthttp.RequestCtx{}
	req.Request.Header.SetMethod("DELETE")
	req.URI().SetPath("/user/keys/1337")
	c := app.AcquireCtx(req).(*DefaultCtx)

	b.ResetTimer()
	var err error
	for b.Loop() {
		c.indexRoute = -1
		_, err = app.next(c)
	}
	if err != nil {
		b.Fatal(err)
	}
}
