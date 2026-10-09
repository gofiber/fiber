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
