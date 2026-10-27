package proxy

import (
	"bytes"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/fiber/v3/internal/wiretarget"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Balancer creates a load balancer among multiple upstream servers.
// Servers (or a Client with Clients) is required: without one Balancer panics at startup.
func Balancer(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)
	policy := resolvePolicy(cfg.SecurityPolicy)

	// Load balanced client
	lbc := &fasthttp.LBClient{}
	opaqueClient := false
	// Servers and the per-host options are not used when Client is set.
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

			lbc.Clients = append(lbc.Clients, schemeClient{newHostClient(u, &cfg, policy)})
		}
	} else {
		// Set custom client
		lbc = cfg.Client
		// fasthttp panics on the first request of an LBClient with no clients.
		if len(lbc.Clients) == 0 {
			panic("Client.Clients cannot be empty")
		}
		// A host client the caller built normalizes the request line on the way
		// out unless told otherwise, which would decode "%2F" into a separator and
		// merge "//" and so undo the routed target forwarded below. Any other
		// BalancingClient cannot be told, so with one present a target its
		// normalization could change is refused per request.
		for _, bc := range lbc.Clients {
			if hc, ok := bc.(*fasthttp.HostClient); ok {
				// Written only when unset: a client that already has it, as one in use must (see
				// Config.Client), is not written to while another goroutine may be reading it.
				if !hc.DisablePathNormalizing {
					hc.DisablePathNormalizing = true
				}
				continue
			}
			opaqueClient = true
		}
	}

	// KeepConnectionHeader exempts Connection from the hop-by-hop strip.
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
		keepForwardingOrder(c)

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

// schemeClient sends every request with the scheme its HostClient speaks: a HostClient refuses a
// request of another scheme, and Balancer forwards the scheme of the connection it arrived on.
type schemeClient struct {
	*fasthttp.HostClient
}

func (c schemeClient) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	if c.IsTLS {
		req.URI().SetScheme(schemeHTTPS)
	} else {
		req.URI().SetScheme(schemeHTTP)
	}
	return c.HostClient.DoDeadline(req, resp, deadline)
}

// newHostClient builds the client Balancer dials one validated upstream with.
func newHostClient(u *url.URL, cfg *Config, policy SecurityPolicy) *fasthttp.HostClient {
	isTLS := u.Scheme == schemeHTTPS
	// Add the scheme's default port: only fasthttp's own dialer does, not the guard's.
	addr := fasthttp.AddMissingPort(u.Host, isTLS)
	hc := &fasthttp.HostClient{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
		Addr:                     addr,
		IsTLS:                    isTLS,
		MaxConns:                 cfg.MaxConnsPerHost,

		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,

		TLSConfig: secureTLSConfig(cfg.TLSConfig),

		DialDualStack: cfg.DialDualStack,

		MaxResponseBodySize: cfg.MaxResponseBodySize,
	}
	// Validate the resolved IP at dial time so DNS rebinding cannot swap in a private address.
	// fasthttp dials through DialTimeout when it is set and Dial otherwise, so both are guarded.
	if !policy.AllowPrivateIPs {
		g := newDialGuard(cfg.DialDualStack)
		hc.Dial = g.dialChecked
		hc.DialTimeout = g.dialCheckedTimeout
	}
	return hc
}
