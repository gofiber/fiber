package deprecation

import (
	"bytes"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
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
	link, links := formatLinks(cfg.Link, cfg.SunsetLink)
	linkBytes := []byte(link)

	// Replacing the header block drops every header set below at once, so the
	// first of them tells whether the response still carries them.
	sentinel, sentinelValue := fiber.HeaderLink, linkBytes
	switch {
	case deprecation != "":
		sentinel, sentinelValue = fiber.HeaderDeprecation, nil
	case sunset != "":
		sentinel, sentinelValue = fiber.HeaderSunset, nil
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set before the chain runs, so a response the handler writes itself with
		// c.End(), and the 500 an outer recover.New() sends for a panic, carry them.
		if deprecation != "" {
			c.Set(fiber.HeaderDeprecation, deprecation)
		}
		if sunset != "" {
			c.Set(fiber.HeaderSunset, sunset)
		}
		if link != "" {
			addLinks(c, link, links, headerlookup.Canonical(c))
		}

		err := c.Next()

		// A handler timeout.New gave up on may still write the response; IsAbandoned orders the read.
		if c.IsAbandoned() && c.RequestCtx().LastTimeoutErrorResponse() != nil {
			return err
		}

		// A proxied response, c.Response().Reset() and the 304 or 416 of SendFile
		// replace the header block. The byte-exact read answers for the common
		// case, where the headers set above are still there.
		h := &c.Response().Header
		if v := h.Peek(sentinel); len(v) > 0 && (sentinelValue == nil || bytes.Contains(v, sentinelValue)) {
			return err
		}

		// A proxied response may hold field names as the upstream spelled them.
		canonical := fieldname.Canonical(h)
		if deprecation != "" && len(fieldname.First(h, fiber.HeaderDeprecation, canonical)) == 0 {
			c.Set(fiber.HeaderDeprecation, deprecation)
		}
		if sunset != "" && len(fieldname.First(h, fiber.HeaderSunset, canonical)) == 0 {
			c.Set(fiber.HeaderSunset, sunset)
		}
		if link != "" && !hasLink(h, linkBytes, canonical) {
			addLinks(c, link, links, canonical)
		}

		return err
	}
}

// formatLinks returns the Link value the middleware adds when the response has
// no Link header yet, and the pairs c.Links takes to fold them into one.
//
//nolint:nonamedreturns // gocritic unnamedResult prefers naming the value and the pairs
func formatLinks(deprecationLink, sunsetLink string) (link string, links []string) {
	if deprecationLink != "" {
		links = append(links, deprecationLink, "deprecation")
		link = "<" + deprecationLink + `>; rel="deprecation"`
	}
	if sunsetLink != "" {
		links = append(links, sunsetLink, "sunset")
		if link != "" {
			link += ","
		}
		link += "<" + sunsetLink + `>; rel="sunset"`
	}

	return link, links
}

// addLinks writes link as the Link header, or folds the links into the Link
// header already set, under any spelling of its name.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func addLinks(c fiber.Ctx, link string, links []string, canonical bool) {
	if len(fieldname.First(&c.Response().Header, fiber.HeaderLink, canonical)) == 0 {
		c.Set(fiber.HeaderLink, link)
		return
	}
	c.Links(links...)
}

// hasLink reports whether a Link field line of h, under any spelling of the
// name, holds link.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func hasLink(h *fasthttp.ResponseHeader, link []byte, canonical bool) bool {
	for _, v := range fieldname.Lines(h, fiber.HeaderLink, canonical) {
		if bytes.Contains(v, link) {
			return true
		}
	}

	return false
}
