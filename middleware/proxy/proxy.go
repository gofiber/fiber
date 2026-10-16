package proxy

import (
	"bytes"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/internal/idnafold"
	"github.com/gofiber/fiber/v3/internal/wiretarget"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/fasthttp"
)

// Balancer creates a load balancer among multiple upstream servers.
//
// Servers (or a custom Client) is required, with or without a Config: without
// one Balancer panics at startup. The upstreams are validated and their clients
// built here, once, so a bad entry stops the application when the handler is
// built and not on a request.
func Balancer(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)
	policy := resolvePolicy(cfg.SecurityPolicy)

	// Load balanced client
	lbc := &fasthttp.LBClient{}
	// Note that Servers, Timeout, MaxConnsPerHost, ReadBufferSize,
	// WriteBufferSize, MaxResponseBodySize, TLSConfig and DialDualStack
	// will not be used if the client is set.
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

			lbc.Clients = append(lbc.Clients, newHostClient(u, &cfg, policy))
		}
	} else {
		// Set custom client
		lbc = cfg.Client
	}
	// A host client the caller built normalizes the request line on the way
	// out unless told otherwise, which would decode "%2F" into a separator and
	// merge "//" and so undo the routed target forwarded below. Any other
	// BalancingClient cannot be told, so with one present a target its
	// normalization could change is refused per request.
	opaqueClient := false
	for _, bc := range lbc.Clients {
		if hc, ok := bc.(*fasthttp.HostClient); ok {
			hc.DisablePathNormalizing = true
			continue
		}
		opaqueClient = true
	}

	// The Connection header is the one hop-by-hop field the legacy
	// KeepConnectionHeader option lets through.
	var keepHeaders []string
	if cfg.KeepConnectionHeader {
		keepHeaders = []string{fiber.HeaderConnection}
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
		normalized := headerlookup.Canonical(c)

		// Read before the hop-by-hop fields go: Config.ProxyHeader may name
		// one the client listed in Connection, and the address is this hop's
		// view of the request as received.
		var scratch [realIPScratch]byte
		ip := realIP(c, scratch[:0])

		if !policy.KeepHopByHopHeaders {
			stripHopByHopRequestHeaders(req, normalized, keepHeaders...)
		}

		// The Host field holds no userinfo (RFC 9112 Section 3.2). fasthttp
		// reads "svc:pw@backend" as one when it parses the origin-form
		// target below, and writes it upstream as a Basic Authorization
		// header, over any the application had pinned.
		if bytes.IndexByte(req.Header.Host(), '@') >= 0 {
			return fiber.ErrBadRequest
		}

		// Forward the path the router matched (see wiretarget.Routed), and
		// refuse one the upstream could read as another path. A target
		// fasthttp would read as an authority (see wiretarget.ParsesAsPath),
		// set as the request line, would hand the upstream a Host, and
		// credentials, taken from the path. One holding an escaped slash, an
		// empty segment or a forged escape (see wiretarget.SegmentsAsRouted)
		// would be served by an upstream that decodes "%2F", merges "//" or
		// decodes the forged escape as a path no middleware here matched.
		// Behind a BalancingClient the proxy cannot configure, one that
		// normalization would change (see wiretarget.SurvivesNormalization)
		// is refused too.
		target := wiretarget.Routed(c)
		if !wiretarget.ParsesAsPath(target, req.Header.Host()) || !segmentsAsRouted(target, policy) ||
			(opaqueClient && !wiretarget.SurvivesNormalization(target)) {
			return fiber.ErrBadRequest
		}
		originalURL := utils.CopyString(c.OriginalURL())
		defer req.SetRequestURI(originalURL)
		req.SetRequestURI(target)

		// After the Connection listing has been applied, so the line is this
		// hop's and not one the client named there; before ModifyRequest, so
		// the application still has the last word.
		setRealIP(c, ip)

		// Modify request
		if cfg.ModifyRequest != nil {
			if err := cfg.ModifyRequest(c); err != nil {
				return err
			}
		}

		// The upstream connection speaks HTTP/1.1: reset a protocol token
		// inherited from the inbound request (e.g. "HTTP/2" propagated by
		// the net/http adaptor) so the serialized request line stays valid.
		req.Header.SetProtocol("HTTP/1.1")

		// Forward request
		if err := lbc.Do(req, res); err != nil {
			return err
		}

		if !policy.KeepHopByHopHeaders {
			stripHopByHopResponseHeaders(res, keepHeaders...)
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

// newHostClient builds the client Balancer dials one validated upstream with.
func newHostClient(u *url.URL, cfg *Config, policy SecurityPolicy) *fasthttp.HostClient {
	hc := &fasthttp.HostClient{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		Addr:                     u.Host,
		IsTLS:                    u.Scheme == schemeHTTPS,
		MaxConns:                 cfg.MaxConnsPerHost,

		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,

		TLSConfig: secureTLSConfig(cfg.TLSConfig),

		DialDualStack: cfg.DialDualStack,

		MaxResponseBodySize: cfg.MaxResponseBodySize,
	}
	// When private targets are disallowed, validate the resolved IP at dial
	// time so DNS-rebinding cannot swap a public address for a private one
	// between validation and connection.
	if !policy.AllowPrivateIPs {
		hc.Dial = newDialGuard(cfg.DialDualStack).strictDialer()
	}
	return hc
}

// defaultClient is what Do, Forward and the other runtime helpers dispatch
// through until WithClient replaces it. It leaves fasthttp's zero values for
// the read and write timeouts and for the response size cap, so a slow or
// oversized upstream is not cut short: bound them with a client registered
// through WithClient or passed per call, or use DoTimeout and DoDeadline for
// one request.
var defaultClient = &fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
	MaxConnsPerHost:          defaultMaxConnsPerHost,
}

// sharedClient is the client the runtime helpers use when none is passed per
// call: defaultClient, or the one WithClient registered last.
var sharedClient atomic.Pointer[fasthttp.Client]

var (
	errNilProxyClientOverride = errors.New("proxy: nil client override passed to Do/Forward")
	errNilGlobalProxyClient   = errors.New("proxy: global client is nil, set a non-nil client with proxy.WithClient")
)

// segmentsAsRouted reports whether a forwarding handler may hand target on
// under policy: wiretarget.SegmentsAsRouted, with escaped slashes and empty
// segments let through when policy.AllowAmbiguousSlashes is set.
func segmentsAsRouted(target string, policy SecurityPolicy) bool {
	if policy.AllowAmbiguousSlashes {
		return wiretarget.SegmentsAsRoutedSlashesAside(target)
	}
	return wiretarget.SegmentsAsRouted(target)
}

func init() {
	ensureClientGuarded(defaultClient)
	sharedClient.Store(defaultClient)
}

// WithClient sets the global proxy client.
// This function should be called before Do and Forward — doing so installs
// the dial-time SSRF guard (via the client's ConfigureClient hook,
// composing with any hook it already carries) before the client dials any
// host, so requests dispatched through it re-validate the resolved IP at
// connect time, matching the default client's behavior.
func WithClient(cli *fasthttp.Client) {
	if cli == nil {
		panic("proxy: WithClient requires a non-nil *fasthttp.Client")
	}

	ensureClientGuarded(cli)
	sharedClient.Store(cli)
}

// forwardWithRealIP is what Forward, DomainForward and BalancerForward share:
// the request goes to addr under policy, carrying this hop's X-Real-IP. The
// address is read from the request as received and written once the client's
// Connection listing has been applied to it, which is the order RFC 9110
// §7.6.1 gives an intermediary: the received message loses the fields it
// lists, then this hop adds its own. The policy is the caller's snapshot, so
// the target it checked and the dispatch are judged by the same one.
func forwardWithRealIP(c fiber.Ctx, addr string, policy SecurityPolicy, clients ...*fasthttp.Client) error {
	var scratch [realIPScratch]byte
	ip := realIP(c, scratch[:0])
	return doActionWithPolicy(c, addr, policy, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
		setRealIP(c, ip)
		return cli.Do(req, resp)
	}, clients...)
}

