package openapi

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

const (
	securitySchemeBearer = "bearerAuth"
	securitySchemeBasic  = "basicAuth"
)

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

const authMiddleware middlewareSet = 1<<kindKeyAuth | 1<<kindBasicAuth | 1<<kindJWT

// fiberMiddlewareDirs lists the Fiber middleware directories the document can describe.
var fiberMiddlewareDirs = [...]struct {
	dir  string
	kind middlewareKind
}{
	{"keyauth", kindKeyAuth},
	{"basicauth", kindBasicAuth},
	{"csrf", kindCSRF},
	{"requestid", kindRequestID},
	{"limiter", kindLimiter},
	{"etag", kindETag},
	{"cache", kindCache},
}

// contribMiddleware matches gofiber/contrib middleware by source path fragment. Fiber v3 uses
// gofiber/contrib/v3/jwt; the unversioned contrib/jwt is v2 and returns a v2 handler. Fragments end at
// the package directory so a versioned module cache ("@v1.2.3") and a checkout both match.
var contribMiddleware = [...]struct {
	dir  string
	kind middlewareKind
}{
	{"/gofiber/contrib/v3/jwt@", kindJWT},
	{"/gofiber/contrib/v3/jwt/", kindJWT},
}

// fiberRoot is the directory of Fiber's own source (checkout, module cache, vendor or -trimpath),
// so middleware is matched against Fiber's files and not any "middleware/keyauth" directory.
var fiberRoot = sync.OnceValue(func() string {
	fn := runtime.FuncForPC(reflect.ValueOf(fiber.New).Pointer())
	if fn == nil {
		return ""
	}
	file, _ := fn.FileLine(fn.Entry())
	file = filepath.ToSlash(file)
	return file[:max(strings.LastIndexByte(file, '/'), 0)]
})

type middlewareKind uint8

type middlewareSet uint16

func (s middlewareSet) has(kind middlewareKind) bool { return s&(1<<kind) != 0 }

func (s *middlewareSet) add(kind middlewareKind) { *s |= 1 << kind }

// middlewareInFile reports which recognized middleware a source file belongs to. Matching is by
// file because an inlined New names its closure after the caller's package.
func middlewareInFile(file string) middlewareSet {
	var set middlewareSet
	file = filepath.ToSlash(file)
	if root := fiberRoot(); root != "" {
		for i := range fiberMiddlewareDirs {
			if strings.HasPrefix(file, root+"/middleware/"+fiberMiddlewareDirs[i].dir+"/") {
				set.add(fiberMiddlewareDirs[i].kind)
			}
		}
	}
	for i := range contribMiddleware {
		if strings.Contains(file, contribMiddleware[i].dir) {
			set.add(contribMiddleware[i].kind)
		}
	}
	return set
}

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
		set |= middlewareInFile(file)
	}
	return set
}

type coveringMiddleware struct {
	prefix string
	domain string
	set    middlewareSet
}

// coversRoute reports whether every request the route can answer passes through a Use route with
// the given prefix. Literal prefix segments only cover the same literal and optional ones may consume
// nothing, so neither is claimed where the route has a parameter or the prefix may vanish.
func coversRoute(prefix, path string, equal segmentEqual) bool {
	prefix = utils.TrimRight(prefix, '/')
	if prefix == "" {
		return true
	}
	prefixRest := strings.TrimPrefix(prefix, "/")
	pathRest := strings.TrimPrefix(utils.TrimRight(path, '/'), "/")
	pathDone := false
	for {
		segment, nextPrefix, more := utils.CutByte(prefixRest, '/')
		kind, tokens := classifySegment(segment)
		switch {
		case kind == segmentGreedy:
			return !pathDone || !strings.Contains(tokens, "+")
		case kind == segmentOptional, pathDone:
			return false
		}

		var pathSegment string
		var found bool
		pathSegment, pathRest, found = utils.CutByte(pathRest, '/')
		pathDone = !found
		if strings.HasPrefix(tokens, ":") {
			if !paramSegmentCovers(segment, pathSegment) {
				return false
			}
		} else if routeTokens(pathSegment) != pathSegment || !equal(fiber.RemoveEscapeChar(segment), fiber.RemoveEscapeChar(pathSegment)) {
			return false
		}
		if !more {
			return true
		}
		prefixRest = nextPrefix
	}
}

