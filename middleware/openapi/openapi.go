package openapi

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

const (
	// segmentLiteral matches exactly one path segment, with no parameter in it.
	segmentLiteral segmentKind = iota
	// segmentParam is a mixed or whole-segment parameter: one path segment.
	segmentParam
	// segmentOptional matches zero or one segment.
	segmentOptional
	// segmentGreedy ("*" or "+") matches any number of segments.
	segmentGreedy
)

// appEquality is the path comparison an app's CaseSensitive setting selects.
type appEquality struct {
	app   *fiber.App
	equal func(a, b string) bool
}

// maxCachedSwaggerPages bounds the UI page cache so a parameterized mount cannot
// grow it without limit. The same bound applies to the per-app cache map, which evicts when full.
const maxCachedSwaggerPages = 32

// appCache holds one app's artifacts. The spec bytes do not depend on the target
// path so one entry suffices; UI pages embed the spec URL and stay keyed per target.
type appCache struct {
	uiPages  map[string][]byte
	specData []byte
	specRev  uint64
}

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Scoped per *fiber.App: one handler may serve several apps, and one app's
	// revision counter must never validate another's cached bytes.
	var (
		cacheMu sync.Mutex
		caches  = make(map[*fiber.App]*appCache)
	)

	// cacheFor returns the app's cache entry, creating it on first use. Past the
	// bound an arbitrary other entry is evicted, so a handler serving more apps
	// than the bound keeps caching instead of rebuilding on every request. The
	// caller must hold cacheMu.
	cacheFor := func(app *fiber.App) *appCache {
		cache, ok := caches[app]
		if !ok {
			cache = &appCache{uiPages: make(map[string][]byte)}
			if len(caches) >= maxCachedSwaggerPages {
				for evicted := range caches {
					delete(caches, evicted)
					break
				}
			}
			caches[app] = cache
		}
		return cache
	}

	// specBytes returns the cached spec, regenerating when the route revision
	// moved. Unlocked with defer so a panic below cannot wedge the cache.
	specBytes := func(app *fiber.App) ([]byte, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		rev := app.RoutesRevision()
		if cache.specData == nil || cache.specRev != rev {
			// GetRoutes deep-copies under the router lock, so generation never
			// races registration or the documentation helpers.
			appCfg := app.Config()
			equal := utils.EqualFold[string]
			if appCfg.CaseSensitive {
				equal = stringsEqual
			}
			spec := generateSpec(app.GetRoutes(false), &cfg, specEnv{validator: appCfg.StructValidator, equal: equal})
			data, err := appCfg.JSONEncoder(spec)
			if err != nil {
				return nil, fmt.Errorf("openapi: marshal spec: %w", err)
			}
			cache.specData, cache.specRev = data, rev
		}
		return cache.specData, nil
	}

	// uiBytes returns the app's cached Swagger UI page for targetPath, building
	// it on first use. Same defer rationale as specBytes.
	uiBytes := func(app *fiber.App, targetPath string) ([]byte, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		if data, ok := cache.uiPages[targetPath]; ok {
			return data, nil
		}
		data, err := buildSwaggerUIPage(targetPath, &cfg, app.Config().JSONEncoder)
		if err != nil {
			return nil, fmt.Errorf("openapi: build swagger ui page: %w", err)
		}
		if len(cache.uiPages) >= maxCachedSwaggerPages {
			// Keys come from the request path, so they are attacker-controlled;
			// drop them all rather than letting junk pin the cache.
			clear(cache.uiPages)
		}
		cache.uiPages[targetPath] = data
		return data, nil
	}

	specPath := utils.TrimRight(normalizedPath(cfg.Path), '/')
	uiPath := utils.TrimRight(normalizedPath(cfg.UIPath), '/')

	// Config() copies the whole struct, so the case rule is resolved once per
	// app rather than on every request that passes through the middleware.
	var lastEquality atomic.Pointer[appEquality]
	equalityFor := func(app *fiber.App) func(a, b string) bool {
		if e := lastEquality.Load(); e != nil && e.app == app {
			return e.equal
		}
		equal := utils.EqualFold[string]
		if app.Config().CaseSensitive {
			equal = stringsEqual
		}
		lastEquality.Store(&appEquality{app: app, equal: equal})
		return equal
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Next()
		}

		equal := equalityFor(c.App())

		request := utils.TrimRight(c.Path(), '/')
		route := c.Route()
		isMiddleware := route != nil && route.IsMiddleware()

		// Fast path for prefix-mounted middleware: most requests cannot match
		// either target, so skip target resolution without allocating.
		if isMiddleware && !hasSuffix(request, specPath, equal) && !hasSuffix(request, uiPath, equal) {
			return c.Next()
		}

		targets := resolveTargets(c, specPath, uiPath, equal)

		switch {
		case targets.specOK && equal(request, targets.spec):
			// Cached per app and invalidated by the route revision, so changes
			// are reflected without regenerating on every request.
			data, err := specBytes(c.App())
			if err != nil {
				return err
			}

			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		case targets.uiOK && equal(request, targets.ui):
			data, err := uiBytes(c.App(), targets.spec)
			if err != nil {
				return err
			}

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		default:
			return c.Next()
		}
	}
}

