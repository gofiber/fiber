// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// domainLocalsKeyType is an unexported type used as the Locals key for domain
// parameters, preventing collisions with user or middleware keys.
type domainLocalsKeyType struct{}

// domainLocalsKey is the typed key used in c.Locals() to store domain parameter values.
var domainLocalsKey = domainLocalsKeyType{}

// domainParams stores domain parameter names and their values for a request.
type domainParams struct {
	names  []string
	values []string
}

// domainCheckResult caches a domain match result for a single request.
// It stores the matched domain params (if any) alongside the match status
// to avoid allocating a new domainParams struct for every handler invocation.
type domainCheckResult struct {
	params    *domainParams // pre-built params (nil if no params or no match)
	isMatched bool
}

// domainMatcher holds the parsed domain pattern for matching against request hostnames.
type domainMatcher struct {
	parts      []string // domain parts split by "."
	paramIdx   []int    // indices of parameter parts
	paramNames []string // parameter names (without ":")
	numParts   int      // total number of parts
}

// maxDomainParts defines the maximum number of domain labels allowed (e.g., sub.domain.example.com = 4 parts).
// This prevents DoS attacks from patterns or hostnames with excessive label counts.
// RFC 1035 suggests 127 labels max, but we use a more conservative limit to prevent memory exhaustion.
const maxDomainParts = 16

// parseDomainPattern parses a domain pattern like ":subdomain.example.com"
// into a domainMatcher. Parameter parts start with ":".
// Constant labels are lowercased per RFC 4343 (domain names are case-insensitive),
// but parameter names are preserved as-is so that DomainParam lookups work with
// the exact names the caller used (e.g., ":User" → param name "User").
func parseDomainPattern(pattern string) domainMatcher {
	pattern = utils.TrimSpace(pattern)
	// Trim trailing dot of a fully-qualified domain name (RFC 3986),
	// consistent with Fiber's own host normalization in Subdomains().
	pattern = utils.TrimRight(pattern, '.')

	// Validate pattern is not empty after trimming
	if pattern == "" {
		panic("Domain pattern cannot be empty")
	}

	// Enforce RFC 1035 total length limit on patterns
	if len(pattern) > 253 {
		panic(fmt.Sprintf("Domain pattern '%s' exceeds RFC 1035 maximum of 253 characters (%d chars)",
			pattern, len(pattern)))
	}

	parts := strings.Split(pattern, ".")

	// Prevent DoS from patterns with excessive label counts
	if len(parts) > maxDomainParts {
		panic(fmt.Sprintf("Domain pattern '%s' has %d parts, which exceeds the maximum of %d",
			pattern, len(parts), maxDomainParts))
	}

	m := domainMatcher{
		parts:    make([]string, len(parts)),
		numParts: len(parts),
	}

	for i, part := range parts {
		// Validate no empty labels (e.g., "example..com" is invalid)
		if part == "" {
			panic(fmt.Sprintf("Domain pattern '%s' contains empty label at position %d", pattern, i))
		}

		if part[0] == ':' {
			// Validate parameter name is not empty
			if len(part) == 1 {
				panic(fmt.Sprintf("Domain pattern '%s' contains empty parameter name at position %d", pattern, i))
			}
			paramName := part[1:]
			// Validate parameter name contains only ASCII-safe characters (a-z, A-Z, 0-9, underscore, hyphen).
			// Using explicit ASCII ranges rather than unicode.IsLetter/IsDigit to reject non-ASCII
			// characters that are invalid in DNS names.
			for _, ch := range paramName {
				if !isASCIIAlphanumeric(ch) && ch != '_' && ch != '-' {
					panic(fmt.Sprintf("Domain pattern '%s' contains invalid parameter name '%s' with character '%c'", pattern, paramName, ch))
				}
			}
			m.paramIdx = append(m.paramIdx, i)
			m.paramNames = append(m.paramNames, paramName) // preserve original case
			m.parts[i] = part                              // keep ":param" marker for matching
		} else {
			// Only lowercase constant labels (RFC 4343)
			// Enforce RFC 1035 per-label length limit (63 characters)
			if len(part) > 63 {
				panic(fmt.Sprintf("Domain pattern '%s' has label '%s' exceeding RFC 1035 limit of 63 characters (%d chars)",
					pattern, part, len(part)))
			}
			// Validate label contains only valid ASCII domain characters (a-z, 0-9, hyphen).
			normalized := utilsstrings.ToLower(part)
			for _, ch := range normalized {
				if !isASCIIAlphanumeric(ch) && ch != '-' {
					panic(fmt.Sprintf("Domain pattern '%s' contains invalid character '%c' in label '%s'", pattern, ch, part))
				}
			}
			m.parts[i] = normalized
		}
	}

	// Check if the domain pattern has too many parameters
	if len(m.paramNames) > maxParams {
		panic(fmt.Sprintf("Domain pattern '%s' has %d parameters, which exceeds the maximum of %d",
			pattern, len(m.paramNames), maxParams))
	}

	return m
}

