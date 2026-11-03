package rulematch

import (
	"regexp"
	"strings"
)

// Syntax controls whether non-wildcard pattern text is literal or regexp syntax.
type Syntax uint8

const (
	// Literal quotes everything except the "*" wildcard, as rewrite requires.
	Literal Syntax = iota
	// Regexp preserves regexp syntax and capture groups, as redirect requires.
	Regexp
)

// MustCompile compiles a whole-path pattern, replacing each "*" with "(.*)".
// Regexp syntax preserves existing groups and flags; Literal syntax treats all
// other characters as path text. Invalid patterns panic.
func MustCompile(from string, syntax Syntax) *regexp.Regexp {
	wildcard := "*"
	if syntax == Literal {
		from = regexp.QuoteMeta(from)
		wildcard = `\*`
	}
	pattern := strings.ReplaceAll(from, wildcard, "(.*)")
	// Group before anchoring so alternatives cannot escape either anchor, without
	// adding a capture that would shift the caller's "$N" substitutions (#4476).
	return regexp.MustCompile("^(?:" + pattern + ")$")
}
