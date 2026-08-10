// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"math/bits"
	"slices"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/urlnorm"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/gofiber/utils/v2/swar"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// Router defines all router handle interface, including app and group router.
type Router interface {
	Use(args ...any) Router

	Get(path string, handler any, handlers ...any) Router
	Head(path string, handler any, handlers ...any) Router
	Post(path string, handler any, handlers ...any) Router
	Put(path string, handler any, handlers ...any) Router
	Delete(path string, handler any, handlers ...any) Router
	Connect(path string, handler any, handlers ...any) Router
	Options(path string, handler any, handlers ...any) Router
	Trace(path string, handler any, handlers ...any) Router
	Patch(path string, handler any, handlers ...any) Router
	Query(path string, handler any, handlers ...any) Router

	Add(methods []string, path string, handler any, handlers ...any) Router
	All(path string, handler any, handlers ...any) Router

	Group(prefix string, handlers ...any) Router

	Domain(host string) Router

	RouteChain(path string) Register
	Route(prefix string, fn func(router Router), name ...string) Router

	Name(name string) Router
}

// Route is a struct that holds all metadata for each registered handler.
//
//nolint:govet // fieldalignment: the router's scan dictates this order, see below
type Route struct {
	// ### important: always keep in sync with the copy method "app.copyRoute" and all creations of Route struct ###
	//
	// Field order is load-bearing. App.next scans a bucket of routes and
	// discards most of them, so the fields it needs to do that lead the struct
	// and a rejected candidate costs one cache line instead of touching the
	// whole thing. prefix/prefixMask come first because they alone reject the
	// common case; path, Params' length, the routing flags and routeParser's
	// slash bounds follow for the candidates that survive. Keep new routing
	// state up here and everything else below routeParser.

	prefix     uint64 // leading path bytes packed little-endian, see buildPrefixFilter
	prefixMask uint64 // covers the bytes of prefix that are known constants; 0 disables the filter

	path string // Prettified path

	Params []string `json:"params"` // Case-sensitive param keys

	// Data for routing
	use           bool // USE matches path prefixes
	mount         bool // Indicated a mounted app on a specific route
	star          bool // Path equals '*'
	root          bool // Path equals '/'
	autoHead      bool // Automatically generated HEAD route
	caseSensitive bool // Whether parameter matching is case-sensitive

	routeParser routeParser // Parameter parser

	Handlers []Handler `json:"-"` // Ctx handlers

	group *Group // Group instance. used for routes in groups

	// Public fields
	Method string `json:"method"` // HTTP method
	Name   string `json:"name"`   // Route's name
	//nolint:revive // Having both a Path (uppercase) and a path (lowercase) is fine
	Path string `json:"path"` // Original registered route path
}

var (
	defaultGreedyParameterKeys    = []string{"*", "+"}
	preferredPlusGreedyParameters = []string{"+", "*"}
)

// routeTree is a flat, open-addressed index from a tree-path hash to the route
// bucket of a single HTTP method. It serves the request-time lookup that used
// to go through treeStack's map[int][]*Route, where hashing the key and walking
// the bucket group accounted for ~14% of a routed request in CPU profiles.
// Here the lookup is a multiply-shift plus a short linear probe, and a miss
// resolves to the globals bucket inline instead of costing a second lookup.
type routeTree struct {
	// hashes is a power-of-two sized probe table holding the tree-path hash of
	// each slot, 0 meaning free. It is kept apart from the buckets so a probe
	// walks 4 bytes per slot (16 per cache line) instead of a slice header.
	// nil when the method has no prefixed bucket and every request hits globals.
	hashes []int32
	// buckets is parallel to hashes and holds the routes of each occupied slot
	buckets [][]*Route
	// globals is the bucket for tree hash 0, which is also where misses go
	globals []*Route
	// mask is len(hashes)-1, used to wrap the probe
	mask uint32
	// shift is 32-log2(len(hashes)), so slot keeps the high bits of the
	// multiply. A fixed shift would cap the index at whatever it exposes and
	// leave larger tables with an unreachable upper half.
	shift uint32
}

// routeTreeHashMul is the golden-ratio constant used to spread tree hashes,
// whose low bits are just the third path byte, across the probe table.
const routeTreeHashMul = 0x9E3779B1