// match checks if a hostname matches the domain pattern.
// It returns true if matched and a slice of parameter values (parallel to paramNames).
// Uses a stack-allocated buffer to avoid heap allocation for typical domain names.
// Validates hostname to prevent DoS attacks from malicious input.
func (m *domainMatcher) match(hostname string) (bool, []string) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	// Trim trailing dot of a fully-qualified domain name (RFC 3986),
	// consistent with Fiber's own host normalization in Subdomains().
	hostname = utils.TrimRight(hostname, '.')

	// Validate hostname is not empty and not excessively long (DoS protection)
	// RFC 1035 limits domain names to 253 characters
	if hostname == "" || len(hostname) > 253 {
		return false, nil
	}

	// Domain names are case-insensitive per RFC 4343; lowercase after cheap validation
	hostname = utilsstrings.ToLower(hostname)

	// Use stack-allocated array for typical domain names (up to 16 labels).
	// This avoids heap allocation for most common cases, consistent with
	// the Subdomains() implementation in req.go.
	// The buffer size matches maxDomainParts to prevent overflow.
	var partsBuf [maxDomainParts]string
	parts := partsBuf[:0]
	labelCount := 0
	for part := range strings.SplitSeq(hostname, ".") {
		labelCount++
		// DoS protection: reject hostnames with too many labels
		if labelCount > maxDomainParts {
			return false, nil
		}
		// DoS protection: reject empty labels or excessively long labels
		// RFC 1035 limits each label to 63 characters
		if part == "" || len(part) > 63 {
			return false, nil
		}
		// Validate label contains only safe ASCII domain characters (basic sanitization)
		// This prevents injection attacks via malicious hostnames
		for _, ch := range part {
			if ch != '-' && !isASCIIAlphanumeric(ch) {
				return false, nil
			}
		}
		parts = append(parts, part)
	}

	if len(parts) != m.numParts {
		return false, nil
	}

	// First pass: validate all constant labels without allocating paramValues.
	for i, patternPart := range m.parts {
		if patternPart != "" && patternPart[0] == ':' {
			// Parameter segment; skip in this pass.
			continue
		}
		if patternPart != parts[i] {
			return false, nil
		}
	}

	// No parameters to capture; avoid allocating an empty slice.
	if len(m.paramIdx) == 0 {
		return true, nil
	}

	// Second pass: now that constants are confirmed, allocate and fill paramValues.
	paramValues := make([]string, len(m.paramIdx))
	paramIter := 0
	for i, patternPart := range m.parts {
		if patternPart != "" && patternPart[0] == ':' {
			paramValues[paramIter] = parts[i]
			paramIter++
		}
	}

	return true, paramValues
}

