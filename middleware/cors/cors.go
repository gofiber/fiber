package cors

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

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

	// Warning logs if both AllowOrigins and AllowOriginsFunc are set
	if len(cfg.AllowOrigins) > 0 && cfg.AllowOriginsFunc != nil {
		log.Warn("[CORS] Both 'AllowOrigins' and 'AllowOriginsFunc' have been defined.")
	}

	// allowOrigins is a slice of strings that contains the allowed origins
	// defined in the 'AllowOrigins' configuration.
	allowOrigins := []string{}
	allowSOrigins := []subdomain{}
	allowAllOrigins := false

	// Validate and normalize static AllowOrigins
	if len(cfg.AllowOrigins) == 0 && cfg.AllowOriginsFunc == nil {
		allowAllOrigins = true
	}
	for _, origin := range cfg.AllowOrigins {
		if origin == "*" {
			allowAllOrigins = true
			break
		}
		if i := strings.Index(origin, "://*."); i != -1 {
			trimmedOrigin := strings.TrimSpace(origin[:i+3] + origin[i+4:])
			isValid, normalizedOrigin := normalizeOrigin(trimmedOrigin)
			if !isValid {
				panic("[CORS] Invalid origin format in configuration: " + trimmedOrigin)
			}
			sd := subdomain{prefix: normalizedOrigin[:i+3], suffix: normalizedOrigin[i+3:]}
			allowSOrigins = append(allowSOrigins, sd)
		} else {
			trimmedOrigin := strings.TrimSpace(origin)
			isValid, normalizedOrigin := normalizeOrigin(trimmedOrigin)
			if !isValid {
				panic("[CORS] Invalid origin format in configuration: " + trimmedOrigin)
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

		// Get originHeader header
		originHeader := strings.ToLower(c.Get(fiber.HeaderOrigin))

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
			for _, origin := range allowOrigins {
				if origin == originHeader {
					allowOrigin = originHeader
					break
				}
			}

			// Check if the origin is in the list of allowed subdomains
			if allowOrigin == "" {
				for _, sOrigin := range allowSOrigins {
					if sOrigin.match(originHeader) {
						allowOrigin = originHeader
						break
					}
				}
			}
		}

		// Run AllowOriginsFunc if the logic for
		// handling the value in 'AllowOrigins' does
		// not result in allowOrigin being set.
		if allowOrigin == "" && cfg.AllowOriginsFunc != nil && cfg.AllowOriginsFunc(originHeader) {
			allowOrigin = originHeader
		}

		// Simple request
		// Ommit allowMethods and allowHeaders, only used for pre-flight requests
		if c.Method() != fiber.MethodOptions {
			if !allowAllOrigins {
				// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
				c.Vary(fiber.HeaderOrigin)
			}
			setSimpleHeaders(c, allowOrigin, maxAge, cfg)
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

		setSimpleHeaders(c, allowOrigin, maxAge, cfg)

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
func setSimpleHeaders(c fiber.Ctx, allowOrigin, maxAge string, cfg Config) {
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

	// Set MaxAge if set
	if cfg.MaxAge > 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, maxAge)
	} else if cfg.MaxAge < 0 {
		c.Set(fiber.HeaderAccessControlMaxAge, "0")
	}

	// Set Expose-Headers if not empty
	if len(cfg.ExposeHeaders) > 0 {
		c.Set(fiber.HeaderAccessControlExposeHeaders, strings.Join(cfg.ExposeHeaders, ", "))
	}
}
