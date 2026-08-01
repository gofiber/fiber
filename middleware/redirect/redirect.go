package redirect

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// compiledRule is one configured rule with its target and the decision, made
// once at construction, of whether that target picks its own destination.
type compiledRule struct {
	target string
	// authorityStart and authorityEnd bound the target's own authority — the
	// span that decides which host the redirect reaches. Both are 0 when the
	// target names no authority.
	authorityStart int
	authorityEnd   int
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
		start, end := authoritySpan(v)
		cfg.rulesRegex[regexp.MustCompile(pattern)] = compiledRule{
			target:         v,
			authorityStart: start,
			authorityEnd:   end,
			sameOrigin:     !targetNamesAuthority(v),
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
			replacer := captureTokens(k, c.Path())
			if replacer == nil {
				continue
			}

			// Substitute the target's own authority separately from the rest.
			// A target may put a capture inside it — "https://$1.cdn.example.com"
			// is a plausible way to route per tenant — and the author means $1
			// to be a label, not a whole URL. A capture holding "evil.com/x"
			// composes "https://evil.com/x.cdn.example.com", whose authority
			// ends at that slash: the browser goes to evil.com.
			//
			// The template's authority span contains none of the four bytes
			// that end an authority, by definition of where it ends, so any
			// that show up after substitution came from a capture and would cut
			// the authority short. Refuse the rule rather than guess what the
			// author meant; the request falls through to the app.
			authority := replacer.Replace(rule.target[rule.authorityStart:rule.authorityEnd])
			if strings.ContainsAny(authority, `/\?#`) {
				continue
			}
			location := asBrowserReads(
				rule.target[:rule.authorityStart] + authority + replacer.Replace(rule.target[rule.authorityEnd:]),
			)

			// Normalize on every branch, not just the guarded one. The bytes
			// asBrowserReads removes are never meaningful in a URL and the
			// client drops them anyway, so an author-configured absolute target
			// loses nothing — and the guard below then always runs on the
			// location as it will actually be read.
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
