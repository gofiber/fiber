// Transport adapters unify fasthttp clients behind a shared interface so the
// Fiber client can coordinate behavior like redirects, TLS overrides, and
// dial customizations regardless of the underlying transport type.
package client

import (
	"crypto/tls"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// defaultRedirectLimit mirrors fasthttp's default when callers supply a negative redirect cap.
const defaultRedirectLimit = 16

var (
	// Pre-allocated byte slice for http/https scheme comparison
	httpScheme  = []byte("http")
	httpsScheme = []byte("https")
)

// httpClientTransport unifies the operations exposed by the Fiber client across
// the fasthttp.Client, fasthttp.HostClient, and fasthttp.LBClient adapters so
// helper logic can treat the concrete transports uniformly.
type httpClientTransport interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
	DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error
	DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error
	DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error
	CloseIdleConnections()
	TLSConfig() *tls.Config
	SetTLSConfig(config *tls.Config)
	SetDial(dial fasthttp.DialFunc)
	Client() any
	StreamResponseBody() bool
	SetStreamResponseBody(enable bool)
}

// standardClientTransport adapts fasthttp.Client to the httpClientTransport
// interface used by Fiber's client helpers.
type standardClientTransport struct {
	client *fasthttp.Client
}

func newStandardClientTransport(client *fasthttp.Client) *standardClientTransport {
	return &standardClientTransport{client: client}
}

func (s *standardClientTransport) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return s.client.Do(req, resp)
}

func (s *standardClientTransport) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return s.client.DoTimeout(req, resp, timeout)
}

func (s *standardClientTransport) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return s.client.DoDeadline(req, resp, deadline)
}

// DoRedirects delegates to fasthttp, whose redirect loop resolves relative
// Location values, applies RFC 9110 §15.4.4 to 303 responses for every method,
// and strips credentials on redirects away from the initial host. Routing this
// through doRedirectsWithClient instead would substitute a narrower
// reimplementation of all three.
//
// The trade is that the scheme checks doRedirectsWithClient makes — refusing an
// HTTPS-to-HTTP downgrade and any non-http(s) target — do not apply here.
// fasthttp follows a downgrade the way net/http does, keeping credentials
// because the host is unchanged; see ErrRedirectDowngrade.
func (s *standardClientTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	return s.client.DoRedirects(req, resp, maxRedirects)
}

func (s *standardClientTransport) CloseIdleConnections() {
	s.client.CloseIdleConnections()
}

func (s *standardClientTransport) TLSConfig() *tls.Config {
	return s.client.TLSConfig
}

func (s *standardClientTransport) SetTLSConfig(config *tls.Config) {
	s.client.TLSConfig = config
}

func (s *standardClientTransport) SetDial(dial fasthttp.DialFunc) {
	s.client.Dial = dial
}

func (s *standardClientTransport) Client() any {
	return s.client
}

func (s *standardClientTransport) StreamResponseBody() bool {
	return s.client.StreamResponseBody
}

func (s *standardClientTransport) SetStreamResponseBody(enable bool) {
	s.client.StreamResponseBody = enable
}

// hostClientTransport adapts fasthttp.HostClient to the httpClientTransport
// interface used by Fiber's client helpers.
type hostClientTransport struct {
	client *fasthttp.HostClient
}

func newHostClientTransport(client *fasthttp.HostClient) *hostClientTransport {
	return &hostClientTransport{client: client}
}

func (h *hostClientTransport) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return h.client.Do(req, resp)
}

func (h *hostClientTransport) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return h.client.DoTimeout(req, resp, timeout)
}

func (h *hostClientTransport) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return h.client.DoDeadline(req, resp, deadline)
}

// DoRedirects delegates to fasthttp for the same reasons as
// standardClientTransport.DoRedirects.
func (h *hostClientTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	return h.client.DoRedirects(req, resp, maxRedirects)
}

func (h *hostClientTransport) CloseIdleConnections() {
	h.client.CloseIdleConnections()
}

