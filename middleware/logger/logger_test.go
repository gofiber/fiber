package logger

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Logger
func Test_Logger(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${error}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return errors.New("some random error")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)
	utils.AssertEqual(t, "some random error", buf.String())
}

// go test -run Test_Logger_Next
func Test_Logger_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Logger_ErrorTimeZone
func Test_Logger_ErrorTimeZone(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		TimeZone: "invalid",
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

type fakeOutput int

func (o *fakeOutput) Write([]byte) (int, error) {
	*o++
	return 0, errors.New("fake output")
}

// go test -run Test_Logger_ErrorOutput
func Test_Logger_ErrorOutput(t *testing.T) {
	o := new(fakeOutput)
	app := fiber.New()
	app.Use(New(Config{
		Output: o,
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	utils.AssertEqual(t, 2, int(*o))
}

// go test -run Test_Logger_All
func Test_Logger_All(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${pid}${referer}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${header:test}${query:test}${form:test}${cookie:test}${non}",
		Output: buf,
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("%dhttp0.0.0.0example.com//%s%s%s%s%s%s%s%s%s-", os.Getpid(), cBlack, cRed, cGreen, cYellow, cBlue, cMagenta, cCyan, cWhite, cReset)
	utils.AssertEqual(t, expected, buf.String())
}

// go test -run Test_Logger_AppendUint
func Test_Logger_AppendUint(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("hello")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "0 5 200", buf.String())
}

// go test -v -run=^$ -bench=Benchmark_Logger -benchmem -count=4
func Benchmark_Logger(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Output: ioutil.Discard,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	utils.AssertEqual(b, 200, fctx.Response.Header.StatusCode())
}
