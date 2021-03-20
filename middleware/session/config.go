package session

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Allowed session duration
	// Optional. Default value 24 * time.Hour
	Expiration time.Duration

	// Storage interface to store the session data
	// Optional. Default value memory.New()
	Storage fiber.Storage

	// Name of the session cookie. This cookie will store session key.
	// Optional. Default value "session_id".
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

	// KeyGenerator generates the session key.
	// Optional. Default value utils.UUIDv4
	KeyGenerator func() string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Expiration:   24 * time.Hour,
	CookieName:   "session_id",
	KeyGenerator: utils.UUIDv4,
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
	if int(cfg.Expiration.Seconds()) <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.CookieName == "" {
		cfg.CookieName = ConfigDefault.CookieName
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}
	return cfg
}