func (h *hostClientTransport) TLSConfig() *tls.Config {
	return h.client.TLSConfig
}

func (h *hostClientTransport) SetTLSConfig(config *tls.Config) {
	h.client.TLSConfig = config
}

func (h *hostClientTransport) SetDial(dial fasthttp.DialFunc) {
	h.client.Dial = dial
}

func (h *hostClientTransport) Client() any {
	return h.client
}

func (h *hostClientTransport) StreamResponseBody() bool {
	return h.client.StreamResponseBody
}

func (h *hostClientTransport) SetStreamResponseBody(enable bool) {
	h.client.StreamResponseBody = enable
}

// lbClientTransport adapts fasthttp.LBClient to the httpClientTransport
// interface used by Fiber's client helpers.
type lbClientTransport struct {
	client *fasthttp.LBClient
}

func newLBClientTransport(client *fasthttp.LBClient) *lbClientTransport {
	return &lbClientTransport{client: client}
}

func (l *lbClientTransport) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return l.client.Do(req, resp)
}

func (l *lbClientTransport) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return l.client.DoTimeout(req, resp, timeout)
}

func (l *lbClientTransport) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return l.client.DoDeadline(req, resp, deadline)
}

// DoRedirects proxies redirect handling through doRedirectsWithClient so the
// load-balanced transport mirrors fasthttp.Client semantics despite
// fasthttp.LBClient not exposing DoRedirects directly.
func (l *lbClientTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	return doRedirectsWithClient(req, resp, maxRedirects, l.client)
}

func (l *lbClientTransport) CloseIdleConnections() {
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.CloseIdleConnections()
	})
}

func (l *lbClientTransport) TLSConfig() *tls.Config {
	if len(l.client.Clients) == 0 {
		return nil
	}
	return extractTLSConfig(l.client.Clients)
}

func (l *lbClientTransport) SetTLSConfig(config *tls.Config) {
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.TLSConfig = config
	})
}

func (l *lbClientTransport) SetDial(dial fasthttp.DialFunc) {
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.Dial = dial
	})
}

func (l *lbClientTransport) Client() any {
	return l.client
}

func (l *lbClientTransport) StreamResponseBody() bool {
	if len(l.client.Clients) == 0 {
		return false
	}
	// Return the StreamResponseBody setting from the first HostClient
	var streamEnabled bool
	for _, c := range l.client.Clients {
		if walkBalancingClientWithBreak(c, func(hc *fasthttp.HostClient) bool {
			streamEnabled = hc.StreamResponseBody
			return true
		}) {
			break
		}
	}
	return streamEnabled
}

func (l *lbClientTransport) SetStreamResponseBody(enable bool) {
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.StreamResponseBody = enable
	})
}

// forEachHostClient applies fn to every host client reachable from the provided
// load balancer by recursively following nested balancers and wrapper types.
func forEachHostClient(lb *fasthttp.LBClient, fn func(*fasthttp.HostClient)) {
	for _, c := range lb.Clients {
		walkBalancingClient(c, fn)
	}
}

// walkBalancingClient traverses balancing clients recursively, invoking fn for
// every host client discovered beneath the current node.
func walkBalancingClient(client any, fn func(*fasthttp.HostClient)) {
	walkBalancingClientWithBreak(client, func(hc *fasthttp.HostClient) bool {
		fn(hc)
		return false
	})
}

// extractTLSConfig returns the first TLS configuration discovered while walking
// the provided balancing clients so cached settings flow through nested load
// balancers without redundant traversal.
func extractTLSConfig(clients []fasthttp.BalancingClient) *tls.Config {
	var cfg *tls.Config
	for _, c := range clients {
		if walkBalancingClientWithBreak(c, func(hc *fasthttp.HostClient) bool {
			if hc.TLSConfig != nil {
				cfg = hc.TLSConfig
				return true
			}
			return false
		}) {
			break
		}
	}
	return cfg
}

