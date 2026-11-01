package requestid

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/stretchr/testify/require"
)

func Test_RequestID(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("Hello, World 👋!") })

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)
	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	require.Len(t, reqid, 43)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add(fiber.HeaderXRequestID, reqid)
	resp = apptest.Expect(t, app, req, fiber.StatusOK)
	require.Equal(t, reqid, resp.Header.Get(fiber.HeaderXRequestID))
}

func Test_RequestID_InvalidHeaderValue(t *testing.T) {
	t.Parallel()
	rid := sanitizeRequestID("bad\r\nid", func() string { return "clean-generated-id" })

	require.Equal(t, "clean-generated-id", rid)
}

// Test_RequestID_Generator covers the ID a custom Generator yields: used when
// it is a valid header value, replaced by a SecureToken when it is not.
func Test_RequestID_Generator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, generated string
		want            string // "" for a 43-byte SecureToken fallback
	}{
		{name: "invalid generated value", generated: "bad\r\nid"},
		// Always invalid due to null byte, so it falls back after 3 invalid attempts.
		{name: "generator always invalid", generated: "invalid\x00id"},
		{name: "custom generator", generated: "custom-valid-id", want: "custom-valid-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Generator: func() string { return tc.generated }}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

			rid := apptest.Expect(t, app, "/", fiber.StatusOK).Header.Get(fiber.HeaderXRequestID)
			if tc.want != "" {
				require.Equal(t, tc.want, rid)
				return
			}
			require.NotEmpty(t, rid)
			require.NotContains(t, rid, "\r")
			require.NotContains(t, rid, "\n")
			require.Len(t, rid, 43, "Fallback should produce a SecureToken")
		})
	}
}

// Test_isValidRequestID covers the visible ASCII a request ID may hold. The
// long inputs put valid and invalid bytes at whole-word, overlapping-tail-word
// and scalar-tail positions of the word-at-a-time scan.
func Test_isValidRequestID(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"request-id-09AZaz ~", true},
		{" ~", true}, // space and tilde are the bounds
		{string([]byte{0x1f}), false},
		{string([]byte{0x7f}), false},
		{"a\tb", false},
		{"", false},
		{"valid\xff", false}, // obs-text
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"abcdefgh", true},
		{"abcdefg\x1f", false},
		{"abcdefgh\x7f", false},
		{"abcdefghij\x00klm", false},
		{"ab\x00", false},
	} {
		t.Run(strconv.Quote(tc.in), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidRequestID(tc.in))
		})
	}
}

func Test_RequestID_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	resp := apptest.Do(t, app, "/")
	require.Empty(t, resp.Header.Get(fiber.HeaderXRequestID))
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_RequestID_FromContext(t *testing.T) {
	t.Parallel()
	reqID := "ThisIsARequestId"

	app := fiber.New()
	app.Use(New(Config{Generator: func() string { return reqID }}))

	var ctxVal string

	app.Use(func(c fiber.Ctx) error {
		ctxVal = FromContext(c)
		return c.Next()
	})

	apptest.Do(t, app, "/")
	require.Equal(t, reqID, ctxVal)
}

func Test_RequestID_LoggerMiddlewareTag(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	app := fiber.New()
	app.Use(New(Config{Generator: func() string { return "req-42" }}))
	app.Use(logger.New(logger.Config{Format: "${requestid} ${request-id}", Stream: &buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Equal(t, "req-42"+" req-42", buf.String())
}

// Test_RequestID_LogWithContextFormat runs serially because it mutates the
// package-global logger output and context format.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_RequestID_LogWithContextFormat(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, fiberlog.RequestIDFormat)

	app := fiber.New()
	app.Use(New(Config{Generator: func() string { return "req-42" }}))
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Contains(t, buf.String(), "[Info] [req-42] start")
}

func Test_RequestID_FromContext_Empty(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	// No middleware

	app.Use(func(c fiber.Ctx) error {
		ctxVal := FromContext(c)
		require.Empty(t, ctxVal)
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Do(t, app, "/")
}

func Test_RequestID_FromContext_Types(t *testing.T) {
	t.Parallel()
	reqID := "request-id-123"

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New(Config{Generator: func() string { return reqID }}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Equal(t, reqID, FromContext(c))
		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, reqID, FromContext(customCtx))
		require.Equal(t, reqID, FromContext(c.RequestCtx()))
		require.Equal(t, reqID, FromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

func Benchmark_isValidRequestID(b *testing.B) {
	inputs := []string{
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // UUID, the middleware's own format
		"trace-12345",
		"bad\x00id-aaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	var ok bool
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			ok = isValidRequestID(in)
		}
	}
	_ = ok
}
