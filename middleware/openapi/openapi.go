package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"weak"

	"github.com/gofiber/fiber/v3/internal/urlnorm"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

type segmentEqual = func(a, b string) bool

type appEquality struct {
	app   weak.Pointer[fiber.App]
	equal segmentEqual
}

const maxCachedEntries = 32

type appCache struct {
	uiPages  map[string]cachedBody
	specBody cachedBody
	specRev  uint64
}

type cachedBody struct {
	etag string
	data []byte
}

func newCachedBody(data []byte) cachedBody {
	sum := sha256.Sum256(data)
	return cachedBody{data: data, etag: `"` + hex.EncodeToString(sum[:16]) + `"`}
}

// send writes the body with its entity tag and answers a matching If-None-Match with 304.
func (b cachedBody) send(c fiber.Ctx, contentType string) error {
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderETag, b.etag)
	c.Set(fiber.HeaderCacheControl, "no-cache")
	if c.Fresh() {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.Status(fiber.StatusOK).Send(b.data)
}

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	var (
		cacheMu sync.Mutex
		caches  = make(map[weak.Pointer[fiber.App]]*appCache)
	)

	// cacheFor drops the caches of collected apps and, past the bound, an arbitrary entry. Caller holds cacheMu.
	cacheFor := func(app *fiber.App) *appCache {
		key := weak.Make(app)
		cache, ok := caches[key]
		if !ok {
			cache = &appCache{uiPages: make(map[string]cachedBody)}
			if len(caches) >= maxCachedEntries {
				for stale := range caches {
					if stale.Value() == nil {
						delete(caches, stale)
					}
				}
			}
			if len(caches) >= maxCachedEntries {
				for evicted := range caches {
					delete(caches, evicted)
					break
				}
			}
			caches[key] = cache
		}
		return cache
	}

	specBytes := func(app *fiber.App) (cachedBody, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		rev := app.RoutesRevision()
		if cache.specBody.data == nil || cache.specRev != rev {
			appCfg := app.Config()
			spec := generateSpec(app.GetRoutes(false), &cfg, specEnv{validator: appCfg.StructValidator, equal: segmentEqualFor(&appCfg), strict: appCfg.StrictRouting})
			data, err := appCfg.JSONEncoder(spec)
			if err != nil {
				return cachedBody{}, fmt.Errorf("openapi: marshal spec: %w", err)
			}
			cache.specBody, cache.specRev = newCachedBody(data), rev
		}
		return cache.specBody, nil
	}

	uiBytes := func(app *fiber.App, targetPath string) (cachedBody, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		if body, ok := cache.uiPages[targetPath]; ok {
			return body, nil
		}
		data, err := buildSwaggerUIPage(targetPath, &cfg, app.Config().JSONEncoder)
		if err != nil {
			return cachedBody{}, fmt.Errorf("openapi: build swagger ui page: %w", err)
		}
		body := newCachedBody(data)
		if len(cache.uiPages) >= maxCachedEntries {
			// Keys derive from the request path (attacker-controlled); drop all rather than let junk pin the cache.
			clear(cache.uiPages)
		}
		cache.uiPages[targetPath] = body
		return body, nil
	}

	specPath := utils.TrimRight(normalizedPath(cfg.Path), '/')
	uiPath := utils.TrimRight(normalizedPath(cfg.UIPath), '/')

	var lastEquality atomic.Pointer[appEquality]
	equalityFor := func(app *fiber.App) func(a, b string) bool {
		if e := lastEquality.Load(); e != nil && e.app.Value() == app {
			return e.equal
		}
		appCfg := app.Config()
		equal := segmentEqualFor(&appCfg)
		lastEquality.Store(&appEquality{app: weak.Make(app), equal: equal})
		return equal
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Next()
		}

		request := utils.TrimRight(c.Path(), '/')
		route := c.Route()
		isMiddleware := route != nil && route.IsMiddleware()

		// Fast path: most prefix-mounted requests match neither target.
		if isMiddleware && !mayEndWith(request, specPath) && !mayEndWith(request, uiPath) {
			return c.Next()
		}

		equal := equalityFor(c.App())
		if isMiddleware && !hasSuffix(request, specPath, equal) && !hasSuffix(request, uiPath, equal) {
			return c.Next()
		}

		targets := resolveTargets(c, specPath, uiPath, equal)

		switch {
		case targets.specOK && equal(request, targets.spec):
			body, err := specBytes(c.App())
			if err != nil {
				return err
			}
			return body.send(c, fiber.MIMEApplicationJSONCharsetUTF8)
		case targets.uiOK && equal(request, targets.ui):
			// A rooted path: "//host/..." would be a network-path reference, and a root spec trims to "".
			specURL := urlnorm.RootedPath(targets.spec)
			body, err := uiBytes(c.App(), specURL)
			if err != nil {
				return err
			}
			return body.send(c, fiber.MIMETextHTMLCharsetUTF8)
		default:
			return c.Next()
		}
	}
}

func stringsEqual(a, b string) bool { return a == b }

func segmentEqualFor(cfg *fiber.Config) segmentEqual {
	if cfg.CaseSensitive {
		return stringsEqual
	}
	return utils.EqualFold[string]
}

// mayEndWith is a cheap necessary condition for hasSuffix under any case rule.
func mayEndWith(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	if suffix == "" {
		return true
	}
	last, want := s[len(s)-1], suffix[len(suffix)-1]
	return last == want || last|0x20 == want|0x20
}

func hasSuffix(s, suffix string, equal segmentEqual) bool {
	return len(s) >= len(suffix) && equal(s[len(s)-len(suffix):], suffix)
}

type specTargets struct {
	spec   string
	ui     string
	specOK bool
	uiOK   bool
}

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
			return specTargets{spec: path, specOK: true}
		default:
			return specTargets{}
		}
	}

	prefix := routePrefix(route.Path, c.Path())
	// Optional or greedy segments consume a varying number of segments, so routePrefix cannot tell where the mount ends.
	if resolved, ok := resolveDynamicMountPrefix(route.Path, utils.TrimRight(c.Path(), '/'), specPath, uiPath, equal); ok {
		prefix = resolved
	}
	switch {
	case specPath != "" && hasSuffix(prefix, specPath, equal):
		base := prefix[:len(prefix)-len(specPath)]
		return specTargets{spec: prefix, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
	case uiPath != "" && hasSuffix(prefix, uiPath, equal):
		return specTargets{spec: prefix + specPath, ui: prefix, specOK: true, uiOK: true}
	default:
		return specTargets{spec: prefix + specPath, ui: prefix + uiPath, specOK: true, uiOK: true}
	}
}

func normalizedPath(cfgPath string) string {
	if !strings.HasPrefix(cfgPath, "/") {
		return "/" + cfgPath
	}
	return cfgPath
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

func routePrefix(pattern, requestPath string) string {
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return ""
	}

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
	if prefix, ok := pathPrefixSegments(requestPath, segments); ok {
		return prefix
	}
	return requestPath
}
