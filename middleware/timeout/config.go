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

	// OnTimeout is executed when a timeout occurs. It may write the response or
	// return a *fiber.Error that shapes it; otherwise the default 503 is sent.
	// When it returns nil while the handler still runs, the middleware returns
	// fiber.ErrServiceUnavailable, so middleware that runs before it leaves the response alone.
	// The timed-out handler may still be running and must stop using the context.
	// After the deadline OnTimeout gets a ctx of its own: the request headers, locals and response headers
	// as the handler got them, without the request body or route params.
	// Optional. Default: nil (return fiber.ErrServiceUnavailable)
	OnTimeout fiber.Handler

	// Errors defines custom errors that are treated as timeouts.
	// Optional. Default: nil
	Errors []error

	// Timeout defines the timeout duration for all routes.
	// Optional. Default: 0 (no timeout)
	Timeout time.Duration
}

// ConfigDefault is the default configuration.
var ConfigDefault = Config{
	Next:      nil,
	Timeout:   0,
	OnTimeout: nil,
	Errors:    nil,
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
	return cfg
}
