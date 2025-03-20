//nolint:depguard // Because we test logging :D
package logger

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

func benchmarkSetup(b *testing.B, app *fiber.App, uri string) {
	b.Helper()

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI(uri)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}
}

func benchmarkSetupParallel(b *testing.B, app *fiber.App, path string) {
	b.Helper()

	handler := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI(path)

		for pb.Next() {
			handler(fctx)
		}
	})
}

// go test -run Test_Logger
func Test_Logger(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${error}",
		Stream: buf,
	}))

	app.Get("/", func(_ fiber.Ctx) error {
		return errors.New("some random error")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "some random error", buf.String())
}

// go test -run Test_Logger_locals
func Test_Logger_locals(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${locals:demo}",
		Stream: buf,
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "johndoe", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/int", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "55", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/empty", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "", buf.String())
}

// go test -run Test_Logger_Next
func Test_Logger_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Logger_Done
func Test_Logger_Done(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBuffer(nil)
	app := fiber.New()

	app.Use(New(Config{
		Done: func(c fiber.Ctx, logString []byte) {
			if c.Response().StatusCode() == fiber.StatusOK {
				_, err := buf.Write(logString)
				require.NoError(t, err)
			}
		},
	})).Get("/logging", func(ctx fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/logging", nil))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Positive(t, buf.Len(), 0)
}

// Test_Logger_Filter tests the Filter functionality of the logger middleware.
// It verifies that logs are written or skipped based on the filter condition.
func Test_Logger_Filter(t *testing.T) {
	t.Parallel()

	t.Run("Test Not Found", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		logOutput := bytes.Buffer{}

		// Return true to skip logging for all requests != 404
		app.Use(New(Config{
			Skip: func(c fiber.Ctx) bool {
				return c.Response().StatusCode() != fiber.StatusNotFound
			},
			Stream: &logOutput,
		}))

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/nonexistent", nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

		// Expect logs for the 404 request
		require.Contains(t, logOutput.String(), "404")
	})

	t.Run("Test OK", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		logOutput := bytes.Buffer{}

		// Return true to skip logging for all requests == 200
		app.Use(New(Config{
			Skip: func(c fiber.Ctx) bool {
				return c.Response().StatusCode() == fiber.StatusOK
			},
			Stream: &logOutput,
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		// We skip logging for status == 200, so "200" should not appear
		require.NotContains(t, logOutput.String(), "200")
	})

	t.Run("Always Skip", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		logOutput := bytes.Buffer{}

		// Filter always returns true => skip all logs
		app.Use(New(Config{
			Skip: func(_ fiber.Ctx) bool {
				return true // always skip
			},
			Stream: &logOutput,
		}))

		app.Get("/something", func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTeapot).SendString("I'm a teapot")
		})

		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/something", nil))
		require.NoError(t, err)

		// Expect NO logs
		require.Empty(t, logOutput.String())
	})

	t.Run("Never Skip", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		logOutput := bytes.Buffer{}

		// Filter always returns false => never skip logs
		app.Use(New(Config{
			Skip: func(_ fiber.Ctx) bool {
				return false // never skip
			},
			Stream: &logOutput,
		}))

		app.Get("/always", func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTeapot).SendString("Teapot again")
		})

		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/always", nil))
		require.NoError(t, err)

		// Expect some logging - check any substring
		require.Contains(t, logOutput.String(), strconv.Itoa(fiber.StatusTeapot))
	})

	t.Run("Skip /healthz", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()

		logOutput := bytes.Buffer{}

		// Filter returns true (skip logs) if the request path is /healthz
		app.Use(New(Config{
			Skip: func(c fiber.Ctx) bool {
				return c.Path() == "/healthz"
			},
			Stream: &logOutput,
		}))

		// Normal route
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello World!")
		})
		// Health route
		app.Get("/healthz", func(c fiber.Ctx) error {
			return c.SendString("OK")
		})

		// Request to "/" -> should be logged
		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		require.NoError(t, err)
		require.Contains(t, logOutput.String(), "200")

		// Reset output buffer
		logOutput.Reset()

		// Request to "/healthz" -> should be skipped
		_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/healthz", nil))
		require.NoError(t, err)
		require.Empty(t, logOutput.String())
	})
}

