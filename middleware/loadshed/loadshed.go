package loadshed

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	// Function to skip this middleware when returned true.
	Next func(c *fiber.Ctx) bool

	// Criteria defines the criteria to be used for load shedding.
	Criteria LoadCriteria
}

var ConfigDefault = Config{
	Next: nil,
	Criteria: &CPULoadCriteria{
		LowerThreshold: 0.90,
		UpperThreshold: 0.95,
		Interval:       10 * time.Second, // Evaluate the average CPU usage over the last 10 seconds.
		Getter:         &DefaultCPUPercentGetter{},
	},
}

func New(config ...Config) fiber.Handler {
	cfg := ConfigDefault

	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Compute the load metric using the specified criteria
		metric, err := cfg.Criteria.Metric(c.Context())
		if err != nil {
			return c.Next() // If unable to get metric, allow the request
		}

		// Shed load if the criteria's ShouldShed method returns true
		if cfg.Criteria.ShouldShed(metric) {
			return fiber.NewError(fiber.StatusServiceUnavailable)
		}

		return c.Next()
	}
}
