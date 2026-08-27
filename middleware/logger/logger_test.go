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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
)

const (
	pathFooBar = "/?foo=bar"
	httpProto  = "HTTP/1.1"
)

func benchmarkSetup(b *testing.B, app *fiber.App, uri string) {
	b.Helper()

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI(uri)

	b.ReportAllocs()

	for b.Loop() {
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "johndoe", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/int", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "55", buf.String())

	buf.Reset()

	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/empty", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Empty(t, buf.String())
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/logging", http.NoBody))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Positive(t, buf.Len(), 0)
}

func Test_Logger_TimeUpdaterStopsOnDone(t *testing.T) {
	t.Parallel()

	var timestamp atomic.Value
	timestamp.Store(time.Now().Format(time.RFC3339Nano))

	done := make(chan struct{})
	cfg := Config{
		Format:           "${time}",
		TimeFormat:       time.RFC3339Nano,
		TimeInterval:     5 * time.Millisecond,
		timeZoneLocation: time.Local,
		TimeDone:         done,
	}

	stoppedCh := startTimestampUpdaterWithStop(&timestamp, &cfg)

	initial, ok := timestamp.Load().(string)
	require.True(t, ok)
	// Wait for a tick rather than assume one lands inside a fixed sleep: the
	// updater is a goroutine on a 5ms ticker, and a loaded machine ran the whole
	// sleep before scheduling it.
	require.Eventually(t, func() bool {
		updated, isString := timestamp.Load().(string)
		return isString && updated != initial
	}, time.Second, time.Millisecond, "timestamp was never updated")

	close(done)
	select {
	case <-stoppedCh:
	case <-time.After(time.Second):
		t.Fatal("timestamp updater did not stop")
	}
	stopped, ok := timestamp.Load().(string)
	require.True(t, ok)
	time.Sleep(20 * time.Millisecond)
	finalValue, ok := timestamp.Load().(string)
	require.True(t, ok)
	require.Equal(t, stopped, finalValue)
}

func Test_Logger_TimestampUpdater_StopsImmediatelyWithoutTimeTag(t *testing.T) {
	t.Parallel()

	var timestamp atomic.Value
	timestamp.Store(time.Now().Format(time.RFC3339Nano))

	stoppedCh := startTimestampUpdaterWithStop(&timestamp, &Config{
		Format: "${pid}",
	})

	select {
	case <-stoppedCh:
	case <-time.After(time.Second):
		t.Fatal("timestamp updater did not stop immediately")
	}
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

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/nonexistent", http.NoBody))
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

		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/something", http.NoBody))
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

		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/always", http.NoBody))
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
		_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Contains(t, logOutput.String(), "200")

		// Reset output buffer
		logOutput.Reset()

		// Request to "/healthz" -> should be skipped
		_, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/healthz", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

	logger := fiberlog.DefaultLogger[*log.Logger]()
	stdlogger := logger.Logger()
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

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/"+level, http.NoBody))
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
			LoggerToWriter[any](nil, fiberlog.LevelFatal)
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 2, *o)
}

// Test_AppendIntPadded pins the right-align pad loop directly: statuses are
// almost always exactly width 3, so the end-to-end test below cannot
// exercise the padding (and net/http rejects 2-digit status lines).
func Test_AppendIntPadded(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	testCases := []struct {
		expected string
		v        int
		width    int
	}{
		{"  5", 5, 3},
		{" 99", 99, 3},
		{"404", 404, 3},
		{"1000", 1000, 3},
		{" -1", -1, 3},
	}
	for _, tc := range testCases {
		buf.Reset()
		appendIntPadded(buf, tc.v, tc.width)
		require.Equal(t, tc.expected, buf.String(), "appendIntPadded(%d, %d)", tc.v, tc.width)
	}
}

// Test_Logger_DefaultFormat_WithoutColor pins the byte layout of the
// non-color default line, in particular the width-3 right-aligned status
// field that is appended digit-wise without an intermediate string.
func Test_Logger_DefaultFormat_WithoutColor(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Stream:        buf,
		DisableColors: true,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotFound)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.Contains(t, buf.String(), " | 404 | ", "status field must be width-3 with separators")
	require.Contains(t, buf.String(), " | GET     | ", "method field must be width-7, left aligned")
}

