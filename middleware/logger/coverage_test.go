package logger

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
)

// Test_methodColor_AllMethods exercises every branch of methodColor, including
// the nil-colors short circuit and the default case.
func Test_methodColor_AllMethods(t *testing.T) {
	t.Parallel()

	colors := &fiber.DefaultColors

	require.Empty(t, methodColor(fiber.MethodGet, nil))

	cases := map[string]string{
		fiber.MethodGet:     colors.Cyan,
		fiber.MethodQuery:   colors.Cyan,
		fiber.MethodPost:    colors.Green,
		fiber.MethodPut:     colors.Yellow,
		fiber.MethodDelete:  colors.Red,
		fiber.MethodPatch:   colors.White,
		fiber.MethodHead:    colors.Magenta,
		fiber.MethodOptions: colors.Blue,
		"UNKNOWN":           colors.Reset,
	}
	for method, want := range cases {
		require.Equal(t, want, methodColor(method, colors), "method %q", method)
	}
}

// Test_statusColor_AllRanges exercises every branch of statusColor, including
// the nil-colors short circuit and each status range.
func Test_statusColor_AllRanges(t *testing.T) {
	t.Parallel()

	colors := &fiber.DefaultColors

	require.Empty(t, statusColor(fiber.StatusOK, nil))

	require.Equal(t, colors.Green, statusColor(fiber.StatusOK, colors))
	require.Equal(t, colors.Blue, statusColor(fiber.StatusMovedPermanently, colors))
	require.Equal(t, colors.Yellow, statusColor(fiber.StatusBadRequest, colors))
	require.Equal(t, colors.Red, statusColor(fiber.StatusInternalServerError, colors))
}

// Test_customLoggerWriter_InvalidLevel verifies the default branch of Write
// returns (0, nil) for a level outside the supported set. An unsupported level
// routes to the default case before loggerInstance is ever touched, so a nil
// instance is safe and we avoid mutating any shared/global logger state.
func Test_customLoggerWriter_InvalidLevel(t *testing.T) {
	t.Parallel()

	cl := &customLoggerWriter[any]{
		level: fiberlog.LevelFatal,
	}

	n, err := cl.Write([]byte("ignored"))
	require.NoError(t, err)
	require.Zero(t, n)
}

// Test_loadTimestamp_Empty verifies loadTimestamp returns an empty string when
// the atomic value has never been stored.
func Test_loadTimestamp_Empty(t *testing.T) {
	t.Parallel()

	var v atomic.Value
	require.Empty(t, loadTimestamp(&v))
}

// Test_UnknownTagError_WithHint covers the Hint branch of Error().
func Test_UnknownTagError_WithHint(t *testing.T) {
	t.Parallel()

	err := &UnknownTagError{Tag: "method", Hint: "did you mean ${method}?"}
	require.Contains(t, err.Error(), "did you mean ${method}?")
	require.Contains(t, err.Error(), `"method"`)
}

