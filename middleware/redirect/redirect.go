package redirect

import (
	"cmp"
	"maps"
	"math/bits"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
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
	target  string
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

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Fixed order, most specific first: two patterns can match the same path, and
	// a map range made the winner vary per run. Every key below must be a function
	// of one rule alone, or the comparison is no ordering and cycles appear.
	keys := slices.Collect(maps.Keys(cfg.Rules))
	slices.SortFunc(keys, func(a, b string) int {
		if d := cmp.Compare(literalPrefixLen(b), literalPrefixLen(a)); d != 0 {
			return d
		}
		if d := cmp.Compare(literalLen(b), literalLen(a)); d != 0 {
			return d
		}
		// A wildcard, a "+" or a "{2,}" matches a run of any length, so a rule holding
		// one is broader than any rule whose every position is bounded: "/api/*" must
		// not shadow the "/api/[ab]" it ties with. Ranked here since a width saturates.
		if d := cmp.Compare(carriesRun(a), carriesRun(b)); d != 0 {
			return d
		}
		// Two rules pinning the same amount are separated by how much else they match:
		// an alternation matches every branch, so "/very/specific|/x" is wider than the
		// exact "/x" it ties with.
		if d := cmp.Compare(patternWidth(a), patternWidth(b)); d != 0 {
			return d
		}
		// The widest position a rule matches, which is what the width loses where it
		// saturates or leaves a run unmeasured. Read after the width, which already
		// separates most pairs.
		if d := cmp.Compare(widestAtom(a), widestAtom(b)); d != 0 {
			return d
		}
		// Whatever the width leaves tied is separated by how many wildcards the rules
		// spend it on: "/p/*a*b" and "/p/*ab" both expand to a single alternative, and
		// key order alone put the broader two-wildcard rule first.
		if d := cmp.Compare(wildcardRank(a), wildcardRank(b)); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})

	for _, k := range keys {
		// Read the target as the client will: a tab the parser deletes was scored
		// as author-written host text, so "https://\t$1" passed as naming its own
		// host while a captured "/evil.com" composed "https:///evil.com".
		v := urlnorm.AsBrowserReads(cfg.Rules[k])
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
			pattern:         regexp.MustCompile(pattern),
			target:          v,
			authorityChunks: chunks,
			authorityEnders: authorityEnders(v),
			opaquePath:      spanStart == spanEnd && schemeEnd(v) > 0,
			sameOrigin:      !targetNamesAuthority(v),
		})
	}

	// Middleware function
	return func(c fiber.Ctx) error {
		// Next request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Rewrite
		for _, rule := range cfg.rulesRegex {
			replacer := captureTokens(rule.pattern, c.Path())
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
			// value writing the "//" that opens one — "myapp:$1@example.com" against
			// "//evil.com/x" — hands the destination to the request.
			if rule.opaquePath {
				if start, end := authoritySpan(location); start != end {
					continue
				}
			}
			if rule.sameOrigin {
				location = keepSameOrigin(location)
			}
			location = withRequestQuery(location, utils.UnsafeString(c.RequestCtx().QueryArgs().QueryString()))
			return c.Redirect().Status(cfg.StatusCode).To(location)
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
	if len(input) > 1 {
		input = utils.TrimRight(input, '/')
	}
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil
	}
	values := groups[0][1:]
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
		if i > 0 && strings.HasSuffix(chunks[i-1].text, ":") && !chunks[i-1].placeholder {
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
// accepts spellings net.ParseIP does not — "127.1", "0x7f.1" and "2130706433"
// all name 127.0.0.1 — and judging by net.ParseIP dropped rules pinning a host.
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

// patternBytes are the bytes of a rule key that match something other than
// themselves: "*" is the documented wildcard and the rest is regexp syntax,
// "." and the anchors among them, since none of those pins a byte of a path.
const patternBytes = `*.[](){}+?^|\$`

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard. An alternation pins only what its least specific branch does, and
// the count is of path characters rather than rule bytes: "\x{61}" is one "a".
func literalPrefixLen(rule string) int {
	return minOverBranches(rule, func(branch string) int {
		s := literalScanner{rule: branch}
		return s.pinnedPrefix()
	})
}

// literalLen returns how much of a rule's path is pinned in total, separating
// two rules whose wildcards start together, as "/cdn/*" and "/cdn/*x" do. A
// group counts for its least specific branch, as a top-level alternation does.
func literalLen(rule string) int {
	s := literalScanner{rule: rule}
	return s.widestBranch()
}

// literalScanner walks a rule once, so that measuring a group can be a recursive
// call that leaves the position just past the ")" it consumed.
type literalScanner struct {
	rule string
	i    int
	// widestAtom is the breadth of the widest single construct the rule matches,
	// recorded by width as it reads each one. See widestAtom.
	widest int
	// folded is whether case folding is in force at the position width has
	// reached, an "(?i)" having turned it on. See foldedWidth.
	folded bool
	// groupEmpty is what the group width just finished reading matches: true
	// where that is the empty string and nothing else, however deeply the
	// group's own groups nest.
	groupEmpty bool
}

// widestBranch counts the bytes pinned from the current position to the end of
// the rule or to the ")" closing the group beginning there, taking the smallest
// count over the branches it passed: a rule pins only what its widest path does.
func (s *literalScanner) widestBranch() int {
	smallest, n := -1, 0
	inClass := false
	// atom is what the construct just read pins, which a quantifier that allows
	// none of it takes back: "/api/a?" and "/api/a{0,1}" pin "/api/" alone.
	atom := 0
	for s.i < len(s.rule) {
		c := s.rule[s.i]
		switch {
		case c == '\\':
			if q := scanQuoted(s.rule, s.i); q.ok && !inClass {
				// Quoted text pins every byte it spells, a "?" among them.
				s.i = q.end
				atom = 0
				if q.text != "" {
					n, atom = n+len(q.text), 1
				}
				continue
			}
			size, pins := escapeSpan(s.rule, s.i)
			s.i += size
			atom = 0
			if pins && !inClass {
				n, atom = n+1, 1
			}
			continue
		case inClass:
			if end := posixNameEnd(s.rule, s.i); end > s.i {
				// The "]" of a POSIX name closes the name, not the class.
				s.i = end
				continue
			}
			// A class matches one byte whatever it lists, so its members pin
			// nothing: "/api/[a-z]" is no more specific than "/api/[ab]", and
			// counting them put the broader rule ahead of the narrower one.
			inClass = c != ']'
		case c == '[':
			inClass, atom = true, 0
		case c == '(':
			if flags := groupFlagScope(s.rule, s.i); flags.folds {
				// Folding says what the text it covers matches, so that text spells no path:
				// counting the "a" of "/p/(?i)a" put it ahead of the "/p/[a]" it contains.
				// Only folding does; an "m" or an "s" moves no literal.
				s.i = groupSpan(s.rule, s.i).end
				atom = 0
				if !flags.scoped {
					// Set for the rest of the rule, which pins nothing either.
					return smallerBranch(smallest, n)
				}
				continue
			}
			s.i = skipGroupPrefix(s.rule, s.i+1)
			atom = s.widestBranch()
			n += atom
			continue
		case c == ')':
			s.i++
			return smallerBranch(smallest, n)
		case c == '|':
			smallest, n, atom = smallerBranch(smallest, n), 0, 0
		case c == '?':
			// Zero or one, so what it follows pins nothing at all. "*" is not
			// read this way: a rule's "*" is Fiber's wildcard, already replaced
			// by "(.*)" before the key is compiled.
			n, atom = n-atom, 0
		case c == '{':
			// The bounds are syntax, and counting their digits and comma put
			// "/api/a{0,1}" three bytes ahead of the exact "/api/a" it shadowed.
			var bounds quantifierBounds
			s.i, bounds = skipQuantifier(s.rule, s.i)
			if bounds.allowsNone {
				n -= atom
			}
			atom = 0
			continue
		case strings.IndexByte(patternBytes, c) < 0:
			n, atom = n+1, 1
		default:
			atom = 0
		}
		s.i++
	}
	return smallerBranch(smallest, n)
}

// pinnedPrefix counts what the rule pins before its first wildcard, stopping at
// the first construct that pins nothing — a class, a wildcard, a class escape,
// or an optional atom. A group stops it too, save for what its branches share.
func (s *literalScanner) pinnedPrefix() int {
	return len(s.pinnedAtoms(0))
}

// maxPinnedDepth bounds how many groups deep the prefix is followed. Each level
// scans to the ")" closing it, so the work rises with the square of the nesting:
// sixteen thousand groups spent 2.4 seconds of startup. Stopping short is safe.
const maxPinnedDepth = 32

// pinnedAtoms returns what the rule pins before its first wildcard, one entry
// per character of the path rather than per byte of the rule: "\x{61}" is six
// bytes standing for the one byte "a", and the two have to compare alike.
func (s *literalScanner) pinnedAtoms(depth int) []string {
	var pinned []string
	for s.i < len(s.rule) {
		c := s.rule[s.i]
		if c == '(' {
			// A group pins what every branch of it pins and stops the prefix there: the
			// branches of "(?:a|aa)" agree on an "a", which stopping at the "(" missed.
			folded := groupFlagScope(s.rule, s.i).folds
			if depth >= maxPinnedDepth {
				return pinned
			}
			g := groupSpan(s.rule, s.i)
			s.i = g.end
			if none := absentAtom(s.rule, g.end); none.ok {
				// Counted none times, the group matches nothing and the rest of
				// the rule pins what it did.
				s.i = none.end
				continue
			}
			switch {
			case folded:
				// Folding says what the text inside matches, which is no longer
				// the path it spells: "(?i:a)" matches an "A" too, and pinning
				// its "a" put it level with the "/p/a" it contains.
				return pinned
			case quantifierAllowsNone(s.rule, g.end):
				// The whole group may be absent, so it pins nothing at all:
				// "/api/(ab)?c" matches "/api/c".
				return pinned
			}
			return append(pinned, commonPinnedAtoms(g.text, depth+1)...)
		}

		if c == '^' || c == '$' {
			// An anchor asserts a position and consumes nothing, so it pins no byte and
			// ends nothing: New anchors every rule already, and stopping at the "^" of
			// "^/p/a" left it pinning none of the path it spells.
			s.i++
			continue
		}

		size, atom := 1, ""
		switch {
		case c == '\\':
			if q := scanQuoted(s.rule, s.i); q.ok {
				// Quoted text pins every byte it spells: reading the span as an
				// escape that pins nothing stopped "/p/\Qab\E" at three, behind
				// the "/p/a.+" containing it.
				text := q.text
				s.i = q.end
				if text != "" {
					// A quantifier after the "\E" reaches the last byte quoted
					// and no more, so a count of none takes that byte away and
					// leaves the rest of the rule pinning what it did.
					if none := absentAtom(s.rule, q.end); none.ok {
						pinned = append(pinned, byteAtoms(text[:len(text)-1])...)
						s.i = none.end
						continue
					}
					if quantifierAllowsNone(s.rule, q.end) {
						return append(pinned, byteAtoms(text[:len(text)-1])...)
					}
				}
				pinned = append(pinned, byteAtoms(text)...)
				continue
			}
			if escapeAsserts(s.rule, s.i) {
				// An assertion consumes nothing, so it pins nothing and ends
				// nothing either: "\A/p/a" pins what "/p/a" does.
				span, _ := escapeSpan(s.rule, s.i)
				s.i += span
				continue
			}
			pins := false
			if size, pins = escapeSpan(s.rule, s.i); !pins {
				return pinned
			}
			atom = pinnedAtom(s.rule, s.i, size)
		case c == '[':
			// A class pins nothing whatever it lists, so the prefix ends here —
			// unless a count of none takes it away, leaving what follows pinned.
			class := literalScanner{rule: s.rule, i: s.i}
			class.classWidth() // leaves class.i just past the "]"
			none := absentAtom(s.rule, class.i)
			if !none.ok {
				return pinned
			}
			s.i = none.end
			continue
		case strings.IndexByte(patternBytes, c) >= 0:
			return pinned
		default:
			atom = s.rule[s.i : s.i+1]
		}

		after := s.i + size
		if none := absentAtom(s.rule, after); none.ok {
			// A count of none takes the atom away and leaves the rest of the
			// rule pinning what it did: "/p/a{0}a" matches "/p/a" alone, and
			// stopping here read it as pinning no more than "/p/".
			s.i = none.end
			continue
		}
		if quantifierAllowsNone(s.rule, after) {
			// An atom that may be absent is pinned by no path: "/p/a?b" matches
			// "/p/b", so the prefix ends before the "a".
			return pinned
		}
		s.i += size
		pinned = append(pinned, atom)
	}
	return pinned
}

// byteAtoms names each byte of text as a pinned character of its own.
func byteAtoms(text string) []string {
	atoms := make([]string, 0, len(text))
	for i := range len(text) {
		atoms = append(atoms, text[i:i+1])
	}
	return atoms
}

// pinnedAtom names the character an escape spells.
func pinnedAtom(rule string, i, size int) string {
	if b, ok := escapeByte(rule, i, size); ok {
		return string([]byte{b})
	}
	return rule[i : i+size]
}

// commonPinnedAtoms returns what every branch of a group's body pins alike,
// which is all of it the rule can be said to pin: a branch pinning something
// else is a path the rule matches without it.
func commonPinnedAtoms(body string, depth int) []string {
	var common []string
	for n, branch := range splitAlternation(body) {
		s := literalScanner{rule: branch}
		if atoms := s.pinnedAtoms(depth); n == 0 {
			common = atoms
		} else {
			common = commonAtoms(common, atoms)
		}
	}
	return common
}

// commonAtoms returns the leading atoms two branches agree on.
func commonAtoms(a, b []string) []string {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:min(len(a), len(b))]
}

// groupBody is what a group holds and where it ends.
type groupBody struct {
	text string
	end  int
}

// groupSpan returns the body of the group opening at i and the index just past
// its ")", or the rest of the rule where none closes it.
func groupSpan(rule string, i int) groupBody {
	start := skipGroupPrefix(rule, i+1)
	depth := 1
	for j := start; j < len(rule); {
		switch rule[j] {
		case '\\':
			if q := scanQuoted(rule, j); q.ok {
				j = q.end
				continue
			}
			size, _ := escapeSpan(rule, j)
			j += size
			continue
		case '[':
			s := literalScanner{rule: rule, i: j}
			s.classWidth() // leaves s.i just past the "]"
			j = s.i
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return groupBody{text: rule[start:j], end: j + 1}
			}
		}
		j++
	}
	return groupBody{text: rule[start:], end: len(rule)}
}

