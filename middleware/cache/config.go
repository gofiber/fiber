package cache

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Expiration is the time that an cached response will live
	//
	// Optional. Default: 1 * time.Minute
	Expiration time.Duration

	// CacheControl enables client side caching if set to true
	//
	// Optional. Default: false
	CacheControl bool

	// Key allows you to generate custom keys, by default c.Path() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.Path()
	// }
	Key func(*fiber.Ctx) string

	// Deprecated, use Storage instead
	Store fiber.Storage

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Storage fiber.Storage

	// Internally used - if true, the simpler method of two maps is used in order to keep
	// execution time down.
	defaultStore bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Expiration:   1 * time.Minute,
	CacheControl: false,
	Key: func(c *fiber.Ctx) string {
		return c.Path()
	},
	defaultStore: true,
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
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if int(cfg.Expiration.Seconds()) == 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.Key == nil {
		cfg.Key = ConfigDefault.Key
	}
	if cfg.Storage == nil && cfg.Store == nil {
		cfg.defaultStore = true
	}
	if cfg.Store != nil {
		fmt.Println("cache: `Store` is deprecated, use `Storage` instead")
		cfg.Storage = cfg.Store
		cfg.defaultStore = true
	}
	return cfg
}
