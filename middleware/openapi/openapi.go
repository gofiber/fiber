package openapi

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

// segmentEqual compares route segments under the app's case rule.
type segmentEqual = func(a, b string) bool

type appEquality struct {
	app   *fiber.App
	equal segmentEqual
}

// maxCachedEntries bounds both the per-app UI page cache and the app cache map.
const maxCachedEntries = 32

// appCache holds one app's artifacts; UI pages embed the spec URL so they are keyed per target.
type appCache struct {
	uiPages  map[string][]byte
	specData []byte
	specRev  uint64
}

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Scoped per app: one app's revision counter must not validate another's bytes.
	var (
		cacheMu sync.Mutex
		caches  = make(map[*fiber.App]*appCache)
	)

	// cacheFor evicts an arbitrary entry past the bound so many apps keep caching. Caller holds cacheMu.
	cacheFor := func(app *fiber.App) *appCache {
		cache, ok := caches[app]
		if !ok {
			cache = &appCache{uiPages: make(map[string][]byte)}
			if len(caches) >= maxCachedEntries {
				for evicted := range caches {
					delete(caches, evicted)
					break
				}
			}
			caches[app] = cache
		}
		return cache
	}

	// specBytes regenerates when the route revision moved. Deferred unlock so a panic cannot wedge the cache.
	specBytes := func(app *fiber.App) ([]byte, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		rev := app.RoutesRevision()
		if cache.specData == nil || cache.specRev != rev {
			// GetRoutes copies under the router lock, so generation never races registration.
			appCfg := app.Config()
			spec := generateSpec(app.GetRoutes(false), &cfg, specEnv{validator: appCfg.StructValidator, equal: segmentEqualFor(&appCfg)})
			data, err := appCfg.JSONEncoder(spec)
			if err != nil {
				return nil, fmt.Errorf("openapi: marshal spec: %w", err)
			}
			cache.specData, cache.specRev = data, rev
		}
		return cache.specData, nil
	}

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
		if len(cache.uiPages) >= maxCachedEntries {
			// Keys derive from the request path (attacker-controlled); drop all rather than let junk pin the cache.
			clear(cache.uiPages)
		}
		cache.uiPages[targetPath] = data
		return data, nil
	}

	specPath := utils.TrimRight(normalizedPath(cfg.Path), '/')
	uiPath := utils.TrimRight(normalizedPath(cfg.UIPath), '/')

	// Config() copies the whole struct; resolve the case rule once per app.
	var lastEquality atomic.Pointer[appEquality]
	equalityFor := func(app *fiber.App) func(a, b string) bool {
		if e := lastEquality.Load(); e != nil && e.app == app {
			return e.equal
		}
		appCfg := app.Config()
		equal := segmentEqualFor(&appCfg)
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

		// Fast path: most prefix-mounted requests match neither target.
		if isMiddleware && !hasSuffix(request, specPath, equal) && !hasSuffix(request, uiPath, equal) {
			return c.Next()
		}

		targets := resolveTargets(c, specPath, uiPath, equal)

		switch {
		case targets.specOK && equal(request, targets.spec):
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

// segmentEqualFor returns the comparison the router uses for route segments under cfg.
func segmentEqualFor(cfg *fiber.Config) segmentEqual {
	if cfg.CaseSensitive {
		return stringsEqual
	}
	return utils.EqualFold[string]
}

// hasSuffix reports whether s ends with suffix under equal.
func hasSuffix(s, suffix string, equal segmentEqual) bool {
	return len(s) >= len(suffix) && equal(s[len(s)-len(suffix):], suffix)
}

// specTargets holds the paths this handler answers on. The ok flags exist because an empty path is valid (a root target trims to "").
type specTargets struct {
	spec   string
	ui     string
	specOK bool
	uiOK   bool
}

// resolveTargets derives the spec and UI paths from the route the handler runs on.
func resolveTargets(c fiber.Ctx, specPath, uiPath string, equal segmentEqual) specTargets {
	route := c.Route()
	if route == nil {
		return specTargets{spec: specPath, ui: uiPath, specOK: true, uiOK: true}
	}

	if !route.IsMiddleware() {
		path := utils.TrimRight(c.Path(), '/')
		switch {
		case specPath != "" && hasSuffix(path, specPath, equal):
			base := path[:len(path)-len(specPath)]
			return specTargets{spec: path, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
		case uiPath != "" && hasSuffix(path, uiPath, equal):
			base := path[:len(path)-len(uiPath)]
			return specTargets{spec: base + specPath, ui: path, specOK: true, uiOK: true}
		case len(route.Params) == 0:
			// Fixed custom path, e.g. app.Get("/docs", openapi.New()).
			return specTargets{spec: path, specOK: true}
		default:
			// A wildcard matches paths the author never enumerated; serving there would leak the route inventory.
			return specTargets{}
		}
	}

	prefix := routePrefix(route.Path, c.Path())
	// Optional or greedy segments consume a varying number of request segments, so routePrefix cannot tell where the mount ends.
	if resolved, ok := resolveDynamicMountPrefix(route.Path, utils.TrimRight(c.Path(), '/'), specPath, uiPath, equal); ok {
		prefix = resolved
	}
	switch {
	case specPath != "" && hasSuffix(prefix, specPath, equal):
		// The mount itself is the spec target; the UI sits beside it.
		base := prefix[:len(prefix)-len(specPath)]
		return specTargets{spec: prefix, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
	case uiPath != "" && hasSuffix(prefix, uiPath, equal):
		// The mount is the UI, so the spec must stay under it for the page to load.
		return specTargets{spec: prefix + specPath, ui: prefix, specOK: true, uiOK: true}
	default:
		return specTargets{spec: prefix + specPath, ui: prefix + uiPath, specOK: true, uiOK: true}
	}
}

// normalizedPath returns cfgPath with a leading slash.
func normalizedPath(cfgPath string) string {
	if !strings.HasPrefix(cfgPath, "/") {
		return "/" + cfgPath
	}
	return cfgPath
}

// prefixSegmentBounds reports the min/max segments a mount can consume (-1 max when greedy) and whether it is dynamic.
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

func countPathSegments(requestPath string) int {
	trimmed := utils.Trim(requestPath, '/')
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}

// pathPrefixSegments returns the leading n segments of requestPath, or false if it has fewer.
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

// routePrefix derives a middleware route's mount prefix, taking parameter values from the request and stopping at the first greedy or optional segment.
func routePrefix(pattern, requestPath string) string {
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return ""
	}

	// Constraints and escaped characters are literals; classify by routing tokens only.
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
		return utils.TrimRight(fiber.RemoveEscapeChar(pattern), '/')
	}
	if segments == 0 {
		return ""
	}
	// A request ending inside the prefix is its own prefix.
	if prefix, ok := pathPrefixSegments(requestPath, segments); ok {
		return prefix
	}
	return requestPath
}