// buildRouteTree converts one method's tree-path buckets into a routeTree.
//
// It returns a pointer, and allocates rather than recycling the previous
// tables, so that publishing a rebuilt tree is a single pointer store. Holding
// the tree by value instead cost a five-word store, which let a reader pair the
// new hash table with the old buckets; recycling the tables let a reader see a
// cleared one and fall through to globals. Both were measurable — 24 wrong
// answers in 20k requests served across a RebuildTree loop, against 0 before
// the index existed — so this restores the parity that assigning treeStack's
// map pointer used to provide.
//
// The buckets are freshly allocated for each build so a published tree remains
// immutable while a request scans it. Publishing is still an unsynchronized
// store, however, so this does not make RebuildTree safe for concurrent use;
// RebuildTree's own doc states the contract.
func buildRouteTree(buckets map[int][]*Route) *routeTree {
	tree := &routeTree{globals: buckets[0]}

	prefixed := len(buckets)
	if _, ok := buckets[0]; ok {
		prefixed--
	}
	if prefixed == 0 {
		return tree
	}

	// Keep the table at most half full: probes stay short, and a free slot is
	// always reachable, which is what terminates the loop in lookup.
	size := 8
	for size < prefixed*2 {
		size *= 2
	}

	tree.hashes = make([]int32, size)
	tree.buckets = make([][]*Route, size)
	tree.mask = uint32(size - 1) //nolint:gosec // G115 - size is a small power of two
	// size is a power of two, so its trailing-zero count is log2(size).
	tree.shift = uint32(32 - bits.TrailingZeros32(uint32(size))) //nolint:gosec // G115 - size is a small power of two

	for hash, routes := range buckets {
		if hash == 0 {
			continue
		}
		i := tree.slot(hash)
		for tree.hashes[i] != 0 {
			i = (i + 1) & tree.mask
		}
		tree.hashes[i] = int32(hash) //nolint:gosec // G115 - tree hashes are 24-bit, built from three path bytes
		tree.buckets[i] = routes
	}

	return tree
}

// slot returns the probe start index for a tree-path hash.
//
// Fibonacci hashing: the multiply pushes the entropy of the low bytes up into
// the high bits, so the index has to be taken from the top. Shifting by a fixed
// amount and masking cannot do that -- it bounds the index by whatever the
// shift leaves, which for a table bigger than that bound makes the upper half
// unreachable as a probe start and collapses the lower half into a linear scan.
func (t *routeTree) slot(hash int) uint32 {
	return uint32(hash) * routeTreeHashMul >> t.shift //nolint:gosec // G115 - tree hashes are 24-bit, built from three path bytes
}

// lookup returns the route bucket for a tree-path hash, falling back to the
// globals bucket when that hash has no bucket of its own.
func (t *routeTree) lookup(hash int) []*Route {
	if hash == 0 || t.hashes == nil {
		return t.globals
	}

	want := int32(hash) //nolint:gosec // G115 - tree hashes are 24-bit, built from three path bytes
	for i := t.slot(hash); ; i = (i + 1) & t.mask {
		switch t.hashes[i] {
		case want:
			return t.buckets[i]
		case 0:
			return t.globals
		}
	}
}

// URL generates a URL from the route path and parameters.
// This method fills in the route parameters with the provided values.
// Parameter matching respects the app's CaseSensitive configuration:
// case-insensitive by default, case-sensitive when CaseSensitive is true.
//
// Example:
//
//	app.Get("/user/:name/:id", handler).Name("user")
//	url, err := app.GetRoute("user").URL(Map{"name": "john", "id": "123"})
//	// Returns: "/user/john/123"
//
//nolint:gocritic // hugeParam: app.GetRoute returns a value, so URL must be callable on that value directly.
func (r Route) URL(params Map) (string, error) {
	if r.Path == "" {
		return "", ErrNotFound
	}

	return buildRouteURL(&r, params)
}

// buildRouteURL generates a URL from route segments and parameters.
// This shared helper is used by both Route.URL() and DefaultRes.getLocationFromRoute()
// to ensure consistent URL generation behavior across APIs.
//
// Parameter resolution uses a deterministic three-step lookup:
//  1. Exact key match on segment.ParamName
//  2. Case-insensitive fallback picking the lexicographically-smallest matching key (when !caseSensitive)
//  3. Greedy parameter fallback for wildcard (*) and plus (+) parameters
func buildRouteURL(route *Route, params Map) (string, error) {
	// No relative URL names such a route. Two leading slashes open an authority,
	// so "//internal" resolves to the host "internal" rather than a path here,
	// and one leading slash reaches a different route. Say so instead of
	// composing either.
	//
	// Judged after the parser's own input handling, since that is what a client
	// applies to whatever is composed: a tab, CR or LF is deleted anywhere in the
	// URL, and a run of controls or spaces is stripped from either end. So a
	// route registered at "/\t/internal" reads as "//internal", and one at
	// "/internal " — reachable through "/internal%20" under UnescapePath — reads
	// as "/internal", a different route. No URL names either.
	normalized := urlnorm.AsBrowserReads(route.Path)
	if normalized != route.Path || urlnorm.LeadingSlashes(normalized) > 1 {
		return "", ErrRouteNotRepresentable
	}

	if len(route.routeParser.segs) == 0 {
		return urlnorm.RootedPath(route.Path), nil
	}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	for _, segment := range route.routeParser.segs {
		if !segment.IsParam {
			_, err := buf.WriteString(segment.Const)
			if err != nil {
				return "", fmt.Errorf("failed to write string: %w", err)
			}
			continue
		}

		var (
			val   any
			found bool
		)

		// Prefer an exact parameter name match
		if val, found = params[segment.ParamName]; !found && !route.caseSensitive {
			// Fall back to a case-insensitive match using a deterministic winner
			var matchedKey string
			foundMatch := false
			for key := range params {
				if utils.EqualFold(key, segment.ParamName) && (!foundMatch || key < matchedKey) {
					matchedKey = key
					foundMatch = true
				}
			}
			if foundMatch {
				val = params[matchedKey]
				found = true
			}
		}

		// For greedy parameters, fall back to generic greedy keys
		if !found && segment.IsGreedy {
			for _, greedyKey := range preferredGreedyParameters(segment.ParamName) {
				if val, found = params[greedyKey]; found {
					break
				}
			}
		}

		if found {
			_, err := buf.WriteString(utils.ToString(val))
			if err != nil {
				return "", fmt.Errorf("failed to write string: %w", err)
			}
		}
	}

	return urlnorm.RootedPath(buf.String()), nil
}