// isASCIIAlphanumeric returns true if the rune is an ASCII letter (a-z, A-Z) or digit (0-9).
// This is used instead of unicode.IsLetter/unicode.IsDigit to ensure only ASCII characters
// are accepted in domain patterns and hostnames, as DNS names are ASCII-only.
func isASCIIAlphanumeric(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// DomainParam returns the value of a domain parameter from the context.
// Domain parameters are set when a route registered via [App.Domain] or [Group.Domain]
// matches the incoming request hostname.
//
//	app.Domain("example.com").Get("/", func(c fiber.Ctx) error {
//	    return c.SendString("Welcome!")
//	})
//
//	app.Domain(":user.example.com").Get("/", func(c fiber.Ctx) error {
//	    user := fiber.DomainParam(c, "user")
//	    return c.SendString("Hello, " + user)
//	})
func DomainParam(c Ctx, key string, defaultValue ...string) string {
	if params, ok := c.Locals(domainLocalsKey).(*domainParams); ok && params != nil {
		for i, name := range params.names {
			if name == key {
				return params.values[i]
			}
		}
	}

	if len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return ""
}

// domainRouter implements [Router] for domain-filtered routing.
// It wraps an underlying [App] or [Group] and checks the request hostname
// against the domain pattern before executing handlers.
//
// Routes registered through a domainRouter have zero impact on routing
// performance for requests that don't use domain-based routing.
//
// Known limitation: because domain filtering is applied at handler-execution
// time (not at route-matching time), Fiber's 405 Method Not Allowed logic
// may advertise methods for domain-scoped routes even when the requesting
// host does not match the domain pattern. Fixing this would require core
// router changes; for now callers should be aware that 405 responses may
// include methods from domain-scoped routes whose host did not match.
type domainRouter struct {
	app     *App
	group   *Group // non-nil when created from a Group
	matcher domainMatcher
}

// Verify domainRouter implements Router at compile time.
var _ Router = (*domainRouter)(nil)

// wrapHandlers wraps every handler in the slice with domain checking.
// The hostname match is computed once per request per domain-router and cached
// so that subsequent handlers in the same route avoid redundant parsing.
// Each handler independently checks the cached result, ensuring that Fiber's
// route-merging behavior (combining handlers from multiple registrations into
// one route) cannot cause a non-domain handler to be skipped.
// domainCheckResult objects are cached per-request in c.Locals() to avoid redundant hostname parsing.
func (d *domainRouter) wrapHandlers(handlers []Handler) []Handler {
	if len(handlers) == 0 {
		return handlers
	}

	// Use the domainRouter pointer as cache key to avoid cross-matcher collisions.
	// Each domainRouter instance gets its own cache slot.
	cacheKey := d

	result := make([]Handler, len(handlers))
	for i, h := range handlers {
		origHandler := h
		result[i] = func(c Ctx) error {
			// Check if we already matched this domain on this request.
			var check *domainCheckResult
			if cached, ok := c.Locals(cacheKey).(*domainCheckResult); ok {
				check = cached
			} else {
				hostname := c.Hostname()
				isMatched, values := d.matcher.match(hostname)
				check = &domainCheckResult{isMatched: isMatched}
				if isMatched && len(values) > 0 {
					// Store values directly — match() returns a fresh slice each time.
					// Build domainParams once and cache it alongside the match result
					// so subsequent handlers reuse the same struct.
					check.params = &domainParams{
						names:  d.matcher.paramNames,
						values: values,
					}
				}
				c.Locals(cacheKey, check)
			}

			if !check.isMatched {
				return c.Next()
			}

			// Reuse the cached domainParams (or nil to clear stale values)
			// instead of allocating a new struct per handler invocation.
			c.Locals(domainLocalsKey, check.params)

			return origHandler(c)
		}
	}

	return result
}

// registerPath returns the full path for registration, taking group prefix into account.
func (d *domainRouter) registerPath(path string) string {
	if d.group != nil {
		return getGroupPath(d.group.Prefix, path)
	}

	return path
}

// registerGroup returns the group to associate with routes, if any.
func (d *domainRouter) registerGroup() *Group {
	return d.group
}

// Use registers a middleware route that will match requests
// with the provided prefix (which is optional and defaults to "/").
//
// The middleware only executes when the request hostname matches the domain pattern.
//
//	api := app.Domain("api.example.com")
//	api.Use(func(c fiber.Ctx) error {
//	    // Only runs for api.example.com requests
//	    return c.Next()
//	})
func (d *domainRouter) Use(args ...any) Router {
	var subApp *App
	var prefix string
	var prefixes []string
	var handlers []Handler

	for i := range args {
		switch arg := args[i].(type) {
		case string:
			prefix = arg
		case []string:
			prefixes = arg
		case *App:
			subApp = arg
		default:
			handler, ok := toFiberHandler(arg)
			if !ok {
				panic(fmt.Sprintf("use: invalid handler %v", reflect.TypeOf(arg)))
			}
			handlers = append(handlers, handler)
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	for _, prefix := range prefixes {
		if subApp != nil {
			d.mount(prefix, subApp)
			continue
		}

		wrapped := d.wrapHandlers(handlers)
		d.app.register([]string{methodUse}, d.registerPath(prefix), d.registerGroup(), wrapped...)
	}

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls (before routes) and route-name calls (after routes).
	if d.group != nil && !d.group.hasAnyRoute {
		d.group.hasAnyRoute = true
	}

	return d
}

// mount attaches a sub-app instance to the domain router at the specified prefix.
// All routes from the sub-app will only be accessible when the request hostname
// matches the domain pattern.
//
// The sub-app is not modified: routes are cloned into a dedicated wrapper app
// with domain-filtered handlers, so the same sub-app can safely be mounted on
// multiple domains without double-wrapping. Routes added to the sub-app after
// mounting will not inherit domain filtering.
func (d *domainRouter) mount(prefix string, subApp *App) Router {
	// Determine the full mount path by combining the domain router's path with the prefix
	var mountPath string
	if d.group != nil {
		mountPath = getGroupPath(d.group.Prefix, prefix)
	} else {
		mountPath = prefix
	}

	// Normalize the mount path
	mountPath = utils.TrimRight(mountPath, '/')
	if mountPath == "" {
		mountPath = "/"
	}

	// Create a wrapper app so that the original sub-app is not mutated.
	// This allows the same sub-app to be reused (e.g., mounted on multiple
	// domains) without double-wrapping handlers.
	//
	// The method table comes from the app being mounted on, not from the
	// sub-app: processSubAppsRoutes reads the wrapper's stack at the parent's
	// method indexes, so a wrapper with a shorter stack would put that read out
	// of range.
	wrapperApp := New(Config{
		CaseSensitive:           subApp.config.CaseSensitive,
		StrictRouting:           subApp.config.StrictRouting,
		RegexHandler:            subApp.config.RegexHandler,
		RequestMethods:          d.app.config.RequestMethods,
		DisableHeadAutoRegister: subApp.config.DisableHeadAutoRegister,
	})
	// Every route the wrapper carries is filtered by the pattern of the mount
	// it came from, which is what the automatic HEAD routes have to respect:
	// the apps behind one wrapper do not all answer for the same hostnames.
	wrapperApp.mountFields.hostScopedRoutes = true

	// Clone routes from the sub-app with domain-wrapped handlers. The clone
	// also collects the constraints of every app it walks, since the wrapper is
	// what the routes are re-parsed against when the mount is expanded, and
	// marks the routes of the apps that register no automatic HEAD routes.
	d.cloneRoutesForDomain(wrapperApp, subApp)

	// Give the clones their automatic HEAD routes. A mounted app normally gets
	// them from startupProcess, which walks the parent's app list — and the
	// wrapper is deliberately not in it, so without this a domain-mounted GET
	// route answers HEAD with 405 where a plain mount answers 200.
	//
	// The routes marked while cloning are passed over, and the marks travel on
	// to the app this one is mounted on, so it withholds them there as well.
	wrapperApp.ensureAutoHeadRoutes()

	// Register the sub-app, and every app it has mounted, as domain mounts of
	// the parent. They are kept out of appList so that their ErrorHandler and
	// view engine only answer for hosts this domain matches, and so that two
	// sub-apps mounted at the same path on different domains do not displace
	// each other.
	//
	// Collect the mounts before taking the parent's lock, and hold only one
	// app's lock at a time while doing it. App.mount writes these lists under
	// the lock of the app they belong to, so reading them unguarded would race
	// — but holding two at once would hang outright when an app is mounted on
	// itself, and could deadlock two goroutines mounting each other's apps.
	//
	// The walk goes through the sub-app's own mount metadata rather than its
	// flattened list, which is filled in as apps are mounted and so misses one
	// mounted on a descendant afterwards; it also picks up the apps each
	// descendant domain-mounted itself, since those records live only on the
	// app they were registered on. The routes reach all of them either way.
	pending := subApp.mountTree()

	// Each app's mount path is written under its own lock, and before the
	// parent's is taken: an app registering a route reads it there, and it may
	// be the parent itself when an app is mounted on itself.
	for i := range pending {
		mount := &pending[i]
		mount.path = getGroupPath(mountPath, mount.path)

		mount.app.mutex.Lock()
		mount.app.mountFields.mountPath = mount.path
		mount.app.mutex.Unlock()
	}

	d.app.mutex.Lock()
	// Support for configs of mounted-apps and sub-mounted-apps
	for i := range pending {
		mount := &pending[i]
		recorded := d.app.newDomainMount(mount.app, mount.path, append(slices.Clone(mount.matchers), d.matcher), mount.ancestors, mount.declared)

		d.app.mountFields.domainAppList = addDomainMount(d.app.mountFields.domainAppList, &recorded)
	}
	d.app.mutex.Unlock()

	// Create a mount group referencing the wrapper app (not the original).
	// During route expansion (processSubAppsRoutes), Fiber reads routes from
	// route.group.app.stack — so using the wrapper ensures expanded routes
	// carry domain-filtered handlers.
	mountGroup := &Group{Prefix: mountPath, app: wrapperApp}

	// Register the mount point - the routes will be expanded during startup
	d.app.register([]string{methodUse}, mountPath, mountGroup)

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(d.app); err != nil {
		panic(err)
	}

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls and route-name calls
	if d.group != nil && !d.group.hasAnyRoute {
		d.group.hasAnyRoute = true
	}

	return d
}

// cloneRoutesForDomain fills dst with domain-filtered clones of src's routes,
// expanding any app src itself has mounted.
//
// The expansion is what keeps a nested mount working. A mount is registered as
// a placeholder route that carries its target app in the unexported group
// field, and copyRoute does not carry that field over, so a cloned placeholder
// would reach startup with mount set and group nil and crash the expansion in
// processSubAppsRoutes. Flattening the descendants here also runs their
// handlers through wrapHandlers, so they stay bound to the domain instead of
// being served on every host.
// Routes cloned from an app that registers no automatic HEAD routes are marked
// as such on the wrapper, so synthesizing them stays off for those and stays on
// for the rest.
func (d *domainRouter) cloneRoutesForDomain(dst, src *App) {
	for m, routes := range d.domainRoutes(dst, src, domainClone{}) {
		dst.stack[m] = routes
	}
}

// domainClone is what a walk down a mounted app's tree carries with it.
type domainClone struct {
	// prefix is the path the app being walked is mounted at, relative to the
	// app the domain mount was registered with
	prefix string
	// chain holds the apps between the mounted sub-app and this one, so a mount
	// cycle terminates. It is a slice rather than a set because it is only ever
	// a few entries deep and each branch needs its own: an app mounted twice on
	// sibling branches is cloned for both.
	chain []*App
	// skipAutoHead marks that an app above this one registers no automatic HEAD
	// routes, and stands in front of these
	skipAutoHead bool
}

// domainRoutes returns src's routes, cloned with domain-filtered handlers and
// with the walk's prefix applied, as one slice per method index of dst. Routes
// of an app src has mounted take the placeholder's position, so the order the
// sub-app registered its routes in is preserved.
func (d *domainRouter) domainRoutes(dst, src *App, walk domainClone) [][]*Route {
	routes := make([][]*Route, len(dst.stack))
	if slices.Contains(walk.chain, src) {
		return routes
	}

	// An app that does not serve HEAD, or that turned the automatic routes off,
	// contributes neither HEAD routes nor HEAD middleware to the clone — so a
	// HEAD route synthesized from one of its GET routes would run the handler
	// with nothing in front of it. That holds for the apps it has mounted too,
	// whose routes it stands in front of, and for no one else: the mount is a
	// tree, and an app opting out says nothing about its siblings or the apps
	// it is mounted inside.
	skipAutoHead := walk.skipAutoHead || src.config.DisableHeadAutoRegister || src.methodInt(MethodHead) < 0

	// Take a snapshot rather than holding the lock for the whole walk: it keeps
	// this from ever holding two apps' locks at once. The routes are copied by
	// value, not by pointer, because registering a route on a path src already
	// serves appends to that route's handlers in place, and renaming one writes
	// its name — a clone taken after the lock was released would race both.
	//
	// A mount placeholder is kept as it is: its target app is read from the
	// group it carries, and the walk into it has to happen unlocked.
	//
	// The snapshot is indexed by dst's methods, not src's: an app is free to
	// configure its own RequestMethods, and the two tables then neither line up
	// nor have to be the same length.
	type sourceRoute struct {
		route       *Route
		owner       *App
		constraints []CustomConstraint
	}

	src.mutex.Lock()
	stack := make([][]sourceRoute, len(routes))
	for m := range stack {
		srcRoutes := src.routesForMethod(dst.method(m))
		stack[m] = make([]sourceRoute, len(srcRoutes))

		for i, route := range srcRoutes {
			if route.mount {
				stack[m][i] = sourceRoute{route: route}
				continue
			}

			// src is itself a wrapper when the sub-app had domain mounts of its
			// own, and already knows the real app behind each of its routes.
			owner := src.routeOwner(route)
			if owner == nil {
				owner = src
			}

			stack[m][i] = sourceRoute{
				route:       src.copyRoute(route),
				owner:       owner,
				constraints: src.routeConstraintsFor(route),
			}
		}
	}
	dst.customConstraints = mergeCustomConstraints(dst.customConstraints, src.customConstraints)
	src.mutex.Unlock()

	// Mounted apps are flattened once and reused across the method indexes:
	// register files a copy of the placeholder in every method's stack, and
	// all of them share the one group it was registered with.
	var mounted map[*Group][][]*Route

	for m := range routes {
		for _, source := range stack[m] {
			if source.route.mount {
				// register only sets mount for a route whose group carries the
				// mounted app, so group and group.app are both set here — the
				// same invariant processSubAppsRoutes relies on.
				group := source.route.group
				if mounted[group] == nil {
					if mounted == nil {
						mounted = make(map[*Group][][]*Route)
					}
					mounted[group] = d.domainRoutes(dst, group.app, domainClone{
						prefix:       getGroupPath(walk.prefix, source.route.path),
						chain:        append(walk.chain, src),
						skipAutoHead: skipAutoHead,
					})
				}

				routes[m] = append(routes[m], mounted[group][m]...)
				continue
			}

			clonedRoute := source.route
			// An empty prefix cannot change the path, and re-parsing the route
			// to reach the same result is the most expensive thing here.
			// The route brings the constraints of the apps that composed its
			// path, and those win over the ones collected from the rest of the
			// tree: two apps mounted side by side can name one constraint
			// differently, and neither binds the other.
			constraints := mergeCustomConstraints(slices.Clone(source.constraints), dst.customConstraints)
			dst.markRouteConstraints(clonedRoute, constraints)

			if walk.prefix != "" {
				dst.addPrefixToRoute(walk.prefix, clonedRoute, src.config.RegexHandler, constraints...)
			}
			clonedRoute.Handlers = d.wrapHandlers(clonedRoute.Handlers)

			// Record the app the route came from, so a request that runs it
			// resolves that app's config rather than one inferred from the host
			// and the path.
			dst.markRouteOwner(clonedRoute, source.owner)

			if skipAutoHead {
				dst.markSkipAutoHead(clonedRoute)
			}

			routes[m] = append(routes[m], clonedRoute)
		}
	}

	return routes
}

// Get registers a route for GET methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Get(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodGet}, path, handler, handlers...)
}

// Head registers a route for HEAD methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Head(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodHead}, path, handler, handlers...)
}

// Post registers a route for POST methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Post(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodPost}, path, handler, handlers...)
}

