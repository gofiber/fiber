package monitor

import "github.com/gofiber/fiber/v2"

// Config defines the config for middleware.
type Config struct {
	// To disable serving HTML, you can make true this option.
	//
	// Optional. Default: false
	DisableHTML bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool
}

var ConfigDefault = Config{
	DisableHTML: false,
	Next:        nil,
}

func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if !cfg.DisableHTML {
		cfg.DisableHTML = ConfigDefault.DisableHTML
	}

	return cfg
}
