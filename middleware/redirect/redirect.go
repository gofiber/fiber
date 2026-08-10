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
		pattern = "^" + pattern + "$"
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
	if special {
		for start < len(target) && (target[start] == '/' || target[start] == '\\') {
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
		value := replacer.Replace(chunk.text)

		if hostPins(chunks[i+1:]) {
			// The author closed the host past this token, so the value is a
			// label inside it and only has to stay one.
			forbidden := `/\?#@:`
			if userinfoCloses(chunks[i+1:]) {
				// Except that the token sits in userinfo, where ":" separates a
				// password and "@" is not the last one — the author wrote that.
				// Only the four that end the authority outright still matter.
				forbidden = `/\?#`
			}
			if strings.ContainsAny(value, forbidden) {
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
	if strings.HasPrefix(mapped, ".") {
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
		// An address pins a host only where the capture cannot reach its first
		// octet. Open on the left it can: against "%3110.0.0.1", a captured "0"
		// composes "0%3110.0.0.1", octal 0127, landing on 72.0.0.1.
		return !openLeft && net.ParseIP(trimmed) != nil
	}
	return true
}

// patternBytes are the bytes of a rule key that match something other than
// themselves. "*" is the documented wildcard; the rest are regexp syntax, which
// reaches the compiled pattern because a key is used as one. "." is among them:
// it matches any byte, so "/api/user." pins no more of a path than "/api/user"
// and must not outrank the exact "/api/users". "$" is not, anchoring rather
// than matching, and the compiled pattern anchors the end regardless.
const patternBytes = `*.[](){}+?^|\`

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
func literalPrefixLen(rule string) int {
	if i := indexUnescapedAny(rule, patternBytes); i >= 0 {
		return i
	}
	return len(rule)
}

// literalLen returns how much of a rule's path is pinned in total, which
// separates two rules whose wildcards start together: "/cdn/*" and "/cdn/*x"
// tie on prefix, and the lexicographic fallback left the narrower one dead.
func literalLen(rule string) int {
	n := 0
	inClass := false
	for i := 0; i < len(rule); i++ {
		if rule[i] == '\\' && i+1 < len(rule) {
			// An escaped metacharacter matches itself, so it pins the one byte it
			// stands for — "/a\.b" pins "/a.b", the backslash being syntax.
			i++
			if !inClass {
				n++
			}
			continue
		}
		switch {
		case inClass:
			// A class matches one byte whatever it lists, so its members pin
			// nothing: "/api/[a-z]" is no more specific than "/api/[ab]", and
			// counting them put the broader rule ahead of the narrower one.
			inClass = rule[i] != ']'
		case rule[i] == '[':
			inClass = true
		case strings.IndexByte(patternBytes, rule[i]) < 0:
			n++
		}
	}
	return n
}

// indexUnescapedAny returns the first index in s of a byte from chars that a
// backslash does not escape, or -1.
func indexUnescapedAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++ // whatever follows matches itself
			continue
		}
		if strings.IndexByte(chars, s[i]) >= 0 {
			return i
		}
	}
	return -1
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
