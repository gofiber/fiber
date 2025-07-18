package basicauth

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_BasicAuth_Next
func Test_BasicAuth_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_Middleware_BasicAuth(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Users: map[string]string{
			"john":  "doe",
			"admin": "123456",
		},
		StorePassword: true,
	}))

	app.Get("/testauth", func(c fiber.Ctx) error {
		username := UsernameFromContext(c)
		password := PasswordFromContext(c)

		return c.SendString(username + password)
	})

	tests := []struct {
		url        string
		username   string
		password   string
		statusCode int
	}{
		{
			url:        "/testauth",
			statusCode: 200,
			username:   "john",
			password:   "doe",
		},
		{
			url:        "/testauth",
			statusCode: 200,
			username:   "admin",
			password:   "123456",
		},
		{
			url:        "/testauth",
			statusCode: 401,
			username:   "ee",
			password:   "123456",
		},
	}

	for _, tt := range tests {
		// Base64 encode credentials for http auth header
		creds := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", tt.username, tt.password))

		req := httptest.NewRequest(fiber.MethodGet, "/testauth", nil)
		req.Header.Add("Authorization", "Basic "+creds)
		resp, err := app.Test(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)

		require.NoError(t, err)
		require.Equal(t, tt.statusCode, resp.StatusCode)

		if tt.statusCode == 200 {
			require.Equal(t, fmt.Sprintf("%s%s", tt.username, tt.password), string(body))
		}
	}
}

func Test_BasicAuth_NoStorePassword(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		Users: map[string]string{"john": "doe"},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		require.Empty(t, PasswordFromContext(c))
		return c.SendStatus(fiber.StatusOK)
	})

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_BasicAuth_WWWAuthenticateHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Users: map[string]string{"john": "doe"}}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, `Basic realm="Restricted", charset="UTF-8"`, resp.Header.Get(fiber.HeaderWWWAuthenticate))
}

func Test_BasicAuth_InvalidHeader(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Users: map[string]string{"john": "doe"}}))

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Basic notbase64")
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func Test_BasicAuth_WhitespaceHandling(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Users: map[string]string{"john": "doe"}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusTeapot) })

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))

	cases := []string{
		"Basic " + creds,
		" Basic \t" + creds,
		"Basic  " + creds + "   ",
	}

	for _, h := range cases {
		req := httptest.NewRequest(fiber.MethodGet, "/", nil)
		req.Header.Set(fiber.HeaderAuthorization, h)
		resp, err := app.Test(req)

		require.NoError(t, err)
		require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	}
}

// go test -v -run=^$ -bench=Benchmark_Middleware_BasicAuth -benchmem -count=4
func Benchmark_Middleware_BasicAuth(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Users: map[string]string{
			"john": "doe",
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

// go test -v -run=^$ -bench=Benchmark_Middleware_BasicAuth -benchmem -count=4
func Benchmark_Middleware_BasicAuth_Upper(b *testing.B) {
	app := fiber.New()

	app.Use(New(Config{
		Users: map[string]string{
			"john": "doe",
		},
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "Basic am9objpkb2U=") // john:doe

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}

func Test_BasicAuth_Immutable(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{Immutable: true})

	app.Use(New(Config{Users: map[string]string{"john": "doe"}}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	creds := base64.StdEncoding.EncodeToString([]byte("john:doe"))
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Basic "+creds)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
}
