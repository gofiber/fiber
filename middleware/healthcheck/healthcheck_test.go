package healthcheck

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", nil))
	require.NoError(t, err)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)

	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
}

func Test_HealthCheck_Custom(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c1 := make(chan struct{}, 1)
	go func() {
		time.Sleep(1 * time.Second)
		c1 <- struct{}{}
	}()

	app.Use(New(Config{
		LivenessProbe: func(_ fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/live",
		ReadinessProbe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
		ReadinessEndpoint: "/ready",
	}))

	// Live should return 200 with GET request
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/live", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)

	// Live should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/live", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/ready", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 503 with GET request before the channel is closed
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)

	time.Sleep(1 * time.Second)

	// Ready should return 200 with GET request after the channel is closed
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
}

func Test_HealthCheck_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, req.StatusCode)
}

func Benchmark_HealthCheck(b *testing.B) {
	app := fiber.New()

	app.Use(New())

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/livez")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h(fctx)
	}

	require.Equal(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
}
