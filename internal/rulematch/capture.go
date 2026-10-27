// Package rulematch provides capture substitution shared by redirect and rewrite
// rules. Callers own pattern compilation and destination normalization.
package rulematch

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
)

// CaptureTokens matches input and returns a replacer for its numbered capture
// groups ($1, $2, ...), or nil when the pattern does not match. A match without
// capture groups returns a non-nil replacer that leaves the target unchanged.
// Captures are preserved verbatim unless unescaped is set. In that case, path
// segments are escaped before substitution because rewrite decodes the resulting
// path again. This prevents a literal "%2e%2e" capture from becoming "..".
// Empty values and trailing slashes are preserved in either mode.
//
// Adapted from https://github.com/labstack/echo/blob/master/middleware/rewrite.go.
func CaptureTokens(pattern *regexp.Regexp, input string, unescaped bool) *strings.Replacer { //nolint:revive // flag-parameter: unescaped is the app's UnescapePath setting
	groups := pattern.FindStringSubmatch(input)
	if groups == nil {
		return nil
	}
	values := groups[1:]
	replace := make([]string, 0, 2*len(values))
	// Highest index first: a Replacer takes the earliest listed key that matches,
	// so "$1" ahead of "$10" read the tenth capture as the first followed by "0".
	for i, v := range slices.Backward(values) {
		if unescaped {
			v = string(utils.AppendPathSegmentsEscape(nil, v))
		}
		replace = append(replace, "$"+strconv.Itoa(i+1), v)
	}
	return strings.NewReplacer(replace...)
}

// DotSegments counts the "." and ".." segments of s, reading "%2e" as a dot and
// "\", "%2f" and "%5c" as slashes, as some reader of the result may. "?" and "#"
// end a segment but not the count, so a capture cannot hide the author's own.
func DotSegments(s string) int {
	count := 0
	// dots in the current segment, -1 once it holds anything else
	dots := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) {
			switch hi, lo := s[i+1], s[i+2]|0x20; {
			case hi == '2' && lo == 'e':
				c, i = '.', i+2
			case hi == '2' && lo == 'f', hi == '5' && lo == 'c':
				c, i = '/', i+2
			}
		}
		switch {
		case c == '/', c == '\\', c == '?', c == '#':
			if dots == 1 || dots == 2 {
				count++
			}
			dots = 0
		case c == '.' && dots >= 0:
			dots++
		default:
			dots = -1
		}
	}
	if dots == 1 || dots == 2 {
		count++
	}
	return count
}
