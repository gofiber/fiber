// Package headerlist walks the comma-separated list values that RFC 9110
// Section 5.6.1 defines, and joins the field lines that Section 5.3 says are
// equivalent to one.
//
// Seven scanners used to spell this out separately: three in the core
// (helpers.go, ctx.go, res.go) and four in middleware (adaptor, proxy, compress,
// cache). They agreed on splitting at commas and trimming OWS, and disagreed on
// everything after that, so the divergences are named here rather than left to
// be rediscovered per call site:
//
//   - Case. [Contains] compares exactly, [ContainsFold] under ASCII case
//     folding. Both exist because the callers genuinely differ: a Vary field
//     name is echoed back to the client as it was written, while a token like
//     "close" or "gzip" is matched however it arrives.
//   - Quoting. [All] treats every comma as a separator, which is what a list of
//     bare tokens wants. [AllQuoted] keeps commas that sit inside a quoted
//     string, which is what a list of entity tags or media-type parameters
//     needs, since those may contain one.
//
// Empty elements are skipped throughout: "a,,b" yields "a" and "b". A list is
// permitted to carry them (RFC 9110 Section 5.6.1 allows the empty element for
// legacy reasons) and no caller here has ever wanted one.
package headerlist

import (
	"iter"

	"github.com/gofiber/utils/v2"
)

// All yields each non-empty element of a comma-separated list value, with
// leading and trailing OWS removed.
//
// Every comma separates. Use [AllQuoted] for a list whose elements may contain
// a quoted comma.
func All(list string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := 0
		for i := range len(list) {
			if list[i] != ',' {
				continue
			}
			if element := utils.TrimSpace(list[start:i]); element != "" {
				if !yield(element) {
					return
				}
			}
			start = i + 1
		}
		if element := utils.TrimSpace(list[start:]); element != "" {
			yield(element)
		}
	}
}

// AllQuoted is [All] for lists whose elements may contain a comma inside a
// quoted string, such as If-None-Match, where the opaque-tag `"v1,v2"` is one
// entity tag and not two elements (RFC 9110 Section 8.8.3).
//
// Only '"' and ',' can affect where an element ends, so the walk jumps between
// them instead of visiting every byte. A quoted string left open by a malformed
// value swallows the rest of the list, which keeps a truncated field from
// splitting into elements the sender did not write.
func AllQuoted(list string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start, pos := 0, 0
		inQuotes := false
		for {
			i := utils.IndexAny2(list[pos:], '"', ',')
			if i == -1 {
				break
			}
			i += pos
			pos = i + 1

			if list[i] == '"' {
				inQuotes = !inQuotes
				continue
			}
			if inQuotes {
				continue
			}
			if element := utils.TrimSpace(list[start:i]); element != "" {
				if !yield(element) {
					return
				}
			}
			start = i + 1
		}
		if element := utils.TrimSpace(list[start:]); element != "" {
			yield(element)
		}
	}
}

// AllLines yields the elements of every field line in order, so that a header
// sent as one line and the same header sent as several read alike (RFC 9110
// Section 5.3).
//
// The lines are read as-is; the caller keeps whatever storage they alias.
func AllLines(lines [][]byte) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, line := range lines {
			for element := range All(utils.UnsafeString(line)) {
				if !yield(element) {
					return
				}
			}
		}
	}
}

// Contains reports whether list has an element equal to value, compared byte
// for byte. An empty value is never present.
//
// value is matched against the elements [All] yields, so it is expected to be
// one element and to carry no OWS of its own: " gzip " is not an element of
// " gzip ", though "gzip" is.
//
// Use [ContainsFold] to match a token whose case the sender chose.
func Contains(list, value string) bool {
	if value == "" {
		return false
	}

	for element := range All(list) {
		if element == value {
			return true
		}
	}
	return false
}

// ContainsFold reports whether list has an element equal to value under ASCII
// case folding. An empty value is never present.
func ContainsFold(list, value string) bool {
	if value == "" {
		return false
	}

	for element := range All(list) {
		if utils.EqualFold(element, value) {
			return true
		}
	}
	return false
}

// Append appends the elements of list to dst and returns it, reusing dst's
// storage. It returns nil for an empty list, leaving dst untouched.
//
// This is the allocation-free counterpart to [All], for the callers that need
// the elements to outlive the walk. It is written as a loop rather than over
// [All] because it sits on the per-request Accept-Encoding path.
func Append(dst []string, list string) []string {
	if list == "" {
		return nil
	}

	dst = dst[:0]
	start := 0
	for i := range len(list) {
		if list[i] != ',' {
			continue
		}
		if element := utils.TrimSpace(list[start:i]); element != "" {
			dst = append(dst, element)
		}
		start = i + 1
	}
	if element := utils.TrimSpace(list[start:]); element != "" {
		dst = append(dst, element)
	}

	return dst
}

// Join renders the field lines of one header as the single combined value that
// RFC 9110 Section 5.3 says they are equivalent to, separated by a bare comma.
//
// A lone line is returned as it stands, so the result aliases the caller's
// storage and must not be retained past it. Only the multi-line case allocates,
// and that case is rare.
func Join(lines [][]byte) []byte {
	switch len(lines) {
	case 0:
		return nil
	case 1:
		return lines[0]
	}

	n := len(lines) - 1
	for _, line := range lines {
		n += len(line)
	}

	joined := make([]byte, 0, n)
	for i, line := range lines {
		if i > 0 {
			joined = append(joined, ',')
		}
		joined = append(joined, line...)
	}
	return joined
}

// AppendUnique adds each value that list does not already carry, separated by
// ", ", and returns the result. It returns "" when nothing was added, so a
// caller can tell "unchanged" from "changed" without comparing the strings.
//
// Presence is decided by [Contains], byte for byte: a value already listed in
// another case is added again rather than silently dropped, since the field
// this serves is echoed to the client as written.
func AppendUnique(list string, values []string) string {
	original := list
	for _, value := range values {
		if value == "" {
			continue
		}
		switch {
		case list == "":
			list = value
		case !Contains(list, value):
			list += ", " + value
		}
	}

	if list == original {
		return ""
	}
	return list
}
