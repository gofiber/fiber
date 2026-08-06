// Package headerlookup answers, for a caller holding a fiber.Ctx, the two
// questions the byte-for-byte header API cannot: whether the stored field names
// are canonical, and what a named request header holds if they are not.
//
// The matching itself lives in internal/fieldname, which takes fasthttp header
// types so package fiber can use it too.
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
// case-insensitively (RFC 9110 Section 5.1).
//
// Ctx.Get compares byte for byte, so under DisableHeaderNormalizing a "referer:"
// or "origin:" from a client behind an HTTP/2 front end reads as absent — which
// makes an extractor refuse a request it should allow, and makes the CSRF origin
// check treat the absence as nothing to verify.
//
// The byte-for-byte lookup runs first, so nothing is walked in the default case.
func Value(c fiber.Ctx, name string) string {
	if v := c.Get(name); v != "" {
		return v
	}
	if Canonical(c) {
		return ""
	}

	return string(fieldname.First(&c.Request().Header, name, false))
}
