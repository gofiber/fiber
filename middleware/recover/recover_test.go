package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

func Test_Recover(t *testing.T) {
	t.Parallel()
	recovered := func(c fiber.Ctx, r any) error { return fmt.Errorf("[RECOVERED]: %w", DefaultPanicHandler(c, r)) }
	errPanic := errors.New("hi, I'm an error object")
	tests := []struct {
		name         string
		panicVal     any
		panicHandler func(c fiber.Ctx, r any) error
		errorMsg     string
	}{
		{"non-error panic will be handled by default", "Hi, I'm an error!", nil, "Hi, I'm an error!"},
		{"error panic will be handled by default", errPanic, nil, "hi, I'm an error object"},
		{"non-error panic will be handled", "Hi, I'm an error!", recovered, "[RECOVERED]: Hi, I'm an error!"},
		{"error panic will be handled", errPanic, recovered, "[RECOVERED]: hi, I'm an error object"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{
				ErrorHandler: func(c fiber.Ctx, err error) error {
					require.Equal(t, tc.errorMsg, err.Error())
					return c.SendStatus(fiber.StatusTeapot)
				},
			})

			app.Use(New(Config{PanicHandler: tc.panicHandler}))
			app.Get("/panic", func(_ fiber.Ctx) error { panic(tc.panicVal) })
			apptest.Expect(t, app, "/panic", fiber.StatusTeapot)
		})
	}
}

func Test_Recover_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

func Test_Recover_EnableStackTrace(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{EnableStackTrace: true}))
	app.Get("/panic", func(_ fiber.Ctx) error { panic("Hi, I'm an error!") })

	apptest.Expect(t, app, "/panic", fiber.StatusInternalServerError)
}

// Not parallel: swaps os.Stderr to capture what the default handler writes.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Recover_DefaultStackTraceHandlerOutput(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	defaultStackTraceHandler(nil, "Hi, I'm an error!")
	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	require.Contains(t, string(out), "recovered panic: Hi, I'm an error!")
	require.Contains(t, string(out), "goroutine")
	// gotestsum reads a leading "panic: " as a crashed run and then skips
	// --rerun-fails for every package, so the recovered panic must not use it.
	require.False(t, strings.HasPrefix(string(out), "panic: "), "output must not look like a fatal panic")
}
