// Transport adapters unify fasthttp clients behind a shared interface so the
// Fiber client can coordinate behavior like redirects, TLS overrides, and
// dial customizations regardless of the underlying transport type.
package client

import (
	"crypto/tls"
	"fmt"
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

// DoRedirects follows redirects through doRedirectsWithClient rather than
// fasthttp's own loop, so every transport applies the same target validation.
// fasthttp's loop takes an HTTPS-to-HTTP hop; this one refuses it.
func (s *standardClientTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	// Before the loop serializes the URI, as fasthttp's own DoRedirects does:
	// the setting has to reach req.URI() first or "/a//b" is normalized away on
	// the very request the transport was configured to leave alone.
	if s.client.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}
	return doRedirectsWithClient(req, resp, maxRedirects, s.client)
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

// DoRedirects uses the shared loop for the same reasons as
// standardClientTransport.DoRedirects.
func (h *hostClientTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	// See standardClientTransport.DoRedirects.
	if h.client.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}
	return doRedirectsWithClient(req, resp, maxRedirects, h.client)
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
// doRedirectsWithClient, which every transport satisfies including
// fasthttp.LBClient, the one that exposes no DoRedirects of its own.
type redirectClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

// doRedirectsWithClient is the redirect loop behind every transport, so target
// validation does not depend on which one a caller built. It always issues the
// initial request, falls back to the default cap for a negative limit, and
// validates a redirect target before following one.
//
// A limit of zero is counted the way fasthttp counts it — the first redirect is
// the first one counted, so it returns ErrTooManyRedirects. Returning the
// redirect response instead left a caller unable to tell a request that finished
// from one that stopped at a hop, and it is what the DoRedirects this stands in
// for does.
func doRedirectsWithClient(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int, client redirectClient) error {
	if resp == nil {
		// "Response is ignored if resp is nil", per fasthttp's own DoRedirects
		// doc. Its loop reads resp.Header regardless and panics, so honor the
		// documented contract here rather than inherit that.
		resp = fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
	}

	currentURL := req.URI().String()
	initialHostname := hostnameWithoutPort(string(req.URI().Host()))
	redirects := 0

	if maxRedirects < 0 {
		maxRedirects = defaultRedirectLimit
	}

	for {
		req.SetRequestURI(currentURL)

		// Where fasthttp's own loop calls the error-returning req.parseURI, and
		// for the same reason: Request.URI swallows the error and hands back a
		// half-parsed URI, so "http://example.com:abc/x" keeps the malformed
		// authority as its host and loses the path. A HostClient dials its fixed
		// Addr regardless, so that host went out as the Host header of a request
		// the caller never wrote. The redirect targets below are checked as they
		// are composed; this is the same check for the URI the caller supplied.
		if err := parsesAsURI([]byte(currentURL)); err != nil {
			return err
		}

		if err := client.Do(req, resp); err != nil {
			return err
		}

		statusCode := resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(statusCode) {
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

		// 301, 302 and 303 retry as a GET without the body: net/http's
		// redirectBehavior. The body goes even when the method already was GET or
		// HEAD, or it would be replayed to the target, possibly a different host.
		switch statusCode {
		case fasthttp.StatusMovedPermanently, fasthttp.StatusFound, fasthttp.StatusSeeOther:
			if !req.Header.IsGet() && !req.Header.IsHead() {
				req.Header.SetMethod(fasthttp.MethodGet)
			}
			dropRequestBody(req)
		}

		// Credentials are scoped to the origin that issued them, so drop them once
		// the chain leaves it. net/http's boundary, which fasthttp also implements:
		// against the initial host, ignoring the port, subdomains still trusted.
		if !trustedRedirectTarget(nextHost, initialHostname) {
			for _, h := range crosshost.SensitiveHeaders {
				req.Header.Del(h)
			}
		}
	}
}

// dropRequestBody removes a request's body and everything framing or describing
// it: Content-Length, Transfer-Encoding and Trailer signal one (RFC 9112),
// Content-Type and Content-Encoding describe one no longer there.
func dropRequestBody(req *fasthttp.Request) {
	req.Header.Del(fasthttp.HeaderContentLength)
	req.Header.Del(fasthttp.HeaderContentType)
	req.Header.Del(fasthttp.HeaderContentEncoding)
	req.Header.Del(fasthttp.HeaderTransferEncoding)
	req.Header.Del(fasthttp.HeaderTrailer)
	// A 100-continue expectation may not be generated without content
	// (RFC 9110 Section 10.1.1), and a strict target answers the bodyless
	// follow-up with 417 rather than the redirect the caller was following.
	req.Header.Del(fasthttp.HeaderExpect)
	req.ResetBody()
	// Load-bearing, not bookkeeping: Request.Write falls back to the parsed form
	// when the body is empty, so a caller that had read PostArgs would have sent
	// the old "a=1" as the body of the redirected GET.
	//
	// fasthttp's own loop clears the parsed flag beside this, which leaves a
	// reused request re-parsing whatever body it is given next. That half is not
	// reachable from here: parsedPostArgs is unexported and only Request.Reset
	// and ReadLimitBody clear it, and Reset would take the URI, headers and TLS
	// flag this loop is still using. What differs is confined to a caller that
	// reuses the request without resetting it: its PostArgs reads empty rather
	// than re-parsing. The body it sends is the new one either way, and an
	// untouched fasthttp request is worse off there — it hands back the previous
	// body's values, since SetBody does not invalidate the form either.
	req.PostArgs().Reset()
}

// hostnameWithoutPort strips the port from a host[:port] value, leaving a
// bracketed IPv6 literal's own colons alone.
func hostnameWithoutPort(host string) string {
	// More than one colon without brackets is a bare IPv6 literal, where the last
	// colon belongs to the address: truncating there folds "fe80::1" and
	// "fe80::2" to one name, and a hop between them would keep its credentials.
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
// may still be sent to host, which may carry a port: trusted when it is that
// host or a subdomain — net/http's rule for forwarding sensitive headers.
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
	// The same bail-out net/http's isDomainOrSubdomain makes: a ':' or '%' means
	// this is no hostname, and the suffix test would match inside an IPv6 zone
	// identifier — "[::1%.example.com]" would come back a subdomain of it.
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
// parsesAsURI reports whether fasthttp reads full as a URI at all, returning the
// reason it does not. Only Parse surfaces that; Update and UpdateBytes discard it.
func parsesAsURI(full []byte) error {
	check := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(check)

	if err := check.Parse(nil, full); err != nil {
		return fmt.Errorf("%w: %w", fasthttp.ErrorInvalidURI, err)
	}
	return nil
}

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

	// UpdateBytes resolves the reference but discards the parse error, leaving
	// whatever it had read behind: "http://example.com:abc/x" keeps the host
	// "example.com:abc" and loses the path. fasthttp's own redirect loop returned
	// that error rather than sending a second request, and a HostClient connects
	// to its fixed Addr, so following one carries the malformed Host to a server
	// that would otherwise never have been asked. Parse reports what UpdateBytes
	// swallowed, so ask it of the result.
	if err := parsesAsURI(uri.FullURI()); err != nil {
		return "", "", err
	}

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