// walkBalancingClientWithBreak traverses balancing clients recursively and
// invokes fn for each host client until fn signals success, enabling early
// termination once a match is found.
func walkBalancingClientWithBreak(client any, fn func(*fasthttp.HostClient) bool) bool {
	switch c := client.(type) {
	case *fasthttp.HostClient:
		return fn(c)
	case *fasthttp.LBClient:
		for _, nestedClient := range c.Clients {
			if walkBalancingClientWithBreak(nestedClient, fn) {
				return true
			}
		}
	case interface{ LBClient() *fasthttp.LBClient }:
		if nested := c.LBClient(); nested != nil {
			if walkBalancingClientWithBreak(nested, fn) {
				return true
			}
		}
	}
	return false
}

// redirectClient describes the minimal Do-capable surface needed by
// doRedirectsWithClient so transports that do not expose DoRedirects (such as
// fasthttp.LBClient) can participate in redirect handling.
type redirectClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

// doRedirectsWithClient mirrors fasthttp's redirect loop for transports that do
// not expose DoRedirects (e.g. fasthttp.LBClient). The helper always issues the
// initial request, respects zero redirect limits, falls back to the default cap
// for negative values, and validates redirect targets before following them.
func doRedirectsWithClient(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, client redirectClient) error {
	currentURL := req.URI().String()
	initialHostname := hostnameWithoutPort(string(req.URI().Host()))
	redirects := 0
	singleRequestOnly := maxRedirects <= 0

	if maxRedirects < 0 {
		maxRedirects = defaultRedirectLimit
		singleRequestOnly = false
	}

	for {
		req.SetRequestURI(currentURL)

		if err := client.Do(req, resp); err != nil {
			return err
		}

		statusCode := resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(statusCode) {
			return nil
		}

		if singleRequestOnly {
			return nil
		}

		redirects++
		if redirects > maxRedirects {
			return fasthttp.ErrTooManyRedirects
		}

		location := resp.Header.Peek("Location")
		if len(location) == 0 {
			return fasthttp.ErrMissingLocation
		}

		nextURL, nextHost, err := composeRedirectURL(currentURL, location, req.DisableRedirectPathNormalizing)
		if err != nil {
			return err
		}
		currentURL = nextURL

		// 301, 302 and 303 turn any body-carrying method into a GET, body and
		// all — net/http's redirectBehavior, for every method rather than just
		// POST, since Fiber drives QUERY through here too and one that kept its
		// method would replay its body to another host. Required for 303 by RFC
		// 9110 Section 15.4.4, permitted for 301 and 302 by 15.4.2 and 15.4.3.
		//
		// fasthttp changes only POST and keeps the body; this is the one place
		// the two deliberately differ. 307 and 308 preserve both by design.
		switch statusCode {
		case fasthttp.StatusMovedPermanently, fasthttp.StatusFound, fasthttp.StatusSeeOther:
			if !req.Header.IsGet() && !req.Header.IsHead() {
				req.Header.SetMethod(fasthttp.MethodGet)
				dropRequestBody(req)
			}
		}

		// Credentials are scoped to the origin that issued them, so drop them
		// once the chain leaves it. The trust boundary is net/http's, which
		// fasthttp's own loop also implements: measured against the *initial*
		// host rather than the previous hop, ignoring the port, and treating a
		// subdomain of it as still trusted.
		if !trustedRedirectTarget(nextHost, initialHostname) {
			for _, h := range crosshost.SensitiveHeaders {
				req.Header.Del(h)
			}
		}
	}
}

// dropRequestBody removes a request's body and everything that frames or
// describes it, so the next hop carries none of it.
//
// Content-Length, Transfer-Encoding and Trailer signal a body (RFC 9112);
// Content-Type and Content-Encoding describe one that is no longer there, and a
// GET announcing a coding with nothing to decode is rejectable. PostArgs is
// reset last: it is parsed lazily, and a stale copy would be re-serialized.
func dropRequestBody(req *fasthttp.Request) {
	req.Header.Del(fasthttp.HeaderContentLength)
	req.Header.Del(fasthttp.HeaderContentType)
	req.Header.Del(fasthttp.HeaderContentEncoding)
	req.Header.Del(fasthttp.HeaderTransferEncoding)
	req.Header.Del(fasthttp.HeaderTrailer)
	req.ResetBody()
	req.PostArgs().Reset()
}

