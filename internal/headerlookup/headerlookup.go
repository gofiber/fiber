// Package headerlookup answers, for a caller holding a fiber.Ctx, the two
// questions the byte-for-byte header API cannot: whether the stored field names
// are canonical, and what a named request header holds if they are not.
//
// The matching itself lives in internal/fieldname, which takes fasthttp header
// types so package fiber can use it too. This package is the thin part that
// needs a Ctx.
package headerlookup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
)

// Canonical reports whether fasthttp normalized this request's header names,
// which decides whether a byte-for-byte lookup finds all of them.
//
// fasthttp keeps the answer private, so take it from the app config that set
// it. Getting this wrong in the safe direction only costs a walk; wrong the
// other way loses field lines, and what that means depends on the caller — for
// Authorization it reads a request as anonymous, and for a sanitizer it leaves
// one removing nothing.
//
// This answers for the request store, which is the server's. A proxied response
// is parsed by an outbound fasthttp.Client carrying its own setting, so this
// says nothing about the keys stored there — ask the header instead.
func Canonical(c fiber.Ctx) bool {
	return !c.App().Config().DisableHeaderNormalizing
}

// Value returns the value of the request header named name, matching the field
// name case-insensitively (RFC 9110 Section 5.1).
//
// Ctx.Get compares the stored key byte for byte, which finds the header only
// while fasthttp has canonicalized it. Under DisableHeaderNormalizing the store
// keeps the spelling the client sent — and lower case is not exotic there:
// HTTP/2 and HTTP/3 require it on the wire, so it is what a front end
// translating down to HTTP/1.1 preserves. A framework check reading through Get
// then sees no header at all, and what that means depends on the check: a
// credential extractor refuses a request it should have allowed, while the CSRF
// origin check treats the absence as nothing to verify and lets a cross-site
// POST through.
//
// The byte-for-byte lookup runs first, so nothing is walked in the normalizing
// case, which is the default.
func Value(c fiber.Ctx, name string) string {
	if v := c.Get(name); v != "" {
		return v
	}
	if Canonical(c) {
		return ""
	}

	return string(fieldname.First(&c.Request().Header, name, false))
}
