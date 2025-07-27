package session

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

// Config defines the configuration for the session middleware.
type Config struct {
	// Storage interface for storing session data.
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage

	// Store defines the session store.
	//
	// Required.
	Store *Store

	// Next defines a function to skip this middleware when it returns true.
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ErrorHandler defines a function to handle errors.
	//
	// Optional. Default: nil
	ErrorHandler func(fiber.Ctx, error)

	// KeyGenerator generates the session key.
	//
	// Optional. Default: utils.UUIDv4
	KeyGenerator func() string

	// CookieDomain defines the domain of the session cookie.
	//
	// Optional. Default: ""
	CookieDomain string

	// CookiePath defines the path of the session cookie.
	//
	// Optional. Default: ""
	CookiePath string

	// CookieSameSite specifies the SameSite attribute of the cookie.
	//
	// Optional. Default: "Lax"
	CookieSameSite string

	// Extractor is used to extract the session ID from the request.
	//
	// Optional. Default: FromCookie("session_id")
	Extractor Extractor

	// IdleTimeout defines the maximum duration of inactivity before the session expires.
	//
	// Note: The idle timeout is updated on each `Save()` call. If a middleware handler is used, `Save()` is called automatically.
	//
	// Optional. Default: 30 * time.Minute
	IdleTimeout time.Duration

	// AbsoluteTimeout defines the maximum duration of the session before it expires.
	//
	// If set to 0, the session will not have an absolute timeout, and will expire after the idle timeout.
	//
	// Optional. Default: 0
	AbsoluteTimeout time.Duration

	// CookieSecure specifies if the session cookie should be secure.
	//
	// Optional. Default: false
	CookieSecure bool

	// CookieHTTPOnly specifies if the session cookie should be HTTP-only.
	//
	// Optional. Default: false
	CookieHTTPOnly bool

	// CookieSessionOnly determines if the cookie should expire when the browser session ends.
	//
	// If true, the cookie will be deleted when the browser is closed.
	// Note: This will not delete the session data from the store.
	//
	// Optional. Default: false
	CookieSessionOnly bool
}

// ConfigDefault provides the default configuration.
var ConfigDefault = Config{
	IdleTimeout:    30 * time.Minute,
	KeyGenerator:   utils.UUIDv4,
	Extractor:      FromCookie("session_id"),
	CookieSameSite: "Lax",
}

// DefaultErrorHandler logs the error and sends a 500 status code.
//
// Parameters:
//   - c: The Fiber context.
//   - err: The error to handle.
//
// Usage:
//
//	DefaultErrorHandler(c, err)
func DefaultErrorHandler(c fiber.Ctx, err error) {
	log.Errorf("session error: %v", err)
	if sendErr := c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error"); sendErr != nil {
		log.Errorf("failed to send error response: %v", sendErr)
	}
}

// configDefault sets default values for the Config struct.
//
// This function ensures that all necessary fields have sensible defaults
// if they are not explicitly set by the user.
//
// Parameters:
//   - config: Variadic parameter to override default config.
//
// Returns:
//   - Config: The configuration with defaults applied.
//
// Usage:
//
//	cfg := configDefault()
//	cfg := configDefault(customConfig)
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

	// Ensure AbsoluteTimeout is greater than or equal to IdleTimeout.
	if cfg.AbsoluteTimeout > 0 && cfg.AbsoluteTimeout < cfg.IdleTimeout {
		panic("[session] AbsoluteTimeout must be greater than or equal to IdleTimeout")
	}

	// Check if we have a zero-value Extractor
	if cfg.Extractor.Extract == nil {
		cfg.Extractor = ConfigDefault.Extractor
	}

	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}

	if cfg.CookieSameSite == "" {
		cfg.CookieSameSite = ConfigDefault.CookieSameSite
	}

	return cfg
}
