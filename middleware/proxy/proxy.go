package proxy

import (
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dial in a round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Hosts string

	// Before allows you to alter the request
	Before fiber.Handler

	// After allows you to alter the response
	After fiber.Handler
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next: nil,
}

// New creates a new middleware handler
func New(config Config) fiber.Handler {
	// Override config if provided
	cfg := config

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}
	if cfg.Hosts == "" {
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	// Create host client
	// https://godoc.org/github.com/valyala/fasthttp#HostClient
	hostClient := fasthttp.HostClient{
		Addr:                     cfg.Hosts,
		NoDefaultUserAgentHeader: true,
	}

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set request and response
		req := c.Request()
		res := c.Response()

		// Don't proxy "Connection" header
		req.Header.Del(fiber.HeaderConnection)

		// Modify request
		if cfg.Before != nil {
			if err = cfg.Before(c); err != nil {
				return err
			}
		}

		// Forward request
		if err = hostClient.Do(req, res); err != nil {
			return err
		}

		// Don't proxy "Connection" header
		res.Header.Del(fiber.HeaderConnection)

		// Modify response
		if cfg.After != nil {
			if err = cfg.After(c); err != nil {
				return err
			}
		}

		// Return nil to end proxying if no error
		return nil
	}
}
