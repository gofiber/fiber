// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"reflect"
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
	params  *domainParams // pre-built params (nil if no params or no match)
	matched bool
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
				matched, values := d.matcher.match(hostname)
				check = &domainCheckResult{matched: matched}
				if matched && len(values) > 0 {
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

			if !check.matched {
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
			return d.mount(prefix, subApp)
		}

		wrapped := d.wrapHandlers(handlers)
		d.app.register([]string{methodUse}, d.registerPath(prefix), d.registerGroup(), wrapped...)
	}

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls (before routes) and route-name calls (after routes).
	if d.group != nil && !d.group.anyRouteDefined {
		d.group.anyRouteDefined = true
	}

	return d
}

// mount attaches a sub-app instance to the domain router at the specified prefix.
// All routes from the sub-app will only be accessible when the request hostname
// matches the domain pattern.
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

	// Wrap all handlers in the sub-app with domain checking BEFORE mounting
	// This ensures that when the routes are expanded during startup, they already
	// have domain filtering applied.
	for m := range subApp.stack {
		for _, route := range subApp.stack[m] {
			if len(route.Handlers) > 0 {
				route.Handlers = d.wrapHandlers(route.Handlers)
			}
		}
	}

	d.app.mutex.Lock()
	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subAppInstance := range subApp.mountFields.appList {
		path := getGroupPath(mountPath, mountedPrefixes)

		subAppInstance.mountFields.mountPath = path
		d.app.mountFields.appList[path] = subAppInstance
	}
	d.app.mutex.Unlock()

	// Create a mount group that references the sub-app
	mountGroup := &Group{Prefix: mountPath, app: subApp}

	// Register the mount point - the routes will be expanded during startup
	d.app.register([]string{methodUse}, mountPath, mountGroup)

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(d.app); err != nil {
		panic(err)
	}

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls and route-name calls
	if d.group != nil && !d.group.anyRouteDefined {
		d.group.anyRouteDefined = true
	}

	return d
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

// Add allows you to specify multiple HTTP methods to register a route.
// The handler only executes when the request hostname matches the domain pattern.
func (d *domainRouter) Add(methods []string, path string, handler any, handlers ...any) Router {
	converted := collectHandlers("domain", append([]any{handler}, handlers...)...)
	wrapped := d.wrapHandlers(converted)
	d.app.register(methods, d.registerPath(path), d.registerGroup(), wrapped...)

	// Mark the underlying group so Name() can distinguish between
	// group-name-prefix calls (before routes) and route-name calls (after routes).
	if d.group != nil && !d.group.anyRouteDefined {
		d.group.anyRouteDefined = true
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
