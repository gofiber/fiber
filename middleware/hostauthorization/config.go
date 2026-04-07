package hostauthorization

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

// ErrForbiddenHost is returned when the Host header does not match any allowed host.
var ErrForbiddenHost = errors.New("hostauthorization: forbidden host")

// Config defines the config for the host authorization middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	// Use this to exclude health check endpoints or other paths from host validation.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// AllowedHosts is the list of permitted host values.
	// Supports three match types:
	//   - Exact:     "api.myapp.com"
	//   - Subdomain: ".myapp.com" (leading dot matches any subdomain, NOT the bare domain)
	//   - CIDR:      "10.0.0.0/8" (matches hosts that are IPs in the range)
	//
	// Required if AllowedHostsFunc is nil.
	AllowedHosts []string

	// AllowedHostsFunc is a dynamic validator called when static AllowedHosts
	// don't match. Receives the hostname (port stripped, lowercased).
	// Return true to allow.
	//
	// Optional. Default: nil
	AllowedHostsFunc func(host string) bool

	// ErrorHandler is called when a request is rejected.
	// Receives ErrForbiddenHost as the error.
	//
	// Optional. Default: returns 403 Forbidden with "Forbidden" body.
	ErrorHandler fiber.ErrorHandler
}

// ConfigDefault is the default config.
var ConfigDefault = Config{}

func configDefault(config ...Config) Config {
	if len(config) < 1 {
		panic("hostauthorization: AllowedHosts or AllowedHostsFunc is required")
	}

	cfg := config[0]

	if len(cfg.AllowedHosts) == 0 && cfg.AllowedHostsFunc == nil {
		panic("hostauthorization: AllowedHosts or AllowedHostsFunc is required")
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c fiber.Ctx, _ error) error {
			return c.Status(fiber.StatusForbidden).SendString("Forbidden")
		}
	}

	return cfg
}
