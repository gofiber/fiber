package healthcheck

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func shouldGiveStatus(t *testing.T, app *fiber.App, path string, expectedStatus int) {
	t.Helper()
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
	require.NoError(t, err)
	require.Equal(t, expectedStatus, req.StatusCode, "path: "+path+" should match "+strconv.Itoa(expectedStatus))
}

func shouldGiveOK(t *testing.T, app *fiber.App, path string) {
	t.Helper()
	shouldGiveStatus(t, app, path, fiber.StatusOK)
}

func shouldGiveNotFound(t *testing.T, app *fiber.App, path string) {
	t.Helper()
	shouldGiveStatus(t, app, path, fiber.StatusNotFound)
}

func Test_HealthCheck_Strict_Routing_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		StrictRouting: true,
	})

	app.Get(DefaultLivenessEndpoint, New())
	app.Get(DefaultReadinessEndpoint, New())
	app.Get(DefaultStartupEndpoint, New())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveOK(t, app, "/startupz")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")
	shouldGiveNotFound(t, app, "/startupz/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/startupz")
}

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get(DefaultLivenessEndpoint, New())
	app.Get(DefaultReadinessEndpoint, New())
	app.Get(DefaultStartupEndpoint, New())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveOK(t, app, "/startupz")
	shouldGiveOK(t, app, "/readyz/")
	shouldGiveOK(t, app, "/livez/")
	shouldGiveOK(t, app, "/startupz/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/startupz")
}

func Test_HealthCheck_Custom(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := make(chan struct{}, 1)
	app.Get("/live", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/ready", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))
	app.Get(DefaultStartupEndpoint, New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Setup custom liveness and readiness probes to simulate application health status
	// Live should return 200 with GET request
	shouldGiveOK(t, app, "/live")
	// Live should return 404 with POST request
	req, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/live", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMethodNotAllowed, req.StatusCode)

	// Ready should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/ready", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMethodNotAllowed, req.StatusCode)

	// Ready should return 503 with GET request before the channel is closed
	shouldGiveStatus(t, app, "/ready", fiber.StatusServiceUnavailable)

	shouldGiveStatus(t, app, "/startupz", fiber.StatusServiceUnavailable)

	// Ready should return 200 with GET request after the channel is closed
	c1 <- struct{}{}
	shouldGiveOK(t, app, "/ready")
}

func Test_HealthCheck_Custom_Nested(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c1 := make(chan struct{}, 1)
	app.Get("/probe/live", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/probe/ready", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))

	// Testing custom health check endpoints with nested paths
	shouldGiveOK(t, app, "/probe/live")
	shouldGiveStatus(t, app, "/probe/ready", fiber.StatusServiceUnavailable)
	shouldGiveOK(t, app, "/probe/live/")
	shouldGiveStatus(t, app, "/probe/ready/", fiber.StatusServiceUnavailable)
	shouldGiveNotFound(t, app, "/probe/livez")
	shouldGiveNotFound(t, app, "/probe/readyz")
	shouldGiveNotFound(t, app, "/probe/livez/")
	shouldGiveNotFound(t, app, "/probe/readyz/")
	shouldGiveNotFound(t, app, "/livez")
	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")

	c1 <- struct{}{}
	shouldGiveOK(t, app, "/probe/ready")
	c1 <- struct{}{}
	shouldGiveOK(t, app, "/probe/ready/")
}

func Test_HealthCheck_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	checker := New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	})

	app.Get(DefaultLivenessEndpoint, checker)
	app.Get(DefaultReadinessEndpoint, checker)
	app.Get(DefaultStartupEndpoint, checker)

	// This should give not found since there are no other handlers to execute
	// so it's like the route isn't defined at all
	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/livez")
	shouldGiveNotFound(t, app, "/startupz")
}

func Benchmark_HealthCheck(b *testing.B) {
	app := fiber.New()

	app.Get(DefaultLivenessEndpoint, New())
	app.Get(DefaultReadinessEndpoint, New())
	app.Get(DefaultStartupEndpoint, New())

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

func Benchmark_HealthCheck_Parallel(b *testing.B) {
	app := fiber.New()

	app.Get(DefaultLivenessEndpoint, New())
	app.Get(DefaultReadinessEndpoint, New())
	app.Get(DefaultStartupEndpoint, New())

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/livez")

		for pb.Next() {
			h(fctx)
		}
	})
}
