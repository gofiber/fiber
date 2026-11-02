package earlydata

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

const (
	headerName   = "Early-Data"
	headerValOn  = "1"
	headerValOff = "0"
)

const (
	trustedRemoteAddr   = "0.0.0.0:1234"
	untrustedRemoteAddr = "203.0.113.1:1234"
)

func appWithConfig(t *testing.T, c *fiber.Config) *fiber.App {
	t.Helper()

	var app *fiber.App
	if c == nil {
		app = fiber.New()
	} else {
		app = fiber.New(*c)
	}

	app.Use(New())

	// Fail the request unless IsEarly agrees with the header and the method.
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

	app.Add([]string{fiber.MethodGet, fiber.MethodPost, fiber.MethodQuery}, "/", func(c fiber.Ctx) error {
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

type requestExpectation struct {
	method string
	header string
	status int
}

func executeExpectations(t *testing.T, app *fiber.App, remoteAddr string, expectations []requestExpectation) {
	t.Helper()

	for _, expectation := range expectations {
		req := apptest.Req(expectation.method, "/", apptest.WithHeaderIf(headerName, expectation.header))
		req.RemoteAddr = remoteAddr

		apptest.Expect(t, app, req, expectation.status)
	}
}

func Test_EarlyData(t *testing.T) {
	t.Parallel()
	// Early data on a safe method is only allowed from a trusted proxy, and on
	// any other method it never is. QUERY is safe (RFC 10008), so it behaves
	// like GET.
	expectations := func(trusted bool) []requestExpectation {
		safeEarly := fiber.StatusTooEarly
		if trusted {
			safeEarly = fiber.StatusOK
		}

		return []requestExpectation{
			{method: fiber.MethodGet, status: fiber.StatusOK},
			{method: fiber.MethodGet, header: headerValOff, status: fiber.StatusOK},
			{method: fiber.MethodGet, header: headerValOn, status: safeEarly},
			{method: fiber.MethodPost, status: fiber.StatusOK},
			{method: fiber.MethodPost, header: headerValOff, status: fiber.StatusOK},
			{method: fiber.MethodPost, header: headerValOn, status: fiber.StatusTooEarly},
			{method: fiber.MethodQuery, header: headerValOn, status: safeEarly},
		}
	}

	for _, tc := range []struct {
		config     *fiber.Config
		name       string
		remoteAddr string
		trusted    bool
	}{
		{name: "empty config", remoteAddr: untrustedRemoteAddr},
		{name: "default config", config: &fiber.Config{}, remoteAddr: untrustedRemoteAddr},
		{
			name: "config with TrustProxy and untrusted remote", remoteAddr: untrustedRemoteAddr,
			config: &fiber.Config{TrustProxy: true},
		},
		{
			name:       "config with TrustProxy and trusted TrustProxyConfig.Proxies",
			remoteAddr: trustedRemoteAddr, trusted: true,
			config: &fiber.Config{
				TrustProxy: true, TrustProxyConfig: fiber.TrustProxyConfig{Proxies: []string{"0.0.0.0"}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := appWithConfig(t, tc.config)
			executeExpectations(t, app, tc.remoteAddr, expectations(tc.trusted))
		})
	}
}

func Test_EarlyDataNext(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(fiber.Ctx) bool { return true }}))

	called := false
	app.Get("/", func(c fiber.Ctx) error {
		called = true
		if IsEarly(c) {
			return errors.New("IsEarly(c) should be false when Next returns true")
		}
		return nil
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set(headerName, headerValOn)
	apptest.Expect(t, app, req, fiber.StatusOK)
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
	custom := Config{Next: func(_ fiber.Ctx) bool { called = true; return false }, Error: expectedErr}

	cfg := configDefault(custom)

	// Next is preserved and not invoked by configDefault, as is the custom error.
	require.False(t, called)
	require.Equal(t, reflect.ValueOf(custom.Next).Pointer(), reflect.ValueOf(cfg.Next).Pointer())
	require.Equal(t, expectedErr, cfg.Error)
	// Missing fields are set to the defaults, which behave as documented.
	require.NotNil(t, cfg.IsEarlyData)
	require.NotNil(t, cfg.AllowEarlyData)
	c := apptest.Ctx(t, fiber.New())
	c.Request().Header.Set(DefaultHeaderName, DefaultHeaderTrueValue)
	c.Request().Header.SetMethod(fiber.MethodGet)
	require.True(t, cfg.IsEarlyData(c))
	require.True(t, cfg.AllowEarlyData(c))
}
