package loadshed

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

type MockCPUPercentGetter struct {
	MockedPercentage []float64
}

func (m *MockCPUPercentGetter) PercentWithContext(_ context.Context, _ time.Duration, _ bool) ([]float64, error) {
	return m.MockedPercentage, nil
}

func ReturnOK(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

func Test_Loadshed_LowerThreshold(t *testing.T) {
	app := fiber.New()

	mockGetter := &MockCPUPercentGetter{MockedPercentage: []float64{89.0}}
	var cfg Config
	cfg.Criteria = &CPULoadCriteria{
		LowerThreshold: 0.90,
		UpperThreshold: 0.95,
		Interval:       time.Second,
		Getter:         mockGetter,
	}
	app.Use(New(cfg))
	app.Get("/", ReturnOK)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)

	status := resp.StatusCode
	if status != fiber.StatusOK && status != fiber.StatusServiceUnavailable {
		t.Fatalf("Expected status code %d or %d but got %d", fiber.StatusOK, fiber.StatusServiceUnavailable, status)
	}
}

func Test_Loadshed_MiddleValue(t *testing.T) {
	app := fiber.New()

	mockGetter := &MockCPUPercentGetter{MockedPercentage: []float64{93.0}}
	var cfg Config
	cfg.Criteria = &CPULoadCriteria{
		LowerThreshold: 0.90,
		UpperThreshold: 0.95,
		Interval:       time.Second,
		Getter:         mockGetter,
	}
	app.Use(New(cfg))
	app.Get("/", ReturnOK)

	rejectedCount := 0
	acceptedCount := 0
	iterations := 100000

	for i := 0; i < iterations; i++ {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)

		if resp.StatusCode == fiber.StatusServiceUnavailable {
			rejectedCount++
		} else {
			acceptedCount++
		}
	}

	t.Logf("Accepted: %d, Rejected: %d", acceptedCount, rejectedCount)
	if acceptedCount == 0 || rejectedCount == 0 {
		t.Fatalf("Expected both accepted and rejected requests, but got Accepted: %d, Rejected: %d", acceptedCount, rejectedCount)
	}
}

func Test_Loadshed_UpperThreshold(t *testing.T) {
	app := fiber.New()

	mockGetter := &MockCPUPercentGetter{MockedPercentage: []float64{96.0}}
	var cfg Config
	cfg.Criteria = &CPULoadCriteria{
		LowerThreshold: 0.90,
		UpperThreshold: 0.95,
		Interval:       time.Second,
		Getter:         mockGetter,
	}
	app.Use(New(cfg))
	app.Get("/", ReturnOK)

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}
