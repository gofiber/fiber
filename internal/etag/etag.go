// Package etag parses and compares HTTP entity tags (RFC 9110 Section 8.8.3).
// It is shared by the core conditional-request path (Ctx.Fresh) and the ETag
// middleware, which previously carried divergent parsers: the middleware split
// If-None-Match on every comma, mis-parsing an opaque-tag that contains one.
package etag

import (
	"strings"

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

// AnyMatch reports whether any entity tag in the raw If-None-Match field value
// matches etag. Comparison is weak as defined by RFC 9110 Section 8.8.3.2, and
// "*" matches every entity tag. An empty field value matches nothing.
func AnyMatch(header, etag string) bool {
	header = utils.TrimSpace(header)

	// Short-circuit the wildcard case: "*" matches any entity tag.
	if header == "*" {
		return true
	}

	// Split the header on commas that sit outside DQUOTE-delimited opaque-tags:
	// etagc permits "," inside the quoted tag (RFC 9110 Section 8.8.3), so
	// `"v1,v2"` is a single entity tag, not two list elements. Only '"' and ','
	// affect the split, so jump between them instead of visiting every byte.
	start := 0
	pos := 0
	inQuotes := false
	for {
		i := utils.IndexAny2(header[pos:], '"', ',')
		if i == -1 {
			break
		}
		i += pos
		pos = i + 1
		if header[i] == '"' {
			inQuotes = !inQuotes
		} else if !inQuotes {
			if Match(utils.TrimSpace(header[start:i]), etag) {
				return true
			}
			start = i + 1
		}
	}

	return Match(utils.TrimSpace(header[start:]), etag)
}