// patternWidth returns how many alternatives a rule expands to, which separates
// two that pin the same amount: "/p/[a-z](x|y)" matches everything "/p/[a-z]x"
// does and one path more, so it must not sort ahead of it.
func patternWidth(rule string) int {
	s := literalScanner{rule: rule}
	return s.width()
}

// widestAtom returns the breadth of the widest single construct a rule matches:
// a class, a group, a set escape, or any byte for Fiber's "*". It separates the
// rules a saturated or run-blind width leaves tied, and is read of every rule.
func widestAtom(rule string) int {
	s := literalScanner{rule: rule}
	s.width()
	return s.widest
}

// width is patternWidth from the current position to the end of the rule or to
// the ")" closing the group beginning there: the alternatives of a sequence
// multiply and those of an alternation add. A run is left to carriesRun.
func (s *literalScanner) width() int {
	total, n := 0, 1
	// atom is what the construct just read multiplied n by and prev what the run
	// measured before it, so a quantifier can put the atom back. quantified marks a
	// quantifier, whose trailing "?" is Go's non-greedy marker rather than another.
	atom, prev, quantified := 1, 1, false
	// empty marks the construct just read as one matching only the empty string,
	// whose every count spells the same path. filled is whether anything read here
	// matches more, which is what the group closing this call reports to its caller.
	empty, filled := true, false
	commit := func() {
		filled = filled || !empty
	}
	for s.i < len(s.rule) {
		c := s.rule[s.i]
		switch c {
		case '\\':
			commit()
			if q := scanQuoted(s.rule, s.i); q.ok {
				// Quoted text spells one path and no alternatives — unless
				// folding is in force, where each byte of it matches its own
				// case and the others: "(?i:\Qk\E)" matches three characters.
				s.i = q.end
				atom = 1
				for i := range len(q.text) {
					atom = scaledWidth(atom, s.foldedChar(q.text[i]))
				}
				prev, quantified, empty = n, false, q.text == ""
				s.widest = max(s.widest, atom)
				n = scaledWidth(n, atom)
				continue
			}
			// An escape naming a set matches what a class of the same members
			// does, and counting it as one character measured "/p/\w[ab]" as
			// half the "/p/[ab]{2}" it contains.
			size, pins := escapeSpan(s.rule, s.i)
			atom = 1
			switch {
			case escapeSet(s.rule, s.i):
				atom = s.foldedWidth(setMemberWidth)
			case pins:
				// The character it spells, which folding may match in another
				// case as well.
				if b, spells := escapeByte(s.rule, s.i, size); spells {
					atom = s.foldedChar(b)
				}
			}
			// An assertion consumes nothing, so it leaves what it stands beside
			// as empty as it found it: "(?:\b)+a" matches "/p/a" and no more.
			asserts := escapeAsserts(s.rule, s.i)
			s.i += size
			prev, quantified, empty = n, false, asserts
			if !asserts {
				s.widest = max(s.widest, atom)
				n = scaledWidth(n, atom)
			}
			continue
		case '[':
			commit()
			atom, prev, quantified, empty = s.foldedWidth(s.classWidth()), n, false, false
			s.widest = max(s.widest, atom)
			n = scaledWidth(n, atom)
			continue
		case '.':
			// Any byte, so it is the widest single position there is.
			commit()
			atom, prev, quantified, empty = 256, n, false, false
			s.widest = max(s.widest, atom)
			n = scaledWidth(n, atom)
		case '(':
			commit()
			// Flags reach the text they cover, and an "(?i)" closing at its own
			// ")" reaches everything standing after it as well.
			flags := groupFlagScope(s.rule, s.i)
			outer := s.folded
			s.folded = (s.folded || flags.folds) && !flags.clears
			s.i = skipGroupPrefix(s.rule, s.i+1)
			prev, quantified = n, false
			atom = s.width()
			if flags.scoped || !flags.sets {
				s.folded = outer
			}
			// A group holding nothing but empty groups is empty itself, which
			// reading only what it spells missed.
			empty = s.groupEmpty
			// The group's own width is no position of the rule but the product of the
			// positions inside it, each of which recorded itself as this call read them.
			n = scaledWidth(n, atom)
			continue
		case ')':
			commit()
			s.i++
			s.groupEmpty = !filled
			return clampWidth(total + n)
		case '|':
			commit()
			total, n, atom, prev, quantified, empty = clampWidth(total+n), 1, 1, 1, false, true
		case '^', '$':
			// An anchor asserts a position and consumes nothing.
			atom, prev, quantified, empty = 1, n, false, true
		case '?':
			// A "?" following a quantifier only says the repetition is not
			// greedy, and "/p/[b]+?" matches what "/p/[b]+" does: counting it
			// as an atom that may be absent widened the narrower of a pair.
			if !quantified && !empty {
				// An atom that may be absent matches everything it does and
				// one path more, so "/p/*a?" is wider than the "/p/*a" it ties
				// with — and wider than "/p/*[a]*", which the count sorts below.
				n, prev = repeated(prev, atom, 0, 1), n
			}
			atom, quantified = 1, true
		case '*':
			// Fiber's wildcard, expanded to "(.*)" before the key is compiled,
			// so it matches any byte. Measured no wider than the byte it is
			// written as, the run itself being ranked by carriesRun.
			commit()
			s.widest = max(s.widest, 256)
			atom, prev, quantified, empty = 1, n, false, false
		case '+':
			// The run it names is ranked by carriesRun rather than measured
			// here, but it is a quantifier still: the "?" that may follow is
			// Go's non-greedy marker.
			atom, prev, quantified = 1, n, true
		case '{':
			var bounds quantifierBounds
			s.i, bounds = skipQuantifier(s.rule, s.i)
			if !bounds.quantifies {
				// Braces standing as text: the "{" is a byte like any other,
				// and what it holds is measured rather than passed over.
				commit()
				atom, prev, quantified, empty = 1, n, false, false
				continue
			}
			if !bounds.runsOn && !empty {
				n, prev = repeated(prev, atom, bounds.lo, bounds.hi), n
			}
			// A count of none takes the atom away, leaving what stands here as
			// empty as no atom at all.
			if !bounds.runsOn && bounds.hi == 0 {
				empty = true
			}
			atom, quantified = 1, true
			continue
		default:
			// One byte of the path, which is the narrowest position there is
			// unless case folding matches it in another case as well.
			commit()
			atom, prev, quantified, empty = s.foldedChar(c), n, false, false
			s.widest = max(s.widest, atom)
			n = scaledWidth(n, atom)
		}
		s.i++
	}
	commit()
	s.groupEmpty = !filled
	return clampWidth(total + n)
}

