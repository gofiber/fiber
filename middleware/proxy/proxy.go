package proxy

import (
	"bytes"
	"crypto/tls"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

// New is deprecated
func New(config Config) fiber.Handler {
	log.Warn("[PROXY] proxy.New is deprecated, please use proxy.Balancer instead")
	return Balancer(config)
}

// Balancer creates a load balancer among multiple upstream servers
func Balancer(config Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config)

	// Load balanced client
	lbc := &fasthttp.LBClient{}
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
	return func(c *fiber.Ctx) error {
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
			if err := cfg.ModifyRequest(c); err != nil {
				return err
			}
		}

		req.SetRequestURI(utils.UnsafeString(req.RequestURI()))

		// Forward request
		if err := lbc.Do(req, res); err != nil {
			return err
		}

		// Don't proxy "Connection" header
		res.Header.Del(fiber.HeaderConnection)

		// Modify response
		if cfg.ModifyResponse != nil {
			if err := cfg.ModifyResponse(c); err != nil {
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
//
//nolint:stylecheck,revive // TODO: Rename to "WithTLSConfig" in v3
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
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.Do(req, resp)
	}, clients...)
}

// DoRedirects performs the given http request and fills the given http response, following up to maxRedirectsCount redirects.
// When the redirect count exceeds maxRedirectsCount, ErrTooManyRedirects is returned.
// This method can be used within a fiber.Handler
func DoRedirects(c *fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.DoRedirects(req, resp, maxRedirectsCount)
	}, clients...)
}

// DoDeadline performs the given request and waits for response until the given deadline.
// This method can be used within a fiber.Handler
func DoDeadline(c *fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.DoDeadline(req, resp, deadline)
	}, clients...)
}

// DoTimeout performs the given request and waits for response during the given timeout duration.
// This method can be used within a fiber.Handler
func DoTimeout(c *fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.DoTimeout(req, resp, timeout)
	}, clients...)
}

func doAction(
	c *fiber.Ctx,
	addr string,
	action func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error,
	clients ...*fasthttp.Client,
) error {
	var cli *fasthttp.Client

	// set local or global client
	if len(clients) != 0 {
		cli = clients[0]
	} else {
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
	// Reference: https://github.com/gofiber/fiber/issues/1762
	if scheme := getScheme(utils.UnsafeBytes(copiedURL)); len(scheme) > 0 {
		req.URI().SetSchemeBytes(scheme)
	}

	req.Header.Del(fiber.HeaderConnection)
	if err := action(cli, req, res); err != nil {
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

// DomainForward performs an http request based on the given domain and populates the given http response.
// This method will return an fiber.Handler
func DomainForward(hostname, addr string, clients ...*fasthttp.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		host := string(c.Request().Host())
		if host == hostname {
			return Do(c, addr+c.OriginalURL(), clients...)
		}
		return nil
	}
}

type roundrobin struct {
	sync.Mutex

	current int
	pool    []string
}

// this method will return a string of addr server from list server.
func (r *roundrobin) get() string {
	r.Lock()
	defer r.Unlock()

	if r.current >= len(r.pool) {
		r.current %= len(r.pool)
	}

	result := r.pool[r.current]
	r.current++
	return result
}

// BalancerForward Forward performs the given http request with round robin algorithm to server and fills the given http response.
// This method will return an fiber.Handler
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler {
	r := &roundrobin{
		current: 0,
		pool:    servers,
	}
	return func(c *fiber.Ctx) error {
		server := r.get()
		if !strings.HasPrefix(server, "http") {
			server = "http://" + server
		}
		c.Request().Header.Add("X-Real-IP", c.IP())
		return Do(c, server+c.OriginalURL(), clients...)
	}
}
