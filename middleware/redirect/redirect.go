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
		// Two rules pinning the same amount are separated by how much else they
		// match: an alternation matches every branch, so "/very/specific|/x" is
		// wider than the exact "/x" it ties with.
		if d := cmp.Compare(len(splitAlternation(a)), len(splitAlternation(b))); d != 0 {
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
		case special && !isFileScheme(target):
			start = i + 1
		default:
			// No authority to move. Under "file" that holds without the slashes
			// too: the parser leaves "file state" for "path state" unless one
			// follows, so "file:tmp/x" is the path "/tmp/x" of an empty host and
			// dropping the rule told the author something untrue about it. What a
			// value can still do is write the "//" itself, which the composed
			// location is checked for.
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
// and must not outrank the exact "/api/users". "$" is not, anchoring rather
// than matching, and the compiled pattern anchors the end regardless.
const patternBytes = `*.[](){}+?^|\`

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
//
// An alternation pins only what its least specific branch does: "/very/specific|/x"
// matches "/x", so it pins one byte rather than the fourteen standing before the
// "|", which had it outrank the exact "/x" and shadow it outright.
func literalPrefixLen(rule string) int {
	return minOverBranches(rule, func(branch string) int {
		if i := indexUnescapedAny(branch, patternBytes); i >= 0 {
			return i
		}
		return len(branch)
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
			atom = 0
			if s.i+1 < len(s.rule) && !inClass && escapePinsAByte(s.rule[s.i+1]) {
				n, atom = n+1, 1
			}
			s.i += 2
			continue
		case inClass:
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
			var zeroMin bool
			s.i, zeroMin = skipQuantifier(s.rule, s.i)
			if zeroMin {
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

// smallerBranch folds one more branch's count into the smallest seen, where -1
// stands for no branch counted yet.
func smallerBranch(smallest, n int) int {
	if smallest < 0 || n < smallest {
		return n
	}
	return smallest
}

// skipQuantifier returns the index just past the "{m,n}" beginning at i, and
// whether it allows none of what it follows. An unclosed "{" is an ordinary byte
// to the regexp parser, so it is left as one here: the index only moves past it.
func skipQuantifier(rule string, i int) (int, bool) {
	end := strings.IndexByte(rule[i:], '}')
	if end < 0 {
		return i + 1, false
	}

	body := rule[i+1 : i+end]
	return i + end + 1, body == "0" || strings.HasPrefix(body, "0,")
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

// escapePinsAByte reports whether a backslash followed by c stands for the one
// literal byte it is written with. Punctuation does — "/a\.b" pins "/a.b" — while
// a letter or digit introduces a class, an assertion or a numeric escape: "\d"
// matches any digit, so "/api/\d+" pins no more than "/api/" and must not
// outrank the exact "/api/1" it would otherwise shadow.
func escapePinsAByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return false
	default:
		return true
	}
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

// indexUnescapedAny returns the first index in s of a byte from chars that a
// backslash does not escape, or -1.
func indexUnescapedAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			if i+1 >= len(s) || !escapePinsAByte(s[i+1]) {
				// A class or an assertion pins nothing, so the prefix ends here.
				return i
			}
			i++ // an escaped metacharacter matches itself
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
