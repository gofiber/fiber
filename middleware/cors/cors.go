package cors

import (
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	originpkg "github.com/gofiber/fiber/v3/internal/origin"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// corsSchemes is the scheme policy for Access-Control-Allow-Origin: any
// scheme, since the browser is the party enforcing the check and CORS is
// used beyond http(s).
const corsSchemes = originpkg.AnyScheme

const redactedValue = "[redacted]"

// headerLists holds the comma-joined forms of the list-valued CORS response
// headers. Their sources are fixed when New returns, so each is built once
// there instead of being re-joined on every request that emits it.
type headerLists struct {
	allowMethods  string
	allowHeaders  string
	exposeHeaders string

	// Whether each list was configured at all, which is not the same question
	// as whether its joined form is empty: AllowHeaders: []string{""} is a
	// configured list that joins to "". For Access-Control-Allow-Headers the
	// difference decides the response — an empty value authorizes no headers,
	// while an absent list falls back to echoing whatever the request asked
	// for in Access-Control-Request-Headers.
	hasAllowMethods  bool
	hasAllowHeaders  bool
	hasExposeHeaders bool
}

// Vary takes its field names variadically, and a fresh "..." argument list is
// a slice the compiler has to heap-allocate: Vary's result reaches the header
// store, so escape analysis marks the elements as leaking even though fasthttp
// copies the bytes. Passing a package-level slice with "..." hands the callee
// the existing backing array instead, which removed the only allocation on the
// simple-request path and three of the four on preflight. They are only ever
// passed through vary below, which keeps the shared arrays away from a Ctx
// that might write to them.
var (
	varyOrigin           = []string{fiber.HeaderOrigin}
	varyPreflight        = []string{fiber.HeaderAccessControlRequestMethod, fiber.HeaderAccessControlRequestHeaders, fiber.HeaderOrigin}
	varyPreflightPrivate = []string{
		fiber.HeaderAccessControlRequestMethod,
		fiber.HeaderAccessControlRequestHeaders,
		fiber.HeaderAccessControlRequestPrivateNetwork,
		fiber.HeaderOrigin,
	}
)

// vary adds fields to the Vary response header.
//
// The lists above are shared by every request this middleware serves, so they
// must not reach an implementation that could write to them: a custom Ctx is
// free to sort or otherwise rewrite its variadic argument, and doing that to a
// package-level array would corrupt the field names of later responses and
// race with the requests running alongside. DefaultCtx only reads what Vary is
// given, so it gets the shared slice; anything else gets a copy of its own.
func vary(c fiber.Ctx, fields []string) {
	if dc, ok := c.(*fiber.DefaultCtx); ok {
		dc.Vary(fields...)
		return
	}
	c.Vary(slices.Clone(fields)...)
}

// isOriginSerializedOrNull checks if the origin is a serialized origin or the literal "null".
// It returns two booleans: (isSerialized, isNull).
func isOriginSerializedOrNull(originHeaderRaw string) (isSerialized, isNull bool) { //nolint:nonamedreturns // gocritic unnamedResult prefers naming serialization and null status results
	if originHeaderRaw == "null" {
		return false, true
	}

	_, originIsSerialized := originpkg.Normalize(originHeaderRaw, corsSchemes)
	return originIsSerialized, false
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if len(cfg.AllowMethods) == 0 {
			cfg.AllowMethods = ConfigDefault.AllowMethods
		}
	}

	redactValues := !cfg.DisableValueRedaction

	maskValue := func(value string) string {
		if redactValues {
			return redactedValue
		}
		return value
	}

	// Warning logs if both AllowOrigins and AllowOriginsFunc are set
	if len(cfg.AllowOrigins) > 0 && cfg.AllowOriginsFunc != nil {
		log.Warn("[CORS] Both 'AllowOrigins' and 'AllowOriginsFunc' have been defined.")
	}

	// allowOrigins is a set of strings that contains the allowed origins
	// defined in the 'AllowOrigins' configuration.
	allowOrigins := make(map[string]struct{}, len(cfg.AllowOrigins))
	allowSubOrigins := []originpkg.Subdomain{}

	// Validate and normalize static AllowOrigins
	allowAllOrigins := len(cfg.AllowOrigins) == 0 && cfg.AllowOriginsFunc == nil
	for _, origin := range cfg.AllowOrigins {
		if origin == "*" {
			allowAllOrigins = true
			break
		}

		trimmedOrigin := utils.TrimSpace(origin)
		pattern, ok := originpkg.ParsePattern(trimmedOrigin, corsSchemes)
		if !ok {
			panic("[CORS] Invalid origin format in configuration: " + maskValue(trimmedOrigin))
		}
		if pattern.Wildcard {
			allowSubOrigins = append(allowSubOrigins, pattern.Subdomain)
		} else {
			allowOrigins[pattern.Origin] = struct{}{}
		}
	}

	// Validate CORS credentials configuration
	if cfg.AllowCredentials && allowAllOrigins {
		panic("[CORS] Configuration error: When 'AllowCredentials' is set to true, 'AllowOrigins' cannot contain a wildcard origin '*'. Please specify allowed origins explicitly or adjust 'AllowCredentials' setting.")
	}

	// Warn if allowAllOrigins is set to true and AllowOriginsFunc is defined
	if allowAllOrigins && cfg.AllowOriginsFunc != nil {
		log.Warn("[CORS] 'AllowOrigins' is set to allow all origins, 'AllowOriginsFunc' will not be used.")
	}

	// Convert int to string
	maxAge := strconv.Itoa(cfg.MaxAge)

	// The list-valued response headers are built from configuration that
	// cannot change once New returns, so they are joined here rather than on
	// every request. strings.Join was the only remaining allocation on the
	// preflight path.
	lists := headerLists{
		hasAllowMethods:  len(cfg.AllowMethods) > 0,
		hasAllowHeaders:  len(cfg.AllowHeaders) > 0,
		hasExposeHeaders: len(cfg.ExposeHeaders) > 0,
	}
	if lists.hasAllowMethods {
		lists.allowMethods = strings.Join(cfg.AllowMethods, ", ")
	}
	if lists.hasAllowHeaders {
		lists.allowHeaders = strings.Join(cfg.AllowHeaders, ", ")
	}
	if lists.hasExposeHeaders {
		lists.exposeHeaders = strings.Join(cfg.ExposeHeaders, ", ")
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Every request header here is read case-insensitively, as csrf already
		// does: Ctx.Get is byte-exact, so under DisableHeaderNormalizing the
		// lower-case names HTTP/2 sends read as absent and CORS never applied.
		// Origin, with the original case kept for the response.
		originHeaderRaw, _ := headerlookup.Value(c, fiber.HeaderOrigin)
		originHeader := utilsstrings.ToLower(originHeaderRaw)

		// If the request does not have Origin header, the request is outside the scope of CORS
		if originHeader == "" {
			// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
			// Unless all origins are allowed, we include the Vary header to cache the response correctly
			if !allowAllOrigins {
				vary(c, varyOrigin)
			}

			return c.Next()
		}

		// If it's a preflight request and doesn't have Access-Control-Request-Method header, it's outside the scope of CORS.
		// The header is read only for OPTIONS, so plain cross-origin requests skip the lookup.
		requestMethod := ""
		if c.Method() == fiber.MethodOptions {
			requestMethod, _ = headerlookup.Value(c, fiber.HeaderAccessControlRequestMethod)
		}
		if c.Method() == fiber.MethodOptions && requestMethod == "" {
			// Response to OPTIONS request should not be cached but,
			// some caching can be configured to cache such responses.
			// To Avoid poisoning the cache, we include the Vary header
			// for non-CORS OPTIONS requests:
			vary(c, varyOrigin)
			return c.Next()
		}

		// Set default allowOrigin to empty string
		allowOrigin := ""

		// Check allowed origins
		if allowAllOrigins {
			allowOrigin = "*"
		} else {
			// Check if the origin is in the list of allowed origins
			if _, ok := allowOrigins[originHeader]; ok {
				allowOrigin = originHeaderRaw
			}

			// Check if the origin is in the list of allowed subdomains
			if allowOrigin == "" && originpkg.MatchAny(allowSubOrigins, originHeader, corsSchemes) {
				allowOrigin = originHeaderRaw
			}
		}

		// Run AllowOriginsFunc if the logic for
		// handling the value in 'AllowOrigins' does
		// not result in allowOrigin being set.
		if allowOrigin == "" && cfg.AllowOriginsFunc != nil && cfg.AllowOriginsFunc(originHeaderRaw) {
			originIsSerialized, originIsNull := isOriginSerializedOrNull(originHeaderRaw)
			if originIsSerialized || originIsNull {
				allowOrigin = originHeaderRaw
			}
		}

		// Simple request
		// Omit allowMethods and allowHeaders, only used for pre-flight requests
		if c.Method() != fiber.MethodOptions {
			if !allowAllOrigins {
				// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
				vary(c, varyOrigin)
			}
			setSimpleHeaders(c, allowOrigin, &cfg, &lists)
			return c.Next()
		}

		// Pre-flight request

		// Response to OPTIONS request should not be cached but,
		// some caching can be configured to cache such responses.
		// To avoid poisoning the cache, we include the Vary header
		// of preflight responses, set in a single variadic Vary call
		// per branch since every Vary call scans all response headers.
		privateNetworkRequested := false
		if cfg.AllowPrivateNetwork {
			// Read only when the feature is on: with the default config the
			// header's value is discarded, so the lookup would be pure cost.
			privateNetwork, _ := headerlookup.Value(c, fiber.HeaderAccessControlRequestPrivateNetwork)
			privateNetworkRequested = privateNetwork == "true"
		}
		if privateNetworkRequested {
			vary(c, varyPreflightPrivate)
			c.Set(fiber.HeaderAccessControlAllowPrivateNetwork, "true")
		} else {
			vary(c, varyPreflight)
		}

		setPreflightHeaders(c, allowOrigin, maxAge, &cfg, &lists)

		// Set Preflight headers
		if lists.hasAllowMethods {
			c.Set(fiber.HeaderAccessControlAllowMethods, lists.allowMethods)
		}
		if lists.hasAllowHeaders {
			c.Set(fiber.HeaderAccessControlAllowHeaders, lists.allowHeaders)
		} else {
			// Combined, not Value: this one is a list field, so a peer may
			// legally split it over two lines and both name headers to allow.
			h := headerlookup.Combined(c, fiber.HeaderAccessControlRequestHeaders)
			if h != "" {
				c.Set(fiber.HeaderAccessControlAllowHeaders, h)
			}
		}

		// Send 204 No Content
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// Function to set Simple CORS headers
func setSimpleHeaders(c fiber.Ctx, allowOrigin string, cfg *Config, lists *headerLists) {
	if cfg == nil {
		return
	}

	if cfg.AllowCredentials {
		// When AllowCredentials is true, set the Access-Control-Allow-Origin to the specific origin instead of '*'
		if allowOrigin == "*" {
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
			log.Warn("[CORS] 'AllowCredentials' is true, but 'AllowOrigins' cannot be set to '*'.")
		} else if allowOrigin != "" {
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
		}
	} else if allowOrigin != "" {
		// For non-credential requests, it's safe to set to '*' or specific origins
		c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
	}

	// Set Expose-Headers if not empty. lists is nil-tolerant for the same
	// reason cfg is: the helper is called directly by tests.
	if lists != nil && lists.hasExposeHeaders {
		c.Set(fiber.HeaderAccessControlExposeHeaders, lists.exposeHeaders)
	}
}

// Function to set Preflight CORS headers
func setPreflightHeaders(c fiber.Ctx, allowOrigin, maxAge string, cfg *Config, lists *headerLists) {
	setSimpleHeaders(c, allowOrigin, cfg, lists)

	// Set MaxAge if set
	if cfg != nil && cfg.MaxAge > 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, maxAge)
	} else if cfg != nil && cfg.MaxAge < 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, "0")
	}
}
