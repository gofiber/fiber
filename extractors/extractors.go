package extractors

// Package extractors provides shared value extraction utilities for Fiber middleware.
// This package helps reduce code duplication across middleware packages
// while ensuring consistent behavior, security practices, and RFC compliance.
// It can extract string values from various HTTP request sources including
// headers, cookies, query parameters, form data, and URL parameters.
//
// Example usage:
//
//	import "github.com/gofiber/fiber/v3/extractors"
//
//	// Extract from Authorization header
//	authExtractor := extractors.FromAuthHeader("Bearer")
//
//	// Chain multiple sources with fallback
//	tokenExtractor := extractors.Chain(
//	    extractors.FromHeader("X-API-Key"),
//	    extractors.FromCookie("api_key"),
//	    extractors.FromQuery("token"),
//	)
//
// Security considerations:
//   - Query parameters and form data can leak sensitive information
//   - Use HTTPS to protect extracted values in transit
//   - Consider source-specific security policies for your use case

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/gofiber/fiber/v3/internal/ctxlocal"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/utils/v2"
)

// Source represents the type of source from which an API key is extracted.
// This is informational metadata that helps developers understand the extractor behavior.
type Source int

const (
	// SourceHeader indicates the value is extracted from an HTTP header.
	SourceHeader Source = iota

	// SourceAuthHeader indicates the value is extracted from the Authorization header.
	SourceAuthHeader

	// SourceForm indicates the value is extracted from form data.
	SourceForm

	// SourceQuery indicates the value is extracted from URL query parameters.
	SourceQuery

	// SourceParam indicates the value is extracted from URL path parameters.
	SourceParam

	// SourceCookie indicates the value is extracted from cookies.
	SourceCookie

	// SourceCustom indicates the value is extracted using a custom extractor function.
	SourceCustom
)

// ErrNotFound is returned when the requested value is missing or empty.
var ErrNotFound = errors.New("value not found")

// ErrChainCycle is returned when a chain extractor recursively invokes itself.
var ErrChainCycle = errors.New("cyclic extractor chain")

// Extractor defines a value extraction method with metadata.
type Extractor struct {
	Extract    func(fiber.Ctx) (string, error)
	Key        string      // The parameter/header name used for extraction
	AuthScheme string      // The auth scheme used, e.g., "Bearer"
	Chain      []Extractor // For chained extractors, stores all extractors in the chain
	Source     Source      // The type of source being extracted from
}

// Result is a resolved extraction: the value, and the provenance of the
// extractor that supplied it.
//
// It carries metadata only. A caller learns where a value came from without
// receiving anything runnable, so the defensive copy a chain keeps of its
// children can never be reached, let alone rewritten, through a Result.
type Result struct {
	// Value is the extracted value. It is non-empty only when Resolve
	// returned a nil error.
	Value string

	// Key is the parameter, header or cookie name the winning extractor read.
	Key string

	// Source is the kind of source the winning extractor read from.
	Source Source

	// Resolved reports whether Key and Source name the extractor that actually
	// produced Value.
	//
	// It is false when the value came out of a chain whose own Extract answered
	// without any child being observed to win — a full replacement, or a
	// decorator that does not delegate. Key and Source are then the chain's
	// declared metadata (its first child), which says nothing about where the
	// value came from. A caller making a security decision from provenance —
	// deciding a value may be written back to the place it was read from —
	// must treat false as "origin unknown" and refuse, exactly as it would for
	// a read-only source. Always false when err is non-nil.
	Resolved bool
}

