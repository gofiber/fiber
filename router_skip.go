// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// skipRouteIndex holds the precomputed routing state used to resolve unmatched
// requests cheaply, kept off the App struct to keep it focused.
//
// Most of it backs the opt-in SkipUnmatchedRoutes fast path, which answers
// 404/405 before the middleware chain runs: staticMethods, bucketParamMethods,
// paramRoutes, hasUseRoutes and hasParamUse. The method mask
// (routeMethods/methodMaskValid) additionally prunes the cross-method 405
// fallback in next()/nextCustom() and is maintained even when the option is off.
//
// The whole struct is rebuilt from the route tree by buildSkipIndexes whenever
// the tree changes, so it always reflects the current route set.
type skipRouteIndex struct {
	// staticMethods indexes static (non-param, non-root, non-star, non-use,
	// non-mount) endpoints. The key is the prettified route path; the value is a
	// bitmask of method ints that have a static endpoint at that path. uint64 keeps
	// the mask 64-bit wide on every platform. Only built when SkipUnmatchedRoutes
	// is enabled.
	staticMethods map[string]uint64
	// bucketParamMethods is a per-tree-bucket bitmask of method ints that have at least
	// one parametric/root/star endpoint in that bucket. When the mask for the relevant
	// bucket is zero, the static index is authoritative and no cross-method scan is
	// needed. Only built when SkipUnmatchedRoutes is enabled.
	bucketParamMethods map[int]uint64
	// paramRoutes mirrors treeStack but holds only the parametric/root/star endpoints
	// of each bucket together with their index in that bucket, so the SkipUnmatchedRoutes
	// lookahead scans just the candidate routes instead of the whole bucket. It has an
	// entry for every treeStack bucket (empty when the bucket has none), so a lookup miss
	// mirrors a treeStack miss and selects the bucket-0 fallback. Indexed by method int,
	// then tree-bucket key. Only built when SkipUnmatchedRoutes is enabled.
	paramRoutes []map[int][]indexedRoute
	// routeMethods is a bitmask of method ints that own at least one non-use route.
	// The 404/405 cross-method fallback in next()/nextCustom() consults it to skip
	// methods that can never contribute an Allow entry, avoiding their tree-bucket map
	// lookups. Only trusted when methodMaskValid is true (RequestMethods <= 64).
	routeMethods uint64
	// hasUseRoutes is true when at least one middleware (use) route is registered.
	// When false, SkipUnmatchedRoutes is a no-op (next() already answers 404/405 without
	// running anything), so the lookahead is skipped entirely.
	hasUseRoutes bool
	// hasParamUse is true when at least one middleware (use) route has parameters or
	// is a wildcard, i.e. its match writes into c.values. When false, no middleware can
	// clobber the params the SkipUnmatchedRoutes lookahead already wrote for the matched
	// endpoint, so next() can reuse them instead of re-matching the route.
	hasParamUse bool
	// methodMaskValid is true when routeMethods can be trusted to prune the cross-method
	// fallback scan, i.e. RequestMethods fits in a 64-bit mask. When false (an unusually
	// large custom RequestMethods list), the fallback scans every method as before.
	methodMaskValid bool
}

// indexedRoute pairs a candidate route with its index in the tree bucket it was
// taken from, so the SkipUnmatchedRoutes lookahead can hand that index to
// next()/nextCustom() (via firstMatchIndex) to skip endpoints already ruled out.
type indexedRoute struct {
	route *Route
	idx   int
}

// SkipUnmatchedRoutes decision codes returned by resolveSkip.
const (
	skipRunChain  = iota // proceed to the normal middleware/handler chain
	skipNotFound         // answer 404 without running the chain
	skipMethodNot        // answer 405 without running the chain; allowMask holds the methods
)

// skipResult is the outcome of the SkipUnmatchedRoutes lookahead.
type skipResult struct {
	decision   int    // skipRunChain, skipNotFound, or skipMethodNot
	allowMask  uint64 // methods to advertise in the Allow header (skipMethodNot only)
	matchIndex int    // pre-resolved endpoint index for next()/nextCustom(), or -1
}

