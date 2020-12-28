package limiter

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

	// Max number of recent connections during `Duration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// KeyGenerator allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	KeyGenerator func(*fiber.Ctx) string

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c *fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Storage fiber.Storage

	// DEPRECATED: Use Expiration instead
	Duration time.Duration

	// DEPRECATED, use Storage instead
	Store fiber.Storage

	// DEPRECATED, use KeyGenerator instead
	Key func(*fiber.Ctx) string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
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
	if int(cfg.Duration.Seconds()) > 0 {
		fmt.Println("[LIMITER] Duration is deprecated, please use Expiration")
		cfg.Expiration = cfg.Duration
	}
	if cfg.Key != nil {
		fmt.Println("[LIMITER] Key is deprecated, please us KeyGenerator")
		cfg.KeyGenerator = cfg.Key
	}
	if cfg.Store != nil {
		fmt.Println("[LIMITER] Store is deprecated, please use Storage")
		cfg.Storage = cfg.Store
	}
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Max <= 0 {
		cfg.Max = ConfigDefault.Max
	}
	if int(cfg.Expiration.Seconds()) <= 0 {
		cfg.Expiration = ConfigDefault.Expiration
	}
	if cfg.KeyGenerator == nil {
		cfg.KeyGenerator = ConfigDefault.KeyGenerator
	}
	if cfg.LimitReached == nil {
		cfg.LimitReached = ConfigDefault.LimitReached
	}
	return cfg
}