// foldedWidth doubles what a position matches where case folding is in force,
// each letter being matched in either case. A floor rather than a count, since
// how many of the bytes a class lists are letters is not read here.
func (s *literalScanner) foldedWidth(w int) int {
	if !s.folded {
		return w
	}
	return min(scaledWidth(w, 2), 256)
}

// foldedChar counts what one character matches where folding is in force, which
// is knowable exactly: Go folds a "k" to "K" and to the Kelvin sign as well, so
// three, where an "a" matches two and a "1" one.
func (s *literalScanner) foldedChar(b byte) int {
	if !s.folded || b >= utf8.RuneSelf {
		return 1
	}

	r, n := rune(b), 0
	for f := r; ; {
		n++
		if f = unicode.SimpleFold(f); f == r {
			return n
		}
	}
}

// repeated returns the width a run reaches once the atom ending it, itself atom
// alternatives wide, may repeat lo to hi times: every permitted count is a set
// of paths of its own, and they add. prev is carried in rather than divided out,
// a product that reached the clamp being past undoing.
func repeated(prev, atom, lo, hi int) int {
	w := max(atom, 1)
	total, term := 0, 1 // w to the zero, the one path where the atom is absent
	for k := 0; k <= hi && total < maxPatternWidth; k++ {
		if k >= lo {
			total = clampWidth(total + term)
		}
		term = scaledWidth(term, w)
	}
	return scaledWidth(prev, max(total, 1))
}