// Put registers a route for PUT methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Put(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodPut}, path, handler, handlers...)
}

// Delete registers a route for DELETE methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Delete(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodDelete}, path, handler, handlers...)
}

// Connect registers a route for CONNECT methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Connect(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodConnect}, path, handler, handlers...)
}

// Options registers a route for OPTIONS methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Options(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodOptions}, path, handler, handlers...)
}

// Trace registers a route for TRACE methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Trace(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodTrace}, path, handler, handlers...)
}

// Patch registers a route for PATCH methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Patch(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodPatch}, path, handler, handlers...)
}

// Query registers a route for QUERY methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Query(path string, handler any, handlers ...any) Router {
	return d.Add([]string{MethodQuery}, path, handler, handlers...)
}

// Add allows you to specify multiple HTTP methods to register a route.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Add(methods []string, path string, handler any, handlers ...any) Router {
	converted := collectHandlers("domain", append([]any{handler}, handlers...)...)
	wrapped := d.wrapHandlers(converted)
	d.app.register(methods, d.registerPath(path), d.registerGroup(), wrapped...)

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls (before routes) and route-name calls (after routes).
	if d.group != nil && !d.group.hasAnyRoute {
		d.group.hasAnyRoute = true
	}

	return d
}

// All registers the handler on all HTTP methods.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) All(path string, handler any, handlers ...any) Router {
	return d.Add(d.app.config.RequestMethods, path, handler, handlers...)
}

