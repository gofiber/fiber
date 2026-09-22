package openapi

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// middlewareKind identifies a Fiber middleware the document can describe.
type middlewareKind uint8

const (
	kindKeyAuth middlewareKind = iota
	kindBasicAuth
	kindJWT
	kindCSRF
	kindRequestID
	kindLimiter
	kindETag
	kindCache
)

// middlewareSet is the set of recognized middleware on a request's path.
type middlewareSet uint16

func (s middlewareSet) has(kind middlewareKind) bool { return s&(1<<kind) != 0 }

func (s *middlewareSet) add(kind middlewareKind) { *s |= 1 << kind }

// knownMiddleware lists the source directory each recognized middleware's
// handler is compiled from. The handler is a closure returned by New, so it
// is matched by the file it lives in rather than by name: when New is small
// enough to inline, the runtime names its closure after the caller's package.
var knownMiddleware = [...]struct {
	dir  string
	kind middlewareKind
}{
	{"/middleware/keyauth/", kindKeyAuth},
	{"/middleware/basicauth/", kindBasicAuth},
	{"/contrib/jwt/", kindJWT},
	{"/middleware/csrf/", kindCSRF},
	{"/middleware/requestid/", kindRequestID},
	{"/middleware/limiter/", kindLimiter},
	{"/middleware/etag/", kindETag},
	{"/middleware/cache/", kindCache},
}

// handlerMiddleware reports which recognized middleware the handlers are.
func handlerMiddleware(handlers []fiber.Handler) middlewareSet {
	var set middlewareSet
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
		if fn == nil || !strings.Contains(fn.Name(), ".New.") {
			continue
		}
		file, _ := fn.FileLine(fn.Entry())
		file = filepath.ToSlash(file)
		for i := range knownMiddleware {
			if strings.Contains(file, knownMiddleware[i].dir) {
				set.add(knownMiddleware[i].kind)
			}
		}
	}
	return set
}

// coveringMiddleware is a Use route registered ahead of the routes of one
// method, remembered by the prefix and host it applies to.
type coveringMiddleware struct {
	prefix string
	domain string
	set    middlewareSet
}

// coversPath reports whether a Use route's prefix matches path the way the
// router matches it: entirely, or up to a segment boundary.
func coversPath(prefix, path string) bool {
	if prefix == "" || prefix == "/" {
		return true
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	return len(path) == len(prefix) || path[len(prefix)] == '/'
}

// middlewareOn is the recognized middleware a request to route passes
// through: the Use routes registered ahead of it whose prefix and host cover
// it, then the route's own handlers.
func middlewareOn(covering []coveringMiddleware, route *fiber.Route) middlewareSet {
	set := handlerMiddleware(route.Handlers)
	for i := range covering {
		cover := &covering[i]
		if cover.domain != "" && cover.domain != route.Domain() {
			continue
		}
		if coversPath(cover.prefix, route.Path) {
			set |= cover.set
		}
	}
	return set
}

// Names of the security schemes the document declares for recognized
// authentication middleware, and of the headers they document.
const (
	securitySchemeBearer = "bearerAuth"
	securitySchemeBasic  = "basicAuth"

	headerWWWAuthenticate = "WWW-Authenticate"
	headerRetryAfter      = "Retry-After"
	headerRateLimitLimit  = "X-RateLimit-Limit"
	headerRateLimitRemain = "X-RateLimit-Remaining"
	headerRateLimitReset  = "X-RateLimit-Reset"
	headerCSRFToken       = "X-Csrf-Token" //nolint:gosec // G101: a header name, not a credential
	headerCacheStatus     = "X-Cache"
)

// securitySchemes keeps the schemes the recognized middleware asked for while
// generating, so components.securitySchemes can declare the ones the user
// did not.
type securitySchemes struct {
	bearer bool
	basic  bool
	// jwtOnly stays true while every bearer scheme came from the JWT
	// middleware, in which case the document can name the token format.
	jwtOnly bool
}

func newSecuritySchemes() *securitySchemes {
	return &securitySchemes{jwtOnly: true}
}

// requirement returns the security requirement the middleware imposes: all
// of them at once, since a chained middleware each has to pass.
func (s *securitySchemes) requirement(set middlewareSet) map[string][]string {
	requirement := map[string][]string{}
	if set.has(kindKeyAuth) || set.has(kindJWT) {
		requirement[securitySchemeBearer] = []string{}
		s.bearer = true
		if set.has(kindKeyAuth) {
			s.jwtOnly = false
		}
	}
	if set.has(kindBasicAuth) {
		requirement[securitySchemeBasic] = []string{}
		s.basic = true
	}
	if len(requirement) == 0 {
		return nil
	}
	return requirement
}

// declarations returns the scheme objects for the schemes in use.
func (s *securitySchemes) declarations() map[string]any {
	if s == nil {
		return nil
	}
	schemes := make(map[string]any, 2)
	if s.bearer {
		bearer := map[string]any{"type": "http", "scheme": "bearer"}
		if s.jwtOnly {
			bearer["bearerFormat"] = "JWT"
		}
		schemes[securitySchemeBearer] = bearer
	}
	if s.basic {
		schemes[securitySchemeBasic] = map[string]any{"type": "http", "scheme": "basic"}
	}
	if len(schemes) == 0 {
		return nil
	}
	return schemes
}

// isSafeMethod reports whether the CSRF middleware lets a method through
// without a token: the methods RFC 9110 defines as safe, and QUERY.
func isSafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery:
		return true
	default:
		return false
	}
}