// classWidth returns how many bytes the character class at the current position
// matches, leaving the position just past its "]". A class pins nothing whatever
// it lists, so this is the only place its breadth is read.
func (s *literalScanner) classWidth() int {
	rule := s.rule
	j := s.i + 1
	negated := false
	if j < len(rule) && rule[j] == '^' {
		negated, j = true, j+1
	}

	var members classMembers
	if j < len(rule) && rule[j] == ']' {
		// First inside the brackets, "]" is a member rather than the close.
		members.addByte(']')
		j++
	}

	for j < len(rule) && rule[j] != ']' {
		if end := posixNameEnd(rule, j); end > j {
			// A POSIX name stands for a set of its own, counted like a class
			// escape. Its own "]" is not the class's, and reading one as the
			// close left the rest of the class scanned as pattern text.
			members.addSet(rule[j:end])
			j = end
			continue
		}

		lo := readClassMember(rule, j)
		if !lo.spellsByte {
			members.addNamed(rule[j:lo.end], lo.width)
			j = lo.end
			continue
		}

		// A "-" between two members names every byte from one to the other, and
		// either endpoint may be spelled as an escape: Go reads "[\x{61}-\x{7a}]"
		// as "[a-z]", where reading the endpoints as written counted the braces.
		if lo.end+1 < len(rule) && rule[lo.end] == '-' && rule[lo.end+1] != ']' {
			if hi := readClassMember(rule, lo.end+1); hi.spellsByte {
				if hi.b >= lo.b {
					members.addRange(lo.b, hi.b)
				}
				j = hi.end
				continue
			}
		}
		members.addByte(lo.b)
		j = lo.end
	}
	if j < len(rule) {
		j++ // past the "]"
	}
	s.i = j

	size := members.width()
	if negated {
		size = 256 - size
	}
	return max(size, 1)
}

