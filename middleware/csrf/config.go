package csrf

import (
	"net/textproto"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/utils/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// KeyLookup is a string in the form of "<source>:<key>" that is used
	// to create an Extractor that extracts the token from the request.
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Ignored if an Extractor is explicitly set.
	//
	// Optional. Default: "header:X-Csrf-Token"
	KeyLookup string

	// Name of the session cookie. This cookie will store session key.
	// Optional. Default value "csrf_".
	// Overridden if KeyLookup == "cookie:<name>"
	CookieName string

	// Domain of the CSRF cookie.
	// Optional. Default value "".
	CookieDomain string

	// Path of the CSRF cookie.
	// Optional. Default value "".
	CookiePath string

	// Indicates if CSRF cookie is secure.
	// Optional. Default value false.
	CookieSecure bool

	// Indicates if CSRF cookie is HTTP only.
	// Optional. Default value false.
	CookieHTTPOnly bool

	// Value of SameSite cookie.
	// Optional. Default value "Lax".
	CookieSameSite string

	// Decides whether cookie should last for only the browser sesison.
	// Ignores Expiration if set to true
	CookieSessionOnly bool

	// Expiration is the duration before csrf token will expire
	//
	// Optional. Default: 1 * time.Hour
	Expiration time.Duration

	// SingleUseToken indicates if the CSRF token be destroyed
	// and a new one generated on each use.
	//
	// Optional. Default: false
	SingleUseToken bool

	// Store is used to store the state of the middleware
	//
	// Optional. Default: memory.New()
	// Ignored if Session is set.
	Storage fiber.Storage

	// Session is used to store the state of the middleware
	//
	// Optional. Default: nil
	// If set, the middleware will use the session store instead of the storage
	Session *session.Store

	// SessionKey is the key used to store the token in the session
	//
	// Default: "csrfToken"
	SessionKey string

	// TrustedOrigins is a list of trusted origins for unsafe requests.
	// For requests that use the Origin header, the origin must match the
	// Host header or one of the TrustedOrigins.
	// For secure requests, that do not include the Origin header, the Referer
	// header must match the Host header or one of the TrustedOrigins.
	//
	// This supports subdomain matching, so you can use a value like "https://.example.com"
	// to allow any subdomain of example.com to submit requests.
	//
	//
	// Optional. Default: []
	TrustedOrigins []string

	// KeyGenerator creates a new CSRF token
	//
	// Optional. Default: utils.UUID
	KeyGenerator func() string

	// ErrorHandler is executed when an error is returned from fiber.Handler.
	//
	// Optional. Default: DefaultErrorHandler
	ErrorHandler fiber.ErrorHandler

	// Extractor returns the csrf token
	//
	// If set this will be used in place of an Extractor based on KeyLookup.
	//
	// Optional. Default will create an Extractor based on KeyLookup.
	Extractor func(c fiber.Ctx) (string, error)
}

const HeaderName = "X-Csrf-Token"

// ConfigDefault is the default config
var ConfigDefault = Config{
	KeyLookup:      "header:" + HeaderName,
	CookieName:     "csrf_",
	CookieSameSite: "Lax",
	Expiration:     1 * time.Hour,
	KeyGenerator:   utils.UUIDv4,
	ErrorHandler:   defaultErrorHandler,
	Extractor:      FromHeader(HeaderName),
	SessionKey:     "csrfToken",
}

// default ErrorHandler that process return error from fiber.Handler
func defaultErrorHandler(_ fiber.Ctx, _ error) error {
	return fiber.ErrForbidden
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.KeyLookup == "" {
		cfg.KeyLookup = ConfigDefault.KeyLookup
	}
	if int(cfg.Expiration.Seconds()) <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.CookieName == "" {
		cfg.CookieName = ConfigDefault.CookieName
	}
	if cfg.CookieSameSite == "" {
		cfg.CookieSameSite = ConfigDefault.CookieSameSite
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = ConfigDefault.ErrorHandler
	}
	if cfg.SessionKey == "" {
		cfg.SessionKey = ConfigDefault.SessionKey
	}

	// Generate the correct extractor to get the token from the correct location
	selectors := strings.Split(cfg.KeyLookup, ":")

	const numParts = 2
	if len(selectors) != numParts {
		panic("[CSRF] KeyLookup must in the form of <source>:<key>")
	}

	if cfg.Extractor == nil {
		// By default we extract from a header
		cfg.Extractor = FromHeader(textproto.CanonicalMIMEHeaderKey(selectors[1]))

		switch selectors[0] {
		case "form":
			cfg.Extractor = FromForm(selectors[1])
		case "query":
			cfg.Extractor = FromQuery(selectors[1])
		case "param":
			cfg.Extractor = FromParam(selectors[1])
		case "cookie":
			if cfg.Session == nil {
				log.Warn("[CSRF] Cookie extractor is not recommended without a session store")
			}
			if cfg.CookieSameSite == "None" || cfg.CookieSameSite != "Lax" && cfg.CookieSameSite != "Strict" {
				log.Warn("[CSRF] Cookie extractor is only recommended for use with SameSite=Lax or SameSite=Strict")
			}
			cfg.Extractor = FromCookie(selectors[1])
			cfg.CookieName = selectors[1] // Cookie name is the same as the key
		}
	}

	return cfg
}
