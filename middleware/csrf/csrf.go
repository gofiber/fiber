package csrf

import (
	"errors"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	ErrTokenNotFound   = errors.New("csrf token not found")
	ErrTokenInvalid    = errors.New("csrf token invalid")
	ErrRefererNotFound = errors.New("referer not supplied")
	ErrRefererInvalid  = errors.New("referer invalid")
	ErrRefererNoMatch  = errors.New("referer does not match host and is not a trusted origin")
	ErrOriginInvalid   = errors.New("origin invalid")
	ErrOriginNoMatch   = errors.New("origin does not match host and is not a trusted origin")
	ErrNotGetStorage   = errors.New("csrf storage not found data")
	ErrNotSetStorage   = errors.New("csrf storage not set data")

	errOriginNotFound = errors.New("origin not supplied or is null") // internal error, will not be returned to the user
	dummyValue        = []byte{'+'}
)

// Handler for CSRF middleware
type Handler struct {
	config         Config
	sessionManager *sessionManager
	storageManager *storageManager
}

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// The keys for the values in context
const (
	tokenKey contextKey = iota
	handlerKey
)

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

	// Pre-parse trusted origins
	trustedOrigins := []string{}
	trustedSubOrigins := []subdomain{}

	for _, origin := range cfg.TrustedOrigins {
		if i := strings.Index(origin, "://*."); i != -1 {
			trimmedOrigin := strings.TrimSpace(origin[:i+3] + origin[i+4:])
			isValid, normalizedOrigin := normalizeOrigin(trimmedOrigin)
			if !isValid {
				panic("[CSRF] Invalid origin format in configuration:" + origin)
			}
			sd := subdomain{prefix: normalizedOrigin[:i+3], suffix: normalizedOrigin[i+3:]}
			trustedSubOrigins = append(trustedSubOrigins, sd)
		} else {
			trimmedOrigin := strings.TrimSpace(origin)
			isValid, normalizedOrigin := normalizeOrigin(trimmedOrigin)
			if !isValid {
				panic("[CSRF] Invalid origin format in configuration:" + origin)
			}
			trustedOrigins = append(trustedOrigins, normalizedOrigin)
		}
	}

	// Create the handler outside of the returned function
	handler := &Handler{
		config:         cfg,
		sessionManager: sessionManager,
		storageManager: storageManager,
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Store the CSRF handler in the context
		c.Locals(handlerKey, handler)

		var token string

		// Action depends on the HTTP method
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
			cookieToken := c.Cookies(cfg.CookieName)
			if cookieToken != "" {
				// In this case, handling error doesn't make sense because we have validations after the switch.
				raw, err := getRawFromStorage(c, cookieToken, cfg, sessionManager, storageManager)
				if raw != nil && err == nil {
					token = cookieToken // Token is valid, safe to set it
				}
			}
		default:
			// Assume that anything not defined as 'safe' by RFC7231 needs protection

			// Enforce an origin check for unsafe requests.
			err := originMatchesHost(c, trustedOrigins, trustedSubOrigins)

			// If there's no origin, enforce a referer check for HTTPS connections.
			if errors.Is(err, errOriginNotFound) {
				if c.Scheme() == "https" {
					err = refererMatchesHost(c, trustedOrigins, trustedSubOrigins)
				} else {
					// If it's not HTTPS, clear the error to allow the request to proceed.
					err = nil
				}
			}

			// If there's an error (either from origin check or referer check), handle it.
			if err != nil {
				return cfg.ErrorHandler(c, err)
			}

			// Extract token from client request i.e. header, query, param, form or cookie
			extractedToken, err := cfg.Extractor(c)
			if err != nil {
				return cfg.ErrorHandler(c, err)
			}

			if extractedToken == "" {
				return cfg.ErrorHandler(c, ErrTokenNotFound)
			}

			// If not using FromCookie extractor, check that the token matches the cookie
			// This is to prevent CSRF attacks by using a Double Submit Cookie method
			// Useful when we do not have access to the users Session
			if !isFromCookie(cfg.Extractor) && !compareStrings(extractedToken, c.Cookies(cfg.CookieName)) {
				return cfg.ErrorHandler(c, ErrTokenInvalid)
			}

			raw, err := getRawFromStorage(c, extractedToken, cfg, sessionManager, storageManager)
			if err != nil || raw == nil {
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

		// Store the token in the context
		c.Locals(tokenKey, token)

		// Continue stack
		return c.Next()
	}
}