// classMember is one member of a character class: the byte it spells, where it
// ends, and what it is worth where no byte answers for it — a set escape, or a
// character wider than the bytes a class is counted in.
type classMember struct {
	end        int
	width      int
	b          byte
	spellsByte bool
}

// readClassMember reads the member beginning at i, which is one byte unless a
// backslash opens it.
func readClassMember(rule string, i int) classMember {
	if rule[i] != '\\' {
		return classMember{b: rule[i], end: i + 1, width: 1, spellsByte: true}
	}

	span, pins := escapeSpan(rule, i)
	if !pins {
		// A class escape stands for a set of its own — unless it spells one
		// byte, as "[\.]" does, which is a member like any other: counting that
		// as a set put it behind the "[.-/]" range containing it.
		w := 1
		if escapeSet(rule, i) {
			w = setMemberWidth
		}
		return classMember{end: i + span, width: w}
	}
	if b, ok := escapeByte(rule, i, span); ok {
		return classMember{b: b, end: i + span, width: 1, spellsByte: true}
	}
	return classMember{end: i + span, width: 1}
}

// escapeByte returns the byte an escape spelling one character stands for. A
// "\x{3B1}" names a character of more than one byte, which no member of a class
// counted in bytes is, so it answers no.
func escapeByte(rule string, i, span int) (byte, bool) {
	if i+1 >= len(rule) {
		return 0, false // a trailing backslash spells nothing
	}

	switch c := rule[i+1]; {
	case c == 'a':
		return 0x07, true
	case c == 'f':
		return 0x0C, true
	case c == 'n':
		return '\n', true
	case c == 'r':
		return '\r', true
	case c == 't':
		return '\t', true
	case c == 'v':
		return 0x0B, true
	case c == 'x':
		return parsedByte(strings.Trim(rule[i+2:i+span], "{}"), 16)
	case c >= '0' && c <= '7':
		return parsedByte(rule[i+1:i+span], 8)
	default:
		return rule[i+span-1], true // "\." and the like: the byte it escapes
	}
}

// parsedByte reads digits naming one byte, answering no where they name more.
func parsedByte(digits string, base int) (byte, bool) {
	n, err := strconv.ParseUint(digits, base, 8)
	if err != nil {
		return 0, false
	}
	return byte(n), true
}

// classMembers accumulates what a character class matches: the bytes it lists,
// held one bit each, and the sets it names, held by spelling so that repeating
// one counts once. Two spellings of one set still count twice: see setMemberWidth.
type classMembers struct {
	named []namedMember
	bytes [4]uint64
}

// namedMember is a member no byte value answers for: a set spelled by name, or a
// character of more than one byte. Held by its spelling so repeating it counts
// once, and carrying what it is worth — a set is setMemberWidth, a character 1.
type namedMember struct {
	spelling string
	width    int
}

func (m *classMembers) addByte(b byte) {
	m.bytes[b>>6] |= 1 << (b & 63)
}

func (m *classMembers) addRange(lo, hi byte) {
	for b := lo; ; b++ {
		m.addByte(b)
		if b == hi {
			return
		}
	}
}

func (m *classMembers) addSet(spelling string) {
	m.addNamed(spelling, setMemberWidth)
}

func (m *classMembers) addNamed(spelling string, w int) {
	for _, named := range m.named {
		if named.spelling == spelling {
			return
		}
	}
	m.named = append(m.named, namedMember{spelling: spelling, width: w})
}

func (m *classMembers) width() int {
	n := 0
	for _, named := range m.named {
		n += named.width
	}
	for _, w := range m.bytes {
		n += bits.OnesCount64(w)
	}
	return n
}

