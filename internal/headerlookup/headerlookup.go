// Package headerlookup reads a request header by name the way a recipient must,
// for the framework decisions that cannot be left to a byte-for-byte match.
package headerlookup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

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
// case, which is the default. This lives in one place because the same read is
// needed from packages that cannot see each other's helpers, and the copy that
// was not written is the one that stayed broken.
func Value(c fiber.Ctx, name string) string {
	if v := c.Get(name); v != "" {
		return v
	}
	if !c.App().Config().DisableHeaderNormalizing {
		return ""
	}

	for k, v := range c.Request().Header.All() {
		if utils.EqualFold(utils.UnsafeString(k), name) {
			return string(v)
		}
	}
	return ""
}
