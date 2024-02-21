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

	app.Use(New())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
}

func Test_HealthCheck_Group_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Group("/v1", New())
	v2Group := app.Group("/v2/")
	customer := v2Group.Group("/customer/")
	customer.Use(New())

	v3Group := app.Group("/v3/")
	v3Group.Group("/todos/", New(Config{ReadinessEndpoint: "/readyz/", LivenessEndpoint: "/livez/"}))

	shouldGiveOK(t, app, "/v1/readyz")
	shouldGiveOK(t, app, "/v1/livez")
	shouldGiveOK(t, app, "/v1/readyz/")
	shouldGiveOK(t, app, "/v1/livez/")
	shouldGiveOK(t, app, "/v2/customer/readyz")
	shouldGiveOK(t, app, "/v2/customer/livez")
	shouldGiveOK(t, app, "/v2/customer/readyz/")
	shouldGiveOK(t, app, "/v2/customer/livez/")
	shouldGiveNotFound(t, app, "/v3/todos/readyz")
	shouldGiveNotFound(t, app, "/v3/todos/livez")
	shouldGiveOK(t, app, "/v3/todos/readyz/")
	shouldGiveOK(t, app, "/v3/todos/livez/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/readyz/")
	shouldGiveNotFound(t, app, "/notDefined/livez/")

	// strict routing
	app = fiber.New(fiber.Config{
		StrictRouting: true,
	})
	app.Group("/v1", New())
	v2Group = app.Group("/v2/")
	customer = v2Group.Group("/customer/")
	customer.Use(New())

	v3Group = app.Group("/v3/")
	v3Group.Group("/todos/", New(Config{ReadinessEndpoint: "/readyz/", LivenessEndpoint: "/livez/"}))

	shouldGiveOK(t, app, "/v1/readyz")
	shouldGiveOK(t, app, "/v1/livez")
	shouldGiveNotFound(t, app, "/v1/readyz/")
	shouldGiveNotFound(t, app, "/v1/livez/")
	shouldGiveOK(t, app, "/v2/customer/readyz")
	shouldGiveOK(t, app, "/v2/customer/livez")
	shouldGiveNotFound(t, app, "/v2/customer/readyz/")
	shouldGiveNotFound(t, app, "/v2/customer/livez/")
	shouldGiveNotFound(t, app, "/v3/todos/readyz")
	shouldGiveNotFound(t, app, "/v3/todos/livez")
	shouldGiveOK(t, app, "/v3/todos/readyz/")
	shouldGiveOK(t, app, "/v3/todos/livez/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/readyz/")
	shouldGiveNotFound(t, app, "/notDefined/livez/")
}

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

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
	shouldGiveOK(t, app, "/live")
	// Live should return 404 with POST request
	req, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/live", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/ready", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, req.StatusCode)

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

	app.Use(New(Config{
		LivenessProbe: func(_ fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/probe/live",
		ReadinessProbe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
		ReadinessEndpoint: "/probe/ready",
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

	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/livez")
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
