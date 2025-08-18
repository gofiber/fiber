package csrf

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/utils/v2"
)

// Config defines the config for CSRF middleware.
type Config struct {
	// Storage is used to store the state of the middleware.
	//
	// Optional. Default: memory.New()
	// Ignored if Session is set.
	Storage fiber.Storage

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Session is used to store the state of the middleware.
	//
	// Optional. Default: nil
	// If set, the middleware will use the session store instead of the storage.
	Session *session.Store

	// KeyGenerator creates a new CSRF token.
	//
	// Optional. Default: utils.UUIDv4
	KeyGenerator func() string

	// ErrorHandler is executed when an error is returned from fiber.Handler.
	//
	// Optional. Default: defaultErrorHandler
	ErrorHandler fiber.ErrorHandler

	// CookieName is the name of the CSRF cookie.
	//
	// Optional. Default: "csrf_"
	CookieName string

	// CookieDomain is the domain of the CSRF cookie.
	//
	// Optional. Default: ""
	CookieDomain string

	// CookiePath is the path of the CSRF cookie.
	//
	// Optional. Default: ""
	CookiePath string

	// CookieSameSite is the SameSite attribute of the CSRF cookie.
	//
	// Optional. Default: "Lax"
	CookieSameSite string

	// TrustedOrigins is a list of trusted origins for unsafe requests.
	// For requests that use the Origin header, the origin must match the
	// Host header or one of the TrustedOrigins.
	// For secure requests that do not include the Origin header, the Referer
	// header must match the Host header or one of the TrustedOrigins.
	//
	// This supports matching subdomains at any level. This means you can use a value like
	// "https://*.example.com" to allow any subdomain of example.com to submit requests,
	// including multiple subdomain levels such as "https://sub.sub.example.com".
	//
	// Optional. Default: []
	TrustedOrigins []string

	// Extractor returns the CSRF token from the request.
	//
	// Optional. Default: FromHeader("X-Csrf-Token")
	//
	// Available extractors:
	//   - FromHeader: Most secure, recommended for APIs
	//   - FromForm: Secure, recommended for form submissions
	//   - FromQuery: Less secure, URLs may be logged
	//   - FromParam: Less secure, URLs may be logged
	//   - Chain: Advanced chaining of multiple extractors
	//
	// WARNING: Never create custom extractors that read from cookies with the same
	// CookieName as this defeats CSRF protection entirely.
	Extractor Extractor

	// IdleTimeout is the duration of time the CSRF token is valid.
	//
	// Optional. Default: 30 * time.Minute
	IdleTimeout time.Duration

	// CookieSecure indicates if CSRF cookie is secure.
	//
	// Optional. Default: false
	CookieSecure bool

	// CookieHTTPOnly indicates if CSRF cookie is HTTP only.
	//
	// Optional. Default: false
	CookieHTTPOnly bool

	// CookieSessionOnly decides whether cookie should last for only the browser session.
	// Ignores Expiration if set to true.
	//
	// Optional. Default: false
	CookieSessionOnly bool

	// SingleUseToken indicates if the CSRF token should be destroyed
	// and a new one generated on each use.
	//
	// Optional. Default: false
	SingleUseToken bool
}

// HeaderName is the default header name for CSRF tokens.
const HeaderName = "X-Csrf-Token"

// ConfigDefault is the default config for CSRF middleware.
var ConfigDefault = Config{
	CookieName:     "csrf_",
	CookieSameSite: "Lax",
	IdleTimeout:    30 * time.Minute,
	KeyGenerator:   utils.UUIDv4,
	ErrorHandler:   defaultErrorHandler,
	Extractor:      FromHeader(HeaderName),
}

// defaultErrorHandler is the default error handler that processes errors from fiber.Handler.
func defaultErrorHandler(_ fiber.Ctx, _ error) error {
	return fiber.ErrForbidden
}

// configDefault is a helper function to set default values.
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = ConfigDefault.IdleTimeout
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
	// Check if Extractor is zero value (since it's a struct)
	if cfg.Extractor.Extract == nil {
		cfg.Extractor = ConfigDefault.Extractor
	}
	// Validate extractor security configurations
	validateExtractorSecurity(cfg)

	return cfg
}

// validateExtractorSecurity checks for insecure extractor configurations
func validateExtractorSecurity(cfg Config) {
	// Check primary extractor
	if isInsecureCookieExtractor(cfg.Extractor, cfg.CookieName) {
		panic("CSRF: Extractor reads from the same cookie '" + cfg.CookieName +
			"' used for token storage. This completely defeats CSRF protection.")
	}

	// Check chained extractors
	for i, extractor := range cfg.Extractor.Chain {
		if isInsecureCookieExtractor(extractor, cfg.CookieName) {
			panic(fmt.Sprintf("CSRF: Chained extractor #%d reads from the same cookie '%s' "+
				"used for token storage. This completely defeats CSRF protection.", i+1, cfg.CookieName))
		}
	}

	// Additional security warnings (non-fatal)
	if cfg.Extractor.Source == SourceQuery || cfg.Extractor.Source == SourceParam {
		log.Warnf("[CSRF WARNING] Using %v extractor - URLs may be logged", cfg.Extractor.Source)
	}
}

// isInsecureCookieExtractor checks if an extractor unsafely reads from the CSRF cookie
func isInsecureCookieExtractor(extractor Extractor, cookieName string) bool {
	if extractor.Source == SourceCookie {
		// Exact match - definitely insecure
		if extractor.Key == cookieName {
			return true
		}

		// Case-insensitive match - potentially confusing, warn but don't panic
		if strings.EqualFold(extractor.Key, cookieName) && extractor.Key != cookieName {
			log.Warnf("[CSRF WARNING] Extractor cookie name '%s' is similar to CSRF cookie '%s' - this may be confusing",
				extractor.Key, cookieName)
		}
	}

	return false
}
