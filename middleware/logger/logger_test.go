//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package logger

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Logger
func Test_Logger(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusInternalServerError, resp.StatusCode)
	utils.AssertEqual(t, "some random error", buf.String())
}

// go test -run Test_Logger_locals
func Test_Logger_locals(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${locals:demo}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Locals("demo", "johndoe")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/int", func(c *fiber.Ctx) error {
		c.Locals("demo", 55)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/empty", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "johndoe", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/int", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "55", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/empty", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "", buf.String())
}

// go test -run Test_Logger_Next
func Test_Logger_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ *fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Logger_Done
func Test_Logger_Done(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBuffer(nil)
	app := fiber.New()
	app.Use(New(Config{
		Done: func(c *fiber.Ctx, logString []byte) {
			if c.Response().StatusCode() == fiber.StatusOK {
				_, err := buf.Write(logString)
				utils.AssertEqual(t, nil, err)
			}
		},
	})).Get("/logging", func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/logging", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, true, buf.Len() > 0)
}

// go test -run Test_Logger_ErrorTimeZone
func Test_Logger_ErrorTimeZone(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		TimeZone: "invalid",
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)
}

type fakeOutput int

func (o *fakeOutput) Write([]byte) (int, error) {
	*o++
	return 0, nil
}

// go test -run Test_Logger_ErrorOutput_WithoutColor
func Test_Logger_ErrorOutput_WithoutColor(t *testing.T) {
	t.Parallel()
	o := new(fakeOutput)
	app := fiber.New()
	app.Use(New(Config{
		Output:        o,
		DisableColors: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	utils.AssertEqual(t, 1, int(*o))
}

// go test -run Test_Logger_ErrorOutput
func Test_Logger_ErrorOutput(t *testing.T) {
	t.Parallel()
	o := new(fakeOutput)
	app := fiber.New()
	app.Use(New(Config{
		Output: o,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	utils.AssertEqual(t, 1, int(*o))
}

// go test -run Test_Logger_All
func Test_Logger_All(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${pid}${reqHeaders}${referer}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${header:test}${query:test}${form:test}${cookie:test}${non}",
		Output: buf,
	}))

	// Alias colors
	colors := app.Config().ColorScheme

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("%dHost=example.comhttp0.0.0.0example.com/?foo=bar/%s%s%s%s%s%s%s%s%sCannot GET /", os.Getpid(), colors.Black, colors.Red, colors.Green, colors.Yellow, colors.Blue, colors.Magenta, colors.Cyan, colors.White, colors.Reset)
	utils.AssertEqual(t, expected, buf.String())
}

func getLatencyTimeUnits() []struct {
	unit string
	div  time.Duration
} {
	// windows does not support µs sleep precision
	// https://github.com/golang/go/issues/29485
	if runtime.GOOS == "windows" {
		return []struct {
			unit string
			div  time.Duration
		}{
			{"ms", time.Millisecond},
			{"s", time.Second},
		}
	}
	return []struct {
		unit string
		div  time.Duration
	}{
		{"µs", time.Microsecond},
		{"ms", time.Millisecond},
		{"s", time.Second},
	}
}

// go test -run Test_Logger_WithLatency
func Test_Logger_WithLatency(t *testing.T) {
	t.Parallel()
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)
	app := fiber.New()
	logger := New(Config{
		Output: buff,
		Format: "${latency}",
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Initialize a new time unit
	sleepDuration := 1 * time.Nanosecond

	// Define a test route that sleeps
	app.Get("/test", func(c *fiber.Ctx) error {
		time.Sleep(sleepDuration)
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the sleep duration for the next iteration
		sleepDuration = 1 * tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil), int(2*time.Second))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		// Assert that the log output contains the expected latency value in the current time unit
		utils.AssertEqual(t, bytes.HasSuffix(buff.Bytes(), []byte(tu.unit)), true, fmt.Sprintf("Expected latency to be in %s, got %s", tu.unit, buff.String()))

		// Reset the buffer
		buff.Reset()
	}
}

// go test -run Test_Logger_WithLatency_DefaultFormat
func Test_Logger_WithLatency_DefaultFormat(t *testing.T) {
	t.Parallel()
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)
	app := fiber.New()
	logger := New(Config{
		Output: buff,
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Initialize a new time unit
	sleepDuration := 1 * time.Nanosecond

	// Define a test route that sleeps
	app.Get("/test", func(c *fiber.Ctx) error {
		time.Sleep(sleepDuration)
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the sleep duration for the next iteration
		sleepDuration = 1 * tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil), int(2*time.Second))
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)

		// Assert that the log output contains the expected latency value in the current time unit
		// parse out the latency value from the log output
		latency := bytes.Split(buff.Bytes(), []byte(" | "))[2]
		// Assert that the latency value is in the current time unit
		utils.AssertEqual(t, bytes.HasSuffix(latency, []byte(tu.unit)), true, fmt.Sprintf("Expected latency to be in %s, got %s", tu.unit, latency))

		// Reset the buffer
		buff.Reset()
	}
}

// go test -run Test_Query_Params
func Test_Query_Params(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${queryParams}",
		Output: buf,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar&baz=moz", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	expected := "foo=bar&baz=moz"
	utils.AssertEqual(t, expected, buf.String())
}

// go test -run Test_Response_Body
func Test_Response_Body(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${resBody}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Sample response body")
	})

	app.Post("/test", func(c *fiber.Ctx) error {
		return c.Send([]byte("Post in test"))
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)

	expectedGetResponse := "Sample response body"
	utils.AssertEqual(t, expectedGetResponse, buf.String())

	buf.Reset() // Reset buffer to test POST

	_, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/test", nil))
	utils.AssertEqual(t, nil, err)

	expectedPostResponse := "Post in test"
	utils.AssertEqual(t, expectedPostResponse, buf.String())
}

// go test -run Test_Logger_AppendUint
func Test_Logger_AppendUint(t *testing.T) {
	t.Parallel()
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "0 5 200", buf.String())
}

