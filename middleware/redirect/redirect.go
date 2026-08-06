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
	// authorityChunks splits the target's authority into literal text and "$N"
	// tokens, so each value can be judged by where it lands. Empty when the
	// authority holds no placeholder and so cannot be moved by a request.
	authorityChunks []authorityChunk
	// sameOrigin is set when the target names no authority of its own. The "$N"
	// values spliced into such a target come from the request path, so they must
	// not be able to introduce one.
	sameOrigin bool
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Walk the rules in a fixed order, most specific first: two patterns can
	// match the same path, and a map range made the winner vary from run to run.
	// Lexicographic order alone would sort "/*" ahead of "/old/*", so rank by
	// what a rule pins before its first wildcard, then by total pinned length
	// ("/cdn/*x" ahead of "/cdn/*"), then by key to stay total.
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
		// Read the target the way the client will before judging it: a tab the
		// parser deletes was otherwise scored as author-written host text, so
		// "https://\t$1" passed as naming its own host while a captured
		// "/evil.com" composed "https:///evil.com" — evil.com to a browser.
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
			// Refused like the rest, but not for their reason: the author may
			// well have pinned the network here, as "https://[2001:db8::$1]"
			// does, and only "https://[$1::1]" hands it over. Claiming either
			// way would be untrue of half the rules that reach this, and the
			// escape hatch is a different one.
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
			// The host ends in a capture with nothing pinned after it, so
			// ".evil.com" would extend it into a domain the author does not
			// control. Refused per request, which would otherwise read as the
			// rule quietly not firing.
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

		cfg.rulesRegex = append(cfg.rulesRegex, compiledRule{
			pattern:         regexp.MustCompile(pattern),
			target:          v,
			authorityChunks: chunks,
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
			if !authorityHolds(rule.authorityChunks, replacer) {
				continue
			}

			// Normalize on every branch: the bytes removed are never meaningful
			// and the client drops them anyway, so the guard below always runs
			// on the location as it will be read.
			location := urlnorm.AsBrowserReads(replacer.Replace(rule.target))
			if rule.sameOrigin {
				location = keepSameOrigin(location)
			}
			location = withRequestQuery(location, utils.UnsafeString(c.RequestCtx().QueryArgs().QueryString()))
			return c.Redirect().Status(cfg.StatusCode).To(location)
		}
		return c.Next()
	}
}

// withRequestQuery carries the request's query string onto the composed
// location.
//
// Appending "?" + query suited only a target holding neither. One with its own
// query got a second "?", folding the request's query into the target's last
// value ("/new?from=old" + "bar=2" -> "from=old?bar=2"); one with a fragment got
// the query after the "#", losing it. Merge, and place it before the fragment.
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
// destination — an absolute URL, or a protocol-relative one. Where it does,
// leaving this origin is what the author asked for and the composed location is
// left alone.
func targetNamesAuthority(target string) bool {
	return strings.HasPrefix(target, "//") || schemeEnd(target) > 0
}

// authoritySpan returns the byte range of target's own authority, or 0, 0 when
// target names none.
//
// It runs from the "//" that opens it to the first "/", "?" or "#" (RFC 3986),
// plus "\", which the WHATWG parser treats as a slash there.
//
// Slashes right after the opening pair belong to neither side: the parser skips
// the whole run before reading the host, so stopping at the first made "///$1"
// look like an empty authority with nothing to guard.
//
// A special scheme needs no "//" at all — "https:host" and "https://host" name
// the same host — so requiring it guarded one spelling and not the other.
func authoritySpan(target string) (start, end int) { //nolint:nonamedreturns // the pair is a range; names say which is which
	switch {
	case strings.HasPrefix(target, "//"):
		start = 2
	default:
		i := schemeEnd(target)
		switch {
		case i <= 0:
			return 0, 0
		case strings.HasPrefix(target[i+1:], "//"):
			start = i + 3
		case isSpecialScheme(target[:i]):
			start = i + 1
		default:
			return 0, 0
		}
	}

	for start < len(target) && (target[start] == '/' || target[start] == '\\') {
		start++
	}

	if offset := strings.IndexAny(target[start:], `/\?#`); offset >= 0 {
		return start, start + offset
	}
	return start, len(target)
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

// authorityHolds reports whether the values about to be substituted leave the
// target's authority naming the host the author wrote.
//
// Where a token sits decides what it may contain:
//
//   - More authority follows it ("https://$1.cdn.example.com"): the value is a
//     label inside a host the author closed, so it must carry nothing that ends
//     the authority ("/", "\", "?", "#"), makes what precedes it userinfo ("@"),
//     or opens a port (":"). Otherwise "evil.com/x" composes
//     "https://evil.com/x.cdn.example.com", whose authority stops at the slash.
//   - It ends the authority ("https://cdn.example.com$1"): nothing closes the
//     host, so the value must open a component, be empty, or be a port where the
//     author wrote the colon — else "evil.com" composes "example.comevil.com".
//
// A target whose authority is nothing but a capture is refused by New instead.
func authorityHolds(chunks []authorityChunk, replacer *strings.Replacer) bool {
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
			if strings.ContainsAny(value, `/\?#@:`) {
				return false
			}
			continue
		}

		// Nothing closes the host after this token, so a value that does not
		// open the next component extends it into a registrable domain:
		// "evil.com" composes "example.comevil.com".
		switch {
		case value == "":
		case strings.IndexByte(`/\?#`, value[0]) >= 0 && hostPins(chunks[:i]):
			// Opens the next component, so the authority ended at the author's
			// host — but only where they wrote one: with nothing but separators
			// ahead, "//$1." composing "///evil.com." ends no authority, since
			// the parser skips the slashes and reads the rest as the host.
		case i > 0 && strings.HasSuffix(chunks[i-1].text, ":") && isAllDigits(value):
			// A port. The author wrote the colon, and the URL parser rejects a
			// port holding anything but digits outright, so digits are the only
			// value that can be what they meant.
		default:
			return false
		}
	}
	return true
}

