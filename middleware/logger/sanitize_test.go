//nolint:depguard // Because we test logging :D
package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
)

// Test_Logger_SanitizesControlChars verifies that user-controlled tag values are
// stripped of ASCII control characters before they reach the log line. Without
// sanitization an attacker can embed CR/LF in request data (path, headers, ...)
// and forge additional log entries (log injection). See issue #4341.
func Test_Logger_SanitizesControlChars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		setup  func(req *fasthttp.Request)
		want   string
	}{
		{
			name:   "user-agent",
			format: "${ua}",
			setup: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderUserAgent, "evil\r\n200 GET /ok")
			},
			want: "evil  200 GET /ok",
		},
		{
			name:   "referer",
			format: "${referer}",
			setup: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderReferer, "http://x/\r\nfake")
			},
			want: "http://x/  fake",
		},
		{
			name:   "custom request header",
			format: "${reqHeader:X-Trace}",
			setup: func(req *fasthttp.Request) {
				req.Header.Set("X-Trace", "id\r\ninjected")
			},
			want: "id  injected",
		},
		{
			name:   "tab is preserved",
			format: "${ua}",
			setup: func(req *fasthttp.Request) {
				req.Header.Set(fiber.HeaderUserAgent, "col1\tcol2")
			},
			want: "col1\tcol2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app.Use(New(Config{
				Format: tc.format,
				Stream: buf,
			}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")
			tc.setup(&fctx.Request)
			h(fctx)

			got := buf.String()
			require.NotContains(t, got, "\r", "CR must be sanitized")
			require.NotContains(t, got, "\n", "LF must be sanitized")
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_Logger_SanitizesLocals ensures the locals tag, which can carry
// application data derived from user input, is also sanitized.
func Test_Logger_SanitizesLocals(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Use(New(Config{
		Format: "${locals:user}",
		Stream: buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Locals("user", "admin\r\nrole=root")
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	h(fctx)

	got := buf.String()
	require.NotContains(t, got, "\n")
	require.Equal(t, "admin  role=root", got)
}

// Test_Logger_SanitizesError verifies error messages, which frequently echo
// user input, do not allow control characters through (with and without colors).
func Test_Logger_SanitizesError(t *testing.T) {
	t.Parallel()

	for _, colors := range []bool{false, true} {
		t.Run("colors="+boolString(colors), func(t *testing.T) {
			t.Parallel()

			app := fiber.New()

			buf := bytebufferpool.Get()
			defer bytebufferpool.Put(buf)

			app.Use(New(Config{
				Format:      "${error}",
				ForceColors: colors,
				Stream:      buf,
			}))
			app.Get("/", func(_ fiber.Ctx) error {
				return fiber.NewError(fiber.StatusInternalServerError, "boom\r\nINJECTED")
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")
			h(fctx)

			got := buf.String()
			require.NotContains(t, got, "\r")
			require.NotContains(t, got, "\n")
			require.Contains(t, got, "boom  INJECTED")
		})
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
