package proxy

import (
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/internal/wiretarget"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
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
// a private one between validation and connection. The policy is read on
// every request, so keep it installed while the handler serves.
//
// The default client sets no timeout and no response size cap.
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
// The request reaches the upstream as HTTP/1.1. As with Forward, there is no
// timeout or size cap; see DoTimeout and DoDeadline.
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
	u, err := validateUpstream(addrCopy, policy, lookupsFor(cli))
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

// forwardRouted forwards the routed path to base for DomainForward and BalancerForward. A path
// an upstream could read as another (wiretarget.SegmentsAsRouted) is a 400 unless AllowAmbiguousSlashes.
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

// get picks the next upstream with one Add. The counter wraps after 2^64 picks, where the cycle
// takes one uneven step.
func (r *urlRoundrobin) get() *url.URL {
	return r.pool[(r.next.Add(1)-1)%uint64(len(r.pool))]
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
		base, err := validateUpstream(s, policy, nil)
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
