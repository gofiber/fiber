package deprecation

import (
	"bytes"
	"slices"

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
	l := newLinks(cfg.Link, cfg.SunsetLink)

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set before the chain runs, so a response the handler writes itself with
		// c.End(), and the 500 an outer recover.New() sends for a panic, carry them.
		h := &c.Response().Header
		canonical := headerlookup.Canonical(c)
		if deprecation != "" {
			replace(c, h, fiber.HeaderDeprecation, deprecation, canonical)
		}
		if sunset != "" {
			replace(c, h, fiber.HeaderSunset, sunset, canonical)
		}
		l.add(c, h, canonical)

		err := c.Next()

		// A handler timeout.New gave up on may still write the response; IsAbandoned orders the read.
		if c.IsAbandoned() && c.RequestCtx().LastTimeoutErrorResponse() != nil {
			return err
		}

		// A proxied response, c.Response().Reset() and the 304 or 416 of SendFile
		// replace the header block, and a later handler may remove or add a line,
		// so each header is checked on its own. The keyed reads answer for the
		// common case, where every line set above is still there and alone.
		if canonical && intact(h, deprecation != "", sunset != "", l.parts) {
			return err
		}

		// A proxied response may hold field names as the upstream spelled them.
		canonical = fieldname.Canonical(h)
		if deprecation != "" {
			keepOne(c, h, fiber.HeaderDeprecation, deprecation, canonical)
		}
		if sunset != "" {
			keepOne(c, h, fiber.HeaderSunset, sunset, canonical)
		}
		l.add(c, h, canonical)

		return err
	}
}

// intact reports, in one walk over the canonical names, whether each configured
// date has exactly one line and the Link lines hold every link.
//
//nolint:revive // flag-parameter: the flags say which headers are configured
func intact(h *fasthttp.ResponseHeader, deprecation, sunset bool, parts [][]byte) bool {
	if fieldname.HasTrailers(h) {
		return false
	}
	var dep, sun int
	var held uint8
	for k, v := range h.All() {
		switch string(k) {
		case fiber.HeaderDeprecation:
			dep++
		case fiber.HeaderSunset:
			sun++
		case fiber.HeaderLink:
			for i, part := range parts {
				if bytes.Contains(v, part) {
					held |= 1 << i
				}
			}
		}
	}

	return (!deprecation || dep == 1) && (!sunset || sun == 1) && held == 1<<len(parts)-1
}

// replace sets name to value, removing the field under any other spelling.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func replace(c fiber.Ctx, h *fasthttp.ResponseHeader, name, value string, canonical bool) {
	if !canonical {
		fieldname.Del(h, name, false)
	}
	c.Set(name, value)
}

// keepOne leaves the single-valued field name with exactly one line (RFC 9110
// Section 5.3): value when the line is gone, and a later writer's value over
// ours when both are there.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func keepOne(c fiber.Ctx, h *fasthttp.ResponseHeader, name, value string, canonical bool) {
	lines := fieldname.Lines(h, name, canonical)
	switch len(lines) {
	case 1:
		return
	case 0:
		c.Set(name, value)
		return
	}
	// fasthttp's Del moves the last line into the gap, so the order of the lines
	// says nothing; ours is the one the middleware set before the chain.
	keep := value
	for _, v := range lines {
		if string(v) != value {
			keep = string(v)
			break
		}
	}
	fieldname.Del(h, name, canonical)
	c.Set(name, keep)
}

// links holds the Link values the middleware adds, formatted once.
type links struct {
	joined string   // every link, the Link value when the response has none
	parts  [][]byte // each link on its own, to find the ones a Link line holds
	pairs  []string // the URL and rel pairs c.Links takes, two per part
}

func newLinks(deprecationLink, sunsetLink string) links {
	var l links
	for _, p := range [][2]string{{deprecationLink, "deprecation"}, {sunsetLink, "sunset"}} {
		if p[0] == "" {
			continue
		}
		part := "<" + p[0] + `>; rel="` + p[1] + `"`
		if l.joined != "" {
			l.joined += ","
		}
		l.joined += part
		l.parts = append(l.parts, []byte(part))
		l.pairs = append(l.pairs, p[0], p[1])
	}

	return l
}

// add writes the links as the Link header, or folds the ones no Link line holds
// yet into the Link header already set, under any spelling of its name.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func (l *links) add(c fiber.Ctx, h *fasthttp.ResponseHeader, canonical bool) {
	if l.joined == "" {
		return
	}
	lines := fieldname.Lines(h, fiber.HeaderLink, canonical)
	if len(lines) == 0 {
		c.Set(fiber.HeaderLink, l.joined)
		return
	}

	missing := l.pairs
	if found := l.held(lines); found != 0 {
		if found == len(l.parts) {
			return
		}
		// Only one of two links is held, so the other pair is the one left.
		missing = l.pairs[2:]
		if !holds(lines, l.parts[0]) {
			missing = l.pairs[:2]
		}
	}

	// c.Links may rewrite its arguments in a custom Ctx, and pairs is shared by
	// every request (see vary in middleware/cors).
	if dc, ok := c.(*fiber.DefaultCtx); ok {
		dc.Links(missing...)
		return
	}
	c.Links(slices.Clone(missing)...)
}

// held counts the links some Link line already holds.
func (l *links) held(lines [][]byte) int {
	n := 0
	for _, part := range l.parts {
		if holds(lines, part) {
			n++
		}
	}

	return n
}

// holds reports whether one of lines holds part.
func holds(lines [][]byte, part []byte) bool {
	for _, v := range lines {
		if bytes.Contains(v, part) {
			return true
		}
	}

	return false
}
