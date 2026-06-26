package logger

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
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
// returns (0, nil) for a level outside the supported set.
func Test_customLoggerWriter_InvalidLevel(t *testing.T) {
	t.Parallel()

	logger := fiberlog.DefaultLogger[*log.Logger]()
	logger.SetOutput(bytes.NewBuffer(nil))

	cl := &customLoggerWriter[*log.Logger]{
		loggerInstance: logger,
		level:          fiberlog.LevelFatal,
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
	require.NotEmpty(t, buf.String())
}