// Forward performs the given http request and fills the given http response.
// This method will return a fiber.Handler
//
// SSRF note: Forward validates the upstream host against the active
// SecurityPolicy up front and, when AllowPrivateIPs is false, re-validates
// the resolved IP at dial time via the guard installed on the dispatching
// client, so a rebinding-capable resolver cannot swap a public answer for
// a private one between validation and connection. The active policy is read
// on every request, so keep the one installed with WithSecurityPolicy in place
// while the handler serves.
//
// Limits: the default client applies no timeout and no cap on the size of a
// response. Pass a client with ReadTimeout, WriteTimeout and
// MaxResponseBodySize set, or register one with WithClient, when an upstream
// cannot be trusted to answer promptly and in small bodies.
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		return forwardWithRealIP(c, addr, currentSecurityPolicy(), clients...)
	}
}

// Do performs the given http request and fills the given http response.
// This method can be used within a fiber.Handler
//
// SSRF note: the upstream host is validated against the active
// SecurityPolicy before the request is sent, and when AllowPrivateIPs is
// false the resolved IP is re-validated at dial time by the guard
// installed on the dispatching *fasthttp.Client. The connection therefore
// targets exactly the address that passed the blocklist, so a
// rebinding-capable resolver cannot slip a private IP past the check.
//
// The guard is installed via the client's ConfigureClient hook, which only
// affects HostClients created after installation. The default client and
// WithClient clients are guarded before first use; a client supplied as a
// per-call variadic override is guarded on first use, so a host it had
// already dialed keeps a cached, pre-guard HostClient. For a full guarantee
// with a custom client, register it via WithClient (or pass a dedicated,
// unused client) before first use.
//
// The request reaches the upstream as HTTP/1.1, whatever protocol the inbound
// request used. As with Forward, the default client sets no timeout and no cap
// on the size of a response: use DoTimeout or DoDeadline to bound one request.
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
//
// SSRF note: every hop's host is validated against the policy, and when
// AllowPrivateIPs is false each dial is re-validated by the guard on the
// dispatching client — so per-hop redirects (including cross-host ones)
// cannot be rebound to a private IP between validation and connection.
func DoRedirects(c fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error {
	policy := currentSecurityPolicy()
	return doActionWithPolicy(c, addr, policy, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, u *url.URL) error {
		return followRedirects(cli, req, resp, maxRedirectsCount, u, policy)
	}, clients...)
}

