package openapi

import (
	"maps"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/schemautil"
	"github.com/gofiber/utils/v2"
)

const (
	securitySchemeBearer = "bearerAuth"
	securitySchemeBasic  = "basicAuth"
	securitySchemeAPIKey = "apiKeyAuth"
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
	kindTimeout
	kindIdempotency
	kindResponseTime
	kindCORS
	kindCompress
	kindRecover
	kindSession
	kindHealthcheck
	kindEarlyData
	// kindCustom is the kind of the first Config.Middleware entry, the next one of the second, and so on.
	kindCustom
)

// maxCustomMiddleware is how many entries of Config.Middleware a middlewareSet can hold.
const maxCustomMiddleware = 64 - int(kindCustom)

// middlewareNames are the names Config.InferMiddleware receives, by kind.
var middlewareNames = [...]string{
	kindKeyAuth:      MiddlewareKeyAuth,
	kindBasicAuth:    MiddlewareBasicAuth,
	kindJWT:          MiddlewareJWT,
	kindCSRF:         MiddlewareCSRF,
	kindRequestID:    MiddlewareRequestID,
	kindLimiter:      MiddlewareLimiter,
	kindETag:         MiddlewareETag,
	kindCache:        MiddlewareCache,
	kindTimeout:      MiddlewareTimeout,
	kindIdempotency:  MiddlewareIdempotency,
	kindResponseTime: MiddlewareResponseTime,
	kindCORS:         MiddlewareCORS,
	kindCompress:     MiddlewareCompress,
	kindRecover:      MiddlewareRecover,
	kindSession:      MiddlewareSession,
	kindHealthcheck:  MiddlewareHealthcheck,
	kindEarlyData:    MiddlewareEarlyData,
}

const authMiddleware middlewareSet = 1<<kindKeyAuth | 1<<kindBasicAuth | 1<<kindJWT

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
	{"timeout", kindTimeout},
	{"idempotency", kindIdempotency},
	{"responsetime", kindResponseTime},
	{"cors", kindCORS},
	{"compress", kindCompress},
	{"recover", kindRecover},
	{"session", kindSession},
	{"healthcheck", kindHealthcheck},
	{"earlydata", kindEarlyData},
}

// contribMiddleware matches gofiber/contrib middleware by source path fragment (contrib/v3/jwt; the unversioned jwt is v2).
var contribMiddleware = [...]struct {
	dir  string
	kind middlewareKind
}{
	{"/gofiber/contrib/v3/jwt@", kindJWT},
	{"/gofiber/contrib/v3/jwt/", kindJWT},
}

// fiberRoot is the directory of Fiber's own source, so only its middleware is matched.
var fiberRoot = sync.OnceValue(func() string {
	fn := runtime.FuncForPC(reflect.ValueOf(fiber.New).Pointer())
	if fn == nil {
		return ""
	}
	file, _ := fn.FileLine(fn.Entry())
	file = filepath.ToSlash(file)
	return file[:max(strings.LastIndexByte(file, '/'), 0)]
})

// fiberMiddlewarePrefixes holds the source path prefix of each fiberMiddlewareDirs entry.
var fiberMiddlewarePrefixes = sync.OnceValue(func() []string {
	root := fiberRoot()
	if root == "" {
		return nil
	}
	prefixes := make([]string, len(fiberMiddlewareDirs))
	for i := range fiberMiddlewareDirs {
		prefixes[i] = root + "/middleware/" + fiberMiddlewareDirs[i].dir + "/"
	}
	return prefixes
})

type middlewareKind uint8

type middlewareSet uint64

func (s middlewareSet) has(kind middlewareKind) bool { return s&(1<<kind) != 0 }

func (s *middlewareSet) add(kind middlewareKind) { *s |= 1 << kind }

// middlewareInFile reports which recognized middleware a source file belongs to, by file because an inlined New names its closure after the caller.
func middlewareInFile(file string) middlewareSet {
	var set middlewareSet
	file = filepath.ToSlash(file)
	for i, prefix := range fiberMiddlewarePrefixes() {
		if strings.HasPrefix(file, prefix) {
			set.add(fiberMiddlewareDirs[i].kind)
		}
	}
	for i := range contribMiddleware {
		if strings.Contains(file, contribMiddleware[i].dir) {
			set.add(contribMiddleware[i].kind)
		}
	}
	return set
}

