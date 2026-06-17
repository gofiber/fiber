package proxy

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/utils/v2"

	"github.com/gofiber/fiber/v3"

	"github.com/valyala/fasthttp"
)

// Balancer creates a load balancer among multiple upstream servers
func Balancer(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)
	policy := resolvePolicy(cfg.SecurityPolicy)

	// Load balanced client
	lbc := &fasthttp.LBClient{}
	// Note that Servers, Timeout, WriteBufferSize, ReadBufferSize and TLSConfig
	// will not be used if the client are set.
	if cfg.Client == nil {
		// Set timeout
		lbc.Timeout = cfg.Timeout
		// Validate each upstream against the configured policy and build
		// a HostClient per server.
		for _, server := range cfg.Servers {
			u, err := validateUpstreamForBalancer(server, policy)
			if err != nil {
				panic(err)
			}

			client := &fasthttp.HostClient{
				NoDefaultUserAgentHeader: true,
				DisablePathNormalizing:   true,
				Addr:                     u.Host,
				MaxConns:                 cfg.MaxConnsPerHost,

				ReadBufferSize:  cfg.ReadBufferSize,
				WriteBufferSize: cfg.WriteBufferSize,

				TLSConfig: secureTLSConfig(cfg.TLSConfig),

				DialDualStack: cfg.DialDualStack,

				MaxResponseBodySize: cfg.MaxResponseBodySize,
			}
			if u.Scheme == "https" {
				client.IsTLS = true
			}
			// When private targets are disallowed, validate the resolved IP
			// at dial time so DNS-rebinding cannot swap a public address for
			// a private one between validation and connection.
			if !policy.AllowPrivateIPs {
				client.Dial = newSSRFDialer(cfg.DialDualStack)
			}

			lbc.Clients = append(lbc.Clients, client)
		}
	} else {
		// Set custom client
		lbc = cfg.Client
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Set request and response
		req := c.Request()
		res := c.Response()

		if !policy.KeepHopByHopHeaders {
			if cfg.KeepConnectionHeader {
				stripHopByHopRequestHeaders(req, fiber.HeaderConnection)
			} else {
				stripHopByHopRequestHeaders(req)
			}
		}

		// Modify request
		if cfg.ModifyRequest != nil {
			if err := cfg.ModifyRequest(c); err != nil {
				return err
			}
		}

		if c.App().Config().Immutable {
			req.SetRequestURIBytes(req.RequestURI())
		} else {
			req.SetRequestURI(utils.UnsafeString(req.RequestURI()))
		}

		// Forward request
		if err := lbc.Do(req, res); err != nil {
			return err
		}

		if !policy.KeepHopByHopHeaders {
			if cfg.KeepConnectionHeader {
				stripHopByHopResponseHeaders(res, fiber.HeaderConnection)
			} else {
				stripHopByHopResponseHeaders(res)
			}
		}

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

var defaultClient = &fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
	MaxConnsPerHost:          defaultMaxConnsPerHost,
}

var client atomic.Pointer[fasthttp.Client]

var (
	errNilProxyClientOverride = errors.New("proxy: nil client override passed to Do/Forward")
	errNilGlobalProxyClient   = errors.New("proxy: global client is nil, set a non-nil client with proxy.WithClient")
)

func init() {
	client.Store(defaultClient)
}

// WithClient sets the global proxy client.
// This function should be called before Do and Forward.
func WithClient(cli *fasthttp.Client) {
	if cli == nil {
		panic("proxy: WithClient requires a non-nil *fasthttp.Client")
	}

	client.Store(cli)
}

// Forward performs the given http request and fills the given http response.
// This method will return a fiber.Handler
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Request().Header.Set("X-Real-IP", c.IP())
		return Do(c, addr, clients...)
	}
}

// Do performs the given http request and fills the given http response.
// This method can be used within a fiber.Handler
func Do(c fiber.Ctx, addr string, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.Do(req, resp)
	}, clients...)
}

// DoRedirects performs the given http request and fills the given http response, following up to maxRedirectsCount redirects.
// When the redirect count exceeds maxRedirectsCount, ErrTooManyRedirects is returned.
// This method can be used within a fiber.Handler
//
// Each redirect target is re-validated against the active SecurityPolicy.
// Unless AllowHTTPSDowngrade is enabled, redirects from an HTTPS origin
// to a plaintext HTTP target are rejected with ErrRedirectDowngrade.
func DoRedirects(c fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error {
	policy := currentSecurityPolicy()
	return doActionWithPolicy(c, addr, policy, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return followRedirects(cli, req, resp, maxRedirectsCount, policy)
	}, clients...)
}

// DoDeadline performs the given request and waits for response until the given deadline.
// This method can be used within a fiber.Handler
func DoDeadline(c fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.DoDeadline(req, resp, deadline)
	}, clients...)
}

// DoTimeout performs the given request and waits for response during the given timeout duration.
// This method can be used within a fiber.Handler
func DoTimeout(c fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
		return cli.DoTimeout(req, resp, timeout)
	}, clients...)
}

func doAction(
	c fiber.Ctx,
	addr string,
	action func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error,
	clients ...*fasthttp.Client,
) error {
	return doActionWithPolicy(c, addr, currentSecurityPolicy(), action, clients...)
}

