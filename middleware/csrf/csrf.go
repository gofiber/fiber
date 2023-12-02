package csrf

import (
	"errors"
	"net/url"
	"reflect"
	"time"

	"github.com/gofiber/fiber/v2"
)

var (
	ErrTokenNotFound = errors.New("csrf token not found")
	ErrTokenInvalid  = errors.New("csrf token invalid")
	ErrNoReferer     = errors.New("referer not supplied")
	ErrBadReferer    = errors.New("referer invalid")
	dummyValue       = []byte{'+'}
)

type CSRFHandler struct {
	config         *Config
	sessionManager *sessionManager
	storageManager *storageManager
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Create manager to simplify storage operations ( see *_manager.go )
	var sessionManager *sessionManager
	var storageManager *storageManager
	if cfg.Session != nil {
		// Register the Token struct in the session store
		cfg.Session.RegisterType(Token{})

		sessionManager = newSessionManager(cfg.Session, cfg.SessionKey)
	} else {
		storageManager = newStorageManager(cfg.Storage)
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Store the CSRF handler in the context if a context key is specified
		if cfg.HandlerContextKey != "" {
			c.Locals(cfg.HandlerContextKey, &CSRFHandler{
				config:         &cfg,
				sessionManager: sessionManager,
				storageManager: storageManager,
			})
		}

		var token string

		// Action depends on the HTTP method
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
			cookieToken := c.Cookies(cfg.CookieName)

			if cookieToken != "" {
				raw := getRawFromStorage(c, cookieToken, cfg, sessionManager, storageManager)

				if raw != nil {
					token = cookieToken // Token is valid, safe to set it
				}
			}
		default:
			// Assume that anything not defined as 'safe' by RFC7231 needs protection

			// Enforce an origin check for HTTPS connections.
			if c.Protocol() == "https" {
				if err := refererMatchesHost(c); err != nil {
					return cfg.ErrorHandler(c, err)
				}
			}

			// Extract token from client request i.e. header, query, param, form or cookie
			extractedToken, err := cfg.Extractor(c)
			if err != nil {
				return cfg.ErrorHandler(c, err)
			}

			if extractedToken == "" {
				return cfg.ErrorHandler(c, ErrTokenNotFound)
			}

			// If not using CsrfFromCookie extractor, check that the token matches the cookie
			// This is to prevent CSRF attacks by using a Double Submit Cookie method
			// Useful when we do not have access to the users Session
			if !isCsrfFromCookie(cfg.Extractor) && !compareStrings(extractedToken, c.Cookies(cfg.CookieName)) {
				return cfg.ErrorHandler(c, ErrTokenInvalid)
			}

			raw := getRawFromStorage(c, extractedToken, cfg, sessionManager, storageManager)

			if raw == nil {
				// If token is not in storage, expire the cookie
				expireCSRFCookie(c, cfg)
				// and return an error
				return cfg.ErrorHandler(c, ErrTokenNotFound)
			}
			if cfg.SingleUseToken {
				// If token is single use, delete it from storage
				deleteTokenFromStorage(c, extractedToken, cfg, sessionManager, storageManager)
			} else {
				token = extractedToken // Token is valid, safe to set it
			}
		}

		// Generate CSRF token if not exist
		if token == "" {
			// And generate a new token
			token = cfg.KeyGenerator()
		}

		// Create or extend the token in the storage
		createOrExtendTokenInStorage(c, token, cfg, sessionManager, storageManager)

		// Update the CSRF cookie
		updateCSRFCookie(c, cfg, token)

		// Tell the browser that a new header value is generated
		c.Vary(fiber.HeaderCookie)

		// Store the token in the context if a context key is specified
		if cfg.ContextKey != nil {
			c.Locals(cfg.ContextKey, token)
		}

		// Continue stack
		return c.Next()
	}
}

// getRawFromStorage returns the raw value from the storage for the given token
// returns nil if the token does not exist, is expired or is invalid
func getRawFromStorage(c *fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) []byte {
	if cfg.Session != nil {
		return sessionManager.getRaw(c, token, dummyValue)
	}
	return storageManager.getRaw(token)
}

// createOrExtendTokenInStorage creates or extends the token in the storage
func createOrExtendTokenInStorage(c *fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) {
	if cfg.Session != nil {
		sessionManager.setRaw(c, token, dummyValue, cfg.Expiration)
	} else {
		storageManager.setRaw(token, dummyValue, cfg.Expiration)
	}
}

func deleteTokenFromStorage(c *fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) {
	if cfg.Session != nil {
		sessionManager.delRaw(c)
	} else {
		storageManager.delRaw(token)
	}
}

// Update CSRF cookie
// if expireCookie is true, the cookie will expire immediately
func updateCSRFCookie(c *fiber.Ctx, cfg Config, token string) {
	setCSRFCookie(c, cfg, token, cfg.Expiration)
}

func expireCSRFCookie(c *fiber.Ctx, cfg Config) {
	setCSRFCookie(c, cfg, "", -time.Hour)
}

func setCSRFCookie(c *fiber.Ctx, cfg Config, token string, expiry time.Duration) {
	cookie := &fiber.Cookie{
		Name:        cfg.CookieName,
		Value:       token,
		Domain:      cfg.CookieDomain,
		Path:        cfg.CookiePath,
		Secure:      cfg.CookieSecure,
		HTTPOnly:    cfg.CookieHTTPOnly,
		SameSite:    cfg.CookieSameSite,
		SessionOnly: cfg.CookieSessionOnly,
		Expires:     time.Now().Add(expiry),
	}

	// Set the CSRF cookie to the response
	c.Cookie(cookie)
}

// DeleteToken removes the token found in the context from the storage
// and expires the CSRF cookie
func (handler *CSRFHandler) DeleteToken(c *fiber.Ctx) error {
	// Get the config from the context
	config := handler.config
	if config == nil {
		panic("CSRFHandler config not found in context")
	}
	// Extract token from the client request cookie
	cookieToken := c.Cookies(config.CookieName)
	if cookieToken == "" {
		return config.ErrorHandler(c, ErrTokenNotFound)
	}
	// Remove the token from storage
	deleteTokenFromStorage(c, cookieToken, *config, handler.sessionManager, handler.storageManager)
	// Expire the cookie
	expireCSRFCookie(c, *config)
	return nil
}

// isCsrfFromCookie checks if the extractor is set to ExtractFromCookie
func isCsrfFromCookie(extractor interface{}) bool {
	return reflect.ValueOf(extractor).Pointer() == reflect.ValueOf(CsrfFromCookie).Pointer()
}

// refererMatchesHost checks that the referer header matches the host header
// returns an error if the referer header is not present or is invalid
// returns nil if the referer header is valid
func refererMatchesHost(c *fiber.Ctx) error {
	referer := c.Get(fiber.HeaderReferer)
	if referer == "" {
		return ErrNoReferer
	}

	refererURL, err := url.Parse(referer)
	if err != nil {
		return ErrBadReferer
	}

	if refererURL.Scheme+"://"+refererURL.Host != c.Protocol()+"://"+c.Hostname() {
		return ErrBadReferer
	}

	return nil
}
