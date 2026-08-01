package redirect

import (
	"cmp"
	"maps"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// authorityChunk is one piece of a target's own authority: either literal text
// the author wrote or a "$N" token the request fills in.
type authorityChunk struct {
	text        string // literal text, or the token itself for a placeholder
	placeholder bool
}

// compiledRule is one configured rule with its target and the decision, made
// once at construction, of whether that target picks its own destination.
type compiledRule struct {
	pattern *regexp.Regexp
	target  string
	// authorityChunks splits the target's own authority — the span that decides
	// which host the redirect reaches — into literal text and "$N" tokens, so
	// each substituted value can be judged by where it lands. Empty when the
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

	// Initialize. The rules are walked in a fixed order rather than the map's:
	// two patterns can match the same path, and with a map range the winner —
	// and now, since a rule can be refused and fall through to the next, whether
	// there is a redirect at all — changed from run to run.
	//
	// Most specific first, measured by how much of the path a rule pins before
	// its first wildcard. Plain lexicographic order would sort "/*" ahead of
	// "/old/*" and let the catch-all shadow it on every request; ties fall back
	// to the key so the order stays total.
	keys := slices.Collect(maps.Keys(cfg.Rules))
	slices.SortFunc(keys, func(a, b string) int {
		if d := cmp.Compare(literalPrefixLen(b), literalPrefixLen(a)); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})

	for _, k := range keys {
		// Read the target the way the client will, before deciding anything
		// about it. The URL parser deletes every tab, LF and CR before it
		// parses, and the guard below was reading the bytes as written — so a
		// tab was scored as author-written host text that then vanished on the
		// way to the client. "https://\t$1" passed as a target naming its own
		// host, and a captured "/evil.com" composed "https:///evil.com", which
		// a browser reads as evil.com. A tab also defeated the leading-slash
		// skip in authoritySpan, leaving "https://\t/[$1::1]" with no authority
		// to guard at all. Normalizing here closes both, and composing from
		// these bytes changes no Location, since the same pass runs on the
		// composed value anyway.
		v := asBrowserReads(cfg.Rules[k])
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
			// The request would pick the host this redirects to. That is an open
			// redirect by construction — nothing here can tell the intended
			// destination from an attacker's — so the rule never fires, and this
			// says why, since the alternative is a rule that silently does
			// nothing.
			log.Warnf("[REDIRECT] rule %q is ignored: its target takes the redirect host from the request path, "+
				"so anyone who can shape the path would choose where the client lands. Pin the host in the target", k)
		case authorityEndsInOpenCapture(chunks):
			// The target's host ends in a capture with nothing pinned after it,
			// so a value like ".evil.com" would extend the host into a domain
			// the author does not control. Such a value is refused per request,
			// which without this would look like the rule quietly not firing.
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

			// A target may put a capture inside its own authority —
			// "https://$1.cdn.example.com" is a plausible way to route per
			// tenant — and the author means $1 to be a label, not a whole URL.
			// Refuse the rule when a value would move the host, rather than
			// guess what the author meant; the request falls through to the app.
			if !authorityHolds(rule.authorityChunks, replacer) {
				continue
			}

			// Normalize on every branch, not just the guarded one. The bytes
			// asBrowserReads removes are never meaningful in a URL and the
			// client drops them anyway, so an author-configured absolute target
			// loses nothing — and the guard below then always runs on the
			// location as it will actually be read.
			location := asBrowserReads(replacer.Replace(rule.target))
			if rule.sameOrigin {
				location = keepSameOrigin(location)
			}
			queryString := utils.UnsafeString(c.RequestCtx().QueryArgs().QueryString())
			if queryString != "" {
				location += "?" + queryString
			}
			return c.Redirect().Status(cfg.StatusCode).To(location)
		}
		return c.Next()
	}
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
// An authority runs from the "//" that opens it to the first byte that ends it:
// "/", "?" or "#" per RFC 3986, plus "\" because the WHATWG URL parser treats a
// backslash there as a slash. A scheme with no "//" after it — "mailto:x" — has
// no authority at all, and neither does a path-only target.
//
// Any further slashes right after the opening pair belong to neither: the
// parser's special-authority-ignore-slashes state skips the whole run before it
// starts reading the host. Stopping the span at the first of them instead made
// "///$1" look like a target with an empty authority — no chunks to guard, and
// still "absolute" enough to skip keepSameOrigin — while the browser read
// "///evil.com" as evil.com.
func authoritySpan(target string) (start, end int) { //nolint:nonamedreturns // the pair is a range; names say which is which
	switch {
	case strings.HasPrefix(target, "//"):
		start = 2
	default:
		i := schemeEnd(target)
		if i <= 0 || !strings.HasPrefix(target[i+1:], "//") {
			return 0, 0
		}
		start = i + 3
	}

	for start < len(target) && (target[start] == '/' || target[start] == '\\') {
		start++
	}

	if offset := strings.IndexAny(target[start:], `/\?#`); offset >= 0 {
		return start, start + offset
	}
	return start, len(target)
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
			chunks = append(chunks, authorityChunk{text: authority[literal:i]})
		}
		chunks = append(chunks, authorityChunk{text: authority[i:j], placeholder: true})
		literal = j
		i = j
	}
	if chunks == nil {
		return nil
	}
	if literal < len(authority) {
		chunks = append(chunks, authorityChunk{text: authority[literal:]})
	}
	return chunks
}

