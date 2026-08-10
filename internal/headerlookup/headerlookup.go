// Package headerlookup answers, for a caller holding a fiber.Ctx, the two
// questions the byte-exact header API cannot: whether the stored field names are
// canonical, and what a named request header holds if they are not.
package headerlookup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
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
// ok is false where the message carries the field more than once, whatever the
// lines are spelled like or hold. Every caller here reads a field defined as a
// single value — Origin, Referer, Sec-Fetch-Site, Authorization — for which
// RFC 9110 §5.2 says a sender must not generate a second line at all, so such a
// message is malformed and what it means is not a question with an answer.
// Answering with any one of the lines lets whoever wrote the other one decide:
// picking the first hides a value behind an empty line, and picking the first
// non-empty defeats a middleware that cleared the field while a differently
// spelled line from the client remained.
//
// ok is true for a single line that is present and empty, which is not
// ambiguous — it says the field holds nothing, and "" is that answer.
//
// A caller must not read a false ok as an absent header: absent is a state
// these fields are allowed to be in and some checks skip themselves for it,
// which is the opposite of what a malformed message should get. Refuse instead.
// The value is empty whenever ok is false, so a caller that only refuses empty
// values still fails closed.
func Value(c fiber.Ctx, name string) (string, bool) {
	cfg := c.App().Config()
	h := &c.Request().Header

	var (
		found []byte
		ok    bool
	)
	if cfg.DisableHeaderNormalizing {
		found, ok = foldValue(h, name)
	} else {
		found, ok = canonicalValue(h, name)
	}
	if !ok || len(found) == 0 {
		return "", ok
	}

	// The bytes live as long as the request, so a copy is made only where
	// Immutable promises the caller one — matching what Ctx.Get hands back.
	if cfg.Immutable {
		return string(found), true
	}
	return utils.UnsafeString(found), true
}

// canonicalValue answers Value for a store whose field names fasthttp folded on
// the way in, so one PeekAll — which costs no allocation and resolves the names
// kept in a slot of their own — sees every spelling at once.
func canonicalValue(h *fasthttp.RequestHeader, name string) ([]byte, bool) {
	lines := h.PeekAll(name)
	if len(lines) > 1 {
		return nil, false
	}
	if len(lines) == 0 {
		return nil, true
	}
	return lines[0], true
}

// foldValue answers Value for a store keeping whatever spelling the peer sent.
//
// Split out rather than written beside canonicalValue: All's iterator is
// heap-allocated where the compiler cannot see the branch is dead, so holding it
// in Value would cost the normalized path an allocation it never uses.
func foldValue(h *fasthttp.RequestHeader, name string) ([]byte, bool) {
	var found []byte
	seen := 0
	for k, v := range h.All() {
		if !utils.EqualFold(utils.UnsafeString(k), name) {
			continue
		}
		seen++
		if seen > 1 {
			return nil, false
		}
		found = v
	}
	return found, true
}
