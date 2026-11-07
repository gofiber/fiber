package deprecation

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// Config defines the config for middleware.
type Config struct {
	// Deprecation is when the resource was, or will be, deprecated. It is sent
	// in the Deprecation header as a Structured Field Date (RFC 9745). A date
	// in the future announces an upcoming deprecation.
	//
	// Optional. Default: zero time (header not sent)
	Deprecation time.Time

	// Sunset is when the resource is expected to become unresponsive. It is
	// sent in the Sunset header as an HTTP-date (RFC 8594), and must not be
	// earlier than Deprecation (RFC 9745 Section 4).
	//
	// Optional. Default: zero time (header not sent)
	Sunset time.Time

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Link is a URI that documents the deprecation, such as a migration
	// guide. It is added to the Link header with rel="deprecation".
	//
	// Optional. Default: ""
	Link string

	// SunsetLink is a URI that documents the sunset policy. It is added to
	// the Link header with rel="sunset".
	//
	// Optional. Default: ""
	SunsetLink string
}

// ConfigDefault is the default config.
var ConfigDefault = Config{
	Next: nil,
}

// Helper function to set default values and validate the config.
func configDefault(config ...Config) Config {
	cfg := ConfigDefault
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Deprecation.IsZero() && cfg.Sunset.IsZero() {
		panic("deprecation: Deprecation or Sunset is required")
	}

	if !cfg.Deprecation.IsZero() && !cfg.Sunset.IsZero() && cfg.Sunset.Before(cfg.Deprecation) {
		panic("deprecation: Sunset must not be earlier than Deprecation")
	}

	return cfg
}
