package basicauth

import (
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	b64 "encoding/base64"

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

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
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
	}))

	app.Get("/testauth", func(c fiber.Ctx) error {
		username := c.Locals("username").(string)
		password := c.Locals("password").(string)

		return c.SendString(username + password)
	})

	tests := []struct {
		url        string
		statusCode int
		username   string
		password   string
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
		creds := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", tt.username, tt.password)))

		req := httptest.NewRequest("GET", "/testauth", nil)
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
	fctx.Request.Header.SetMethod("GET")
	fctx.Request.SetRequestURI("/")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "basic am9objpkb2U=") // john:doe

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	require.Equal(b, fiber.StatusTeapot, fctx.Response.Header.StatusCode())
}
