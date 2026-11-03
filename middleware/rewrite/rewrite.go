package rewrite

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/gofiber/fiber/v3/internal/rulematch"
)

type compiledRule struct {
	pattern *regexp.Regexp
	to      string
	// dotSegments counts the dot segments the author wrote into to.
	dotSegments int
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Initialize
	rules := orderedRules(cfg)
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		compiled = append(compiled, compiledRule{
			pattern:     rulematch.MustCompile(rule.From, rulematch.Literal),
			to:          rule.To,
			dotSegments: rulematch.DotSegments(rule.To),
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
			replacer := rulematch.CaptureTokens(rule.pattern, c.Path(), appconfig.Of(c.App()).UnescapePath)
			if replacer == nil {
				continue
			}
			path := replacer.Replace(rule.to)
			// c.Path resolves dot segments, so one spelled by a capture leaves To's prefix.
			if rulematch.DotSegments(path) > rule.dotSegments {
				continue
			}
			c.Path(path)
			break
		}
		return c.Next()
	}
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