// Resolve extracts a value and reports which extractor supplied it.
//
// Prefer this over Extract when the caller needs the provenance of a value —
// its Key as well as its Source — for example to write a value back to the
// same place it was read from. Extractor.Source is declared/static metadata
// (for a chain, the first child); SourceHeader is the zero value, so a
// hand-rolled Extract without an explicit Source reports SourceHeader. No
// extra struct field is required, so existing unkeyed Extractor literals keep
// compiling.
//
// Behavior:
//   - Leaf (Extract set, no Chain): Extract runs and e's own metadata is the
//     provenance, with Resolved true.
//   - Chain (Extract set and Chain non-empty): Extract runs, so legacy
//     overrides and decoration (validation) are honored. A built-in Chain
//     records its winning child while it runs, and that child is reported with
//     Resolved true. If Extract answers without such a record — a full
//     replacement, or a decorator that does not delegate — e's declared
//     metadata is reported with Resolved FALSE, because which child (if any)
//     produced the value is unknown; e.Chain is not re-walked to guess.
//   - Chain with nil Extract: walk children (same success rules as
//     Chain.Extract), skip nil Extract, report the winning child.
//   - Neither: ErrNotFound.
//
// A record only counts when it was made by the very chain being resolved, so a
// value cannot be credited to a chain some unrelated helper happened to run.
//
// Key and Source are meaningful for security decisions only when err is nil
// AND Resolved is true. Extract is not deprecated.
func Resolve(e Extractor, c fiber.Ctx) (Result, error) {
	return resolveOne(&e, c, nil)
}

// resolveOne is Resolve by pointer, carrying the request's chain state so a walk
// looks it up once rather than once per child. A nil st is resolved on first
// need.
func resolveOne(e *Extractor, c fiber.Ctx, st *chainState) (Result, error) {
	if e.Extract != nil {
		if st == nil {
			st = chainStateFor(c)
		}
		// Marks the frame, so Chain.Extract records a winner only while a
		// source-aware caller is active.
		parentWin, parentGuard := st.enterCapture()
		defer st.leaveCapture(parentWin, parentGuard)

		v, err := e.Extract(c)
		// Read before the deferred restore runs, and only accept a record this
		// very chain made: a leaf whose Extract consults some other chain must
		// not inherit that chain's winner.
		win := st.capturedFor(e.Chain)
		if err != nil {
			return Result{Key: e.Key, Source: e.Source}, err
		}
		if v == "" {
			return Result{Key: e.Key, Source: e.Source}, ErrNotFound
		}
		if win != nil {
			return Result{Value: v, Key: win.Key, Source: win.Source, Resolved: true}, nil
		}
		// No record we can vouch for. A leaf that ran no chain at all is its own
		// origin. Anything else — a chain that answered without a child
		// winning, a leaf that ran some other chain, or a chain whose public
		// Chain was cleared so a record can no longer be tied to it — cannot
		// say where the value came from, so the declared metadata is reported
		// as unresolved rather than passed off as provenance.
		resolved := len(e.Chain) == 0 && st.win == nil
		return Result{Value: v, Key: e.Key, Source: e.Source, Resolved: resolved}, nil
	}
	if len(e.Chain) > 0 {
		if st == nil {
			st = chainStateFor(c)
		}
		return resolveChain(e, c, st)
	}
	return Result{Key: e.Key, Source: e.Source}, ErrNotFound
}

// chainGuard returns the cycle-guard identity of a chain: the address of the
// first element of its public Chain array, which survives the value copies an
// Extractor makes. chain must not be empty.
func chainGuard(chain []Extractor) *byte {
	return (*byte)(unsafe.Pointer(&chain[0])) //nolint:gosec // G103: identity for the cycle guard only, never dereferenced
}

// chainState is what every chain on one request shares. Holding it in a single
// request-local entry costs a chain one Locals lookup rather than one per
// guard, depth and winner operation, and makes recording a winner a field
// write rather than a re-allocated []Source.
type chainState struct {
	win      *Extractor // the innermost winning child, nil when none
	winGuard *byte      // identity of the chain that recorded win
	active   []*byte    // guards of the chains executing right now, innermost last
	depth    int        // open Resolve frames; winners are recorded while > 0
}

