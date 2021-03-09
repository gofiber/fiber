package csrf

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Create manager to simplify storage operations ( see manager.go )
	manager := newManager(cfg.Storage)

	dummyValue := []byte{'+'}

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		var token string

		// Action depends on the HTTP method
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
			// Declare empty token and try to get existing CSRF from cookie
			token = c.Cookies(cfg.CookieName)
		default:
			// Assume that anything not defined as 'safe' by RFC7231 needs protection

			// Extract token from client request i.e. header, query, param, form or cookie
			token, err = cfg.extractor(c)
			if err != nil {
				return cfg.ErrorHandler(c, err)
			}

			// if token does not exist in Storage
			if manager.getRaw(token) == nil {
				// Expire cookie
				c.Cookie(&fiber.Cookie{
					Name:     cfg.CookieName,
					Domain:   cfg.CookieDomain,
					Path:     cfg.CookiePath,
					Expires:  time.Now().Add(-1 * time.Minute),
					Secure:   cfg.CookieSecure,
					HTTPOnly: cfg.CookieHTTPOnly,
					SameSite: cfg.CookieSameSite,
				})
				return cfg.ErrorHandler(c, err)
			}
		}

		// Generate CSRF token if not exist
		if token == "" {
			// And generate a new token
			token = cfg.KeyGenerator()
		}

		// Add/update token to Storage
		manager.setRaw(token, dummyValue, cfg.Expiration)

		// Create cookie to pass token to client
		cookie := &fiber.Cookie{
			Name:     cfg.CookieName,
			Value:    token,
			Domain:   cfg.CookieDomain,
			Path:     cfg.CookiePath,
			Expires:  time.Now().Add(cfg.Expiration),
			Secure:   cfg.CookieSecure,
			HTTPOnly: cfg.CookieHTTPOnly,
			SameSite: cfg.CookieSameSite,
		}
		// Set cookie to response
		c.Cookie(cookie)

		// Protect clients from caching the response by telling the browser
		// a new header value is generated
		c.Vary(fiber.HeaderCookie)

		// Store token in context if set
		if cfg.ContextKey != "" {
			c.Locals(cfg.ContextKey, token)
		}

		// Continue stack
		return c.Next()
	}
}