// preferredGreedyParameters returns the generic greedy fallback lookup order
// for a route parameter name.
// Parameter names starting with '+' prefer '+' before '*', names starting with
// '*' prefer '*' before '+', and all other names fall back to the default order.
func preferredGreedyParameters(paramName string) []string {
	if paramName != "" {
		switch paramName[0] {
		case plusParam:
			return preferredPlusGreedyParameters
		case wildcardParam:
			return defaultGreedyParameterKeys
		}
	}

	return defaultGreedyParameterKeys
}

// pathHeadWord packs the first swar.WordLen bytes of a detection path into a
// little-endian word, zero-padded when the path is shorter, so a route's
// precomputed prefix can be tested with a single masked compare.
func pathHeadWord(s string) uint64 {
	if len(s) >= swar.WordLen {
		return swar.Load8(s, 0)
	}

	var word uint64
	for i := range len(s) {
		word |= uint64(s[i]) << (8 * i)
	}
	return word
}

// buildPrefixFilter precomputes the leading-byte filter that App.next uses to
// discard routes before paying for Route.match.
//
// The filter mirrors the first comparison each branch of match would make, so
// it can only reject what match would reject:
//   - a route without parameters is compared against r.path in full (exact for
//     an endpoint, as a prefix for middleware), so r.path leads it
//   - a parametric route enters getMatch, which requires the constant part of
//     the first segment; an optional trailing slash may be dropped there, so
//     that byte is left out
//   - '*' matches every path, and a first segment that is itself a parameter
//     constrains nothing, so both disable the filter
//
// The star check has to come first, before the parametric branch. star is
// derived from the unescaped path (isStar in register), while Params comes from
// parsing the escaped one, so a route registered as `/\*` arrives here with
// star set and no params — and match returns true for it unconditionally.
//
// Anything that changes a route's path, params or parser must run this again.
// The three places that can are register, which builds the route, copyRoute,
// which carries the fields over, and addPrefixToRoute, which rewrites them.
// Test_Route_PrefixFilter_NotStale pins that the set is complete.
//
// It is deliberately not recomputed in buildTree. buildTree runs from
// RebuildTree, whose whole purpose is registering routes on a live app, and the
// routes it would write to are the ones in-flight requests are reading: a
// reader that caught this function between setting prefix and narrowing
// prefixMask compared a short prefix under an all-ones mask and rejected a
// route it matches. That measured 178 spurious misses in 22.8M requests served
// across a rebuild loop, against 0 before the filter existed.
func (r *Route) buildPrefixFilter() {
	r.prefix, r.prefixMask = computePrefixFilter(r)
}

// computePrefixFilter derives a route's leading-byte filter without publishing
// it, so the two words can be installed in one assignment and the result can be
// recomputed and compared in tests. It returns the packed prefix word and the
// mask covering its known bytes.
//
//nolint:nonamedreturns // the pair is easier to read named than by position
func computePrefixFilter(r *Route) (word, mask uint64) {
	if r.star {
		return 0, 0
	}

	prefix := r.path
	if len(r.Params) > 0 {
		segs := r.routeParser.segs
		if len(segs) == 0 || segs[0].IsParam {
			return 0, 0
		}
		prefix = segs[0].Const
		if segs[0].HasOptionalSlash {
			prefix = prefix[:len(prefix)-1]
		}
	}

	if len(prefix) > swar.WordLen {
		prefix = prefix[:swar.WordLen]
	}
	if prefix == "" {
		return 0, 0
	}

	mask = ^uint64(0)
	if n := len(prefix); n < swar.WordLen {
		mask = uint64(1)<<(8*n) - 1
	}
	return pathHeadWord(prefix), mask
}

// prefixRejects reports whether the leading bytes of a detection path, packed
// by pathHeadWord, already rule this route out.
//
// This is the whole of the leading-byte filter, and it lives here rather than
// inline at the scan sites so there is one copy to change and one copy for the
// differential tests to guard. Six hand-written copies could drift from each
// other, and a test asserting against a seventh would not notice.
func (r *Route) prefixRejects(head uint64) bool {
	return (head^r.prefix)&r.prefixMask != 0
}