// DoDeadline performs the given request and waits for response until the given deadline.
// This method can be used within a fiber.Handler
//
// SSRF note: same protection as Do — up-front host validation plus the
// dial-time validated-IP guard when AllowPrivateIPs is false.
func DoDeadline(c fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error {
	return doAction(c, addr, func(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, _ *url.URL) error {
		return cli.DoDeadline(req, resp, deadline)
	}, clients...)
}

// DoTimeout performs the given request and waits for response during the given timeout duration.
// This method can be used within a fiber.Handler
//
// SSRF note: same protection as Do — up-front host validation plus the
// dial-time validated-IP guard when AllowPrivateIPs is false.
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
	globalClient := sharedClient.Load()

	cli, err := selectClient(globalClient, clients...)
	if err != nil {
		return err
	}

	// The global client (default or WithClient) is guarded at registration,
	// so only a per-call variadic override needs guarding here. Skipping the
	// global client keeps the common request path free of the guard lock.
	// The guard re-validates the resolved IP at dial time; the
	// validateUpstream call below is only the up-front host check.
	if len(clients) != 0 {
		ensureClientGuarded(cli)
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
	addrCopy := utils.CopyString(addr)
	u, err := validateUpstream(addrCopy, policy)
	if err != nil {
		return err
	}

	req := c.Request()
	res := c.Response()
	normalized := headerlookup.Canonical(c)
	originalURL := utils.CopyString(c.OriginalURL())
	// fasthttp rewrote the Host header to the upstream's; put the client's back for later middleware.
	originalHost := utils.CopyBytes(req.Header.Host())
	defer func() {
		req.SetRequestURI(originalURL)
		req.Header.SetHostBytes(originalHost)
	}()

	req.SetRequestURI(u.String())
	// SetScheme takes the string directly (fasthttp appends its bytes),
	// avoiding the per-request []byte(u.Scheme) allocation. Re-applying
	// the scheme is required because SetRequestURI keeps the previous
	// scheme when req.isTLS is set — see
	// https://github.com/gofiber/fiber/issues/1762.
	req.URI().SetScheme(u.Scheme)

	if !policy.KeepHopByHopHeaders {
		stripHopByHopRequestHeaders(req, normalized)
	}
	// The upstream connection speaks HTTP/1.1: reset a protocol token
	// inherited from the inbound request (e.g. "HTTP/2" propagated by the
	// net/http adaptor) so the serialized request line stays valid.
	req.Header.SetProtocol("HTTP/1.1")
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
//
// Each hop is rewritten by redirectRequest for the status that sent it there.
// A redirect that leaves the current host or port also drops the credentials
// bound to the original origin (see stripCrossHostHeaders).
func followRedirects(cli *fasthttp.Client, req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, initialURL *url.URL, policy SecurityPolicy) error {
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	currentURL := initialURL
	currentHost := currentURL.Host
	for redirects := 0; ; redirects++ {
		// Rendered once per hop: SetRequestURI copies the bytes and
		// resolveRedirect below needs the same string, so sharing it saves
		// a URL.String() allocation on every redirect.
		currentURLStr := currentURL.String()
		req.SetRequestURI(currentURLStr)
		// Re-apply the scheme each hop: SetRequestURI keeps the previous
		// scheme when req.isTLS is set, so a redirect that changes scheme
		// (HTTP→HTTPS upgrade, or HTTPS→HTTP when AllowHTTPSDowngrade is
		// enabled) would otherwise be dispatched with the wrong scheme.
		// See https://github.com/gofiber/fiber/issues/1762.
		req.URI().SetScheme(currentURL.Scheme)
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
		nextURL, err := resolveRedirect(currentURLStr, location, policy)
		if err != nil {
			return err
		}
		redirectRequest(req, status)
		// Strip credentials when the redirect crosses to a different origin —
		// another host or another port of the same host; the scheme is not
		// compared — so secrets bound to the original origin are not leaked to
		// a third party (RFC 9110 §15.4 advisory; matches browser behavior).
		// URL.Host carries the port, which is what makes the port count.
		if !utils.EqualFold(nextURL.Host, currentHost) {
			stripCrossHostHeaders(req)
			currentHost = nextURL.Host
		}
		currentURL = nextURL
	}
}

// redirectRequest turns req into the request the next hop of a redirect
// carries, given the status of the response that sent it there. RFC 9110
// §15.4.4 has a 303 (See Other) turn every method but GET and HEAD into GET,
// and §15.4.2 and §15.4.3 let a 301 or 302 turn POST into GET, as browsers and
// fasthttp do. A 307 or 308 repeats the request as it was. When the method
// changes the body goes with it, along with the fields that frame it.
func redirectRequest(req *fasthttp.Request, status int) {
	switch status {
	case fasthttp.StatusSeeOther:
		if req.Header.IsGet() || req.Header.IsHead() {
			return
		}
	case fasthttp.StatusMovedPermanently, fasthttp.StatusFound:
		if !req.Header.IsPost() {
			return
		}
	default:
		return
	}
	req.Header.SetMethod(fasthttp.MethodGet)
	req.ResetBody()
	req.Header.Del(fasthttp.HeaderContentType)
	req.Header.Del(fasthttp.HeaderContentLength)
	req.Header.Del(fasthttp.HeaderTransferEncoding)
	req.Header.Del(fasthttp.HeaderTrailer)
}

// resolveRedirect parses a redirect target relative to the current URL
// and applies the SecurityPolicy. CRLF and other control bytes are
// rejected to prevent header injection via Location. The returned value
// is the validated *url.URL produced by validateUpstream, so callers
// pass it straight into network sinks without re-parsing user-controlled
// strings.
func resolveRedirect(currentURL string, location []byte, policy SecurityPolicy) (*url.URL, error) {
	// A control byte must not appear in a Location value: anything < 0x20
	// (including HTAB) or DEL. Bytes >= 0x80 never match; they are handled by
	// URI parsing, not header-injection checks. That is exactly the set
	// utils.IndexControl reports.
	if utils.IndexControl(location) != -1 {
		return nil, fasthttp.ErrorInvalidURI
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

// foldHostnameLabel returns hostname with its host label folded to lowercase
// Punycode, preserving an explicit port unchanged. It is the construction-time
// counterpart to hostWithoutPort: that strips a port from the per-request wire
// value, this normalizes the configured value once so the two sides compare
// correctly regardless of how the operator spelled the domain.
func foldHostnameLabel(hostname string) string {
	if host, port, ok := utils.SplitHostPort(hostname); ok {
		return net.JoinHostPort(idnafold.ToASCII(utilsstrings.ToLower(host)), port)
	}
	return idnafold.ToASCII(utilsstrings.ToLower(hostname))
}

// hostWithoutPort strips the port from a Host header value, keeping a bracketed IPv6 literal.
func hostWithoutPort(host string) string {
	if strings.HasPrefix(host, "[") {
		if end := strings.IndexByte(host, ']'); end != -1 {
			return host[:end+1]
		}
		return host
	}
	if i := strings.LastIndexByte(host, ':'); i != -1 && strings.IndexByte(host[:i], ':') == -1 {
		return host[:i]
	}
	return host
}

// DomainForward performs an http request based on the given domain and populates the given http response.
// This method will return a fiber.Handler.
//
// The upstream addr is validated at handler construction: scheme allowlist,
// SSRF block, and URL parsing all run once. Unlike Balancer (which defers
// DNS to dial time to survive transient resolver failures at startup), this
// construction check resolves the host, so a misconfigured or unresolvable
// addr panics at startup instead of failing per request. Each request also
// re-validates the host against the current policy before dispatch.
//
// SSRF note: the per-request validation is an up-front host check, and
// when AllowPrivateIPs is false the dispatching client's dial-time guard
// re-validates the resolved IP at connect time — so a rebinding-capable
// resolver cannot reach a private address through this handler.
func DomainForward(hostname, addr string, clients ...*fasthttp.Client) fiber.Handler {
	base, err := validateUpstream(addr, currentSecurityPolicy())
	if err != nil {
		panic(err)
	}
	// A Unicode hostname literal — entirely natural to write for a non-ASCII
	// domain — never appears on the wire: a conforming client always sends
	// the Punycode form (RFC 5891), so folding once here, rather than on
	// every request, is what makes the two sides comparable at all.
	hostname = foldHostnameLabel(hostname)
	return func(c fiber.Ctx) error {
		// Host names are case-insensitive (RFC 9110 §4.2.3) and fasthttp
		// does not case-fold the raw Host header, so compare with
		// EqualFold — otherwise "API.Example.com" would slip past a
		// DomainForward("api.example.com", ...) gate and be passed
		// through unproxied.
		// Match the host with or without its port; another host is passed on to the next handler.
		host := utils.UnsafeString(c.Request().Host())
		if !utils.EqualFold(host, hostname) && !utils.EqualFold(hostWithoutPort(host), hostname) {
			return c.Next()
		}
		return forwardRouted(c, base, clients...)
	}
}

// forwardRouted sends the request on to base, carrying the path the router
// matched, for DomainForward and BalancerForward. A routed path an upstream
// could read as another path (see wiretarget.SegmentsAsRouted) is refused with
// 400 Bad Request, unless the active policy lets escaped slashes and empty
// segments through (SecurityPolicy.AllowAmbiguousSlashes).
func forwardRouted(c fiber.Ctx, base *url.URL, clients ...*fasthttp.Client) error {
	target := wiretarget.Routed(c)
	active := currentSecurityPolicy()
	if !segmentsAsRouted(target, active) {
		return fiber.ErrBadRequest
	}
	return forwardWithRealIP(c, joinUpstreamPath(base, target), active, clients...)
}

// urlRoundrobin is the *url.URL-typed equivalent of the legacy
// string-based round-robin. Storing parsed URLs lets BalancerForward
// skip per-request url.Parse on its configured upstream list.
type urlRoundrobin struct {
	pool []*url.URL

	next atomic.Uint64
}

// Compare-and-swap rather than a bare Add, so the counter holds the next index
// already reduced into the pool and the sequence stays a strict round-robin
// across a wrap. The modulo also reduces a counter this loop did not produce.
func (r *urlRoundrobin) get() *url.URL {
	poolSize := uint64(len(r.pool))
	for {
		next := r.next.Load()
		index := next % poolSize
		following := index + 1
		if following == poolSize {
			following = 0
		}
		if r.next.CompareAndSwap(next, following) {
			return r.pool[index]
		}
	}
}

// BalancerForward Forward performs the given http request with round robin algorithm to server and fills the given http response.
// This method will return a fiber.Handler.
//
// As with DomainForward, every server is parsed and policy-checked at
// handler construction. A misconfigured entry panics at startup.
//
// SSRF note: despite the name, this helper dispatches through the
// shared/user-supplied client rather than a Balancer HostClient, but it
// gets the same protection as Do — up-front host validation plus the
// dial-time validated-IP guard on the dispatching client when
// AllowPrivateIPs is false.
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
		return forwardRouted(c, r.get(), clients...)
	}
}