// maxPatternWidth bounds the product a nest of groups builds, since the count is
// only ever compared against another and nothing needs the exact figure.
const maxPatternWidth = 1 << 20

// maxRepeatCount is the largest count Go's regexp parser takes in a "{m,n}",
// above which it refuses the pattern outright: regexp/syntax spells it 1000.
const maxRepeatCount = 1000

// setMemberWidth is what a class member standing for a set of its own counts,
// written as an escape, "[\d]", or as a POSIX name, "[[:alpha:]]". Its true size
// is unknowable here, and two says the one certain thing: a set beats one byte.
const setMemberWidth = 2

func clampWidth(n int) int {
	return min(n, maxPatternWidth)
}

// scaledWidth multiplies one width by another, saturating rather than wrapping:
// the product of two clamped widths does not fit a thirty-two bit int, and one
// that wrapped came back negative and sorted a rule ahead of the subset it holds.
func scaledWidth(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > maxPatternWidth/b {
		return maxPatternWidth
	}
	return clampWidth(a * b)
}

// carriesRun grades how open-ended a rule's broadest run is: 0 where every
// position is bounded, 1 where a run repeats what the rule names beside it, and
// 2 for Fiber's "*", which repeats anything. A saturating width cannot say this.
func carriesRun(rule string) int {
	r := scanRuns(rule)
	switch {
	case r.wildcards > 0:
		return 2
	case r.unbounded:
		return 1
	}
	return 0
}

// wildcardRank returns the number of Fiber "*" wildcards in a rule, the last
// thing separating two that the width leaves tied: "/p/*a*b" and "/p/*ab" both
// expand to a single alternative.
func wildcardRank(rule string) int {
	return scanRuns(rule).wildcards
}

// ruleRuns is what a rule matches without bound: the Fiber "*" wildcards it
// carries, and whether a quantifier lets some atom repeat without end.
type ruleRuns struct {
	wildcards int
	unbounded bool
}

// scanRuns reads both in one walk, since telling either from the rule means
// skipping the same classes and quoted spans, where a star names itself: the
// expansion leaves "[(.*)]" a class and "\Q(.*)\E" literal text.
func scanRuns(rule string) ruleRuns {
	runs := ruleRuns{}
	// filled holds, for each group still open, whether anything matching more than
	// the empty string has been read in it; empty marks the construct just read as
	// one matching only the empty string, whose repetitions are all the same path.
	var filled []bool
	empty := true
	commit := func() {
		if !empty && len(filled) > 0 {
			filled[len(filled)-1] = true
		}
	}
	for i := 0; i < len(rule); {
		switch rule[i] {
		case '\\':
			commit()
			if q := scanQuoted(rule, i); q.ok {
				// Only the quoted text is given up: the wildcards standing
				// before it are still expanded, so the count already reached is
				// what holds.
				i, empty = q.end, q.text == ""
				break
			}
			// An assertion consumes nothing, and leaves what stands beside it as
			// empty as it found it.
			empty = escapeAsserts(rule, i)
			size, _ := escapeSpan(rule, i)
			i += size
		case '[':
			commit()
			s := literalScanner{rule: rule, i: i}
			s.classWidth() // leaves s.i just past the "]"
			i, empty = s.i, false
		case '(':
			commit()
			i = skipGroupPrefix(rule, i+1)
			filled = append(filled, false)
			empty = true
			continue
		case ')':
			// A group holding nothing but empty groups is empty itself, which
			// reading only what it spells missed: "(?:(?:))" matches the empty
			// string alone, and its "+" was graded a run all the same.
			commit()
			empty = len(filled) == 0 || !filled[len(filled)-1]
			if len(filled) > 0 {
				filled = filled[:len(filled)-1]
			}
			i++
			continue
		case '^', '$':
			// An anchor asserts a position and consumes nothing.
			i++
			empty = true
			continue
		case '|':
			// A branch begins here with nothing read in it yet, and the separator matches
			// nothing: reading it as a construct filled a group its branches left empty.
			commit()
			i++
			empty = true
			continue
		case '*':
			// Fiber's own wildcard, which is expanded to "(.*)" whatever stands
			// before it, so it runs on even where that is an empty group.
			commit()
			runs.wildcards++
			i++
			empty = false
		case '?':
			// Quantifier syntax, whether it repeats what stands before it or only says a
			// repetition is not greedy. Either way it matches nothing of its own, and
			// reading it as a construct lost the emptiness of what it followed.
			i++
			continue
		case '+':
			runs.unbounded = runs.unbounded || !empty
			i++
			continue
		case '{':
			var bounds quantifierBounds
			i, bounds = skipQuantifier(rule, i)
			if !bounds.quantifies {
				commit()
				empty = false
				break
			}
			switch {
			case bounds.runsOn:
				runs.unbounded = runs.unbounded || !empty
			case bounds.hi == 0:
				// A count of none takes the atom away.
				empty = true
			}
			continue
		default:
			commit()
			i++
			empty = false
		}
	}
	return runs
}

// quantifierAllowsNone reports whether a quantifier at i lets what precedes it
// match nothing, which is what stops "/api/ab?" from pinning the "b".
func quantifierAllowsNone(rule string, i int) bool {
	if i >= len(rule) {
		return false
	}
	if rule[i] == '?' {
		return true
	}
	if rule[i] != '{' {
		return false
	}

	_, bounds := skipQuantifier(rule, i)
	return bounds.allowsNone
}

// absentQuantifier is a quantifier that takes what precedes it away outright,
// and where it ends.
type absentQuantifier struct {
	end int
	ok  bool
}