// authorityHolds reports whether the values about to be substituted leave the
// target's authority naming the host the author wrote.
//
// Where a token sits decides what it may contain:
//
//   - More authority follows it ("https://$1.cdn.example.com"): the author
//     pinned the end of the host, so the value is a label inside it and must not
//     carry a byte that ends the authority ("/", "\", "?", "#"), starts a new
//     host by making everything before it userinfo ("@"), or opens a port (":").
//     Without that check a value of "evil.com/x" composes
//     "https://evil.com/x.cdn.example.com", whose authority stops at the slash —
//     the browser goes to evil.com.
//   - It ends the authority ("https://cdn.example.com$1", and equally
//     "https://example.com$1/health"): nothing the author wrote closes the host,
//     so any value that does not open the next component extends it into a
//     domain they do not control — "evil.com" composes "example.comevil.com",
//     which someone can register. The value must open a component — "/", "\",
//     "?" or "#" — or be empty, or be a port where the author wrote the colon.
//     What follows the authority in the target makes no difference: the host is
//     already decided by then.
//
// A target whose authority is nothing but a capture never reaches here: New
// marks it letsRequestPickHost and the rule is refused outright.
func authorityHolds(chunks []authorityChunk, replacer *strings.Replacer) bool {
	for i, chunk := range chunks {
		if !chunk.placeholder {
			continue
		}
		// Through the Replacer, not by indexing the captures: the Replacer is
		// what composes the location, and it matches its patterns in the order
		// they were given, so "$10" is consumed as "$1" followed by a literal
		// "0". Indexing would have judged the tenth capture while the first was
		// the one actually spliced into the host.
		value := replacer.Replace(chunk.text)

		if hostPinnedAfter(chunks, i) {
			// The author closed the host past this token, so the value is a
			// label inside it and only has to stay one.
			if strings.ContainsAny(value, `/\?#@:`) {
				return false
			}
			continue
		}

		// Nothing the author wrote closes the host after this token, so any
		// value that does not open the next component extends it into a domain
		// they do not control: "evil.com" composes "example.comevil.com", which
		// someone can register.
		switch {
		case value == "":
		case strings.IndexByte(`/\?#`, value[0]) >= 0 && hostPinnedBefore(chunks, i):
			// Opens the next component, so the authority ended at the host the
			// author wrote. Only where they wrote one: with nothing but
			// separators ahead of it, "//$1." composing "///evil.com." does not
			// end an authority at all — the parser skips every leading slash and
			// reads the rest as the host.
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

// hostPinnedAfter reports whether the author wrote host text past the chunk at
// index i, so that a value there lands inside a host they closed themselves.
//
// A port does not close a host and neither do the separators that may trail
// one, which is why "https://$1:$2" and "https://$1:8080" leave $1 free to name
// the host outright: ":443" and ":8080" pin nothing.
func hostPinnedAfter(chunks []authorityChunk, i int) bool {
	for j := i + 1; j < len(chunks); j++ {
		if !chunks[j].placeholder && pinsHost(chunks[j].text) {
			return true
		}
	}
	return false
}

// targetLetsRequestPickHost reports whether the request, not the author, decides
// the host a target reaches.
//
// An authority that holds a capture and no host text of the author's hands the
// host over whole. "https://$1" and "//$1" are the plain spellings, but a
// capture followed by nothing that pins a host does the same: "https://$1:8080"
// pins a port and "//$1." a trailing dot, and neither is a host. This is the
// static half of the question authorityHolds asks per value, so the two agree
// by construction — every value it would refuse belongs to a rule refused here
// first.
//
// "https:$1" has no "//" at all, so it names no authority for authorityChunks
// to guard and no origin for keepSameOrigin to hold it to — yet a value of
// "//evil.com" still composes "https://evil.com".
func targetLetsRequestPickHost(target string, chunks []authorityChunk) bool {
	// A non-nil chunk list always holds a capture — authorityChunks returns nil
	// unless it appended one — so the only question left is whether the author
	// wrote any host text of their own alongside it.
	if chunks != nil {
		if captureInBrackets(target) {
			return true
		}
		for _, chunk := range chunks {
			if !chunk.placeholder && pinsHost(chunk.text) {
				// The host is theirs, and authorityHolds judges the captures
				// around it per value.
				return false
			}
		}
		return true
	}

	// No authority span. Only a scheme the client navigates by can still reach a
	// host of the request's choosing — "https:$1" with a value of "//evil.com"
	// composes "https://evil.com". A scheme with no authority syntax at all,
	// "mailto:$1@example.com", has no host to hijack, and refusing it would drop
	// a working rule while telling the author something untrue about it.
	i := schemeEnd(target)
	if i <= 0 || strings.HasPrefix(target[i+1:], "//") {
		return false
	}
	if !utils.EqualFold(target[:i], "http") && !utils.EqualFold(target[:i], "https") {
		return false
	}
	return containsPlaceholder(target[i+1:])
}

// captureInBrackets reports whether a "$N" token falls inside the brackets of
// an IPv6 literal in the target's authority.
//
// Everywhere else the author's host text sits after the capture, because a DNS
// name is written least significant label first: "$1.example.com" stays under
// example.com whatever the request supplies. A bracketed address reverses that
// — it is written most significant group first, so there the text before the
// capture is what pins where the redirect lands.
//
// Rather than carry a second and opposite reading of the same question, refuse
// the shape. Both halves of it were wrong when judged by the ordinary rule:
// "https://[$1::1]" let the request choose the routing prefix and reach
// loopback or link-local, and "https://[2001:db8::$1]" — where the author did
// pin the network — redirected only for a value that happened to be all
// digits, silently dropping every hex group. An author who wants either can
// write the address in full and capture the port.
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

// containsPlaceholder reports whether s holds a "$N" token.
func containsPlaceholder(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '$' && s[i+1] >= '0' && s[i+1] <= '9' {
			return true
		}
	}
	return false
}

// keepSameOrigin strips an authority that the captured path segments introduced
// into a target that named none.
//
// The target is written by the application author, but the "$N" values spliced
// into it are request path bytes, and the path arrives with its slash runs
// intact. So the documented rule "/api/*" -> "/$1" turns a request for
// "/api//evil.com" into "Location: //evil.com" — a network-path reference the
// browser follows to evil.com — and "/redirect/*" -> "$1" turns
// "/redirect/https://evil.com" into an outright absolute redirect. An author
// who wrote a path-only target did not ask for either.
// It expects a location that has already been through asBrowserReads, so the
// checks below run on the bytes the client will act on rather than on the bytes
// as composed.
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
	// Collapse the run to the single slash the target asked for. Backslashes
	// count as slashes here: the WHATWG URL parser folds them in this position,
	// so a browser reaches evil.com from "/\evil.com" exactly as it does from
	// "//evil.com".
	return "/" + location[n:]
}

// asBrowserReads returns location as the client will actually see it, so the
// checks above run on that rather than on the bytes as composed.
//
// Two rewrites happen to a Location before anything navigates. A recipient
// strips leading and trailing optional whitespace from a field value
// (RFC 9110 Section 5.5), and the WHATWG URL parser removes every ASCII tab,
// LF and CR anywhere in the input before it parses. Skipping them leaves the
// same normalization mismatch this guard exists to close: with UnescapePath
// enabled, "/r/%20//evil.com" under the rule "$1" composes " //evil.com",
// whose leading slash run the guard never sees, and "/api/%09/evil.com" under
// "/$1" composes "/\t/evil.com", which the parser turns back into
// "//evil.com". Both reach evil.com. Ordinary spaces are left alone — the
// parser percent-encodes them rather than removing them, so an interior one
// cannot form an authority.
func asBrowserReads(location string) string {
	var b []byte
	for i := 0; i < len(location); i++ {
		switch c := location[i]; c {
		case '\t', '\n', '\r':
			if b == nil {
				b = append(make([]byte, 0, len(location)), location[:i]...)
			}
		default:
			if b != nil {
				b = append(b, c)
			}
		}
	}
	if b != nil {
		location = string(b)
	}
	return strings.TrimFunc(location, func(r rune) bool { return r <= ' ' })
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
		if !chunk.placeholder || hostPinnedAfter(chunks, i) {
			continue
		}
		if i > 0 && strings.HasSuffix(chunks[i-1].text, ":") && !chunks[i-1].placeholder {
			continue // a port position
		}
		return true
	}
	return false
}