// Group creates a new sub-router with a common prefix, scoped to the domain pattern.
// Routes registered through the returned Router also inherit the domain filter.
func (d *domainRouter) Group(prefix string, handlers ...any) Router {
	fullPrefix := d.registerPath(prefix)

	if len(handlers) > 0 {
		converted := collectHandlers("domain", handlers...)
		wrapped := d.wrapHandlers(converted)
		d.app.register([]string{methodUse}, fullPrefix, d.registerGroup(), wrapped...)
	}

	// Create a new group on the app
	newGrp := &Group{Prefix: fullPrefix, app: d.app, parentGroup: d.group}
	if err := d.app.hooks.executeOnGroupHooks(*newGrp); err != nil {
		panic(err)
	}

	return &domainRouter{
		app:     d.app,
		group:   newGrp,
		matcher: d.matcher,
	}
}

// RouteChain creates a Registering instance for the domain router.
func (d *domainRouter) RouteChain(path string) Register {
	return &domainRegistering{
		domain: d,
		path:   d.registerPath(path),
	}
}

// Route defines routes with a common prefix inside the supplied function,
// scoped to the domain pattern.
func (d *domainRouter) Route(prefix string, fn func(router Router), name ...string) Router {
	if fn == nil {
		panic("route handler 'fn' cannot be nil")
	}

	group := d.Group(prefix)
	if len(name) > 0 {
		group.Name(name[0])
	}

	fn(group)

	return group
}

