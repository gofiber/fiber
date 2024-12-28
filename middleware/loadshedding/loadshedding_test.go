package loadshedding_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/loadshedding"
	"github.com/stretchr/testify/require"
)

// Helper handlers
func successHandler(c fiber.Ctx) error {
	return c.SendString("Request processed successfully!")
}

func timeoutHandler(c fiber.Ctx) error {
	time.Sleep(2 * time.Second) // Simulate a long-running request
	return c.SendString("This should not appear")
}

func loadSheddingHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusServiceUnavailable).SendString("Service Overloaded")
}

func excludedHandler(c fiber.Ctx) error {
	return c.SendString("Excluded route")
}

// go test -run Test_LoadSheddingExcluded
func Test_LoadSheddingExcluded(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Middleware with exclusion
	app.Use(loadshedding.New(
		1*time.Second,
		loadSheddingHandler,
		func(c fiber.Ctx) bool { return c.Path() == "/excluded" },
	))
	app.Get("/", successHandler)
	app.Get("/excluded", excludedHandler)

	// Test excluded route
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/excluded", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// go test -run Test_LoadSheddingTimeout
func Test_LoadSheddingTimeout(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Middleware without exclusions
	app.Use(loadshedding.New(
		1*time.Second, // Set timeout for the middleware
		loadSheddingHandler,
		nil,
	))
	app.Get("/", timeoutHandler)

	// Create a custom HTTP client with a sufficient timeout

	// Test timeout behavior
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req, fiber.TestConfig{
		Timeout: 3 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

// go test -run Test_LoadSheddingSuccessfulRequest
func Test_LoadSheddingSuccessfulRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Middleware with sufficient time for request to complete
	app.Use(loadshedding.New(
		2*time.Second,
		loadSheddingHandler,
		nil,
	))
	app.Get("/", successHandler)

	// Test successful request
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}
