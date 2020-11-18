package proxy

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// New is deprecated
func New(config Config) fiber.Handler {
	fmt.Println("proxy.New is deprecated, please use proxy.Balancer instead")
	return Balancer(config)
}

// Balancer creates a load balancer among multiple upstream servers
func Balancer(config Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config)

	client := fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	}

	// Scheme must be provided, falls back to http
	for i := 0; i < len(cfg.Servers); i++ {
		if !strings.HasPrefix(cfg.Servers[i], "http") {
			cfg.Servers[i] = "http://" + cfg.Servers[i]
		}
	}

	var counter = 0

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
		if cfg.ModifyRequest != nil {
			if err = cfg.ModifyRequest(c); err != nil {
				return err
			}
		}

		req.SetRequestURI(cfg.Servers[counter] + utils.UnsafeString(req.RequestURI()))
		counter = (counter + 1) % len(cfg.Servers)

		// Forward request
		if err = client.Do(req, res); err != nil {
			return err
		}

		// Don't proxy "Connection" header
		res.Header.Del(fiber.HeaderConnection)

		// Modify response
		if cfg.ModifyResponse != nil {
			if err = cfg.ModifyResponse(c); err != nil {
				return err
			}
		}

		// Return nil to end proxying if no error
		return nil
	}
}

var client = fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
}

// Forward performs the given http request and fills the given http response.
// This method will return an fiber.Handler
func Forward(addr string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return Do(c, addr)
	}
}

// Do performs the given http request and fills the given http response.
// This method can be used within a fiber.Handler
func Do(c *fiber.Ctx, addr string) error {
	req := c.Request()
	res := c.Response()
	req.SetRequestURI(addr)
	req.Header.Del(fiber.HeaderConnection)
	if err := client.Do(req, res); err != nil {
		return err
	}
	res.Header.Del(fiber.HeaderConnection)
	return nil
}
