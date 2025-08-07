package keyauth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_KeyAuth_ConfigDefault_NoConfig tests the case where no config is provided.
func Test_KeyAuth_ConfigDefault_NoConfig(t *testing.T) {
	t.Parallel()
	// The New function will call configDefault with no arguments
	// which will panic because ConfigDefault.Validator is nil.
	assert.PanicsWithValue(t, "fiber: keyauth middleware requires a validator function", func() {
		New()
	}, "Calling New() without a validator should panic")
}

// Test_KeyAuth_ConfigDefault_PanicWithoutValidator tests that configDefault panics when Validator is nil.
func Test_KeyAuth_ConfigDefault_PanicWithoutValidator(t *testing.T) {
	t.Parallel()
	assert.PanicsWithValue(t, "fiber: keyauth middleware requires a validator function", func() {
		configDefault(Config{})
	}, "configDefault should panic if validator is not provided")
}

// Test_KeyAuth_ConfigDefault_WithValidator tests that default values are set when only a validator is provided.
func Test_KeyAuth_ConfigDefault_WithValidator(t *testing.T) {
	t.Parallel()
	validator := func(fiber.Ctx, string) (bool, error) { return true, nil }
	cfg := configDefault(Config{
		Validator: validator,
	})

	require.NotNil(t, cfg.Validator)
	assert.Equal(t, ConfigDefault.Realm, cfg.Realm)
	require.NotNil(t, cfg.SuccessHandler)
	require.NotNil(t, cfg.ErrorHandler)

	// Check that the extractor is not nil, as it's set in New()
	assert.NotNil(t, cfg.Extractor.Extract)
}

// Test_KeyAuth_ConfigDefault_CustomConfig tests that custom values are preserved.
func Test_KeyAuth_ConfigDefault_CustomConfig(t *testing.T) {
	t.Parallel()
	nextFunc := func(_ fiber.Ctx) bool { return true }
	successHandler := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	errorHandler := func(c fiber.Ctx, _ error) error { return c.SendStatus(fiber.StatusForbidden) }
	validator := func(_ fiber.Ctx, _ string) (bool, error) { return true, nil }
	extractor := FromHeader("X-API-Key")

	cfg := configDefault(Config{
		Next:           nextFunc,
		SuccessHandler: successHandler,
		ErrorHandler:   errorHandler,
		Validator:      validator,
		Realm:          "API",
		Extractor:      extractor,
	})

	// Using reflect.ValueOf to compare function pointers
	assert.Equal(t, reflect.ValueOf(nextFunc).Pointer(), reflect.ValueOf(cfg.Next).Pointer())
	assert.Equal(t, reflect.ValueOf(successHandler).Pointer(), reflect.ValueOf(cfg.SuccessHandler).Pointer())
	assert.Equal(t, reflect.ValueOf(errorHandler).Pointer(), reflect.ValueOf(cfg.ErrorHandler).Pointer())
	assert.Equal(t, reflect.ValueOf(validator).Pointer(), reflect.ValueOf(cfg.Validator).Pointer())
	assert.Equal(t, reflect.ValueOf(extractor.Extract).Pointer(), reflect.ValueOf(cfg.Extractor.Extract).Pointer())

	assert.Equal(t, "API", cfg.Realm)
}

// Test_KeyAuth_ConfigDefault_DefaultErrorHandler tests the default error handler.
func Test_KeyAuth_ConfigDefault_DefaultErrorHandler(t *testing.T) {
	t.Parallel()
	validator := func(_ fiber.Ctx, s string) (bool, error) { return true, nil }

	t.Run("with default realm", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		cfg := configDefault(Config{
			Validator: validator,
		})
		app.Use(func(c fiber.Ctx) error {
			return cfg.ErrorHandler(c, ErrMissingOrMalformedAPIKey)
		})
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, `Bearer realm="Restricted"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
		assert.Equal(t, ErrMissingOrMalformedAPIKey.Error(), string(body))
	})

	t.Run("with custom realm", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		cfg := configDefault(Config{
			Validator: validator,
			Realm:     "CustomRealm",
		})
		app.Use(func(c fiber.Ctx) error {
			return cfg.ErrorHandler(c, errors.New("some other error"))
		})
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, `Bearer realm="CustomRealm"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
		assert.Equal(t, "Invalid or expired API Key", string(body))
	})
}

// Test_KeyAuth_New_DefaultExtractor tests that the default extractor is set correctly by New().
func Test_KeyAuth_New_DefaultExtractor(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	validator := func(_ fiber.Ctx, s string) (bool, error) { return s == "mykey", nil }

	// Let New create the default extractor
	handler := New(Config{Validator: validator})
	app.Use(handler)

	// Add a route to handle the request after the middleware succeeds
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer mykey")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