// go test -run Test_Logger_ErrorOutput
func Test_Logger_ErrorOutput(t *testing.T) {
	t.Parallel()
	o := new(fakeErrorOutput)
	app := fiber.New()

	app.Use(New(Config{
		Stream: o,
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 2, *o)
}

func Test_Logger_ErrorOutput_TemplateFailure(t *testing.T) {
	t.Parallel()

	templateErr := errors.New("template failure")
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${fail}",
		Stream: buf,
		CustomTags: map[string]LogFunc{
			"fail": func(_ Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
				return 0, templateErr
			},
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.Equal(t, templateErr.Error(), buf.String())
}

func Test_Logger_UnknownTagPanicsWithTypedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		wantTag  string
		wantPara string
	}{
		{name: "parametric tag", format: "${missing:value}", wantTag: "missing:value", wantPara: "value"},
		{name: "bare tag", format: "${missing}", wantTag: "missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				r := recover()
				require.NotNil(t, r, "New must panic for unknown tag")

				err, ok := r.(error)
				require.True(t, ok, "panic value must be an error, got %T", r)
				require.ErrorIs(t, err, ErrUnknownTag)

				var typed *UnknownTagError
				require.ErrorAs(t, err, &typed)
				require.Equal(t, tt.wantTag, typed.Tag)
				require.Equal(t, tt.wantPara, typed.Param)
				require.EqualError(t, err, `logger: unknown template tag: "`+tt.wantTag+`"`)
			}()

			New(Config{Format: tt.format})
		})
	}
}

// go test -run Test_Logger_All
func Test_Logger_All(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}",
		Stream: buf,
	}))

	// Alias colors
	colors := app.Config().ColorScheme

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, pathFooBar, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("%dHost=example.comhttpHTTP/1.10.0.0.0example.com/?foo=bar/%s%s%s%s%s%s%s%s%sNot Found", os.Getpid(), colors.Black, colors.Red, colors.Green, colors.Yellow, colors.Blue, colors.Magenta, colors.Cyan, colors.White, colors.Reset)
	require.Equal(t, expected, buf.String())
}

func Test_Logger_CLF_Format(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: CommonFormat,
		Stream: buf,
	}))

	method := fiber.MethodGet
	status := fiber.StatusNotFound
	bytesSent := 0

	resp, err := app.Test(httptest.NewRequest(method, pathFooBar, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode)

	pattern := fmt.Sprintf(`0\.0\.0\.0 - - \[\d{2}:\d{2}:\d{2}\] "%s %s %s" %d %d`, method, regexp.QuoteMeta(pathFooBar), httpProto, status, bytesSent)
	require.Regexp(t, pattern, buf.String())
}

func Test_Logger_Combined_CLF_Format(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: CombinedFormat,
		Stream: buf,
	}))

	method := fiber.MethodGet
	status := fiber.StatusNotFound
	bytesSent := 0
	referer := "http://example.com"
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36"

	req := httptest.NewRequest(method, pathFooBar, http.NoBody)
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", ua)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode)

	pattern := fmt.Sprintf(`0\.0\.0\.0 - - \[\d{2}:\d{2}:\d{2}\] "%s %s %s" %d %d "%s" "%s"`, method, regexp.QuoteMeta(pathFooBar), httpProto, status, bytesSent, regexp.QuoteMeta(referer), regexp.QuoteMeta(ua)) //nolint:gocritic // double quoting for regex and string is not needed
	require.Regexp(t, pattern, buf.String())
}

func Test_Logger_Json_Format(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: JSONFormat,
		Stream: buf,
	}))

	method := fiber.MethodGet
	status := fiber.StatusNotFound
	ip := "0.0.0.0"
	bytesSent := 0

	req := httptest.NewRequest(method, pathFooBar, http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode)

	pattern := fmt.Sprintf(`\{"time":"\d{2}:\d{2}:\d{2}","ip":"%s","method":%q,"url":"%s","status":%d,"bytesSent":%d\}`, regexp.QuoteMeta(ip), method, regexp.QuoteMeta(pathFooBar), status, bytesSent) //nolint:gocritic // double quoting for regex and string is not needed
	require.Regexp(t, pattern, buf.String())
}

