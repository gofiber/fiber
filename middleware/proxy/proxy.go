package proxy

import (
	"errors"
	"net/url"
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
			if u.Scheme == schemeHTTPS {
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
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
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
	return doActionWithPolicy(c, addr, policy, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, u *url.URL) error {
		return followRedirects(cli, req, resp, maxRedirectsCount, u, policy)
	}, clients...)
}

// DoDeadline performs the given request and waits for response until the given deadline.
// This method can be used within a fiber.Handler
func DoDeadline(c fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
		return cli.DoDeadline(req, resp, deadline)
	}, clients...)
}

// DoTimeout performs the given request and waits for response during the given timeout duration.
// This method can be used within a fiber.Handler
func DoTimeout(c fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
		return cli.DoTimeout(req, resp, timeout)
	}, clients...)
}

// doActionFunc is the per-method callback driven by doActionWithPolicy.
// Receivers are handed the validated *url.URL alongside the request so
// they can avoid re-parsing or re-validating the upstream addr.
type doActionFunc func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, u *url.URL) error

func doAction(
	c fiber.Ctx,
	addr string,
	action doActionFunc,
	clients ...*fasthttp.Client,
) error {
	return doActionWithPolicy(c, addr, currentSecurityPolicy(), action, clients...)
}

func doActionWithPolicy(
	c fiber.Ctx,
	addr string,
	policy SecurityPolicy,
	action doActionFunc,
	clients ...*fasthttp.Client,
) error {
	globalClient := client.Load()

	cli, err := selectClient(globalClient, clients...)
	if err != nil {
		return err
	}

	// Clone addr once at the parse boundary instead of cloning u.Scheme
	// and u.String() individually after the fact. url.Parse fills u's
	// field slices (Scheme/Host/Path/...) with substrings of the input,
	// so if addr is itself a slice of the request buffer (e.g. a caller
	// derived it from c.OriginalURL()), every field in u aliases the
	// same backing array — and SetRequestURI below will overwrite it
	// mid-request. Decoupling u from the request buffer with one clone
	// here covers SetRequestURI/SetSchemeBytes and also the action
	// callback in followRedirects, which inspects u.Host after the
	// SetRequestURI write.
	addrCopy := strings.Clone(addr)
	u, err := validateUpstream(addrCopy, policy)
	if err != nil {
		return err
	}

	req := c.Request()
	res := c.Response()
	originalURL := utils.CopyString(c.OriginalURL())
	defer req.SetRequestURI(originalURL)

	req.SetRequestURI(u.String())
	req.URI().SetSchemeBytes([]byte(u.Scheme))

	if !policy.KeepHopByHopHeaders {
		stripHopByHopRequestHeaders(req)
	}
	if err := action(cli, req, res, u); err != nil {
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
//
// initialURL is the validated *url.URL produced by doActionWithPolicy's
// validateUpstream call. Passing it in lets the loop skip a redundant
// re-validation (one url.Parse + one DNS lookup) on entry while still
// guaranteeing every SetRequestURI is fed a string from a
// validateUpstream output — the property CodeQL's go/request-forgery
// query needs to see.
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, initialURL *url.URL, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	currentURL := initialURL
	currentHost := currentURL.Host
	for redirects := 0; ; redirects++ {
		req.SetRequestURI(currentURL.String())
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
		nextURL, err := resolveRedirect(currentURL.String(), location, policy)
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
		if !utils.EqualFold(nextURL.Host, currentHost) {
			stripCrossHostHeaders(req)
			currentHost = nextURL.Host
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

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location. The returned value
// is the validated *url.URL produced by validateUpstream, so callers
// pass it straight into network sinks without re-parsing user-controlled
// strings.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy) (*url.URL, error) {
	for _, b := range location {
		if b < 0x20 || b == 0x7f {
			return nil, fasthttp.ErrorInvalidURI
		}
	}
	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)
	uri.Update(currentURL)
	// previousScheme is at most "https" (5 bytes). A stack-sized scratch
	// buffer holds it without escaping to the heap, eliminating the
	// per-hop allocation that the previous append([]byte(nil), ...)
	// pattern produced.
	var schemeBuf [8]byte
	previousScheme := append(schemeBuf[:0], uri.Scheme()...)
	uri.UpdateBytes(location)
	if len(uri.Host()) == 0 {
		return nil, fasthttp.ErrorInvalidURI
	}
	target, err := validateUpstream(uri.String(), policy)
	if err != nil {
		return nil, err
	}
	if !policy.AllowHTTPSDowngrade && utils.EqualFold(previousScheme, httpsSchemeBytes) && target.Scheme == schemeHTTP {
		return nil, ErrRedirectDowngrade
	}
	return target, nil
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
// This method will return a fiber.Handler.
//
// The upstream addr is validated at handler construction (matches the
// Balancer contract): scheme allowlist, SSRF block, and URL parsing all
// run once. Misconfigured addresses panic at startup instead of failing
// per request. doActionWithPolicy still re-checks SSRF against the
// current policy on every hop so DNS-rebinding protection is preserved.
func DomainForward(hostname, addr string, clients ...*fasthttp.Client) fiber.Handler {
	base, err := validateUpstream(addr, currentSecurityPolicy())
	if err != nil {
		panic(err)
	}
	return func(c fiber.Ctx) error {
		// Host names are case-insensitive (RFC 9110 §4.2.3) and fasthttp
		// does not case-fold the raw Host header, so compare with
		// EqualFold — otherwise "API.Example.com" would slip past a
		// DomainForward("api.example.com", ...) gate and be passed
		// through unproxied.
		host := utils.UnsafeString(c.Request().Host())
		if !utils.EqualFold(host, hostname) {
			return nil
		}
		c.Request().Header.Set("X-Real-IP", c.IP())
		return doActionWithPolicy(c, joinUpstreamPath(base, c.OriginalURL()), currentSecurityPolicy(),
			func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
				return cli.Do(req, resp)
			}, clients...)
	}
}

// urlRoundrobin is the *url.URL-typed equivalent of the legacy
// string-based round-robin. Storing parsed URLs lets BalancerForward
// skip per-request url.Parse on its configured upstream list.
type urlRoundrobin struct {
	pool []*url.URL

	current int
	sync.Mutex
}

func (r *urlRoundrobin) get() *url.URL {
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
// This method will return a fiber.Handler.
//
// As with DomainForward, every server is parsed and policy-checked at
// handler construction. A misconfigured entry panics at startup.
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler {
	if len(servers) == 0 {
		panic("Servers cannot be empty")
	}
	policy := currentSecurityPolicy()
	bases := make([]*url.URL, len(servers))
	for i, s := range servers {
		base, err := validateUpstream(s, policy)
		if err != nil {
			panic(err)
		}
		bases[i] = base
	}
	r := &urlRoundrobin{pool: bases}
	return func(c fiber.Ctx) error {
		base := r.get()
		c.Request().Header.Set("X-Real-IP", c.IP())
		return doActionWithPolicy(c, joinUpstreamPath(base, c.OriginalURL()), currentSecurityPolicy(),
			func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
				return cli.Do(req, resp)
			}, clients...)
	}
}
