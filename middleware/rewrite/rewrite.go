package rewrite

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type compiledRule struct {
	pattern *regexp.Regexp
	to      string
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Initialize
	rules := orderedRules(cfg)
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		compiled = append(compiled, compiledRule{
			pattern: compilePattern(rule.From),
			to:      rule.To,
		})
	}

	// Bound before the closure so it captures this alone.
	next := cfg.Next

	// Middleware function
	return func(c fiber.Ctx) error {
		// Next request to skip middleware
		if next != nil && next(c) {
			return c.Next()
		}
		// Rewrite
		for _, rule := range compiled {
			replacer := captureTokens(rule.pattern, c.Path())
			if replacer != nil {
				c.Path(replacer.Replace(rule.to))
				break
			}
		}
		return c.Next()
	}
}

// compilePattern turns a rule into the regexp that matches it. Everything but
// "*" is quoted, so a rule is the path text it spells: "/preis-1.000" matches
// that path and not "/preis-1X000". "*" stands for any run of bytes except a
// newline, which is what "." excludes and no ordinary path carries.
func compilePattern(from string) *regexp.Regexp {
	pattern := strings.ReplaceAll(regexp.QuoteMeta(from), `\*`, "(.*)")
	// Anchor both ends so a rule matches the whole path rather than any suffix
	// (see issue #4476). The group is belt and braces: quoting already rules out
	// the "|" that made "^/a|/b$" parse as "(^/a)|(/b$)".
	return regexp.MustCompile("^(?:" + pattern + ")$")
}

// orderedRules returns the rules to try, in the order to try them. RuleList is
// the author's own order, first match wins. The deprecated map has none, so its
// keys are ranked most-specific first and that order is documented.
func orderedRules(cfg Config) []Rule {
	if len(cfg.RuleList) > 0 {
		return cfg.RuleList
	}

	keys := make([]string, 0, len(cfg.Rules))
	for k := range cfg.Rules {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		// What each pins before its first wildcard, then in total: "/api/users"
		// outranks the "/api/*" that would otherwise answer for it.
		if d := cmp.Compare(pinnedPrefixLen(b), pinnedPrefixLen(a)); d != 0 {
			return d
		}
		if d := cmp.Compare(pinnedLen(b), pinnedLen(a)); d != 0 {
			return d
		}
		// A second wildcard usually widens the claim, so the rule carrying fewer
		// of them goes first. Two adjacent asterisks mean no more than one, and
		// there the count misreads which rule is narrower.
		if d := cmp.Compare(strings.Count(a, "*"), strings.Count(b, "*")); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})

	rules := make([]Rule, 0, len(keys))
	for _, k := range keys {
		rules = append(rules, Rule{From: k, To: cfg.Rules[k]})
	}
	return rules
}

// pinnedPrefixLen returns how many bytes of a path a rule pins before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
func pinnedPrefixLen(rule string) int {
	if i := strings.IndexByte(rule, '*'); i >= 0 {
		return i
	}
	return len(rule)
}

// pinnedLen returns how many bytes of a rule stand for themselves, which
// separates two rules whose wildcards start together: "/cdn/*" and "/cdn/*x".
func pinnedLen(rule string) int {
	return len(rule) - strings.Count(rule, "*")
}

// https://github.com/labstack/echo/blob/master/middleware/rewrite.go
func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	groups := pattern.FindStringSubmatch(input)
	if groups == nil {
		return nil
	}
	values := groups[1:]
	replace := make([]string, 2*len(values))
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	return strings.NewReplacer(replace...)
}
