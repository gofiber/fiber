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
type domainCheckResult struct {
	values  []string
	matched bool
}

// domainMatcher holds the parsed domain pattern for matching against request hostnames.
type domainMatcher struct {
	parts      []string // domain parts split by "."
	paramIdx   []int    // indices of parameter parts
	paramNames []string // parameter names (without ":")
	numParts   int      // total number of parts
}

// parseDomainPattern parses a domain pattern like ":subdomain.example.com"
// into a domainMatcher. Parameter parts start with ":".
func parseDomainPattern(pattern string) domainMatcher {
	pattern = utils.TrimSpace(pattern)
	// Domain names are case-insensitive per RFC 4343
	pattern = utilsstrings.ToLower(pattern)
	// Trim trailing dot of a fully-qualified domain name (RFC 3986),
	// consistent with Fiber's own host normalization in Subdomains().
	pattern = strings.TrimSuffix(pattern, ".")

	parts := strings.Split(pattern, ".")
	m := domainMatcher{
		parts:    parts,
		numParts: len(parts),
	}

	for i, part := range parts {
		if part != "" && part[0] == ':' {
			m.paramIdx = append(m.paramIdx, i)
			m.paramNames = append(m.paramNames, part[1:])
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
func (m *domainMatcher) match(hostname string) (bool, []string) { //nolint:gocritic // named returns conflict with nonamedreturns linter
	// Domain names are case-insensitive per RFC 4343
	hostname = utilsstrings.ToLower(hostname)
	// Trim trailing dot of a fully-qualified domain name (RFC 3986),
	// consistent with Fiber's own host normalization in Subdomains().
	hostname = strings.TrimSuffix(hostname, ".")

	parts := strings.Split(hostname, ".")
	if len(parts) != m.numParts {
		return false, nil
	}

	var paramValues []string
	if len(m.paramIdx) > 0 {
		paramValues = make([]string, len(m.paramIdx))
	}

	paramIter := 0
	for i, patternPart := range m.parts {
		if patternPart != "" && patternPart[0] == ':' {
			paramValues[paramIter] = parts[i]
			paramIter++
		} else if patternPart != parts[i] {
			return false, nil
		}
	}

	return true, paramValues
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
	if params, ok := c.Locals(domainLocalsKey).(*domainParams); ok {
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
			var matched bool
			var values []string
			if cached, ok := c.Locals(cacheKey).(*domainCheckResult); ok {
				matched = cached.matched
				values = cached.values
			} else {
				hostname := c.Hostname()
				matched, values = d.matcher.match(hostname)
				c.Locals(cacheKey, &domainCheckResult{matched: matched, values: values})
			}

			if !matched {
				return c.Next()
			}

			if len(values) > 0 {
				c.Locals(domainLocalsKey, &domainParams{
					names:  d.matcher.paramNames,
					values: values,
				})
			} else {
				// Clear any previously stored domain params when the domain matches
				// but has no parameters, to avoid leaking stale values.
				c.Locals(domainLocalsKey, nil)
			}

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
			// Domain routers do not support mounting sub-apps via Use.
			panic("use: mounting *App via Domain(...).Use is not supported; use app.Mount(...) or domain-specific handlers instead")
		default:
			handler, ok := toFiberHandler(arg)
			if !ok {
				panic(fmt.Sprintf("use: invalid handler %v\n", reflect.TypeOf(arg)))
			}
			handlers = append(handlers, handler)
		}
	}

	if len(prefixes) == 0 {
		prefixes = append(prefixes, prefix)
	}

	wrapped := d.wrapHandlers(handlers)
	for _, prefix := range prefixes {
		d.app.register([]string{methodUse}, d.registerPath(prefix), d.registerGroup(), wrapped...)
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
func (d *domainRouter) Name(name string) Router {
	d.app.Name(name)
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
