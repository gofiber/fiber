package deprecation

import (
	"net/url"
	"strings"
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

	// Link is a URI reference that documents the deprecation, such as a
	// migration guide or a deprecation policy. It is added to the Link header
	// with rel="deprecation". It may be set without Deprecation to publish a
	// policy before anything is deprecated (RFC 9745 Section 3.1).
	//
	// Optional. Default: ""
	Link string

	// SunsetLink is a URI reference that documents the sunset policy. It is
	// added to the Link header with rel="sunset", and may be set without
	// Sunset (RFC 8594 Section 6).
	//
	// Optional. Default: ""
	SunsetLink string
}

// ConfigDefault is the default config. It sets nothing, so New panics with it:
// at least one date or link is required.
var ConfigDefault = Config{}

// maxSFInteger is the largest magnitude a Structured Field Integer, and so the
// seconds of a Structured Field Date, can carry (RFC 9651 Section 3.3.1).
const maxSFInteger = 999_999_999_999_999

// uriPunctuation is every byte RFC 3986 allows in a URI reference besides
// letters and digits: the unreserved and reserved characters and '%'.
const uriPunctuation = "-._~:/?#[]@!$&'()*+,;=%"

// Helper function to set default values and validate the config.
func configDefault(config ...Config) Config {
	cfg := ConfigDefault
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.Deprecation.IsZero() && cfg.Sunset.IsZero() && cfg.Link == "" && cfg.SunsetLink == "" {
		panic("deprecation: Deprecation, Sunset, Link or SunsetLink is required")
	}

	if !cfg.Deprecation.IsZero() {
		if sec := cfg.Deprecation.Unix(); sec < -maxSFInteger || sec > maxSFInteger {
			panic("deprecation: Deprecation is out of the range of a Structured Field Date")
		}
	}
	if !cfg.Sunset.IsZero() {
		if y := cfg.Sunset.UTC().Year(); y < 0 || y > 9999 {
			panic("deprecation: Sunset must fall within the years 0000 to 9999 of an HTTP-date")
		}
	}

	// Both headers carry whole seconds, so compare what is sent.
	if !cfg.Deprecation.IsZero() && !cfg.Sunset.IsZero() &&
		cfg.Sunset.Truncate(time.Second).Before(cfg.Deprecation.Truncate(time.Second)) {
		panic("deprecation: Sunset must not be earlier than Deprecation")
	}

	if cfg.Link != "" && !isURIReference(cfg.Link) {
		panic("deprecation: Link must be a URI reference")
	}
	if cfg.SunsetLink != "" && !isURIReference(cfg.SunsetLink) {
		panic("deprecation: SunsetLink must be a URI reference")
	}

	return cfg
}

// isURIReference reports whether s is a URI reference that can be written
// between the angle brackets of a Link header (RFC 8288 Section 3): only the
// characters RFC 3986 allows, parsable, and with a host when it is an http or
// https URI (RFC 9110 Section 4.2). A relative reference is allowed, and
// recipients resolve it against the request URI.
func isURIReference(s string) bool {
	for i := range len(s) {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') &&
			strings.IndexByte(uriPunctuation, c) < 0 {
			return false
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	return (u.Scheme != "http" && u.Scheme != "https") || u.Host != ""
}