func (r *Route) match(detectionPath, path string, params *[maxParams]string, pathSlashes int) bool {
	// root detectionPath check
	if r.root && len(detectionPath) == 1 && detectionPath[0] == '/' {
		return true
	}

	// '*' wildcard matches any detectionPath
	if r.star {
		if len(path) > 1 {
			params[0] = path[1:]
		} else {
			params[0] = ""
		}
		return true
	}

	// Does this route have parameters?
	if len(r.Params) > 0 {
		// Quick-reject on the precomputed slash-count bounds before walking segments.
		// pathSlashes 0 means the count is unknown and the filter must stay out of
		// the way; prefix (use) routes may extend past the pattern, so only the
		// lower bound applies to them.
		p := &r.routeParser
		if pathSlashes > 0 && (pathSlashes < int(p.minSlashes) || (!r.use && p.maxBounded && pathSlashes > int(p.maxSlashes))) {
			return false
		}
		// Match params using precomputed routeParser
		return p.getMatch(detectionPath, path, params, r.use)
	}

	// Middleware route?
	if r.use {
		// Single slash or prefix match
		plen := len(r.path)
		if r.root {
			// If r.root is '/', it matches everything starting at '/'
			if detectionPath != "" && detectionPath[0] == '/' {
				return true
			}
		} else if len(detectionPath) >= plen && detectionPath[:plen] == r.path {
			if hasPartialMatchBoundary(detectionPath, plen) {
				return true
			}
		}
	} else if len(r.path) == len(detectionPath) && detectionPath == r.path {
		// Check exact match
		return true
	}

	// No match
	return false
}

func (app *App) next(c *DefaultCtx) (bool, error) {
	methodInt := c.methodInt
	treeHash := c.treePathHash
	detectionPath := utils.UnsafeString(c.detectionPath)
	path := utils.UnsafeString(c.path)
	// Get the route bucket for this method and tree path
	tree := app.treeIndex[methodInt].lookup(treeHash)
	head := pathHeadWord(detectionPath)
	indexRoute := max(c.indexRoute+1, 0)
	// Hoist loop invariants: route.match takes &c.values, so these would reload each iteration.
	pathSlashes := c.pathSlashCount(app)
	firstMatchIndex := c.firstMatchIndex
	skipNonUse := c.shouldSkipNonUseRoutes
	skipHasParamUse := app.skip.hasParamUse

	// Loop over the route stack starting from previous index;
	// the clamp above plus the len(tree) guard keep tree[indexRoute] bounds-check free
	for ; indexRoute < len(tree); indexRoute++ {
		// Get *Route
		route := tree[indexRoute]

		// Reject on the leading path bytes before touching the rest of the route
		if route.prefixRejects(head) {
			continue
		}

		if route.mount {
			continue
		}

		// Lookahead pre-resolved the endpoint: skip endpoints already ruled out; middleware still runs.
		if firstMatchIndex >= 0 && !route.use {
			if indexRoute < firstMatchIndex {
				continue
			}
			// Reuse the lookahead's params unless param/wildcard middleware may have clobbered them.
			if indexRoute == firstMatchIndex && !skipHasParamUse && !skipNonUse {
				c.route = route
				c.isMatched = true
				if len(route.Handlers) > 0 {
					c.indexHandler = 0
					c.indexRoute = indexRoute
					return true, route.Handlers[0](c)
				}
				return true, nil
			}
		}

		// Check if it matches the request path
		if !route.match(detectionPath, path, &c.values, pathSlashes) {
			continue
		}

		if skipNonUse && !route.use {
			continue
		}

		// Pass route reference and param values
		c.route = route
		// Non use handler matched
		if !route.use {
			c.isMatched = true
		}
		// Execute first handler of route
		if len(route.Handlers) > 0 {
			c.indexHandler = 0
			c.indexRoute = indexRoute
			return true, route.Handlers[0](c)
		}

		return true, nil // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if c.shouldSkipNonUseRoutes {
		return false, nil
	}

	if c.isMatched {
		return false, ErrNotFound
	}

	exists := false
	methods := app.config.RequestMethods
	prune := app.skip.methodMaskValid
	routeMethods := app.skip.routeMethods
	for i := range methods {
		// Skip original method
		if methodInt == i {
			continue
		}
		// Methods with no non-use route can never add an Allow entry.
		if prune && routeMethods&(uint64(1)<<i) == 0 {
			continue
		}
		// Reset stack index
		indexRoute := -1

		tree := app.treeIndex[i].lookup(treeHash)
		// Get stack length
		lenr := len(tree) - 1
		// Loop over the route stack starting from previous index
		for indexRoute < lenr {
			// Increment route index
			indexRoute++
			// Get *Route
			route := tree[indexRoute]
			// Skip use routes and routes the leading path bytes already rule out
			if route.use || route.prefixRejects(head) {
				continue
			}
			// Check if it matches the request path
			// No match, next route
			if route.match(detectionPath, path, &c.values, pathSlashes) {
				// We matched
				exists = true
				// Add method to Allow header
				c.Append(HeaderAllow, methods[i])
				// Break stack loop
				break
			}
		}
		c.indexRoute = indexRoute
	}
	if exists {
		return false, ErrMethodNotAllowed
	}
	return false, ErrNotFound
}

