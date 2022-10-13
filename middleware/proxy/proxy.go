package proxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"sync"

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
	var lbc = &fasthttp.LBClient{}
	// Note that Servers, Timeout, WriteBufferSize, ReadBufferSize and TlsConfig
	// will not be used if the client are set.
	if config.Client == nil {
		// Set timeout
		lbc.Timeout = cfg.Timeout
		// Scheme must be provided, falls back to http
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

				ReadBufferSize:  config.ReadBufferSize,
				WriteBufferSize: config.WriteBufferSize,

				TLSConfig: config.TlsConfig,
			}

			lbc.Clients = append(lbc.Clients, client)
		}
	} else {
		// Set custom client
		lbc = config.Client
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

var client = &fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
}

var lock sync.RWMutex

// WithTlsConfig update http client with a user specified tls.config
// This function should be called before Do and Forward.
// Deprecated: use WithClient instead.
func WithTlsConfig(tlsConfig *tls.Config) {
	client.TLSConfig = tlsConfig
}

// WithClient sets the global proxy client.
// This function should be called before Do and Forward.
func WithClient(cli *fasthttp.Client) {
	lock.Lock()
	defer lock.Unlock()
	client = cli
}

// Forward performs the given http request and fills the given http response.
// This method will return an fiber.Handler
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return Do(c, addr, clients...)
	}
}

// Do performs the given http request and fills the given http response.
// This method can be used within a fiber.Handler
func Do(c *fiber.Ctx, addr string, clients ...*fasthttp.Client) error {
	var cli *fasthttp.Client
	if len(clients) != 0 {
		// Set local client
		cli = clients[0]
	} else {
		// Set global client
		lock.RLock()
		cli = client
		lock.RUnlock()
	}
	req := c.Request()
	res := c.Response()
	originalURL := utils.CopyString(c.OriginalURL())
	defer req.SetRequestURI(originalURL)

	copiedURL := utils.CopyString(addr)
	req.SetRequestURI(copiedURL)
	// NOTE: if req.isTLS is true, SetRequestURI keeps the scheme as https.
	// issue reference:
	// https://github.com/gofiber/fiber/issues/1762
	if scheme := getScheme(utils.UnsafeBytes(copiedURL)); len(scheme) > 0 {
		req.URI().SetSchemeBytes(scheme)
	}

	req.Header.Del(fiber.HeaderConnection)
	if err := cli.Do(req, res); err != nil {
		return err
	}
	res.Header.Del(fiber.HeaderConnection)
	return nil
}

func getScheme(uri []byte) []byte {
	i := bytes.IndexByte(uri, '/')
	if i < 1 || uri[i-1] != ':' || i == len(uri)-1 || uri[i+1] != '/' {
		return nil
	}
	return uri[:i-1]
}