// paramSegmentCovers reports whether a prefix segment that is a parameter is passed by every request the route
// segment answers. A parameter needs a non-empty value, so the route must have a segment there that is
// neither optional nor greedy. A constraint narrows the values, so one is covered only by a parameter with
// the same constraint; a literal might not satisfy it.
func paramSegmentCovers(prefixSegment, routeSegment string) bool {
	if routeSegment == "" {
		return false
	}
	if kind, _ := classifySegment(routeSegment); kind == segmentOptional || kind == segmentGreedy {
		return false
	}
	constraint := func(segment string) string {
		if i := strings.IndexByte(segment, '<'); i >= 0 {
			return segment[i:]
		}
		return ""
	}
	want := constraint(prefixSegment)
	if want == "" {
		return true
	}
	kind, _ := classifySegment(routeSegment)
	if kind == segmentParam {
		return constraint(routeSegment) == want
	}
	return literalSatisfies(want, routeSegment)
}

// literalSatisfies reports whether a literal route segment is accepted by one of the simple constraints it
// can be checked against; any other constraint is assumed not to accept it.
func literalSatisfies(constraint, literal string) bool {
	if literal == "" {
		return false
	}
	switch constraint {
	case "<int>":
		digits := strings.TrimPrefix(strings.TrimPrefix(literal, "-"), "+")
		return digits != "" && strings.Trim(digits, "0123456789") == ""
	case "<alpha>":
		return strings.Trim(literal, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") == ""
	case "<bool>":
		return literal == "true" || literal == "false"
	default:
		return false
	}
}

func middlewareOn(covering []coveringMiddleware, route *fiber.Route, equal segmentEqual) middlewareSet {
	set := handlerMiddleware(route.InnerHandlers())
	for i := range covering {
		cover := &covering[i]
		if cover.domain != "" && cover.domain != route.Domain() {
			continue
		}
		if coversRoute(cover.prefix, route.Path, equal) {
			set |= cover.set
		}
	}
	return set
}

// securitySchemes collects the schemes recognized middleware asked for, so components.securitySchemes
// can declare the ones the user did not.
type securitySchemes struct {
	bearer bool
	basic  bool
	// jwtOnly stays true while every bearer scheme came from the JWT middleware (the token format is then nameable).
	jwtOnly bool
}

func newSecuritySchemes() *securitySchemes {
	return &securitySchemes{jwtOnly: true}
}

// requirement returns the requirement for all middleware at once, since each chained one must pass.
func (*securitySchemes) requirement(set middlewareSet) map[string][]string {
	requirement := map[string][]string{}
	if set.has(kindKeyAuth) || set.has(kindJWT) {
		requirement[securitySchemeBearer] = []string{}
	}
	if set.has(kindBasicAuth) {
		requirement[securitySchemeBasic] = []string{}
	}
	if len(requirement) == 0 {
		return nil
	}
	return requirement
}

// use records the schemes an operation that made it into the document refers to, so a route dropped as a
// duplicate declares none.
func (s *securitySchemes) use(set middlewareSet) {
	if set.has(kindKeyAuth) || set.has(kindJWT) {
		s.bearer = true
		if set.has(kindKeyAuth) {
			s.jwtOnly = false
		}
	}
	if set.has(kindBasicAuth) {
		s.basic = true
	}
}

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

// isSafeMethod reports whether CSRF lets the method through without a token (RFC 9110 safe methods and QUERY).
func isSafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery:
		return true
	default:
		return false
	}
}

func headerObject(description, schemaType string) map[string]any {
	header := map[string]any{"schema": map[string]any{schemaKeyType: schemaType}}
	if description != "" {
		header["description"] = description
	}
	return header
}

