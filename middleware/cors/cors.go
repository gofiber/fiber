package cors

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// AllowOriginsFunc defines a function that will set the 'access-control-allow-origin'
	// response header to the 'origin' request header when returned true. This allows for
	// dynamic evaluation of allowed origins. Note if AllowCredentials is true, wildcard origins
	// will be not have the 'access-control-allow-credentials' header set to 'true'.
	//
	// Optional. Default: nil
	AllowOriginsFunc func(origin string) bool

	// AllowOrigin defines a comma separated list of origins that may access the resource.
	//
	// Optional. Default value "*"
	AllowOrigins string

	// AllowMethods defines a list methods allowed when accessing the resource.
	// This is used in response to a preflight request.
	//
	// Optional. Default value "GET,POST,HEAD,PUT,DELETE,PATCH"
	AllowMethods string

	// AllowHeaders defines a list of request headers that can be used when
	// making the actual request. This is in response to a preflight request.
	//
	// Optional. Default value "".
	AllowHeaders string

	// AllowCredentials indicates whether or not the response to the request
	// can be exposed when the credentials flag is true. When used as part of
	// a response to a preflight request, this indicates whether or not the
	// actual request can be made using credentials. Note: If true, AllowOrigins
	// cannot be set to a wildcard ("*") to prevent security vulnerabilities.
	//
	// Optional. Default value false.
	AllowCredentials bool

	// ExposeHeaders defines a whitelist headers that clients are allowed to
	// access.
	//
	// Optional. Default value "".
	ExposeHeaders string

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached.
	// If you pass MaxAge 0, Access-Control-Max-Age header will not be added and
	// browser will use 5 seconds by default.
	// To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header 0.
	//
	// Optional. Default value 0.
	MaxAge int
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:             nil,
	AllowOriginsFunc: nil,
	AllowOrigins:     "*",
	AllowMethods: strings.Join([]string{
		fiber.MethodGet,
		fiber.MethodPost,
		fiber.MethodHead,
		fiber.MethodPut,
		fiber.MethodDelete,
		fiber.MethodPatch,
	}, ","),
	AllowHeaders:     "",
	AllowCredentials: false,
	ExposeHeaders:    "",
	MaxAge:           0,
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.AllowMethods == "" {
			cfg.AllowMethods = ConfigDefault.AllowMethods
		}
		// When none of the AllowOrigins or AllowOriginsFunc config was defined, set the default AllowOrigins value with "*"
		if cfg.AllowOrigins == "" && cfg.AllowOriginsFunc == nil {
			cfg.AllowOrigins = ConfigDefault.AllowOrigins
		}
	}

	// Warning logs if both AllowOrigins and AllowOriginsFunc are set
	if cfg.AllowOrigins != "" && cfg.AllowOriginsFunc != nil {
		log.Warn("[CORS] Both 'AllowOrigins' and 'AllowOriginsFunc' have been defined.")
	}

	// Validate CORS credentials configuration
	if cfg.AllowCredentials && cfg.AllowOrigins == "*" {
		panic("[CORS] Insecure setup, 'AllowCredentials' is set to true, and 'AllowOrigins' is set to a wildcard.")
	}

	// Validate and normalize static AllowOrigins if not using AllowOriginsFunc
	if cfg.AllowOriginsFunc == nil && cfg.AllowOrigins != "" && cfg.AllowOrigins != "*" {
		validatedOrigins := []string{}
		for _, origin := range strings.Split(cfg.AllowOrigins, ",") {
			isValid, normalizedOrigin := normalizeOrigin(origin)
			if isValid {
				validatedOrigins = append(validatedOrigins, normalizedOrigin)
			} else {
				log.Warnf("[CORS] Invalid origin format in configuration: %s", origin)
				panic("[CORS] Invalid origin provided in configuration")
			}
		}
		cfg.AllowOrigins = strings.Join(validatedOrigins, ",")
	}

	// Convert string to slice
	allowOrigins := strings.Split(strings.ReplaceAll(cfg.AllowOrigins, " ", ""), ",")

	// Strip white spaces
	allowMethods := strings.ReplaceAll(cfg.AllowMethods, " ", "")
	allowHeaders := strings.ReplaceAll(cfg.AllowHeaders, " ", "")
	exposeHeaders := strings.ReplaceAll(cfg.ExposeHeaders, " ", "")

	// Convert int to string
	maxAge := strconv.Itoa(cfg.MaxAge)

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get originHeader header
		originHeader := c.Get(fiber.HeaderOrigin)
		allowOrigin := ""

		// Check allowed origins
		for _, origin := range allowOrigins {
			if origin == "*" {
				allowOrigin = "*"
				break
			}
			if validateDomain(originHeader, origin) {
				allowOrigin = originHeader
				break
			}
		}

		// Run AllowOriginsFunc if the logic for
		// handling the value in 'AllowOrigins' does
		// not result in allowOrigin being set.
		if allowOrigin == "" && cfg.AllowOriginsFunc != nil {
			if cfg.AllowOriginsFunc(originHeader) {
				allowOrigin = originHeader
			}
		}

		// Simple request
		if c.Method() != fiber.MethodOptions {
			c.Vary(fiber.HeaderOrigin)
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)

			if cfg.AllowCredentials {
				c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
			}
			if exposeHeaders != "" {
				c.Set(fiber.HeaderAccessControlExposeHeaders, exposeHeaders)
			}
			return c.Next()
		}

		// Preflight request
		c.Vary(fiber.HeaderOrigin)
		c.Vary(fiber.HeaderAccessControlRequestMethod)
		c.Vary(fiber.HeaderAccessControlRequestHeaders)
		c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
		c.Set(fiber.HeaderAccessControlAllowMethods, allowMethods)

		if cfg.AllowCredentials {
			// When AllowCredentials is true, set the Access-Control-Allow-Origin to the specific origin instead of '*'
			if allowOrigin != "*" && allowOrigin != "" {
				c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
				c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
			} else if allowOrigin == "*" {
				log.Warn("[CORS] 'AllowCredentials' is true, but 'AllowOrigins' cannot be set to '*'.")
			}
		} else {
			// For non-credential requests, it's safe to set to '*' or specific origins
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
		}

		// Set Allow-Headers if not empty
		if allowHeaders != "" {
			c.Set(fiber.HeaderAccessControlAllowHeaders, allowHeaders)
		} else {
			h := c.Get(fiber.HeaderAccessControlRequestHeaders)
			if h != "" {
				c.Set(fiber.HeaderAccessControlAllowHeaders, h)
			}
		}

		// Set MaxAge is set
		if cfg.MaxAge > 0 {
			c.Set(fiber.HeaderAccessControlMaxAge, maxAge)
		} else if cfg.MaxAge < 0 {
			c.Set(fiber.HeaderAccessControlMaxAge, "0")
		}

		// Send 204 No Content
		return c.SendStatus(fiber.StatusNoContent)
	}
}