// Name assigns a name to the most recently registered route.
// When the domain router was created from a Group, this delegates to the
// group's Name method so that group name prefixes are applied correctly.
func (d *domainRouter) Name(name string) Router {
	if d.group != nil {
		d.group.Name(name)
	} else {
		d.app.Name(name)
	}
	return d
}

// Domain creates a new domain router that inherits this domain router's
// group (if any) but uses a different hostname pattern.
func (d *domainRouter) Domain(host string) Router {
	return &domainRouter{
		app:     d.app,
		group:   d.group,
		matcher: parseDomainPattern(host),
	}
}

// domainRegistering provides route registration helpers for a specific path
// on a domain router, implementing the [Register] interface.
type domainRegistering struct {
	domain *domainRouter
	path   string
}

// Verify domainRegistering implements Register at compile time.
var _ Register = (*domainRegistering)(nil)

func (r *domainRegistering) All(handler any, handlers ...any) Register {
	converted := collectHandlers("domain", append([]any{handler}, handlers...)...)
	wrapped := r.domain.wrapHandlers(converted)
	r.domain.app.register([]string{methodUse}, r.path, r.domain.registerGroup(), wrapped...)

	return r
}

func (r *domainRegistering) Get(handler any, handlers ...any) Register {
	return r.Add([]string{MethodGet}, handler, handlers...)
}