// chainStateKey is the Locals key of the request's chainState.
type chainStateKey struct{}

var chainStatePool = sync.Pool{
	New: func() any { return &chainState{active: make([]*byte, 0, 4)} },
}

// chainStateFor returns the request's chainState, creating it on first use.
// Being a Locals value, it comes back through Close when fasthttp resets the
// request, so a request running any number of chains allocates for none.
func chainStateFor(c fiber.Ctx) *chainState {
	if st, ok := c.Locals(chainStateKey{}).(*chainState); ok && st != nil {
		return st
	}
	st, ok := chainStatePool.Get().(*chainState)
	if !ok || st == nil {
		st = &chainState{active: make([]*byte, 0, 4)}
	}
	ctxlocal.Set(c, chainStateKey{}, st)
	return st
}

// Close returns the state to the pool. fasthttp calls it on every
// request-local io.Closer when it resets the request; callers should not.
func (s *chainState) Close() error {
	s.reset()
	chainStatePool.Put(s)
	return nil
}

// reset returns the state to the shape a fresh one has. Split from Close so it
// can be tested without handing the state to the pool, where another request
// may already own it.
func (s *chainState) reset() {
	// Cleared to the buffer's capacity, not its length: leave() reslices, so
	// entries past the end still hold guards, and a retained one keeps a
	// chain's Extractor array and whatever its closures captured alive for as
	// long as the pool holds this state.
	clear(s.active[:cap(s.active)])
	// Whole-struct, so a field added later cannot leak one request's state
	// into the next. The buffer survives: the literal is evaluated first.
	*s = chainState{active: s.active[:0]}
}

// enter marks a chain as executing, or reports false if it already is: a
// cycle.
func (s *chainState) enter(guard *byte) bool {
	if slices.Contains(s.active, guard) {
		return false
	}
	s.active = append(s.active, guard)
	return true
}

// leave unmarks the innermost executing chain.
func (s *chainState) leave() {
	if n := len(s.active); n > 0 {
		s.active = s.active[:n-1]
	}
}

// enterCapture opens a Resolve frame and returns the record the enclosing one
// held, which the frame hides so a leaf is not attributed to it. Pass both
// results back to leaveCapture.
func (s *chainState) enterCapture() (win *Extractor, guard *byte) { //nolint:nonamedreturns // the pair is one record; names say which half is which
	s.depth++
	win, guard = s.win, s.winGuard
	s.win, s.winGuard = nil, nil
	return win, guard
}

// capturedFor returns the child recorded by the chain whose children are chain,
// or nil when the current record was made by some other chain, or none was.
// Tying the record to its chain is what keeps a value from being credited to a
// chain that merely ran somewhere inside the same frame.
func (s *chainState) capturedFor(chain []Extractor) *Extractor {
	if s.win == nil || len(chain) == 0 || s.winGuard != chainGuard(chain) {
		return nil
	}
	return s.win
}

// leaveCapture closes a Resolve frame, dropping its own record and restoring
// the enclosing frame's. A decorated chain can call its base and then make
// another source-aware call before returning, and the record the base made has
// to survive that.
func (s *chainState) leaveCapture(win *Extractor, guard *byte) {
	s.depth--
	s.win, s.winGuard = win, guard
}

// resolveChain walks e.Chain in order and returns the first non-empty value
// with the extractor that produced it. A chain that re-enters itself fails with
// ErrChainCycle. If nothing matched, the last error seen wins over ErrNotFound, so
// the caller learns why the chain failed rather than only that it did.
func resolveChain(e *Extractor, c fiber.Ctx, st *chainState) (Result, error) {
	// Snapshotted once: a child's Extract that reassigns the public slice
	// mid-walk must not move the ground under the loop.
	chain := e.Chain
	if !st.enter(chainGuard(chain)) {
		return Result{Key: e.Key, Source: e.Source}, ErrChainCycle
	}
	defer st.leave()

	var lastErr error
	lastRes := Result{Key: e.Key, Source: e.Source}
	for i := range chain {
		child := &chain[i]
		if child.Extract == nil && len(child.Chain) == 0 {
			continue
		}
		// Nested chains and leaves both go through the same walk.
		res, err := resolveOne(child, c, st)
		if err == nil && res.Value != "" {
			return res, nil
		}
		if err != nil {
			lastErr = err
			lastRes = Result{Key: res.Key, Source: res.Source}
		}
	}
	if lastErr != nil {
		return lastRes, lastErr
	}
	return Result{Key: e.Key, Source: e.Source}, ErrNotFound
}