func Test_Logger_ECS_Format(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: ECSFormat,
		Stream: buf,
	}))

	method := fiber.MethodGet
	status := fiber.StatusNotFound
	ip := "0.0.0.0"
	bytesSent := 0
	msg := fmt.Sprintf("%s %s responded with %d", method, pathFooBar, status)

	req := httptest.NewRequest(method, pathFooBar, http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode)

	pattern := fmt.Sprintf(`\{"@timestamp":"\d{2}:\d{2}:\d{2}","ecs":\{"version":"1.6.0"\},"client":\{"ip":"%s"\},"http":\{"request":\{"method":%q,"url":"%s","protocol":%q\},"response":\{"status_code":%d,"body":\{"bytes":%d\}\}\},"log":\{"level":"INFO","logger":"fiber"\},"message":"%s"\}`, regexp.QuoteMeta(ip), method, regexp.QuoteMeta(pathFooBar), httpProto, status, bytesSent, regexp.QuoteMeta(msg)) //nolint:gocritic // double quoting for regex and string is not needed
	require.Regexp(t, pattern, buf.String())
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

	var latencyDuration time.Duration
	fixedStart := time.Unix(0, 0)
	logger := New(Config{
		Stream: buff,
		Format: "${latency}",
		LoggerFunc: func(c fiber.Ctx, data *Data, cfg *Config) error {
			data.Start = fixedStart
			data.Stop = fixedStart.Add(latencyDuration)
			return defaultLoggerInstance(c, data, cfg)
		},
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Define a test route
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the latency duration for the next iteration
		latencyDuration = tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
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

	var latencyDuration time.Duration
	fixedStart := time.Unix(0, 0)
	logger := New(Config{
		Stream: buff,
		LoggerFunc: func(c fiber.Ctx, data *Data, cfg *Config) error {
			data.Start = fixedStart
			data.Stop = fixedStart.Add(latencyDuration)
			return defaultLoggerInstance(c, data, cfg)
		},
	})
	app.Use(logger)

	// Define a list of time units to test
	timeUnits := getLatencyTimeUnits()

	// Define a test route
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// Loop through each time unit and assert that the log output contains the expected latency value
	for _, tu := range timeUnits {
		// Update the latency duration for the next iteration
		latencyDuration = tu.div

		// Create a new HTTP request to the test route
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody), fiber.TestConfig{
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/?foo=bar&baz=moz", http.NoBody))
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

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	expectedGetResponse := "Sample response body"
	require.Equal(t, expectedGetResponse, buf.String())

	buf.Reset() // Reset buffer to test POST
	_, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/test", http.NoBody))

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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "-2 0 200", buf.String())

	buf.Reset()
	resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/content", http.NoBody))
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
	wg.Go(func() {
		resp1, err1 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	})
	resp2, err2 = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	wg.Wait()

	require.NoError(t, err1)
	require.Equal(t, fiber.StatusOK, resp1.StatusCode)
	require.NoError(t, err2)
	require.Equal(t, fiber.StatusOK, resp2.StatusCode)
}

func Test_Logger_TimeUpdatesAfterInterval(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New(Config{
		Format:       "${time}",
		TimeFormat:   time.RFC3339Nano,
		TimeInterval: 10 * time.Millisecond,
		Stream:       &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
	first := buf.String()
	require.NotEmpty(t, first)

	var second string
	require.Eventually(t, func() bool {
		buf.Reset()

		resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusNoContent, resp.StatusCode)

		second = buf.String()
		return second != "" && second != first
	}, 200*time.Millisecond, 5*time.Millisecond)
}

func Test_Logger_SharedTimestampState(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("test/zone", 3600)
	first := sharedTimestamp(time.RFC3339, loc, 10*time.Millisecond)
	second := sharedTimestamp(time.RFC3339, loc, 10*time.Millisecond)
	third := sharedTimestamp(time.RFC3339Nano, loc, 10*time.Millisecond)

	require.Same(t, first, second)
	require.NotSame(t, first, third)
	loaded, ok := first.Load().(string)
	require.True(t, ok)
	require.NotEmpty(t, loaded)
}

// go test -run Test_Response_Header
func Test_Response_Header(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Set(fiber.HeaderXRequestID, "Hello fiber!")
		return c.Next()
	})
	app.Use(New(Config{
		Format: "${respHeader:X-Request-ID}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello fiber!", buf.String())
}

func Test_Logger_RegisteredTag(t *testing.T) {
	t.Parallel()

	const tag = "registered-test-tag"

	require.NoError(t, RegisterTag(tag, func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
		return output.WriteString("registered")
	}))

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${" + tag + "}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "registered", buf.String())
}