// go test -run Test_Logger_ErrorTimeZone
func Test_Logger_ErrorTimeZone(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		TimeZone: "invalid",
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_Logger_Fiber_Logger
func Test_Logger_LoggerToWriter(t *testing.T) {
	app := fiber.New()

	buf := bytebufferpool.Get()
	t.Cleanup(func() {
		bytebufferpool.Put(buf)
	})

	logger := fiberlog.DefaultLogger()
	stdlogger, ok := logger.Logger().(*log.Logger)
	require.True(t, ok)

	stdlogger.SetFlags(0)
	logger.SetOutput(buf)

	testCases := []struct {
		levelStr string
		level    fiberlog.Level
	}{
		{
			level:    fiberlog.LevelTrace,
			levelStr: "Trace",
		},
		{
			level:    fiberlog.LevelDebug,
			levelStr: "Debug",
		},
		{
			level:    fiberlog.LevelInfo,
			levelStr: "Info",
		},
		{
			level:    fiberlog.LevelWarn,
			levelStr: "Warn",
		},
		{
			level:    fiberlog.LevelError,
			levelStr: "Error",
		},
	}

	for _, tc := range testCases {
		level := strconv.Itoa(int(tc.level))
		t.Run(level, func(t *testing.T) {
			buf.Reset()

			app.Use("/"+level, New(Config{
				Format: "${error}",
				Stream: LoggerToWriter(logger, tc.
					level),
			}))

			app.Get("/"+level, func(_ fiber.Ctx) error {
				return errors.New("some random error")
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/"+level, nil))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
			require.Equal(t, "["+tc.levelStr+"] some random error\n", buf.String())
		})

		require.Panics(t, func() {
			LoggerToWriter(logger, fiberlog.LevelPanic)
		})

		require.Panics(t, func() {
			LoggerToWriter(logger, fiberlog.LevelFatal)
		})

		require.Panics(t, func() {
			LoggerToWriter(nil, fiberlog.LevelFatal)
		})
	}
}

type fakeErrorOutput int

func (o *fakeErrorOutput) Write([]byte) (int, error) {
	*o++
	return 0, errors.New("fake output")
}

// go test -run Test_Logger_ErrorOutput_WithoutColor
func Test_Logger_ErrorOutput_WithoutColor(t *testing.T) {
	t.Parallel()
	o := new(fakeErrorOutput)
	app := fiber.New()

	app.Use(New(Config{
		Stream:        o,
		DisableColors: true,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 2, *o)
}

// go test -run Test_Logger_ErrorOutput
func Test_Logger_ErrorOutput(t *testing.T) {
	t.Parallel()
	o := new(fakeErrorOutput)
	app := fiber.New()

	app.Use(New(Config{
		Stream: o,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 2, *o)
}

// go test -run Test_Logger_All
func Test_Logger_All(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}${non}",
		Stream: buf,
	}))

	// Alias colors
	colors := app.Config().ColorScheme

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("%dHost=example.comhttpHTTP/1.10.0.0.0example.com/?foo=bar/%s%s%s%s%s%s%s%s%sCannot GET /", os.Getpid(), colors.Black, colors.Red, colors.Green, colors.Yellow, colors.Blue, colors.Magenta, colors.Cyan, colors.White, colors.Reset)
	require.Equal(t, expected, buf.String())
}

func Test_Logger_CLF_Format_With_Name(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "common",
		Stream: buf,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("0.0.0.0 - - [%s] \"%s %s %s\" %d %d\n",
		time.Now().Format("15:04:05"),
		fiber.MethodGet, "/?foo=bar", "HTTP/1.1",
		fiber.StatusNotFound,
		0)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_CLF_Format_With_Const(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: FormatCommonLog,
		Stream: buf,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("0.0.0.0 - - [%s] \"%s %s %s\" %d %d\n",
		time.Now().Format("15:04:05"),
		fiber.MethodGet, "/?foo=bar", "HTTP/1.1",
		fiber.StatusNotFound,
		0)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_Combined_CLF_Format_With_Name(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "combined",
		Stream: buf,
	}))
	const expectedUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36"
	const expectedReferer = "http://example.com"
	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	req.Header.Set("Referer", expectedReferer)
	req.Header.Set("User-Agent", expectedUA)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("0.0.0.0 - - [%s] %q %d %d %q %q\n",
		time.Now().Format("15:04:05"),
		fmt.Sprintf("%s %s %s", fiber.MethodGet, "/?foo=bar", "HTTP/1.1"),
		fiber.StatusNotFound,
		0,
		expectedReferer,
		expectedUA)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_Combined_CLF_Format_With_Const(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: FormatCombined,
		Stream: buf,
	}))
	const expectedUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36"
	const expectedReferer = "http://example.com"
	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	req.Header.Set("Referer", expectedReferer)
	req.Header.Set("User-Agent", expectedUA)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("0.0.0.0 - - [%s] %q %d %d %q %q\n",
		time.Now().Format("15:04:05"),
		fmt.Sprintf("%s %s %s", fiber.MethodGet, "/?foo=bar", "HTTP/1.1"),
		fiber.StatusNotFound,
		0,
		expectedReferer,
		expectedUA)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_Json_Format_With_Name(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "json",
		Stream: buf,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf(
		"{\"time\":%q,\"ip\":%q,\"method\":%q,\"url\":%q,\"status\":%d,\"bytesSent\":%d}\n",
		time.Now().Format("15:04:05"),
		"0.0.0.0",
		fiber.MethodGet,
		"/?foo=bar",
		fiber.StatusNotFound,
		0,
	)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_Json_Format_With_Const(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: FormatJSON,
		Stream: buf,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf(
		"{\"time\":%q,\"ip\":%q,\"method\":%q,\"url\":%q,\"status\":%d,\"bytesSent\":%d}\n",
		time.Now().Format("15:04:05"),
		"0.0.0.0",
		fiber.MethodGet,
		"/?foo=bar",
		fiber.StatusNotFound,
		0,
	)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_ECS_Format_With_Name(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "ecs",
		Stream: buf,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf(
		"{\"@timestamp\":%q,\"ecs\":{\"version\":\"1.6.0\"},\"client\":{\"ip\":%q},\"http\":{\"request\":{\"method\":%q,\"url\":%q,\"protocol\":%q},\"response\":{\"status_code\":%d,\"body\":{\"bytes\":%d}}},\"log\":{\"level\":\"INFO\",\"logger\":\"fiber\"},\"message\":%q}\n",
		time.Now().Format("15:04:05"),
		"0.0.0.0",
		fiber.MethodGet,
		"/?foo=bar",
		"HTTP/1.1",
		fiber.StatusNotFound,
		0,
		fmt.Sprintf("%s %s responded with %d", fiber.MethodGet, "/?foo=bar", fiber.StatusNotFound),
	)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
}

func Test_Logger_ECS_Format_With_Const(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: FormatECS,
		Stream: buf,
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/?foo=bar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf(
		"{\"@timestamp\":%q,\"ecs\":{\"version\":\"1.6.0\"},\"client\":{\"ip\":%q},\"http\":{\"request\":{\"method\":%q,\"url\":%q,\"protocol\":%q},\"response\":{\"status_code\":%d,\"body\":{\"bytes\":%d}}},\"log\":{\"level\":\"INFO\",\"logger\":\"fiber\"},\"message\":%q}\n",
		time.Now().Format("15:04:05"),
		"0.0.0.0",
		fiber.MethodGet,
		"/?foo=bar",
		"HTTP/1.1",
		fiber.StatusNotFound,
		0,
		fmt.Sprintf("%s %s responded with %d", fiber.MethodGet, "/?foo=bar", fiber.StatusNotFound),
	)
	logResponse := buf.String()
	require.Equal(t, expected, logResponse)
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
			{unit: "ms", div: time.Millisecond},
			{unit: "s", div: time.Second},
		}
	}
	return []struct {
		unit string
		div  time.Duration
	}{
		{unit: "µs", div: time.Microsecond},
		{unit: "ms", div: time.Millisecond},
		{unit: "s", div: time.Second},
	}
}

// go test -run Test_Logger_WithLatency
func Test_Logger_WithLatency(t *testing.T) {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)
	app := fiber.New()

	logger := New(Config{
		Stream: buff,
		Format: "${latency}",
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Initialize a new time unit
	sleepDuration := 1 * time.Nanosecond

	// Define a test route that sleeps
	app.Get("/test", func(c fiber.Ctx) error {
		time.Sleep(sleepDuration)
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the sleep duration for the next iteration
		sleepDuration = 1 * tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil), fiber.TestConfig{
			Timeout:       3 * time.Second,
			FailOnTimeout: true,
		})
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		// Assert that the log output contains the expected latency value in the current time unit
		require.True(t, bytes.HasSuffix(buff.Bytes(), []byte(tu.unit)), "Expected latency to be in %s, got %s", tu.unit, buff.String())

		// Reset the buffer
		buff.Reset()
	}
}

// go test -run Test_Logger_WithLatency_DefaultFormat
func Test_Logger_WithLatency_DefaultFormat(t *testing.T) {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)
	app := fiber.New()

	logger := New(Config{
		Stream: buff,
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Initialize a new time unit
	sleepDuration := 1 * time.Nanosecond

	// Define a test route that sleeps
	app.Get("/test", func(c fiber.Ctx) error {
		time.Sleep(sleepDuration)
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the sleep duration for the next iteration
		sleepDuration = 1 * tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", nil), fiber.TestConfig{
			Timeout:       2 * time.Second,
			FailOnTimeout: true,
		})
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		// Assert that the log output contains the expected latency value in the current time unit
		// parse out the latency value from the log output
		latency := bytes.Split(buff.Bytes(), []byte(" | "))[2]
		// Assert that the latency value is in the current time unit
		require.True(t, bytes.HasSuffix(latency, []byte(tu.unit)), "Expected latency to be in %s, got %s", tu.unit, latency)

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
		Stream: buf,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar&baz=moz", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := "foo=bar&baz=moz"
	require.Equal(t, expected, buf.String())
}

// go test -run Test_Response_Body
func Test_Response_Body(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "${resBody}",
		Stream: buf,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Sample response body")
	})

	app.Post("/test", func(c fiber.Ctx) error {
		return c.Send([]byte("Post in test"))
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)

	expectedGetResponse := "Sample response body"
	require.Equal(t, expectedGetResponse, buf.String())

	buf.Reset() // Reset buffer to test POST
	_, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/test", nil))

	expectedPostResponse := "Post in test"
	require.NoError(t, err)
	require.Equal(t, expectedPostResponse, buf.String())
}

// go test -run Test_Request_Body
func Test_Request_Body(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	app := fiber.New()

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Stream: buf,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		c.Response().Header.SetContentLength(5)
		return c.SendString("World")
	})

	// Create a POST request with a body
	body := []byte("Hello")
	req := httptest.NewRequest(fiber.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")

	_, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "5 5 200", buf.String())
}

