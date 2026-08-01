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
		cfg.rulesRegex[regexp.MustCompile(pattern)] = compiledRule{
			target:     v,
			sameOrigin: !targetNamesAuthority(v),
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
			if replacer != nil {
				location := replacer.Replace(rule.target)
				if rule.sameOrigin {
					location = keepSameOrigin(location)
				}
				queryString := utils.UnsafeString(c.RequestCtx().QueryArgs().QueryString())
				if queryString != "" {
					location += "?" + queryString
				}
				return c.Redirect().Status(cfg.StatusCode).To(location)
			}
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
