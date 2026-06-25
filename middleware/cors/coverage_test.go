package cors

import (
	"bytes"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_CORS_setSimpleHeaders_NilConfig ensures the nil-config guard returns
// without panicking or mutating the response headers.
func Test_CORS_setSimpleHeaders_NilConfig(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	require.NotPanics(t, func() {
		setSimpleHeaders(c, "https://example.com", nil)
	})
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

// Test_CORS_setSimpleHeaders_WildcardWithCredentials verifies that when
// AllowCredentials is true and the resolved origin is "*", the middleware logs
// a warning and still reflects the value (the configuration is considered
// invalid, but the header is set to surface the misconfiguration).
func Test_CORS_setSimpleHeaders_WildcardWithCredentials(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	setSimpleHeaders(c, "*", &Config{AllowCredentials: true})

	require.Equal(t, "*", string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Contains(t, buf.String(), "'AllowCredentials' is true, but 'AllowOrigins' cannot be set to '*'")
}

// Test_CORS_setPreflightHeaders_NilConfig ensures the preflight helper tolerates
// a nil config (delegating to setSimpleHeaders and skipping MaxAge handling).
func Test_CORS_setPreflightHeaders_NilConfig(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	require.NotPanics(t, func() {
		setPreflightHeaders(c, "https://example.com", "600", nil)
	})
	require.Empty(t, string(c.Response().Header.Peek(fiber.HeaderAccessControlMaxAge)))
}
