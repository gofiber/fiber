package proxy

import (
	"fmt"
	"net/url"
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

	// Load balanced client
	var lbc fasthttp.LBClient

	// Scheme must be provided, falls back to http
	// TODO add https support
	for _, server := range cfg.Servers {
		if !strings.HasPrefix(server, "http") {
			server = "http://" + server
		}

		u, err := url.Parse(server)
		if err != nil {
			panic(err)
		}

		client := &fasthttp.HostClient{
			NoDefaultUserAgentHeader: true,
			DisablePathNormalizing:   true,
			Addr:                     u.Host,
		}

		lbc.Clients = append(lbc.Clients, client)
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
		if cfg.ModifyRequest != nil {
			if err = cfg.ModifyRequest(c); err != nil {
				return err
			}
		}

		req.SetRequestURI(utils.UnsafeString(req.RequestURI()))

		// Forward request
		if err = lbc.Do(req, res); err != nil {
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
