// Package etag parses and compares HTTP entity tags (RFC 9110 Section 8.8.3).
// It is shared by the core conditional-request path (Ctx.Fresh) and the ETag
// middleware, which previously carried divergent parsers: the middleware split
// If-None-Match on every comma, mis-parsing an opaque-tag that contains one.
package etag

import (
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/utils/v2"
)

// weakPrefix marks a weak entity tag (RFC 9110 Section 8.8.3).
const weakPrefix = "W/"

// Parse validates an entity tag and returns the value without quotes.
// weak is true if the tag has the "W/" prefix.
func Parse(t string) (value string, weak, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the parsed ETag components
	weak = strings.HasPrefix(t, weakPrefix)
	if weak {
		t = t[len(weakPrefix):]
	}

	if len(t) < 2 || t[0] != '"' || t[len(t)-1] != '"' {
		return "", weak, false
	}
	return t[1 : len(t)-1], weak, true
}

// Match performs a weak comparison of entity tags according to
// RFC 9110 Section 8.8.3.2. The weak indicator ("W/") is ignored, but both tags
// must be properly quoted. Invalid tags result in a mismatch.
func Match(s, etag string) bool {
	n1, _, ok1 := Parse(s)
	n2, _, ok2 := Parse(etag)
	if !ok1 || !ok2 {
		return false
	}

	return n1 == n2
}

// MatchStrong performs a strong entity-tag comparison following
// RFC 9110 Section 8.8.3.1. A weak tag never matches a strong one, even if the
// quoted values are identical.
func MatchStrong(s, etag string) bool {
	n1, w1, ok1 := Parse(s)
	n2, w2, ok2 := Parse(etag)
	if !ok1 || !ok2 || w1 || w2 {
		return false
	}

	return n1 == n2
}

// Split returns the entity tags in a raw If-None-Match or If-Match field value.
// A comma inside a quoted opaque-tag does not separate it (RFC 9110
// Section 8.8.3), and empty list elements are skipped. Returns nil for no tags.
func Split(header string) []string {
	if header == "" {
		return nil
	}

	// One more than the commas is an upper bound: a comma inside a quoted
	// opaque-tag only over-estimates, never under.
	tags := slices.AppendSeq(make([]string, 0, strings.Count(header, ",")+1), headerlist.AllQuoted(header))
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// AnyMatch reports whether any entity tag in the raw If-None-Match field value
// matches etag. Comparison is weak as defined by RFC 9110 Section 8.8.3.2, and
// "*" matches every entity tag. An empty field value matches nothing.
func AnyMatch(header, etag string) bool {
	rest := utils.TrimSpace(header)

	// Short-circuit the wildcard case: "*" matches any entity tag.
	if rest == "*" {
		return true
	}

	// etag is the same for every element, so parse it once rather than through
	// Match on each. An unparseable one matches nothing (RFC 9110 8.8.3.2).
	want, _, ok := Parse(etag)
	if !ok {
		return false
	}

	// Commas inside the opaque-tag do not separate: etagc permits "," within the
	// quoted tag (RFC 9110 Section 8.8.3), so `"v1,v2"` is one entity tag.
	for tag := range headerlist.AllQuoted(header) {
		if got, _, ok := Parse(tag); ok && got == want {
			return true
		}
	}

	return false
}