// go test -run Test_Logger_AppendUint
func Test_Logger_AppendUint(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Stream: buf,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	app.Get("/content", func(c fiber.Ctx) error {
		c.Response().Header.SetContentLength(5)
		return c.SendString("hello")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "-2 0 200", buf.String())

	buf.Reset()
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/content", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "-2 5 200", buf.String())
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
		resp1, err1 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		wg.Done()
	}()
	resp2, err2 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	wg.Wait()

	require.NoError(t, err1)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)
	require.NoError(t, err2)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

// go test -run Test_Response_Header
func Test_Response_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(requestid.New(requestid.Config{
		Next:      nil,
		Header:    fiber.HeaderXRequestID,
		Generator: func() string { return "Hello fiber!" },
	}))
	app.Use(New(Config{
		Format: "${respHeader:X-Request-ID}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}

// go test -run Test_Req_Header
func Test_Req_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "${reqHeader:test}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	headerReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	headerReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(headerReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}

// go test -run Test_ReqHeader_Header
func Test_ReqHeader_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "${reqHeader:test}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	reqHeaderReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(reqHeaderReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
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
			"custom_tag": func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
				return output.WriteString(customTag)
			},
		},
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	reqHeaderReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(reqHeaderReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, customTag, buf.String())
}