func Test_Logger_CustomTagOverridesRegisteredTag(t *testing.T) {
	t.Parallel()

	const tag = "registered-override-test-tag"

	require.NoError(t, RegisterTag(tag, func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
		return output.WriteString("registered")
	}))

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		Format: "${" + tag + "}",
		CustomTags: map[string]LogFunc{
			tag: func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
				return output.WriteString("override")
			},
		},
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "override", buf.String())
}

func Test_Logger_RegisterTagRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, RegisterTag("", func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
		return output.WriteString("ignored")
	}), ErrTagInvalid)
	require.ErrorIs(t, RegisterTag("missing", nil), ErrTagInvalid)
}

func Test_Logger_MustRegisterTagPanicsOnInvalidInput(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, ErrTagInvalid.Error(), func() {
		MustRegisterTag("", func(output Buffer, _ fiber.Ctx, _ *Data, _ string) (int, error) {
			return output.WriteString("ignored")
		})
	})
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
	headerReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
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
	reqHeaderReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	reqHeaderReq.Header.Add("test", "Hello fiber!")

	resp, err := app.Test(reqHeaderReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, customTag, buf.String())
}

func Test_Logger_DataLegacyTemplateChains(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()
	app.Use(New(Config{
		DisableColors: true,
		Format:        "${method} ${path}",
		LoggerFunc: func(c fiber.Ctx, data *Data, _ *Config) error {
			if len(data.TemplateChain) != len(data.LogFuncChain) {
				return fmt.Errorf("template/log func chain length mismatch: template=%d logfunc=%d", len(data.TemplateChain), len(data.LogFuncChain))
			}
			for i, logFunc := range data.LogFuncChain {
				switch {
				case logFunc == nil:
					if _, err := buf.Write(data.TemplateChain[i]); err != nil {
						return fmt.Errorf("write template chain: %w", err)
					}
				case data.TemplateChain[i] == nil:
					if _, err := logFunc(buf, c, data, ""); err != nil {
						return err
					}
				default:
					if _, err := logFunc(buf, c, data, string(data.TemplateChain[i])); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello fiber!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "GET /", buf.String())
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	require.EqualValues(t, 1, *o)
}

// go test -run Test_Logger_ForceColors
func Test_Logger_ForceColors(t *testing.T) {
	t.Parallel()
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New()

	app.Use(New(Config{
		Format:        "${ip}${status}${method}${path}${error}\n",
		Stream:        buf,
		DisableColors: true,
		ForceColors:   true,
	}))

	// Alias colors
	colors := app.Config().ColorScheme

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	expected := fmt.Sprintf("0.0.0.0%s404%s%sGET%s/%sNot Found%s\n", colors.Yellow, colors.Reset, colors.Cyan, colors.Reset, colors.Red, colors.Reset)
	require.Equal(t, expected, buf.String())
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

	b.Run("DefaultFormatForceColors", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream:      io.Discard,
			ForceColors: true,
		}))
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("Hello, World!")
		})
		benchmarkSetup(bb, app, "/")
	})

	b.Run("DefaultFormatWithFiberLog", func(bb *testing.B) {
		app := fiber.New()
		logger := fiberlog.DefaultLogger[*log.Logger]()
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
			Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}",
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
		logger := fiberlog.DefaultLogger[*log.Logger]()
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

	b.Run("DefaultFormatForceColors", func(bb *testing.B) {
		app := fiber.New()
		app.Use(New(Config{
			Stream:      io.Discard,
			ForceColors: true,
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
			Format: "${pid}${reqHeaders}${referer}${scheme}${protocol}${ip}${ips}${host}${url}${ua}${body}${route}${black}${red}${green}${yellow}${blue}${magenta}${cyan}${white}${reset}${error}${reqHeader:test}${query:test}${form:test}${cookie:test}",
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

// failingBuffer is a Buffer whose WriteString starts failing after failAfter
// successful calls, so the error paths of writeSanitizedColored can be reached
// without a real failing sink.
type failingBuffer struct {
	*bytebufferpool.ByteBuffer
	failAfter int
	calls     int
}

var errWriteFailed = errors.New("write failed")

func (b *failingBuffer) WriteString(s string) (int, error) {
	if b.calls >= b.failAfter {
		return 0, errWriteFailed
	}
	b.calls++
	n, err := b.ByteBuffer.WriteString(s)
	if err != nil {
		return n, fmt.Errorf("failingBuffer write string: %w", err)
	}
	return n, nil
}

func (b *failingBuffer) Write(p []byte) (int, error) {
	if b.calls >= b.failAfter {
		return 0, errWriteFailed
	}
	b.calls++
	n, err := b.ByteBuffer.Write(p)
	if err != nil {
		return n, fmt.Errorf("failingBuffer write: %w", err)
	}
	return n, nil
}

// Test_writeSanitizedColored_PropagatesWriteErrors pins the short-circuits in
// writeSanitizedColored. A sink that fails partway must stop the sequence and
// surface the error rather than writing a reset escape with no matching color
// (or silently dropping the failure), and the byte count returned must reflect
// only what actually reached the sink.
func Test_writeSanitizedColored_PropagatesWriteErrors(t *testing.T) {
	t.Parallel()

	const color, value, reset = "<c>", "va\nlue", "<r>"

	t.Run("color write fails", func(t *testing.T) {
		t.Parallel()

		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 0}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := writeSanitizedColored(buf, color, value, reset)
		require.ErrorIs(t, err, errWriteFailed)
		require.Zero(t, n)
		require.Empty(t, buf.String(), "nothing may reach the sink once the color fails")
	})

	t.Run("value write fails", func(t *testing.T) {
		t.Parallel()

		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 1}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := writeSanitizedColored(buf, color, value, reset)
		require.ErrorIs(t, err, errWriteFailed)
		require.Equal(t, len(color), n, "only the color made it out")
		require.Equal(t, color, buf.String(), "the reset must not be written after a failure")
	})

	t.Run("reset write fails", func(t *testing.T) {
		t.Parallel()

		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 2}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := writeSanitizedColored(buf, color, value, reset)
		require.ErrorIs(t, err, errWriteFailed)
		require.Equal(t, len(color)+len(value), n, "the color and value made it out, the reset did not")
		require.Equal(t, "<c>va lue", buf.String())
	})

	t.Run("all writes succeed", func(t *testing.T) {
		t.Parallel()

		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 99}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := writeSanitizedColored(buf, color, value, reset)
		require.NoError(t, err)
		require.Equal(t, len(color)+len(value)+len(reset), n)
		require.Equal(t, "<c>va lue<r>", buf.String(),
			"the value is scrubbed but the color escapes pass through verbatim")
	})
}

// Test_Logger_SanitizesLocalsByType covers each arm of ${locals:}'s type
// switch. The []byte arm has its own scrubbing call, so a []byte local carrying
// CR/LF must be scrubbed just like the string one — otherwise a handler
// stashing raw request bytes in Locals would reopen the injection this scrubs.
func Test_Logger_SanitizesLocalsByType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		value    any
		name     string
		expected string
	}{
		{name: "bytes", value: []byte("a\r\nb"), expected: "a  b"},
		{name: "string", value: "a\r\nb", expected: "a  b"},
		{name: "nil", value: nil, expected: ""},
		{name: "other", value: struct{ V string }{"a\r\nb"}, expected: "{a  b}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app := fiber.New()
			app.Use(New(Config{Format: "${locals:demo}", Stream: buf}))
			app.Get("/", func(c fiber.Ctx) error {
				if tc.value != nil {
					c.Locals("demo", tc.value)
				}
				return c.SendStatus(fiber.StatusOK)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Equal(t, tc.expected, buf.String())
		})
	}
}

// Test_Logger_SanitizesControlBytes ensures user-controlled values cannot
// inject CR/LF (or other C0/DEL bytes) into a log line and forge log entries.
// See https://github.com/gofiber/fiber/issues/4341.
func Test_Logger_SanitizesControlBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		build    func() *http.Request
		expected string
	}{
		{
			name:   "query",
			format: "${query:q}",
			build: func() *http.Request {
				return httptest.NewRequest(fiber.MethodGet, "/?q=a%0d%0a200+GET+/legit", http.NoBody)
			},
			expected: "a  200 GET /legit",
		},
		{
			name:   "body",
			format: "${body}",
			build: func() *http.Request {
				req := httptest.NewRequest(fiber.MethodPost, "/", strings.NewReader("a\r\nb\x00c"))
				req.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
				return req
			},
			expected: "a  b c",
		},
		{
			name:   "tab is preserved",
			format: "${query:q}",
			build: func() *http.Request {
				return httptest.NewRequest(fiber.MethodGet, "/?q=a%09b", http.NoBody)
			},
			expected: "a\tb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app := fiber.New()
			app.Use(New(Config{Format: tt.format, Stream: buf}))
			app.Add([]string{fiber.MethodGet, fiber.MethodPost}, "/", func(c fiber.Ctx) error {
				return c.SendString("ok")
			})

			_, err := app.Test(tt.build())
			require.NoError(t, err)
			require.Equal(t, tt.expected, buf.String())
		})
	}
}