// buildSkipIndexes rebuilds app.skip from the freshly built route tree. It always
// refreshes the 405-fallback method mask, and additionally builds the
// SkipUnmatchedRoutes lookahead indexes when that option is enabled. It is called
// from buildTree so the indexes stay in sync with the current route set.
func (app *App) buildSkipIndexes() {
	idx := skipRouteIndex{}

	// The per-method masks are 64-bit. If RequestMethods exceeds 64, method indexes
	// >= 64 would shift out of range and miscompute, so leave every mask disabled;
	// the fallback then scans all methods and the SkipUnmatchedRoutes fast path stays
	// off, letting next() answer 404/405 as usual.
	idx.methodMaskValid = len(app.config.RequestMethods) <= 64
	if !idx.methodMaskValid {
		app.skip = idx
		return
	}

	// Method mask for the 404/405 cross-method fallback prune. Maintained regardless
	// of SkipUnmatchedRoutes: a method with no non-use route can never add an Allow
	// entry, so the fallback can skip its buckets entirely.
	for method := range app.config.RequestMethods {
		for _, route := range app.stack[method] {
			if !route.use {
				idx.routeMethods |= uint64(1) << method
				break
			}
		}
	}

	// The remaining indexes only serve the opt-in SkipUnmatchedRoutes fast path.
	if app.config.SkipUnmatchedRoutes {
		idx.buildLookahead(app)
	}

	app.skip = idx
}

// buildLookahead fills the SkipUnmatchedRoutes lookahead indexes: a method-global
// index of static endpoints, a per-tree-bucket bitmask of methods that own
// parametric/root/star endpoints, and a per-bucket candidate list of those
// endpoints.
func (idx *skipRouteIndex) buildLookahead(app *App) {
	static := make(map[string]uint64)
	bucketParam := make(map[int]uint64)
	paramRoutes := make([]map[int][]indexedRoute, len(app.config.RequestMethods))
	hasUse := false
	hasParamUse := false

	for method := range app.config.RequestMethods {
		bit := uint64(1) << method
		paramRoutes[method] = make(map[int][]indexedRoute)

		// Static index from the flat stack. A static endpoint is matched by
		// route.match via a plain exact compare against route.path, so keying by
		// route.path and looking up by detectionPath is exact.
		for _, route := range app.stack[method] {
			if route.use {
				hasUse = true
				// A parametric or wildcard middleware writes into c.values when it
				// matches, which would clobber the lookahead's params.
				if route.star || len(route.Params) > 0 {
					hasParamUse = true
				}
				continue
			}
			if route.mount || route.star || route.root || len(route.Params) > 0 {
				continue
			}
			static[route.path] |= bit
		}

		// Per-bucket candidate list of parametric/root/star endpoints from the
		// final buckets (post bucket-0 replication) so they mirror exactly what
		// next() scans per bucket. The stored idx is the route's position in that
		// same bucket, which next() iterates. An entry is created for every bucket
		// (empty when it has no candidates) so a lookup miss in resolveSkip mirrors
		// a treeStack miss and selects the bucket-0 fallback. bucketParamMethods
		// records which methods have candidates per bucket, for the authoritative
		// no-scan miss. Mounted sub-apps are expanded into normal routes before
		// buildTree runs, so no mount route reaches this point.
		for treeHash, bucket := range app.treeStack[method] {
			var cands []indexedRoute
			for i, route := range bucket {
				if route.use || route.mount {
					continue
				}
				if route.root || route.star || len(route.Params) > 0 {
					cands = append(cands, indexedRoute{route: route, idx: i})
					bucketParam[treeHash] |= bit
				}
			}
			paramRoutes[method][treeHash] = cands
		}
	}

	idx.staticMethods = static
	idx.bucketParamMethods = bucketParam
	idx.paramRoutes = paramRoutes
	idx.hasUseRoutes = hasUse
	idx.hasParamUse = hasParamUse
}