// handlerMiddleware reports the recognized middleware among handlers: Fiber's own by the file they were compiled
// from, and the ones of Config.Middleware by the code of their constructor.
func handlerMiddleware(handlers []fiber.Handler, custom map[codeSite]middlewareKind) middlewareSet {
	var set middlewareSet
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
		if fn == nil {
			continue
		}
		file, line := fn.FileLine(fn.Entry())
		if kind, ok := custom[codeSite{file: filepath.ToSlash(file), line: line}]; ok {
			set.add(kind)
			continue
		}
		if strings.Contains(fn.Name(), ".New") {
			set |= middlewareInFile(file)
		}
	}
	return set
}

// codeSite is where a function was written. A handler is identified by it rather than by its code pointer,
// as a constructor inlined into its caller gets a copy of its closure for each call site.
type codeSite struct {
	file string
	line int
}

// siteOf returns where the code of a handler was written.
func siteOf(handler fiber.Handler) codeSite {
	fn := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if fn == nil {
		return codeSite{}
	}
	file, line := fn.FileLine(fn.Entry())
	return codeSite{file: filepath.ToSlash(file), line: line}
}

type coveringMiddleware struct {
	prefix string
	domain string
	set    middlewareSet
}

// coversRoute reports whether every request the route can answer passes through a Use route with the given prefix.
// Parameter and optional prefix segments are not claimed where they might not match.
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

// paramSegmentCovers reports whether a parameter prefix segment is passed by every request the route segment answers;
// a constraint is covered only by the same constraint.
func paramSegmentCovers(prefixSegment, routeSegment string) bool {
	if routeSegment == "" {
		return false
	}
	kind, _ := classifySegment(routeSegment)
	if kind == segmentOptional || kind == segmentGreedy {
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
	if kind == segmentParam {
		return constraint(routeSegment) == want
	}
	return literalSatisfies(want, routeSegment)
}

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

func middlewareOn(covering []coveringMiddleware, route *fiber.Route, policy *routePolicy) middlewareSet {
	set := handlerMiddleware(route.InnerHandlers(), policy.custom)
	host, _ := effectiveHost(route.Domains())
	for i := range covering {
		cover := &covering[i]
		if !hostCovers(cover.domain, host) {
			continue
		}
		if coversRoute(cover.prefix, route.Path, policy.equal) {
			set |= cover.set
		}
	}
	if policy.infer != nil && set != 0 {
		for kind := range middlewareKind(64) {
			if !set.has(kind) {
				continue
			}
			if name := policy.middlewareName(kind); name != "" && !policy.infer(route, name) {
				set &^= 1 << kind
			}
		}
	}
	return set
}

// securitySchemes collects the schemes recognized middleware asked for.
type securitySchemes struct {
	keyLookup *APIKeyLookup // where keyauth reads its key from; nil documents a bearer token
	custom    map[string]any
	docs      []MiddlewareDoc
	bearer    bool
	basic     bool
	apiKey    bool
	jwtOnly   bool
}

func newSecuritySchemes(cfg *Config) *securitySchemes {
	return &securitySchemes{jwtOnly: true, keyLookup: cfg.KeyAuthLookup, docs: cfg.Middleware}
}

// keyAuthIsBearer reports whether the keyauth middleware is documented as a bearer token.
func (s *securitySchemes) keyAuthIsBearer() bool { return s.keyLookup == nil }

func (s *securitySchemes) requirement(set middlewareSet) map[string][]string {
	requirement := map[string][]string{}
	if set.has(kindJWT) || (set.has(kindKeyAuth) && s.keyAuthIsBearer()) {
		requirement[securitySchemeBearer] = []string{}
	}
	if set.has(kindKeyAuth) && !s.keyAuthIsBearer() {
		requirement[securitySchemeAPIKey] = []string{}
	}
	if set.has(kindBasicAuth) {
		requirement[securitySchemeBasic] = []string{}
	}
	for i := range s.docs {
		if set.has(kindCustom + middlewareKind(i)) {
			for name, scopes := range s.docs[i].Security {
				requirement[name] = slices.Clone(scopes)
			}
		}
	}
	if len(requirement) == 0 {
		return nil
	}
	return requirement
}

// use records the schemes an operation that made it into the document refers to.
func (s *securitySchemes) use(set middlewareSet) {
	if set.has(kindJWT) || (set.has(kindKeyAuth) && s.keyAuthIsBearer()) {
		s.bearer = true
		if set.has(kindKeyAuth) {
			s.jwtOnly = false
		}
	}
	if set.has(kindKeyAuth) && !s.keyAuthIsBearer() {
		s.apiKey = true
	}
	if set.has(kindBasicAuth) {
		s.basic = true
	}
	for i := range s.docs {
		if !set.has(kindCustom + middlewareKind(i)) {
			continue
		}
		for name, scheme := range s.docs[i].SecuritySchemes {
			if s.custom == nil {
				s.custom = make(map[string]any)
			}
			if _, taken := s.custom[name]; !taken {
				s.custom[name] = scheme
			}
		}
	}
}

func (s *securitySchemes) declarations() map[string]any {
	if s == nil {
		return nil
	}
	schemes := make(map[string]any, 3+len(s.custom))
	maps.Copy(schemes, s.custom)
	if s.bearer {
		bearer := map[string]any{schemaKeyType: "http", "scheme": "bearer"}
		if s.jwtOnly {
			bearer["bearerFormat"] = "JWT"
		}
		schemes[securitySchemeBearer] = bearer
	}
	if s.apiKey {
		schemes[securitySchemeAPIKey] = map[string]any{schemaKeyType: "apiKey", "in": s.keyLookup.In, keyNameField: s.keyLookup.Name}
	}
	if s.basic {
		schemes[securitySchemeBasic] = map[string]any{schemaKeyType: "http", "scheme": "basic"}
	}
	if len(schemes) == 0 {
		return nil
	}
	return schemes
}

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
			cfg.ErrorProduces: contentEntry(&fiber.RouteMediaType{Schema: cfg.ErrorSchema}, reg),
		}
	}
	return resp
}

