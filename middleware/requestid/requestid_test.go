package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// go test -run Test_RequestID
func Test_RequestID(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World 👋!")
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	reqid := resp.Header.Get(fiber.HeaderXRequestID)
	require.Len(t, reqid, 43)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add(fiber.HeaderXRequestID, reqid)

	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, reqid, resp.Header.Get(fiber.HeaderXRequestID))
}

func Test_RequestID_InvalidHeaderValue(t *testing.T) {
	t.Parallel()

	rid := sanitizeRequestID("bad\r\nid", func() string {
		return "clean-generated-id"
	})

	require.Equal(t, "clean-generated-id", rid)
}

func Test_RequestID_InvalidGeneratedValue(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return "bad\r\nid"
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	rid := resp.Header.Get(fiber.HeaderXRequestID)
	require.NotEmpty(t, rid)
	require.NotContains(t, rid, "\r")
	require.NotContains(t, rid, "\n")
	require.Len(t, rid, 43, "Fallback should produce a SecureToken")
}

func Test_RequestID_GeneratorAlwaysInvalid(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return "invalid\x00id" // Always invalid due to null byte
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	rid := resp.Header.Get(fiber.HeaderXRequestID)
	require.NotEmpty(t, rid)
	require.Len(t, rid, 43, "Should fall back to SecureToken after 3 invalid attempts")
}

func Test_RequestID_CustomGenerator(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return "custom-valid-id"
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	rid := resp.Header.Get(fiber.HeaderXRequestID)
	require.Equal(t, "custom-valid-id", rid)
}

func Test_isValidRequestID_VisibleASCII(t *testing.T) {
	t.Parallel()

	require.True(t, isValidRequestID("request-id-09AZaz ~"))
}

func Test_isValidRequestID_Boundaries(t *testing.T) {
	t.Parallel()

	t.Run("allows space and tilde", func(t *testing.T) {
		t.Parallel()

		require.True(t, isValidRequestID(" ~"))
	})

	t.Run("rejects out of range", func(t *testing.T) {
		t.Parallel()

		require.False(t, isValidRequestID(string([]byte{0x1f})))
		require.False(t, isValidRequestID(string([]byte{0x7f})))
	})

	t.Run("rejects empty", func(t *testing.T) {
		t.Parallel()

		require.False(t, isValidRequestID(""))
	})
}

func Test_isValidRequestID_RejectsObsText(t *testing.T) {
	t.Parallel()

	require.False(t, isValidRequestID("valid\xff"))
}

// go test -run Test_RequestID_Next
func Test_RequestID_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Empty(t, resp.Header.Get(fiber.HeaderXRequestID))
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -run Test_RequestID_Locals
func Test_RequestID_FromContext(t *testing.T) {
	t.Parallel()
	reqID := "ThisIsARequestId"

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return reqID
		},
	}))

	var ctxVal string

	app.Use(func(c fiber.Ctx) error {
		ctxVal = FromContext(c)
		return c.Next()
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, reqID, ctxVal)
}

func Test_RequestID_FromContext_Empty(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	// No middleware

	app.Use(func(c fiber.Ctx) error {
		ctxVal := FromContext(c)
		require.Empty(t, ctxVal)
		return c.SendStatus(fiber.StatusOK)
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
}

// go test -run Test_RequestID_FromStdContext
func Test_RequestID_FromStdContext(t *testing.T) {
	t.Parallel()
	reqID := "ThisIsARequestId"

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return reqID
		},
	}))

	var ctxVal string

	app.Use(func(c fiber.Ctx) error {
		// Retrieve request ID from the standard context.Context,
		// simulating what a service layer would do.
		ctxVal = FromStdContext(c.Context())
		return c.Next()
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, reqID, ctxVal)
}

// go test -run Test_RequestID_FromStdContext_Empty
func Test_RequestID_FromStdContext_Empty(t *testing.T) {
	t.Parallel()

	// FromStdContext on a plain context should return empty string
	ctxVal := FromStdContext(context.Background())
	require.Empty(t, ctxVal)
}

// go test -run Test_RequestID_FromStdContext_WrappedContext
func Test_RequestID_FromStdContext_WrappedContext(t *testing.T) {
	t.Parallel()
	reqID := "WrappedContextRequestId"

	app := fiber.New()
	app.Use(New(Config{
		Generator: func() string {
			return reqID
		},
	}))

	var ctxVal string

	app.Use(func(c fiber.Ctx) error {
		// Wrap the context further (simulating passing through layers)
		stdCtx := c.Context()
		wrappedCtx := context.WithValue(stdCtx, "some-other-key", "some-value")

		// The request ID should still be retrievable from the wrapped context
		ctxVal = FromStdContext(wrappedCtx)
		return c.Next()
	})

	_, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, reqID, ctxVal)
}
