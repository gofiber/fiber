package deprecation

import (
	"net/url"
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

	// Link is an absolute URI that documents the deprecation, such as a
	// migration guide or a deprecation policy. It is added to the Link header
	// with rel="deprecation". It may be set without Deprecation to publish a
	// policy before anything is deprecated (RFC 9745 Section 3.1).
	//
	// Optional. Default: ""
	Link string

	// SunsetLink is an absolute URI that documents the sunset policy. It is
	// added to the Link header with rel="sunset", and may be set without
	// Sunset (RFC 8594 Section 6).
	//
	// Optional. Default: ""
	SunsetLink string
}

// ConfigDefault is the default config. It sets nothing, so New panics with it:
// at least one date or link is required.
var ConfigDefault = Config{}

// Helper function to set default values and validate the config.
func configDefault(config ...Config) Config {
	cfg := ConfigDefault
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Deprecation.IsZero() && cfg.Sunset.IsZero() && cfg.Link == "" && cfg.SunsetLink == "" {
		panic("deprecation: Deprecation, Sunset, Link or SunsetLink is required")
	}

	// Both headers carry whole seconds, so compare what is sent.
	if !cfg.Deprecation.IsZero() && !cfg.Sunset.IsZero() &&
		cfg.Sunset.Truncate(time.Second).Before(cfg.Deprecation.Truncate(time.Second)) {
		panic("deprecation: Sunset must not be earlier than Deprecation")
	}

	if cfg.Link != "" && !isLinkTarget(cfg.Link) {
		panic("deprecation: Link must be an absolute URI")
	}
	if cfg.SunsetLink != "" && !isLinkTarget(cfg.SunsetLink) {
		panic("deprecation: SunsetLink must be an absolute URI")
	}

	return cfg
}

// isLinkTarget reports whether s can be written between the angle brackets of
// a Link header (RFC 8288 Section 3): an absolute URI with no '<', '>',
// whitespace or control character, which would end the target early.
func isLinkTarget(s string) bool {
	for i := range len(s) {
		if c := s[i]; c <= ' ' || c == '<' || c == '>' || c == 0x7f {
			return false
		}
	}
	u, err := url.Parse(s)

	return err == nil && u.IsAbs()
}
