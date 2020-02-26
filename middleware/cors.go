package middleware

import (
	"strconv"
	"strings"

	".."
)

// package main
//
// import (
// 	"github.com/gofiber/fiber"
// 	"github.com/gofiber/fiber/middleware"
// )
//
// func main() {
// 	app := fiber.New()
// 	app.Use(middleware.Cors())
// 	app.Get("/", func(c *fiber.Ctx) {
// 		c.Send("Hello jQuery!")
// 	})
// 	app.Listen(3000)
// }

// CORSConfig ...
type CORSConfig struct {
	// Optional. Default value []string{"*"}.
	AllowOrigins []string

	// Optional. Default value DefaultCORSConfig.AllowMethods.
	AllowMethods []string

	// Optional. Default value []string{}.
	AllowHeaders []string

	// Optional. Default value false.
	AllowCredentials bool

	// Optional. Default value []string{}.
	ExposeHeaders []string

	// Optional. Default value 0.
	MaxAge int
}

var defaultCORSConfig = CORSConfig{
	AllowOrigins: []string{"*"},
	AllowMethods: []string{
		fiber.MethodGet,
		fiber.MethodPost,
		fiber.MethodHead,
		fiber.MethodPut,
		fiber.MethodDelete,
		fiber.MethodPatch,
	},
}

// Cors ...
func Cors(config ...CORSConfig) func(*fiber.Ctx) {
	// Init config, if any...
	var cfg CORSConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = defaultCORSConfig.AllowOrigins
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = defaultCORSConfig.AllowMethods
	}
	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")
	maxAge := strconv.Itoa(cfg.MaxAge)

	return func(c *fiber.Ctx) {
		origin := c.Get(fiber.HeaderOrigin)
		allowOrigin := ""
		// Check allowed origins
		for _, o := range cfg.AllowOrigins {
			if o == "*" && cfg.AllowCredentials {
				allowOrigin = origin
				break
			}
			if o == "*" || o == origin {
				allowOrigin = o
				break
			}
			if matchSubdomain(origin, o) {
				allowOrigin = origin
				break
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
			c.Next()
			return
		}
		// Preflight request
		c.Vary(fiber.HeaderOrigin)
		c.Vary(fiber.HeaderAccessControlRequestMethod)
		c.Vary(fiber.HeaderAccessControlRequestHeaders)
		c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
		c.Set(fiber.HeaderAccessControlAllowMethods, allowMethods)

		if cfg.AllowCredentials {
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
		}
		if allowHeaders != "" {
			c.Set(fiber.HeaderAccessControlAllowHeaders, allowHeaders)
		} else {
			h := c.Get(fiber.HeaderAccessControlRequestHeaders)
			if h != "" {
				c.Set(fiber.HeaderAccessControlAllowHeaders, h)
			}
		}
		if cfg.MaxAge > 0 {
			c.Set(fiber.HeaderAccessControlMaxAge, maxAge)
		}
		c.SendStatus(204) // No Content
	}
}
