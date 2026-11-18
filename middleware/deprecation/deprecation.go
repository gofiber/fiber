package deprecation

import (
	"bytes"
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
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
		// panic, carry them. The spelling is read from the response: an earlier
		// middleware may have stopped normalizing it, whatever the app config says.
		h := &c.Response().Header
		canonical := fieldname.Canonical(h)
		if deprecation != "" {
			replace(c, h, fiber.HeaderDeprecation, deprecation, canonical)
		}
		if sunset != "" {
			replace(c, h, fiber.HeaderSunset, sunset, canonical)
		}
		if missing := missingLinks(h.Peek(fiber.HeaderLink), links, parts); len(missing) > 0 {
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

// replace sets name to value, removing the field under any other spelling.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func replace(c fiber.Ctx, h *fasthttp.ResponseHeader, name, value string, canonical bool) {
	// Set overwrites only the first line, so every earlier one goes first.
	fieldname.Del(h, name, canonical)
	c.Set(name, value)
}

// missingLinks returns the pairs of links whose link the Link value does not hold
// yet, so an instance that runs again on one response, as after
// c.RestartRouting, does not repeat its links.
func missingLinks(existing []byte, links []string, parts [][]byte) []string {
	if len(existing) == 0 {
		return links
	}
	// At most two links, so the common answers, all or none, need no copy.
	var held [2]bool
	n := 0
	for i, part := range parts {
		if held[i] = bytes.Contains(existing, part); held[i] {
			n++
		}
	}
	switch n {
	case 0:
		return links
	case len(parts):
		return nil
	}
	if held[0] {
		return links[2:]
	}

	return links[:2]
}