// Contains reports whether this extractor, or any extractor in its chain, matches pred.
//
// If pred is nil, Contains returns false.
func (e Extractor) Contains(pred func(Extractor) bool) bool {
	if pred == nil {
		return false
	}

	stack := make([]*Extractor, 0, len(e.Chain)+1)
	stack = append(stack, &e)
	visited := make(map[*Extractor]struct{}, len(e.Chain)+1)

	for len(stack) > 0 {
		last := len(stack) - 1
		curr := stack[last]
		stack = stack[:last]
		if _, ok := visited[curr]; ok {
			continue
		}
		visited[curr] = struct{}{}

		if pred(*curr) {
			return true
		}

		for i := range curr.Chain {
			stack = append(stack, &curr.Chain[i])
		}
	}

	return false
}

// FromAuthHeader extracts a value from the Authorization header with an optional prefix.
// This function implements RFC 9110 compliant Authorization header parsing with strict token68 validation.
//
// RFC Compliance:
//   - Follows RFC 9110 Section 11.6.2 for Authorization header format
//   - Requires exactly one SP between auth-scheme and credentials. RFC 9110
//     permits 1*SP, but a single space is what clients send in practice and
//     the stricter rule keeps the parse unambiguous.
//   - Implements RFC 7235 token68 character validation for extracted tokens
//   - Case-insensitive auth scheme matching per HTTP standards
//
// Token68 Validation:
//   - Only allows characters: A-Z, a-z, 0-9, -, ., _, ~, +, /, =
//   - Rejects tokens containing spaces, tabs, or other whitespace
//   - Validates proper padding: = only at end, no characters after padding starts
//   - Prevents tokens starting with = (invalid padding)
//
// Security Features:
//   - Strict validation prevents header injection attacks
//   - Rejects malformed tokens that could bypass authentication
//   - Consistent error handling for missing or invalid credentials
//
// Parameters:
//   - authScheme: The auth scheme to strip from the header value (e.g., "Bearer", "Basic").
//     If empty, the entire header value is returned without validation.
//
// Returns:
//
//	An Extractor that attempts to retrieve and parse the Authorization header.
//	Returns ErrNotFound if the header is missing, malformed, or doesn't match the expected scheme.
//
// Examples:
//
//	// Extract Bearer token with validation
//	extractor := FromAuthHeader("Bearer")
//	// Input: "Bearer abc123" -> Output: "abc123"
//	// Input: "Bearer abc def" -> Output: ErrNotFound (space in token)
//	// Input: "Basic dXNlcjpwYXNz" -> Output: ErrNotFound (wrong scheme)
//
//	// Extract raw header value (no validation)
//	extractor := FromAuthHeader("")
//	// Input: "CustomAuth token123" -> Output: "CustomAuth token123"
func FromAuthHeader(authScheme string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		// A second Authorization line, whatever it is spelled like, makes
		// the credential ambiguous — including where middleware cleared this
		// field and a line the client sent stayed behind it.
		authHeader, ok := headerlookup.Value(c, fiber.HeaderAuthorization)
		if !ok || authHeader == "" {
			return "", ErrNotFound
		}

		// Check if the header starts with the specified auth scheme
		if authScheme != "" {
			schemeLen := len(authScheme)
			if len(authHeader) <= schemeLen || !utils.EqualFold(authHeader[:schemeLen], authScheme) {
				return "", ErrNotFound
			}
			rest := authHeader[schemeLen:]
			if rest == "" || rest[0] != ' ' {
				return "", ErrNotFound
			}

			// Extract token after the required space
			token := rest[1:]
			if token == "" {
				return "", ErrNotFound
			}

			if !isValidToken68(token) {
				return "", ErrNotFound
			}

			return token, nil
		}

		return authHeader, nil
	}
	return Extractor{
		Extract:    fn,
		Key:        fiber.HeaderAuthorization,
		Source:     SourceAuthHeader,
		AuthScheme: authScheme,
	}
}