// hostPins reports whether the author wrote host text among these chunks, which
// is asked of the chunks on one side of a capture or the other.
//
// Ahead of a capture it is what leaves the author's text as the host once the
// value opens the next component; past one it is what makes the value land
// inside a host the author closed. A port closes nothing, which is why
// "https://$1:8080" still leaves $1 free to name the host outright.
func hostPins(chunks []authorityChunk) bool {
	for _, chunk := range chunks {
		if !chunk.placeholder && chunk.pins {
			return true
		}
	}
	return false
}

// targetLetsRequestPickHost reports whether the request, not the author, decides
// the host a target reaches.
//
// An authority holding a capture and no host text of the author's hands the
// host over whole — "https://$1", and equally "https://$1:8080", since a port
// pins no host. This is the static half of the question authorityHolds asks per
// value, so the two agree by construction.
//
// "https:$1" names no authority to guard and no origin to hold it to, yet a
// value of "//evil.com" still composes "https://evil.com".
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

	// No authority span, so a value reaches a host only by opening one itself,
	// and only from immediately after the scheme's colon. "//" does that for
	// every scheme, so "mailto:$1" and "myapp:$1" both composed a host of the
	// request's choosing; special schemes need no slashes at all.
	//
	// Author text between the colon and the capture settles it, and a leading
	// capture is still safe where host text follows — "mailto:$1@example.com"
	// reads example.com as the host and the capture as userinfo.
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