// applyErrorSchema gives a declared error response that documents no body the error media type and schema
// of the config, as the responses the app's error handler writes have.
func applyErrorSchema(responses map[string]response, declared map[string]fiber.RouteResponse, cfg *Config, reg *schemaRegistry) {
	if cfg.ErrorSchema == nil || cfg.ErrorProduces == "" {
		return
	}
	for code := range declared {
		status, err := strconv.Atoi(code)
		if err != nil || status < 400 {
			continue
		}
		resp := declared[code]
		if resp.Schema != nil || resp.SchemaRef != "" || resp.Example != nil || len(resp.Examples) > 0 ||
			len(resp.Content) > 0 || len(resp.MediaTypes) > 0 {
			continue
		}
		merged := responses[code]
		merged.Content = errorResponse("", cfg, reg).Content
		responses[code] = merged
	}
}

// applyMiddlewareResponses adds the responses and headers recognized middleware writes; declared responses only gain missing headers.
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
	if set.has(kindTimeout) {
		addError("503", "Service Unavailable")
	}
	if set.has(kindRecover) {
		addError("500", "Internal Server Error")
	}
	// An early-data request is let through only for a safe method, and answered with 425 Too Early otherwise.
	if set.has(kindEarlyData) && !isSafeMethod(method) {
		addError("425", "Too Early")
	}
	for i := range cfg.Middleware {
		if !set.has(kindCustom + middlewareKind(i)) {
			continue
		}
		declared := make(map[string]fiber.RouteResponse, len(cfg.Middleware[i].Responses))
		for status, resp := range cfg.Middleware[i].Responses {
			key := keyDefault
			if status != 0 {
				key = utils.FormatInt(int64(status))
			}
			declared[key] = resp
		}
		for key, resp := range convertRouteResponses(declared, cfg.DefaultProduces, reg) {
			if _, ok := responses[key]; !ok {
				responses[key] = resp
			}
		}
	}

	for status, resp := range responses {
		if set.has(kindRequestID) {
			addHeader(&resp, cfg.RequestIDHeader, headerObject("The request identifier", schemaTypeString))
		}
		success := len(status) == 3 && status[0] == '2'
		// The limiter reports its window only on the responses it lets through.
		if set.has(kindLimiter) && !cfg.DisableRateLimitHeaders && success {
			addHeader(&resp, cfg.RateLimitHeaders.Limit, headerObject("Requests allowed per window", schemaTypeInteger))
			addHeader(&resp, cfg.RateLimitHeaders.Remaining, headerObject("Requests left in the window", schemaTypeInteger))
			addHeader(&resp, cfg.RateLimitHeaders.Reset, headerObject("Seconds until the window resets", schemaTypeInteger))
		}
		if set.has(kindETag) && (status == "200" || (status == "304" && readsOnly)) {
			addHeader(&resp, fiber.HeaderETag, headerObject("The entity tag of the response", schemaTypeString))
		}
		if set.has(kindSession) {
			addHeader(&resp, sessionResponseHeader(cfg), headerObject("The session ID, set when the session is created or changed", schemaTypeString))
		}
		if set.has(kindResponseTime) {
			addHeader(&resp, cfg.ResponseTimeHeader, headerObject("How long the request took to handle", schemaTypeString))
		}
		if set.has(kindCORS) {
			addHeader(&resp, fiber.HeaderAccessControlAllowOrigin, headerObject("The origin the CORS policy allows, set when the request names one", schemaTypeString))
		}
		if set.has(kindCompress) && success && status != "204" && status != "205" {
			addHeader(&resp, fiber.HeaderContentEncoding, headerObject("The encoding the response body was compressed with, when the client accepts one", schemaTypeString))
		}
		for i := range cfg.Middleware {
			if set.has(kindCustom + middlewareKind(i)) {
				for name, header := range cfg.Middleware[i].ResponseHeaders {
					addHeader(&resp, name, headerValue(header))
				}
			}
		}
		// The cache reports its status on every response the handler produced, but not on earlier middleware's.
		if set.has(kindCache) && status != "401" && status != "403" && status != "429" {
			addHeader(&resp, cfg.CacheHeader, headerObject("Whether the response was served from cache", schemaTypeString))
		}
		responses[status] = resp
	}
}