// Test_RegisterContextTag verifies that a context tag registered through the
// public helper renders in a logger format and panics on invalid input.
func Test_RegisterContextTag(t *testing.T) {
	t.Parallel()

	const tagName = "cov-ctx-tag"
	RegisterContextTag(tagName, func(_ any) string {
		return "rendered-value"
	})

	buf := bytes.NewBuffer(nil)
	app := fiber.New()
	app.Use(New(Config{
		Format: "${" + tagName + "}\n",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "rendered-value")

	require.Panics(t, func() {
		RegisterContextTag("", func(_ any) string { return "" })
	})
	require.Panics(t, func() {
		RegisterContextTag("name", nil)
	})
}

// Test_RegisterContextTag_EmptyValue ensures the empty-return path of the
// registered renderer writes nothing.
func Test_RegisterContextTag_EmptyValue(t *testing.T) {
	t.Parallel()

	const tagName = "cov-ctx-empty"
	RegisterContextTag(tagName, func(_ any) string {
		return ""
	})

	buf := bytes.NewBuffer(nil)
	app := fiber.New()
	app.Use(New(Config{
		Format: "[${" + tagName + "}]\n",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "[]")
}

// Test_Logger_PreRegisteredMiddlewareTag exercises emptyLogTag: a format that
// references a built-in middleware tag (api-key) compiles and renders nothing
// when the producing middleware has not registered a value.
func Test_Logger_PreRegisteredMiddlewareTag(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBuffer(nil)
	app := fiber.New()
	app.Use(New(Config{
		Format: "[${" + fiberlog.TagAPIKey + "}]\n",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "[]")
}

// Test_Logger_New_TimeDoneUpdater verifies the New path that starts a dedicated
// timestamp updater when TimeDone is configured (covers startTimestampUpdater).
func Test_Logger_New_TimeDoneUpdater(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	defer close(done)

	buf := bytes.NewBuffer(nil)
	app := fiber.New()
	app.Use(New(Config{
		Format:       "${time}\n",
		TimeFormat:   time.RFC3339Nano,
		TimeInterval: 5 * time.Millisecond,
		TimeDone:     done,
		Stream:       buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// The format always emits a trailing newline, so a non-empty buffer alone
	// would pass even if ${time} rendered empty. Trim it and assert the rendered
	// timestamp is present and well-formed, proving the updater path ran.
	rendered := strings.TrimRight(buf.String(), "\n")
	require.NotEmpty(t, rendered)
	_, parseErr := time.Parse(time.RFC3339Nano, rendered)
	require.NoError(t, parseErr)
}

func Test_sanitizeLog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "hello"},
		{"hello\nworld", "hello world"},
		{"hello\rworld", "hello world"},
		{"hello\x00world", "hello world"},
		{"\x1b[31mred\x1b[0m", " [31mred [0m"},
		{"tab\there", "tab\there"}, // tab is preserved
		{"clean string", "clean string"},
		{"\x7fdelete", " delete"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, sanitizeLog(tt.input), "input: %q", tt.input)
	}
}

func Test_sanitizeLogBytes(t *testing.T) {
	tests := []struct {
		input    []byte
		expected []byte
	}{
		{[]byte(""), []byte("")},
		{[]byte("hello"), []byte("hello")},
		{[]byte("hello\nworld"), []byte("hello world")},
		{[]byte("hello\x00world"), []byte("hello world")},
		{[]byte("tab\there"), []byte("tab\there")}, // tab is preserved
		{[]byte("\x7fdelete"), []byte(" delete")},
	}
	for _, tt := range tests {
		original := make([]byte, len(tt.input))
		copy(original, tt.input)
		result := sanitizeLogBytes(tt.input)
		require.Equal(t, tt.expected, result, "input: %q", tt.input)
		require.Equal(t, original, tt.input, "original should not be mutated")
	}
}

func Test_sanitizeLog_ZeroAllocOnCleanInput(t *testing.T) {
	input := "GET /health?check=true HTTP/1.1"
	allocs := testing.AllocsPerRun(100, func() {
		sanitizeLog(input)
	})
	require.Equal(t, float64(0), allocs, "sanitizeLog must not allocate on clean input")
}

func Test_sanitizeLogBytes_ZeroAllocOnCleanInput(t *testing.T) {
	input := []byte("GET /health?check=true HTTP/1.1")
	allocs := testing.AllocsPerRun(100, func() {
		sanitizeLogBytes(input)
	})
	require.Equal(t, float64(0), allocs, "sanitizeLogBytes must not allocate on clean input")
}

func Test_Logger_HeaderInjectionPrevented(t *testing.T) {
	app := fiber.New()
	var buf bytes.Buffer
	app.Use(New(Config{
		Format: "${reqHeader:X-Evil}\n",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Evil", "value\ninjected-header: evil")
	_, err := app.Test(req)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "value injected-header: evil")
}

func Test_Logger_QueryInjectionPrevented(t *testing.T) {
	app := fiber.New()
	var buf bytes.Buffer
	app.Use(New(Config{
		Format: "${query:foo}\n",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(200) })

	// Fiber decodes %0a in query param values to a raw newline.
	req := httptest.NewRequest(http.MethodGet, "/?foo=bar%0ainjected", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "bar injected")
}

func Test_Logger_PathInjectionPrevented(t *testing.T) {
	app := fiber.New()
	var buf bytes.Buffer
	app.Use(New(Config{
		Format: "${path}\n",
		Stream: &buf,
	}))
	app.Get("/*", func(c fiber.Ctx) error { return c.SendStatus(200) })

	// Fiber does NOT decode %0a in c.Path() — it stays percent-encoded.
	// The sanitizer must not break the encoded form.
	req := httptest.NewRequest(http.MethodGet, "/safe%0apath", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	logged := buf.String()
	require.NotContains(t, logged, "\n\n", "raw newline must not appear in log")
	require.Contains(t, logged, "/safe%0apath", "percent-encoded form must be preserved")
}

func Test_Logger_BodyInjectionPrevented(t *testing.T) {
	app := fiber.New()
	var buf bytes.Buffer
	app.Use(New(Config{
		Format: "${body}\n",
		Stream: &buf,
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello\ninjected"))
	req.Header.Set("Content-Type", "text/plain")
	_, err := app.Test(req)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "hello injected")
}

func Test_Logger_ErrorInjectionPrevented(t *testing.T) {
	app := fiber.New()
	var buf bytes.Buffer
	app.Use(New(Config{
		Format: "${error}\n",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return errors.New("bad input\ninjected-log: evil")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "bad input injected-log: evil")
}

func Benchmark_sanitizeLog_Clean(b *testing.B) {
	input := "GET /api/v1/users?page=1&limit=10 HTTP/1.1"
	b.ReportAllocs()
	for b.Loop() {
		sanitizeLog(input)
	}
}

func Benchmark_sanitizeLog_Dirty(b *testing.B) {
	input := "GET /api/v1/users?page=1\nX-Injected: evil HTTP/1.1"
	b.ReportAllocs()
	for b.Loop() {
		sanitizeLog(input)
	}
}