// Test_Logger_SanitizesPath covers the decoded-path case: with UnescapePath
// enabled c.Path() carries the percent-decoded bytes, so ${path} would
// otherwise emit raw CRLF.
func Test_Logger_SanitizesPath(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Use(New(Config{Format: "${path}", Stream: buf}))

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/admin%0d%0a200", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, "/admin  200", buf.String())
}

// Test_Logger_DefaultFormat_SanitizesControlBytes covers the default logger
// configuration, which takes defaultLoggerInstance's hand-written fast path
// instead of the tag map. Without scrubbing there, plain logger.New() — by far
// the most common setup — still lets a request forge extra log lines (#4341).
func Test_Logger_DefaultFormat_SanitizesControlBytes(t *testing.T) {
	t.Parallel()

	for _, colors := range []bool{false, true} {
		t.Run(fmt.Sprintf("colors=%v", colors), func(t *testing.T) {
			t.Parallel()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app := fiber.New(fiber.Config{UnescapePath: true})
			cfg := Config{Stream: buf}
			if colors {
				cfg.ForceColors = true
			} else {
				cfg.DisableColors = true
			}
			app.Use(New(cfg))
			app.Get("/*", func(_ fiber.Ctx) error {
				return errors.New("boom\r\nFORGED-ERROR-LINE")
			})

			_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/admin%0d%0aFORGED-PATH-LINE", http.NoBody))
			require.NoError(t, err)

			out := buf.String()
			require.NotContains(t, out, "\r")
			require.Equal(t, 1, strings.Count(out, "\n"), "log entry must stay on one line: %q", out)
			require.Contains(t, out, "FORGED-PATH-LINE")
			require.Contains(t, out, "FORGED-ERROR-LINE")
		})
	}
}

