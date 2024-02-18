package healthcheck

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

func shouldWork(app *fiber.App, t *testing.T, path string) {
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, req.StatusCode)
}

func shouldNotWork(app *fiber.App, t *testing.T, path string) {
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)
}

func Test_HealthCheck_Strict_Routing_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		StrictRouting: true,
	})

	app.Use(New())

	shouldWork(app, t, "/readyz")
	shouldWork(app, t, "/livez")
	shouldNotWork(app, t, "/readyz/")
	shouldNotWork(app, t, "/livez/")
	shouldNotWork(app, t, "/notDefined/readyz")
	shouldNotWork(app, t, "/notDefined/livez")
}

func Test_HealthCheck_Group_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Group("/v1", New())
	v2Group := app.Group("/v2/")
	customer := v2Group.Group("/customer/")
	customer.Use(New())

	shouldWork(app, t, "/v1/readyz")
	shouldWork(app, t, "/v1/livez")
	shouldWork(app, t, "/v1/readyz/")
	shouldWork(app, t, "/v1/livez/")
	shouldWork(app, t, "/v2/customer/readyz")
	shouldWork(app, t, "/v2/customer/livez")
	shouldWork(app, t, "/v2/customer/readyz/")
	shouldWork(app, t, "/v2/customer/livez/")
	shouldNotWork(app, t, "/v2/customer/readyz/")
	shouldNotWork(app, t, "/v2/customer/livez/")
	shouldNotWork(app, t, "/notDefined/readyz")
	shouldNotWork(app, t, "/notDefined/livez")
}

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New())

	shouldWork(app, t, "/readyz")
	shouldWork(app, t, "/livez")
	shouldWork(app, t, "/readyz/")
	shouldWork(app, t, "/livez/")
	shouldNotWork(app, t, "/notDefined/readyz")
	shouldNotWork(app, t, "/notDefined/livez")
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
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/live",
		ReadinessProbe: func(c *fiber.Ctx) bool {
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
	shouldWork(app, t, "/live")
	// Live should return 404 with POST request
	req, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/live", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/ready", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 503 with GET request before the channel is closed
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusServiceUnavailable, req.StatusCode)

	time.Sleep(1 * time.Second)

	// Ready should return 200 with GET request after the channel is closed
	shouldWork(app, t, "/ready")
}

func Test_HealthCheck_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return true
		},
	}))

	shouldNotWork(app, t, "/readyz")
	shouldNotWork(app, t, "/livez")
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

	utils.AssertEqual(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
}
