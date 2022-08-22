package logger

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
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

	app.Get("/", func(c fiber.Ctx) error {
		return errors.New("some random error")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "some random error", buf.String())
}

// go test -run Test_Logger_locals
func Test_Logger_locals(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${locals:demo}",
		Output: buf,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Locals("demo", "johndoe")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/int", func(c fiber.Ctx) error {
		c.Locals("demo", 55)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/empty", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "johndoe", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest("GET", "/int", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "55", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest("GET", "/empty", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "", buf.String())
}

// go test -run Test_Logger_Next
func Test_Logger_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Logger_ErrorTimeZone
func Test_Logger_ErrorTimeZone(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		TimeZone: "invalid",
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
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
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	require.Equal(t, 2, int(*o))
}

// go test -run Test_Logger_All
func Test_Logger_All(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${header:test}${query:test}${form:test}${cookie:test}${non}",
		Output: buf,
	}))

	// Alias colors
	colors := app.Config().ColorScheme

	resp, err := app.Test(httptest.NewRequest("GET", "/?foo=bar", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("%dHost=example.comhttpHTTP/1.10.0.0.0example.com/?foo=bar/%s%s%s%s%s%s%s%s%sCannot GET /", os.Getpid(), colors.Black, colors.Red, colors.Green, colors.Yellow, colors.Blue, colors.Magenta, colors.Cyan, colors.White, colors.Reset)
	require.Equal(t, expected, buf.String())
}

// go test -run Test_Query_Params
func Test_Query_Params(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${queryParams}",
		Output: buf,
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/?foo=bar&baz=moz", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := "foo=bar&baz=moz"
	require.Equal(t, expected, buf.String())
}

// go test -run Test_Response_Body
func Test_Response_Body(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${resBody}",
		Output: buf,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Sample response body")
	})

	app.Post("/test", func(c fiber.Ctx) error {
		return c.Send([]byte("Post in test"))
	})

	_, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)

	expectedGetResponse := "Sample response body"
	require.Equal(t, expectedGetResponse, buf.String())

	buf.Reset() // Reset buffer to test POST

	_, err = app.Test(httptest.NewRequest("POST", "/test", nil))
	require.NoError(t, err)

	expectedPostResponse := "Post in test"
	require.Equal(t, expectedPostResponse, buf.String())
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

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "0 5 200", buf.String())
}

// go test -run Test_Logger_Data_Race -race
func Test_Logger_Data_Race(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(ConfigDefault))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	var (
		resp1, resp2 *http.Response
		err1, err2   error
	)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		resp1, err1 = app.Test(httptest.NewRequest("GET", "/", nil))
		wg.Done()
	}()
	resp2, err2 = app.Test(httptest.NewRequest("GET", "/", nil))
	wg.Wait()

	require.Nil(t, err1)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)
	require.Nil(t, err2)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Logger -benchmem -count=4
func Benchmark_Logger(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Output: io.Discard,
	}))
	app.Get("/", func(c fiber.Ctx) error {
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

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
}

// go test -run Test_Response_Header
func Test_Response_Header(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(requestid.New(requestid.Config{
		Next:       nil,
		Header:     fiber.HeaderXRequestID,
		Generator:  func() string { return "Hello fiber!" },
		ContextKey: "requestid",
	}))
	app.Use(New(Config{
		Format: "${respHeader:X-Request-ID}",
		Output: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}

// go test -run Test_Req_Header
func Test_Req_Header(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${reqHeader:test}",
		Output: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	headerReq := httptest.NewRequest("GET", "/", nil)
	headerReq.Header.Add("test", "Hello fiber!")
	resp, err := app.Test(headerReq)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}

// go test -run Test_ReqHeader_Header
func Test_ReqHeader_Header(t *testing.T) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${reqHeader:test}",
		Output: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	reqHeaderReq := httptest.NewRequest("GET", "/", nil)
	reqHeaderReq.Header.Add("test", "Hello fiber!")
	resp, err := app.Test(reqHeaderReq)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}