// TokenFromContext returns the token found in the context
// returns an empty string if the token does not exist
func TokenFromContext(c fiber.Ctx) string {
	token, ok := c.Locals(tokenKey).(string)
	if !ok {
		return ""
	}
	return token
}

// HandlerFromContext returns the Handler found in the context
// returns nil if the handler does not exist
func HandlerFromContext(c fiber.Ctx) *Handler {
	handler, ok := c.Locals(handlerKey).(*Handler)
	if !ok {
		return nil
	}
	return handler
}

// getRawFromStorage returns the raw value from the storage for the given token
// returns nil if the token does not exist, is expired or is invalid
func getRawFromStorage(c fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) ([]byte, error) {
	if cfg.Session != nil {
		return sessionManager.getRaw(c, token, dummyValue)
	}
	return storageManager.getRaw(token)
}

// createOrExtendTokenInStorage creates or extends the token in the storage
func createOrExtendTokenInStorage(c fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) {
	if cfg.Session != nil {
		sessionManager.setRaw(c, token, dummyValue, cfg.Expiration)
	} else {
		storageManager.setRaw(token, dummyValue, cfg.Expiration)
	}
}

func deleteTokenFromStorage(c fiber.Ctx, token string, cfg Config, sessionManager *sessionManager, storageManager *storageManager) {
	if cfg.Session != nil {
		sessionManager.delRaw(c)
	} else {
		storageManager.delRaw(token)
	}
}

// Update CSRF cookie
// if expireCookie is true, the cookie will expire immediately
func updateCSRFCookie(c fiber.Ctx, cfg Config, token string) {
	setCSRFCookie(c, cfg, token, cfg.Expiration)
}

func expireCSRFCookie(c fiber.Ctx, cfg Config) {
	setCSRFCookie(c, cfg, "", -time.Hour)
}

func setCSRFCookie(c fiber.Ctx, cfg Config, token string, expiry time.Duration) {
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
func (handler *Handler) DeleteToken(c fiber.Ctx) error {
	// Extract token from the client request cookie
	cookieToken := c.Cookies(handler.config.CookieName)
	if cookieToken == "" {
		return handler.config.ErrorHandler(c, ErrTokenNotFound)
	}
	// Remove the token from storage
	deleteTokenFromStorage(c, cookieToken, handler.config, handler.sessionManager, handler.storageManager)
	// Expire the cookie
	expireCSRFCookie(c, handler.config)
	return nil
}

// isFromCookie checks if the extractor is set to ExtractFromCookie
func isFromCookie(extractor any) bool {
	return reflect.ValueOf(extractor).Pointer() == reflect.ValueOf(FromCookie).Pointer()
}

// originMatchesHost checks that the origin header matches the host header
// returns an error if the origin header is not present or is invalid
// returns nil if the origin header is valid
func originMatchesHost(c fiber.Ctx, trustedOrigins []string, trustedSubOrigins []subdomain) error {
	origin := strings.ToLower(c.Get(fiber.HeaderOrigin))
	if origin == "" || origin == "null" { // "null" is set by some browsers when the origin is a secure context https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Origin#description
		return errOriginNotFound
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return ErrOriginInvalid
	}

	if originURL.Scheme == c.Scheme() && originURL.Host == c.Host() {
		return nil
	}

	for _, trustedOrigin := range trustedOrigins {
		if origin == trustedOrigin {
			return nil
		}
	}

	for _, trustedSubOrigin := range trustedSubOrigins {
		if trustedSubOrigin.match(origin) {
			return nil
		}
	}

	return ErrOriginNoMatch
}

// refererMatchesHost checks that the referer header matches the host header
// returns an error if the referer header is not present or is invalid
// returns nil if the referer header is valid
func refererMatchesHost(c fiber.Ctx, trustedOrigins []string, trustedSubOrigins []subdomain) error {
	referer := strings.ToLower(c.Get(fiber.HeaderReferer))
	if referer == "" {
		return ErrRefererNotFound
	}

	refererURL, err := url.Parse(referer)
	if err != nil {
		return ErrRefererInvalid
	}

	if refererURL.Scheme == c.Scheme() && refererURL.Host == c.Host() {
		return nil
	}

	referer = refererURL.String()

	for _, trustedOrigin := range trustedOrigins {
		if referer == trustedOrigin {
			return nil
		}
	}

	for _, trustedSubOrigin := range trustedSubOrigins {
		if trustedSubOrigin.match(referer) {
			return nil
		}
	}

	return ErrRefererNoMatch
}
