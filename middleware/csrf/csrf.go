package csrf

import (
	"errors"
	"fmt"
	"net/textproto"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Set default values
	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}

	// Generate the correct extractor to get the token from the correct location
	selectors := strings.Split(cfg.KeyLookup, ":")

	if len(selectors) != 2 {
		panic("[CSRF] KeyLookup must in the form of <source>:<key>")
	}

	// By default we extract from a header
	extractor := csrfFromHeader(textproto.CanonicalMIMEHeaderKey(selectors[1]))

	switch selectors[0] {
	case "form":
		extractor = csrfFromForm(selectors[1])
	case "query":
		extractor = csrfFromQuery(selectors[1])
	case "param":
		extractor = csrfFromParam(selectors[1])
	case "cookie":
		if selectors[1] == cfg.Cookie.Name {
			panic(fmt.Sprintf("KeyLookup key %s can't be the same as Cookie.Name %s", selectors[1], cfg.Cookie.Name))
		}
		extractor = csrfFromCookie(selectors[1])
	}

	// We only use Keys in Storage, so we need a dummy value
	dummyVal := []byte{'+'}

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		var token string

		// Action depends on the HTTP method
		switch c.Method() {
		case fiber.MethodGet:
			// Generate CSRF token if not exist

			// Declare empty token and try to get existing CSRF from cookie
			token = c.Cookies(cfg.Cookie.Name)

			// Do we have an existing CSRF token?
			if token == "" {
				// Generate new CSRF token
				token = cfg.KeyGenerator()

				// Add token to Storage
				if err = cfg.Storage.Set(token, dummyVal, cfg.Expiration); err != nil {
					fmt.Println("[CSRF]", err.Error())
				}
			}

			// Create cookie to pass token to client
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
		case fiber.MethodPost, fiber.MethodDelete, fiber.MethodPatch, fiber.MethodPut:
			// Verify CSRF token
			// Extract token from client request i.e. header, query, param, form or cookie
			token, err = extractor(c)
			if err != nil {
				return fiber.ErrForbidden
			}
			// We have a problem extracting the csrf token from Storage
			if _, err = cfg.Storage.Get(token); err != nil {
				// The token is invalid, let client generate a new one
				if err = cfg.Storage.Delete(token); err != nil {
					fmt.Println("[CSRF]", err.Error())
				}
				// Expire cookie
				c.Cookie(&fiber.Cookie{
					Name:     cfg.Cookie.Name,
					Domain:   cfg.Cookie.Domain,
					Path:     cfg.Cookie.Path,
					Expires:  time.Now().Add(-1 * time.Minute),
					Secure:   cfg.Cookie.Secure,
					HTTPOnly: cfg.Cookie.HTTPOnly,
					SameSite: cfg.Cookie.SameSite,
				})
				return fiber.ErrForbidden
			}
		}

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

var (
	errMissingHeader = errors.New("missing csrf token in header")
	errMissingQuery  = errors.New("missing csrf token in query")
	errMissingParam  = errors.New("missing csrf token in param")
	errMissingForm   = errors.New("missing csrf token in form")
	errMissingCookie = errors.New("missing csrf token in cookie")
)

// csrfFromHeader returns a function that extracts token from the request header.
func csrfFromHeader(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Get(param)
		if token == "" {
			return "", errMissingHeader
		}
		return token, nil
	}
}

// csrfFromQuery returns a function that extracts token from the query string.
func csrfFromQuery(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Query(param)
		if token == "" {
			return "", errMissingQuery
		}
		return token, nil
	}
}

// csrfFromParam returns a function that extracts token from the url param string.
func csrfFromParam(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Params(param)
		if token == "" {
			return "", errMissingParam
		}
		return token, nil
	}
}

// csrfFromForm returns a function that extracts a token from a multipart-form.
func csrfFromForm(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.FormValue(param)
		if token == "" {
			return "", errMissingForm
		}
		return token, nil
	}
}

// csrfFromCookie returns a function that extracts token from the cookie header.
func csrfFromCookie(param string) func(c *fiber.Ctx) (string, error) {
	return func(c *fiber.Ctx) (string, error) {
		token := c.Cookies(param)
		if token == "" {
			return "", errMissingCookie
		}
		return token, nil
	}
}