// go test -run Test_Logger_Data_Race -race
func Test_Logger_Data_Race(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(ConfigDefault))
	app.Use(New(Config{
		Format: "${time} | ${pid} | ${locals:requestid} | ${status} | ${latency} | ${method} | ${path}\n",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("hello")
	})

	var (
		resp1, resp2 *http.Response
		err1, err2   error
	)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		resp1, err1 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		wg.Done()
	}()
	resp2, err2 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	wg.Wait()
	utils.AssertEqual(t, nil, err1)
	utils.AssertEqual(t, fiber.StatusOK, resp1.StatusCode)
	utils.AssertEqual(t, nil, err2)
	utils.AssertEqual(t, fiber.StatusOK, resp2.StatusCode)
}

// go test -v -run=^$ -bench=Benchmark_Logger -benchmem -count=4
func Benchmark_Logger(b *testing.B) {
	benchSetup := func(b *testing.B, app *fiber.App) {
		b.Helper()

		h := app.Handler()

		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/")

		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			h(fctx)
		}

		utils.AssertEqual(b, 200, fctx.Response.Header.StatusCode())
	}

	b.Run("Base", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status}",
			Output: io.Discard,
		}))
		app.Get("/", func(c *fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchSetup(bb, app)
	})

	b.Run("DefaultFormat", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Output: io.Discard,
		}))
		app.Get("/", func(c *fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchSetup(bb, app)
	})

	b.Run("WithTagParameter", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status} ${reqHeader:test}",
			Output: io.Discard,
		}))
		app.Get("/", func(c *fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchSetup(bb, app)
	})
}

// go test -run Test_Response_Header
func Test_Response_Header(t *testing.T) {
	t.Parallel()
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
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "Hello fiber!", buf.String())
}

// go test -run Test_Req_Header
func Test_Req_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${header:test}",
		Output: buf,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	headerReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	headerReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(headerReq)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "Hello fiber!", buf.String())
}

// go test -run Test_ReqHeader_Header
func Test_ReqHeader_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${reqHeader:test}",
		Output: buf,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	reqHeaderReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(reqHeaderReq)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "Hello fiber!", buf.String())
}

// go test -run Test_CustomTags
func Test_CustomTags(t *testing.T) {
	t.Parallel()
	customTag := "it is a custom tag"

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${custom_tag}",
		CustomTags: map[string]LogFunc{
			"custom_tag": func(output Buffer, c *fiber.Ctx, data *Data, extraParam string) (int, error) {
				return output.WriteString(customTag)
			},
		},
		Output: buf,
	}))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	reqHeaderReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(reqHeaderReq)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, customTag, buf.String())
}

// go test -run Test_Logger_ByteSent_Streaming
func Test_Logger_ByteSent_Streaming(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Output: buf,
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			var i int
			for {
				i++
				msg := fmt.Sprintf("%d - the time is %v", i, time.Now())
				fmt.Fprintf(w, "data: Message: %s\n\n", msg)
				err := w.Flush()
				if err != nil {
					break
				}
				if i == 10 {
					break
				}
			}
		})
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode)
	utils.AssertEqual(t, "0 0 200", buf.String())
}

// go test -run Test_Logger_EnableColors
func Test_Logger_EnableColors(t *testing.T) {
	t.Parallel()
	o := new(fakeOutput)
	app := fiber.New()
	app.Use(New(Config{
		Output: o,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, resp.StatusCode)

	utils.AssertEqual(t, 1, int(*o))
}
