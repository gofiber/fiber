package probechecker

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_ProbeChecker(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c1 := make(chan struct{}, 1)
	go func() {
		time.Sleep(1 * time.Second)
		c1 <- struct{}{}
	}()

	app.Use(New(Config{
		IsLive: func(c *fiber.Ctx) bool {
			return true
		},
		IsLiveEndpoint: "/live",
		IsReady: func(c *fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
		IsReadyEndpoint: "/ready",
	}))

	// Live should return 200 with GET request
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/live", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, req.StatusCode)

	// Live should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/live", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)

	// Ready should return 503 with GET request before the channel is closed
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusServiceUnavailable, req.StatusCode)

	time.Sleep(1 * time.Second)

	// Ready should return 200 with GET request after the channel is closed
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/ready", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusOK, req.StatusCode)
}

func Test_ProbeChecker_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		Next: func(c *fiber.Ctx) bool {
			return true
		},
	}))

	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)
}

func Test_ProbeChecker_NilChecker(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Use(New(Config{
		IsReady: nil,
	}))

	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusNotFound, req.StatusCode)
}
