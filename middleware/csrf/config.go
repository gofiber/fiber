package csrf

import (
	"fmt"
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

	// KeyLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default: "header:X-CSRF-Token"
	KeyLookup string

	// Name of the session cookie. This cookie will store session key.
	// Optional. Default value "csrf_".
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

	// Indicates if CSRF cookie is HTTP only.
	// Optional. Default value "Strict".
	CookieSameSite string

	// Expiration is the duration before csrf token will expire
	//
	// Optional. Default: 1 * time.Hour
	Expiration time.Duration

	// Store is used to store the state of the middleware
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage

	// Context key to store generated CSRF token into context.
	// If left empty, token will not be stored in context.
	//
	// Optional. Default: ""
	ContextKey string

	// KeyGenerator creates a new CSRF token
	//
	// Optional. Default: utils.UUID
	KeyGenerator func() string

	// Deprecated, please use Expiration
	CookieExpires time.Duration

	// Deprecated, please use Cookie* related fields
	Cookie *fiber.Cookie

	// Deprecated, please use KeyLookup
	TokenLookup string

	// ErrorHandler is executed when an error is returned from fiber.Handler.
	//
	// Optional. Default: DefaultErrorHandler
	ErrorHandler fiber.ErrorHandler
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	KeyLookup:      "header:X-Csrf-Token",
	CookieName:     "csrf_",
	CookieSameSite: "Strict",
	Expiration:     1 * time.Hour,
	KeyGenerator:   utils.UUID,
	ErrorHandler:   defaultErrorHandler,
}

// default ErrorHandler that process return error from fiber.Handler
var defaultErrorHandler = func(c *fiber.Ctx, err error) error {
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
	if cfg.TokenLookup != "" {
		fmt.Println("[CSRF] TokenLookup is deprecated, please use KeyLookup")
		cfg.KeyLookup = cfg.TokenLookup
	}
	if int(cfg.CookieExpires.Seconds()) > 0 {
		fmt.Println("[CSRF] CookieExpires is deprecated, please use Expiration")
		cfg.Expiration = cfg.CookieExpires
	}
	if cfg.Cookie != nil {
		fmt.Println("[CSRF] Cookie is deprecated, please use Cookie* related fields")
		if cfg.Cookie.Name != "" {
			cfg.CookieName = cfg.Cookie.Name
		}
		if cfg.Cookie.Domain != "" {
			cfg.CookieDomain = cfg.Cookie.Domain
		}
		if cfg.Cookie.Path != "" {
			cfg.CookiePath = cfg.Cookie.Path
		}
		cfg.CookieSecure = cfg.Cookie.Secure
		cfg.CookieHTTPOnly = cfg.Cookie.HTTPOnly
		if cfg.Cookie.SameSite != "" {
			cfg.CookieSameSite = cfg.Cookie.SameSite
		}
	}
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

	return cfg
}