// Test_SanitizeValue covers the exported helper the docs point custom tags,
// context tags and LoggerFunc implementations at. It has to apply exactly what
// the built-in tags apply, or the guidance is wrong.
func Test_SanitizeValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   string
		want string
	}{
		{name: "clean passes through", in: "plain-value", want: "plain-value"},
		{name: "empty", in: "", want: ""},
		{name: "CRLF is neutralized", in: "a\r\nb", want: "a  b"},
		{name: "tab is preserved", in: "a\tb", want: "a\tb"},
		{name: "NUL and DEL", in: "a\x00b\x7fc", want: "a b c"},
		{name: "escape sequence", in: "a\x1b[31mb", want: "a [31mb"},
		// Documented limitation: only ASCII controls are replaced, so C1
		// controls (including NEL U+0085) survive.
		{name: "C1 NEL passes through", in: "a\u0085b", want: "a\u0085b"},
		{name: "multibyte is untouched", in: "h\u00e9llo", want: "h\u00e9llo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, SanitizeValue(tc.in))
			// The exported helper must not diverge from the internal one the
			// built-in tags use.
			require.Equal(t, sanitizeLogValue(tc.in), SanitizeValue(tc.in))
		})
	}
}

// Test_Logger_IPs_RepeatedFieldLines asserts ${ips} logs every X-Forwarded-For
// field line, not just the first.
//
// A recipient may combine repeated field lines into the comma-joined form (RFC
// 9110 §5.2), and Fiber's proxy-header accessors do exactly that — so reading
// only the first here logged a shorter chain than the one c.IP() and c.IPs()
// parsed, and the access log did not explain what was actually enforced.
func Test_Logger_IPs_RepeatedFieldLines(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app := fiber.New(fiber.Config{TrustProxy: true})
	app.Use(New(Config{Format: "${ips}", Stream: buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("hi") })

	raw := "GET / HTTP/1.1\r\nHost: example.com\r\n" +
		"X-Forwarded-For: 1.1.1.1\r\n" +
		"X-Forwarded-For: 2.2.2.2, 3.3.3.3\r\n\r\n"

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, nil, nil)
	app.Handler()(fctx)

	require.Equal(t, "1.1.1.1,2.2.2.2,3.3.3.3", buf.String(),
		"every field line must be logged, as the chain the framework parsed")
}