// absentAtom reads the quantifier at i, answering yes only where it permits no
// count at all. A "?" or a "{0,2}" merely permits none, which ends a prefix
// rather than skipping a byte of it, since a path may arrive without the atom.
func absentAtom(rule string, i int) absentQuantifier {
	if i >= len(rule) || rule[i] != '{' {
		return absentQuantifier{end: i}
	}

	end, bounds := skipQuantifier(rule, i)
	if !bounds.quantifies || bounds.runsOn || bounds.hi != 0 {
		return absentQuantifier{end: i}
	}
	return absentQuantifier{end: end, ok: true}
}

// groupFlags is what a group's prefix says about match flags. sets marks a group
// naming any; scoped marks one closing them at its own ")", where "(?i)" leaves
// them set for everything after it.
type groupFlags struct {
	sets   bool
	scoped bool
	folds  bool
	clears bool
}

// groupFlagScope reads the prefix of the group opening at i. Flags say what the
// text they cover matches, so that text spells no path the rule pins: "(?i:a)"
// matches an "A" as well, and "(?i)a" leaves the "a" after it matching one too.
func groupFlagScope(rule string, i int) groupFlags {
	if i+1 >= len(rule) || rule[i+1] != '?' {
		return groupFlags{}
	}

	folds, cleared, clearing := false, false, false
	for j := i + 2; j < len(rule); j++ {
		switch c := rule[j]; c {
		case ':':
			return groupFlags{sets: j > i+2, scoped: true, folds: folds, clears: cleared && !folds}
		case ')':
			return groupFlags{sets: j > i+2, folds: folds, clears: cleared && !folds}
		case '-':
			cleared = false
			clearing = true
		case 'i':
			// Named before the "-" it is set, and after it cleared: "(?i-s:)"
			// folds case where "(?-i:)" stops folding it.
			if clearing {
				cleared = true
			} else {
				folds = true
			}
		case 'm', 's', 'U':
			continue
		default:
			return groupFlags{} // a capture name, or no group prefix at all
		}
	}
	return groupFlags{}
}

// smallerBranch folds one more branch's count into the smallest seen, where -1
// stands for no branch counted yet.
func smallerBranch(smallest, n int) int {
	if smallest < 0 || n < smallest {
		return n
	}
	return smallest
}

// quantifierBounds is how many times a "{m,n}" lets the atom before it repeat.
// runsOn marks the "{2,}" naming no upper bound; allowsNone is the min of zero
// read alone; quantifies separates a real bound from braces standing as text.
type quantifierBounds struct {
	lo, hi     int
	allowsNone bool
	runsOn     bool
	quantifies bool
}

// skipQuantifier returns the index just past the "{m,n}" beginning at i and what
// it bounds. An unclosed "{", or one whose body spells no bound, is an ordinary
// byte to the regexp parser: the index moves past it alone, so it is walked.
func skipQuantifier(rule string, i int) (int, quantifierBounds) {
	brace := strings.IndexByte(rule[i:], '}')
	if brace < 0 {
		return i + 1, quantifierBounds{lo: 1, hi: 1}
	}

	bounds := quantifierRange(rule[i+1 : i+brace])
	if !bounds.quantifies {
		return i + 1, bounds
	}
	return i + brace + 1, bounds
}

// quantifierRange reads the body of a "{m,n}". A body spelling no bound is no
// quantifier to Go's parser either — the braces of "{id}" are literal text — so
// it is read as bounding nothing.
func quantifierRange(body string) quantifierBounds {
	once := quantifierBounds{lo: 1, hi: 1}

	lo, hi, comma := strings.Cut(body, ",")
	m, ok := repeatCount(lo)
	if !ok {
		return once
	}

	switch {
	case !comma:
		return quantifierBounds{lo: m, hi: m, allowsNone: m == 0, quantifies: true}
	case hi == "":
		return quantifierBounds{lo: m, allowsNone: m == 0, runsOn: true, quantifies: true}
	}

	n, ok := repeatCount(hi)
	if !ok || n < m {
		return once
	}
	return quantifierBounds{lo: m, hi: n, allowsNone: m == 0, quantifies: true}
}

// repeatCount reads one bound of a "{m,n}". Go spells a bound in decimal digits
// and nothing else, so a sign makes the braces literal text: "a{-0}" is a path
// of six bytes, not an "a" repeated none.
func repeatCount(bound string) (int, bool) {
	if bound == "" {
		return 0, false
	}
	for i := 0; i < len(bound); i++ {
		if bound[i] < '0' || bound[i] > '9' {
			return 0, false
		}
	}

	// Go's parser refuses a count above maxRepeatCount, so such a rule never
	// compiles. Capped here too, since the sort measures a rule before compiling it
	// and walking to a bound of a billion spent seconds of startup.
	n, err := strconv.Atoi(bound)
	if err != nil || n > maxRepeatCount {
		return 0, false
	}
	return n, true
}

// posixNameEnd returns the index just past the POSIX class name beginning at i
// inside a character class — "[[:alpha:]]" names the letters — or i when nothing
// there names one. Its own "]" is not the class's, and stopping there misread it.
func posixNameEnd(rule string, i int) int {
	if i+1 >= len(rule) || rule[i] != '[' || rule[i+1] != ':' {
		return i
	}
	if end := strings.Index(rule[i+2:], ":]"); end >= 0 {
		return i + 2 + end + 2
	}
	return i
}

// skipGroupPrefix returns the index of a group's body, past the "?" section of a
// non-capturing, flagged or named group — "(?:", "(?i:", "(?P<id>" — whose bytes
// are syntax rather than path.
func skipGroupPrefix(rule string, i int) int {
	if i >= len(rule) || rule[i] != '?' {
		return i
	}
	for j := i + 1; j < len(rule); j++ {
		switch rule[j] {
		case ':', '>':
			return j + 1
		case ')':
			// A flag-only group, "(?i)". Leave the ")" for the caller to close on.
			return j
		}
	}
	return i
}

