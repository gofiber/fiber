package session

import (
	"strings"
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

	// Next defines a function to skip this middleware when it returns true.
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Store defines the session store.
	//
	// Required.
	Store *Store

	// ErrorHandler defines a function to handle errors.
	//
	// Optional. Default: nil
	ErrorHandler func(*fiber.Ctx, error)

	// KeyGenerator generates the session key.
	//
	// Optional. Default: utils.UUIDv4
	KeyGenerator func() string

	// KeyLookup is a string in the format "<source>:<name>" used to extract the session ID from the request.
	//
	// Possible values: "header:<name>", "query:<name>", "cookie:<name>"
	//
	// Optional. Default: "cookie:session_id"
	KeyLookup string

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

	// Source defines where to obtain the session ID.
	source Source

	// sessionName is the name of the session.
	sessionName string

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

// Source represents the type of session ID source.
type Source string

const (
	SourceCookie   Source = "cookie"
	SourceHeader   Source = "header"
	SourceURLQuery Source = "query"
)

// ConfigDefault provides the default configuration.
var ConfigDefault = Config{
	IdleTimeout:  30 * time.Minute,
	KeyLookup:    "cookie:session_id",
	KeyGenerator: utils.UUIDv4,
	source:       SourceCookie,
	sessionName:  "session_id",
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
func DefaultErrorHandler(c *fiber.Ctx, err error) {
	log.Errorf("session: %v", err)
	if c != nil {
		if sendErr := (*c).SendStatus(fiber.StatusInternalServerError); sendErr != nil {
			log.Errorf("session: %v", sendErr)
		}
	}
}

// configDefault sets default values for the Config struct.
//
// Parameters:
//   - config: Variadic parameter to override the default config.
//
// Returns:
//   - Config: The configuration with default values set.
//
// Usage:
//
//	cfg := configDefault()
//	cfg := configDefault(customConfig)
func configDefault(config ...Config) Config {
	// Return default config if none provided.
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config with provided config.
	cfg := config[0]

	// Set default values where necessary.
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = ConfigDefault.IdleTimeout
	}
	// Ensure AbsoluteTimeout is greater than or equal to IdleTimeout.
	if cfg.AbsoluteTimeout > 0 && cfg.AbsoluteTimeout < cfg.IdleTimeout {
		panic("[session] AbsoluteTimeout must be greater than or equal to IdleTimeout")
	}
	if cfg.KeyLookup == "" {
		cfg.KeyLookup = ConfigDefault.KeyLookup
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}

	// Parse KeyLookup into source and session name.
	selectors := strings.Split(cfg.KeyLookup, ":")
	const numSelectors = 2
	if len(selectors) != numSelectors {
		panic("[session] KeyLookup must be in the format '<source>:<name>'")
	}
	switch Source(selectors[0]) {
	case SourceCookie:
		cfg.source = SourceCookie
	case SourceHeader:
		cfg.source = SourceHeader
	case SourceURLQuery:
		cfg.source = SourceURLQuery
	default:
		panic("[session] unsupported source in KeyLookup")
	}
	cfg.sessionName = selectors[1]

	return cfg
}