func doActionWithPolicy(
	c fiber.Ctx,
	addr string,
	policy SecurityPolicy,
	action func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error,
	clients ...*fasthttp.Client,
) error {
	globalClient := client.Load()

	cli, err := selectClient(globalClient, clients...)
	if err != nil {
		return err
	}

	u, err := validateUpstream(addr, policy)
	if err != nil {
		return err
	}

	req := c.Request()
	res := c.Response()
	originalURL := utils.CopyString(c.OriginalURL())
	defer req.SetRequestURI(originalURL)

	// Snapshot scheme/target into freshly allocated buffers BEFORE any
	// SetRequestURI call. Both u.Scheme and u.String() may alias the
	// caller-supplied addr, which itself can be a slice of the request
	// buffer that SetRequestURI is about to overwrite — without these
	// copies, the scheme/host bytes get clobbered mid-request.
	scheme := strings.Clone(u.Scheme)
	targetURL := strings.Clone(u.String())
	req.SetRequestURI(targetURL)
	req.URI().SetSchemeBytes([]byte(scheme))

	if !policy.KeepHopByHopHeaders {
		stripHopByHopRequestHeaders(req)
	}
	if err := action(cli, req, res); err != nil {
		return err
	}
	if !policy.KeepHopByHopHeaders {
		stripHopByHopResponseHeaders(res)
	}
	return nil
}

// followRedirects implements a redirect loop that re-validates each
// target against policy before issuing the next request. It replaces
// fasthttp.Client.DoRedirects so we can reject HTTPS→HTTP downgrades and
// reapply SSRF checks to caller-controlled Location headers.
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	currentURL := string(req.URI().FullURI())
	currentHost := string(req.URI().Host())
	for redirects := 0; ; redirects++ {
		req.SetRequestURI(currentURL)
		if err := cli.Do(req, resp); err != nil {
			return err
		}
		status := resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(status) {
			return nil
		}
		if redirects >= maxRedirects {
			return fasthttp.ErrTooManyRedirects
		}
		location := resp.Header.Peek(fiber.HeaderLocation)
		if len(location) == 0 {
			return fasthttp.ErrMissingLocation
		}
		nextURL, err := resolveRedirect(currentURL, location, policy)
		if err != nil {
			return err
		}
		// POST→GET on 301/302/303 mirrors browser and fasthttp behavior.
		if req.Header.IsPost() && (status == fasthttp.StatusMovedPermanently || status == fasthttp.StatusFound || status == fasthttp.StatusSeeOther) {
			req.Header.SetMethod(fasthttp.MethodGet)
			req.SetBody(nil)
			req.Header.Del(fasthttp.HeaderContentType)
			req.Header.Del(fasthttp.HeaderContentLength)
		}
		// Strip credentials when the redirect crosses to a different host so
		// secrets bound to the original origin are not leaked to a third
		// party (RFC 9110 §15.4 advisory; matches browser behavior).
		if nextHost := redirectHost(nextURL); !utils.EqualFold(nextHost, currentHost) {
			stripCrossHostHeaders(req)
			currentHost = nextHost
		}
		currentURL = nextURL
	}
}

// crossHostSensitiveHeaders lists headers that carry credentials bound to
// a specific origin and must not survive a redirect to a different host.
var crossHostSensitiveHeaders = []string{
	fiber.HeaderAuthorization,
	fiber.HeaderProxyAuthorization,
	fiber.HeaderCookie,
}

func stripCrossHostHeaders(req *fasthttp.Request) {
	for _, h := range crossHostSensitiveHeaders {
		req.Header.Del(h)
	}
}

// redirectHost extracts the host (host:port) component of an absolute URL
// without allocating a full *url.URL.
func redirectHost(rawURL string) string {
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.Update(rawURL)
	return string(uri.Host())
}

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy) (string, error) {
	for _, b := range location {
		if b < 0x20 || b == 0x7f {
			return "", fasthttp.ErrorInvalidURI
		}
	}
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.Update(currentURL)
	previousScheme := append([]byte(nil), uri.Scheme()...)
	uri.UpdateBytes(location)
	if len(uri.Host()) == 0 {
		return "", fasthttp.ErrorInvalidURI
	}
	next := uri.String()
	target, err := validateUpstream(next, policy)
	if err != nil {
		return "", err
	}
	if !policy.AllowHTTPSDowngrade && bytes.EqualFold(previousScheme, []byte("https")) && target.Scheme == "http" {
		return "", ErrRedirectDowngrade
	}
	return target.String(), nil
}

func selectClient(globalClient *fasthttp.Client, clients ...*fasthttp.Client) (*fasthttp.Client, error) {
	if len(clients) != 0 {
		if clients[0] == nil {
			return nil, errNilProxyClientOverride
		}

		return clients[0], nil
	}

	if globalClient == nil {
		return nil, errNilGlobalProxyClient
	}

	return globalClient, nil
}

// DomainForward performs an http request based on the given domain and populates the given http response.
// This method will return a fiber.Handler
func DomainForward(hostname, addr string, clients ...*fasthttp.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		host := utils.UnsafeString(c.Request().Host())
		if host == hostname {
			c.Request().Header.Set("X-Real-IP", c.IP())
			policy := currentSecurityPolicy()
			base, err := validateUpstream(addr, policy)
			if err != nil {
				return err
			}
			return doActionWithPolicy(c, joinUpstreamPath(base, c.OriginalURL()), policy,
				func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
					return cli.Do(req, resp)
				}, clients...)
		}
		return nil
	}
}

type roundrobin struct {
	pool []string

	current int
	sync.Mutex
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
// This method will return a fiber.Handler
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler {
	if len(servers) == 0 {
		panic("Servers cannot be empty")
	}
	r := &roundrobin{
		current: 0,
		pool:    servers,
	}
	return func(c fiber.Ctx) error {
		server := r.get()
		c.Request().Header.Set("X-Real-IP", c.IP())
		policy := currentSecurityPolicy()
		base, err := validateUpstream(server, policy)
		if err != nil {
			return err
		}
		return doActionWithPolicy(c, joinUpstreamPath(base, c.OriginalURL()), policy,
			func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response) error {
				return cli.Do(req, resp)
			}, clients...)
	}
}
