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
	// a map range made the winner vary per run. Rank by what a rule pins before
	// its first wildcard, then by total pinned length, then by key to stay total.
	keys := slices.Collect(maps.Keys(cfg.Rules))
	slices.SortFunc(keys, func(a, b string) int {
		if d := cmp.Compare(literalPrefixLen(b), literalPrefixLen(a)); d != 0 {
			return d
		}
		if d := cmp.Compare(literalLen(b), literalLen(a)); d != 0 {
			return d
		}
		// A wildcard matches a run of any length, and so does a "+" or a
		// "{2,}", so a rule holding one is broader than any rule whose every
		// position is bounded, however much either pins: "/api/*" must not
		// shadow the "/api/[ab]" it ties with. Ranked on its own rather than
		// counted as a width, which saturates — two rules whose widths both
		// reached the clamp would tie again. Only whether a rule carries a run
		// is read here: a second wildcard says nothing about what the rest of
		// the rule pins, so counting them ahead of the width put the broad
		// "/p/[a-d]*" in front of the narrow "/p/([a]*|[c]*)".
		if d := cmp.Compare(carriesRun(a), carriesRun(b)); d != 0 {
			return d
		}
		// Two rules pinning the same amount are separated by how much else they
		// match: an alternation matches every branch, so "/very/specific|/x" is
		// wider than the exact "/x" it ties with. Two wildcard rules land here
		// too, separated by everything beside the wildcard they share.
		if d := cmp.Compare(patternWidth(a), patternWidth(b)); d != 0 {
			return d
		}
		// Whatever the width leaves tied is separated by how many wildcards the
		// rules spend it on, since a width stands for none of them: "/p/*a*b"
		// and "/p/*ab" both expand to a single alternative, and the key order
		// below put the broader two-wildcard rule first.
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
		// Anchor both ends so a rule matches the whole path. Without the leading
		// "^" the pattern matches any suffix, so a request can be redirected by a
		// rule whose path it only happens to end with (e.g. "/old" would also
		// redirect "/very/old"). See issue #4476.
		//
		// Grouped first, because concatenation binds looser than "|": "/a|/b"
		// anchored by hand is "(^/a)|(/b$)", which anchors neither branch at both
		// ends and redirected "/a-extra" and "extra/b". Non-capturing, so the
		// "$N" tokens still number the author's own groups.
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
			// The target had a scheme and an opaque path, so it named no host at
			// all. A value writing the "//" that opens one — "myapp:$1@example.com"
			// against "//evil.com/x" — hands the destination to the request, and
			// the "@example.com" that looked like it pinned the host is only path.
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
			// "file state" takes one slash to reach "file slash state" and one
			// more to reach the host, and both fold a backslash into a slash. So
			// the authority opens after any two of them: "file:/\evil.com/share"
			// and "file:\\evil.com/share" name the host evil.com exactly as
			// "file://evil.com/share" does, and reading only "//" let a captured
			// "\evil.com/share" pick the host of a rule targeting "file:/$1".
			if i+2 >= len(target) || !isSlash(target[i+1]) || !isSlash(target[i+2]) {
				// Fewer than two, so the parser leaves "file state" for "path
				// state" and there is no authority: "file:tmp/x" is the path
				// "/tmp/x" of an empty host, and dropping the rule told the author
				// something untrue about it. What a value can still do is write
				// the slashes itself, which the composed location is checked for.
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

	// Only a special scheme ignores the slashes past the first two: the parser
	// reaches "special authority ignore slashes" for those alone. Under any
	// other scheme the third one terminates an empty authority, so "myapp:///$1"
	// names no host and the capture is path — skipping it read $1 as the host
	// and the rule was dropped at startup for handing the host away.
	//
	// "file" is special but takes its own route through the parser, going from
	// "file slash state" straight to the host without that ignore step, so
	// "file:///$1" has the empty authority of a local path. It keeps the
	// backslash folding, which is why it stays in the special list.
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
// "\" into "/" only under a special scheme; under any other one it is an
// ordinary authority byte, so "myapp://example.com\@evil.com" is userinfo
// "example.com\" and a host of evil.com. A target with no scheme is
// protocol-relative, and the scheme it inherits is special.
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
		// Through the Replacer, not by indexing: it composes the location and
		// matches patterns in the order given, so "$10" is "$1" then a literal
		// "0". Indexing judged the tenth capture while the first was spliced in.
		// Read as the client will, since that is who acts on the composition: the
		// parser deletes tabs, CRs and LFs, so a value of "\t/ok" reaches "/ok"
		// and ends the authority there. Judging the tab instead refused a
		// redirect the client would have followed safely.
		value := urlnorm.StripTabCRLF(replacer.Replace(chunk.text))

		if hostPins(chunks[i+1:]) {
			// The author closed the host past this token, so the value is a
			// label inside it and only has to stay one. What ends the authority
			// is scheme-dependent — a backslash does so only under a special
			// scheme — so "myapp://$1@example.com" takes a value of "user\name"
			// as the userinfo it is, rather than refusing a redirect whose host
			// the author had pinned.
			if strings.ContainsAny(value, enders) {
				return false
			}
			// Outside userinfo the "@" and the ":" matter too: either would move
			// the host or open a port the author did not write. Inside it the
			// author wrote the "@" that closes it, and a ":" only separates a
			// password.
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
// accepts spellings net.ParseIP does not — "127.1" is 127.0.0.1, so is "0x7f.1",
// and "2130706433" is the same address written as one number — and judging them
// by net.ParseIP dropped rules whose host the author had pinned outright.
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
		// An address pins a host only where the author wrote the whole of it.
		// Open on the left the capture reaches its first octet: against
		// "%3110.0.0.1", a captured "0" composes "0%3110.0.0.1", octal 0127,
		// landing on 72.0.0.1. And a leading "." says a capture already supplied
		// the octets before this text, which is what "https://$1.1" does —
		// "127.0.0" composes "https://127.0.0.1". Either way the author pinned
		// only part of an address, which pins no host at all.
		return !openLeft && !tail && isIPv4Host(trimmed)
	}
	return true
}

// patternBytes are the bytes of a rule key that match something other than
// themselves. "*" is the documented wildcard; the rest are regexp syntax, which
// reaches the compiled pattern because a key is used as one. "." is among them:
// it matches any byte, so "/api/user." pins no more of a path than "/api/user"
// and must not outrank the exact "/api/users". So are "^" and "$": an anchor
// asserts a position and consumes nothing, and counting one as a path byte put
// the broader "/p/[a-z]$" ahead of "/p/[a]" on a path they both match.
const patternBytes = `*.[](){}+?^|\$`

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
//
// An alternation pins only what its least specific branch does: "/very/specific|/x"
// matches "/x", so it pins one byte rather than the fourteen standing before the
// "|", which had it outrank the exact "/x" and shadow it outright.
// Counted in what a path is pinned to rather than in bytes of the rule, since
// the two part company over escapes: "\x{61}" is six bytes standing for the one
// byte "a", and measuring the rule made it tie the class "[a-z]" it should
// outrank, and outrank the "/p/a" it merely equals.
func literalPrefixLen(rule string) int {
	return minOverBranches(rule, func(branch string) int {
		s := literalScanner{rule: branch}
		return s.pinnedPrefix()
	})
}

// literalLen returns how much of a rule's path is pinned in total, which
// separates two rules whose wildcards start together: "/cdn/*" and "/cdn/*x"
// tie on prefix, and the lexicographic fallback left the narrower one dead.
// A group is measured the same way a top-level alternation is, by its least
// specific branch: "/api/[a-z](specific|x)" pins no more of a path than
// "/api/[a-z]x" does, and crediting it with the letters of every alternative had
// it sort first and shadow the narrower rule on every request they share.
func literalLen(rule string) int {
	s := literalScanner{rule: rule}
	return s.widestBranch()
}

// literalScanner walks a rule once, so that measuring a group can be a recursive
// call that leaves the position just past the ")" it consumed.
type literalScanner struct {
	rule string
	i    int
}

// widestBranch counts the bytes pinned from the current position to the end of
// the rule, or to the ")" closing the group beginning there, and returns the
// smallest count over the alternation branches it passed — a rule pins only what
// the widest path it matches does.
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
// the first construct that pins nothing — a class, a group, a wildcard, a class
// escape, or an atom a quantifier makes optional.
func (s *literalScanner) pinnedPrefix() int {
	n := 0
	for s.i < len(s.rule) {
		size := 1
		if c := s.rule[s.i]; c == '\\' {
			pins := false
			if size, pins = escapeSpan(s.rule, s.i); !pins {
				return n
			}
		} else if strings.IndexByte(patternBytes, c) >= 0 {
			return n
		}
		if quantifierAllowsNone(s.rule, s.i+size) {
			return n
		}
		s.i += size
		n++
	}
	return n
}

// patternWidth returns how many alternatives a rule expands to, which separates
// two that pin the same amount: "/p/[a-z](x|y)" matches everything
// "/p/[a-z]x" does and one path more, so it must not sort ahead of it. Counting
// only top-level branches left the two tied, and key order put the wider first.
func patternWidth(rule string) int {
	s := literalScanner{rule: rule}
	return s.width()
}

// width is patternWidth from the current position to the end of the rule or to
// the ")" closing the group beginning there. The alternatives of a sequence
// multiply and those of an alternation add, so a group counts wherever it sits.
//
// A quantifier that runs on — "+", or "{2,}" — is no count of alternatives at
// all and is left unmeasured here, carriesRun having already sorted the rule
// behind every bounded one. The width goes on measuring the atom it repeats,
// which is what separates "/p/[z]+" from the "/p/[a-z]+" that contains it.
func (s *literalScanner) width() int {
	total, n := 0, 1
	// atom is what the construct just read multiplied n by, and prev is what the
	// run measured before it did — both kept so a quantifier can put the atom
	// back and count what it permits instead. quantified marks the construct
	// just read as a quantifier itself, since the "?" following one is Go's
	// non-greedy marker rather than a second quantifier of its own.
	atom, prev, quantified := 1, 1, false
	for s.i < len(s.rule) {
		switch s.rule[s.i] {
		case '\\':
			size, _ := escapeSpan(s.rule, s.i)
			s.i += size
			atom, prev, quantified = 1, n, false
			continue
		case '[':
			atom, prev, quantified = s.classWidth(), n, false
			n = scaledWidth(n, atom)
			continue
		case '.':
			// Any byte, so it is the widest single position there is.
			atom, prev, quantified = 256, n, false
			n = scaledWidth(n, atom)
		case '(':
			s.i = skipGroupPrefix(s.rule, s.i+1)
			prev, quantified = n, false
			atom = s.width()
			n = scaledWidth(n, atom)
			continue
		case ')':
			s.i++
			return clampWidth(total + n)
		case '|':
			total, n, atom, prev, quantified = clampWidth(total+n), 1, 1, 1, false
		case '?':
			// A "?" following a quantifier only says the repetition is not
			// greedy, and "/p/[b]+?" matches what "/p/[b]+" does: counting it
			// as an atom that may be absent widened the narrower of a pair.
			if !quantified {
				// An atom that may be absent matches everything it does and
				// one path more, so "/p/*a?" is wider than the "/p/*a" it ties
				// with — and wider than "/p/*[a]*", which the count sorts below.
				n, prev = repeated(prev, atom, 0, 1), n
			}
			atom, quantified = 1, true
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
				atom, prev, quantified = 1, n, false
				continue
			}
			if !bounds.runsOn {
				n, prev = repeated(prev, atom, bounds.lo, bounds.hi), n
			}
			atom, quantified = 1, true
			continue
		default:
			atom, prev, quantified = 1, n, false
		}
		s.i++
	}
	return clampWidth(total + n)
}

// repeated returns the width a run reaches once the atom ending it, itself atom
// alternatives wide, may repeat lo to hi times. Each count the quantifier permits
// is a set of paths of its own and they add: "[ab]{1,2}" matches the two "[ab]"
// does and the four its pairs spell, where "[a][ab]?" matches three. Counting
// only whether none was permitted left the wider of the two the narrower by this
// measure.
//
// prev is what the run measured before the atom multiplied it in, carried here
// rather than divided back out of it: a product that reached the clamp cannot be
// undone, and dividing invented a width below it — one that sorted a five-class
// rule ahead of the subset it contains.
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
// it lists, so this is the only place its breadth is read: it separates two
// rules that pin the same amount, "[a-z]" matching twenty-six paths where "[a]"
// matches one.
func (s *literalScanner) classWidth() int {
	rule := s.rule
	j := s.i + 1
	negated := false
	if j < len(rule) && rule[j] == '^' {
		negated, j = true, j+1
	}

	size := 0
	if j < len(rule) && rule[j] == ']' {
		// First inside the brackets, "]" is a member rather than the close.
		size, j = 1, j+1
	}

	for j < len(rule) && rule[j] != ']' {
		if end := posixNameEnd(rule, j); end > j {
			// A POSIX name stands for a set of its own, counted like a class
			// escape. Its own "]" is not the class's, and reading one as the
			// close left the rest of the class scanned as pattern text.
			size, j = size+setMemberWidth, end
			continue
		}
		switch {
		case rule[j] == '\\':
			// A class escape stands for a set of its own — unless it spells one
			// byte, as "[\.]" does, which is a member like any other: counting
			// that as a set put it behind the "[.-/]" range containing it.
			span, pins := escapeSpan(rule, j)
			if pins {
				size, j = size+1, j+span
				break
			}
			size, j = size+setMemberWidth, j+span
		case j+2 < len(rule) && rule[j+1] == '-' && rule[j+2] != ']':
			if lo, hi := rule[j], rule[j+2]; hi >= lo {
				size += int(hi-lo) + 1
			}
			j += 3
		default:
			size, j = size+1, j+1
		}
	}
	if j < len(rule) {
		j++ // past the "]"
	}
	s.i = j

	if negated {
		size = 256 - size
	}
	return max(size, 1)
}

// maxPatternWidth bounds the product a nest of groups builds, since the count is
// only ever compared against another and nothing needs the exact figure.
const maxPatternWidth = 1 << 20

// maxRepeatCount is the largest count Go's regexp parser takes in a "{m,n}",
// above which it refuses the pattern outright: regexp/syntax spells it 1000.
const maxRepeatCount = 1000

// setMemberWidth is what a class member standing for a set of its own counts,
// whether it is written as an escape, "[\d]", or as a POSIX name, "[[:alpha:]]".
// How large the set is cannot be read off either spelling alone and would not
// separate two of them anyway, but one member's worth read "[[:alpha:]]" as no
// wider than the "[a]" it contains, and the tie left key order to pick between
// them. Two says the one thing that is certain: a set is wider than the single
// byte a listed member pins.
const setMemberWidth = 2

func clampWidth(n int) int {
	return min(n, maxPatternWidth)
}

// scaledWidth multiplies one width by another, saturating rather than wrapping.
// Both stay at or below maxPatternWidth, but their product does not fit an int
// where an int is thirty-two bits wide, and a product that wraps comes back
// negative: on a 386 build "/p/[a-z][a-z][a-z][a-z][a-z][a-zA-Z]{1,2}" measured
// -1405091840 and sorted ahead of the "/p/[a][a-z][a-z][a-z][a-z][a]" it contains.
func scaledWidth(a, b int) int {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > maxPatternWidth/b {
		return maxPatternWidth
	}
	return clampWidth(a * b)
}

// carriesRun grades how open-ended a rule's broadest run of bytes is: 0 where
// every position is bounded, 1 where a run repeats something the rule names, and
// 2 where it repeats anything at all. The rule that runs on sorts behind the one
// that does not, and the run repeating anything behind the run that does not.
//
// Fiber's "*" is the second kind, expanded to "(.*)" before the key is compiled;
// a "+" or a "{2,}" is the first, since what it repeats is written beside it.
// The distinction is what separates "/p/*a" from the "/p/(a|aa)+" it contains:
// both run on, and grading them alike left the pair to a width that reads the
// wildcard's bytes as one — measuring the broad rule 1 against the narrow rule's
// 2, and handing the shared path and its query to the broad target.
//
// Their breadth is one no width can stand for, since a width saturates and two
// saturated rules tie. Graded here, the width goes on measuring what the rules
// pin besides the run — which is what separates "/p/[z]+" from the "/p/[a-z]+"
// containing it, a pair a saturated width left tied for key order to pick apart.
// How many wildcards a rule carries is read after the width, by wildcardRank: a
// second wildcard widens a rule but says nothing about what the rest of it pins,
// so a rule holding two can still be the narrower of the pair.
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

// wildcardRank returns the number of Fiber "*" wildcards in a rule, which is the
// last thing separating two that the width leaves tied: "/p/*a*b" and "/p/*ab"
// both expand to a single alternative, so nothing but the count stands between
// the broader rule and the narrower one it would shadow.
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
// skipping the same classes and quoted spans.
//
// A star inside a character class or a "\Q ... \E" span names itself instead:
// the expansion leaves "[(.*)]" a class and "\Q(.*)\E" literal text.
func scanRuns(rule string) ruleRuns {
	runs := ruleRuns{}
	for i := 0; i < len(rule); {
		switch rule[i] {
		case '\\':
			if i+1 < len(rule) && rule[i+1] == 'Q' {
				// Quoted to the matching "\E", or to the end of the rule when
				// there is none, which is how Go's parser reads one. Only the
				// quoted tail is given up: the wildcards standing before it are
				// still expanded, so the count already reached is what holds.
				if end := strings.Index(rule[i+2:], `\E`); end >= 0 {
					i += 2 + end + 2
					continue
				}
				return runs
			}
			size, _ := escapeSpan(rule, i)
			i += size
		case '[':
			s := literalScanner{rule: rule, i: i}
			s.classWidth() // leaves s.i just past the "]"
			i = s.i
		case '*':
			runs.wildcards++
			i++
		case '+':
			runs.unbounded = true
			i++
		case '{':
			var bounds quantifierBounds
			i, bounds = skipQuantifier(rule, i)
			runs.unbounded = runs.unbounded || bounds.runsOn
		default:
			i++
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

// smallerBranch folds one more branch's count into the smallest seen, where -1
// stands for no branch counted yet.
func smallerBranch(smallest, n int) int {
	if smallest < 0 || n < smallest {
		return n
	}
	return smallest
}

// quantifierBounds is how many times a "{m,n}" lets the atom before it repeat.
// runsOn marks the "{2,}" that names no upper bound, whose min and max say
// nothing; allowsNone is the min of zero read on its own, since that is all the
// literal length needs of it. quantifies separates the braces that bound a
// repetition from the ones standing as text, whose bounds say nothing either.
type quantifierBounds struct {
	lo, hi     int
	allowsNone bool
	runsOn     bool
	quantifies bool
}

// skipQuantifier returns the index just past the "{m,n}" beginning at i and what
// it bounds. An unclosed "{" is an ordinary byte to the regexp parser, so it is
// left as one here: the index only moves past it, bounding nothing.
//
// So is a "{" whose body spells no bound. Its braces and everything between them
// are rule like any other and are walked rather than skipped: skipping them hid
// the run in "/p/{x*}", which then sorted ahead of the "/p/[{][x]([a]+)[}]" it
// contains.
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
// it is read as repeating what it follows exactly once, which is to say as
// bounding nothing.
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

// repeatCount reads one bound of a "{m,n}". Go's repetition grammar spells a
// bound in decimal digits and nothing else, so a sign makes the braces literal
// text: "a{-0}" is a path of six bytes, not an "a" repeated none. Reading the
// sign as a count took four bytes off what that rule pins.
func repeatCount(bound string) (int, bool) {
	if bound == "" {
		return 0, false
	}
	for i := 0; i < len(bound); i++ {
		if bound[i] < '0' || bound[i] > '9' {
			return 0, false
		}
	}

	// Go's parser refuses a count above maxRepeatCount, so a rule carrying one
	// never compiles and the braces are left to be read as they stand. Capped
	// here as well as there, since the sort measures a rule before it is
	// compiled: walking to a bound of a billion spent seconds of startup on a
	// rule that was going to be rejected anyway.
	n, err := strconv.Atoi(bound)
	if err != nil || n > maxRepeatCount {
		return 0, false
	}
	return n, true
}

// posixNameEnd returns the index just past the POSIX class name beginning at i
// inside a character class — "[[:alpha:]]" names the letters — or i when nothing
// there names one. The name carries a "]" of its own, and stopping the class
// scan at it left the members standing after the name read as pattern text: the
// stars of "[[:alpha:]*]" counted as wildcards rather than as class members.
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

// escapeSpan measures the escape beginning at the backslash at i: how many bytes
// of the rule it occupies, and whether it stands for one character of a path.
//
// Punctuation does — "/a\.b" pins "/a.b" — and so does every spelling that names
// one character outright, including "\x{61}", "\x61", "\101" and "\t". A letter
// leading anything else introduces a class or an assertion: "\d" matches any
// digit, so "/api/\d+" pins no more than "/api/" and must not outrank the exact
// "/api/1" it would otherwise shadow.
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
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		// "\d", "\w", "\p{Greek}", "\b" — a class or an assertion. Measured as
		// two bytes even where it runs longer, which only ends the prefix sooner.
		return 2, false
	default:
		return 2, true
	}
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
		case c == '\\' && i+1 < len(rule):
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
// placeholders in between are stepped over; each is asked for digits in turn,
// which is the same question asked of the whole.
func opensPort(before []authorityChunk) bool {
	for _, chunk := range slices.Backward(before) {
		if chunk.placeholder {
			continue
		}
		return strings.HasSuffix(chunk.text, ":")
	}
	return false
}
