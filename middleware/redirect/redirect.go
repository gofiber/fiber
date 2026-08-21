package redirect

import (
	"cmp"
	"maps"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/urlnorm"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	"golang.org/x/net/idna"
)

// authorityChunk is one piece of a target's own authority: either literal text
// the author wrote or a "$N" token the request fills in.
type authorityChunk struct {
	text        string // literal text, or the token itself for a placeholder
	placeholder bool
	// pins records whether this literal is author-written host text. Answered
	// once here because pinsHost maps the text the way a URL parser does, which
	// is too much work to repeat for every chunk on every request.
	pins bool
}

// compiledRule is one configured rule with its target and the decision, made
// once at construction, of whether that target picks its own destination.
type compiledRule struct {
	pattern *regexp.Regexp
	// from is the rule as the author wrote it, kept for the warnings only.
	from   string
	target string
	// authorityEnders are the bytes a value may open the next component with,
	// which is scheme-dependent: see authorityEnders.
	authorityEnders string
	// authorityChunks splits the target's authority into literal text and "$N"
	// tokens, so each value can be judged by where it lands. Empty when the
	// authority holds no placeholder and so cannot be moved by a request.
	authorityChunks []authorityChunk
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
		chunks := authorityChunks(v)

		letsRequestPickHost := targetLetsRequestPickHost(v, chunks)

		switch {
		case captureInBrackets(v):
			// Refused like the rest, but not for their reason: the author may have
			// pinned the network, as "https://[2001:db8::$1]" does, and only
			// "https://[$1::1]" hands it over.
			log.Warnf("[REDIRECT] rule %q is ignored: its target captures inside the brackets of an IPv6 "+
				"literal, which is written most significant group first — so whether the capture picks the "+
				"network or only a group within it depends on where it sits, and Fiber does not tell the two "+
				"apart. Write the address in full and capture the port instead", k)
		case letsRequestPickHost:
			// An open redirect by construction: nothing here can tell the
			// intended destination from an attacker's. The rule never fires, and
			// this says why rather than leaving it silently dead.
			log.Warnf("[REDIRECT] rule %q is ignored: its target takes the redirect host from the request path, "+
				"so anyone who can shape the path would choose where the client lands. Pin the host in the target", k)
		case authorityEndsInOpenCapture(chunks):
			// The host ends in a capture with nothing pinned after it, so ".evil.com"
			// extends it into a domain the author does not control. Refused per
			// request, which would otherwise read as the rule quietly not firing.
			log.Warnf("[REDIRECT] rule %q ends in a capture inside its host, so only a value that cannot "+
				"extend that host is honored — one opening a path, query or fragment. Pin what follows the "+
				"capture in the target to redirect on every value", k)
		}

		if letsRequestPickHost {
			// Not compiled at all, so "never fires" is structural rather than a
			// flag a later edit could forget to test — and the request path is
			// not matched against a pattern whose result is thrown away.
			continue
		}

		spanStart, spanEnd := authoritySpan(v)
		cfg.rulesRegex = append(cfg.rulesRegex, compiledRule{
			from:            k,
			pattern:         regexp.MustCompile(pattern),
			target:          v,
			authorityChunks: chunks,
			authorityEnders: authorityEnders(v),
			opaquePath:      spanStart == spanEnd && schemeEnd(v) > 0,
			sameOrigin:      !targetNamesAuthority(v),
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

			// In "https://$1.cdn.example.com" the author means $1 as a label,
			// not a whole URL. Refuse the value that would move the host rather
			// than guess; the request falls through to the app.
			if !authorityHolds(rule.authorityChunks, replacer, rule.authorityEnders) {
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
// scheme needs no "//" — "https:host" and "https://host" name the same host.
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

// authorityChunks splits target's own authority into literal text and "$N"
// tokens. It returns nil when the authority holds no token, which is the common
// case and means no request can move the host.
func authorityChunks(target string) []authorityChunk {
	start, end := authoritySpan(target)
	authority := target[start:end]

	var chunks []authorityChunk
	literal := 0
	for i := 0; i < len(authority); {
		if authority[i] != '$' {
			i++
			continue
		}
		j := i + 1
		for j < len(authority) && authority[j] >= '0' && authority[j] <= '9' {
			j++
		}
		if j == i+1 { // a bare '$' is literal text
			i++
			continue
		}
		if i > literal {
			text := authority[literal:i]
			// A literal only has a capture on its left once one has been
			// appended; the authority's first chunk opens the host itself.
			chunks = append(chunks, authorityChunk{text: text, pins: pinsHost(text, len(chunks) > 0)})
		}
		chunks = append(chunks, authorityChunk{text: authority[i:j], placeholder: true})
		literal = j
		i = j
	}
	if chunks == nil {
		return nil
	}
	if literal < len(authority) {
		text := authority[literal:]
		chunks = append(chunks, authorityChunk{text: text, pins: pinsHost(text, true)})
	}
	return chunks
}

// authorityHolds reports whether the values leave the target's authority naming
// the author's host: a token with more authority after it must stay one label,
// one ending the authority must open the next component, be empty, or be a port.
func authorityHolds(chunks []authorityChunk, replacer *strings.Replacer, enders string) bool {
	for i, chunk := range chunks {
		if !chunk.placeholder {
			continue
		}
		// Through the Replacer, not by indexing: it matches patterns in the order
		// given, so "$10" is "$1" then a literal "0". Tabs, CRs and LFs are stripped
		// because the parser deletes them, so "\t/ok" ends the authority at the "/".
		value := urlnorm.StripTabCRLF(replacer.Replace(chunk.text))

		if hostPins(chunks[i+1:]) {
			// The author closed the host past this token, so the value is a label inside
			// it and only has to stay one. What ends the authority is scheme-dependent: a
			// backslash does so only under a special scheme.
			if strings.ContainsAny(value, enders) {
				return false
			}
			// Outside userinfo the "@" and the ":" matter too: either would move the host
			// or open a port the author did not write. Inside it the author wrote the "@"
			// that closes it, and a ":" only separates a password.
			if !userinfoCloses(chunks[i+1:]) && strings.ContainsAny(value, "@:") {
				return false
			}
			continue
		}

		// Nothing closes the host after this token, so a value that does not
		// open the next component extends it into a registrable domain:
		// "evil.com" composes "example.comevil.com".
		switch {
		case value == "":
		case strings.IndexByte(enders, value[0]) >= 0 && hostPins(chunks[:i]):
			// Opens the next component, so the authority ended at the author's host —
			// but only where they wrote one: "//$1." composing "///evil.com." ends no
			// authority, since the parser skips the slashes.
		case opensPort(chunks[:i]) && isAllDigits(value):
			// A port. The author wrote the colon, and the URL parser rejects a
			// port holding anything but digits outright, so digits are the only
			// value that can be what they meant.
		default:
			return false
		}
	}
	return true
}

// hostPins reports whether the author wrote host text among these chunks, asked
// of one side of a capture or the other. A port closes nothing, which is why
// "https://$1:8080" still leaves $1 free to name the host outright.
func hostPins(chunks []authorityChunk) bool {
	for _, chunk := range chunks {
		if !chunk.placeholder && chunk.pins {
			return true
		}
	}
	return false
}

// userinfoCloses reports whether the author wrote an "@" among these chunks,
// asked of what follows a capture. The host begins after the last one, so a
// capture before it is userinfo and names no label.
func userinfoCloses(chunks []authorityChunk) bool {
	for _, chunk := range chunks {
		if !chunk.placeholder && strings.ContainsRune(chunk.text, '@') {
			return true
		}
	}
	return false
}

// targetLetsRequestPickHost reports whether the request, not the author, decides
// the host a target reaches — "https://$1", and equally "https://$1:8080", since
// a port pins no host. The static half of what authorityHolds asks per value.
func targetLetsRequestPickHost(target string, chunks []authorityChunk) bool {
	// A non-nil chunk list always holds a capture — authorityChunks returns nil
	// unless it appended one — so the only question left is whether the author
	// wrote any host text of their own alongside it.
	if chunks != nil {
		if captureInBrackets(target) {
			return true
		}
		// Anywhere in the authority: the host is theirs, and authorityHolds
		// judges the captures around it per value.
		return !hostPins(chunks)
	}

	// No authority span, so a value reaches a host only by opening one from right
	// after the scheme's colon — "mailto:$1" and "myapp:$1" both did. Author text
	// in between settles it; so does host text after, as "mailto:$1@example.com".
	i := schemeEnd(target)
	if i <= 0 || strings.HasPrefix(target[i+1:], "//") {
		return false
	}
	rest := target[i+1:]
	j := placeholderEnd(rest)
	if j < 0 {
		return false
	}
	return !pinsHost(rest[j:], true)
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

// captureInBrackets reports whether a "$N" token falls inside an IPv6 literal.
// A DNS name is written least significant label first; a bracketed address
// reverses that, so refuse the shape rather than carry an opposite rule.
func captureInBrackets(target string) bool {
	start, end := authoritySpan(target)
	authority := target[start:end]

	// Everything through the last "@" is userinfo, where a bracket is an ordinary
	// character rather than a host delimiter. Only an "@" outside the brackets
	// ends it: one inside is no delimiter to the parser either.
	depth, userinfoEnd := 0, -1
	for i := 0; i < len(authority); i++ {
		switch authority[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '@':
			if depth == 0 {
				userinfoEnd = i
			}
		}
	}
	if userinfoEnd >= 0 {
		authority = authority[userinfoEnd+1:]
	}
	if !strings.HasPrefix(authority, "[") {
		return false
	}

	depth = 0
	for i := 0; i < len(authority); i++ {
		switch authority[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '$':
			if depth > 0 && i+1 < len(authority) && authority[i+1] >= '0' && authority[i+1] <= '9' {
				return true
			}
		}
	}
	return false
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

// isAllDigits reports whether s is a non-empty run of ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// authorityEndsInOpenCapture reports whether a target's host is closed by a
// capture rather than by the author, so a value can extend it into a domain
// they do not control. The static half of what authorityHolds decides per value.
func authorityEndsInOpenCapture(chunks []authorityChunk) bool {
	for i, chunk := range chunks {
		if !chunk.placeholder || hostPins(chunks[i+1:]) {
			continue
		}
		if opensPort(chunks[:i]) {
			continue // a port position
		}
		return true
	}
	return false
}

// isIPv4Number reports whether a host label is read as a number by the IPv4
// address parser, which is what makes the whole host an address rather than a
// name. Decimal, and the hex the parser also accepts.
func isIPv4Number(label string) bool {
	if label == "" {
		return false
	}
	if len(label) >= 2 && label[0] == '0' && (label[1] == 'x' || label[1] == 'X') {
		for i := 2; i < len(label); i++ {
			if _, ok := unhex(label[i]); !ok {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(label); i++ {
		if label[i] < '0' || label[i] > '9' {
			return false
		}
	}
	return true
}

// isIPv4Host reports whether the URL parser reads host as an IPv4 address. It
// accepts spellings net.ParseIP does not: "127.1", "0x7f.1" and "2130706433"
// all name 127.0.0.1, and judging by net.ParseIP dropped rules pinning a host.
func isIPv4Host(host string) bool {
	parts := strings.Split(host, ".")
	// One trailing dot is allowed and names no part: "127.0.0.1." is an address.
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 4 {
		return false
	}

	last := len(parts) - 1
	for i, part := range parts {
		n, ok := ipv4Number(part)
		if !ok {
			return false
		}
		// Every part but the last is one octet. The last takes whatever octets
		// were not written, which is what makes "127.1" and "2130706433" work.
		limit := uint64(256)
		if i == last {
			limit = uint64(1) << (8 * (4 - last))
		}
		if n >= limit {
			return false
		}
	}
	return true
}

// ipv4Number parses one dotted part the way the URL parser's IPv4 number parser
// does: "0x" for hex, a bare leading "0" for octal, decimal otherwise. Reports
// the value and whether the part was a number at all.
func ipv4Number(part string) (uint64, bool) {
	base := 10
	switch {
	case len(part) >= 2 && part[0] == '0' && (part[1] == 'x' || part[1] == 'X'):
		part, base = part[2:], 16
	case len(part) >= 2 && part[0] == '0':
		part, base = part[1:], 8
	}
	if part == "" {
		// What "0" and "0x" are left as, both of which are zero.
		return 0, true
	}

	n, err := strconv.ParseUint(part, base, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// percentDecode decodes the valid "%XX" escapes in s, as a URL parser does
// before reading a host: "https://$1%2E" otherwise pinned a host on three
// characters decoding to a bare ".". A stray "%" is left as the parser leaves it.
func percentDecode(s string) string {
	i := strings.IndexByte(s, '%')
	if i < 0 {
		return s
	}

	decoded := make([]byte, 0, len(s))
	decoded = append(decoded, s[:i]...)
	for ; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) {
			decoded = append(decoded, s[i])
			continue
		}
		hi, hiOK := unhex(s[i+1])
		lo, loOK := unhex(s[i+2])
		if !hiOK || !loOK {
			decoded = append(decoded, s[i])
			continue
		}
		decoded = append(decoded, hi<<4|lo)
		i += 2
	}
	return string(decoded)
}

// unhex returns the value of a hex digit, and whether it was one.
func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// hostMapping applies the same UTS #46 mapping a URL parser applies to a domain
// before reading it. The validation options are off because the question here
// is only what text survives, not whether the whole authority is well formed.
var hostMapping = idna.New(
	idna.MapForLookup(),
	idna.Transitional(false),
	idna.StrictDomainName(false),
	idna.ValidateLabels(false),
	idna.VerifyDNSLength(false),
)

// pinsHost reports whether a literal chunk of a target's authority actually
// fixes part of the host: only what follows the last "@" and precedes the port
// colon counts. openLeft says where the literal sits; only the caller knows.
//
//nolint:revive // flag-parameter: openLeft is a position, not a mode
func pinsHost(literal string, openLeft bool) bool {
	if i := strings.LastIndexByte(literal, '@'); i >= 0 {
		literal = literal[i+1:]
		// The "@" is where the host starts, so whatever a capture put before it
		// is userinfo and cannot reach into the label that follows.
		openLeft = false
	}
	// An IPv6 literal carries colons of its own, so the port colon is the one
	// past the closing bracket rather than the first.
	if i := strings.IndexByte(literal, ']'); i >= 0 {
		// A closing bracket with no opener means a capture split the address and
		// this is its tail — the low bits, not the network. "::1]" in
		// "https://[$1::1]" leaves the routing prefix to the request.
		j := strings.IndexByte(literal[:i], '[')
		if j < 0 {
			return false
		}
		// With an opener the author wrote an address — but only a real one, so ask
		// whether it parses: "[zzz1]" is hex digits, not an address. Brackets hold
		// IPv6 only, and the colon separates the spellings ParseIP takes both of.
		inner := literal[j+1 : i]
		return strings.IndexByte(inner, ':') >= 0 && net.ParseIP(inner) != nil
	}
	if strings.IndexByte(literal, '[') >= 0 {
		// The other half: leading groups of a split address pin no host on their
		// own. captureInBrackets refuses such targets outright; this only keeps
		// the two answers consistent.
		return false
	}

	if i := strings.IndexByte(literal, ':'); i >= 0 {
		literal = literal[:i]
	}
	// Read what is left the way the parser does: UTS #46 deletes some 270 code
	// points, so an ASCII-only trim scored a soft hyphen as author text. Invalid
	// UTF-8 becomes U+FFFD with no error, so rule it out first.
	decoded := percentDecode(literal)
	if !utf8.ValidString(decoded) {
		return false
	}

	mapped, err := hostMapping.ToUnicode(decoded)
	if err != nil {
		return false
	}

	// Nor anything at or below a space, nor DEL: the parser strips a leading or
	// trailing run of them, so "https://$1\x01$2" shipped "https://evil.com". One
	// in the middle is a forbidden domain code point, so it pins nothing either.
	trimmed := strings.TrimFunc(mapped, func(r rune) bool {
		return r <= ' ' || r == 0x7f || strings.ContainsRune(".[:", r)
	})
	if trimmed == "" {
		return false
	}

	// A numeric last label is parsed as IPv4, the author's text being the low
	// octets: "https://$1.1" composed "https://127.0.0.1" from "127.0.0". Judged
	// before the trim — the leading "." is what closes the label.
	tail := strings.HasPrefix(mapped, ".")
	if tail {
		openLeft = false
	}
	if openLeft && strings.IndexByte(trimmed, '.') < 0 {
		return false
	}

	last := trimmed
	if i := strings.LastIndexByte(trimmed, '.'); i >= 0 {
		last = trimmed[i+1:]
	}
	if isIPv4Number(last) {
		// An address pins a host only where the author wrote the whole of it. Open on
		// the left the capture reaches its first octet, and a leading "." says one
		// already supplied the octets before this text, as "https://$1.1" does.
		return !openLeft && !tail && isIPv4Host(trimmed)
	}
	return true
}

// opensPort reports whether the nearest literal before a token ends in the colon
// that opens a port. Several captures may compose one — "example.com:$1$2" — so
// placeholders in between are stepped over and each asked for digits in turn.
func opensPort(before []authorityChunk) bool {
	for _, chunk := range slices.Backward(before) {
		if chunk.placeholder {
			continue
		}
		return strings.HasSuffix(chunk.text, ":")
	}
	return false
}
