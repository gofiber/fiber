package routeguard

import (
	"github.com/gofiber/fiber/v3"
)
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ErrorHandler defines a function to customize the response when
	// no registered route matches the request.
	//
	// Optional. Default: DefaultErrorHandler
	ErrorHandler fiber.Handler
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next:         nil,
	ErrorHandler: defaultErrorHandler,
}

// configDefault sets default values on the config.
func configDefault(config ...Config) Config {
	if len(config) < 1 {
		return ConfigDefault
	}
	cfg := config[0]
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = defaultErrorHandler
	}
	return cfg
}

// defaultErrorHandler returns 404 Not Found.
func defaultErrorHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "not found",
	})
}