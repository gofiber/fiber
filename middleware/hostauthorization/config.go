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

	// AllowedHostsFunc is a dynamic validator called only when no static
	// AllowedHosts rule matches. Receives the normalized hostname: port stripped,
	// trailing dot removed, IPv6 brackets removed, lowercased, IDN converted to
	// Punycode. An IPv6 address is passed in its RFC 5952 Section 4 form ("::1")
	// whatever spelling the client sent, an IPv4-mapped one as IPv4 ("127.0.0.1").
	// The string is only valid during the call; copy it to keep it.
	// Return true to allow.
	//
	// Optional. Default: nil
	AllowedHostsFunc func(host string) bool

	// ErrorHandler is called when a request is rejected.
	// Receives ErrForbiddenHost as the error.
	//
	// Optional. Default: returns 403 Forbidden.
	ErrorHandler fiber.ErrorHandler

	// AllowedHosts is the list of permitted host values.
	// Supports two match types:
	//   - Exact:     "api.myapp.com"
	//   - Subdomain: "*.myapp.com" (matches any subdomain, NOT the bare domain — list both for apex+subdomains)
	//
	// Entries are normalized at startup: port stripped, trailing dot removed,
	// lowercased, IDN labels converted to Punycode, IPv6 addresses written in
	// their RFC 5952 Section 4 form, RFC 1035 length limits enforced
	// (≤253 total / ≤63 per-label). An IPv4-mapped entry is stored as its IPv4
	// address ("[::ffff:10.0.0.5]" admits "10.0.0.5"). An entry with a colon that is
	// not an IPv6 address, or a "*." entry holding an IP address, panics.
	//
	// Required if AllowedHostsFunc is nil.
	AllowedHosts []string
}

// ConfigDefault is the default config.
var ConfigDefault = Config{}

func configDefault(config ...Config) Config {
	cfg := ConfigDefault
	if len(config) > 0 {
		cfg = config[0]
	}

	if len(cfg.AllowedHosts) == 0 && cfg.AllowedHostsFunc == nil {
		panic("hostauthorization: AllowedHosts or AllowedHostsFunc is required")
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c fiber.Ctx, _ error) error {
			return c.SendStatus(fiber.StatusForbidden)
		}
	}

	return cfg
}