// headerValue is a response header of Config.Middleware as the document carries it: a map is the Header Object
// itself, and a bare string is its description.
func headerValue(header any) map[string]any {
	if object, ok := schemautil.AsMap(header); ok {
		return object
	}
	description, _ := header.(string) //nolint:errcheck // anything else documents a header without a description
	return headerObject(description, schemaTypeString)
}

// sessionLookup is where the session middleware keeps the session ID: a "session_id" cookie unless configured.
func sessionLookup(cfg *Config) APIKeyLookup {
	if cfg.SessionLookup != nil {
		return *cfg.SessionLookup
	}
	return APIKeyLookup{In: locCookie, Name: "session_id"}
}

// sessionResponseHeader is the response header that carries the session ID: Set-Cookie for a cookie.
func sessionResponseHeader(cfg *Config) string {
	if lookup := sessionLookup(cfg); lookup.In == locHeader {
		return lookup.Name
	}
	return fiber.HeaderSetCookie
}

func sessionParameter(cfg *Config) fiber.RouteParameter {
	lookup := sessionLookup(cfg)
	return fiber.RouteParameter{
		Name:        lookup.Name,
		In:          fiber.ParamLocation(lookup.In),
		Description: "The session ID, issued by an earlier response",
		Schema:      map[string]any{schemaKeyType: schemaTypeString},
	}
}

// healthResponses are the responses of a route whose handler is the healthcheck middleware: 200 when the probe
// passes and 503 when it fails, written in the configured format.
func healthResponses(cfg *Config) map[string]response {
	mediaType := cfg.HealthcheckProduces
	schema := map[string]any{schemaKeyType: schemaTypeString}
	if !strings.HasPrefix(mediaType, "text/") {
		schema = map[string]any{
			schemaKeyType:       schemaTypeObject,
			schemaKeyProperties: map[string]any{"status": map[string]any{schemaKeyType: schemaTypeString}},
			schemaKeyRequired:   []any{"status"},
		}
	}
	content := func() map[string]map[string]any {
		return map[string]map[string]any{mediaType: {"schema": maps.Clone(schema)}}
	}
	return map[string]response{
		"200": {Description: "OK", Content: content()},
		"503": {Description: "Service Unavailable", Content: content()},
	}
}

func idempotencyParameter(header string) fiber.RouteParameter {
	return fiber.RouteParameter{
		Name:        header,
		In:          fiber.ParamInHeader,
		Description: "A key that makes a retried request replay the first response instead of running again",
		Schema:      map[string]any{schemaKeyType: schemaTypeString},
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