func (app *App) nextCustom(c CustomCtx) (bool, error) {
	methodInt := c.getMethodInt()
	treeHash := c.getTreePathHash()
	// Get the route bucket for this method and tree path
	tree := app.treeIndex[methodInt].lookup(treeHash)
	indexRoute := max(c.getIndexRoute()+1, 0)
	// Hoist loop-invariant accessors; nothing changes mid-loop (Next()/RestartRouting re-enter with fresh reads).
	detectionPath := c.getDetectionPath()
	head := pathHeadWord(detectionPath)
	path := c.Path()
	values := c.getValues()
	pathSlashes := c.pathSlashCount(app)
	firstMatchIndex := c.getFirstMatchIndex()
	skipNonUse := c.getSkipNonUseRoutes()
	skipHasParamUse := app.skip.hasParamUse

	// Loop over the route stack starting from previous index;
	// the clamp above plus the len(tree) guard keep tree[indexRoute] bounds-check free
	for ; indexRoute < len(tree); indexRoute++ {
		// Get *Route
		route := tree[indexRoute]

		// Reject on the leading path bytes before touching the rest of the route
		if route.prefixRejects(head) {
			continue
		}

		if route.mount {
			continue
		}

		// Lookahead pre-resolved the endpoint: skip endpoints already ruled out; middleware still runs.
		if firstMatchIndex >= 0 && !route.use {
			if indexRoute < firstMatchIndex {
				continue
			}
			// Reuse the lookahead's params unless param/wildcard middleware may have clobbered them.
			if indexRoute == firstMatchIndex && !skipHasParamUse && !skipNonUse {
				c.setRoute(route)
				c.setMatched(true)
				if len(route.Handlers) > 0 {
					c.setIndexHandler(0)
					c.setIndexRoute(indexRoute)
					return true, route.Handlers[0](c)
				}
				return true, nil
			}
		}

		// Check if it matches the request path
		if !route.match(detectionPath, path, values, pathSlashes) {
			continue
		}
		if skipNonUse && !route.use {
			continue
		}

		// Pass route reference and param values
		c.setRoute(route)
		// Non use handler matched
		if !route.use {
			c.setMatched(true)
		}
		// Execute first handler of route
		if len(route.Handlers) > 0 {
			c.setIndexHandler(0)
			c.setIndexRoute(indexRoute)
			return true, route.Handlers[0](c)
		}
		return true, nil // Stop scanning the stack
	}

	// If c.Next() does not match, return 404
	// If no match, scan stack again if other methods match the request
	// Moved from app.handler because middleware may break the route chain
	if c.getSkipNonUseRoutes() {
		return false, nil
	}

	if c.getMatched() {
		return false, ErrNotFound
	}

	exists := false
	methods := app.config.RequestMethods
	prune := app.skip.methodMaskValid
	routeMethods := app.skip.routeMethods
	for i := range methods {
		// Skip original method
		if methodInt == i {
			continue
		}
		// Methods with no non-use route can never add an Allow entry.
		if prune && routeMethods&(uint64(1)<<i) == 0 {
			continue
		}
		// Reset stack index
		indexRoute := -1

		tree := app.treeIndex[i].lookup(treeHash)
		// Get stack length
		lenr := len(tree) - 1
		// Loop over the route stack starting from previous index
		for indexRoute < lenr {
			// Increment route index
			indexRoute++
			// Get *Route
			route := tree[indexRoute]
			// Skip use routes and routes the leading path bytes already rule out
			if route.use || route.prefixRejects(head) {
				continue
			}
			// Check if it matches the request path
			// No match, next route
			if route.match(detectionPath, path, values, pathSlashes) {
				// We matched
				exists = true
				// Add method to Allow header
				c.Append(HeaderAllow, methods[i])
				// Break stack loop
				break
			}
		}
		c.setIndexRoute(indexRoute)
	}
	if exists {
		return false, ErrMethodNotAllowed
	}
	return false, ErrNotFound
}

func (app *App) defaultRequestHandler(rctx *fasthttp.RequestCtx) {
	ctx, ok := app.acquireDefaultCtx(rctx)
	if !ok {
		app.customRequestHandler(rctx)
		return
	}
	defer app.releaseDefaultCtx(ctx)

	// Check if the HTTP method is valid
	if ctx.methodInt == -1 {
		_ = ctx.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
		return
	}

	// Optional: check flash messages (hot path, see hasFlashCookie); before the
	// short-circuit so a skipped 404/405 still clears them.
	if hasFlashCookie(&ctx.fasthttp.Request.Header) {
		ctx.Redirect().parseAndClearFlashMessages()
	}

	// Early 404/405 before the middleware chain; enabled implies middleware exists
	// (without middleware next() already answers 404/405 cheaply). CORS preflight is
	// exempt so cors middleware can answer paths that lack an explicit OPTIONS route.
	if app.skip.enabled && !ctx.IsPreflight() {
		res := app.resolveSkip(ctx.methodInt, ctx.treePathHash, ctx.pathSlashCount(app),
			utils.UnsafeString(ctx.detectionPath), utils.UnsafeString(ctx.path), &ctx.values)
		switch res.decision {
		case skipNotFound:
			app.emitSkip(ctx, 0, ErrNotFound)
			return
		case skipNotAllowed:
			app.emitSkip(ctx, res.allowMask, ErrMethodNotAllowed)
			return
		default:
			ctx.firstMatchIndex = res.matchIndex
		}
	}

	_, err := app.next(ctx)
	if err != nil {
		if catch := ctx.App().ErrorHandler(ctx, err); catch != nil {
			_ = ctx.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
		}
	}
}

