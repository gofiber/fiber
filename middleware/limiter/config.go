package limiter

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

const defaultLimiterMax = 5

// Config defines the config for middleware.
type Config struct {
	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Storage fiber.Storage

	// LimiterMiddleware is the struct that implements a limiter middleware.
	//
	// Default: a new Fixed Window Rate Limiter
	LimiterMiddleware Handler
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// A function to dynamically calculate the max requests supported by the rate limiter middleware
	//
	// Default: func(c fiber.Ctx) int {
	//   return c.Max
	// }
	MaxFunc func(c fiber.Ctx) int

	// KeyGenerator allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c fiber.Ctx) string {
	//   return c.IP()
	// }
	KeyGenerator func(fiber.Ctx) string

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler

	// Max number of recent connections during `Expiration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration

	// When set to true, requests with StatusCode >= 400 won't be counted.
	//
	// Default: false
	SkipFailedRequests bool

	// When set to true, requests with StatusCode < 400 won't be counted.
	//
	// Default: false
	SkipSuccessfulRequests bool

	// When set to true, the middleware will not include the rate limit headers (X-RateLimit-* and Retry-After) in the response.
	//
	// Default: false
	DisableHeaders bool

	// DisableValueRedaction turns off masking limiter keys in logs and error messages when set to true.
	//
	// Default: false
	DisableValueRedaction bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Max:        defaultLimiterMax,
	Expiration: 1 * time.Minute,
	MaxFunc: func(_ fiber.Ctx) int {
		return defaultLimiterMax
	},
	KeyGenerator: func(c fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
	SkipFailedRequests:     false,
	SkipSuccessfulRequests: false,
	DisableHeaders:         false,
	DisableValueRedaction:  false,
	LimiterMiddleware:      FixedWindow{},
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
	if cfg.LimiterMiddleware == nil {
		cfg.LimiterMiddleware = ConfigDefault.LimiterMiddleware
	}
	if cfg.MaxFunc == nil {
		cfg.MaxFunc = func(_ fiber.Ctx) int {
			return cfg.Max
		}
	}
	return cfg
}
