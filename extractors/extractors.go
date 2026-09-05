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
	"strings"
	"unsafe"

	"github.com/gofiber/fiber/v3"
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

// ExtractWithSource returns the extracted value together with its source.
//
// Prefer this over Extract when the caller needs the source that actually
// supplied a value. Source on Extractor is declared/static metadata (for a
// chain, the first child); SourceHeader is the zero value, so a hand-rolled
// Extract without an explicit Source reports SourceHeader. No extra struct
// field is required, so existing unkeyed Extractor literals keep compiling.
//
// Behavior:
//   - Extract set (leaf or chain): call Extract so legacy overrides /
//     decoration (validation, normalization) are honored. For built-in
//     Chain, the winning Source is pushed on a request-local stack during
//     that Extract (no second child walk; survives public Chain reassignment).
//     If Extract succeeds without a capture (custom replacement, or leaf),
//     the declared e.Source is returned — e.Chain is not re-walked.
//   - Chain with nil Extract: walk children (same success rules as Chain.Extract),
//     skip nil Extract, return the winning child's Source.
//   - Neither: ErrNotFound.
//
// The returned Source is meaningful for security decisions only when err is nil.
// On failure it may be static or last-child fallback metadata and must not be
// treated as the origin of a value. Extract is not deprecated in this release.
func ExtractWithSource(e Extractor, c fiber.Ctx) (string, Source, error) {
	// Mark this request frame so Chain.Extract only pushes winner captures when
	// a source-aware caller is active. Bare chain.Extract must not leave stack
	// entries that a later ExtractWithSource on an unrelated leaf would consume.
	enterChainWinCapture(c)
	defer leaveChainWinCapture(c)

	if e.Extract != nil {
		v, err := e.Extract(c)
		// Prefer source captured during Extract. Built-in Chain pushes on
		// success while capture is active, even if e.Chain was cleared.
		if src, ok := popChainWinningSource(c); ok {
			if err != nil {
				return "", e.Source, err
			}
			if v == "" {
				return "", e.Source, ErrNotFound
			}
			return v, src, nil
		}
		// No capture: use declared Source. Do not re-walk e.Chain after a
		// successful custom/replaced Extract — that would attribute the
		// replacement's value to whichever child happens to succeed on peek.
		if err != nil {
			return "", e.Source, err
		}
		if v == "" {
			return "", e.Source, ErrNotFound
		}
		return v, e.Source, nil
	}
	if len(e.Chain) > 0 {
		return extractChainWithSource(e, c)
	}
	return "", e.Source, ErrNotFound
}

// chainGuardFor returns a Locals key shared by Chain.Extract and
// extractChainWithSource for the public Chain backing array.
func chainGuardFor(chain []Extractor) (chainGuardKey, bool) {
	if len(chain) == 0 {
		return chainGuardKey{}, false
	}
	// Address of the first element is stable for the shared backing array.
	return chainGuardKey{id: (*byte)(unsafe.Pointer(&chain[0]))}, true //nolint:gosec // G103: identity key for Locals cycle guard only
}

// chainWinStackKey is a request-local stack of Sources recorded by Chain.Extract.
// A stack (not a key derived from e.Chain) is required so nested chains can
// propagate the true winning child Source outward, and so clearing/reassigning
// the public Chain field cannot orphan or retarget the capture.
type chainWinStackKey struct{}

// chainWinDepthKey counts nested ExtractWithSource frames. Chain.Extract only
// pushes winners while depth > 0 so legacy Extract-only calls leave no stale
// entries for a later source-aware call on the same Ctx.
type chainWinDepthKey struct{}

func chainWinDepth(c fiber.Ctx) int {
	depth, ok := c.Locals(chainWinDepthKey{}).(int)
	if !ok {
		return 0
	}
	return depth
}

func enterChainWinCapture(c fiber.Ctx) {
	c.Locals(chainWinDepthKey{}, chainWinDepth(c)+1)
}

func leaveChainWinCapture(c fiber.Ctx) {
	depth := chainWinDepth(c)
	if depth <= 1 {
		c.Locals(chainWinDepthKey{}, nil)
		return
	}
	c.Locals(chainWinDepthKey{}, depth-1)
}

func chainWinCaptureActive(c fiber.Ctx) bool {
	return chainWinDepth(c) > 0
}

func chainWinStack(c fiber.Ctx) []Source {
	prev, ok := c.Locals(chainWinStackKey{}).([]Source)
	if !ok {
		return nil
	}
	return prev
}

func chainWinStackLen(c fiber.Ctx) int {
	return len(chainWinStack(c))
}

func truncateChainWinStack(c fiber.Ctx, n int) {
	prev := chainWinStack(c)
	if len(prev) == 0 {
		return
	}
	if n <= 0 {
		c.Locals(chainWinStackKey{}, nil)
		return
	}
	if len(prev) > n {
		c.Locals(chainWinStackKey{}, prev[:n])
	}
}

func pushChainWinningSource(c fiber.Ctx, src Source) {
	stack := append(append([]Source(nil), chainWinStack(c)...), src)
	c.Locals(chainWinStackKey{}, stack)
}

