// Package headerlookup answers, for a caller holding a fiber.Ctx, the two
// questions the byte-exact header API cannot: whether the stored field names are
// canonical, and what a named request header holds if they are not.
package headerlookup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
)

// Canonical reports whether fasthttp normalized this request's header names.
//
// fasthttp keeps the answer private, so take it from the app config that set
// it. This answers for the request store only: a proxied response is parsed by
// an outbound fasthttp.Client carrying its own setting.
func Canonical(c fiber.Ctx) bool {
	return !c.App().Config().DisableHeaderNormalizing
}

// Value returns the named request header, matching the field name
// case-insensitively (RFC 9110 §5.1). Ctx.Get is byte-exact, so a lower-case
// "origin:" read as absent and the CSRF check found nothing to verify.
//
// An empty answer from Ctx.Get is not taken for an absent header, whatever the
// names are spelled like: it reports the first line under that exact key, and a
// message carrying "Origin:" ahead of the real one reads as having none. So the
// walk backs up an empty result as well as a byte-exact miss.
func Value(c fiber.Ctx, name string) string {
	if v := c.Get(name); v != "" {
		return v
	}

	return string(fieldname.First(&c.Request().Header, name, Canonical(c)))
}
