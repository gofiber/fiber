package earlydata

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
)

const (
	DefaultHeaderName      = "Early-Data"
	DefaultHeaderTrueValue = "1"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// IsEarlyData returns whether the request is an early-data request.
	//
	// Optional. Default: a function which returns true if the "Early-Data" request header or trailer is present (per RFC 8470 §5.1).
	IsEarlyData func(c fiber.Ctx) bool

	// AllowEarlyData returns whether the early-data request should be allowed or rejected.
	//
	// Optional. Default: a function which rejects the request on unsafe and allows the request on safe HTTP request methods.
	AllowEarlyData func(c fiber.Ctx) bool

	// Error is returned if an early-data request is rejected.
	//
	// Optional. Default: fiber.ErrTooEarly.
	Error error
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	IsEarlyData: func(c fiber.Ctx) bool {
		// Per RFC 8470 §5.1, multiple or invalid instances of Early-Data MUST
		// be treated as equivalent to a single "1", and Early-Data MUST NOT be
		// included in a trailer. Treating any presence of the field as early
		// data handles valid "1", invalid values, repeated lines, and trailers.
		h := &c.Request().Header
		return len(fieldname.Lines(h, DefaultHeaderName, headerlookup.Canonical(c))) > 0 ||
			fieldname.InTrailer(h, DefaultHeaderName)
	},

	AllowEarlyData: func(c fiber.Ctx) bool {
		return c.IsSafe()
	},

	Error: fiber.ErrTooEarly,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values

	if cfg.IsEarlyData == nil {
		cfg.IsEarlyData = ConfigDefault.IsEarlyData
	}

	if cfg.AllowEarlyData == nil {
		cfg.AllowEarlyData = ConfigDefault.AllowEarlyData
	}

	if cfg.Error == nil {
		cfg.Error = ConfigDefault.Error
	}

	return cfg
}
