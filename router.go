// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"math/bits"
	"reflect"
	"slices"
	"sync/atomic"

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
	// Summary sets a short summary for the most recently registered route.
	Summary(sum string) Router
	// Description sets a human-readable description for the most recently
	// registered route.
	Description(desc string) Router
	// Consumes sets the request media type for the most recently
	// registered route.
	Consumes(typ string) Router
	// Produces sets the response media type for the most recently
	// registered route.
	Produces(typ string) Router
	// RequestBody documents the request body for the most recently
	// registered route.
	RequestBody(description string, required bool, mediaTypes ...string) Router
	// RequestBodyWithExample documents the request body for the most recently
	// registered route with schema references and examples.
	RequestBodyWithExample(description string, required bool, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router
	// Parameter documents an input parameter for the most recently
	// registered route.
	Parameter(name, in string, required bool, schema map[string]any, description string) Router
	// ParameterWithExample documents an input parameter for the most recently
	// registered route, including schema references and examples.
	ParameterWithExample(name, in string, required bool, schema map[string]any, schemaRef, description string, example any, examples map[string]any) Router
	// Response documents an HTTP response for the most recently
	// registered route.
	Response(status int, description string, mediaTypes ...string) Router
	// ResponseWithExample documents an HTTP response for the most recently
	// registered route, including schema references and examples.
	ResponseWithExample(status int, description string, schema map[string]any, schemaRef string, example any, examples map[string]any, mediaTypes ...string) Router
	// Tags sets the tags for the most recently registered route.
	Tags(tags ...string) Router
	// Deprecated marks the most recently registered route as deprecated.
	Deprecated() Router
	// Security sets the security requirements for the most recently registered
	// route. Each requirement maps a security scheme name to its required
	// scopes; multiple requirements are combined with OR semantics. Passing an
	// empty requirement (an empty map) documents that the operation requires no
	// authentication, overriding any document-level default.
	Security(requirements ...map[string][]string) Router
	// ResponseHeader documents a response header for the given status code on the
	// most recently registered route, creating the response entry if needed.
	ResponseHeader(status int, name, description string, schema map[string]any) Router
	// Hidden excludes the most recently registered route from the generated
	// OpenAPI specification.
	Hidden() Router
	// AddParameter documents an input parameter using the full RouteParameter,
	// allowing advanced fields (deprecated, style, explode, allowEmptyValue,
	// allowReserved) that the simpler Parameter helpers do not expose.
	AddParameter(param RouteParameter) Router
	// OperationExternalDocs sets the externalDocs of the most recently registered
	// operation.
	OperationExternalDocs(description, url string) Router
	// RequestBodyContent documents a request body with a different schema/example/
	// encoding per media type.
	RequestBodyContent(description string, required bool, content map[string]RouteMediaType) Router
	// ResponseContent documents a response with a different schema/example/encoding
	// per media type for the given status code.
	ResponseContent(status int, description string, content map[string]RouteMediaType) Router
	// ResponseLink documents a response link for the given status code, creating the
	// response entry if needed.
	ResponseLink(status int, name string, link map[string]any) Router
	// OperationExtension shallow-merges arbitrary fields (e.g. servers, callbacks,
	// x-* extensions) into the most recently registered operation object.
	OperationExtension(fields map[string]any) Router
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

	// domain is the host pattern the route was registered under via
	// app.Domain(); empty for regular routes. Same-path registrations on
	// different domains must never be compression-merged, so each keeps its
	// own handlers and documentation metadata. Read only at registration
	// time, never by the request scan.
	domain string

	// regID identifies the register() call that created this route, so
	// chainable helpers (Name, Summary, ...) can reach every stack entry of
	// the same registration.
	regID uint64

	// OpenAPI documentation metadata. The request scan never reads any of it,
	// so it stays below the routing state above.
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	Consumes    string `json:"consumes,omitempty"`
	Produces    string `json:"produces,omitempty"`

	Responses   map[string]RouteResponse `json:"responses,omitempty"`
	RequestBody *RouteRequestBody        `json:"requestBody,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase

	Parameters          []RouteParameter      `json:"parameters,omitempty"`
	Tags                []string              `json:"tags,omitempty"`
	Security            []map[string][]string `json:"security,omitempty"`            // OpenAPI security requirements
	ExternalDocs        map[string]any        `json:"externalDocs,omitempty"`        //nolint:tagliatelle // OpenAPI operation externalDocs
	OperationExtensions map[string]any        `json:"operationExtensions,omitempty"` //nolint:tagliatelle // internal route metadata

	Deprecated bool `json:"deprecated,omitempty"`
	hidden     bool // Excluded from the generated OpenAPI specification
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
// The buckets themselves are still recycled by reuseRouteBucket, so this does
// not make RebuildTree safe against in-flight requests and nothing here should
// be read as claiming it does. A rebuild appends over the backing array the
// published tree points at, so a scan in progress can observe a bucket
// mid-rewrite; publishing is also an unsynchronized store, so a reader is not
// guaranteed to see a rebuild at all, or to see one method's tree and another's
// from the same build. RebuildTree's own doc states the contract.
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
	return buildRouteURLFrom(route.Path, &route.routeParser, route.caseSensitive, params)
}

// buildRouteURLFrom is buildRouteURL over the three fields it actually reads, so
// callers on the request path can build a URL without materializing a Route.
//
//nolint:revive // flag-parameter: caseSensitive mirrors Route.caseSensitive, it selects a lookup rule rather than a branch of behavior.
func buildRouteURLFrom(path string, parser *routeParser, caseSensitive bool, params Map) (string, error) {
	if len(parser.segs) == 0 {
		return path, nil
	}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	for _, segment := range parser.segs {
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
		if val, found = params[segment.ParamName]; !found && !caseSensitive {
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

	return buf.String(), nil
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

// IsMiddleware reports whether this route was registered via Use() and
// therefore matches path prefixes rather than exact paths. This is useful
// for filtering middleware routes from generated API specifications.
func (r *Route) IsMiddleware() bool {
	return r.use
}

// IsAutoHead reports whether this route was automatically generated as a
// HEAD counterpart of a GET route.
func (r *Route) IsAutoHead() bool {
	return r.autoHead
}

// IsHidden reports whether this route is excluded from the generated OpenAPI
// specification (set via the Hidden helper).
func (r *Route) IsHidden() bool {
	return r.hidden
}

// RouteParameter describes an input captured by a route.
type RouteParameter struct {
	Schema          map[string]any `json:"schema"`
	SchemaRef       string         `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Example         any            `json:"example,omitempty"`
	Examples        map[string]any `json:"examples,omitempty"`
	Explode         *bool          `json:"explode,omitempty"`
	Description     string         `json:"description"`
	Name            string         `json:"name"`
	In              string         `json:"in"`
	Style           string         `json:"style,omitempty"`
	Required        bool           `json:"required"`
	Deprecated      bool           `json:"deprecated,omitempty"`
	AllowEmptyValue bool           `json:"allowEmptyValue,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	AllowReserved   bool           `json:"allowReserved,omitempty"`   //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteMediaType describes a single media type entry, allowing a different
// schema, examples and encoding per content type within one request body or
// response.
type RouteMediaType struct {
	Schema    map[string]any `json:"schema,omitempty"`
	Example   any            `json:"example,omitempty"`
	Examples  map[string]any `json:"examples,omitempty"`
	Encoding  map[string]any `json:"encoding,omitempty"`
	SchemaRef string         `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteResponse describes a response emitted by a route.
type RouteResponse struct {
	Example     any                       `json:"example,omitempty"`
	Schema      map[string]any            `json:"schema,omitempty"`
	Examples    map[string]any            `json:"examples,omitempty"`
	Headers     map[string]any            `json:"headers,omitempty"`
	Links       map[string]any            `json:"links,omitempty"`
	Content     map[string]RouteMediaType `json:"content,omitempty"`
	SchemaRef   string                    `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Description string                    `json:"description"`
	MediaTypes  []string                  `json:"mediaTypes"` //nolint:tagliatelle // OpenAPI spec uses camelCase
}

// RouteRequestBody describes the request payload accepted by a route.
type RouteRequestBody struct {
	Example     any                       `json:"example,omitempty"`
	Schema      map[string]any            `json:"schema,omitempty"`
	Examples    map[string]any            `json:"examples,omitempty"`
	Content     map[string]RouteMediaType `json:"content,omitempty"`
	SchemaRef   string                    `json:"schemaRef,omitempty"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Description string                    `json:"description"`
	MediaTypes  []string                  `json:"mediaTypes"` //nolint:tagliatelle // OpenAPI spec uses camelCase
	Required    bool                      `json:"required"`
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
	// The prefix may introduce parameters of its own (e.g. mounting under
	// "/:tenant"), so the parameter names must be re-derived from the
	// prefixed path just like register() derives them from the raw path.
	route.Params = parseRoute(prefixedPath, regexHandler, customConstraints...).params
	route.root = false
	route.star = false
	route.caseSensitive = app.config.CaseSensitive
	// buildTree recomputes this for every route, but this function rewrites the
	// path and parser a filter is derived from, so refresh it here too rather
	// than depend on a caller marking the routes refreshed.
	route.buildPrefixFilter()

	return route
}

func (app *App) copyRoute(route *Route) *Route {
	copied := app.copyRouteValue(route)
	return &copied
}

// copyRouteValue is copyRoute without the heap allocation, for callers that
// return the clone by value (GetRoute, GetRoutes). Building the clone in place
// keeps a route lookup off the heap even though it still deep-copies.
func (app *App) copyRouteValue(route *Route) Route {
	copied := app.copyRouteBaseValue(route)

	// An undocumented route — the common case on a route lookup — has nothing
	// to deep-copy, so the base copy is already a complete one.
	if route.RequestBody == nil && route.Parameters == nil && route.Responses == nil &&
		route.Tags == nil && route.Security == nil && route.ExternalDocs == nil &&
		route.OperationExtensions == nil {
		return copied
	}

	copied.RequestBody = cloneRouteRequestBody(route.RequestBody)
	copied.Parameters = cloneRouteParameters(route.Parameters)
	copied.Responses = cloneRouteResponses(route.Responses)
	copied.Tags = append([]string(nil), route.Tags...)
	copied.Security = cloneRouteSecurity(route.Security)
	copied.ExternalDocs = copyAnyMap(route.ExternalDocs)
	copied.OperationExtensions = copyAnyMap(route.OperationExtensions)

	return copied
}

// copyRouteBase copies routing data and scalar metadata but skips the deep
// clone of documentation maps/slices. Auto-generated HEAD twins use it because
// their doc metadata is never read: the OpenAPI middleware excludes autoHead
// routes and HEAD serves by re-running the copied GET handler stack.
func (app *App) copyRouteBase(route *Route) *Route {
	copied := app.copyRouteBaseValue(route)
	return &copied
}

// copyRouteBaseValue is copyRouteBase without the heap allocation.
//
// It copies the route wholesale and then clears exactly the fields a base copy
// must not share — the group pointer and the documentation containers — which
// is both equivalent to naming every field explicitly and markedly cheaper: a
// single move instead of two dozen field writes, on a struct this large.
func (*App) copyRouteBaseValue(route *Route) Route {
	copied := *route

	copied.group = nil
	copied.RequestBody = nil
	copied.Parameters = nil
	copied.Responses = nil
	copied.Tags = nil
	copied.Security = nil
	copied.ExternalDocs = nil
	copied.OperationExtensions = nil

	return copied
}

func cloneRouteSecurity(requirements []map[string][]string) []map[string][]string {
	if len(requirements) == 0 {
		return nil
	}
	cloned := make([]map[string][]string, len(requirements))
	for i, requirement := range requirements {
		entry := make(map[string][]string, len(requirement))
		for scheme, scopes := range requirement {
			// make+copy keeps an empty scope list non-nil so it marshals as
			// the spec-required [] rather than null.
			cloned := make([]string, len(scopes))
			copy(cloned, scopes)
			entry[scheme] = cloned
		}
		cloned[i] = entry
	}
	return cloned
}

func cloneRouteRequestBody(body *RouteRequestBody) *RouteRequestBody {
	if body == nil {
		return nil
	}
	clone := &RouteRequestBody{
		Description: body.Description,
		Required:    body.Required,
	}
	if len(body.Schema) > 0 {
		clone.Schema = copyAnyMap(body.Schema)
	}
	clone.SchemaRef = body.SchemaRef
	if len(body.Examples) > 0 {
		clone.Examples = copyAnyMap(body.Examples)
	}
	clone.Example = copyAnyValue(body.Example)
	if len(body.MediaTypes) > 0 {
		clone.MediaTypes = append([]string(nil), body.MediaTypes...)
	}
	clone.Content = cloneRouteMediaTypeMap(body.Content)
	return clone
}

func cloneRouteMediaTypeMap(content map[string]RouteMediaType) map[string]RouteMediaType {
	if len(content) == 0 {
		return nil
	}
	cloned := make(map[string]RouteMediaType, len(content))
	for mediaType, mt := range content {
		cloned[mediaType] = RouteMediaType{
			Schema:    copyAnyMap(mt.Schema),
			SchemaRef: mt.SchemaRef,
			Example:   copyAnyValue(mt.Example),
			Examples:  copyAnyMap(mt.Examples),
			Encoding:  copyAnyMap(mt.Encoding),
		}
	}
	return cloned
}

func cloneRouteParameters(params []RouteParameter) []RouteParameter {
	if len(params) == 0 {
		return nil
	}
	cloned := make([]RouteParameter, len(params))
	for i := range params {
		p := &params[i]
		cloned[i] = RouteParameter{
			Name:            p.Name,
			In:              p.In,
			Required:        p.Required,
			Description:     p.Description,
			Deprecated:      p.Deprecated,
			Style:           p.Style,
			AllowEmptyValue: p.AllowEmptyValue,
			AllowReserved:   p.AllowReserved,
			Schema:          copyAnyMap(p.Schema),
			SchemaRef:       p.SchemaRef,
			Examples:        copyAnyMap(p.Examples),
			Example:         copyAnyValue(p.Example),
		}
		if p.Explode != nil {
			explode := *p.Explode
			cloned[i].Explode = &explode
		}
	}
	return cloned
}

func cloneRouteResponses(responses map[string]RouteResponse) map[string]RouteResponse {
	if len(responses) == 0 {
		return nil
	}
	cloned := make(map[string]RouteResponse, len(responses))
	for code, resp := range responses {
		copyResp := RouteResponse{
			Description: resp.Description,
			Schema:      copyAnyMap(resp.Schema),
			SchemaRef:   resp.SchemaRef,
			Examples:    copyAnyMap(resp.Examples),
			Example:     copyAnyValue(resp.Example),
			Headers:     copyAnyMap(resp.Headers),
			Links:       copyAnyMap(resp.Links),
			Content:     cloneRouteMediaTypeMap(resp.Content),
		}
		if len(resp.MediaTypes) > 0 {
			copyResp.MediaTypes = append([]string(nil), resp.MediaTypes...)
		}
		cloned[code] = copyResp
	}
	return cloned
}

// maxCopyDepth bounds the deep copy of route documentation metadata. Users can
// store arbitrary values there, including self-referential ones; without a
// bound such a value turns every GetRoutes call into an unrecoverable stack
// overflow. Real documentation nests far shallower than this.
const maxCopyDepth = 100

func copyAnyMap(src map[string]any) map[string]any {
	return copyAnyMapDepth(src, 0)
}

func copyAnyMapDepth(src map[string]any, depth int) map[string]any {
	if len(src) == 0 {
		return nil
	}
	if depth >= maxCopyDepth {
		// Cyclic or pathologically deep metadata: stop copying rather than
		// recursing until the goroutine stack dies. Sharing the reference is
		// the lesser evil, and encoding/json reports the cycle itself.
		return src
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = copyAnyValueDepth(value, depth+1)
	}
	return dst
}

func copyAnyValue(src any) any {
	return copyAnyValueDepth(src, 0)
}

func copyAnyValueDepth(src any, depth int) any {
	if src == nil {
		return nil
	}
	if depth >= maxCopyDepth {
		return src
	}

	switch value := src.(type) {
	case map[string]any:
		return copyAnyMapDepth(value, depth)
	case []any:
		copied := make([]any, len(value))
		for i := range value {
			copied[i] = copyAnyValueDepth(value[i], depth+1)
		}
		return copied
	case []map[string]any:
		copied := make([]map[string]any, len(value))
		for i := range value {
			copied[i] = copyAnyMapDepth(value[i], depth+1)
		}
		return copied
	default:
		return copyCompositeValue(src)
	}
}

func copyCompositeValue(src any) any {
	value := reflect.ValueOf(src)

	switch value.Kind() {
	case reflect.Slice:
		if value.IsNil() {
			return src
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			// A nil element yields an invalid reflect.Value; leave the zero
			// value in place instead of panicking in Set.
			if elem := copyAnyValue(value.Index(i).Interface()); elem != nil {
				copied.Index(i).Set(reflect.ValueOf(elem))
			}
		}
		return copied.Interface()
	case reflect.Map:
		if value.IsNil() {
			return src
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			// SetMapIndex with an invalid value deletes the key, so map a nil
			// element to the element type's zero value to preserve it.
			val := reflect.Zero(value.Type().Elem())
			if elem := copyAnyValue(iter.Value().Interface()); elem != nil {
				val = reflect.ValueOf(elem)
			}
			copied.SetMapIndex(iter.Key(), val)
		}
		return copied.Interface()
	default:
		return src
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
			app.bumpRoutesRevision()

			// Invalidate the registration cursor and batch so later chained
			// helpers become no-ops instead of mutating a removed route.
			if route == app.latestRoute {
				app.latestRoute = nil
			}
			if slices.Contains(app.latestBatch, route) {
				app.latestBatch = app.latestBatch[:0]
				app.latestBatchID = 0
			}

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
		app.bumpRoutesRevision()
		atomic.AddUint32(&app.handlersCount, ^uint32(len(headRoute.Handlers)-1)) //nolint:gosec // G115 - handler count is always small
		return
	}
}

// register creates one stack entry per method for the given path and returns
// the registration ID stamped on every entry, so scoped helpers (Group,
// Registering, domainRouter) can target exactly this registration later.
// domain is the host pattern for app.Domain() registrations, "" otherwise.
func (app *App) register(methods []string, pathRaw string, group *Group, domain string, handlers ...Handler) uint64 {
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

	// One registration ID for the whole call, so chainable helpers reach the
	// routes of every method registered together.
	regID := atomic.AddUint64(&app.registrationID, 1)

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
			regID:         regID,
			domain:        domain,

			path:        pathClean,
			routeParser: parsedPretty,
			Params:      parsedRaw.params,
			group:       group,

			Path:        pathRaw,
			Method:      method,
			Handlers:    handlers,
			Summary:     "",
			Description: "",
			// Consumes/Produces stay empty until set explicitly; the OpenAPI
			// middleware treats empty as "unspecified" and emits no media type.
			Consumes: "",
			Produces: "",
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

	return regID
}

func (app *App) addRoute(method string, route *Route) {
	app.mutex.Lock()

	// Get unique HTTP method identifier
	m := app.methodInt(method)

	if method == MethodHead && !route.mount && !route.use {
		app.pruneAutoHeadRouteLocked(route.path)
	}

	// The stack entry the registration ends up in: the route itself, or the
	// pre-existing entry it was compression-merged into.
	liveRoute := route

	// A new registration always starts a fresh helper-target batch, even when
	// every one of its entries ends up compression-merged away.
	app.resetBatchIfNewRegistrationLocked(route.regID)

	// prevent identically route registration
	l := len(app.stack[m])
	if l > 0 && app.stack[m][l-1].Path == route.Path && route.use == app.stack[m][l-1].use &&
		!route.mount && !app.stack[m][l-1].mount && app.stack[m][l-1].domain == route.domain {
		preRoute := app.stack[m][l-1]
		preRoute.Handlers = append(preRoute.Handlers, route.Handlers...)
		// Consecutive same-path registrations share one stack entry, and its
		// documentation deliberately belongs to the latest registration
		// (chained .Name()/.Summary() on the newest Use()/route wins, matching
		// Fiber's established naming behavior). Restamping the entry keeps the
		// registration-ID lookup agreeing with the batch fast path: without it
		// a scoped helper called after an unrelated registration would scan for
		// an ID no stack entry carries and silently document nothing.
		preRoute.regID = route.regID
		liveRoute = preRoute
		app.latestBatch = append(app.latestBatch, preRoute)
	} else {
		route.Method = method
		// Add route to the stack
		app.stack[m] = append(app.stack[m], route)
		app.hasRoutesRefreshed = true
		app.latestBatch = append(app.latestBatch, route)
	}

	app.bumpRoutesRevision()

	// Track the most recent registration so chained helpers (Name, Summary,
	// ...) target it. Mount routes are tracked too — otherwise a helper
	// chained onto app.Use("/api", subApp) would mutate whatever route was
	// registered before the mount — but onRoute hooks are not fired for them.
	app.latestRoute = liveRoute

	// Snapshot the route under the lock, then fire hooks after releasing it so
	// they may safely call locking app methods (GetRoutes, documentation
	// helpers, RemoveRoute, ...). The private snapshot keeps hook reads from
	// racing concurrent documentation of the live route.
	var hookRoute *Route
	if !route.mount && len(app.hooks.onRoute) > 0 {
		hookRoute = app.copyRoute(liveRoute)
	}
	app.mutex.Unlock()
	if hookRoute != nil {
		if err := app.hooks.executeOnRouteHooks(hookRoute); err != nil {
			panic(err)
		}
	}
}

// resetBatchIfNewRegistrationLocked starts a fresh helper-target batch when
// regID belongs to a new registration. Documentation helpers use the batch to
// reach every stack entry of the most recent registration in O(batch) instead
// of scanning the stack. The caller must hold app.mutex.
func (app *App) resetBatchIfNewRegistrationLocked(regID uint64) {
	if regID != app.latestBatchID {
		app.latestBatchID = regID
		app.latestBatch = app.latestBatch[:0]
	}
}

func (app *App) ensureAutoHeadRoutes() {
	app.mutex.Lock()
	twins := app.ensureAutoHeadRoutesLocked()
	app.mutex.Unlock()

	// Fire hooks after releasing the lock so they may safely call locking app
	// methods (GetRoutes, documentation helpers, RemoveRoute, ...).
	app.fireOnRouteHooks(twins)
}

// ensureAutoHeadRoutesLocked creates the missing auto-HEAD twins and returns
// private snapshots of them; the caller must hold app.mutex and fire the
// onRoute hooks for the returned snapshots after releasing it.
func (app *App) ensureAutoHeadRoutesLocked() []*Route {
	if app.config.DisableHeadAutoRegister {
		return nil
	}

	headIndex := app.methodInt(MethodHead)
	getIndex := app.methodInt(MethodGet)
	if headIndex == -1 || getIndex == -1 {
		return nil
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
		return nil
	}

	var (
		twins []*Route
		added bool
	)

	for _, route := range app.stack[getIndex] {
		if route.mount || route.use {
			continue
		}
		if _, ok := existing[route.path]; ok {
			continue
		}

		headRoute := app.copyRouteBase(route)
		headRoute.group = route.group
		headRoute.Method = MethodHead
		headRoute.autoHead = true
		// Twins carry no documentation at all: copyRouteBase skips the doc
		// maps/slices, and the scalar doc fields are blanked here so Stack and
		// GetRoutes never expose a half-documented HEAD route. Spec consumers
		// filter twins via IsAutoHead.
		headRoute.Summary = ""
		headRoute.Description = ""
		headRoute.Consumes = ""
		headRoute.Produces = ""
		headRoute.Deprecated = false
		// Fasthttp automatically omits response bodies when transmitting
		// HEAD responses, so the copied GET handler stack can execute
		// unchanged while still producing an empty body on the wire.

		headStack = append(headStack, headRoute)
		existing[route.path] = struct{}{}
		app.hasRoutesRefreshed = true
		added = true
		// Snapshot for the onRoute hooks, which run after the lock is
		// released and must not read the live route. Nothing to snapshot when
		// no hook will observe it.
		if len(app.hooks.onRoute) > 0 {
			twins = append(twins, app.copyRoute(headRoute))
		}

		atomic.AddUint32(&app.handlersCount, uint32(len(headRoute.Handlers))) //nolint:gosec // G115 - handler count is always small

		// The twin deliberately stays out of the registration batch and never
		// becomes latestRoute: it carries no documentation of its own (the
		// fields were blanked above), and letting a post-startup helper reach
		// it would re-document an arbitrary route.
	}

	if added {
		app.stack[headIndex] = headStack
		app.bumpRoutesRevision()
	}
	return twins
}

// fireOnRouteHooks runs the onRoute hooks for each route, panicking on error
// exactly like route registration does. Callers must not hold app.mutex.
func (app *App) fireOnRouteHooks(routes []*Route) {
	for _, route := range routes {
		if err := app.hooks.executeOnRouteHooks(route); err != nil {
			panic(err)
		}
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

			if len(route.routeParser.segs) > 0 && len(route.routeParser.segs[0].Const) >= maxDetectionPaths {
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

		prevBuckets := app.treeStack[method]
		tsMap := make(map[int][]*Route, len(prefixCounts)+1)
		tsMap[0] = reuseRouteBucket(prevBuckets, 0, globalCount)
		for treePath, count := range prefixCounts {
			tsMap[treePath] = reuseRouteBucket(prevBuckets, treePath, count+globalCount)
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

func reuseRouteBucket(prev map[int][]*Route, key, capHint int) []*Route {
	if bucket, ok := prev[key]; ok && cap(bucket) >= capHint {
		return bucket[:0]
	}
	return make([]*Route, 0, capHint)
}