func (app *App) customRequestHandler(rctx *fasthttp.RequestCtx) {
	ctx := app.AcquireCtx(rctx)
	defer app.ReleaseCtx(ctx)

	// Check if the HTTP method is valid
	if ctx.getMethodInt() == -1 {
		_ = ctx.SendStatus(StatusNotImplemented) //nolint:errcheck // Always return nil
		return
	}

	// Optional: check flash messages (hot path, see hasFlashCookie); before the
	// short-circuit so a skipped 404/405 still clears them.
	if hasFlashCookie(&ctx.Request().Header) {
		ctx.Redirect().parseAndClearFlashMessages()
	}

	// Early 404/405 before the middleware chain; enabled implies middleware exists
	// (without middleware next() already answers 404/405 cheaply). CORS preflight is
	// exempt so cors middleware can answer paths that lack an explicit OPTIONS route.
	if app.skip.enabled && !ctx.IsPreflight() {
		res := app.resolveSkip(ctx.getMethodInt(), ctx.getTreePathHash(), ctx.pathSlashCount(app),
			ctx.getDetectionPath(), ctx.Path(), ctx.getValues())
		switch res.decision {
		case skipNotFound:
			app.emitSkipCustom(ctx, 0, ErrNotFound)
			return
		case skipNotAllowed:
			app.emitSkipCustom(ctx, res.allowMask, ErrMethodNotAllowed)
			return
		default:
			ctx.setFirstMatchIndex(res.matchIndex)
		}
	}

	_, err := app.nextCustom(ctx)
	if err != nil {
		if catch := ctx.App().ErrorHandler(ctx, err); catch != nil {
			_ = ctx.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
		}
	}
}

func (app *App) addPrefixToRoute(prefix string, route *Route, regexHandler any, customConstraints ...CustomConstraint) *Route {
	prefixedPath := getGroupPath(prefix, route.Path)
	prettyPath := prefixedPath
	// Case-sensitive routing, all to lowercase
	if !app.config.CaseSensitive {
		prettyPath = utilsstrings.ToLower(prettyPath)
	}
	// Strict routing, remove trailing slashes
	if !app.config.StrictRouting && len(prettyPath) > 1 {
		prettyPath = utils.TrimRight(prettyPath, '/')
	}

	route.Path = prefixedPath
	route.path = RemoveEscapeChar(prettyPath)
	route.routeParser = parseRoute(prettyPath, regexHandler, customConstraints...)
	route.root = false
	route.star = false
	route.caseSensitive = app.config.CaseSensitive
	// buildTree recomputes this for every route, but this function rewrites the
	// path and parser a filter is derived from, so refresh it here too rather
	// than depend on a caller marking the routes refreshed.
	route.buildPrefixFilter()

	return route
}

func (*App) copyRoute(route *Route) *Route {
	return &Route{
		// Leading-byte filter
		prefix:     route.prefix,
		prefixMask: route.prefixMask,

		// Router booleans
		use:           route.use,
		mount:         route.mount,
		star:          route.star,
		root:          route.root,
		autoHead:      route.autoHead,
		caseSensitive: route.caseSensitive,

		// Path data
		path:        route.path,
		routeParser: route.routeParser,

		// Public data
		Path:     route.Path,
		Params:   route.Params,
		Name:     route.Name,
		Method:   route.Method,
		Handlers: route.Handlers,
	}
}

func (app *App) normalizePath(path string) string {
	if path == "" {
		path = "/"
	}
	if path[0] != '/' {
		path = "/" + path
	}
	if !app.config.CaseSensitive {
		path = utilsstrings.ToLower(path)
	}
	if !app.config.StrictRouting && len(path) > 1 {
		path = utils.TrimRight(path, '/')
	}
	return RemoveEscapeChar(path)
}

// RemoveRoute is used to remove a route from the stack by path.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
func (app *App) RemoveRoute(path string, methods ...string) {
	// Normalize same as register uses
	norm := app.normalizePath(path)

	pathMatchFunc := func(r *Route) bool {
		return r.path == norm // compare private normalized path
	}
	app.deleteRoute(methods, pathMatchFunc)
}

// RemoveRouteByName is used to remove a route from the stack by name.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
func (app *App) RemoveRouteByName(name string, methods ...string) {
	matchFunc := func(r *Route) bool { return r.Name == name }
	app.deleteRoute(methods, matchFunc)
}

// RemoveRouteFunc is used to remove a route from the stack by a custom match function.
// If no methods are specified, it will remove the route for all methods defined in the app.
// You should call RebuildTree after using this to ensure consistency of the tree.
// Note: The route.Path is original path, not the normalized path.
func (app *App) RemoveRouteFunc(matchFunc func(r *Route) bool, methods ...string) {
	app.deleteRoute(methods, matchFunc)
}

