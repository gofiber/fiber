package client

import (
	"bytes"
	"crypto/tls"
	"time"

	"github.com/valyala/fasthttp"
)

// defaultRedirectLimit mirrors fasthttp's default when callers supply a negative redirect cap.
const defaultRedirectLimit = 16

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
	Reset()
	Client() any
}

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

func (s *standardClientTransport) Reset() {
	s.client = &fasthttp.Client{}
}

func (s *standardClientTransport) Client() any {
	return s.client
}

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

func (h *hostClientTransport) Reset() {
	h.client = &fasthttp.HostClient{}
}

func (h *hostClientTransport) Client() any {
	return h.client
}

type lbClientTransport struct {
	client    *fasthttp.LBClient
	tlsConfig *tls.Config
	dial      fasthttp.DialFunc
}

func newLBClientTransport(client *fasthttp.LBClient) *lbClientTransport {
	t := &lbClientTransport{client: client}

	if len(client.Clients) > 0 {
		if cfg := extractTLSConfig(client.Clients); cfg != nil {
			t.tlsConfig = cfg
		}
		if dial := extractDial(client.Clients); dial != nil {
			t.dial = dial
		}
	}

	return t
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
	return l.tlsConfig
}

func (l *lbClientTransport) SetTLSConfig(config *tls.Config) {
	l.tlsConfig = config
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.TLSConfig = config
	})
}

func (l *lbClientTransport) SetDial(dial fasthttp.DialFunc) {
	l.dial = dial
	forEachHostClient(l.client, func(hc *fasthttp.HostClient) {
		hc.Dial = dial
	})
}

func (l *lbClientTransport) Reset() {
	l.client = &fasthttp.LBClient{}
	l.tlsConfig = nil
	l.dial = nil
}

func (l *lbClientTransport) Client() any {
	return l.client
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
	switch c := client.(type) {
	case *fasthttp.HostClient:
		fn(c)
	case *fasthttp.LBClient:
		for _, nestedClient := range c.Clients {
			walkBalancingClient(nestedClient, fn)
		}
	case interface{ LBClient() *fasthttp.LBClient }:
		if nested := c.LBClient(); nested != nil {
			walkBalancingClient(nested, fn)
		}
	}
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

// extractDial returns the first dial function discovered while walking the
// provided balancing clients so overrides propagate through nested transports.
func extractDial(clients []fasthttp.BalancingClient) fasthttp.DialFunc {
	var dial fasthttp.DialFunc
	for _, c := range clients {
		if walkBalancingClientWithBreak(c, func(hc *fasthttp.HostClient) bool {
			if hc.Dial != nil {
				dial = hc.Dial
				return true
			}
			return false
		}) {
			break
		}
	}
	return dial
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

		nextURL, err := composeRedirectURL(currentURL, location, req.DisableRedirectPathNormalizing)
		if err != nil {
			return err
		}
		currentURL = nextURL

		if req.Header.IsPost() && (statusCode == fasthttp.StatusMovedPermanently || statusCode == fasthttp.StatusFound || statusCode == fasthttp.StatusSeeOther) {
			req.Header.SetMethod(fasthttp.MethodGet)
			req.SetBody(nil)
			req.Header.Del(fasthttp.HeaderContentType)
		}
	}
}

// composeRedirectURL resolves a redirect target relative to the current request
// URL while rejecting suspicious payloads (e.g. control characters) and
// restricting schemes to HTTP/S so caller-provided Location headers cannot
// trigger arbitrary transports.
func composeRedirectURL(base string, location []byte, disablePathNormalizing bool) (string, error) {
	for _, b := range location {
		if b < 0x20 || b == 0x7f {
			return "", fasthttp.ErrorInvalidURI
		}
	}

	uri := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(uri)

	uri.Update(base)
	uri.UpdateBytes(location)
	uri.DisablePathNormalizing = disablePathNormalizing

	if scheme := uri.Scheme(); len(scheme) > 0 && !bytes.EqualFold(scheme, []byte("http")) && !bytes.EqualFold(scheme, []byte("https")) {
		return "", fasthttp.ErrorInvalidURI
	}

	if len(uri.Scheme()) > 0 && len(uri.Host()) == 0 {
		return "", fasthttp.ErrorInvalidURI
	}

	return uri.String(), nil
}