// hostPinnedBefore reports whether the author wrote host text ahead of the
// chunk at index i, so that a value opening the next component really does
// leave that text as the host.
//
// Separators alone do not count. "//$1." puts nothing but a dot before the end
// of the authority, so a value of "/evil.com" composes "///evil.com." — three
// slashes the WHATWG parser skips on its way to reading "evil.com." as the
// host, rather than a path hanging off a host the author chose.
func hostPinnedBefore(chunks []authorityChunk, i int) bool {
	for j := range i {
		if chunks[j].placeholder {
			continue
		}
		if pinsHost(chunks[j].text) {
			return true
		}
	}
	return false
}

// pinsHost reports whether a literal chunk of a target's authority actually
// fixes part of the host.
//
// Only what follows the last "@" and precedes the port colon counts, and only
// once the punctuation that may surround a hostname is stripped. ":8080" pins
// nothing — "evil.com:8080" is still evil.com — and neither does a lone ".",
// since "evil.com." is "evil.com" with the DNS root spelled out. Text before an
// "@" is userinfo, not host: "https://example.com@$1" reads example.com as a
// username and leaves the host to the capture. Nor do the brackets around an
// IPv6 literal, so "https://[$1]:8080" hands over the address the same way
// "https://$1:8080" hands over the name. Treating any of them as author-written
// host text left the capture beside it judged as an interior label, and the
// request named the host.
//
// The "@" is read within this chunk. A capture can split an authority so that
// its real last "@" falls in a later chunk, which makes a literal here score as
// host text when the URL parser reads it as userinfo — "https://a.com$1@$2" is
// the shape. Every value authorityHolds then accepts either ends the authority
// before that "@" or leaves the host empty, so nothing escapes, but the model
// is per chunk rather than per authority.
func pinsHost(literal string) bool {
	if i := strings.LastIndexByte(literal, '@'); i >= 0 {
		literal = literal[i+1:]
	}
	// An IPv6 literal carries colons of its own, so the port colon is the one
	// past the closing bracket rather than the first.
	if i := strings.IndexByte(literal, ']'); i >= 0 {
		// A closing bracket with no opener in this chunk means a capture split
		// the address, and what stands here is its tail. The tail of an IPv6
		// literal is the low bits, not the network it routes to: "::1]" in
		// "https://[$1::1]" leaves the leading group — the routing prefix — to
		// the request, which is the whole address as far as where it lands.
		j := strings.IndexByte(literal[:i], '[')
		if j < 0 {
			return false
		}
		// With an opener the author wrote an address, brackets and all — but
		// only if they wrote one. Brackets holding anything else pin no host
		// and compose nothing a client can parse, so counting them as
		// author-written text bought a rule that matched every request and
		// redirected to a location no client could follow. Ask whether it
		// parses rather than whether it looks like it might: "[zzz1]" and
		// "[evil.com1]" hold hex digits and are not addresses.
		//
		// Brackets hold an IPv6 address only, so "[127.0.0.1]" is not a host
		// however well it parses on its own. The colon test is what separates
		// the two spellings — an address written as IPv4 has none, and one
		// written as IPv6 always has — since ParseIP alone accepts both, and
		// To4 would reject "[::ffff:127.0.0.1]", which is a legal host.
		inner := literal[j+1 : i]
		return strings.IndexByte(inner, ':') >= 0 && net.ParseIP(inner) != nil
	}
	if i := strings.IndexByte(literal, ':'); i >= 0 {
		literal = literal[:i]
	}
	// Whitespace pins nothing: the parser either deletes it outright or
	// percent-encodes it into a host that fails to parse.
	return strings.Trim(literal, ".[: \t\n\r\v\f") != ""
}

// literalPrefixLen returns how much of a rule's path is pinned before its first
// wildcard, which is what makes one rule more specific than another it overlaps.
func literalPrefixLen(rule string) int {
	if i := strings.IndexByte(rule, '*'); i >= 0 {
		return i
	}
	return len(rule)
}
