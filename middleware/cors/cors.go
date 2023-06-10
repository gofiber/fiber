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
	// response header to the 'origin' request header when returned true.
	//
	// Optional. Default: nil
	AllowOriginsFunc func(origin string) bool

	// AllowOrigin defines a list of origins that may access the resource.
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
	// actual request can be made using credentials.
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
		if cfg.AllowOrigins == "" {
			cfg.AllowOrigins = ConfigDefault.AllowOrigins
		}
	}

	// Warning logs if both AllowOrigins and AllowOriginsFunc are set
	if cfg.AllowOrigins != ConfigDefault.AllowOrigins && cfg.AllowOriginsFunc != nil {
		log.Warn("[CORS] Both 'AllowOrigins' and 'AllowOriginsFunc' have been defined.")
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

		// Get origin header
		origin := c.Get(fiber.HeaderOrigin)
		allowOrigin := ""

		// Check allowed origins
		for _, o := range allowOrigins {
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = o
				break
			}
			if matchSubdomain(origin, o) {
				allowOrigin = origin
				break
			}
		}

		// Run AllowOriginsFunc if the logic for
		// handling the value in 'AllowOrigins' does
		// not result in allowOrigin being set.
		if (allowOrigin == "" || allowOrigin == ConfigDefault.AllowOrigins) && cfg.AllowOriginsFunc != nil {
			if cfg.AllowOriginsFunc(origin) {
				allowOrigin = origin
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

		// Set Allow-Credentials if set to true
		if cfg.AllowCredentials {
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
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
		}

		// Send 204 No Content
		return c.SendStatus(fiber.StatusNoContent)
	}
}
