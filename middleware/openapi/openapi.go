package openapi

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// maxCachedSwaggerPages bounds the UI page cache so a parameterized mount cannot
// grow it without limit. The same bound applies to the per-app cache map.
const maxCachedSwaggerPages = 32

// maxPathVariants bounds the optional-parameter expansion, which is otherwise
// exponential in the number of optional parameters on one route.
const maxPathVariants = 64

// appCache holds one app's artifacts. The spec bytes do not depend on the target
// path so one entry suffices; UI pages embed the spec URL and stay keyed per target.
type appCache struct {
	uiPages  map[string][]byte
	specData []byte
	specRev  uint64
}

// New creates a new middleware handler that serves the generated OpenAPI specification.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)

	// Scoped per *fiber.App: one handler may serve several apps, and one app's
	// revision counter must never validate another's cached bytes.
	var (
		cacheMu sync.Mutex
		caches  = make(map[*fiber.App]*appCache)
	)

	// cacheFor returns the app's cache entry, creating it on first use. Past the
	// bound it is returned but not retained. The caller must hold cacheMu.
	cacheFor := func(app *fiber.App) *appCache {
		cache, ok := caches[app]
		if !ok {
			cache = &appCache{uiPages: make(map[string][]byte)}
			if len(caches) < maxCachedSwaggerPages {
				caches[app] = cache
			}
		}
		return cache
	}

	// specBytes returns the cached spec, regenerating when the route revision
	// moved. Unlocked with defer so a panic below cannot wedge the cache.
	specBytes := func(app *fiber.App) ([]byte, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		rev := app.RoutesRevision()
		if cache.specData == nil || cache.specRev != rev {
			// GetRoutes deep-copies under the router lock, so generation never
			// races registration or the documentation helpers.
			spec := generateSpec(app.GetRoutes(), &cfg)
			data, err := app.Config().JSONEncoder(spec)
			if err != nil {
				return nil, fmt.Errorf("openapi: marshal spec: %w", err)
			}
			cache.specData, cache.specRev = data, rev
		}
		return cache.specData, nil
	}

	// uiBytes returns the app's cached Swagger UI page for targetPath, building
	// it on first use. Same defer rationale as specBytes.
	uiBytes := func(app *fiber.App, targetPath string) ([]byte, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		cache := cacheFor(app)
		if data, ok := cache.uiPages[targetPath]; ok {
			return data, nil
		}
		data, err := buildSwaggerUIPage(targetPath, &cfg, app.Config().JSONEncoder)
		if err != nil {
			return nil, fmt.Errorf("openapi: build swagger ui page: %w", err)
		}
		if len(cache.uiPages) >= maxCachedSwaggerPages {
			// Keys come from the request path, so they are attacker-controlled;
			// drop them all rather than letting junk pin the cache.
			clear(cache.uiPages)
		}
		cache.uiPages[targetPath] = data
		return data, nil
	}

	specPath := utils.TrimRight(normalizedPath(cfg.Path), '/')
	uiPath := utils.TrimRight(normalizedPath(cfg.UIPath), '/')

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Next()
		}

		equal := utils.EqualFold[string]
		if c.App().Config().CaseSensitive {
			equal = stringsEqual
		}

		request := utils.TrimRight(c.Path(), '/')
		route := c.Route()
		isMiddleware := route != nil && route.IsMiddleware()

		// Fast path for prefix-mounted middleware: most requests cannot match
		// either target, so skip target resolution without allocating.
		if isMiddleware && !hasSuffix(request, specPath, equal) && !hasSuffix(request, uiPath, equal) {
			return c.Next()
		}

		targets := resolveTargets(c, specPath, uiPath, equal)

		switch {
		case targets.specOK && equal(request, targets.spec):
			// Cached per app and invalidated by the route revision, so changes
			// are reflected without regenerating on every request.
			data, err := specBytes(c.App())
			if err != nil {
				return err
			}

			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		case targets.uiOK && equal(request, targets.ui):
			data, err := uiBytes(c.App(), targets.spec)
			if err != nil {
				return err
			}

			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(data)
		default:
			return c.Next()
		}
	}
}

func stringsEqual(a, b string) bool { return a == b }

// hasSuffix reports whether s ends with suffix under the given equality
// function (exact or case-folding).
func hasSuffix(s, suffix string, equal func(a, b string) bool) bool {
	return len(s) >= len(suffix) && equal(s[len(s)-len(suffix):], suffix)
}

// specTargets holds the paths this handler answers on. The ok flags are explicit
// because an empty path is meaningful: it is what a root target ("/") trims to.
type specTargets struct {
	spec   string
	ui     string
	specOK bool
	uiOK   bool
}

