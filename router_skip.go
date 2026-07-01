// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

// skipRouteIndex holds precomputed unmatched-route indexes, rebuilt by buildSkipIndexes with the route tree.
type skipRouteIndex struct {
	// staticMethods is a method bitmask of static endpoints per route path; SkipUnmatchedRoutes only
	staticMethods map[string]uint64
	// buckets holds the per-tree-bucket lookahead state, one entry per treeStack bucket key (union across methods)
	buckets map[int]*skipBucket
	// zeroBucket serves tree hashes with no bucket at all, mirroring next()'s treeStack miss
	zeroBucket *skipBucket
	// routeMethods is a method bitmask with at least one non-use route; only valid when methodMaskValid
	routeMethods uint64
	// enabled gates the fast path: SkipUnmatchedRoutes is on and middleware is registered
	// (without middleware next() already answers 404/405 cheaply)
	enabled bool
	// hasParamUse is true when any param/wildcard middleware exists; when false next() can reuse the lookahead's params
	hasParamUse bool
	// methodMaskValid is true when routeMethods is trustworthy (len(RequestMethods) <= 64)
	methodMaskValid bool
}

// skipBucket holds one tree bucket's param/root/star candidates per method, with the
// bucket-0 fallback materialized at build time so a lookup never needs a second map access.
type skipBucket struct {
	// cands is indexed by method int
	cands [][]indexedRoute
	// paramMask has a bit per method with at least one candidate in cands
	paramMask uint64
}

// indexedRoute pairs a candidate route with its position in its tree bucket.
type indexedRoute struct {
	route *Route
	idx   int
}

// skipDecision is the outcome kind of the SkipUnmatchedRoutes lookahead.
type skipDecision int

const (
	skipRunChain   skipDecision = iota // run the normal chain
	skipNotFound                       // 404 without running the chain
	skipNotAllowed                     // 405; allowMask holds the methods
)

// skipResult is the outcome of the SkipUnmatchedRoutes lookahead.
type skipResult struct {
	decision   skipDecision
	allowMask  uint64 // methods for the Allow header (skipNotAllowed only)
	matchIndex int    // pre-resolved endpoint index for next()/nextCustom(), or -1
}

// buildSkipIndexes rebuilds app.skip from the route tree; called from buildTree.
func (app *App) buildSkipIndexes() {
	idx := skipRouteIndex{}

	// Masks are 64-bit; with more methods leave everything disabled, next() answers as usual.
	idx.methodMaskValid = len(app.config.RequestMethods) <= 64
	if !idx.methodMaskValid {
		app.skip = idx
		return
	}

	// 405-fallback prune mask; maintained even when SkipUnmatchedRoutes is off.
	for method := range app.config.RequestMethods {
		for _, route := range app.stack[method] {
			if !route.use {
				idx.routeMethods |= uint64(1) << method
				break
			}
		}
	}

	if app.config.SkipUnmatchedRoutes {
		idx.buildLookahead(app)
	}

	app.skip = idx
}

// buildLookahead fills the SkipUnmatchedRoutes lookahead indexes.
func (idx *skipRouteIndex) buildLookahead(app *App) {
	nMethods := len(app.config.RequestMethods)
	static := make(map[string]uint64)
	buckets := make(map[int]*skipBucket)
	hasUse := false
	hasParamUse := false

	getBucket := func(treeHash int) *skipBucket {
		b := buckets[treeHash]
		if b == nil {
			b = &skipBucket{cands: make([][]indexedRoute, nMethods)}
			buckets[treeHash] = b
		}
		return b
	}

	for method := range app.config.RequestMethods {
		bit := uint64(1) << method

		// Static endpoints match by exact compare against route.path, so keying by route.path is exact.
		for _, route := range app.stack[method] {
			if route.use {
				hasUse = true
				// param/wildcard middleware clobbers c.values on match
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

		// Candidates come from the final buckets (post bucket-0 replication) so idx lines up with next()'s scan.
		for treeHash, bucket := range app.treeStack[method] {
			sb := getBucket(treeHash)
			for i, route := range bucket {
				if route.use || route.mount {
					continue
				}
				if route.root || route.star || len(route.Params) > 0 {
					sb.cands[method] = append(sb.cands[method], indexedRoute{route: route, idx: i})
					sb.paramMask |= bit
				}
			}
		}
	}

	zero := buckets[0]
	if zero == nil {
		zero = &skipBucket{cands: make([][]indexedRoute, nMethods)}
		buckets[0] = zero
	}

	// Materialize the per-method bucket-0 fallback: a method without a treeStack
	// bucket at this hash scans its bucket 0 in next(), so mirror that here.
	for treeHash, sb := range buckets {
		if treeHash == 0 {
			continue
		}
		for method := range app.config.RequestMethods {
			if _, ok := app.treeStack[method][treeHash]; ok {
				continue
			}
			sb.cands[method] = zero.cands[method]
			if len(zero.cands[method]) > 0 {
				sb.paramMask |= uint64(1) << method
			}
		}
	}

	idx.staticMethods = static
	idx.buckets = buckets
	idx.zeroBucket = zero
	idx.enabled = hasUse
	idx.hasParamUse = hasParamUse
}

// resolveSkip decides 404/405/run-chain. values is scratch: param/wildcard
// middleware may overwrite it before the endpoint runs, so next() re-matches then.
func (app *App) resolveSkip(methodInt, treeHash int, detectionPath, path string, values *[maxParams]string) skipResult {
	skip := &app.skip
	methodBit := uint64(1) << methodInt
	staticMask := skip.staticMethods[detectionPath]

	// Tier 1: a static endpoint matches this method.
	if staticMask&methodBit != 0 {
		return skipResult{decision: skipRunChain, matchIndex: -1}
	}

	// Single bucket lookup; an unknown tree hash falls back to bucket 0 like next() does.
	b, ok := skip.buckets[treeHash]
	if !ok {
		b = skip.zeroBucket
	}

	// Tier 2: scan this method's parametric candidates.
	for _, cand := range b.cands[methodInt] {
		if cand.route.match(detectionPath, path, values) {
			return skipResult{decision: skipRunChain, matchIndex: cand.idx}
		}
	}

	// No param route reachable for any method: the static index alone decides 404 vs 405.
	if b.paramMask == 0 {
		if staticMask != 0 {
			return skipResult{decision: skipNotAllowed, allowMask: staticMask, matchIndex: -1}
		}
		return skipResult{decision: skipNotFound, matchIndex: -1}
	}

	// Cross-method scan to decide 405 vs 404; paramMask prunes methods without candidates.
	allow := staticMask
	for m := range app.config.RequestMethods {
		bit := uint64(1) << m
		if m == methodInt || allow&bit != 0 || b.paramMask&bit == 0 {
			continue
		}
		for _, cand := range b.cands[m] {
			if cand.route.match(detectionPath, path, values) {
				allow |= bit
				break
			}
		}
	}
	if allow != 0 {
		return skipResult{decision: skipNotAllowed, allowMask: allow, matchIndex: -1}
	}
	return skipResult{decision: skipNotFound, matchIndex: -1}
}

// emitSkip answers the skipped 404/405 via the error handler. Takes *DefaultCtx so
// the variadic Append argument stays on the stack (interface dispatch allocates).
func (app *App) emitSkip(c *DefaultCtx, allowMask uint64, err error) {
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

// emitSkipCustom is the CustomCtx counterpart of emitSkip; Append allocates here.
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