// Test_Logger_IPs_WithoutHeaderNormalizing asserts the tag logs the chain the
// framework acted on, whatever case the field name arrived in.
//
// Reading the header here rather than asking c.IPs() meant reading it a second
// way, and PeekAll compares stored names byte for byte — so under
// DisableHeaderNormalizing a lower-case "x-forwarded-for:" matched nothing and
// the tag logged an empty chain, while c.IPs() went on parsing it and the trust
// decisions went on using it.
func Test_Logger_IPs_WithoutHeaderNormalizing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		lines   []string
		disable bool
	}{
		{"normalized, one line", []string{"X-Forwarded-For: 1.1.1.1, 2.2.2.2"}, false},
		{"raw, lower-case name", []string{"x-forwarded-for: 1.1.1.1, 2.2.2.2"}, true},
		{"normalized, two lines", []string{"X-Forwarded-For: 1.1.1.1", "X-Forwarded-For: 2.2.2.2"}, false},
		{"raw, mixed-case names", []string{"x-forwarded-for: 1.1.1.1", "X-Forwarded-For: 2.2.2.2"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app := fiber.New(fiber.Config{
				TrustProxy:               true,
				DisableHeaderNormalizing: tc.disable,
			})
			app.Use(New(Config{Format: "${ips}", Stream: buf}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("hi") })

			parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
			for _, l := range tc.lines {
				parts = append(parts, l+"\r\n")
			}
			raw := strings.Join(append(parts, "\r\n"), "")

			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			if tc.disable {
				req.Header.DisableNormalizing()
			}
			require.NoError(t, req.Read(bufio.NewReader(strings.NewReader(raw))))

			fctx := &fasthttp.RequestCtx{}
			fctx.Init(req, nil, nil)
			app.Handler()(fctx)

			require.Equal(t, "1.1.1.1,2.2.2.2", buf.String(),
				"the log must show the chain the trust decisions used")
		})
	}
}

