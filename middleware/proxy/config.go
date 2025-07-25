package proxy

import (
	"crypto/tls"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Config defines the config for middleware.
type Config struct {
	// Transport defines a transport-like mechanism that wraps every request/response.
	Transport fasthttp.RoundTripper

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

	// Client is custom client when client config is complex.
	// Note that Servers, Timeout, WriteBufferSize, ReadBufferSize, TLSConfig
	// and DialDualStack will not be used if the client are set.
	Client *fasthttp.LBClient

	// Callback for establishing new connections to hosts with timeout.
	DialTimeout fasthttp.DialFuncWithTimeout

	// Callback for establishing new connections to hosts.
	Dial fasthttp.DialFunc

	// When the client encounters an error during a request, the behavior
	// whether to retry and whether to reset the request timeout should be
	// determined based on the return value of this field.
	RetryIfErr fasthttp.RetryIfErrFunc

	// Client name. Used in User-Agent request header.
	Name string

	// Servers defines a list of <scheme>://<host> HTTP servers,
	//
	// which are used in a round-robin manner.
	// i.e.: "https://foobar.com, http://www.foobar.com"
	//
	// Required
	Servers []string

	// Timeout is the request timeout used when calling the proxy client
	//
	// Optional. Default: 1 second
	Timeout time.Duration

	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	ReadBufferSize int

	// Per-connection buffer size for responses' writing.
	WriteBufferSize int

	// Maximum number of connections which may be established to the host.
	MaxConns int

	// Keep-alive connections are closed after this duration.
	MaxConnDuration time.Duration

	// Idle keep-alive connections are closed after this duration.
	MaxIdleConnDuration time.Duration

	// Maximum number of attempts for idempotent calls.
	MaxIdemponentCallAttempts int

	// Maximum duration for full response reading (including body).
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	WriteTimeout time.Duration

	// Maximum response body size.
	MaxResponseBodySize int

	// Maximum duration for waiting for a free connection.
	MaxConnWaitTimeout time.Duration

	// Connection pool strategy. Can be either LIFO or FIFO (default).
	ConnPoolStrategy fasthttp.ConnPoolStrategyType

	// Attempt to connect to both ipv4 and ipv6 host addresses if set to true.
	//
	// By default client connects only to ipv4 addresses, since unfortunately ipv6
	// remains broken in many networks worldwide :)
	//
	// Optional. Default: false
	DialDualStack bool

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	IsTLS bool

	// Header names are passed as-is without normalization if this option is set.
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization.
	DisablePathNormalizing bool

	// Will not log potentially sensitive content in error logs.
	SecureErrorLogMessage bool

	// StreamResponseBody enables response body streaming.
	StreamResponseBody bool

	// KeepConnectionHeader disables the default behavior of stripping
	// the "Connection" header from proxied requests and responses.
	// When set to true, the header will be forwarded as-is.
	KeepConnectionHeader bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:                     nil,
	ModifyRequest:            nil,
	ModifyResponse:           nil,
	Timeout:                  fasthttp.DefaultLBClientTimeout,
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
	KeepConnectionHeader:     false,
}

// configDefault function to set default values
func configDefault(config ...Config) Config {
	cfg := ConfigDefault

	if len(config) < 1 {
		return cfg
	}

	c := config[0]
	if c.Next != nil {
		cfg.Next = c.Next
	}
	if c.ModifyRequest != nil {
		cfg.ModifyRequest = c.ModifyRequest
	}
	if c.ModifyResponse != nil {
		cfg.ModifyResponse = c.ModifyResponse
	}
	if c.TLSConfig != nil {
		cfg.TLSConfig = c.TLSConfig
	}
	if c.Client != nil {
		cfg.Client = c.Client
	}
	if len(c.Servers) != 0 {
		cfg.Servers = c.Servers
	}
	if c.Timeout != 0 {
		cfg.Timeout = c.Timeout
	}
	if c.ReadBufferSize != 0 {
		cfg.ReadBufferSize = c.ReadBufferSize
	}
	if c.WriteBufferSize != 0 {
		cfg.WriteBufferSize = c.WriteBufferSize
	}
	cfg.DialDualStack = c.DialDualStack
	if c.Transport != nil {
		cfg.Transport = c.Transport
	}
	if c.DialTimeout != nil {
		cfg.DialTimeout = c.DialTimeout
	}
	if c.Dial != nil {
		cfg.Dial = c.Dial
	}
	if c.RetryIfErr != nil {
		cfg.RetryIfErr = c.RetryIfErr
	}
	if c.Name != "" {
		cfg.Name = c.Name
	}
	if c.MaxConns != 0 {
		cfg.MaxConns = c.MaxConns
	}
	if c.MaxConnDuration != 0 {
		cfg.MaxConnDuration = c.MaxConnDuration
	}
	if c.MaxIdleConnDuration != 0 {
		cfg.MaxIdleConnDuration = c.MaxIdleConnDuration
	}
	if c.MaxIdemponentCallAttempts != 0 {
		cfg.MaxIdemponentCallAttempts = c.MaxIdemponentCallAttempts
	}
	if c.ReadTimeout != 0 {
		cfg.ReadTimeout = c.ReadTimeout
	}
	if c.WriteTimeout != 0 {
		cfg.WriteTimeout = c.WriteTimeout
	}
	if c.MaxResponseBodySize != 0 {
		cfg.MaxResponseBodySize = c.MaxResponseBodySize
	}
	if c.MaxConnWaitTimeout != 0 {
		cfg.MaxConnWaitTimeout = c.MaxConnWaitTimeout
	}
	if c.ConnPoolStrategy != 0 {
		cfg.ConnPoolStrategy = c.ConnPoolStrategy
	}
	cfg.NoDefaultUserAgentHeader = c.NoDefaultUserAgentHeader
	cfg.IsTLS = c.IsTLS
	cfg.DisableHeaderNamesNormalizing = c.DisableHeaderNamesNormalizing
	cfg.DisablePathNormalizing = c.DisablePathNormalizing
	cfg.SecureErrorLogMessage = c.SecureErrorLogMessage
	cfg.StreamResponseBody = c.StreamResponseBody
	cfg.KeepConnectionHeader = c.KeepConnectionHeader

	if len(cfg.Servers) == 0 && cfg.Client == nil {
		panic("Servers cannot be empty")
	}

	return cfg
}
