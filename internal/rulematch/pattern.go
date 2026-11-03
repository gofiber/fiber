package rulematch

import (
	"regexp"
	"strings"
)

// MustCompileRegexp compiles a redirect pattern: regexp syntax with its groups
// and flags, each "*" a "(.*)" capture. An invalid pattern panics.
func MustCompileRegexp(from string) *regexp.Regexp {
	return mustCompileWhole(strings.ReplaceAll(from, "*", "(.*)"))
}

// MustCompilePath compiles a rewrite pattern: path text matched as written,
// each "*" a "(.*)" capture.
func MustCompilePath(from string) *regexp.Regexp {
	return mustCompileWhole(strings.ReplaceAll(regexp.QuoteMeta(from), `\*`, "(.*)"))
}

// mustCompileWhole anchors pattern to the whole path. Grouped before anchoring so
// alternatives cannot escape either anchor, without adding a capture that would
// shift the caller's "$N" substitutions (#4476).
func mustCompileWhole(pattern string) *regexp.Regexp {
	return regexp.MustCompile("^(?:" + pattern + ")$")
}