// resolveTargets derives the spec and UI target paths for this request from the
// route the handler runs on. Prefix middleware serves them under its mount; an
// exact method route is itself the target, its suffix deciding which one.
func resolveTargets(c fiber.Ctx, specPath, uiPath string, equal func(a, b string) bool) specTargets {
	route := c.Route()
	if route == nil {
		return specTargets{spec: specPath, ui: uiPath, specOK: true, uiOK: true}
	}

	if !route.IsMiddleware() {
		// An exact match means the request path IS the registered path, with any
		// pattern parameters already substituted.
		path := utils.TrimRight(c.Path(), '/')
		switch {
		case specPath != "" && hasSuffix(path, specPath, equal):
			base := path[:len(path)-len(specPath)]
			return specTargets{spec: path, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
		case uiPath != "" && hasSuffix(path, uiPath, equal):
			base := path[:len(path)-len(uiPath)]
			return specTargets{spec: base + specPath, ui: path, specOK: true, uiOK: true}
		case len(route.Params) == 0:
			// A fixed custom path (app.Get("/docs", openapi.New())) serves the
			// specification.
			return specTargets{spec: path, specOK: true}
		default:
			// A wildcard registration matches paths the author never enumerated,
			// so serving there would leak the route inventory.
			return specTargets{}
		}
	}

	prefix := routePrefix(route.Path, c.Path())
	// Optional or greedy segments consume a varying number of request segments,
	// so the truncation above cannot say where the mount ends. The request is
	// compared trimmed so a trailing slash still resolves.
	if resolved, ok := resolveDynamicMountPrefix(route.Path, utils.TrimRight(c.Path(), '/'), specPath, uiPath, equal); ok {
		prefix = resolved
	}
	switch {
	case specPath != "" && hasSuffix(prefix, specPath, equal):
		// e.g. app.Use("/v1/openapi.json", openapi.New()): the mount itself is
		// the spec target; the UI, if enabled, sits beside it.
		base := prefix[:len(prefix)-len(specPath)]
		return specTargets{spec: prefix, ui: base + uiPath, specOK: true, uiOK: uiPath != ""}
	case uiPath != "" && hasSuffix(prefix, uiPath, equal):
		// e.g. app.Use("/swagger", New()): the mount is the UI, so the spec must
		// stay under it or the page could never load what it points at.
		return specTargets{spec: prefix + specPath, ui: prefix, specOK: true, uiOK: true}
	default:
		return specTargets{spec: prefix + specPath, ui: prefix + uiPath, specOK: true, uiOK: true}
	}
}

type swaggerUITemplateData struct {
	SwaggerOptionsJSON         string
	Title                      string
	OpenAPIURL                 string
	SwaggerCSSURL              string
	SwaggerBundleURL           string
	SwaggerStandalonePresetURL string
}

var swaggerUITemplate = htemplate.Must(htemplate.New("swagger-ui").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - Swagger UI</title>
    <link
      rel="stylesheet"
      href="{{ .SwaggerCSSURL }}"
    />
  </head>
  <body>
    <div id="swagger-ui" data-swagger-options='{{ .SwaggerOptionsJSON }}'></div>

    <script
      src="{{ .SwaggerBundleURL }}"
      crossorigin="anonymous"
    ></script>
    {{ if .SwaggerStandalonePresetURL }}<script
      src="{{ .SwaggerStandalonePresetURL }}"
      crossorigin="anonymous"
    ></script>{{ end }}
    <script>
      window.addEventListener("load", function () {
        const options = JSON.parse(document.getElementById("swagger-ui").dataset.swaggerOptions);

        const presets = [SwaggerUIBundle.presets.apis];
        const config = {
          url: "{{ .OpenAPIURL }}",
          dom_id: "#swagger-ui",
          persistAuthorization: true,
        };
        if (typeof SwaggerUIStandalonePreset !== "undefined") {
          presets.push(SwaggerUIStandalonePreset);
          config.layout = "StandaloneLayout";
        }
        config.presets = presets;

        window.ui = SwaggerUIBundle({
          ...config,
          ...options,
        });
      });
    </script>
  </body>
</html>
`))

// buildSwaggerUIPage renders the UI page for a spec URL. Options use the app's
// JSON encoder, and html/template escapes the result whichever encoder ran.
func buildSwaggerUIPage(openAPIURL string, cfg *Config, encode utils.JSONMarshal) ([]byte, error) {
	if encode == nil {
		encode = json.Marshal
	}
	swaggerOptionsJSON, err := encode(cfg.SwaggerOptions)
	if err != nil {
		return nil, fmt.Errorf("marshal swagger options: %w", err)
	}
	if len(swaggerOptionsJSON) == 0 || string(swaggerOptionsJSON) == "null" {
		swaggerOptionsJSON = []byte("{}")
	}

	data := swaggerUITemplateData{
		Title:                      cfg.Title,
		OpenAPIURL:                 openAPIURL,
		SwaggerCSSURL:              cfg.SwaggerCSSURL,
		SwaggerBundleURL:           cfg.SwaggerBundleURL,
		SwaggerStandalonePresetURL: cfg.SwaggerStandalonePresetURL,
		SwaggerOptionsJSON:         string(swaggerOptionsJSON),
	}

	var builder strings.Builder
	if err := swaggerUITemplate.Execute(&builder, data); err != nil {
		return nil, fmt.Errorf("execute swagger ui template: %w", err)
	}

	return []byte(builder.String()), nil
}

// normalizedPath returns cfgPath with a leading slash. Defaults for empty
// paths are applied earlier by configDefault.
func normalizedPath(cfgPath string) string {
	if !strings.HasPrefix(cfgPath, "/") {
		return "/" + cfgPath
	}
	return cfgPath
}

// routeTokens returns a segment's routing-relevant characters, dropping escapes
// and <constraint> spans so neither is mistaken for a routing token.
//
// TODO: replace this re-lexing with the router's parsed segments once exposed.
func routeTokens(seg string) string {
	var b strings.Builder
	inConstraint := false
	for i := 0; i < len(seg); i++ {
		switch ch := seg[i]; {
		case ch == '\\':
			i++ // the escaped character is a literal
		case inConstraint:
			if ch == '>' {
				inConstraint = false
			}
		case ch == '<':
			inConstraint = true
		default:
			_ = b.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// openAPIOperationMethods is the set of methods a Path Item can express. CONNECT
// has no OpenAPI operation; `query` is 3.2-only and gated by the caller.
var openAPIOperationMethods = map[string]struct{}{
	fiber.MethodGet:     {},
	fiber.MethodHead:    {},
	fiber.MethodPost:    {},
	fiber.MethodPut:     {},
	fiber.MethodPatch:   {},
	fiber.MethodDelete:  {},
	fiber.MethodOptions: {},
	fiber.MethodTrace:   {},
	fiber.MethodQuery:   {},
}

// isOpenAPIOperationMethod reports whether method maps to a Path Item
// operation key.
func isOpenAPIOperationMethod(method string) bool {
	_, ok := openAPIOperationMethods[method]
	return ok
}

// normalizePathHierarchy blanks template names ("/files/{dir}" → "/files/{}") so
// paths identical up to parameter names share one key.
func normalizePathHierarchy(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	inTemplate := false
	for i := 0; i < len(path); i++ {
		switch ch := path[i]; {
		case inTemplate:
			if ch == '}' {
				inTemplate = false
				_, _ = b.WriteString("{}") //nolint:errcheck // strings.Builder.WriteString never returns an error
			}
		case ch == '{':
			inTemplate = true
		default:
			_ = b.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// resolveDynamicMountPrefix picks the mount prefix when the pattern has optional
// or greedy segments. Every split the segment bounds allow is tried, shortest
// first, and the one whose remainder is a target wins.
func resolveDynamicMountPrefix(pattern, requestPath, specPath, uiPath string, equal func(a, b string) bool) (string, bool) {
	minSegments, maxSegments, dynamic := prefixSegmentBounds(pattern)
	if !dynamic {
		// A static mount is its own prefix, and its escaped form still needs
		// unescaping, which routePrefix handles.
		return "", false
	}

	if maxSegments < 0 || maxSegments > countPathSegments(requestPath) {
		// A greedy segment is unbounded; no split can consume more segments
		// than the request has.
		maxSegments = countPathSegments(requestPath)
	}

	for n := minSegments; n <= maxSegments; n++ {
		candidate, ok := pathPrefixSegments(requestPath, n)
		if !ok {
			break
		}
		if specPath != "" && equal(candidate+specPath, requestPath) {
			return candidate, true
		}
		if uiPath != "" && equal(candidate+uiPath, requestPath) {
			return candidate, true
		}
	}
	return "", false
}

// prefixSegmentBounds reports how many leading segments the mount can consume and
// whether it is dynamic. A greedy segment makes the maximum unbounded (-1).
func prefixSegmentBounds(pattern string) (minSegments, maxSegments int, dynamic bool) { //nolint:nonamedreturns // three ints and a bool read better named
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return 0, 0, false
	}

	greedy := false
	for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
		// Classified by routing tokens only, so a constraint or an escaped
		// character never reads as a parameter.
		tokens := routeTokens(seg)
		switch {
		case strings.ContainsAny(tokens, "*+"):
			// "+" needs at least one segment; "*" may match none.
			if strings.Contains(tokens, "+") {
				minSegments++
			}
			greedy = true
			dynamic = true
		case strings.HasSuffix(tokens, "?"):
			// Optional: zero or one segment.
			maxSegments++
			dynamic = true
		default:
			if strings.Contains(tokens, ":") {
				dynamic = true
			}
			minSegments++
			maxSegments++
		}
	}

	if greedy {
		return minSegments, -1, true
	}
	return minSegments, maxSegments, dynamic
}

// countPathSegments counts the slash-separated segments of a request path.
func countPathSegments(requestPath string) int {
	trimmed := strings.Trim(requestPath, "/")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "/") + 1
}

// pathPrefixSegments returns the leading n segments of requestPath, reporting
// false when it does not have that many segments to give.
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

// routePrefix derives the mount prefix of a middleware route. A static prefix is
// the route path unescaped; a parameterized one takes its values from the
// request, ending at the first greedy or optional segment.
func routePrefix(pattern, requestPath string) string {
	pattern = utils.TrimRight(pattern, '/')
	if pattern == "" {
		return ""
	}

	// Classify segments by routing tokens only: constraints and escaped
	// characters are literals.
	parameterized := false
	segments := 0
	counting := true
	for seg := range strings.SplitSeq(strings.TrimPrefix(pattern, "/"), "/") {
		tokens := routeTokens(seg)
		if strings.ContainsAny(tokens, ":*+") || strings.HasSuffix(tokens, "?") {
			parameterized = true
		}
		if counting && (seg == "" || strings.ContainsAny(tokens, "*+") || strings.HasSuffix(tokens, "?")) {
			counting = false
		}
		if counting {
			segments++
		}
	}
	if !parameterized {
		// Static prefix: serve it in its unescaped (request) form.
		return utils.TrimRight(fiber.RemoveEscapeChar(pattern), '/')
	}
	if segments == 0 {
		return ""
	}

	idx := 0
	for range segments {
		if idx+1 >= len(requestPath) {
			return requestPath
		}
		next := strings.IndexByte(requestPath[idx+1:], '/')
		if next < 0 {
			// The request path ends inside the prefix (e.g. a request for the
			// bare mount path).
			return requestPath
		}
		idx += 1 + next
	}
	return requestPath[:idx]
}

type openAPISpec struct {
	Paths             map[string]map[string]operation `json:"paths"`
	Components        map[string]any                  `json:"components,omitempty"`
	Webhooks          map[string]any                  `json:"webhooks,omitempty"`
	ExternalDocs      *ExternalDocs                   `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Info              openAPIInfo                     `json:"info"`
	OpenAPI           string                          `json:"openapi"`
	Self              string                          `json:"$self,omitempty"`
	JSONSchemaDialect string                          `json:"jsonSchemaDialect,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Servers           []Server                        `json:"servers,omitempty"`
	Security          []map[string][]string           `json:"security,omitempty"`
	Tags              []Tag                           `json:"tags,omitempty"`
}

type openAPIInfo struct {
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Title          string   `json:"title"`
	Version        string   `json:"version"`
	Summary        string   `json:"summary,omitempty"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

type operation struct {
	Responses    map[string]response `json:"responses"`
	RequestBody  *requestBody        `json:"requestBody,omitempty"`  //nolint:tagliatelle // OpenAPI spec uses camelCase
	ExternalDocs map[string]any      `json:"externalDocs,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	extensions   map[string]any      // Arbitrary operation-object fields, merged at marshal time

	OperationID string `json:"operationId,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`

	Parameters []parameter           `json:"parameters,omitempty"`
	Tags       []string              `json:"tags,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`
}

// MarshalJSON merges operation extensions into the operation object without
// clobbering generated keys.
//
//nolint:gocritic // hugeParam: a value receiver is required so map values (which are not addressable) are marshaled through this method
func (o operation) MarshalJSON() ([]byte, error) {
	type alias operation
	base, err := json.Marshal(alias(o))
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal operation: %w", err)
	}
	if len(o.extensions) == 0 {
		return base, nil
	}
	// Spliced into the encoded object rather than round-tripped through
	// map[string]any, which would turn every number into a float64.
	var existing map[string]json.RawMessage
	if err = json.Unmarshal(base, &existing); err != nil {
		return nil, fmt.Errorf("openapi: merge operation extensions: %w", err)
	}

	keys := make([]string, 0, len(o.extensions))
	for key := range o.extensions {
		if _, taken := existing[key]; taken {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return base, nil
	}
	// Sorted so the generated document is byte-stable across runs.
	slices.Sort(keys)

	// Encode first and size the buffer by summing the parts, so the capacity is
	// exact and the computation cannot overflow.
	type extensionPair struct{ key, value []byte }
	pairs := make([]extensionPair, 0, len(keys))
	size := len(base)
	for _, key := range keys {
		encodedKey, kErr := json.Marshal(key)
		if kErr != nil {
			return nil, fmt.Errorf("openapi: marshal operation extension key %q: %w", key, kErr)
		}
		encodedValue, vErr := json.Marshal(o.extensions[key])
		if vErr != nil {
			return nil, fmt.Errorf("openapi: marshal operation extension %q: %w", key, vErr)
		}
		pairs = append(pairs, extensionPair{key: encodedKey, value: encodedValue})
		size += len(encodedKey) + len(encodedValue) + 2 // ':' separator and ',' delimiter
	}

	out := make([]byte, 0, size)
	out = append(out, base[:len(base)-1]...) // everything but the closing brace
	for _, pair := range pairs {
		if len(out) > 1 {
			out = append(out, ',')
		}
		out = append(out, pair.key...)
		out = append(out, ':')
		out = append(out, pair.value...)
	}
	out = append(out, '}')
	return out, nil
}

type response struct {
	Content     map[string]map[string]any `json:"content,omitempty"`
	Headers     map[string]any            `json:"headers,omitempty"`
	Links       map[string]any            `json:"links,omitempty"`
	Description string                    `json:"description"`
}

type parameter struct {
	Schema          map[string]any            `json:"schema,omitempty"`
	Content         map[string]map[string]any `json:"content,omitempty"`
	Example         any                       `json:"example,omitempty"`
	Examples        map[string]any            `json:"examples,omitempty"`
	Explode         *bool                     `json:"explode,omitempty"`
	Description     string                    `json:"description,omitempty"`
	Name            string                    `json:"name"`
	In              string                    `json:"in"`
	Style           string                    `json:"style,omitempty"`
	Required        bool                      `json:"required"`
	Deprecated      bool                      `json:"deprecated,omitempty"`
	AllowEmptyValue bool                      `json:"allowEmptyValue,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	AllowReserved   bool                      `json:"allowReserved,omitempty"`   //nolint:tagliatelle // OpenAPI spec uses camelCase
}

type requestBody struct {
	Content     map[string]map[string]any `json:"content"`
	Description string                    `json:"description,omitempty"`
	Required    bool                      `json:"required,omitempty"`
}

type pathVariant struct {
	PathParamAliases map[string]string
	ParamConstraints map[string]string // Parameter name -> raw "<...>" constraint text
	Path             string
	ParamNames       []string
}

type resolvedParamName struct {
	openAPI string
	raw     string
}

const wildcardParamName = "wildcard"

const (
	paramLocationPath = "path"
	// paramLocationQuerystring is the OpenAPI 3.2 location that describes the
	// entire query string as one value. It must be paired with "content".
	paramLocationQuerystring = "querystring"
	// querystringMediaType is the media type used to wrap a querystring
	// parameter's schema when the author supplied no explicit content map.
	querystringMediaType = "application/x-www-form-urlencoded"

	schemaKeyType     = "type"
	schemaKeyFormat   = "format"
	schemaTypeString  = "string"
	schemaTypeObject  = "object"
	schemaTypeBoolean = "boolean"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
)

// generateOperationID derives a stable operationId from a method and path, e.g.
// ("GET", "/users/{id}") -> "getUsersId", for routes with no explicit Name.
func generateOperationID(method, path string) string {
	var b strings.Builder
	_, _ = b.WriteString(utilsstrings.ToLower(method)) //nolint:errcheck // strings.Builder.WriteString never returns an error
	capNext := true
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case c >= 'a' && c <= 'z':
			if capNext {
				c -= 'a' - 'A'
				capNext = false
			}
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		case (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			capNext = false
			_ = b.WriteByte(c) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		default:
			capNext = true
		}
	}
	return b.String()
}

// uniqueOperationID returns id, or id with a numeric suffix until unique, so the
// document never repeats an operationId.
func uniqueOperationID(id string, used map[string]struct{}) string {
	if id == "" {
		id = "operation"
	}
	candidate := id
	for i := 2; ; i++ {
		if _, exists := used[candidate]; !exists {
			used[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", id, i)
	}
}

// openAPIVersionRank orders the supported OpenAPI versions for comparison.
var openAPIVersionRank = map[string]int{
	versionOpenAPI30: 0,
	versionOpenAPI31: 1,
	versionOpenAPI32: 2,
}

// versionAtLeast reports whether version is greater than or equal to minimum.
func versionAtLeast(version, minimum string) bool {
	return openAPIVersionRank[version] >= openAPIVersionRank[minimum]
}

// generateSpec builds the OpenAPI document from a snapshot of routes (deep
// copies from App.GetRoutes, safe to read without further locking).
func generateSpec(routes []fiber.Route, cfg *Config) openAPISpec {
	paths := make(map[string]map[string]operation)
	// usedOperationIDs guarantees operationId uniqueness across the document,
	// which the OpenAPI specification requires.
	usedOperationIDs := make(map[string]struct{})
	// hierarchyPaths maps a name-blanked template to the path already published
	// for it: the spec forbids two paths differing only in parameter names.
	hierarchyPaths := make(map[string]canonicalPathItem)

	for i := range routes {
		r := &routes[i]
		// A Path Item has a fixed set of operation keys, so a custom method has
		// no valid representation and is skipped. CONNECT has none either.
		if !isOpenAPIOperationMethod(r.Method) {
			continue
		}
		// The OpenAPI `query` operation key exists only in 3.2+; skip QUERY
		// routes for earlier versions, where it cannot be represented.
		if r.Method == fiber.MethodQuery && !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
			continue
		}
		if r.IsMiddleware() {
			continue
		}
		if r.IsAutoHead() {
			continue
		}
		if r.IsHidden() {
			continue
		}

		variants := buildOpenAPIPathVariants(r.Path, r.Params)
		for _, variant := range variants {
			methodLower := utilsstrings.ToLower(r.Method)
			// The router dispatches to the first match, so on a path+method
			// collision the earlier registration describes the behavior.
			if _, exists := paths[variant.Path][methodLower]; exists {
				continue
			}
			// Templates identical up to parameter names MUST NOT coexist, and
			// the router would never dispatch to the later one — first wins.
			hierarchy := normalizePathHierarchy(variant.Path)
			if canonical, exists := hierarchyPaths[hierarchy]; exists {
				// Already published under a different parameter name: join that
				// path item, or drop the operation if its method is taken.
				if canonical.path != variant.Path {
					if _, taken := paths[canonical.path][methodLower]; taken {
						continue
					}
					// Parameters move with the operation: declaring "name" under
					// a path reading "{id}" makes the document invalid.
					adoptCanonicalParamNames(&variant, canonical.params)
					variant.Path = canonical.path
				}
			} else {
				hierarchyPaths[hierarchy] = canonicalPathItem{
					path:   variant.Path,
					params: variant.ParamNames,
				}
			}

			params := make([]parameter, 0, len(variant.ParamNames))
			paramIndex := make(map[string]int, len(variant.ParamNames))
			for _, p := range variant.ParamNames {
				param := parameter{
					Name:     p,
					In:       paramLocationPath,
					Required: true,
					// A "<...>" constraint narrows what the router accepts, so
					// the schema reflects it instead of always using string.
					Schema: pathParamSchema(variant.ParamConstraints[p]),
				}
				params = append(params, param)
				paramIndex[param.In+":"+param.Name] = len(params) - 1
			}

			extras := remapRouteParameters(r.Parameters, variant.PathParamAliases, variant.ParamNames)
			// The "querystring" location exists only in 3.2+; emitting it
			// earlier would make the document invalid.
			if !versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
				extras = dropQuerystringParameters(extras)
			}
			params = mergeRouteParameters(params, paramIndex, extras)

			summary := r.Summary
			if summary == "" {
				summary = r.Method + " " + variant.Path
			}
			description := r.Description

			operationID := r.Name
			if operationID == "" {
				operationID = generateOperationID(r.Method, variant.Path)
			}
			operationID = uniqueOperationID(operationID, usedOperationIDs)

			respType := r.Produces

			responses := convertRouteResponses(r.Responses, respType)
			if len(responses) == 0 {
				status, defaultResp := defaultResponseForMethod(r.Method, respType)
				responses = map[string]response{status: defaultResp}
			}

			reqBody := buildRequestBody(r.RequestBody)
			if reqBody == nil {
				reqType := r.Consumes
				if shouldIncludeRequestBody(reqType, r) {
					reqBody = &requestBody{Content: map[string]map[string]any{reqType: {}}}
				}
			}
			// GET and HEAD operations never carry a request body, and a
			// TRACE request MUST NOT include content (RFC 9110).
			if r.Method == fiber.MethodGet || r.Method == fiber.MethodHead || r.Method == fiber.MethodTrace {
				reqBody = nil
			}

			if paths[variant.Path] == nil {
				paths[variant.Path] = make(map[string]operation)
			}

			paths[variant.Path][methodLower] = operation{
				OperationID:  operationID,
				Summary:      summary,
				Description:  description,
				Tags:         r.Tags,
				Deprecated:   r.Deprecated,
				Parameters:   params,
				RequestBody:  reqBody,
				Responses:    responses,
				Security:     r.Security,
				ExternalDocs: maps.Clone(r.ExternalDocs),
				extensions:   maps.Clone(r.OperationExtensions),
			}
		}
	}

	spec := openAPISpec{
		OpenAPI: cfg.OpenAPIVersion,
		Info: openAPIInfo{
			Title:          cfg.Title,
			Version:        cfg.Version,
			Description:    cfg.Description,
			TermsOfService: cfg.TermsOfService,
			Contact:        cfg.Contact,
			License:        cfg.License,
		},
		Paths: paths,
	}

	spec.Servers = buildServers(cfg)

	if len(cfg.Security) > 0 {
		spec.Security = cfg.Security
	}

	if len(cfg.Tags) > 0 {
		spec.Tags = append([]Tag(nil), cfg.Tags...)
	}

	if cfg.ExternalDocs != nil {
		spec.ExternalDocs = &ExternalDocs{
			Description: cfg.ExternalDocs.Description,
			URL:         cfg.ExternalDocs.URL,
		}
	}

	// The License Object allows identifier or url, never both, and the SPDX
	// identifier itself requires 3.1+. Narrow a copy so the caller's is untouched.
	if cfg.License != nil && cfg.License.Identifier != "" {
		licenseCopy := *cfg.License
		if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
			// identifier wins: it is the more precise of the two.
			licenseCopy.URL = ""
		} else {
			licenseCopy.Identifier = ""
		}
		spec.Info.License = &licenseCopy
	}

	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI31) {
		spec.Info.Summary = cfg.Summary
		spec.JSONSchemaDialect = cfg.JSONSchemaDialect
		if len(cfg.Webhooks) > 0 {
			spec.Webhooks = maps.Clone(cfg.Webhooks)
		}
	}

	if versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32) {
		spec.Self = cfg.Self
	}

	spec.Components = buildComponents(cfg)

	return spec
}

// buildServers resolves the server list, preferring Config.Servers and falling
// back to the single Config.ServerURL for backward compatibility.
func buildServers(cfg *Config) []Server {
	// Server.name is an OpenAPI 3.2+ field.
	allowName := versionAtLeast(cfg.OpenAPIVersion, versionOpenAPI32)
	if len(cfg.Servers) > 0 {
		servers := make([]Server, 0, len(cfg.Servers))
		for _, server := range cfg.Servers {
			if server.URL == "" {
				continue
			}
			if !allowName {
				server.Name = ""
			}
			servers = append(servers, server)
		}
		if len(servers) > 0 {
			return servers
		}
	}
	if cfg.ServerURL != "" {
		return []Server{{URL: cfg.ServerURL}}
	}
	return nil
}

// buildComponents merges the user-provided Components with the configured
// SecuritySchemes without mutating either input.
func buildComponents(cfg *Config) map[string]any {
	if len(cfg.Components) == 0 && len(cfg.SecuritySchemes) == 0 {
		return nil
	}

	components := make(map[string]any, len(cfg.Components)+1)
	maps.Copy(components, cfg.Components)

	if len(cfg.SecuritySchemes) > 0 {
		// Preserve any securitySchemes the user already placed in Components by
		// merging rather than overwriting.
		schemes := make(map[string]any, len(cfg.SecuritySchemes))
		maps.Copy(schemes, stringKeyedEntries(components["securitySchemes"]))
		maps.Copy(schemes, cfg.SecuritySchemes)
		components["securitySchemes"] = schemes
	}

	return components
}

// stringKeyedEntries reads any string-keyed map as map[string]any. A typed map
// such as map[string]MyScheme fails a plain type assertion, and treating that as
// "absent" would drop the caller's schemes instead of merging them.
func stringKeyedEntries(src any) map[string]any {
	if src == nil {
		return nil
	}
	if typed, ok := src.(map[string]any); ok {
		return typed
	}
	value := reflect.ValueOf(src)
	if value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String || value.IsNil() {
		return nil
	}
	out := make(map[string]any, value.Len())
	iter := value.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	return out
}

// dropQuerystringParameters filters out parameters using the OpenAPI 3.2-only
// "querystring" location, returning the input slice unchanged when none match.
func dropQuerystringParameters(extras []fiber.RouteParameter) []fiber.RouteParameter {
	isQuerystring := func(in string) bool {
		return utils.EqualFold(utils.TrimSpace(in), paramLocationQuerystring)
	}
	for i := range extras {
		if !isQuerystring(extras[i].In) {
			continue
		}
		filtered := append([]fiber.RouteParameter(nil), extras[:i]...)
		for j := i + 1; j < len(extras); j++ {
			if isQuerystring(extras[j].In) {
				continue
			}
			filtered = append(filtered, extras[j])
		}
		return filtered
	}
	return extras
}

func mergeRouteParameters(params []parameter, index map[string]int, extras []fiber.RouteParameter) []parameter {
	if len(extras) == 0 {
		return params
	}
	for i := range extras {
		extra := &extras[i]
		if utils.TrimSpace(extra.Name) == "" {
			continue
		}
		location := utilsstrings.ToLower(utils.TrimSpace(extra.In))
		if location == "" {
			location = "query"
		}
		// OpenAPI spec: "example" and "examples" are mutually exclusive.
		// Prefer "examples" when both are provided.
		var paramExample any
		var paramExamples map[string]any
		if copiedExamples := maps.Clone(extra.Examples); len(copiedExamples) > 0 {
			paramExamples = copiedExamples
		} else {
			paramExample = extra.Example
		}
		param := parameter{
			Name:            extra.Name,
			In:              location,
			Description:     extra.Description,
			Required:        extra.Required,
			Example:         paramExample,
			Examples:        paramExamples,
			Deprecated:      extra.Deprecated,
			Style:           extra.Style,
			AllowEmptyValue: extra.AllowEmptyValue,
			AllowReserved:   extra.AllowReserved,
		}
		// A Parameter Object describes its value either with "schema" or with
		// "content", never both and never neither.
		switch content := routeMediaTypeContent(extra.Content); {
		case content != nil:
			param.Content = content
			// "example"/"examples" belong to the media type object when content
			// is used, so they are not repeated at the parameter level.
			param.Example = nil
			param.Examples = nil
		case location == paramLocationQuerystring:
			// The 3.2 "querystring" location must use content, so any supplied
			// schema is wrapped rather than emitting neither key.
			param.Content = map[string]map[string]any{
				querystringMediaType: contentEntry(
					schemaFrom(extra.Schema, extra.SchemaRef, schemaTypeString),
					"", paramExample, paramExamples,
				),
			}
			param.Example = nil
			param.Examples = nil
		default:
			param.Schema = schemaFrom(extra.Schema, extra.SchemaRef, schemaTypeString)
		}
		if extra.Explode != nil {
			explode := *extra.Explode
			param.Explode = &explode
		}
		if param.In == paramLocationPath {
			param.Required = true
			// AddParameter injects {"type": "string"} when no schema is given, so
			// a description-only call would otherwise drop the schema the route
			// constraint derived (":id<int>" documented as a string).
			if idx, ok := index[param.In+":"+param.Name]; ok && extra.SchemaRef == "" && param.Content == nil && isDefaultStringSchema(extra.Schema) {
				param.Schema = params[idx].Schema
			}
		}
		params = appendOrReplaceParameter(params, index, &param)
	}
	return params
}

// isDefaultStringSchema reports whether a schema says nothing beyond the string
// default the route helpers inject.
func isDefaultStringSchema(schema map[string]any) bool {
	switch len(schema) {
	case 0:
		return true
	case 1:
		typ, ok := schema[schemaKeyType].(string)
		return ok && typ == schemaTypeString
	default:
		return false
	}
}

func appendOrReplaceParameter(params []parameter, index map[string]int, p *parameter) []parameter {
	if p == nil || p.Name == "" || p.In == "" {
		return params
	}
	key := p.In + ":" + p.Name
	if idx, ok := index[key]; ok {
		params[idx] = *p
		return params
	}
	index[key] = len(params)
	return append(params, *p)
}

func schemaFrom(schema map[string]any, schemaRef, defaultType string) map[string]any {
	if schemaRef != "" {
		return map[string]any{"$ref": schemaRef}
	}

	copied := maps.Clone(schema)
	if copied == nil {
		copied = map[string]any{}
	}
	if _, ok := copied[schemaKeyType]; !ok && defaultType != "" {
		copied[schemaKeyType] = defaultType
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func contentEntry(schema map[string]any, schemaRef string, example any, examples map[string]any) map[string]any {
	entry := map[string]any{}
	if schemaRef != "" {
		entry["schema"] = map[string]any{"$ref": schemaRef}
	} else if copied := maps.Clone(schema); len(copied) > 0 {
		entry["schema"] = copied
	}
	// OpenAPI spec: "example" and "examples" are mutually exclusive.
	// Prefer "examples" when both are provided.
	if ex := maps.Clone(examples); len(ex) > 0 {
		entry["examples"] = ex
	} else if example != nil {
		entry["example"] = example
	}
	return entry
}

// routeMediaTypeContent builds an OpenAPI content map from per-media-type
// entries, allowing a different schema/example/encoding per content type.
func routeMediaTypeContent(content map[string]fiber.RouteMediaType) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(content))
	for mediaType, mt := range content {
		if mediaType == "" {
			continue
		}
		entry := contentEntry(mt.Schema, mt.SchemaRef, mt.Example, mt.Examples)
		if enc := maps.Clone(mt.Encoding); len(enc) > 0 {
			entry["encoding"] = enc
		}
		if len(entry) == 0 {
			entry = map[string]any{}
		}
		out[mediaType] = entry
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// convertRouteResponses converts response metadata, falling back to the route's
// Produces when a schema or example names no media type.
func convertRouteResponses(routeResponses map[string]fiber.RouteResponse, fallbackMediaType string) map[string]response {
	var merged map[string]response
	if len(routeResponses) > 0 {
		merged = make(map[string]response, len(routeResponses))
		for code, resp := range routeResponses {
			content := routeMediaTypeContent(resp.Content)
			if content == nil {
				mediaTypes := resp.MediaTypes
				if len(mediaTypes) == 0 &&
					(len(resp.Schema) > 0 || resp.SchemaRef != "" || resp.Example != nil || len(resp.Examples) > 0) {
					// A schema or example with no media type would be discarded,
					// so fall back to Produces, then to JSON.
					if fallbackMediaType != "" {
						mediaTypes = []string{fallbackMediaType}
					} else {
						mediaTypes = []string{fiber.MIMEApplicationJSON}
					}
				}
				content = mediaTypesToContent(mediaTypes, resp.Schema, resp.SchemaRef, resp.Example, resp.Examples)
			}
			merged[code] = response{
				Description: resp.Description,
				Content:     content,
				Headers:     maps.Clone(resp.Headers),
				Links:       maps.Clone(resp.Links),
			}
		}
	}
	return merged
}

// canonicalPathItem is the path template already published for a hierarchy,
// together with the parameter names it declares.
type canonicalPathItem struct {
	path   string
	params []string
}

// adoptCanonicalParamNames renames a variant's path parameters to the canonical
// template's. Sharing a hierarchy, they correspond position by position.
func adoptCanonicalParamNames(variant *pathVariant, canonical []string) {
	if len(canonical) != len(variant.ParamNames) {
		// Defensive: a hierarchy match implies equal counts. Renaming on a
		// mismatch would be worse than leaving the names alone.
		return
	}

	renamed := make(map[string]string, len(canonical))
	for i, old := range variant.ParamNames {
		renamed[old] = canonical[i] //nolint:gosec // G602: the guard above makes the two slices the same length
	}

	if len(variant.ParamConstraints) > 0 {
		constraints := make(map[string]string, len(variant.ParamConstraints))
		for name, raw := range variant.ParamConstraints {
			if newName, ok := renamed[name]; ok {
				name = newName
			}
			constraints[name] = raw
		}
		variant.ParamConstraints = constraints
	}

	// Aliases map pattern names onto emitted ones, so they must follow the move
	// for AddParameter(in: "path") to keep matching.
	if len(variant.PathParamAliases) > 0 {
		aliases := make(map[string]string, len(variant.PathParamAliases))
		for raw, emitted := range variant.PathParamAliases {
			if newName, ok := renamed[emitted]; ok {
				emitted = newName
			}
			aliases[raw] = emitted
		}
		variant.PathParamAliases = aliases
	}

	variant.ParamNames = append([]string(nil), canonical...)
}

func remapRouteParameters(extras []fiber.RouteParameter, aliases map[string]string, pathParams []string) []fiber.RouteParameter {
	if len(extras) == 0 {
		return nil
	}
	pathSet := make(map[string]struct{}, len(pathParams))
	for _, name := range pathParams {
		pathSet[name] = struct{}{}
	}
	out := make([]fiber.RouteParameter, 0, len(extras))
	for i := range extras {
		copyExtra := extras[i]
		if utils.EqualFold(utils.TrimSpace(copyExtra.In), paramLocationPath) {
			if mapped, ok := aliases[copyExtra.Name]; ok {
				copyExtra.Name = mapped
			}
			if _, ok := pathSet[copyExtra.Name]; !ok {
				continue
			}
		}
		out = append(out, copyExtra)
	}
	return out
}

func mediaTypesToContent(mediaTypes []string, schema map[string]any, schemaRef string, example any, examples map[string]any) map[string]map[string]any {
	if len(mediaTypes) == 0 {
		return nil
	}
	content := make(map[string]map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue
		}
		entry := contentEntry(schema, schemaRef, example, examples)
		if len(entry) == 0 {
			entry = map[string]any{}
		}
		content[mediaType] = entry
	}
	if len(content) == 0 {
		return nil
	}
	return content
}

func buildRequestBody(routeBody *fiber.RouteRequestBody) *requestBody {
	if routeBody == nil {
		return nil
	}
	content := routeMediaTypeContent(routeBody.Content)
	if content == nil {
		content = mediaTypesToContent(routeBody.MediaTypes, routeBody.Schema, routeBody.SchemaRef, routeBody.Example, routeBody.Examples)
	}
	merged := &requestBody{
		Description: routeBody.Description,
		Required:    routeBody.Required,
		Content:     content,
	}
	// Omit requestBody entirely when content could not be built, as the
	// OpenAPI specification requires at least one media type in content.
	if len(merged.Content) == 0 {
		return nil
	}
	return merged
}

// shouldIncludeRequestBody reports whether an undocumented route gets an implicit
// body: only when Consumes declared a media type. generateSpec gates methods.
func shouldIncludeRequestBody(reqType string, route *fiber.Route) bool {
	return reqType != "" && route != nil
}

// defaultResponseForMethod is the response an undocumented route gets. HEAD
// mirrors GET, since RFC 9110 has it answer as GET would minus the content.
func defaultResponseForMethod(method, mediaType string) (string, response) {
	status := "200"
	description := "OK"

	// RFC 9110 names 204 among the statuses a successful DELETE should return.
	if method == fiber.MethodDelete {
		status = "204"
		description = "No Content"
	}

	resp := response{Description: description}
	if mediaType != "" && status != "204" {
		resp.Content = map[string]map[string]any{
			mediaType: {},
		}
	}
	return status, resp
}

// pathState carries the in-progress OpenAPI path while walking a Fiber route
// pattern; optional parameters fork the walk into include/exclude branches.
type pathState struct {
	aliases     map[string]string
	constraints map[string]string // Parameter name -> raw "<...>" constraint text
	path        string
	params      []string
	paramIdx    int
}

func (s pathState) clone() pathState {
	return pathState{
		path:        s.path,
		params:      append([]string(nil), s.params...),
		aliases:     maps.Clone(s.aliases),
		constraints: maps.Clone(s.constraints),
		paramIdx:    s.paramIdx,
	}
}

func buildOpenAPIPathVariants(fiberPath string, params []string) []pathVariant {
	var (
		length   = len(fiberPath)
		variants []pathVariant
	)

	var walk func(i int, current pathState)
	walk = func(i int, current pathState) {
		for i < length {
			switch fiberPath[i] {
			case ':':
				tokenStart := i + 1
				i = tokenStart
				for i < length {
					c := fiberPath[i]
					// '<' opens a constraint; the rest mirrors path.go's
					// parameterEndChars, ':' and '\\' included.
					if c == '<' || c == '?' || c == '/' || c == '-' || c == '.' || c == ':' || c == '\\' {
						break
					}
					i++
				}
				tokenName := fiberPath[tokenStart:i]

				var rawConstraints string
				if i < length && fiberPath[i] == '<' {
					rawConstraints, i = scanConstraintSpan(fiberPath, i)
				}

				isOptional := i < length && fiberPath[i] == '?'
				if isOptional {
					i++
				}

				resolved := resolveOpenAPIPathParamName(current.paramIdx, tokenName, params)
				includeState := current.clone()
				name := uniquePathParamName(resolved.openAPI, includeState.params)
				includeState.path += "{" + name + "}"
				includeState.params = append(includeState.params, name)
				includeState.aliases[resolved.raw] = name
				if tokenName != "" {
					includeState.aliases[tokenName] = name
				}
				if rawConstraints != "" {
					if includeState.constraints == nil {
						includeState.constraints = make(map[string]string, 1)
					}
					includeState.constraints[name] = rawConstraints
				}
				includeState.paramIdx++

				if isOptional {
					excludeState := current.clone()
					excludeState.paramIdx++
					walk(i, includeState)
					// Each optional parameter doubles the walk, so forking stops
					// at the cap; the fully-populated variant is always emitted.
					if len(variants) < maxPathVariants {
						walk(i, excludeState)
					}
					return
				}
				current = includeState

			case '*', '+':
				isOptional := fiberPath[i] == '*'
				resolved := resolveOpenAPIWildcardParamName(current.paramIdx, params)
				includeState := current.clone()
				name := uniquePathParamName(resolved.openAPI, includeState.params)
				includeState.path += "{" + name + "}"
				includeState.params = append(includeState.params, name)
				includeState.aliases[resolved.raw] = name
				includeState.paramIdx++
				i++

				// "*" also matches no segment at all, so the route serves the
				// path without it; "+" needs at least one.
				if isOptional {
					excludeState := current.clone()
					excludeState.paramIdx++
					walk(i, includeState)
					if len(variants) < maxPathVariants {
						walk(i, excludeState)
					}
					return
				}
				current = includeState

			case '\\':
				// The route grammar escapes the next character, matching it
				// literally (see path.go escapeChar).
				if i+1 < length {
					// Slice the byte; string(byte) would re-encode it as a
					// code point and corrupt multi-byte UTF-8.
					current.path += fiberPath[i+1 : i+2]
				}
				i += 2

			default:
				// Append the whole literal run rather than one byte at a time.
				runStart := i
				for i < length {
					c := fiberPath[i]
					if c == ':' || c == '*' || c == '+' || c == '\\' {
						break
					}
					i++
				}
				current.path += encodeLiteralBraces(fiberPath[runStart:i])
			}
		}

		finalPath := current.path
		if finalPath == "" {
			finalPath = "/"
		}
		variants = append(variants, pathVariant{
			Path:             finalPath,
			ParamNames:       append([]string(nil), current.params...),
			PathParamAliases: current.aliases,
			ParamConstraints: current.constraints,
		})
	}

	walk(0, pathState{
		path:     "",
		params:   nil,
		aliases:  map[string]string{},
		paramIdx: 0,
	})

	seen := make(map[string]struct{}, len(variants))
	unique := make([]pathVariant, 0, len(variants))
	for _, variant := range variants {
		path := variant.Path
		for len(path) > 1 && strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		}
		// An interior empty segment only matches a literal "//" request, so
		// publishing either form would document a nonexistent endpoint.
		if strings.Contains(path, "//") {
			continue
		}
		variant.Path = path

		key := variant.Path + "|" + strings.Join(variant.ParamNames, ",")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, variant)
	}

	return unique
}

// encodeLiteralBraces percent-encodes braces coming from the route's literal
// text, which OpenAPI would otherwise read as an undeclared template expression.
func encodeLiteralBraces(literal string) string {
	if !strings.ContainsAny(literal, "{}") {
		return literal
	}
	replaced := strings.ReplaceAll(literal, "{", "%7B")
	return strings.ReplaceAll(replaced, "}", "%7D")
}

// uniquePathParamName suffixes a name already used in this variant: templates
// must not repeat names, but distinct Fiber parameters can sanitize alike.
func uniquePathParamName(name string, used []string) string {
	candidate := name
	for i := 2; slices.Contains(used, candidate); i++ {
		candidate = fmt.Sprintf("%s_%d", name, i)
	}
	return candidate
}

func resolveOpenAPIPathParamName(paramIdx int, extracted string, params []string) resolvedParamName {
	raw := extracted
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	if raw == "" {
		raw = extracted
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIParamName(raw, paramIdx+1),
	}
}

func resolveOpenAPIWildcardParamName(paramIdx int, params []string) resolvedParamName {
	raw := wildcardParamName
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIWildcardParamName(raw, paramIdx+1),
	}
}

func sanitizeOpenAPIWildcardParamName(name string, idx int) string {
	trimmed := strings.TrimLeft(name, "*+")
	if trimmed == "" {
		trimmed = wildcardParamName
	}
	trimmed = strings.TrimLeft(trimmed, "_.-")
	if trimmed == "" {
		trimmed = wildcardParamName
	}
	if trimmed[0] >= '0' && trimmed[0] <= '9' {
		trimmed = wildcardParamName + trimmed
	}
	if !strings.HasPrefix(trimmed, wildcardParamName) {
		trimmed = wildcardParamName + trimmed
	}
	return sanitizeOpenAPIParamName(trimmed, idx)
}

func sanitizeOpenAPIParamName(name string, idx int) string {
	trimmed := strings.TrimLeft(name, "*+")
	if trimmed == "" {
		trimmed = name
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			_, _ = builder.WriteRune(r) //nolint:errcheck // strings.Builder.WriteRune never returns an error
			continue
		}
		_ = builder.WriteByte('_') //nolint:errcheck // strings.Builder.WriteByte never returns an error
	}
	sanitized := builder.String()
	if sanitized == "" {
		return fmt.Sprintf("param%d", idx)
	}
	return sanitized
}
