package deprecation

import (
	"bytes"
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// New creates a new middleware handler that marks responses as deprecated with
// the Deprecation (RFC 9745) and Sunset (RFC 8594) headers.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// The header values do not change between requests, so they are formatted once.
	var deprecation, sunset string
	if !cfg.Deprecation.IsZero() {
		deprecation = "@" + utils.FormatInt(cfg.Deprecation.Unix())
	}
	if !cfg.Sunset.IsZero() {
		sunset = utils.FormatHTTPDate(cfg.Sunset)
	}
	// links holds the URL and rel pairs c.Links takes, parts each pair as c.Links writes it.
	var links []string
	var parts [][]byte
	for _, p := range [...][2]string{{cfg.Link, "deprecation"}, {cfg.SunsetLink, "sunset"}} {
		if p[0] != "" {
			links = append(links, p[0], p[1])
			parts = append(parts, []byte("<"+p[0]+`>; rel="`+p[1]+`"`))
		}
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set before the chain runs, like helmet and cors, so a response the handler
		// writes itself with c.End(), and the 500 an outer recover.New() sends for a
		// panic, carry them.
		if deprecation != "" {
			c.Set(fiber.HeaderDeprecation, deprecation)
		}
		if sunset != "" {
			c.Set(fiber.HeaderSunset, sunset)
		}
		if missing := missingLinks(c.Response().Header.PeekAll(fiber.HeaderLink), links, parts); len(missing) > 0 {
			// c.Links may rewrite its arguments in a custom Ctx, and links is shared
			// by every request (see vary in middleware/cors).
			if dc, ok := c.(*fiber.DefaultCtx); ok {
				dc.Links(missing...)
			} else {
				c.Links(slices.Clone(missing)...)
			}
		}

		return c.Next()
	}
}

// missingLinks returns the pairs of links whose link no Link line holds yet, so
// an instance that runs again on one response, as after c.RestartRouting, does
// not repeat its links.
func missingLinks(lines [][]byte, links []string, parts [][]byte) []string {
	if len(links) == 0 {
		return nil
	}
	// At most two links, so the common answers, all or none, need no copy.
	missing := -1
	for i, part := range parts {
		if !holds(lines, part) {
			if missing >= 0 {
				return links
			}
			missing = i
		}
	}
	if missing < 0 {
		return nil
	}

	return links[2*missing : 2*missing+2]
}

// holds reports whether a line holds part as a whole link-value: one followed by
// more parameters, such as an anchor, describes another context.
func holds(lines [][]byte, part []byte) bool {
	for _, rest := range lines {
		for i := bytes.Index(rest, part); i >= 0; i = bytes.Index(rest, part) {
			rest = rest[i+len(part):]
			if after := utils.TrimLeft(rest, ' '); len(after) == 0 || after[0] == ',' {
				return true
			}
		}
	}
	return false
}
