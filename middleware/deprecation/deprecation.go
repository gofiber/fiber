package deprecation

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// New creates a new middleware handler that marks responses as deprecated with
// the Deprecation (RFC 9745) and Sunset (RFC 8594) headers.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// The header values do not change between requests, so they are formatted once.
	var deprecation, sunset string
	if !cfg.Deprecation.IsZero() {
		deprecation = "@" + strconv.FormatInt(cfg.Deprecation.Unix(), 10)
	}
	if !cfg.Sunset.IsZero() {
		sunset = cfg.Sunset.UTC().Format(http.TimeFormat)
	}

	var links []string
	if cfg.Link != "" {
		links = append(links, cfg.Link, "deprecation")
	}
	if cfg.SunsetLink != "" {
		links = append(links, cfg.SunsetLink, "sunset")
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set before the chain runs so error responses carry the headers too.
		if deprecation != "" {
			c.Set(fiber.HeaderDeprecation, deprecation)
		}
		if sunset != "" {
			c.Set(fiber.HeaderSunset, sunset)
		}
		c.Links(links...)

		return c.Next()
	}
}
