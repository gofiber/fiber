package session

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/storage/memory"
	"github.com/gofiber/fiber/v2/utils"
)

// Config defines the config for middleware.
type Config struct {
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default value "cookie:_csrf".
	// TODO: When to override Cookie.Value?
	KeyLookup string

	// Optional. Session ID generator function.
	//
	// Default: utils.UUID
	KeyGenerator func() string

	// Optional. Cookie to set values on
	//
	// NOTE: Value, MaxAge and Expires will be overriden by the session ID and expiration
	// TODO: Should this be a pointer, if yes why?
	Cookie fiber.Cookie

	// Allowed session duration
	//
	// Optional. Default: 24 hours
	Expiration time.Duration

	// Storage interface
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Cookie: fiber.Cookie{
		Value: "session_id",
	},
	Expiration:   30 * time.Minute,
	KeyGenerator: utils.UUID,
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
	if cfg.Storage == nil {
		cfg.Storage = memory.New()
	}
	return cfg
}