// FromCookie creates an Extractor that retrieves a value from a specified cookie in the request.
//
// The function:
//   - Retrieves the cookie value using the specified name
//   - Returns ErrNotFound if the cookie is missing
//
// Parameters:
//   - key: The name of the cookie from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified cookie.
//	Returns ErrNotFound if the cookie is not present.
//
// Security Note:
//
//	Cookies are generally more secure than query parameters for sensitive data
//	as they are not logged in access logs or visible in browser history.
//	However, ensure cookies are properly secured with appropriate flags.
//
// Example:
//
//	extractor := FromCookie("session_id")
//	// Cookie: "session_id=abc123" -> Output: "abc123"
//	// Missing cookie -> Output: ErrNotFound
func FromCookie(key string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		value := c.Cookies(key)
		if value == "" {
			return "", ErrNotFound
		}
		return value, nil
	}
	return Extractor{
		Extract: fn,
		Key:     key,
		Source:  SourceCookie,
	}
}

// FromParam creates an Extractor that retrieves a value from a specified URL parameter in the request.
// URL parameters are extracted from the route path (e.g., /users/:id).
//
// SECURITY WARNING: Extracting values from URL parameters can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers when following links
//   - Proxy and intermediary server logs
//   - Browser history and bookmarks
//   - Network monitoring tools
//
// For sensitive data, prefer FromAuthHeader, FromCookie, or FromHeader instead.
//
// Parameters:
//   - param: The name of the URL parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified URL parameter.
//	Returns ErrNotFound if the parameter is not present.
//
// Example:
//
//	// Route: GET /users/:userId/posts/:postId
//	userExtractor := FromParam("userId")
//	postExtractor := FromParam("postId")
//	// URL: /users/123/posts/456 -> userId: "123", postId: "456"
func FromParam(param string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		value := c.Params(param)
		if value == "" {
			return "", ErrNotFound
		}
		// Without a percent sign there is nothing to decode, and this is
		// the common case, so it skips the config read below.
		if !strings.Contains(value, "%") {
			return value, nil
		}
		// UnescapePath already decoded the path once in the router, so
		// decoding again here would spend the client's escaping twice: a
		// literal "%20" sent as "%2520" would arrive as a space. Decode
		// only when the router left the value raw, which keeps the number
		// of decodes at one whatever the config says.
		if appconfig.Of(c.App()).UnescapePath {
			return value, nil
		}
		unescapedValue, err := url.PathUnescape(value)
		if err != nil {
			return "", ErrNotFound
		}
		return unescapedValue, nil
	}
	return Extractor{
		Extract: fn,
		Key:     param,
		Source:  SourceParam,
	}
}

// FromForm creates an Extractor that retrieves a value from a specified form field in the request.
// Form data is typically submitted via POST requests with content-type application/x-www-form-urlencoded.
//
// SECURITY WARNING: Extracting values from form data can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers (especially if form is submitted via GET)
//   - Proxy and intermediary server logs
//   - Browser history (if form uses GET method)
//
// For sensitive data, prefer FromAuthHeader or FromCookie instead.
// If using form data, ensure the form uses POST method and HTTPS.
//
// Parameters:
//   - param: The name of the form field from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified form field.
//	Returns ErrNotFound if the field is not present.
//
// Example:
//
//	extractor := FromForm("username")
//	// Form data: "username=john_doe&password=secret" -> Output: "john_doe"
//	// Missing field -> Output: ErrNotFound
func FromForm(param string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		value := c.FormValue(param)
		if value == "" {
			return "", ErrNotFound
		}
		return value, nil
	}
	return Extractor{
		Extract: fn,
		Key:     param,
		Source:  SourceForm,
	}
}

