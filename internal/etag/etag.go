// Package etag parses and compares HTTP entity tags (RFC 9110 Section 8.8.3).
// It is shared by the core conditional-request path (Ctx.Fresh) and the ETag
// middleware, which previously carried divergent parsers: the middleware split
// If-None-Match on every comma, mis-parsing an opaque-tag that contains one.
package etag

import (
	"iter"
	"slices"
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

// cutTag splits the first entity tag off a raw If-None-Match or If-Match field
// value, returning it and the remainder of the list. ok is false only when
// there is nothing left to split.
//
// A comma inside a DQUOTE-delimited opaque-tag does not end an element: etagc
// permits "," inside the quoted tag (RFC 9110 Section 8.8.3), so `"v1,v2"` is a
// single entity tag rather than two list elements.
//
// Written as a stepper rather than a closure so AnyMatch, which runs on the
// conditional-request path, shares the rule without paying for an iterator.
func cutTag(header string) (tag, rest string, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the three parts
	if header == "" {
		return "", "", false
	}

	// Only '"' and ',' affect the split, so jump between them instead of
	// visiting every byte.
	inQuotes := false
	for i := 0; i < len(header); {
		j := utils.IndexAny2(header[i:], '"', ',')
		if j == -1 {
			break
		}
		i += j
		if header[i] == '"' {
			inQuotes = !inQuotes
		} else if !inQuotes {
			return utils.TrimSpace(header[:i]), header[i+1:], true
		}
		i++
	}

	return utils.TrimSpace(header), "", true
}

// Tags iterates the entity tags in a raw If-None-Match or If-Match field value.
// Elements are yielded with surrounding whitespace trimmed and are not
// validated; pass each to Parse to reject malformed ones.
//
// Empty list elements are skipped rather than yielded as empty strings, because
// RFC 9110 Section 5.6.1 requires a recipient to parse and ignore them: a
// trailing comma is not an extra tag. An empty field value yields nothing.
func Tags(header string) iter.Seq[string] {
	return func(yield func(string) bool) {
		rest := utils.TrimSpace(header)
		for rest != "" {
			tag, next, ok := cutTag(rest)
			if !ok {
				return
			}
			if tag != "" && !yield(tag) {
				return
			}
			rest = next
		}
	}
}

// Split returns the entity tags in a raw If-None-Match or If-Match field value.
// It collects Tags, so the same quoted-comma and empty-element rules apply. A
// field value carrying no tags returns nil.
func Split(header string) []string {
	header = utils.TrimSpace(header)
	if header == "" {
		return nil
	}

	// One more than the commas is an upper bound: a comma inside a quoted
	// opaque-tag only over-estimates, never under.
	tags := slices.AppendSeq(make([]string, 0, strings.Count(header, ",")+1), Tags(header))
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

	for rest != "" {
		tag, next, ok := cutTag(rest)
		if !ok {
			return false
		}
		if tag != "" && Match(tag, etag) {
			return true
		}
		rest = next
	}

	return false
}
