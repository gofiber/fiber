package timeout

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the configuration options for the timeout middleware.
type Config struct {
	// Next defines a function to skip this middleware.
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// OnTimeout is called when a timeout occurs.
	// Optional. Default: nil (return fiber.ErrRequestTimeout)
	OnTimeout fiber.Handler

	// Errors defines custom errors that are treated as timeouts.
	// Optional. Default: nil
	Errors []error

	Timeout:   0,
	OnTimeout: nil,
	SkipPaths: nil,
	Routes:    nil,
	Errors:    nil,
}

// Helper function to set default values.
func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}

	cfg := config[0]

	if cfg.Routes == nil {
		cfg.Routes = ConfigDefault.Routes
	} else {
		for p, d := range cfg.Routes {
			if d < 0 {
				cfg.Routes[p] = 0
			}
		}
	}
	if cfg.SkipPaths == nil {
		cfg.SkipPaths = ConfigDefault.SkipPaths
	}
	if cfg.Routes == nil {
		cfg.Routes = ConfigDefault.Routes
	}
	if cfg.OnTimeout == nil {
		cfg.OnTimeout = ConfigDefault.OnTimeout
	}
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Errors == nil {
		cfg.Errors = ConfigDefault.Errors
	}

	return cfg
}
