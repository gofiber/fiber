package logger

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/mattn/go-colorable"
	"github.com/stretchr/testify/require"
)

// The colored default format needs ForceColors to be reachable from a test:
// writing to anything but os.Stdout switches colors off. Without it the whole
// fmt.Fprintf branch of defaultLoggerInstance is only touched by benchmarks.
func Test_Logger_DefaultFormat_Colors(t *testing.T) {
	t.Parallel()
	colors := fiber.DefaultColors
	// column is a column of the line: text in color, between the separators.
	column := func(color, text string) string { return "|" + color + text + colors.Reset + "|" }

	for _, tc := range []struct {
		name   string
		method string // defaults to GET
		want   string // the colored column that must appear in the line
		status int    // defaults to 200
	}{
		{name: "status 200", want: column(colors.Green, " 200 ")},
		{name: "status 301", status: fiber.StatusMovedPermanently, want: column(colors.Blue, " 301 ")},
		{name: "status 404", status: fiber.StatusNotFound, want: column(colors.Yellow, " 404 ")},
		{name: "status 500", status: fiber.StatusInternalServerError, want: column(colors.Red, " 500 ")},
		{name: "method GET", want: column(colors.Cyan, " GET     ")},
		{name: "method POST", method: fiber.MethodPost, want: column(colors.Green, " POST    ")},
		{name: "method PUT", method: fiber.MethodPut, want: column(colors.Yellow, " PUT     ")},
		{name: "method DELETE", method: fiber.MethodDelete, want: column(colors.Red, " DELETE  ")},
		{name: "method PATCH", method: fiber.MethodPatch, want: column(colors.White, " PATCH   ")},
		{name: "method HEAD", method: fiber.MethodHead, want: column(colors.Magenta, " HEAD    ")},
		{name: "method OPTIONS", method: fiber.MethodOptions, want: column(colors.Blue, " OPTIONS ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status := cmp.Or(tc.status, fiber.StatusOK)
			app, buf := loggerTestApp(t, &Config{ForceColors: true})
			app.All("/", func(c fiber.Ctx) error { return c.SendStatus(status) })

			apptest.Expect(t, app, apptest.Req(cmp.Or(tc.method, fiber.MethodGet), "/"), status)
			require.Contains(t, buf.String(), tc.want)
		})
	}
}

// The colored branch assembles the error suffix itself instead of going
// through the ${error} tag, so it needs its own scrubbing check: a raw CR/LF
// in the error would otherwise forge a second access-log line (#4341).
func Test_Logger_DefaultFormat_ColoredError(t *testing.T) {
	t.Parallel()
	colors := fiber.DefaultColors
	app, buf := loggerTestApp(t, &Config{ForceColors: true})
	app.Get("/", func(fiber.Ctx) error { return errors.New("boom\r\n200 | forged") })

	apptest.Expect(t, app, "/", fiber.StatusInternalServerError)

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
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_beforeHandlerFunc_StdoutColorSelection(t *testing.T) {
	stdoutCfg := func() *Config { return &Config{Stream: os.Stdout, areColorsEnabled: true} }
	isPlain := func(w io.Writer) bool {
		_, ok := w.(*colorable.NonColorable)
		return ok
	}

	for _, tc := range []struct {
		name        string
		term        string
		noColor     string
		forceColors bool
		wantPlain   bool
	}{
		{name: "NO_COLOR strips colors", term: "xterm", noColor: "1", wantPlain: true},
		{name: "dumb terminal strips colors", term: "dumb", noColor: "", wantPlain: true},
		{name: "ForceColors overrides both", term: "dumb", noColor: "1", forceColors: true, wantPlain: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			t.Setenv("NO_COLOR", tc.noColor)

			cfg := stdoutCfg()
			cfg.ForceColors = tc.forceColors
			beforeHandlerFunc(cfg)
			require.Equal(t, tc.wantPlain, isPlain(cfg.Stream))
		})
	}

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

	t.Run("nil config", func(t *testing.T) { require.NotPanics(t, func() { beforeHandlerFunc(nil) }) })
}
