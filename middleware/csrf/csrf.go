package csrf

import (
	"context"
	"errors"
	"fmt"
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
	// - "cookie:<name>"
	TokenLookup string

	// Cookie
	//
	// Optional.
	Cookie *fiber.Cookie

	// Deprecated, please use Expiration
	CookieExpires time.Duration

	// Expiration is the duration before csrf token will expire
	//
	// Optional. Default: 24 * time.Hour
	Expiration time.Duration

	// Context key to store generated CSRF token into context.
	//
	// Optional. Default value "csrf".
	ContextKey string

	// CSRF Storage
	//
	// Optional. Default value MemoryStorage
	Store fiber.Storage

	// Key defines a function to generate the unique identifier.
	//
	// Optional. Default: utils.UUID
	Key func() string
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
	Key:           utils.UUID,
	Expiration:    24 * time.Hour,
	CookieExpires: 24 * time.Hour, // deprecated
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
		if cfg.CookieExpires != 0 {
			fmt.Println("[CSRF] CookieExpires is deprecated, please use Expiration")
			cfg.CookieExpires = ConfigDefault.Expiration
		}
		if cfg.Expiration == 0 {
			cfg.Expiration = ConfigDefault.Expiration
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

		if cfg.Key == nil {
			cfg.Key = utils.UUID
		}
	}

	if cfg.Store == nil {
		cfg.Store = newMemoryStorage(context.Background())
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
	case "cookie":
		if selectors[1] == cfg.Cookie.Name {
			panic(fmt.Sprintf("TokenLookup key %s can't be the same as Cookie.Name %s", selectors[1], cfg.Cookie.Name))
		}
		extractor = csrfFromCookie(selectors[1])
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
			token = cfg.Key()
			if err := cfg.Store.Set(token, nil, cfg.Expiration); err != nil {
				return fiber.ErrInternalServerError
			}
		} else {
			// Use the server generated token previously to compare
			// To the extracted token later on
			token = key
		}

		// Verify CSRF token
		if shouldCheckToken(c.Method()) {
			// Extract token from client request i.e. header, query, param or form
			csrf, err := extractor(c)
			if err != nil {
				// We have a problem extracting the csrf token
				return fiber.ErrForbidden
			}

			// Check if token exist or expired
			if _, err := cfg.Store.Get(csrf); err != nil {
				return fiber.ErrForbidden
			}

			// CSRF token should be only used once
			if err := cfg.Store.Delete(csrf); err != nil {
				return fiber.ErrInternalServerError
			}

			token = cfg.Key()
			if err := cfg.Store.Set(token, nil, cfg.Expiration); err != nil {
				return fiber.ErrInternalServerError
			}
		}

		// Create new cookie to send new CSRF token
		cookie := &fiber.Cookie{
			Name:     cfg.Cookie.Name,
			Value:    token,
			Domain:   cfg.Cookie.Domain,
			Path:     cfg.Cookie.Path,
			Expires:  time.Now().Add(cfg.Expiration),
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

// csrfFromParam returns a function that extracts a token from a multipart-form.
func csrfFromForm(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.FormValue(param)
		if token == "" {
			return "", errors.New("missing csrf token in form parameter")
		}
		return token, nil
	}
}

// csrfFromCookie returns a function that extracts token from the cookie header.
func csrfFromCookie(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Cookies(param)
		if token == "" {
			return "", errors.New("missing csrf token in cookie")
		}
		return token, nil
	}
}

func shouldCheckToken(httpMethod string) bool {
	return httpMethod == fiber.MethodPost ||
		httpMethod == fiber.MethodPut ||
		httpMethod == fiber.MethodPatch ||
		httpMethod == fiber.MethodDelete
}
