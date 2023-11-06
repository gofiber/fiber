package loadshed

import (
	"context"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
)

// LoadCriteria interface for different types of load metrics.
type LoadCriteria interface {
	Metric(ctx context.Context) (float64, error)
	ShouldShed(metric float64) bool
}

// CPULoadCriteria for using CPU as a load metric.
type CPULoadCriteria struct {
	LowerThreshold float64
	UpperThreshold float64
	Interval       time.Duration
	Getter         CPUPercentGetter
}

func (c *CPULoadCriteria) Metric(ctx context.Context) (float64, error) {
	percentages, err := c.Getter.PercentWithContext(ctx, c.Interval, false)
	if err != nil || len(percentages) == 0 {
		return 0, err
	}
	return percentages[0], nil
}

func (c *CPULoadCriteria) ShouldShed(metric float64) bool {
	if metric > c.UpperThreshold*100 {
		return true
	} else if metric > c.LowerThreshold*100 {
		rejectionProbability := (metric - c.LowerThreshold*100) / (c.UpperThreshold - c.LowerThreshold)
		// #nosec G404
		return rand.Float64()*100 < rejectionProbability
	}
	return false
}

type CPUPercentGetter interface {
	PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error)
}

type DefaultCPUPercentGetter struct{}

func (r *DefaultCPUPercentGetter) PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
	return cpu.PercentWithContext(ctx, interval, percpu)
}

// // Config defines the config for middleware.
// type Config struct {
// 	// Next defines a function to skip this middleware when returned true.
// 	Next func(c *fiber.Ctx) bool

// 	// LowerThreshold for CPU usage to start shedding load.
// 	LowerThreshold float64

// 	// UpperThreshold for CPU usage to stop shedding load.
// 	UpperThreshold float64

// 	// Interval for checking the CPU usage.
// 	Interval time.Duration

// 	Getter CPUPercentGetter
// }

// var ConfigDefault = Config{
// 	Next:           nil,
// 	LowerThreshold: 0.90,
// 	UpperThreshold: 0.95,
// 	Interval:       time.Second,
// 	Getter:         &RealCPUPercentGetter{},
// }

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

		// Removed for extending the middleware to use different load metrics
		// // Obtain average CPU usage over a set interval
		// percentages, perr := cfg.Getter.PercentWithContext(c.Context(), cfg.Interval, false)
		// if perr != nil || len(percentages) == 0 {
		// 	return c.Next() // If unable to get CPU usage, allow the request
		// }

		// cpuUsage := percentages[0]

		// if cpuUsage > cfg.UpperThreshold*100 {
		// 	return fiber.NewError(fiber.StatusServiceUnavailable) // Reject all requests if CPU usage exceeds the upper threshold

		// } else if cpuUsage > cfg.LowerThreshold*100 { // Check if the CPU usage is between the lower and upper thresholds
		// 	// Calculate the probability of rejecting a request based on how much the current
		// 	// CPU usage exceeds the lower threshold. The closer the usage is to the upper threshold,
		// 	// the higher the rejection probability.
		// 	rejectionProbability := (cpuUsage - cfg.LowerThreshold*100) / (cfg.UpperThreshold - cfg.LowerThreshold)

		// 	// Reject the request probabilistically based on the calculated probability.
		// 	if rand.Float64()*100 < rejectionProbability {
		// 		return fiber.NewError(fiber.StatusServiceUnavailable)
		// 	}
		// }

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