func (app *App) deleteRoute(methods []string, matchFunc func(r *Route) bool) {
	if len(methods) == 0 {
		methods = app.config.RequestMethods
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	removedUseRoutes := make(map[string]struct{})

	for _, method := range methods {
		// Uppercase HTTP methods
		method = utilsstrings.ToUpper(method)

		// Get unique HTTP method identifier
		m := app.methodInt(method)
		if m == -1 {
			continue // Skip invalid HTTP methods
		}

		for i := len(app.stack[m]) - 1; i >= 0; i-- { //nolint:modernize // false positive
			route := app.stack[m][i]
			if !matchFunc(route) {
				continue // Skip if route does not match
			}

			app.stack[m] = append(app.stack[m][:i], app.stack[m][i+1:]...)
			app.hasRoutesRefreshed = true

			// Decrement global handler count. In middleware routes, only decrement once
			if _, ok := removedUseRoutes[route.path]; (route.use && slices.Equal(methods, app.config.RequestMethods) && !ok) || !route.use {
				if route.use {
					removedUseRoutes[route.path] = struct{}{}
				}

				atomic.AddUint32(&app.handlersCount, ^uint32(len(route.Handlers)-1)) //nolint:gosec // G115 - handler count is always small
			}

			if method == MethodGet && !route.use && !route.mount {
				app.pruneAutoHeadRouteLocked(route.path)
			}
		}
	}
}

// pruneAutoHeadRouteLocked removes an automatically generated HEAD route so a
// later explicit registration can take its place without duplicating handler
// chains. The caller must already hold app.mutex.
func (app *App) pruneAutoHeadRouteLocked(path string) {
	headIndex := app.methodInt(MethodHead)
	if headIndex == -1 {
		return
	}

	norm := app.normalizePath(path)

	headStack := app.stack[headIndex]
	for i, headRoute := range slices.Backward(headStack) {
		if headRoute.path != norm || headRoute.mount || headRoute.use || !headRoute.autoHead {
			continue
		}

		app.stack[headIndex] = append(headStack[:i], headStack[i+1:]...)
		app.hasRoutesRefreshed = true
		atomic.AddUint32(&app.handlersCount, ^uint32(len(headRoute.Handlers)-1)) //nolint:gosec // G115 - handler count is always small
		return
	}
}

func (app *App) register(methods []string, pathRaw string, group *Group, handlers ...Handler) {
	// A regular route requires at least one ctx handler
	if len(handlers) == 0 && group == nil {
		panic(fmt.Sprintf("missing handler/middleware in route: %s\n", pathRaw))
	}
	// No nil handlers allowed
	for _, h := range handlers {
		if h == nil {
			panic(fmt.Sprintf("nil handler in route: %s\n", pathRaw))
		}
	}

	// Precompute path normalization ONCE
	if pathRaw == "" {
		pathRaw = "/"
	}
	if pathRaw[0] != '/' {
		pathRaw = "/" + pathRaw
	}
	pathPretty := pathRaw
	if !app.config.CaseSensitive {
		pathPretty = utilsstrings.ToLower(pathPretty)
	}
	if !app.config.StrictRouting && len(pathPretty) > 1 {
		pathPretty = utils.TrimRight(pathPretty, '/')
	}
	pathClean := RemoveEscapeChar(pathPretty)

	parsedRaw := parseRoute(pathRaw, app.config.RegexHandler, app.customConstraints...)
	parsedPretty := parseRoute(pathPretty, app.config.RegexHandler, app.customConstraints...)

	isMount := group != nil && group.app != app

	for _, method := range methods {
		method = utilsstrings.ToUpper(method)
		if method != methodUse && app.methodInt(method) == -1 {
			panic(fmt.Sprintf("add: invalid http method %s\n", method))
		}

		isUse := method == methodUse
		isStar := pathClean == "/*"
		isRoot := pathClean == "/"

		route := Route{
			use:           isUse,
			mount:         isMount,
			star:          isStar,
			root:          isRoot,
			caseSensitive: app.config.CaseSensitive,

			path:        pathClean,
			routeParser: parsedPretty,
			Params:      parsedRaw.params,
			group:       group,

			Path:     pathRaw,
			Method:   method,
			Handlers: handlers,
		}
		route.buildPrefixFilter()

		// Increment global handler count
		atomic.AddUint32(&app.handlersCount, uint32(len(handlers))) //nolint:gosec // G115 - handler count is always small

		// Middleware route matches all HTTP methods
		if isUse {
			// Add route to all HTTP methods stack
			for _, m := range app.config.RequestMethods {
				// Create a route copy to avoid duplicates during compression
				r := route
				app.addRoute(m, &r)
			}
		} else {
			// Add route to stack
			app.addRoute(method, &route)
		}
	}
}

func (app *App) addRoute(method string, route *Route) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	// Get unique HTTP method identifier
	m := app.methodInt(method)

	if method == MethodHead && !route.mount && !route.use {
		app.pruneAutoHeadRouteLocked(route.path)
	}

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use && !route.mount && !app.stack[m][l-1].mount {
		preRoute := app.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
	} else {
		route.Method = method
		// Add route to the stack
		app.stack[m] = append(app.stack[m], route)
		app.hasRoutesRefreshed = true
	}

	// Execute onRoute hooks & change latestRoute if not adding mounted route
	if !route.mount {
		app.latestRoute = route
		if err := app.hooks.executeOnRouteHooks(route); err != nil {
			panic(err)
		}
	}
}

func (app *App) ensureAutoHeadRoutes() {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	app.ensureAutoHeadRoutesLocked()
}

