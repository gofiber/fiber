package logger

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/mattn/go-colorable"
	"github.com/stretchr/testify/require"
)

// The colored default format needs ForceColors to be reachable from a test:
// writing to anything but os.Stdout switches colors off. Without it the whole
// fmt.Fprintf branch of defaultLoggerInstance is only touched by benchmarks.
func Test_Logger_DefaultFormat_StatusColors(t *testing.T) {
	t.Parallel()

	colors := fiber.DefaultColors

	cases := []struct {
		want   string
		status int
	}{
		{colors.Green, fiber.StatusOK},
		{colors.Blue, fiber.StatusMovedPermanently},
		{colors.Yellow, fiber.StatusNotFound},
		{colors.Red, fiber.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			t.Parallel()

			buf := bytes.NewBuffer(nil)
			app := fiber.New()
			app.Use(New(Config{Stream: buf, ForceColors: true}))
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendStatus(tc.status)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, tc.status, resp.StatusCode)
			require.Contains(t, buf.String(), "|"+tc.want+fmt.Sprintf(" %3d ", tc.status)+colors.Reset+"|")
		})
	}
}

func Test_Logger_DefaultFormat_MethodColors(t *testing.T) {
	t.Parallel()

	colors := fiber.DefaultColors

	cases := []struct {
		method string
		want   string
	}{
		{fiber.MethodGet, colors.Cyan},
		{fiber.MethodPost, colors.Green},
		{fiber.MethodPut, colors.Yellow},
		{fiber.MethodDelete, colors.Red},
		{fiber.MethodPatch, colors.White},
		{fiber.MethodHead, colors.Magenta},
		{fiber.MethodOptions, colors.Blue},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			t.Parallel()

			buf := bytes.NewBuffer(nil)
			app := fiber.New()
			app.Use(New(Config{Stream: buf, ForceColors: true}))
			app.All("/", func(c fiber.Ctx) error {
				return c.SendString("ok")
			})

			resp, err := app.Test(httptest.NewRequest(tc.method, "/", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
			require.Contains(t, buf.String(), "|"+tc.want+fmt.Sprintf(" %-7s ", tc.method)+colors.Reset+"|")
		})
	}
}

// The colored branch assembles the error suffix itself instead of going
// through the ${error} tag, so it needs its own scrubbing check: a raw CR/LF
// in the error would otherwise forge a second access-log line (#4341).
func Test_Logger_DefaultFormat_ColoredError(t *testing.T) {
	t.Parallel()

	colors := fiber.DefaultColors
	buf := bytes.NewBuffer(nil)

	app := fiber.New()
	app.Use(New(Config{Stream: buf, ForceColors: true}))
	app.Get("/", func(fiber.Ctx) error {
		return errors.New("boom\r\n200 | forged")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	out := buf.String()
	require.Contains(t, out, colors.Red+" | boom  200 | forged"+colors.Reset)
	require.Equal(t, 1, strings.Count(out, "\n"), "scrubbed error must stay on one line")
}

// beforeHandlerFunc decides whether stdout keeps its escape sequences. Nothing
// pinned that choice so far, and the env overrides are what users reach for in
// CI logs.
//
// No t.Parallel() here or in the subtests: t.Setenv rejects both, and the env
// is what is under test. Same as prefork_test.go and middleware/envvar.
func Test_beforeHandlerFunc_StdoutColorSelection(t *testing.T) {
	stdoutCfg := func() *Config {
		return &Config{Stream: os.Stdout, areColorsEnabled: true}
	}
	isPlain := func(w io.Writer) bool {
		_, ok := w.(*colorable.NonColorable)
		return ok
	}

	t.Run("NO_COLOR strips colors", func(t *testing.T) {
		t.Setenv("TERM", "xterm")
		t.Setenv("NO_COLOR", "1")

		cfg := stdoutCfg()
		beforeHandlerFunc(cfg)
		require.True(t, isPlain(cfg.Stream))
	})

	t.Run("dumb terminal strips colors", func(t *testing.T) {
		t.Setenv("TERM", "dumb")
		t.Setenv("NO_COLOR", "")

		cfg := stdoutCfg()
		beforeHandlerFunc(cfg)
		require.True(t, isPlain(cfg.Stream))
	})

	t.Run("ForceColors overrides both", func(t *testing.T) {
		t.Setenv("TERM", "dumb")
		t.Setenv("NO_COLOR", "1")

		cfg := stdoutCfg()
		cfg.ForceColors = true
		beforeHandlerFunc(cfg)
		require.False(t, isPlain(cfg.Stream))
	})

	t.Run("custom stream is left alone", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		buf := bytes.NewBuffer(nil)
		cfg := &Config{Stream: buf, areColorsEnabled: true}
		beforeHandlerFunc(cfg)
		require.Same(t, buf, cfg.Stream)
	})

	t.Run("colors disabled is left alone", func(t *testing.T) {
		cfg := stdoutCfg()
		cfg.areColorsEnabled = false
		beforeHandlerFunc(cfg)
		require.Same(t, os.Stdout, cfg.Stream)
	})

	t.Run("nil config", func(t *testing.T) {
		require.NotPanics(t, func() { beforeHandlerFunc(nil) })
	})
}
