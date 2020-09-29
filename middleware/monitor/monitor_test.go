package monitor

import (
	"bytes"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

func Test_Monitor_405(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use("/", New())

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
}

func Test_Monitor_Html(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", New())

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("<title>Fiber Monitor</title>")))
}

// go test -run Test_Monitor_JSON -race
func Test_Monitor_JSON(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", New())

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAccept, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMEApplicationJSON, resp.Header.Get(fiber.HeaderContentType))

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("pid")))
	utils.AssertEqual(t, true, bytes.Contains(b, []byte("os")))
}

// go test -v -run=^$ -bench=Benchmark_Monitor -benchmem -count=4
func Benchmark_Monitor(b *testing.B) {
	app := fiber.New()

	app.Get("/", New())

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAccept, fiber.MIMEApplicationJSON)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h(fctx)
		}
	})

	utils.AssertEqual(b, 200, fctx.Response.Header.StatusCode())
	utils.AssertEqual(b,
		fiber.MIMEApplicationJSON,
		string(fctx.Response.Header.Peek(fiber.HeaderContentType)))
}