func (r *domainRegistering) Head(handler any, handlers ...any) Register {
	return r.Add([]string{MethodHead}, handler, handlers...)
}

func (r *domainRegistering) Post(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPost}, handler, handlers...)
}

func (r *domainRegistering) Put(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPut}, handler, handlers...)
}

func (r *domainRegistering) Delete(handler any, handlers ...any) Register {
	return r.Add([]string{MethodDelete}, handler, handlers...)
}

func (r *domainRegistering) Connect(handler any, handlers ...any) Register {
	return r.Add([]string{MethodConnect}, handler, handlers...)
}

func (r *domainRegistering) Options(handler any, handlers ...any) Register {
	return r.Add([]string{MethodOptions}, handler, handlers...)
}

func (r *domainRegistering) Trace(handler any, handlers ...any) Register {
	return r.Add([]string{MethodTrace}, handler, handlers...)
}

func (r *domainRegistering) Patch(handler any, handlers ...any) Register {
	return r.Add([]string{MethodPatch}, handler, handlers...)
}

func (r *domainRegistering) Query(handler any, handlers ...any) Register {
	return r.Add([]string{MethodQuery}, handler, handlers...)
}

func (r *domainRegistering) Add(methods []string, handler any, handlers ...any) Register {
	converted := collectHandlers("domain", append([]any{handler}, handlers...)...)
	wrapped := r.domain.wrapHandlers(converted)
	r.domain.app.register(methods, r.path, r.domain.registerGroup(), wrapped...)

	return r
}

func (r *domainRegistering) RouteChain(path string) Register {
	return &domainRegistering{
		domain: r.domain,
		path:   getGroupPath(r.path, path),
	}
}