// headerObject builds a Header Object with a schema of the given type.
func headerObject(description, schemaType string) map[string]any {
	header := map[string]any{"schema": map[string]any{schemaKeyType: schemaType}}
	if description != "" {
		header["description"] = description
	}
	return header
}

// addHeader documents a header on a response unless it already has one.
func addHeader(resp *response, name string, header map[string]any) {
	if _, ok := resp.Headers[name]; ok {
		return
	}
	if resp.Headers == nil {
		resp.Headers = make(map[string]any)
	}
	resp.Headers[name] = header
}

// errorResponse describes a response the app's error handler writes.
func errorResponse(description string, cfg *Config, reg *schemaRegistry) response {
	resp := response{Description: description}
	if cfg.ErrorProduces != "" {
		resp.Content = map[string]map[string]any{
			cfg.ErrorProduces: contentEntry(cfg.ErrorSchema, "", nil, nil, reg),
		}
	}
	return resp
}

// applyMiddlewareResponses adds what the recognized middleware on a route
// writes: the responses it sends on its own and the headers it sets on every
// response. A response the route already declares keeps its description and
// content, gaining only headers it lacks.
func applyMiddlewareResponses(responses map[string]response, method string, set middlewareSet, cfg *Config, reg *schemaRegistry) {
	addError := func(status, description string) {
		if _, ok := responses[status]; !ok {
			responses[status] = errorResponse(description, cfg, reg)
		}
	}
	if set.has(kindKeyAuth) || set.has(kindBasicAuth) || set.has(kindJWT) {
		addError("401", "Unauthorized")
		resp := responses["401"]
		addHeader(&resp, headerWWWAuthenticate, headerObject("The authentication challenge", schemaTypeString))
		responses["401"] = resp
	}
	if set.has(kindCSRF) && !isSafeMethod(method) {
		addError("403", "Forbidden")
	}
	if set.has(kindLimiter) {
		addError("429", "Too Many Requests")
		resp := responses["429"]
		addHeader(&resp, headerRetryAfter, headerObject("Seconds until the limit resets", schemaTypeInteger))
		responses["429"] = resp
	}
	if set.has(kindETag) && (method == fiber.MethodGet || method == fiber.MethodHead) {
		if _, ok := responses["304"]; !ok {
			responses["304"] = response{Description: "Not Modified"}
		}
	}

	for status, resp := range responses {
		if set.has(kindRequestID) {
			addHeader(&resp, fiber.HeaderXRequestID, headerObject("The request identifier", schemaTypeString))
		}
		if set.has(kindLimiter) {
			addHeader(&resp, headerRateLimitLimit, headerObject("Requests allowed per window", schemaTypeInteger))
			addHeader(&resp, headerRateLimitRemain, headerObject("Requests left in the window", schemaTypeInteger))
			addHeader(&resp, headerRateLimitReset, headerObject("Seconds until the window resets", schemaTypeInteger))
		}
		if set.has(kindETag) && len(status) == 3 && status[0] == '2' {
			addHeader(&resp, fiber.HeaderETag, headerObject("The entity tag of the response", schemaTypeString))
		}
		if set.has(kindCache) {
			addHeader(&resp, headerCacheStatus, headerObject("Whether the response was served from cache", schemaTypeString))
		}
		responses[status] = resp
	}
}

// csrfParameter is the header parameter the CSRF middleware requires on an
// unsafe request.
func csrfParameter() fiber.RouteParameter {
	return fiber.RouteParameter{
		Name:        headerCSRFToken,
		In:          "header",
		Required:    true,
		Description: "The CSRF token issued to the client",
		Schema:      map[string]any{schemaKeyType: schemaTypeString},
	}
}
