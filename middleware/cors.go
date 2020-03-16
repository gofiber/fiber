package middleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber"
)

// CORSConfig ...
type CORSConfig struct {
	Skip func(*fiber.Ctx) bool
	// Optional. Default value []string{"*"}.
	AllowOrigins []string
	// Optional. Default value []string{"GET","POST","HEAD","PUT","DELETE","PATCH"}
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

// CorsConfigDefault is the defaul Cors middleware config.
var CorsConfigDefault = CORSConfig{
	Skip:         nil,
	AllowOrigins: []string{"*"},
	AllowMethods: []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodHead,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	},
}

// Cors ...
func Cors(config ...CORSConfig) func(*fiber.Ctx) {
	log.Println("Warning: middleware.Cors() is deprecated since v1.8.2, please use github.com/gofiber/cors")
	// Init config
	var cfg CORSConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = CorsConfigDefault.AllowOrigins
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = CorsConfigDefault.AllowMethods
	}
	// Middleware settings
	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")
	maxAge := strconv.Itoa(cfg.MaxAge)
	// Middleware function
	return func(c *fiber.Ctx) {
		// Skip middleware if Skip returns true
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
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
		if c.Method() != http.MethodOptions {
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

func matchScheme(domain, pattern string) bool {
	didx := strings.Index(domain, ":")
	pidx := strings.Index(pattern, ":")
	return didx != -1 && pidx != -1 && domain[:didx] == pattern[:pidx]
}

// matchSubdomain compares authority with wildcard
func matchSubdomain(domain, pattern string) bool {
	if !matchScheme(domain, pattern) {
		return false
	}
	didx := strings.Index(domain, "://")
	pidx := strings.Index(pattern, "://")
	if didx == -1 || pidx == -1 {
		return false
	}
	domAuth := domain[didx+3:]
	// to avoid long loop by invalid long domain
	if len(domAuth) > 253 {
		return false
	}
	patAuth := pattern[pidx+3:]

	domComp := strings.Split(domAuth, ".")
	patComp := strings.Split(patAuth, ".")
	for i := len(domComp)/2 - 1; i >= 0; i-- {
		opp := len(domComp) - 1 - i
		domComp[i], domComp[opp] = domComp[opp], domComp[i]
	}
	for i := len(patComp)/2 - 1; i >= 0; i-- {
		opp := len(patComp) - 1 - i
		patComp[i], patComp[opp] = patComp[opp], patComp[i]
	}

	for i, v := range domComp {
		if len(patComp) <= i {
			return false
		}
		p := patComp[i]
		if p == "*" {
			return true
		}
		if p != v {
			return false
		}
	}
	return false
}