// go test -run Test_Logger_ByteSent_Streaming
func Test_Logger_ByteSent_Streaming(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${bytesReceived} ${bytesSent} ${status}",
		Stream: buf,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
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
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// -2 means identity, -1 means chunked, 200 status
	require.Equal(t, "-2 -1 200", buf.String())
}

type fakeOutput int

func (o *fakeOutput) Write(b []byte) (int, error) {
	*o++
	return len(b), nil
}

// go test -run Test_Logger_EnableColors
func Test_Logger_EnableColors(t *testing.T) {
	t.Parallel()
	o := new(fakeOutput)
	app := fiber.New()

	app.Use(New(Config{
		Stream: o,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 1, *o)
}

// go test -v -run=^$ -bench=Benchmark_Logger$ -benchmem -count=4
func Benchmark_Logger(b *testing.B) {
	b.Run("NoMiddleware", func(bb *testing.B) {
		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("WithBytesAndStatus", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("DefaultFormat", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("DefaultFormatDisableColors", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream:        io.Discard,
			DisableColors: true,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("DefaultFormatWithFiberLog", func(bb *testing.B) {
		app := fiber.New()
		logger := fiberlog.DefaultLogger()
		logger.SetOutput(io.Discard)
		app.Use(New(Config{
			Stream: LoggerToWriter(logger, fiberlog.LevelDebug),
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("WithTagParameter", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status} ${reqHeader:test}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("WithLocals", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${locals:demo}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Locals("demo", "johndoe")
			return c.SendStatus(fiber.StatusOK)
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("WithLocalsInt", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${locals:demo}",
			Stream: io.Discard,
		}))
		app.Get("/int", func(c fiber.Ctx) error {
			c.Locals("demo", 55)
			return c.SendStatus(fiber.StatusOK)
		})
		benchmarkSetup(bb, app, "/int")
	})

	b.Run("WithCustomDone", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Done: func(c fiber.Ctx, logString []byte) {
				if c.Response().StatusCode() == fiber.StatusOK {
					io.Discard.Write(logString) //nolint:errcheck // ignore error
				}
			},
			Stream: io.Discard,
		}))
		app.Get("/logging", func(ctx fiber.Ctx) error {
			return ctx.SendStatus(fiber.StatusOK)
		})
		benchmarkSetup(bb, app, "/logging")
	})

	b.Run("WithAllTags", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}${non}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("Streaming", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("Connection", "keep-alive")
			c.Set("Transfer-Encoding", "chunked")
			c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
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
		benchmarkSetup(bb, app, "/")
	})

	b.Run("WithBody", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${resBody}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Sample response body")
		})
		benchmarkSetup(bb, app, "/")
	})
}

// go test -v -run=^$ -bench=Benchmark_Logger_Parallel$ -benchmem -count=4
func Benchmark_Logger_Parallel(b *testing.B) {
	b.Run("NoMiddleware", func(bb *testing.B) {
		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("WithBytesAndStatus", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("DefaultFormat", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("DefaultFormatWithFiberLog", func(bb *testing.B) {
		app := fiber.New()
		logger := fiberlog.DefaultLogger()
		logger.SetOutput(io.Discard)
		app.Use(New(Config{
			Stream: LoggerToWriter(logger, fiberlog.LevelDebug),
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("DefaultFormatDisableColors", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream:        io.Discard,
			DisableColors: true,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("WithTagParameter", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status} ${reqHeader:test}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("test", "test")
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("WithLocals", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${locals:demo}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Locals("demo", "johndoe")
			return c.SendStatus(fiber.StatusOK)
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("WithLocalsInt", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${locals:demo}",
			Stream: io.Discard,
		}))
		app.Get("/int", func(c fiber.Ctx) error {
			c.Locals("demo", 55)
			return c.SendStatus(fiber.StatusOK)
		})
		benchmarkSetupParallel(bb, app, "/int")
	})

	b.Run("WithCustomDone", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Done: func(c fiber.Ctx, logString []byte) {
				if c.Response().StatusCode() == fiber.StatusOK {
					io.Discard.Write(logString) //nolint:errcheck // ignore error
				}
			},
			Stream: io.Discard,
		}))
		app.Get("/logging", func(ctx fiber.Ctx) error {
			return ctx.SendStatus(fiber.StatusOK)
		})
		benchmarkSetupParallel(bb, app, "/logging")
	})

	b.Run("WithAllTags", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}${non}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("Streaming", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${bytesReceived} ${bytesSent} ${status}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("Connection", "keep-alive")
			c.Set("Transfer-Encoding", "chunked")
			c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
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
		benchmarkSetupParallel(bb, app, "/")
	})

	b.Run("WithBody", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Format: "${resBody}",
			Stream: io.Discard,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Sample response body")
		})
		benchmarkSetupParallel(bb, app, "/")
	})
}