// FromHeader creates an Extractor that retrieves a value from a specified HTTP header in the request.
// HTTP headers are commonly used for API keys, tokens, and other metadata.
//
// The function:
//   - Retrieves the header value using the specified name
//   - Returns ErrNotFound if the header is missing
//
// Parameters:
//   - header: The name of the HTTP header from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified HTTP header.
//	Returns ErrNotFound if the header is not present.
//
// Security Note:
//
//	Headers are generally secure for sensitive data as they are not logged
//	in access logs by default. However, be aware that some proxies may log headers.
//
// Example:
//
//	extractor := FromHeader("X-API-Key")
//	// Header: "X-API-Key: abc123" -> Output: "abc123"
//	// Missing header -> Output: ErrNotFound
func FromHeader(header string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		// Not Ctx.Get: it is byte-exact, so under DisableHeaderNormalizing a token
		// sent under the lower-case name HTTP/2 and 3 use was not found, and the
		// request refused for carrying no token when it carried one.
		// Combined, not Value: the name comes from the application's
		// config, and a field it names may be a list one a peer is allowed
		// to send twice — Accept and Forwarded among them. Two lines there
		// are one value rather than two answers, so they are joined the way
		// RFC 9110 §5.3 says a recipient may. A repeated token still fails
		// the comparison the caller makes, so nothing is loosened.
		value := headerlookup.Combined(c, header)
		if value == "" {
			return "", ErrNotFound
		}
		return value, nil
	}
	return Extractor{
		Extract: fn,
		Key:     header,
		Source:  SourceHeader,
	}
}

// FromQuery creates an Extractor that retrieves a value from a specified query parameter in the request.
// Query parameters are extracted from the URL query string (e.g., ?key=value&foo=bar).
//
// SECURITY WARNING: Extracting values from URL query parameters can leak sensitive information through:
//   - Server access logs and error logs
//   - Browser referrer headers when following links
//   - Proxy and intermediary server logs
//   - Browser history and bookmarks
//   - Network monitoring tools and packet sniffers
//   - Web browser developer tools
//
// For sensitive data, prefer FromAuthHeader, FromCookie, or FromHeader instead.
// If query parameters must be used, ensure HTTPS is enforced.
//
// Parameters:
//   - param: The name of the query parameter from which to extract the value.
//
// Returns:
//
//	An Extractor that attempts to retrieve the value from the specified query parameter.
//	Returns ErrNotFound if the parameter is not present.
//
// Example:
//
//	extractor := FromQuery("token")
//	// URL: /api/data?token=abc123&format=json -> Output: "abc123"
//	// URL: /api/data?format=json -> Output: ErrNotFound
func FromQuery(param string) Extractor {
	fn := func(c fiber.Ctx) (string, error) {
		value := c.Query(param)
		if value == "" {
			return "", ErrNotFound
		}
		return value, nil
	}
	return Extractor{
		Extract: fn,
		Key:     param,
		Source:  SourceQuery,
	}
}