// hostnameWithoutPort strips the port from a host[:port] value, leaving a
// bracketed IPv6 literal's own colons alone.
func hostnameWithoutPort(host string) string {
	// More than one colon without brackets is an IPv6 literal written bare,
	// where the last colon belongs to the address: truncating there folds
	// "fe80::1" and "fe80::2" to one name, and a hop between them would keep
	// its credentials.
	if strings.HasPrefix(host, "[") || strings.Count(host, ":") <= 1 {
		if i := strings.LastIndexByte(host, ':'); i >= 0 && strings.IndexByte(host[i:], ']') < 0 {
			host = host[:i]
		}
	}
	// Exactly one matched pair, not every bracket (strings.Trim folds "[[x]]"
	// to "x"). Two spellings must never fold to one name here.
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return host
}

// trustedRedirectTarget reports whether credentials issued for initialHostname
// may still be sent to host, which may carry a port. The target is trusted when
// it is the initial host or a subdomain of it — the rule net/http documents for
// forwarding sensitive headers across redirects.
func trustedRedirectTarget(host, initialHostname string) bool {
	target := hostnameWithoutPort(host)
	if utils.EqualFold(target, initialHostname) {
		return true
	}
	// No initial hostname means no origin to be a subdomain of, and the suffix
	// test degenerates: every host matches "", so a name ending in '.' would
	// back trusted and keep the credentials.
	if initialHostname == "" {
		return false
	}
	// The same bail-out net/http's isDomainOrSubdomain makes, and for the same
	// reason: a ':' or '%' means this is not a hostname, and running the suffix
	// test anyway matches inside an IPv6 zone identifier. "[::1%.example.com]"
	// would come back a subdomain of example.com and carry the Authorization
	// and Cookie headers to a loopback service.
	if strings.ContainsAny(target, ":%") {
		return false
	}
	return len(target) > len(initialHostname) &&
		target[len(target)-len(initialHostname)-1] == '.' &&
		utils.HasSuffixFold(target, initialHostname)
}

// composeRedirectURL resolves a redirect target relative to the current request
// URL while rejecting suspicious payloads (e.g. control characters) and
// restricting schemes to HTTP/S so caller-provided Location headers cannot
// trigger arbitrary transports. Redirects from HTTPS to plaintext HTTP are
// rejected to prevent credentials from leaking after a TLS handshake.
//
// It returns the resolved URL along with its host, which the caller compares
// against the previous hop to decide whether origin-scoped credentials still
// apply.
func composeRedirectURL(base string, location []byte, disablePathNormalizing bool) (redirectURL, host string, err error) { //nolint:nonamedreturns // names document the two string results
	for _, b := range location {
		if b < 0x20 || b == 0x7f {
			return "", "", fasthttp.ErrorInvalidURI
		}
	}

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)

	uri.Update(base)
	wasHTTPS := utils.EqualFold(uri.Scheme(), httpsScheme)
	uri.UpdateBytes(location)
	uri.DisablePathNormalizing = disablePathNormalizing

	scheme := uri.Scheme()
	if len(scheme) > 0 && !utils.EqualFold(scheme, httpScheme) && !utils.EqualFold(scheme, httpsScheme) {
		return "", "", fasthttp.ErrorInvalidURI
	}

	if len(scheme) > 0 && len(uri.Host()) == 0 {
		return "", "", fasthttp.ErrorInvalidURI
	}

	if wasHTTPS && utils.EqualFold(scheme, httpScheme) {
		return "", "", ErrRedirectDowngrade
	}

	return uri.String(), string(uri.Host()), nil
}
