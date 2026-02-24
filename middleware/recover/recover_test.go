package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// go test -run Test_Config
func Test_Config(t *testing.T) {
	defaultConfig := configDefault()
	require.Nil(t, defaultConfig.Next)
	require.NotNil(t, defaultConfig.PanicHandler)
	require.NotNil(t, defaultConfig.StackTraceHandler)
	require.False(t, defaultConfig.EnableStackTrace)
}

// go test -run Test_Recover
func Test_Recover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		panicVal any
		errorMsg string
	}{
		{
			name:     "string panic will be handled",
			panicVal: "Hi, I'm an error!",
			errorMsg: "[RECOVERED]: Hi, I'm an error!",
		},
		{
			name:     "error panic will be handled",
			panicVal: errors.New("hi, I'm an error object"),
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

			app.Use(New(Config{PanicHandler: func(c fiber.Ctx, r any) error {
				return fmt.Errorf("[RECOVERED]: %w", defaultPanicHandler(c, r))
			}}))

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

	app.Get("/panicVal", func(_ fiber.Ctx) error {
		panic("Hi, I'm an error!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/panicVal", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