// FromCustom creates an Extractor using a provided function.
// This allows for custom extraction logic beyond the built-in extractors.
//
// The function:
//   - Accepts a custom extraction function with signature func(fiber.Ctx) (string, error)
//   - Handles nil functions gracefully by returning ErrNotFound
//   - Preserves the custom function for execution
//
// Parameters:
//   - key: A descriptive identifier for the custom extractor.
//     Used for debugging, logging, and Chain metadata. Should be meaningful for introspection.
//     Examples: "X-Custom-Header", "Database-Lookup", "Cache-Key"
//   - fn: The custom function to extract the value from the fiber.Ctx.
//     If nil, the extractor will return ErrNotFound when executed.
//     The function should return (value, nil) on success or ("", error) on failure.
//
// Returns:
//
//	An Extractor that uses the provided function for extraction.
//	If fn is nil, the returned extractor will always return ErrNotFound.
//
// Examples:
//
//	// Custom header with transformation
//	extractor := FromCustom("X-API-Key", func(c fiber.Ctx) (string, error) {
//	    value := c.Get("X-API-Key")
//	    if value == "" {
//	        return "", ErrNotFound
//	    }
//	    return strings.ToUpper(value), nil
//	})
//
//	// Database lookup (pseudo-code)
//	userExtractor := FromCustom("user-from-db", func(c fiber.Ctx) (string, error) {
//	    userID := c.Params("userId")
//	    user, err := db.GetUser(userID)
//	    if err != nil {
//	        return "", err
//	    }
//	    return user.Name, nil
//	})
//
//	// Conditional extraction
//	smartExtractor := FromCustom("smart-auth", func(c fiber.Ctx) (string, error) {
//	    if c.Get("X-Service-Auth") != "" {
//	        return c.Get("X-Service-Auth"), nil
//	    }
//	    return c.Cookies("session"), nil
//	})
func FromCustom(key string, fn func(fiber.Ctx) (string, error)) Extractor {
	if fn == nil {
		fn = func(fiber.Ctx) (string, error) { return "", ErrNotFound }
	}
	return Extractor{
		Extract: fn,
		Key:     key,
		Source:  SourceCustom,
	}
}

// Chain creates an Extractor that tries multiple extractors in order until one succeeds.
// This implements a fallback pattern where multiple extraction sources are attempted in sequence.
//
// The function:
//   - Tries each extractor in the order provided
//   - Returns the first successful extraction (non-empty value with no error)
//   - Skips children with a nil Extract function
//   - Returns the last error encountered if all extractors fail
//   - Returns ErrNotFound if no extractors are provided or all return empty values
//   - Resolve on a chain walks the same children and reports the winning extractor
//
// Parameters:
//   - extractors: A variadic list of Extractor instances to try in sequence.
//     The order matters - more secure/preferred sources should be listed first.
//
// Returns:
//
//	An Extractor that attempts each provided extractor in order.
//	The returned extractor uses the Source and Key from the first extractor for
//	static metadata. Resolve reports the winning child.
//
// Behavior:
//   - Success: Returns the first non-empty value with no error
//   - Partial failure: Continues to next extractor if current returns error or empty value
//   - Total failure: Returns last error encountered, or ErrNotFound if no errors
//   - Empty chain: Always returns ErrNotFound
//   - Extract and Resolve share one cycle guard so a child that
//     re-enters the same chain still returns ErrChainCycle. The guard is cleared
//     on return, so sequential Extract then Resolve is fine.
//
// Examples:
//
//	// Try header first, then cookie, then query param
//	extractor := Chain(
//	    FromHeader("Authorization"),
//	    FromCookie("auth_token"),
//	    FromQuery("token"),
//	)
//
//	// API key from multiple possible sources
//	apiKeyExtractor := Chain(
//	    FromHeader("X-API-Key"),
//	    FromQuery("api_key"),
//	    FromForm("apiKey"),
//	)
//
// Security Note:
//
//	Order extractors by security preference. Most secure sources (headers, cookies)
//	should be attempted before less secure ones (query params, form data).
func Chain(extractors ...Extractor) Extractor {
	notFound := func(fiber.Ctx) (string, error) {
		return "", ErrNotFound
	}

	if len(extractors) == 0 {
		return Extractor{
			Extract: notFound,
			Source:  SourceCustom,
			Key:     "",
			Chain:   []Extractor{},
		}
	}

	// Extract runs kids; Chain exposes pub, a separate copy, so rewriting
	// metadata cannot change which children run.
	kids := append([]Extractor(nil), extractors...)
	pub := append([]Extractor(nil), kids...)
	primarySource := kids[0].Source
	primaryKey := kids[0].Key
	// Keyed on pub, so Resolve shares the cycle identity.
	guard := chainGuard(pub)

	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			st := chainStateFor(c)
			if !st.enter(guard) {
				return "", ErrChainCycle
			}
			defer st.leave()

			var lastErr error // last error encountered (including ErrNotFound)

			// Only inside a Resolve frame, so a bare Extract pays
			// for nothing but the guard.
			capture := st.depth > 0
			for i := range kids {
				kid := &kids[i]
				if kid.Extract == nil {
					continue
				}
				// Only read while a source-aware caller is active: a bare
				// Extract must pay nothing for bookkeeping it never reads.
				var prevWin *Extractor
				var prevGuard *byte
				if capture {
					prevWin, prevGuard = st.win, st.winGuard
				}
				v, err := kid.Extract(c)
				if err == nil && v != "" {
					if capture {
						if st.win == prevWin && st.winGuard == prevGuard {
							// kid recorded nothing of its own, so kid is the
							// origin. A nested chain that did record keeps its
							// innermost child.
							st.win = kid
						}
						// Tag the record as this chain's, so the caller
						// resolving us accepts it and an unrelated chain does
						// not.
						st.winGuard = guard
					}
					return v, nil
				}
				if capture {
					// A child that did not answer records nothing, and must not
					// erase what a sibling chain already recorded.
					st.win, st.winGuard = prevWin, prevGuard
				}
				if err != nil {
					lastErr = err
				}
			}
			if lastErr != nil {
				return "", lastErr
			}
			return "", ErrNotFound
		},
		Source: primarySource,
		Key:    primaryKey,
		Chain:  pub,
	}
}

