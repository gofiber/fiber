package csrf

import (
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// TokenLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	//
	// Optional. Default value "header:X-CSRF-Token".
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	TokenLookup string

	// Cookie
	//
	// Optional.
	Cookie *fiber.Cookie

	// CookieExpires is the duration before the cookie will expire
	//
	// Optional. Default: 24 * time.Hour
	CookieExpires time.Duration

	// Context key to store generated CSRF token into context.
	//
	// Optional. Default value "csrf".
	ContextKey string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:        nil,
	TokenLookup: "header:X-CSRF-Token",
	ContextKey:  "csrf",
	Cookie: &fiber.Cookie{
		Name:     "_csrf",
		SameSite: "Strict",
	},
	CookieExpires: 24 * time.Hour,
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.TokenLookup == "" {
			cfg.TokenLookup = ConfigDefault.TokenLookup
		}
		if cfg.ContextKey == "" {
			cfg.ContextKey = ConfigDefault.ContextKey
		}
		if cfg.CookieExpires == 0 {
			cfg.CookieExpires = ConfigDefault.CookieExpires
		}
		if cfg.Cookie != nil {
			if cfg.Cookie.Name == "" {
				cfg.Cookie.Name = ConfigDefault.Cookie.Name
			}
			if cfg.Cookie.SameSite == "" {
				cfg.Cookie.SameSite = ConfigDefault.Cookie.SameSite
			}
		} else {
			cfg.Cookie = ConfigDefault.Cookie
		}
	}

	// Generate the correct extractor to get the token from the correct location
	selectors := strings.Split(cfg.TokenLookup, ":")

	if len(selectors) != 2 {
		panic("csrf: Token lookup must in the form of <source>:<key>")
	}

	// By default we extract from a header
	extractor := csrfFromHeader(selectors[1])

	switch selectors[0] {
	case "form":
		extractor = csrfFromForm(selectors[1])
	case "query":
		extractor = csrfFromQuery(selectors[1])
	case "param":
		extractor = csrfFromParam(selectors[1])
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Declare empty token and try to get previous generated CSRF from cookie
		token, key := "", c.Cookies(cfg.Cookie.Name)

		// Check if the cookie had a CSRF token
		if key == "" {
			// Create a new CSRF token
			token = utils.UUID()
		} else {
			// Use the server generated token previously to compare
			// To the extracted token later on
			token = key
		}

		// Verify CSRF token on POST requests
		if c.Method() == fiber.MethodPost {
			// Extract token from client request i.e. header, query, param or form
			csrf, err := extractor(c)
			if err != nil {
				// We have a problem extracting the csrf token
				return fiber.ErrForbidden
			}
			// Some magic to compare both cookie and client csrf token
			if subtle.ConstantTimeCompare(utils.UnsafeBytes(token), utils.UnsafeBytes(csrf)) != 1 {
				// Comparison failed, return forbidden
				return fiber.ErrForbidden
			}
		}

		// Create new cookie to send new CSRF token
		cookie := &fiber.Cookie{
			Name:     cfg.Cookie.Name,
			Value:    token,
			Domain:   cfg.Cookie.Domain,
			Path:     cfg.Cookie.Path,
			Expires:  time.Now().Add(cfg.CookieExpires),
			Secure:   cfg.Cookie.Secure,
			HTTPOnly: cfg.Cookie.HTTPOnly,
			SameSite: cfg.Cookie.SameSite,
		}

		// Set cookie to response
		c.Cookie(cookie)

		// Store token in context
		c.Locals(cfg.ContextKey, token)

		// Protect clients from caching the response by telling the browser
		// a new header value is generated
		c.Vary(fiber.HeaderCookie)

		// Continue stack
		return c.Next()
	}
}

// csrfFromHeader returns a function that extracts token from the request header.
func csrfFromHeader(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Get(param)
		if token == "" {
			return "", errors.New("missing csrf token in header")
		}
		return token, nil
	}
}

// csrfFromQuery returns a function that extracts token from the query string.
func csrfFromQuery(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Query(param)
		if token == "" {
			return "", errors.New("missing csrf token in query string")
		}
		return token, nil
	}
}

// csrfFromParam returns a function that extracts token from the url param string.
func csrfFromParam(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Params(param)
		if token == "" {
			return "", errors.New("missing csrf token in url parameter")
		}
		return token, nil
	}
}

// csrfFromParam returns a function that extracts token from the url param string.
func csrfFromForm(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.FormValue(param)
		if token == "" {
			return "", errors.New("missing csrf token in form parameter")
		}
		return token, nil
	}
}