// resolveSkip implements the SkipUnmatchedRoutes two-tier fast path. matchIndex
// lets next/nextCustom skip re-checking the endpoints already ruled out. values
// is used as scratch; on a parametric match it holds that route's params, but
// next() re-matches the route to recompute them (middleware between here and the
// endpoint may overwrite the slice). Method masks are uint64 so they stay 64-bit
// wide on 32-bit platforms.
func (app *App) resolveSkip(methodInt, treeHash int, detectionPath, path string, values *[maxParams]string) skipResult {
	skip := &app.skip
	methodBit := uint64(1) << methodInt
	staticMask := skip.staticMethods[detectionPath]

	// Tier 1a: a static endpoint matches this method -> run the chain normally.
	if staticMask&methodBit != 0 {
		return skipResult{decision: skipRunChain, matchIndex: -1}
	}

	// Resolve this method's parametric candidates for the request's bucket. paramRoutes
	// has an entry for every treeStack bucket, so a lookup miss mirrors a treeStack miss
	// and selects the bucket-0 fallback exactly as next() does; the candidate indices
	// then line up with the bucket next() iterates.
	cands, ok := skip.paramRoutes[methodInt][treeHash]
	if !ok {
		cands = skip.paramRoutes[methodInt][0]
	}

	// Tier 2: scan this method's candidates first, so a match never pays for the
	// authoritative check below.
	for _, cand := range cands {
		if cand.route.match(detectionPath, path, values) {
			return skipResult{decision: skipRunChain, matchIndex: cand.idx}
		}
	}

	// No match for the requested method. If no method has a parametric route reachable
	// for this bucket (the specific bucket, or bucket 0 for methods that lack it), the
	// static index is authoritative and no cross-method scan is needed.
	if skip.bucketParamMethods[treeHash]|skip.bucketParamMethods[0] == 0 {
		if staticMask != 0 {
			return skipResult{decision: skipMethodNot, allowMask: staticMask, matchIndex: -1}
		}
		return skipResult{decision: skipNotFound, matchIndex: -1}
	}

	// Combine the static index with a parametric scan of the other methods to decide
	// between 405 and 404.
	allow := staticMask
	methods := app.config.RequestMethods
	for m := range methods {
		if m == methodInt || allow&(uint64(1)<<m) != 0 {
			continue
		}
		c2, ok := skip.paramRoutes[m][treeHash]
		if !ok {
			c2 = skip.paramRoutes[m][0]
		}
		for _, cand := range c2 {
			if cand.route.match(detectionPath, path, values) {
				allow |= uint64(1) << m
				break
			}
		}
	}
	if allow != 0 {
		return skipResult{decision: skipMethodNot, allowMask: allow, matchIndex: -1}
	}
	return skipResult{decision: skipNotFound, matchIndex: -1}
}

// emitSkip renders a SkipUnmatchedRoutes short-circuit response (404 or 405)
// through the configured error handler, matching the behavior of next()'s own
// terminal 404/405 path but without running the middleware chain. It takes the
// concrete *DefaultCtx so the variadic Append calls stay on the stack (calling
// Append through the Ctx interface forces the value slice to escape and allocate).
func (app *App) emitSkip(c *DefaultCtx, allowMask uint64, err error) {
	// allowMask is only non-zero for the 405 case.
	if allowMask != 0 {
		methods := app.config.RequestMethods
		for i := range methods {
			if allowMask&(uint64(1)<<i) != 0 {
				c.Append(HeaderAllow, methods[i])
			}
		}
	}
	if catch := app.ErrorHandler(c, err); catch != nil {
		_ = c.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
	}
}

// emitSkipCustom is the CustomCtx counterpart of emitSkip for the custom-context
// request path. The Append calls allocate here (interface dispatch), but this
// path is rare compared to the default *DefaultCtx handler.
func (app *App) emitSkipCustom(c CustomCtx, allowMask uint64, err error) {
	if allowMask != 0 {
		methods := app.config.RequestMethods
		for i := range methods {
			if allowMask&(uint64(1)<<i) != 0 {
				c.Append(HeaderAllow, methods[i])
			}
		}
	}
	if catch := app.ErrorHandler(c, err); catch != nil {
		_ = c.SendStatus(StatusInternalServerError) //nolint:errcheck // Always return nil
	}
}