// token68Chars marks the bytes token68 allows before padding: ALPHA, DIGIT and
// "-._~+/" (RFC 7235 Section 2.1). "=" is only valid as trailing padding, which
// the scan handles separately.
var token68Chars = [256]bool{
	'-': true, '.': true, '_': true, '~': true, '+': true, '/': true,
	'0': true, '1': true, '2': true, '3': true, '4': true, '5': true, '6': true, '7': true, '8': true, '9': true,
	'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true, 'G': true, 'H': true, 'I': true, 'J': true,
	'K': true, 'L': true, 'M': true, 'N': true, 'O': true, 'P': true, 'Q': true, 'R': true, 'S': true, 'T': true,
	'U': true, 'V': true, 'W': true, 'X': true, 'Y': true, 'Z': true,
	'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true, 'h': true, 'i': true, 'j': true,
	'k': true, 'l': true, 'm': true, 'n': true, 'o': true, 'p': true, 'q': true, 'r': true, 's': true, 't': true,
	'u': true, 'v': true, 'w': true, 'x': true, 'y': true, 'z': true,
}

// isValidToken68 checks if a string is a valid token68 per RFC 7235/9110: one
// or more token68 characters, then optional "=" padding, and nothing after it.
//
// NOTE: a swar.MatchRangeMask rewrite benchmarked 16% slower than a scalar
// loop. The table is the other direction and does pay, halving the scan on a
// JWT-sized credential against the range-compare switch it replaced.
func isValidToken68(token string) bool {
	if token == "" || token[0] == '=' {
		return false // Empty, or starting with padding
	}
	i := 0
	for i < len(token) && token68Chars[token[i]] {
		i++
	}
	// Whatever stopped the scan must be padding, and so must the rest.
	for ; i < len(token); i++ {
		if token[i] != '=' {
			return false
		}
	}
	return true
}
