package monitor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

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

	// defaults
	app.Get("/", New())
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8,
		resp.Header.Get(fiber.HeaderContentType))
	buf, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(buf, []byte("<title>"+defaultTitle+"</title>")))
	timeoutLine := fmt.Sprintf("setTimeout(fetchJSON, %d)",
		defaultRefresh.Milliseconds()-timeoutDiff)
	utils.AssertEqual(t, true, bytes.Contains(buf, []byte(timeoutLine)))

	// custom config
	conf := Config{Title: "New " + defaultTitle, Refresh: defaultRefresh + time.Second}
	app.Get("/custom", New(conf))
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/custom", nil))

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8,
		resp.Header.Get(fiber.HeaderContentType))
	buf, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(buf, []byte("<title>"+conf.Title+"</title>")))
	timeoutLine = fmt.Sprintf("setTimeout(fetchJSON, %d)",
		conf.Refresh.Milliseconds()-timeoutDiff)
	utils.AssertEqual(t, true, bytes.Contains(buf, []byte(timeoutLine)))
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

// go test -run Test_Monitor_Next
func Test_Monitor_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use("/", New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)
}

// go test -run Test_Monitor_APIOnly -race
func Test_Monitor_APIOnly(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", New(Config{
		APIOnly: true,
	}))

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

// go test -run Test_Monitor_UseCDN -race
func Test_Monitor_UseCDN(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", New(Config{
		UseCDN: true,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte(`<script src="https://cdn.jsdelivr.net/npm/chart.js@2.9/dist/Chart.bundle.min.js"></script>`)))
}

// go test -run Test_Monitor_Script -race
func Test_Monitor_Script(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/", New())

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Sec-Fetch-Dest", "script")
	resp, err := app.Test(req)
	fmt.Print(resp.Header.Get("Sec-Fetch-Dest"))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, "script", resp.Request.Header.Get("Sec-Fetch-Dest"))

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, bytes.Contains(b, []byte(`Chart.js v2.9.0`)))
}
