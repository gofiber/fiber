package timeout

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config holds the configuration for the timeout middleware.
type Config struct {
	// Next defines a function to skip this middleware.
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// OnTimeout is executed when a timeout occurs.
	// Optional. Default: nil (return fiber.ErrRequestTimeout)
	OnTimeout fiber.Handler

	// Errors defines custom errors that are treated as timeouts.
	// Optional. Default: nil
	Errors []error

	// Timeout defines the timeout duration for all routes.
	// Optional. Default: 0 (no timeout)
	Timeout time.Duration

	// GracePeriod is the maximum time to wait for the handler to finish after
	// a timeout occurs. If 0 (default), the middleware waits indefinitely for
	// the handler to complete to avoid race conditions with Fiber's context
	// pooling. If > 0, the middleware returns after GracePeriod even if the
	// handler is still running.
	//
	// Warning: Setting GracePeriod > 0 may cause race conditions if the handler
	// continues to access the Fiber context after the middleware returns.
	// Only use this if you understand the implications and your handlers are
	// designed to handle context cancelation properly.
	//
	// Optional. Default: 0 (wait indefinitely)
	GracePeriod time.Duration
}

// ConfigDefault is the default configuration.
var ConfigDefault = Config{
	Next:        nil,
	Timeout:     0,
	OnTimeout:   nil,
	Errors:      nil,
	GracePeriod: 0,
}

// configDefault returns the first Config value or ConfigDefault.
func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Timeout < 0 {
		cfg.Timeout = ConfigDefault.Timeout
	}
	if cfg.Errors == nil {
		cfg.Errors = ConfigDefault.Errors
	}
	if cfg.OnTimeout == nil {
		cfg.OnTimeout = ConfigDefault.OnTimeout
	}
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.GracePeriod < 0 {
		cfg.GracePeriod = ConfigDefault.GracePeriod
	}
	return cfg
}
