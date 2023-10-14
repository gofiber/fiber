package loadshed

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	Next func(c *fiber.Ctx) bool

	// LowerThreshold for CPU usage to start shedding load.
	LowerThreshold float64

	// UpperThreshold for CPU usage to stop shedding load.
	UpperThreshold float64

	// PollingInterval for checking the CPU usage.
	PollingInterval time.Duration

	// WindowSize for the loadshedder.
	WindowSize int

	// QueueSize defines the maximum size for the request queue.
	QueueSize int
}

var ConfigDefault = Config{
	Next:            nil,
	LowerThreshold:  0.90,
	UpperThreshold:  0.95,
	PollingInterval: time.Second,
	WindowSize:      10,
	QueueSize:       1000,
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

		// Check CPU usage
		percentages, err := cpu.Percent(cfg.PollingInterval, false)
		if err != nil {
			return fiber.NewError(fiber.StatusServiceUnavailable)
		}

		// If CPU usage is beyond the lower threshold, then queue the request
		if len(percentages) > 0 && percentages[0] > cfg.LowerThreshold*100 {
			// If the queue is full, respond with Service Unavailable
			// Otherwise, queue the request
		}

		if len(percentages) > 0 && percentages[0] > cfg.UpperThreshold*100 {
			// If CPU usage is beyond the upper threshold, respond with Service Unavailable
			return fiber.NewError(fiber.StatusServiceUnavailable)
		}

		return nil
	}
}
