package proxy

import (
	"crypto/tls"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// ModifyRequest allows you to alter the request
	//
	// Optional. Default: nil
	ModifyRequest fiber.Handler

	// ModifyResponse allows you to alter the response
	//
	// Optional. Default: nil
	ModifyResponse fiber.Handler

	// tls config for the http client.
	TLSConfig *tls.Config

	// Client is custom client when client config is complex. Each *fasthttp.HostClient
	// among its Clients gets DisablePathNormalizing set when the balancer is built; set it
	// first on a client already in use, which is then left alone.
	// Note that Servers, Timeout, MaxConnsPerHost, ReadBufferSize, WriteBufferSize,
	// MaxResponseBodySize, TLSConfig and DialDualStack will not be used if the client is
	// set, and its dialers are yours to guard.
	Client *fasthttp.LBClient

	// SecurityPolicy overrides the default SSRF, redirect, and
	// hop-by-hop header rules for this balancer. When nil, the
	// package-level policy set via WithSecurityPolicy is used. It is read once,
	// when the balancer is built.
	//
	// Optional. Default: nil
	SecurityPolicy *SecurityPolicy

	// Servers defines a list of <scheme>://<host> HTTP servers,
	// which are used in a round-robin manner.
	// i.e.: "https://foobar.com, http://www.foobar.com"
	//
	// An entry with a path, userinfo, query or fragment panics with ErrUpstreamNotOrigin;
	// "host:port" is read as http://host:port.
	//
	// Required unless Client is set
	Servers []string

	// Timeout is the request timeout used when calling the proxy client
	//
	// Optional. Default: 1 second
	Timeout time.Duration

	// Maximum number of connections per upstream host.
	//
	// Optional. Default: 1024
	MaxConnsPerHost int

	// Per-connection buffer size for reading upstream responses. This also limits the
	// response header size: raise it for upstreams that send multi-KB headers.
	ReadBufferSize int

	// Per-connection buffer size for writing requests to the upstream.
	WriteBufferSize int

	// MaxResponseBodySize bounds the size (in bytes) of upstream
	// responses accepted by the proxy. Responses larger than this are
	// rejected to protect the proxy from memory exhaustion. Zero
	// preserves fasthttp's default unlimited behavior, so set it for untrusted upstreams.
	//
	// Optional. Default: 0
	MaxResponseBodySize int

	// KeepConnectionHeader keeps the "Connection" header when set to true.
	//
	// Note: even when KeepConnectionHeader is true, other RFC 9110 §7.6.1
	// hop-by-hop headers (Keep-Alive, Proxy-Authenticate,
	// Proxy-Authorization, TE, Trailer, Transfer-Encoding, Upgrade) are
	// still stripped. To preserve every hop-by-hop header, set
	// SecurityPolicy.KeepHopByHopHeaders.
	//
	// Optional. Default: false
	KeepConnectionHeader bool

	// Attempt to connect to both ipv4 and ipv6 host addresses if set to true.
	//
	// By default client connects only to ipv4 addresses, since unfortunately ipv6
	// remains broken in many networks worldwide :)
	//
	// Optional. Default: false
	DialDualStack bool
}

const defaultMaxConnsPerHost = 1024

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:                 nil,
	ModifyRequest:        nil,
	ModifyResponse:       nil,
	MaxConnsPerHost:      defaultMaxConnsPerHost,
	Timeout:              fasthttp.DefaultLBClientTimeout,
	KeepConnectionHeader: false,
}

// configDefault function to set default values
func configDefault(config ...Config) Config {
	// Start from the defaults; a Config passed in overrides them
	cfg := ConfigDefault
	if len(config) > 0 {
		cfg = config[0]
	}

	// Set default values
	if cfg.Timeout <= 0 {
		cfg.Timeout = ConfigDefault.Timeout
	}

	if cfg.MaxConnsPerHost <= 0 {
		cfg.MaxConnsPerHost = ConfigDefault.MaxConnsPerHost
	}

	// Servers (or a Client) is required even when no Config was passed
	if len(cfg.Servers) == 0 && cfg.Client == nil {
		panic("Servers cannot be empty")
	}
	return cfg
}

// secureTLSConfig returns a clone of cfg with a TLS 1.2 floor. The
// caller's tls.Config is never mutated so per-host clients can share an
// immutable template without surprise.
func secureTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{MinVersion: tls.VersionTLS12}
	}
	cloned := cfg.Clone()
	if cloned.MinVersion == 0 {
		cloned.MinVersion = tls.VersionTLS12
	}
	return cloned
}
