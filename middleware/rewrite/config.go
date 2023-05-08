package rewrite

import (
	"regexp"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip middleware.
	// Optional. Default: nil
	Next func(*fiber.Ctx) bool

	// Rules defines the URL path rewrite rules. The values captured in asterisk can be
	// retrieved by index e.g. $1, $2 and so on.
	// Required. Example:
	// "/old":              "/new",
	// "/api/*":            "/$1",
	// "/js/*":             "/public/javascripts/$1",
	// "/users/*/orders/*": "/user/$1/order/$2",
	Rules map[string]string

	rulesRegex map[*regexp.Regexp]string
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return Config{}
	}

	// Override default config
	cfg := config[0]

	return cfg
}
