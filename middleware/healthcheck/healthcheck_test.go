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

	app.Get("/livez", NewHealthChecker())
	app.Get("/readyz", NewHealthChecker())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
}

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/livez", NewHealthChecker())
	app.Get("/readyz", NewHealthChecker())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveOK(t, app, "/readyz/")
	shouldGiveOK(t, app, "/livez/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
}

func Test_HealthCheck_Custom(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := make(chan struct{}, 1)
	app.Get("/live", NewHealthChecker(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/ready", NewHealthChecker(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))

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

	// Ready should return 200 with GET request after the channel is closed
	c1 <- struct{}{}
	shouldGiveOK(t, app, "/ready")
}

func Test_HealthCheck_Custom_Nested(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c1 := make(chan struct{}, 1)
	app.Get("/probe/live", NewHealthChecker(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/probe/ready", NewHealthChecker(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))

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

	checker := NewHealthChecker(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	})

	app.Get("/readyz", checker)
	app.Get("/livez", checker)

	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/livez")
}

func Benchmark_HealthCheck(b *testing.B) {
	app := fiber.New()

	app.Get(DefaultLivenessEndpoint, NewHealthChecker())
	app.Get(DefaultReadinessEndpoint, NewHealthChecker())

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
