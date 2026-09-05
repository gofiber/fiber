package redirect

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/urlnorm"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// compiledRule is one configured rule with its target and the decision, made
// once at construction, of whether that target picks its own destination.
type compiledRule struct {
	pattern *regexp.Regexp
	// from is the rule as the author wrote it, kept for the warnings only.
	from   string
	target string
	// opaquePath is set when the target names a scheme but no authority of its
	// own. A capture may write the "//" that opens one, and it need only supply
	// half of it, so the composed location is what gets checked.
	opaquePath bool
	// sameOrigin is set when the target names no authority of its own. The "$N"
	// values spliced into such a target come from the request path, so they must
	// not be able to introduce one.
	sameOrigin bool
}

// patternBytes are the bytes of a rule key that match something other than
// themselves: "*" is the documented wildcard and the rest is regexp syntax,
// since a key reaches the compiled pattern as one.
const patternBytes = `*.[](){}+?^|\$`

// orderedRules returns the rules to try, in the order to try them. RuleList is
// the author's own order, first match wins. The deprecated map has none, so its
// keys are sorted most specific first by the heuristic below.
func orderedRules(cfg Config) []Rule {
	if len(cfg.RuleList) > 0 {
		return cfg.RuleList
	}

	keys := slices.Collect(maps.Keys(cfg.Rules))
	// Most specific first, by what a key pins rather than by what its pattern
	// means: a map hands them over in a random order, so something has to decide,
	// and reading the regexp to measure its breadth cost more than it settled.
	// Every key is a function of one rule alone, and the terminal comparison is
	// on distinct map keys, so the order is total.
	slices.SortFunc(keys, func(a, b string) int {
		// What each pins before its first wildcard, then in total: "/api/users"
		// outranks the "/api/*" that would otherwise answer for it.
		if d := cmp.Compare(pinnedPrefixLen(b), pinnedPrefixLen(a)); d != 0 {
			return d
		}
		if d := cmp.Compare(pinnedLen(b), pinnedLen(a)); d != 0 {
			return d
		}
		// A second wildcard matches everything the first does and more, so the
		// rule carrying fewer of them is the narrower claim: "/p/*ab" must answer
		// the path "/p/*a*b" would otherwise take.
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
	if i := strings.IndexAny(rule, patternBytes); i >= 0 {
		return i
	}
	return len(rule)
}

// pinnedLen returns how many bytes of a rule stand for themselves, which
// separates two rules whose wildcards start together: "/cdn/*" and "/cdn/*x".
func pinnedLen(rule string) int {
	n := 0
	for i := 0; i < len(rule); i++ {
		if strings.IndexByte(patternBytes, rule[i]) < 0 {
			n++
		}
	}
	return n
}

// shadowProbes are the values a wildcard is filled with to ask what a rule
// matches. Only a probe the rule itself matches is used, so a rule is never
// reported against a path it does not answer.
var shadowProbes = []string{"a", "0", "x/y", ""}

// warnShadowedRules warns about a rule an earlier one answers for outright.
// Order is the author's to choose, so this reports the dead rule rather than
// quietly moving it: a rule list generated from config is often emitted in
// alphabetical order, which puts "/api/*" in front of the "/api/users" it eats.
func warnShadowedRules(rules []compiledRule) {
	shadowedRules(rules, func(shadowed, by string) {
		log.Warnf("[REDIRECT] rule %q never fires: the earlier rule %q matches every path it does. "+
			"Rules are tried in order, so move the narrower rule first", shadowed, by)
	})
}

// shadowedRules hands each rule an earlier one answers for to report, and says
// whether it found any. Quadratic in the rule count, so it is bounded: a list
// long enough to cost real time is not one anybody is reading warnings from.
func shadowedRules(rules []compiledRule, report func(shadowed, by string)) bool {
	if len(rules) > maxShadowChecked {
		return false
	}

	found := false
	for i, rule := range rules {
		witnesses := ruleWitnesses(rule.from, rule.pattern)
		if len(witnesses) == 0 {
			continue
		}
		for j := range i {
			if !matchesAll(rules[j].pattern, witnesses) {
				continue
			}
			if report == nil {
				return true
			}
			report(rule.from, rules[j].from)
			found = true
			break
		}
	}
	return found
}

// maxShadowChecked bounds the pairs the shadow scan walks, since it compares
// every rule against every earlier one and runs at startup.
const maxShadowChecked = 100

// ruleWitnesses returns sample paths the rule matches, built by filling its
// wildcards with probe values. A probe the rule turns down is dropped, so what
// comes back is only ever paths this rule answers.
func ruleWitnesses(from string, pattern *regexp.Regexp) []string {
	witnesses := make([]string, 0, len(shadowProbes))
	for _, probe := range shadowProbes {
		candidate := strings.ReplaceAll(from, "*", probe)
		if candidate != "" && pattern.MatchString(candidate) {
			witnesses = append(witnesses, candidate)
		}
	}
	return witnesses
}

// matchesAll reports whether one pattern answers every witness of another rule,
// which is what makes that rule dead behind this one.
func matchesAll(pattern *regexp.Regexp, witnesses []string) bool {
	for _, witness := range witnesses {
		if !pattern.MatchString(witness) {
			return false
		}
	}
	return true
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	for _, rule := range orderedRules(cfg) {
		k := rule.From
		// Read the target as the client will: a tab the parser deletes was scored
		// as author-written host text, so "https://\t$1" passed as naming its own
		// host while a captured "/evil.com" composed "https:///evil.com".
		v := urlnorm.AsBrowserReads(rule.To)
		pattern := strings.ReplaceAll(k, "*", "(.*)")
		// Anchor both ends so a rule matches the whole path rather than any suffix
		// (see issue #4476). Grouped first because concatenation binds looser than
		// "|", and non-capturing so the "$N" tokens still number the author's groups.
		pattern = "^(?:" + pattern + ")$"

		spanStart, spanEnd := authoritySpan(v)
		if captureInAuthority(v, spanStart, spanEnd) {
			// The host is the author's or the rule does not compile. Whether a
			// value is safe inside an authority depends on percent-decoding,
			// IDNA mapping, IPv4 numeric labels, IPv6 brackets and userinfo, and
			// every one of those was a way to move the host. Not compiled at all,
			// so "never fires" is structural rather than a flag a later edit
			// could forget to test.
			log.Warnf("[REDIRECT] rule %q is ignored: its target captures inside the redirect authority, so "+
				"the request would have a say in where the client lands. That covers the host, the port and "+
				"any userinfo. Write the authority in full and capture only the path, query or fragment", k)
			continue
		}

		cfg.rulesRegex = append(cfg.rulesRegex, compiledRule{
			from:       k,
			pattern:    regexp.MustCompile(pattern),
			target:     v,
			opaquePath: spanStart == spanEnd && schemeEnd(v) > 0,
			sameOrigin: !targetNamesAuthority(v),
		})
	}

	warnShadowedRules(cfg.rulesRegex)

	// Bound before the closure so it captures these alone: capturing cfg kept the
	// caller's rule map reachable for the life of the app, long after New read it.
	compiled, statusCode, next := cfg.rulesRegex, cfg.StatusCode, cfg.Next

	// Middleware function
	return func(c fiber.Ctx) error {
		// Next request to skip middleware
		if next != nil && next(c) {
			return c.Next()
		}
		// Read once rather than per rule: under Immutable a Path() is a fresh copy,
		// and the trailing slashes come off the same way for every rule.
		path := c.Path()
		if len(path) > 1 {
			path = utils.TrimRight(path, '/')
		}
		// Rewrite
		for _, rule := range compiled {
			replacer := captureTokens(rule.pattern, path)
			if replacer == nil {
				continue
			}

			// Normalize on every branch: the bytes removed are never meaningful
			// and the client drops them anyway, so the guard below always runs
			// on the location as it will be read.
			location := urlnorm.AsBrowserReads(replacer.Replace(rule.target))
			// The target had a scheme and an opaque path, so it named no host at all. A
			// value writing the "//" that opens one hands the destination to the
			// request: "myapp:$1@example.com" against "//evil.com/x".
			if rule.opaquePath {
				if start, end := authoritySpan(location); start != end {
					continue
				}
			}
			if rule.sameOrigin {
				location = keepSameOrigin(location)
			}
			location = withRequestQuery(location, utils.UnsafeString(c.RequestCtx().QueryArgs().QueryString()))
			return c.Redirect().Status(statusCode).To(location)
		}
		return c.Next()
	}
}

// withRequestQuery carries the request's query onto the composed location.
// Appending "?" gave a target with its own query a second one, folding the
// request's into the last value; with a fragment it landed after the "#".
func withRequestQuery(location, query string) string {
	if query == "" {
		return location
	}

	fragment := ""
	if i := strings.IndexByte(location, '#'); i >= 0 {
		location, fragment = location[:i], location[i:]
	}

	separator := "?"
	if strings.IndexByte(location, '?') >= 0 {
		separator = "&"
	}
	return location + separator + query + fragment
}

// schemeEnd returns the index of the ':' that ends an RFC 3986 scheme at the
// start of s, or -1 when s does not begin with one.
func schemeEnd(s string) int {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// A letter is valid anywhere in an RFC 3986 scheme.
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				return -1 // a scheme has to start with a letter
			}
		case c == ':':
			if i == 0 {
				return -1
			}
			return i
		default:
			return -1
		}
	}
	return -1
}