// captureInBrackets reports whether a "$N" token falls inside the brackets of
// an IPv6 literal in the target's authority.
//
// A DNS name is written least significant label first, so "$1.example.com"
// stays under example.com. A bracketed address reverses that, and rather than
// carry a second and opposite reading of the same question, refuse the shape:
// judged by the ordinary rule, "https://[$1::1]" let the request pick the
// routing prefix and reach loopback. Write the address in full and capture the
// port instead.
func captureInBrackets(target string) bool {
	start, end := authoritySpan(target)
	authority := target[start:end]

	// Everything through the last "@" is userinfo, where a bracket is an
	// ordinary character the parser percent-encodes rather than a host
	// delimiter. Counting one there raised the depth for the rest of the scan
	// and dropped rules whose host the author had pinned outright, telling them
	// the request chose it.
	//
	// Only an "@" outside the brackets ends the userinfo, though. One inside
	// them is not a delimiter to the URL parser either — it rejects the whole
	// authority — so reading it as one would look past the brackets that really
	// do hold the host and let a capture inside them through.
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

// keepSameOrigin strips an authority that the captured path segments introduced
// into a target that named none.
//
// The "$N" values are request path bytes, slash runs intact, so "/api/*" ->
// "/$1" turned a request for "/api//evil.com" into "Location: //evil.com", and
// "/redirect/*" -> "$1" into an outright absolute redirect.
//
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
// capture rather than by anything the author wrote, so a value can extend it
// into a domain they do not control.
//
// It is the static half of what authorityHolds decides per request: a token
// with no host-pinning literal after it. A token right after the port colon
// does not count — a port cannot extend a host, and authorityHolds accepts a
// digit run there, so the rule still fires for every value that could have been
// meant.
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

// percentDecode decodes the valid "%XX" escapes in s and passes anything else
// through untouched, the way a URL parser decodes a host before reading it.
//
// Without it the escape counted as host text of the author's while the client
// saw whatever it stands for: "https://$1%2E" pinned a host on three characters
// that decode to the bare "." this guard already knows pins nothing, and
// "https://$1%C2%AD" on a soft hyphen that the mapping then deletes. Both
// composed a location a browser resolves to the captured host alone.
//
// A stray "%" is left as-is rather than treated as an error, since that is what
// the parser does with it, and returning the raw string on the first bad escape
// would keep exactly the text this is meant to remove.
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
// fixes part of the host.
//
// Only what follows the last "@" and precedes the port colon counts, once the
// punctuation around a hostname is stripped. ":8080" pins nothing, nor does a
// lone "." (the DNS root), nor text before an "@" (userinfo), nor the brackets
// of an IPv6 literal. Treating any as author-written host text left the capture
// beside it judged an interior label, and the request named the host.
//
// The "@" is read per chunk, so a capture splitting an authority ahead of its
// real last "@" scores a literal here as host text — "https://a.com$1@$2".
// Nothing escapes: every value authorityHolds accepts there ends the authority
// first or leaves the host empty.
//
// openLeft says where the literal sits, which only the caller knows: a capture
// reaches a literal it precedes, not one opening the authority or following
// an "@".
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
		// With an opener the author wrote an address — but only if they wrote a
		// real one. Ask whether it parses rather than whether it looks like it:
		// "[zzz1]" holds hex digits and is not an address, and counting it as
		// author text bought a rule redirecting nowhere a client could follow.
		//
		// Brackets hold IPv6 only, so "[127.0.0.1]" is no host however well it
		// parses. The colon separates the spellings, since ParseIP takes both
		// and To4 would reject "[::ffff:127.0.0.1]", which is legal.
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
	// Read what is left the way the parser reads a host: UTS #46 mapping deletes
	// some 270 code points and folds three onto ".", and an ASCII-only trim
	// scored a soft hyphen as author-written text — "https://$1\u00ad" composed
	// "https://evil.com\u00ad", which a browser reads as evil.com. What the
	// mapping refuses outright pins nothing either.
	//
	// Invalid UTF-8 is not refused — it becomes U+FFFD with no error — so rule
	// it out first: a lead byte the value supplies could pair with a
	// continuation byte here.
	decoded := percentDecode(literal)
	if !utf8.ValidString(decoded) {
		return false
	}

	mapped, err := hostMapping.ToUnicode(decoded)
	if err != nil {
		return false
	}

	// Nor does anything at or below a space, nor DEL. The parser strips a
	// leading or trailing run of them from the whole input before it reads
	// anything, and urlnorm.AsBrowserReads does the same to the composed location — so
	// a control character in the target pinned a host that was gone by the time
	// the client saw it. "https://$1\x01$2" composed "https://evil.com\x01"
	// from a captured "evil.com" and an empty second capture, and shipped
	// "https://evil.com". One in the middle is a forbidden domain code point
	// instead, so it pins no host either way.
	trimmed := strings.TrimFunc(mapped, func(r rune) bool {
		return r <= ' ' || r == 0x7f || strings.ContainsRune(".[:", r)
	})
	if trimmed == "" {
		return false
	}

	// A host whose last label reads as a number is parsed as an IPv4 address,
	// where the author's trailing text is the low octets and the request
	// supplies the network: "https://$1.1" composed "https://127.0.0.1" from a
	// captured "127.0.0". Only a complete address pins a host that way.
	// With no dot of its own the literal is only the tail of a label the capture
	// opens, and the value decides what that label becomes:
	//
	//	"https://$1xyz"  + "evil."     -> https://evil.xyz    a registrable name
	//	"https://$1cafe" + "0x"        -> https://0xcafe      0.0.202.254
	//	"https://$1E"    + "evil.com%2"-> https://evil.com%2E evil.com
	//
	// Judged before the trim: the literal's own leading "." is what closes the
	// label, so trimming first made every all-hex TLD look open.
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
		// octet — the network. Open on the left it can: against the literal
		// "%3110.0.0.1", a captured "0" composes "0%3110.0.0.1", which reads as
		// octal 0127 and lands on 72.0.0.1.
		return !openLeft && net.ParseIP(trimmed) != nil
	}
	return true
}

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
func literalPrefixLen(rule string) int {
	if i := strings.IndexByte(rule, '*'); i >= 0 {
		return i
	}
	return len(rule)
}

// literalLen returns how much of a rule's path is pinned in total, which is what
// separates two rules whose wildcards start at the same place.
//
// The prefix alone cannot: "/cdn/*" and "/cdn/*x" both pin "/cdn/", so ordering
// them by prefix is a tie, and the lexicographic fallback put the broader rule
// first — where it matches everything the narrower one would, leaving "/cdn/*x"
// dead. What "x" pins is real, it just sits on the far side of the wildcard.
func literalLen(rule string) int {
	return len(rule) - strings.Count(rule, "*")
}
