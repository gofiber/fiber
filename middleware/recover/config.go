package recover //nolint:predeclared // TODO: Rename to some non-builtin

import (
	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ErrorCustomizer defines a function to customize the error produced from a recovered panic/result.
	//
	// Optional. Default: defaultErrorCustomizer
	ErrorCustomizer func(c fiber.Ctx, r any) error

	// StackTraceHandler defines a function to handle stack trace
	//
	// Optional. Default: defaultStackTraceHandler
	StackTraceHandler func(c fiber.Ctx, e any)

	// EnableStackTrace enables handling stack trace
	//
	// Optional. Default: false
	EnableStackTrace bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:              nil,
	EnableStackTrace:  false,
	StackTraceHandler: defaultStackTraceHandler,
	ErrorCustomizer:   defaultErrorCustomizer,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	if cfg.EnableStackTrace && cfg.StackTraceHandler == nil {
		cfg.StackTraceHandler = defaultStackTraceHandler
	}
	if cfg.ErrorCustomizer == nil {
		cfg.ErrorCustomizer = defaultErrorCustomizer
	}

	return cfg
}