// targetNamesAuthority reports whether a configured target picks its own
// destination. Where it does, leaving this origin is what the author asked for
// and the composed location is left alone.
func targetNamesAuthority(target string) bool {
	return strings.HasPrefix(target, "//") || schemeEnd(target) > 0
}

// authoritySpan returns the byte range of target's own authority, or 0, 0 when
// target names none: from the "//" to the first "/", "?", "#" or "\". A special
// scheme needs no "//": "https:host" and "https://host" name the same host.
func authoritySpan(target string) (start, end int) { //nolint:nonamedreturns // the pair is a range; names say which is which
	special := true
	switch {
	case strings.HasPrefix(target, "//"):
		// Protocol-relative, so the scheme is the page's and every scheme that
		// can be one is special.
		start = 2
	default:
		i := schemeEnd(target)
		special = isSpecialScheme(target[:max(i, 0)])
		switch {
		case i <= 0:
			return 0, 0
		case isFileScheme(target):
			// The authority opens after any two slashes, both of which fold a backslash
			// in: "file:/\evil.com/share" and "file:\\evil.com/share" name the host
			// "file://evil.com/share" does.
			if i+2 >= len(target) || !isSlash(target[i+1]) || !isSlash(target[i+2]) {
				// Fewer than two, so there is no authority: "file:tmp/x" is the path "/tmp/x"
				// of an empty host, and dropping the rule told the author something untrue.
				// A value writing the slashes itself is caught on the composed location.
				return 0, 0
			}
			start = i + 3
		case strings.HasPrefix(target[i+1:], "//"):
			start = i + 3
		case special:
			start = i + 1
		default:
			return 0, 0
		}
	}

	// Only a special scheme ignores the slashes past the first two, so under any
	// other one the third terminates an empty authority. "file" is special but
	// skips that step, leaving "file:///$1" the empty authority of a local path.
	if special && !isFileScheme(target) {
		for start < len(target) && isSlash(target[start]) {
			start++
		}
	}

	if offset := strings.IndexAny(target[start:], authorityEnders(target)); offset >= 0 {
		return start, start + offset
	}
	return start, len(target)
}

