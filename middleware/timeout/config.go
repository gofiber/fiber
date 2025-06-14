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

	// Timeout defines the default timeout duration for all routes.
	// Optional. Default: 0 (no timeout)
	Timeout time.Duration

	// OnTimeout is called when a timeout occurs.
	// Optional. Default: nil (return fiber.ErrRequestTimeout)
	OnTimeout fiber.Handler

	// SkipPaths defines request paths that should ignore the timeout.
	// Optional. Default: nil
	SkipPaths []string

	// Routes allows specifying timeouts per path. If a path is present,
	// its timeout overrides the default Timeout value.
	// Optional. Default: nil
	Routes map[string]time.Duration

	// Errors defines custom errors that are treated as timeouts.
	// Optional. Default: nil
	Errors []error
}

// ConfigDefault is the default configuration.
var ConfigDefault = Config{
	Next:      nil,
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