// Test_Logger_RegisterContextTag_Sanitizes pins that a value rendered by a
// registered context tag cannot close the log line it is written on.
//
// Every built-in tag passes through the sanitizer; this path did not, so a CR
// or LF in whatever the application pulled out of the context — in practice
// request data — started a second entry the reader has no way to tell from a
// real one.
func Test_Logger_RegisterContextTag_Sanitizes(t *testing.T) {
	t.Parallel()

	// Both registries are mutex-guarded and re-registering replaces rather
	// than fails, so this runs in parallel like every other registry test in
	// the package. The name has to be its own, though: registration is global
	// and outlives the test whether or not it is parallel.
	RegisterContextTag("sanitize-probe", func(_ any) string {
		return "before\r\nGET /forged HTTP/1.1\nafter"
	})

	var buf bytes.Buffer
	app := fiber.New()
	app.Use(New(Config{
		Format: "${sanitize-probe}\n",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	line := buf.String()
	require.NotContains(t, line, "\r", "a CR must not reach the log")
	require.Equal(t, 1, strings.Count(line, "\n"), "the entry must occupy one line: %q", line)
	require.Contains(t, line, "before")
	require.Contains(t, line, "after")
}

// Test_TagIPs_PropagatesWriteErrors pins the short-circuits in the ${ips} tag.
// A sink that fails partway must stop and surface the error rather than
// carrying on, and the count returned must reflect only what reached the sink.
func Test_TagIPs_PropagatesWriteErrors(t *testing.T) {
	t.Parallel()

	// A trusted proxy, so c.IPs() returns the forwarded chain rather than
	// nothing at all — the tag needs more than one entry to reach the
	// separator between them.
	app := fiber.New(fiber.Config{
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true},
	})
	tag := createTagMap(&ConfigDefault)[TagIPs]

	withChain := func(t *testing.T, chain string) fiber.Ctx {
		t.Helper()
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/")
		fctx.Request.Header.Set(fiber.HeaderXForwardedFor, chain)
		c := app.AcquireCtx(fctx)
		t.Cleanup(func() { app.ReleaseCtx(c) })
		return c
	}

	t.Run("the whole chain is written", func(t *testing.T) {
		t.Parallel()

		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)

		n, err := tag(buf, withChain(t, "1.1.1.1, 2.2.2.2"), nil, "")
		require.NoError(t, err)
		require.Equal(t, "1.1.1.1,2.2.2.2", buf.String())
		require.Equal(t, len(buf.String()), n)
	})

	t.Run("the first entry fails", func(t *testing.T) {
		t.Parallel()

		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 0}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := tag(buf, withChain(t, "1.1.1.1, 2.2.2.2"), nil, "")
		require.ErrorIs(t, err, errWriteFailed)
		require.Zero(t, n)
	})

	t.Run("the separator fails", func(t *testing.T) {
		t.Parallel()

		// One successful write for the first address, then the "," fails.
		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 1}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := tag(buf, withChain(t, "1.1.1.1, 2.2.2.2"), nil, "")
		require.ErrorIs(t, err, errWriteFailed)
		require.Equal(t, len("1.1.1.1"), n, "only the first address reached the sink")
	})

	t.Run("the second entry fails", func(t *testing.T) {
		t.Parallel()

		// The first address and the separator succeed; the second fails.
		buf := &failingBuffer{ByteBuffer: bytebufferpool.Get(), failAfter: 2}
		defer bytebufferpool.Put(buf.ByteBuffer)

		n, err := tag(buf, withChain(t, "1.1.1.1, 2.2.2.2"), nil, "")
		require.ErrorIs(t, err, errWriteFailed)
		require.Equal(t, len("1.1.1.1,"), n)
	})
}

func Test_Logger_ResBody_DoesNotDrainStream(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	app := fiber.New()
	app.Use(New(Config{
		Format: "[${resBody}]",
		Stream: &logged,
	}))
	app.Get("/events", func(c fiber.Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			w.WriteString("data: one\n\n") //nolint:errcheck // nothing to do with a stream-writer error here
			w.Flush()                      //nolint:errcheck // same
		})
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/events")
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	app.Handler()(fctx)

	require.True(t, fctx.Response.IsBodyStream(), "logging must leave a streamed response streamed")
	require.Equal(t, "[]", logged.String(), "and it logs nothing rather than the buffered stream")

	logged.Reset()
	buffered := fiber.New()
	buffered.Use(New(Config{Format: "[${resBody}]", Stream: &logged}))
	buffered.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	bctx := &fasthttp.RequestCtx{}
	bctx.Request.SetRequestURI("/")
	bctx.Request.Header.SetMethod(fiber.MethodGet)
	buffered.Handler()(bctx)

	require.Equal(t, "[hello]", logged.String())
}
