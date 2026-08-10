// Package headerlookup answers, for a caller holding a fiber.Ctx, the two
// questions the byte-exact header API cannot: whether the stored field names are
// canonical, and what a named request header holds if they are not.
package headerlookup

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/fieldname"
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

// Combined returns the named request header as the one value RFC 9110 §5.3
// gives a field carried on several lines: their values in the order received,
// separated by ", ".
//
// This is for a caller naming a field it does not choose — extractors take the
// name from the application's config, and Accept, Forwarded and the rest are
// list fields a peer may legally send twice. Value's refusal is wrong for them,
// because two lines there are one value, not two answers. Where the field is a
// single value the combination is still not one: a credential joined with
// another does not match the one that was issued, so a caller comparing it
// refuses, which is the outcome Value reaches directly.
func Combined(c fiber.Ctx, name string) string {
	cfg := c.App().Config()
	h := &c.Request().Header

	if utils.EqualFold(name, fiber.HeaderCookie) {
		// Cookie is not a comma-separated list — RFC 6265 §5.4 joins its
		// crumbs with "; " — and fasthttp keeps them in a store of their own,
		// which PeekAll enumerates one cookie at a time. Joining those would
		// hand the caller "a=1, b=2" for a request that said "a=1; b=2".
		// Collecting first makes fasthttp reassemble the field, so what is read
		// below is the one line it actually holds. The lookup is expected to
		// miss; collecting is the point of the call, as it is in the cache's
		// keyFieldLines.
		h.Cookie("")
	}

	lines := fieldname.Lines(h, name, !cfg.DisableHeaderNormalizing)

	switch len(lines) {
	case 0:
		return ""
	case 1:
		if len(lines[0]) == 0 {
			return ""
		}
		if cfg.Immutable {
			return string(lines[0])
		}
		return utils.UnsafeString(lines[0])
	}

	n := 2 * (len(lines) - 1)
	for _, line := range lines {
		n += len(line)
	}
	joined := make([]byte, 0, n)
	for i, line := range lines {
		if i > 0 {
			joined = append(joined, ',', ' ')
		}
		joined = append(joined, line...)
	}
	// The buffer is this function's own, so no copy is owed to Immutable.
	return utils.UnsafeString(joined)
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
