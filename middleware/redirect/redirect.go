package redirect

import (
	"regexp"
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
	target string
	// authorityChunks splits the target's own authority — the span that decides
	// which host the redirect reaches — into literal text and "$N" tokens, so
	// each substituted value can be judged by where it lands. Empty when the
	// authority holds no placeholder and so cannot be moved by a request.
	authorityChunks []authorityChunk
	// authorityEndsTarget is set when the authority runs to the end of the
	// target, so a token closing it has nothing after it and may open the next
	// component itself. Where author text follows ("https://host:$1/health")
	// the token is bounded and only gets the stricter content check.
	authorityEndsTarget bool
	// sameOrigin is set when the target names no authority of its own. The "$N"
	// values spliced into such a target come from the request path, so they must
	// not be able to introduce one.
	sameOrigin bool
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Initialize
	cfg.rulesRegex = map[*regexp.Regexp]compiledRule{}
	for k, v := range cfg.Rules {
		pattern := strings.ReplaceAll(k, "*", "(.*)")
		// Anchor both ends so a rule matches the whole path. Without the leading
		// "^" the pattern matches any suffix, so a request can be redirected by a
		// rule whose path it only happens to end with (e.g. "/old" would also
		// redirect "/very/old"). See issue #4476.
		pattern = "^" + pattern + "$"
		chunks, authorityEnd := authorityChunks(v)

		switch {
		case targetLetsRequestPickHost(v, chunks):
			// The request picks the host this redirects to. That is an open
			// redirect by construction — nothing here can distinguish the
			// intended destination from an attacker's — and it is only reachable
			// because the author wrote it, so say so once at startup rather than
			// silently refusing to honor the rule at request time.
			log.Warnf("[REDIRECT] rule %q sends the client to a host taken from the request path; "+
				"anyone who can shape the path can choose the redirect target", k)
		case authorityEndsInOpenCapture(chunks, authorityEnd == len(v)):
			// The target's host ends in a capture with nothing pinned after it,
			// so a value like ".evil.com" would extend the host into a domain
			// the author does not control. Such a value is refused per request,
			// which without this would look like the rule quietly not firing.
			log.Warnf("[REDIRECT] rule %q ends in a capture inside its host, so only a value that cannot "+
				"extend that host is honored — one opening a path, query or fragment. Pin what follows the "+
				"capture in the target to redirect on every value", k)
		}

		cfg.rulesRegex[regexp.MustCompile(pattern)] = compiledRule{
			target:              v,
			authorityChunks:     chunks,
			authorityEndsTarget: authorityEnd == len(v),
			sameOrigin:          !targetNamesAuthority(v),
		}
	}

	// Middleware function
	return func(c fiber.Ctx) error {
		// Next request to skip middleware
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}
		// Rewrite
		for k, rule := range cfg.rulesRegex {
			replacer, values := captureTokens(k, c.Path())
			if replacer == nil {
				continue
			}

			// A target may put a capture inside its own authority —
			// "https://$1.cdn.example.com" is a plausible way to route per
			// tenant — and the author means $1 to be a label, not a whole URL.
			// Refuse the rule when a value would move the host, rather than
			// guess what the author meant; the request falls through to the app.
			if !authorityHolds(rule.authorityChunks, rule.authorityEndsTarget, values) {
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

	if offset := strings.IndexAny(target[start:], `/\?#`); offset >= 0 {
		return start, start + offset
	}
	return start, len(target)
}

// authorityChunks splits target's own authority into literal text and "$N"
// tokens. It returns nil when the authority holds no token, which is the common
// case and means no request can move the host.
// It also returns where that authority ends, so the caller does not recompute
// the span and the two cannot drift apart.
func authorityChunks(target string) (chunks []authorityChunk, authorityEnd int) { //nolint:nonamedreturns // the pair is a value and the offset it came from; names say which is which
	start, end := authoritySpan(target)
	authority := target[start:end]

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
		return nil, end
	}
	if literal < len(authority) {
		chunks = append(chunks, authorityChunk{text: authority[literal:]})
	}
	return chunks, end
}

// authorityHolds reports whether the values about to be substituted leave the
// target's authority naming the host the author wrote.
//
// Where a token sits decides what it may contain:
//
//   - Something in the target follows it — more authority
//     ("https://$1.cdn.example.com") or a path ("https://cdn.example.com:$1/health").
//     Either way the value is bounded and becomes part of the authority, so it
//     must not carry a byte that ends the authority ("/", "\", "?", "#"),
//     starts a new host by making everything before it userinfo ("@"), or opens
//     a port (":"). Without that check a value of "evil.com/x" composes
//     "https://evil.com/x.cdn.example.com", whose authority stops at the slash —
//     the browser goes to evil.com.
//   - It ends the target ("https://cdn.example.com$1"): the author left the rest
//     of the URL to the request, so the value must open the next component —
//     "/", "\", "?" or "#" — or be empty. Anything else extends the host the
//     author wrote, and "@evil.com" would turn all of it into userinfo.
//   - It is the whole authority ("https://$1"): the author handed over the
//     destination deliberately, so nothing is checked. New warns about it.
func authorityHolds(chunks []authorityChunk, endsTarget bool, values []string) bool {
	if len(chunks) <= 1 {
		return true
	}

	for i, chunk := range chunks {
		if !chunk.placeholder {
			continue
		}
		value, ok := captureValue(chunk.text, values)
		if !ok {
			// A token the pattern has no group for stays literal in the
			// location, so it cannot carry anything from the request.
			continue
		}
		if endsTarget && i == len(chunks)-1 {
			switch {
			case value == "":
			case strings.IndexByte(`/\?#`, value[0]) >= 0:
				// Opens the next component, so the authority ended at the
				// author's own text.
			case i > 0 && strings.HasSuffix(chunks[i-1].text, ":") && isAllDigits(value):
				// A port. The author wrote the colon, and the URL parser rejects
				// a port holding anything but digits outright, so digits are the
				// only value that can be what they meant.
			default:
				return false
			}
			continue
		}
		if strings.ContainsAny(value, `/\?#@:`) {
			return false
		}
	}
	return true
}

// targetLetsRequestPickHost reports whether the request, not the author, decides
// the host a target reaches.
//
// Two spellings do that. "https://$1" and "//$1" hand over the whole authority.
// "https:$1" has no "//" at all, so it names no authority for authorityChunks to
// guard and no origin for keepSameOrigin to hold it to — yet a value of
// "//evil.com" still composes "https://evil.com". Both are open redirects the
// author wrote deliberately, and both are worth saying out loud once.
func targetLetsRequestPickHost(target string, chunks []authorityChunk) bool {
	if len(chunks) == 1 && chunks[0].placeholder {
		return true
	}
	if chunks != nil {
		return false
	}

	// No authority span. Only a scheme with nothing but the request after it
	// can still reach a host of the request's choosing.
	i := schemeEnd(target)
	if i <= 0 || strings.HasPrefix(target[i+1:], "//") {
		return false
	}
	return containsPlaceholder(target[i+1:])
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
// It also returns the captured values themselves, so authorityHolds can judge
// one without running the Replacer over its token a second time.
func captureTokens(pattern *regexp.Regexp, input string) (replacer *strings.Replacer, values []string) { //nolint:nonamedreturns // the pair is a replacer and the values behind it; names say which is which
	if len(input) > 1 {
		input = utils.TrimRight(input, '/')
	}
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil, nil
	}
	values = groups[0][1:]
	replace := make([]string, 2*len(values))
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	return strings.NewReplacer(replace...), values
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

// authorityEndsInOpenCapture reports whether a target's host ends in a capture
// with nothing pinned after it, so a value can extend that host into a domain
// the author does not control.
//
// A capture right after the port colon does not count: a port cannot extend a
// host, and authorityHolds accepts a digit run there, so the rule still fires
// for every value that could have been meant.
func authorityEndsInOpenCapture(chunks []authorityChunk, endsTarget bool) bool {
	if !endsTarget || len(chunks) == 0 {
		return false
	}
	last := len(chunks) - 1
	if last == 0 || !chunks[last].placeholder {
		// A single-chunk authority is the whole-authority-is-a-capture shape,
		// which targetLetsRequestPickHost has already reported.
		return false
	}
	return !strings.HasSuffix(chunks[last-1].text, ":")
}

// captureValue returns the value a "$N" token stands for, or false when the
// pattern captured no such group and the token is left as written.
func captureValue(token string, values []string) (string, bool) {
	n, err := strconv.Atoi(token[1:]) // token is "$" followed by digits
	if err != nil || n < 1 || n > len(values) {
		return "", false
	}
	return values[n-1], true
}
