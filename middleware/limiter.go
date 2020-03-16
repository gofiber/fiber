package middleware

import (
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber"
)

// LimiterConfig ...
type LimiterConfig struct {
	Skip func(*fiber.Ctx) bool
	// Timeout in seconds on how long to keep records of requests in memory
	// Default: 60
	Timeout int
	// Max number of recent connections during `Timeout` seconds before sending a 429 response
	// Default: 10
	Max int
	// Message
	// default: "Too many requests, please try again later."
	Message string
	// StatusCode
	// Default: 429 Too Many Requests
	StatusCode int
	// Key allows to use a custom handler to create custom keys
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	Key func(*fiber.Ctx) string
	// Handler is called when a request hits the limit
	// Default: func(c *fiber.Ctx) {
	//   c.Status(cfg.StatusCode).SendString(cfg.Message)
	// }
	Handler func(*fiber.Ctx)
}

// LimiterConfigDefault is the defaul Limiter middleware config.
var LimiterConfigDefault = LimiterConfig{
	Skip:       nil,
	Timeout:    60,
	Max:        10,
	Message:    "Too many requests, please try again later.",
	StatusCode: 429,
	Key: func(c *fiber.Ctx) string {
		return c.IP()
	},
}

// Limiter ...
func Limiter(config ...LimiterConfig) func(*fiber.Ctx) {
	log.Println("Warning: middleware.Limiter() is deprecated since v1.8.2, please use github.com/gofiber/limiter")
	// Init config
	var cfg LimiterConfig
	// Set config if provided
	if len(config) > 0 {
		cfg = config[0]
	}
	// Set config default values
	if cfg.Timeout == 0 {
		cfg.Timeout = LimiterConfigDefault.Timeout
	}
	if cfg.Max == 0 {
		cfg.Max = LimiterConfigDefault.Max
	}
	if cfg.Message == "" {
		cfg.Message = LimiterConfigDefault.Message
	}
	if cfg.StatusCode == 0 {
		cfg.StatusCode = LimiterConfigDefault.StatusCode
	}
	if cfg.Key == nil {
		cfg.Key = LimiterConfigDefault.Key
	}
	if cfg.Handler == nil {
		cfg.Handler = func(c *fiber.Ctx) {
			c.Status(cfg.StatusCode).SendString(cfg.Message)
		}
	}
	// Limiter settings
	var hits = map[string]int{}
	var reset = map[string]int{}
	var timestamp = int(time.Now().Unix())
	// Update timestamp every second
	go func() {
		for {
			timestamp = int(time.Now().Unix())
			time.Sleep(1 * time.Second)
		}
	}()
	// Reset hits every cfg.Timeout
	go func() {
		for {
			// For every key in reset
			for key := range reset {
				// If resetTime exist and current time is equal or bigger
				if reset[key] != 0 && timestamp >= reset[key] {
					// Reset hits and resetTime
					hits[key] = 0
					reset[key] = 0
				}
			}
			// Wait cfg.Timeout
			time.Sleep(time.Duration(cfg.Timeout) * time.Second)
		}
	}()
	return func(c *fiber.Ctx) {
		// Skip middleware if Skip returns true
		if cfg.Skip != nil && cfg.Skip(c) {
			c.Next()
			return
		}
		// Get key (default is the remote IP)
		key := cfg.Key(c)
		// Increment key hits
		hits[key]++
		// Set unix timestamp if not exist
		if reset[key] == 0 {
			reset[key] = timestamp + cfg.Timeout
		}
		// Get current hits
		hitCount := hits[key]
		// Set how many hits we have left
		remaining := cfg.Max - hitCount
		// Calculate when it resets in seconds
		resetTime := reset[key] - timestamp
		// Check if hits exceed the cfg.Max
		if remaining < 1 {
			// Call Handler func
			cfg.Handler(c)
			// Return response with Retry-After header
			// https://tools.ietf.org/html/rfc6584
			c.Set("Retry-After", strconv.Itoa(resetTime))
			return
		}
		// We can continue, update RateLimit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Reset", strconv.Itoa(resetTime))
		// Bye!
		c.Next()
	}
}
