package fiber_test

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type integrationCustomCtx struct {
	fiber.DefaultCtx
}

func newIntegrationCustomCtx(app *fiber.App) fiber.CustomCtx {
	return &integrationCustomCtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
}

func performOversizedRequest(t *testing.T, app *fiber.App, configure func(req *fasthttp.Request)) *fasthttp.Response {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)

	go func() {
		errCh <- app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err != nil {
			return false
		}
		if err := conn.Close(); err != nil {
			return false
		}
		return true
	}, time.Second, 10*time.Millisecond)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	req.SetRequestURI("http://example.com/")
	req.Header.SetMethod(fiber.MethodPost)
	req.Header.Set(fiber.HeaderOrigin, "https://example.com")
	req.SetBody(bytes.Repeat([]byte{'a'}, 32))
	if configure != nil {
		configure(req)
	}

	client := fasthttp.Client{
		Dial: func(string) (net.Conn, error) {
			return ln.Dial()
		},
	}

	require.NoError(t, client.Do(req, resp))

	respCopy := fasthttp.AcquireResponse()
	resp.CopyTo(respCopy)

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(resp)

	t.Cleanup(func() {
		fasthttp.ReleaseResponse(respCopy)
	})

	return respCopy
}

func Test_Integration_App_ServerErrorHandler_PreservesCORSHeadersOnBodyLimit(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://example.com"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"X-Request-ID"},
	}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "https://example.com", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
	require.Equal(t, "true", string(resp.Header.Peek(fiber.HeaderAccessControlAllowCredentials)))
	require.Equal(t, "X-Request-ID", string(resp.Header.Peek(fiber.HeaderAccessControlExposeHeaders)))
	require.Equal(t, "Origin", string(resp.Header.Peek(fiber.HeaderVary)))
}

func Test_Integration_App_ServerErrorHandler_PreservesHelmetHeadersOnBodyLimit(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(helmet.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
	require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Opener-Policy")))
	require.Equal(t, "same-origin", string(resp.Header.Peek("Cross-Origin-Resource-Policy")))
	require.Equal(t, "require-corp", string(resp.Header.Peek("Cross-Origin-Embedder-Policy")))
}

func Test_Integration_App_ServerErrorHandler_PreservesRequestID(t *testing.T) {
	const expectedRequestID = "integration-request-id"

	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(requestid.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set("X-Request-ID", expectedRequestID)
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, expectedRequestID, string(resp.Header.Peek("X-Request-ID")))
}

func Test_Integration_App_ServerErrorHandler_GroupMiddlewareChain(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 16})
	app.Use(helmet.New())

	api := app.Group("/api")
	api.Use(requestid.New())
	api.Use(func(c fiber.Ctx) error {
		c.Set("X-Group-Middleware", "active")
		return c.Next()
	})
	api.Post("/resource", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.SetRequestURI("http://example.com/api/resource")
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "nosniff", string(resp.Header.Peek(fiber.HeaderXContentTypeOptions)))
	require.NotEmpty(t, resp.Header.Peek("X-Request-ID"))
	require.Equal(t, "active", string(resp.Header.Peek("X-Group-Middleware")))
}

func Test_Integration_App_ServerErrorHandler_RetainsHeadersFromSubsequentMiddleware(t *testing.T) {
	app := fiber.New(fiber.Config{BodyLimit: 8})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Custom-Middleware", "ran")
		return c.Next()
	})
	app.Use(cors.New())
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, nil)

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "ran", string(resp.Header.Peek("X-Custom-Middleware")))
	require.Equal(t, "*", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}

func Test_Integration_App_ServerErrorHandler_WithCustomCtx(t *testing.T) {
	app := fiber.NewWithCustomCtx(newIntegrationCustomCtx, fiber.Config{BodyLimit: 16})
	app.Use(func(c fiber.Ctx) error {
		customCtx, ok := c.(*integrationCustomCtx)
		require.True(t, ok)
		customCtx.Set("X-Custom-Ctx", "true")
		return c.Next()
	})
	app.Use(cors.New(cors.Config{AllowOrigins: []string{"https://example.org"}}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp := performOversizedRequest(t, app, func(req *fasthttp.Request) {
		req.Header.Set(fiber.HeaderOrigin, "https://example.org")
	})

	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode())
	require.Equal(t, "true", string(resp.Header.Peek("X-Custom-Ctx")))
	require.Equal(t, "https://example.org", string(resp.Header.Peek(fiber.HeaderAccessControlAllowOrigin)))
}