func stringsEqual(a, b string) bool { return a == b }

// hasSuffix reports whether s ends with suffix under the given equality
// function (exact or case-folding).
func hasSuffix(s, suffix string, equal func(a, b string) bool) bool {
	return len(s) >= len(suffix) && equal(s[len(s)-len(suffix):], suffix)
}

// specTargets holds the paths this handler answers on. The ok flags are explicit
// because an empty path is meaningful: it is what a root target ("/") trims to.
type specTargets struct {
	spec   string
	ui     string
	specOK bool
	uiOK   bool
}

// resolveTargets derives the spec and UI target paths for this request from the
// route the handler runs on. Prefix middleware serves them under its mount; an
// exact method route is itself the target, its suffix deciding which one.
func resolveTargets(c fiber.Ctx, specPath, uiPath string, equal func(a, b string) bool) specTargets {
	route := c.Route()
	if route == nil {
		return specTargets{spec: specPath, ui: uiPath, specOK: true, uiOK: true}
	}

	if !route.IsMiddleware() {
		// An exact match means the request path IS the registered path, with any
		// pattern parameters already substituted.
		path := utils.TrimRight(c.Path(), '/')
		switch {
		case specPath != "" && hasSuffix(path, specPath, equal):
			base := path[:len(path)-len(specPath)]
			return specTargets{spec: path, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
		case uiPath != "" && hasSuffix(path, uiPath, equal):
			base := path[:len(path)-len(uiPath)]
			return specTargets{spec: base + specPath, ui: path, specOK: true, uiOK: true}
		case len(route.Params) == 0:
			// A fixed custom path (app.Get("/docs", openapi.New())) serves the
			// specification.
			return specTargets{spec: path, specOK: true}
		default:
			// A wildcard registration matches paths the author never enumerated,
			// so serving there would leak the route inventory.
			return specTargets{}
		}
	}

	prefix := routePrefix(route.Path, c.Path())
	// Optional or greedy segments consume a varying number of request segments,
	// so the truncation above cannot say where the mount ends. The request is
	// compared trimmed so a trailing slash still resolves.
	if resolved, ok := resolveDynamicMountPrefix(route.Path, utils.TrimRight(c.Path(), '/'), specPath, uiPath, equal); ok {
		prefix = resolved
	}
	switch {
	case specPath != "" && hasSuffix(prefix, specPath, equal):
		// e.g. app.Use("/v1/openapi.json", openapi.New()): the mount itself is
		// the spec target; the UI, if enabled, sits beside it.
		base := prefix[:len(prefix)-len(specPath)]
		return specTargets{spec: prefix, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
	case uiPath != "" && hasSuffix(prefix, uiPath, equal):
		// e.g. app.Use("/swagger", New()): the mount is the UI, so the spec must
		// stay under it or the page could never load what it points at.
		return specTargets{spec: prefix + specPath, ui: prefix, specOK: true, uiOK: true}
	default:
		return specTargets{spec: prefix + specPath, ui: prefix + uiPath, specOK: true, uiOK: true}
	}
}

// normalizedPath returns cfgPath with a leading slash. Defaults for empty
// paths are applied earlier by configDefault.
func normalizedPath(cfgPath string) string {
	if !strings.HasPrefix(cfgPath, "/") {
		return "/" + cfgPath
	}
	return cfgPath
}

// routeTokens returns a segment's routing-relevant characters, dropping escapes
// and <constraint> spans so neither is mistaken for a routing token.
//
// TODO: replace this re-lexing with the router's parsed segments once exposed.
func routeTokens(seg string) string {
	var b strings.Builder
	inConstraint := false
	for i := 0; i < len(seg); i++ {
		switch ch := seg[i]; {
		case ch == '\\':
			i++ // the escaped character is a literal
		case inConstraint:
			if ch == '>' {
				inConstraint = false
			}
		case ch == '<':
			inConstraint = true
		default:
			_ = b.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// resolveDynamicMountPrefix picks the mount prefix when the pattern has optional
// or greedy segments. Every split the segment bounds allow is tried, shortest
// first, and the one whose remainder is a target wins.
func resolveDynamicMountPrefix(pattern, requestPath, specPath, uiPath string, equal func(a, b string) bool) (string, bool) {
	minSegments, maxSegments, dynamic := prefixSegmentBounds(pattern)
	if !dynamic {
		// A static mount is its own prefix, and its escaped form still needs
		// unescaping, which routePrefix handles.
		return "", false
	}

	if maxSegments < 0 || maxSegments > countPathSegments(requestPath) {
		// A greedy segment is unbounded; no split can consume more segments
		// than the request has.
		maxSegments = countPathSegments(requestPath)
	}

	for n := minSegments; n <= maxSegments; n++ {
		candidate, ok := pathPrefixSegments(requestPath, n)
		if !ok {
			break
		}
		if specPath != "" && equal(candidate+specPath, requestPath) {
			return candidate, true
		}
		if uiPath != "" && equal(candidate+uiPath, requestPath) {
			return candidate, true
		}
	}
	return "", false
}

// prefixSegmentBounds reports how many leading segments the mount can consume and
// whether it is dynamic. A greedy segment makes the maximum unbounded (-1).
// segmentKind classifies a route pattern segment by its routing tokens only,
// so a constraint or an escaped character never reads as a parameter.
type segmentKind uint8

// classifySegment returns the kind of a pattern segment and its routing tokens.
func classifySegment(segment string) (segmentKind, string) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	tokens := routeTokens(segment)
	switch {
	case strings.ContainsAny(tokens, "*+"):
		return segmentGreedy, tokens
	case strings.HasSuffix(tokens, "?"):
		return segmentOptional, tokens
	case strings.Contains(tokens, ":"):
		return segmentParam, tokens
	default:
		return segmentLiteral, tokens
	}
}

func prefixSegmentBounds(pattern string) (minSegments, maxSegments int, dynamic bool) { //nolint:nonamedreturns // three ints and a bool read better named
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return 0, 0, false
	}

	greedy := false
	for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
		kind, tokens := classifySegment(seg)
		switch kind {
		case segmentGreedy:
			// "+" needs at least one segment; "*" may match none.
			if strings.Contains(tokens, "+") {
				minSegments++
			}
			greedy = true
			dynamic = true
		case segmentOptional:
			// Optional: zero or one segment.
			maxSegments++
			dynamic = true
		case segmentParam:
			dynamic = true
			minSegments++
			maxSegments++
		default:
			minSegments++
			maxSegments++
		}
	}

	if greedy {
		return minSegments, -1, true
	}
	return minSegments, maxSegments, dynamic
}

// countPathSegments counts the slash-separated segments of a request path.
func countPathSegments(requestPath string) int {
	trimmed := utils.Trim(requestPath, '/')
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}

// pathPrefixSegments returns the leading n segments of requestPath, reporting
// false when it does not have that many segments to give.
func pathPrefixSegments(requestPath string, n int) (string, bool) {
	if n <= 0 {
		return "", true
	}

	idx := 0
	for range n {
		if idx+1 > len(requestPath) {
			return "", false
		}
		next := strings.IndexByte(requestPath[idx+1:], '/')
		if next < 0 {
			return "", false
		}
		idx += 1 + next
	}
	return requestPath[:idx], true
}

// routePrefix derives the mount prefix of a middleware route. A static prefix is
// the route path unescaped; a parameterized one takes its values from the
// request, ending at the first greedy or optional segment.
func routePrefix(pattern, requestPath string) string {
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return ""
	}

	// Classify segments by routing tokens only: constraints and escaped
	// characters are literals.
	parameterized := false
	segments := 0
	counting := true
	for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
		kind, _ := classifySegment(seg)
		if kind != segmentLiteral {
			parameterized = true
		}
		if counting && (seg == "" || kind == segmentGreedy || kind == segmentOptional) {
			counting = false
		}
		if counting {
			segments++
		}
	}
	if !parameterized {
		// Static prefix: serve it in its unescaped (request) form.
		return utils.TrimRight(fiber.RemoveEscapeChar(pattern), '/')
	}
	if segments == 0 {
		return ""
	}
	// A request that ends inside the prefix (the bare mount path) is its own
	// prefix.
	if prefix, ok := pathPrefixSegments(requestPath, segments); ok {
		return prefix
	}
	return requestPath
}
