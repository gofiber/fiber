package earlydata

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

const (
	headerName   = "Early-Data"
	headerValOn  = "1"
	headerValOff = "0"
)

func appWithConfig(t *testing.T, c *fiber.Config) *fiber.App {
	t.Helper()
	t.Parallel()

	var app *fiber.App
	if c == nil {
		app = fiber.New()
	} else {
		app = fiber.New(*c)
	}

	app.Use(New())

	// Middleware to test IsEarly func
	const localsKeyTestValid = "earlydata_testvalid"
	app.Use(func(c fiber.Ctx) error {
		isEarly := IsEarly(c)

		switch h := c.Get(headerName); h {
		case "", headerValOff:
			if isEarly {
				return errors.New("is early-data even though it's not")
			}

		case headerValOn:
			switch {
			case fiber.IsMethodSafe(c.Method()):
				if !isEarly {
					return errors.New("should be early-data on safe HTTP methods")
				}
			default:
				if isEarly {
					return errors.New("early-data unsupported on unsafe HTTP methods")
				}
			}

		default:
			return fmt.Errorf("header has unsupported value: %s", h)
		}

		_ = c.Locals(localsKeyTestValid, true)

		return c.Next()
	})

	app.Add([]string{
		fiber.MethodGet,
		fiber.MethodPost,
	}, "/", func(c fiber.Ctx) error {
		valid, ok := c.Locals(localsKeyTestValid).(bool)
		if !ok {
			panic(errors.New("failed to type-assert to bool"))
		}
		if !valid {
			return errors.New("handler called even though validation failed")
		}

		return nil
	})

	return app
}

// go test -run Test_EarlyData
func Test_EarlyData(t *testing.T) {
	t.Parallel()

	trustedRun := func(t *testing.T, app *fiber.App) {
		t.Helper()

		{
			req := httptest.NewRequest(fiber.MethodGet, "/", nil)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			req.Header.Set(headerName, headerValOff)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			req.Header.Set(headerName, headerValOn)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)
		}

		{
			req := httptest.NewRequest(fiber.MethodPost, "/", nil)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			req.Header.Set(headerName, headerValOff)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			req.Header.Set(headerName, headerValOn)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)
		}
	}

	untrustedRun := func(t *testing.T, app *fiber.App) {
		t.Helper()

		{
			req := httptest.NewRequest(fiber.MethodGet, "/", nil)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)

			req.Header.Set(headerName, headerValOff)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)

			req.Header.Set(headerName, headerValOn)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)
		}

		{
			req := httptest.NewRequest(fiber.MethodPost, "/", nil)

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)

			req.Header.Set(headerName, headerValOff)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)

			req.Header.Set(headerName, headerValOn)
			resp, err = app.Test(req)
			require.NoError(t, err)
			require.Equal(t, fiber.StatusTooEarly, resp.StatusCode)
		}
	}

	t.Run("empty config", func(t *testing.T) {
		app := appWithConfig(t, nil)
		trustedRun(t, app)
	})
	t.Run("default config", func(t *testing.T) {
		app := appWithConfig(t, &fiber.Config{})
		trustedRun(t, app)
	})

	t.Run("config with TrustProxy", func(t *testing.T) {
		app := appWithConfig(t, &fiber.Config{
			TrustProxy: true,
		})
		untrustedRun(t, app)
	})
	t.Run("config with TrustProxy and trusted TrustProxyConfig.Proxies", func(t *testing.T) {
		app := appWithConfig(t, &fiber.Config{
			TrustProxy: true,
			TrustProxyConfig: fiber.TrustProxyConfig{
				Proxies: []string{
					"0.0.0.0",
				},
			},
		})
		trustedRun(t, app)
	})
}

// Test_EarlyDataNext verifies that the middleware skips its logic when Next returns true.
func Test_EarlyDataNext(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Next: func(fiber.Ctx) bool { return true },
	}))

	called := false
	app.Get("/", func(c fiber.Ctx) error {
		called = true
		if IsEarly(c) {
			return errors.New("next path should not set early flag")
		}
		return nil
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(headerName, headerValOn)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.True(t, called)
}

// Test_configDefault_NoConfig verifies that calling configDefault without
// providing a configuration returns ConfigDefault as-is.
func Test_configDefault_NoConfig(t *testing.T) {
	t.Parallel()
	cfg := configDefault()
	require.Equal(t, ConfigDefault.Error, cfg.Error)
	require.Equal(t, reflect.ValueOf(ConfigDefault.IsEarlyData).Pointer(), reflect.ValueOf(cfg.IsEarlyData).Pointer())
	require.Equal(t, reflect.ValueOf(ConfigDefault.AllowEarlyData).Pointer(), reflect.ValueOf(cfg.AllowEarlyData).Pointer())
}

// Test_configDefault_WithConfig verifies that provided configuration fields are
// kept while missing fields are populated with defaults.
func Test_configDefault_WithConfig(t *testing.T) {
	t.Parallel()
	expectedErr := errors.New("boom")
	called := false
	custom := Config{
		Next:  func(c fiber.Ctx) bool { called = true; return false },
		Error: expectedErr,
	}

	cfg := configDefault(custom)

	// Next should be preserved and not invoked by configDefault.
	require.False(t, called)
	require.Equal(t, reflect.ValueOf(custom.Next).Pointer(), reflect.ValueOf(cfg.Next).Pointer())
	// Custom error must be preserved.
	require.Equal(t, expectedErr, cfg.Error)
	// Missing fields should be set to defaults.
	require.NotNil(t, cfg.IsEarlyData)
	require.NotNil(t, cfg.AllowEarlyData)
	require.Equal(t, custom.Error, cfg.Error)

	// Verify default functions behave as expected.
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	c.Request().Header.Set(DefaultHeaderName, DefaultHeaderTrueValue)
	c.Request().Header.SetMethod(fiber.MethodGet)
	require.True(t, cfg.IsEarlyData(c))
	require.True(t, cfg.AllowEarlyData(c))
	app.ReleaseCtx(c)
}