func addHeader(resp *response, name string, header map[string]any) {
	if _, ok := resp.Headers[name]; ok {
		return
	}
	if resp.Headers == nil {
		resp.Headers = make(map[string]any)
	}
	resp.Headers[name] = header
}

func errorResponse(description string, cfg *Config, reg *schemaRegistry) response {
	resp := response{Description: description}
	if cfg.ErrorProduces != "" {
		resp.Content = map[string]map[string]any{
			cfg.ErrorProduces: contentEntry(fiber.RouteMediaType{Schema: cfg.ErrorSchema}, reg),
		}
	}
	return resp
}

// applyMiddlewareResponses adds the responses and headers recognized middleware writes. Responses the
// route already declares keep their description and content and only gain missing headers.
func applyMiddlewareResponses(responses map[string]response, method string, set middlewareSet, cfg *Config, reg *schemaRegistry) {
	readsOnly := method == fiber.MethodGet || method == fiber.MethodHead
	addError := func(status, description string) {
		if _, ok := responses[status]; !ok {
			responses[status] = errorResponse(description, cfg, reg)
		}
	}
	if set.has(kindKeyAuth) || set.has(kindBasicAuth) || set.has(kindJWT) {
		addError("401", "Unauthorized")
		resp := responses["401"]
		addHeader(&resp, fiber.HeaderWWWAuthenticate, headerObject("The authentication challenge", schemaTypeString))
		responses["401"] = resp
	}
	if set.has(kindCSRF) && !isSafeMethod(method) {
		addError("403", "Forbidden")
	}
	if set.has(kindLimiter) {
		addError("429", "Too Many Requests")
		// The limiter drops Retry-After along with the rate limit headers when its headers are disabled.
		if !cfg.DisableRateLimitHeaders {
			resp := responses["429"]
			addHeader(&resp, fiber.HeaderRetryAfter, headerObject("Seconds until the limit resets", schemaTypeInteger))
			responses["429"] = resp
		}
	}
	if set.has(kindETag) && readsOnly {
		if _, ok := responses["304"]; !ok {
			responses["304"] = response{Description: "Not Modified"}
		}
	}

	for status, resp := range responses {
		if set.has(kindRequestID) {
			addHeader(&resp, cfg.RequestIDHeader, headerObject("The request identifier", schemaTypeString))
		}
		success := len(status) == 3 && status[0] == '2'
		// The limiter reports its window after the handler ran, so only on the responses it lets through; its
		// own 429 carries Retry-After instead, and a response an earlier middleware sent never reached it.
		if set.has(kindLimiter) && !cfg.DisableRateLimitHeaders && success {
			addHeader(&resp, cfg.RateLimitHeaders.Limit, headerObject("Requests allowed per window", schemaTypeInteger))
			addHeader(&resp, cfg.RateLimitHeaders.Remaining, headerObject("Requests left in the window", schemaTypeInteger))
			addHeader(&resp, cfg.RateLimitHeaders.Reset, headerObject("Seconds until the window resets", schemaTypeInteger))
		}
		// ETag is generated for a 200 with a body, whatever the method, and repeated on the 304 it answers.
		if set.has(kindETag) && (status == "200" || (status == "304" && readsOnly)) {
			addHeader(&resp, fiber.HeaderETag, headerObject("The entity tag of the response", schemaTypeString))
		}
		// The cache reports its status on every response the handler produced, errors included, but not on
		// the ones an earlier middleware sent (a 401, a 403) or the limiter's 429.
		if set.has(kindCache) && status != "401" && status != "403" && status != "429" {
			addHeader(&resp, cfg.CacheHeader, headerObject("Whether the response was served from cache", schemaTypeString))
		}
		responses[status] = resp
	}
}

func csrfParameter(header string) fiber.RouteParameter {
	return fiber.RouteParameter{
		Name:        header,
		In:          fiber.ParamInHeader,
		Required:    true,
		Description: "The CSRF token issued to the client",
		Schema:      map[string]any{schemaKeyType: schemaTypeString},
	}
}
