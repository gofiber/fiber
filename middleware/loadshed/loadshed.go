package loadshed

import (
	"context"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
)

type CPUPercentGetter interface {
	PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error)
}

type RealCPUPercentGetter struct{}

func (r *RealCPUPercentGetter) PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
	return cpu.PercentWithContext(ctx, interval, percpu)
}

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	Next func(c *fiber.Ctx) bool

	// LowerThreshold for CPU usage to start shedding load.
	LowerThreshold float64

	// UpperThreshold for CPU usage to stop shedding load.
	UpperThreshold float64

	// Interval for checking the CPU usage.
	Interval time.Duration

	Getter CPUPercentGetter
}

var ConfigDefault = Config{
	Next:           nil,
	LowerThreshold: 0.90,
	UpperThreshold: 0.95,
	Interval:       time.Second,
	Getter:         &RealCPUPercentGetter{},
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

		// Obtain average CPU usage over a set interval
		percentages, perr := cfg.Getter.PercentWithContext(c.Context(), cfg.Interval, false)
		if perr != nil || len(percentages) == 0 {
			return c.Next() // If unable to get CPU usage, allow the request
		}

		cpuUsage := percentages[0]

		if cpuUsage > cfg.UpperThreshold*100 {
			return fiber.NewError(fiber.StatusServiceUnavailable) // Reject all requests if CPU usage exceeds the upper threshold

		} else if cpuUsage > cfg.LowerThreshold*100 { // Check if the CPU usage is between the lower and upper thresholds
			// Calculate the probability of rejecting a request based on how much the current
			// CPU usage exceeds the lower threshold. The closer the usage is to the upper threshold,
			// the higher the rejection probability.
			rejectionProbability := (cpuUsage - cfg.LowerThreshold*100) / (cfg.UpperThreshold - cfg.LowerThreshold)

			// Reject the request probabilistically based on the calculated probability.
			if rand.Float64()*100 < rejectionProbability {
				return fiber.NewError(fiber.StatusServiceUnavailable)
			}
		}

		return nil
	}
}