// quotedSpan is the text a "\Q" quotes and the index just past the span. Go
// reads an unterminated "\Q" as quoting to the end of the rule, which is where
// the span runs: such a rule is measured before it fails to compile.
type quotedSpan struct {
	text string
	end  int
	ok   bool
}

// scanQuoted reads the "\Q...\E" beginning at the backslash at i. What it quotes
// is path rather than pattern: a "?" or a "{2}" inside it is a byte the rule
// pins, and reading one as a quantifier widened an exact rule.
func scanQuoted(rule string, i int) quotedSpan {
	if i+1 >= len(rule) || rule[i+1] != 'Q' {
		return quotedSpan{end: i}
	}
	if n := strings.Index(rule[i+2:], `\E`); n >= 0 {
		return quotedSpan{text: rule[i+2 : i+2+n], end: i + 2 + n + 2, ok: true}
	}
	return quotedSpan{text: rule[i+2:], end: len(rule), ok: true}
}

// escapeSet reports whether the escape at the backslash at i stands for a set of
// characters rather than for one or for none: "\d" and "\p{Greek}" do, where
// "\." spells a byte and "\b" matches nothing at all.
func escapeSet(rule string, i int) bool {
	if i+1 >= len(rule) {
		return false
	}
	switch rule[i+1] {
	case 'd', 'D', 's', 'S', 'w', 'W', 'p', 'P':
		return true
	}
	return false
}

// escapeAsserts reports whether the escape at the backslash at i asserts a
// position rather than matching anything: "\b" stands between a word byte and
// one that is not, so a group holding it is as empty as a group holding nothing.
func escapeAsserts(rule string, i int) bool {
	if i+1 >= len(rule) {
		return false
	}
	switch rule[i+1] {
	case 'b', 'B', 'A', 'z':
		return true
	}
	return false
}

// escapeSpan measures the escape beginning at the backslash at i: how many bytes
// of the rule it occupies, and whether it stands for one character of a path —
// punctuation and "\x{61}" do, where "\d" or "\b" names a class or an assertion.
func escapeSpan(rule string, i int) (int, bool) {
	if i+1 >= len(rule) {
		return 1, false // a trailing backslash names nothing
	}

	switch c := rule[i+1]; {
	case c == 'a', c == 'f', c == 'n', c == 'r', c == 't', c == 'v':
		return 2, true // a control character written by name
	case c == 'x':
		return hexEscapeSpan(rule, i)
	case c >= '0' && c <= '7':
		// Octal, up to three digits: "\101" is "A".
		n := 2
		for n < 4 && i+n < len(rule) && rule[i+n] >= '0' && rule[i+n] <= '7' {
			n++
		}
		return n, true
	case c == 'p', c == 'P':
		return propertySpan(rule, i)
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		// "\d", "\w", "\b" — a class or an assertion. Measured as two bytes
		// even where it runs longer, which only ends the prefix sooner.
		return 2, false
	default:
		return 2, true
	}
}

// propertySpan measures a "\p{Greek}" or a "\pL", neither of which names one
// character: a Unicode property is a set of them. The name is part of the escape,
// and leaving it to be read as rule text pinned "Greek" as path.
func propertySpan(rule string, i int) (int, bool) {
	if i+2 >= len(rule) {
		return 2, false // "\p" alone names nothing
	}
	if rule[i+2] == '{' {
		if end := strings.IndexByte(rule[i+2:], '}'); end >= 0 {
			return 2 + end + 1, false
		}
		return 2, false
	}
	return 3, false // "\pL", one letter naming the property
}

// hexEscapeSpan measures a "\xHH" or "\x{...}" escape, both of which name one
// character. An incomplete one is no literal, and the regexp would not compile.
func hexEscapeSpan(rule string, i int) (int, bool) {
	if i+2 < len(rule) && rule[i+2] == '{' {
		if end := strings.IndexByte(rule[i+2:], '}'); end >= 0 {
			return 2 + end + 1, true
		}
		return 2, false
	}
	if i+3 < len(rule) && isHexDigit(rule[i+2]) && isHexDigit(rule[i+3]) {
		return 4, true
	}
	return 2, false
}

func isHexDigit(c byte) bool {
	_, ok := unhex(c)
	return ok
}

// minOverBranches applies measure to each top-level alternation branch of rule
// and returns the smallest result, since a rule is only as specific as the least
// specific path it can match. Without a "|" that is just measure(rule).
func minOverBranches(rule string, measure func(string) int) int {
	smallest := -1
	for _, branch := range splitAlternation(rule) {
		if n := measure(branch); smallest < 0 || n < smallest {
			smallest = n
		}
	}
	return smallest
}

// splitAlternation splits rule on the "|" bytes that separate its top-level
// branches: unescaped, and outside any group or character class, where a "|" is
// an ordinary member rather than a separator.
func splitAlternation(rule string) []string {
	var branches []string
	depth, start := 0, 0
	inClass := false
	for i := 0; i < len(rule); i++ {
		switch c := rule[i]; {
		case c == '\\':
			if q := scanQuoted(rule, i); q.ok {
				// A "|" inside quoted text separates nothing: it is a byte of
				// the path, and splitting there measured "/p/\Qa|b\E" as the
				// least of two branches that are not there.
				i = q.end - 1
				continue
			}
			i++
		case inClass:
			inClass = c != ']'
		case c == '[':
			inClass = true
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == '|' && depth == 0:
			branches = append(branches, rule[start:i])
			start = i + 1
		}
	}
	return append(branches, rule[start:])
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