func popChainWinningSource(c fiber.Ctx) (Source, bool) {
	prev := chainWinStack(c)
	if len(prev) == 0 {
		return 0, false
	}
	src := prev[len(prev)-1]
	prev = prev[:len(prev)-1]
	if len(prev) == 0 {
		c.Locals(chainWinStackKey{}, nil)
	} else {
		c.Locals(chainWinStackKey{}, prev)
	}
	return src, true
}

func extractChainWithSource(e Extractor, c fiber.Ctx) (string, Source, error) {
	guard, ok := chainGuardFor(e.Chain)
	if !ok {
		return "", SourceCustom, ErrNotFound
	}
	if active, ok := c.Locals(guard).(bool); ok && active {
		return "", e.Source, ErrChainCycle
	}
	c.Locals(guard, true)
	defer c.Locals(guard, false)

	var lastErr error
	lastSource := e.Source
	for _, extractor := range e.Chain {
		if extractor.Extract == nil && len(extractor.Chain) == 0 {
			continue
		}
		// Nested chains and leaves both go through ExtractWithSource.
		if extractor.Extract == nil && len(extractor.Chain) > 0 {
			v, src, err := ExtractWithSource(extractor, c)
			if err == nil && v != "" {
				return v, src, nil
			}
			if err != nil {
				lastErr = err
				lastSource = src
			}
			continue
		}
		if extractor.Extract == nil {
			continue
		}
		v, src, err := ExtractWithSource(extractor, c)
		if err == nil && v != "" {
			return v, src, nil
		}
		if err != nil {
			lastErr = err
			lastSource = src
		}
	}
	if lastErr != nil {
		return "", lastSource, lastErr
	}
	return "", e.Source, ErrNotFound
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

type chainGuardKey struct {
	id *byte
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
		if c.App().Config().UnescapePath {
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
//   - ExtractWithSource on a chain walks the same children and reports the winning Source
//
// Parameters:
//   - extractors: A variadic list of Extractor instances to try in sequence.
//     The order matters - more secure/preferred sources should be listed first.
//
// Returns:
//
//	An Extractor that attempts each provided extractor in order.
//	The returned extractor uses the Source and Key from the first extractor for
//	static metadata. ExtractWithSource reports the winning child's Source.
//
// Behavior:
//   - Success: Returns the first non-empty value with no error
//   - Partial failure: Continues to next extractor if current returns error or empty value
//   - Total failure: Returns last error encountered, or ErrNotFound if no errors
//   - Empty chain: Always returns ErrNotFound
//   - Extract and ExtractWithSource share one cycle guard so a child that
//     re-enters the same chain still returns ErrChainCycle. The guard is cleared
//     on return, so sequential Extract then ExtractWithSource is fine.
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

	// Private execution list captured by Extract. Public Chain is a separate
	// defensive copy so callers can inspect/rewrite metadata without changing
	// which children Extract actually runs.
	kids := append([]Extractor(nil), extractors...)
	pub := append([]Extractor(nil), kids...)
	primarySource := kids[0].Source
	primaryKey := kids[0].Key

	return Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			// Guard on the public Chain array so ExtractWithSource / peek share
			// the same cycle identity (private kids stay execution-only).
			guard, ok := chainGuardFor(pub)
			if !ok {
				return "", ErrNotFound
			}
			if active, ok := c.Locals(guard).(bool); ok && active {
				return "", ErrChainCycle
			}

			c.Locals(guard, true)
			defer c.Locals(guard, false)

			var lastErr error // last error encountered (including ErrNotFound)

			capture := chainWinCaptureActive(c)
			for _, extractor := range kids {
				if extractor.Extract == nil {
					continue
				}
				// Snapshot stack only in source-aware frames so legacy
				// Chain.Extract hot paths skip Locals bookkeeping.
				stackBefore := 0
				if capture {
					stackBefore = chainWinStackLen(c)
				}
				v, err := extractor.Extract(c)
				if err == nil && v != "" {
					if capture {
						// Prefer a Source pushed by a nested Chain.Extract;
						// otherwise the child's declared Source (leaves).
						src := extractor.Source
						if nested, ok := popChainWinningSource(c); ok {
							src = nested
						}
						// Drop any extra leftover pushes from this child.
						truncateChainWinStack(c, stackBefore)
						pushChainWinningSource(c, src)
					}
					return v, nil
				}
				if capture {
					// Child pushed then failed/rejected — discard its capture.
					truncateChainWinStack(c, stackBefore)
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

// isValidToken68 checks if a string is a valid token68 per RFC 7235/9110.
// NOTE: a swar.MatchRangeMask-based rewrite of this scan benchmarked 16%
// slower than this scalar loop (the six-mask character class costs more per
// word than the compiler's optimized switch costs per byte), so it stays.
func isValidToken68(token string) bool {
	if token == "" {
		return false
	}
	paddingStarted := false
	for i := 0; i < len(token); i++ {
		c := token[i]
		switch {
		case (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' || c == '+' || c == '/':
			if paddingStarted {
				return false // No characters allowed after padding starts
			}
		case c == '=':
			if i == 0 {
				return false // Cannot start with padding
			}
			paddingStarted = true
		default:
			return false // Invalid character
		}
	}
	return true
}