func (app *App) ensureAutoHeadRoutesLocked() {
	if app.config.DisableHeadAutoRegister {
		return
	}

	headIndex := app.methodInt(MethodHead)
	getIndex := app.methodInt(MethodGet)
	if headIndex == -1 || getIndex == -1 {
		return
	}

	headStack := app.stack[headIndex]
	existing := make(map[string]struct{}, len(headStack))
	for _, route := range headStack {
		if route.mount || route.use {
			continue
		}
		existing[route.path] = struct{}{}
	}

	if len(app.stack[getIndex]) == 0 {
		return
	}

	var added bool

	for _, route := range app.stack[getIndex] {
		if route.mount || route.use {
			continue
		}
		if _, ok := existing[route.path]; ok {
			continue
		}

		headRoute := app.copyRoute(route)
		headRoute.group = route.group
		headRoute.Method = MethodHead
		headRoute.autoHead = true
		// Fasthttp automatically omits response bodies when transmitting
		// HEAD responses, so the copied GET handler stack can execute
		// unchanged while still producing an empty body on the wire.

		headStack = append(headStack, headRoute)
		existing[route.path] = struct{}{}
		app.hasRoutesRefreshed = true
		added = true

		atomic.AddUint32(&app.handlersCount, uint32(len(headRoute.Handlers))) //nolint:gosec // G115 - handler count is always small

		app.latestRoute = headRoute
		if err := app.hooks.executeOnRouteHooks(headRoute); err != nil {
			panic(err)
		}
	}

	if added {
		app.stack[headIndex] = headStack
	}
}

// RebuildTree rebuilds the prefix tree from the previously registered routes.
// This method is useful when you want to register routes dynamically after the app has started.
// It is not recommended to use this method on production environments because rebuilding
// the tree is performance-intensive and not thread-safe in runtime. Since building the tree
// is only done in the startupProcess of the app, this method does not make sure that the
// routeTree is being safely changed, as it would add a great deal of overhead in the request.
// Latest benchmark results showed a degradation from 82.79 ns/op to 94.48 ns/op and can be found in:
// https://github.com/gofiber/fiber/issues/2769#issuecomment-2227385283
func (app *App) RebuildTree() *App {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	return app.buildTree()
}

// buildTree build the prefix tree from the previously registered routes
func (app *App) buildTree() *App {
	// If routes haven't been refreshed, nothing to do
	if !app.hasRoutesRefreshed {
		return app
	}

	// 1) First loop: determine all possible 3-char prefixes ("treePaths") for each method
	hasParamRoutes := false
	for method := range app.config.RequestMethods {
		routes := app.stack[method]
		treePaths := make([]int, len(routes))

		globalCount := 0
		prefixCounts := make(map[int]int, len(routes))

		for i, route := range routes {
			// The leading-byte filter is deliberately not rebuilt here; see
			// buildPrefixFilter. These routes are live for in-flight requests.

			// Star routes resolve before the slash-count quick-reject in
			// Route.match, so only non-star parametric routes consult it.
			if len(route.Params) > 0 && !route.star {
				hasParamRoutes = true
			}

			if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= maxDetectionPaths &&
				!dropsOptionalSlashBelowTreeHash(route.routeParser.segs[0]) {
				treePaths[i] = int(route.routeParser.segs[0].Const[0])<<16 |
					int(route.routeParser.segs[0].Const[1])<<8 |
					int(route.routeParser.segs[0].Const[2])
			}

			if treePaths[i] == 0 {
				globalCount++
				continue
			}

			prefixCounts[treePaths[i]]++
		}

		// Carve this build's buckets out of one allocation. The arena is fresh
		// per build, so a published bucket is never written through; a bucket
		// per make() would give the same guarantee, but one allocation keeps a
		// rebuild's alloc count flat as routes are added.
		//
		// Each bucket is a three-index slice whose cap is exactly what the
		// append loop below puts in it, so a bucket cannot reach its
		// neighbor: appending past that cap reallocates instead of writing
		// into the next window.
		total := globalCount
		for _, count := range prefixCounts {
			total += count + globalCount
		}
		arena := make([]*Route, total)

		tsMap := make(map[int][]*Route, len(prefixCounts)+1)
		tsMap[0] = arena[0:0:globalCount]
		off := globalCount
		for treePath, count := range prefixCounts {
			end := off + count + globalCount
			tsMap[treePath] = arena[off:off:end]
			off = end
		}

		for i, route := range routes {
			treePath := treePaths[i]

			if treePath == 0 {
				for bucket := range tsMap {
					tsMap[bucket] = append(tsMap[bucket], route)
				}
				continue
			}

			tsMap[treePath] = append(tsMap[treePath], route)
		}

		app.treeStack[method] = tsMap
		app.treeIndex[method] = buildRouteTree(tsMap)
	}
	app.hasParamRoutes = hasParamRoutes

	app.buildSkipIndexes()

	// reset the flag and return
	app.hasRoutesRefreshed = false
	return app
}

// dropsOptionalSlashBelowTreeHash reports whether a route's leading constant
// segment can match a detection path too short to carry a tree hash.
//
// getMatch lets a segment with HasOptionalSlash match one byte less than its
// Const (the trailing '/' is optional), so "/a/" also matches "/a". A
// detection path shorter than maxDetectionPaths always hashes to 0, so a route
// bucketed under the hash of its full Const would be invisible to that
// request: next() would scan bucket 0 and never see it. Consts longer than
// maxDetectionPaths are unaffected — dropping the final '/' leaves the first
// three bytes, and therefore the hash, unchanged.
func dropsOptionalSlashBelowTreeHash(seg *routeSegment) bool {
	return seg.HasOptionalSlash && len(seg.Const) == maxDetectionPaths
}
