package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// go test -run Test_Recover
func Test_Recover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		panicVal     any
		panicHandler func(c fiber.Ctx, r any) error
		errorMsg     string
	}{
		{
			name:         "non-error panic will be handled by default",
			panicVal:     "Hi, I'm an error!",
			panicHandler: nil,
			errorMsg:     "Hi, I'm an error!",
		},
		{
			name:         "error panic will be handled by default",
			panicVal:     errors.New("hi, I'm an error object"),
			panicHandler: nil,
			errorMsg:     "hi, I'm an error object",
		},
		{
			name:     "non-error panic will be handled",
			panicVal: "Hi, I'm an error!",
			panicHandler: func(c fiber.Ctx, r any) error {
				return fmt.Errorf("[RECOVERED]: %w", DefaultPanicHandler(c, r))
			},
			errorMsg: "[RECOVERED]: Hi, I'm an error!",
		},
		{
			name:     "error panic will be handled",
			panicVal: errors.New("hi, I'm an error object"),
			panicHandler: func(c fiber.Ctx, r any) error {
				return fmt.Errorf("[RECOVERED]: %w", DefaultPanicHandler(c, r))
			},
			errorMsg: "[RECOVERED]: hi, I'm an error object",
		},
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

			app.Get("/panic", func(_ fiber.Ctx) error {
				panic(tc.panicVal)
			})

			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/panic", http.NoBody))
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
		})
	}
}

// go test -run Test_Recover_Next
func Test_Recover_Next(t *testing.T) {
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

func Test_Recover_EnableStackTrace(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		EnableStackTrace: true,
	}))

	app.Get("/panic", func(_ fiber.Ctx) error {
		panic("Hi, I'm an error!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/panic", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

// Not parallel: swaps os.Stderr to capture what the default handler writes.
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
