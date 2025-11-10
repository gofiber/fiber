package cors

import (
	"slices"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

const redactedValue = "[redacted]"

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

	// allowOrigins is a slice of strings that contains the allowed origins
	// defined in the 'AllowOrigins' configuration.
	allowOrigins := []string{}
	allowSubOrigins := []subdomain{}

	// Validate and normalize static AllowOrigins
	allowAllOrigins := len(cfg.AllowOrigins) == 0 && cfg.AllowOriginsFunc == nil
	for _, origin := range cfg.AllowOrigins {
		if origin == "*" {
			allowAllOrigins = true
			break
		}

		trimmedOrigin := utils.Trim(origin, ' ')
		if i := strings.Index(trimmedOrigin, "://*."); i != -1 {
			withoutWildcard := trimmedOrigin[:i+len("://")] + trimmedOrigin[i+len("://*."):]
			isValid, normalizedOrigin := normalizeOrigin(withoutWildcard)
			if !isValid {
				panic("[CORS] Invalid origin format in configuration: " + maskValue(trimmedOrigin))
			}
			schemeSep := strings.Index(normalizedOrigin, "://") + len("://")
			sd := subdomain{prefix: normalizedOrigin[:schemeSep], suffix: normalizedOrigin[schemeSep:]}
			allowSubOrigins = append(allowSubOrigins, sd)
		} else {
			isValid, normalizedOrigin := normalizeOrigin(trimmedOrigin)
			if !isValid {
				panic("[CORS] Invalid origin format in configuration: " + maskValue(trimmedOrigin))
			}
			allowOrigins = append(allowOrigins, normalizedOrigin)
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

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get origin header preserving the original case for the response
		originHeaderRaw := c.Get(fiber.HeaderOrigin)
		originHeader := strings.ToLower(originHeaderRaw)

		// If the request does not have Origin header, the request is outside the scope of CORS
		if originHeader == "" {
			// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
			// Unless all origins are allowed, we include the Vary header to cache the response correctly
			if !allowAllOrigins {
				c.Vary(fiber.HeaderOrigin)
			}

			return c.Next()
		}

		// If it's a preflight request and doesn't have Access-Control-Request-Method header, it's outside the scope of CORS
		if c.Method() == fiber.MethodOptions && c.Get(fiber.HeaderAccessControlRequestMethod) == "" {
			// Response to OPTIONS request should not be cached but,
			// some caching can be configured to cache such responses.
			// To Avoid poisoning the cache, we include the Vary header
			// for non-CORS OPTIONS requests:
			c.Vary(fiber.HeaderOrigin)
			return c.Next()
		}

		// Set default allowOrigin to empty string
		allowOrigin := ""

		// Check allowed origins
		if allowAllOrigins {
			allowOrigin = "*"
		} else {
			// Check if the origin is in the list of allowed origins
			if slices.Contains(allowOrigins, originHeader) {
				allowOrigin = originHeaderRaw
			}

			// Check if the origin is in the list of allowed subdomains
			if allowOrigin == "" {
				for _, sOrigin := range allowSubOrigins {
					if sOrigin.match(originHeader) {
						allowOrigin = originHeaderRaw
						break
					}
				}
			}
		}

		// Run AllowOriginsFunc if the logic for
		// handling the value in 'AllowOrigins' does
		// not result in allowOrigin being set.
		if allowOrigin == "" && cfg.AllowOriginsFunc != nil && cfg.AllowOriginsFunc(originHeaderRaw) {
			allowOrigin = originHeaderRaw
		}

		// Simple request
		// Omit allowMethods and allowHeaders, only used for pre-flight requests
		if c.Method() != fiber.MethodOptions {
			if !allowAllOrigins {
				// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
				c.Vary(fiber.HeaderOrigin)
			}
			setSimpleHeaders(c, allowOrigin, &cfg)
			return c.Next()
		}

		// Pre-flight request

		// Response to OPTIONS request should not be cached but,
		// some caching can be configured to cache such responses.
		// To Avoid poisoning the cache, we include the Vary header
		// of preflight responses:
		c.Vary(fiber.HeaderAccessControlRequestMethod)
		c.Vary(fiber.HeaderAccessControlRequestHeaders)
		if cfg.AllowPrivateNetwork && c.Get(fiber.HeaderAccessControlRequestPrivateNetwork) == "true" {
			c.Vary(fiber.HeaderAccessControlRequestPrivateNetwork)
			c.Set(fiber.HeaderAccessControlAllowPrivateNetwork, "true")
		}
		c.Vary(fiber.HeaderOrigin)

		setPreflightHeaders(c, allowOrigin, maxAge, &cfg)

		// Set Preflight headers
		if len(cfg.AllowMethods) > 0 {
			c.Set(fiber.HeaderAccessControlAllowMethods, strings.Join(cfg.AllowMethods, ", "))
		}
		if len(cfg.AllowHeaders) > 0 {
			c.Set(fiber.HeaderAccessControlAllowHeaders, strings.Join(cfg.AllowHeaders, ", "))
		} else {
			h := c.Get(fiber.HeaderAccessControlRequestHeaders)
			if h != "" {
				c.Set(fiber.HeaderAccessControlAllowHeaders, h)
			}
		}

		// Send 204 No Content
		return c.SendStatus(fiber.StatusNoContent)
	}
}

// Function to set Simple CORS headers
func setSimpleHeaders(c fiber.Ctx, allowOrigin string, cfg *Config) {
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

	// Set Expose-Headers if not empty
	if len(cfg.ExposeHeaders) > 0 {
		c.Set(fiber.HeaderAccessControlExposeHeaders, strings.Join(cfg.ExposeHeaders, ", "))
	}
}

// Function to set Preflight CORS headers
func setPreflightHeaders(c fiber.Ctx, allowOrigin, maxAge string, cfg *Config) {
	setSimpleHeaders(c, allowOrigin, cfg)

	// Set MaxAge if set
	if cfg != nil && cfg.MaxAge > 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, maxAge)
	} else if cfg != nil && cfg.MaxAge < 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, "0")
	}
}