// authorityEnders returns the bytes that end target's authority. WHATWG folds
// "\" into "/" only under a special scheme; under any other it is an ordinary
// authority byte. A scheme-less target inherits the page's, which is special.
func authorityEnders(target string) string {
	if i := schemeEnd(target); i > 0 && !isSpecialScheme(target[:i]) {
		return `/?#`
	}
	return `/\?#`
}

// specialSchemes are the schemes the WHATWG URL Standard calls special. The
// parser reaches the authority state for these with or without the "//" that
// every other scheme needs, and it is the only list where that is true.
var specialSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
	"ws":    {},
	"wss":   {},
	"ftp":   {},
	"file":  {},
}

// isSlash reports whether c separates URL components. A backslash does under a
// special scheme, which the parser folds into a slash before reading it.
func isSlash(c byte) bool {
	return c == '/' || c == '\\'
}

// isFileScheme reports whether target names the "file" scheme, which is special
// yet reaches its host through "file slash state" rather than the ignore-slashes
// one every other special scheme takes.
func isFileScheme(target string) bool {
	i := schemeEnd(target)
	return i > 0 && strings.EqualFold(target[:i], "file")
}

// isSpecialScheme reports whether scheme is one of them. A scheme is
// case-insensitive (RFC 3986 Section 3.1), so "HTTPS:host" reaches the host the
// same way "https:host" does.
func isSpecialScheme(scheme string) bool {
	_, ok := specialSchemes[strings.ToLower(scheme)]
	return ok
}

// captureInAuthority reports whether a target's authority holds a "$N" token.
// The span is the caller's, which already read it to compile the rule.
func captureInAuthority(target string, start, end int) bool {
	for i := start; i < end; i++ {
		if placeholderEnd(target[i:]) > 0 {
			return true
		}
	}
	return false
}

// placeholderEnd returns the index just past a "$N" token at the start of s, or
// -1 when s does not begin with one. All the digits are taken, which is the
// widest reading of the token and so the least text credited to the author.
func placeholderEnd(s string) int {
	if len(s) < 2 || s[0] != '$' || s[1] < '0' || s[1] > '9' {
		return -1
	}
	i := 1
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// keepSameOrigin strips an authority the captured path segments introduced into
// a target that named none: "/$1" turned "/api//evil.com" into "//evil.com".
// Expects a location already through urlnorm.AsBrowserReads.
func keepSameOrigin(location string) string {
	if schemeEnd(location) > 0 {
		// Root it so what the capture supplied is read as a path on this
		// origin rather than as a destination of its own.
		return "/" + location
	}

	n := 0
	for n < len(location) && (location[n] == '/' || location[n] == '\\') {
		n++
	}
	if n == 0 || (n == 1 && location[0] == '/') {
		return location
	}
	// Collapse to the single slash the target asked for. Backslashes count as
	// slashes: the parser folds them in this position.
	return "/" + location[n:]
}

// https://github.com/labstack/echo/blob/master/middleware/rewrite.go
func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	// One match or none: the pattern is anchored at both ends, so a match spans
	// the whole input and there is never a second one to look for.
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
