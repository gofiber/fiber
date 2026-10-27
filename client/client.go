// Package client exposes Fiber's HTTP client built on top of fasthttp.
//
// It allows constructing new clients or wrapping existing fasthttp transports
// so applications can share pools, dialers, and TLS settings between Fiber and
// lower-level fasthttp integrations.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	stdlog "log" //nolint:depguard // log.DefaultLogger is keyed by the standard library logger type the default logger exposes.
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/utils/v2"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"golang.org/x/net/http/httpproxy"
)

var ErrFailedToAppendCert = errors.New("failed to append certificate")

// Client provides Fiber's high-level HTTP API while delegating transport work
// to fasthttp.Client, fasthttp.HostClient, or fasthttp.LBClient implementations.
//
// Settings configured on the client are shared across every request and may be
// overridden per request when needed.
// Client is safe for concurrent request execution after configuration is
// complete. Concurrent configuration changes require external synchronization.
type Client struct {
	logger    log.CommonLogger
	transport httpClientTransport

	header  *Header
	params  *QueryParam
	cookies *Cookie
	path    *PathParam

	jsonMarshal   utils.JSONMarshal
	jsonUnmarshal utils.JSONUnmarshal
	xmlMarshal    utils.XMLMarshal
	xmlUnmarshal  utils.XMLUnmarshal
	cborMarshal   utils.CBORMarshal
	cborUnmarshal utils.CBORUnmarshal

	cookieJar                *CookieJar
	retryConfig              *RetryConfig
	baseURL                  string
	userAgent                string
	referer                  string
	userRequestHooks         []RequestHook
	builtinRequestHooks      []RequestHook
	finalRequestHooks        []RequestHook
	userResponseHooks        []ResponseHook
	builtinResponseHooks     []ResponseHook
	redirectSensitiveHeaders crosshost.HeaderNames

	timeout                   time.Duration
	mu                        sync.RWMutex
	isDebug                   bool
	isPathNormalizingDisabled bool
}

// Do executes the request using the underlying fasthttp transport.
//
// It mirrors [fasthttp.Client.Do], [fasthttp.HostClient.Do], or
// [fasthttp.LBClient.Do] depending on how the Fiber client was constructed.
func (c *Client) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return c.transport.Do(req, resp)
}

// DoTimeout executes the request and waits for a response up to the provided timeout.
// It mirrors the behavior of the respective fasthttp client's DoTimeout implementation.
func (c *Client) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return c.transport.DoTimeout(req, resp, timeout)
}

// DoDeadline executes the request and waits for a response until the provided deadline.
// It mirrors the behavior of the respective fasthttp client's DoDeadline implementation.
func (c *Client) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return c.transport.DoDeadline(req, resp, deadline)
}

// DoRedirects executes the request following redirects up to maxRedirects.
// It honors SetRedirectSensitiveHeaders.
func (c *Client) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error {
	opts := c.redirectOptionsFor(maxRedirects, crosshost.HeaderNames{})
	return c.transport.DoRedirects(req, resp, &opts)
}

// CloseIdleConnections closes idle connections on the underlying fasthttp transport when supported.
func (c *Client) CloseIdleConnections() {
	c.transport.CloseIdleConnections()
}

func (c *Client) currentTLSConfig() *tls.Config {
	return c.transport.TLSConfig()
}

func (c *Client) applyTLSConfig(config *tls.Config) {
	c.transport.SetTLSConfig(config)
}

func (c *Client) applyDial(dial fasthttp.DialFunc) {
	c.transport.SetDial(dial)
}

// FasthttpClient returns the underlying *fasthttp.Client if the client was created with one.
func (c *Client) FasthttpClient() *fasthttp.Client {
	if client, ok := c.transport.(*standardClientTransport); ok {
		return client.client
	}
	return nil
}

// HostClient returns the underlying fasthttp.HostClient if the client was created with one.
func (c *Client) HostClient() *fasthttp.HostClient {
	if client, ok := c.transport.(*hostClientTransport); ok {
		return client.client
	}
	return nil
}

// LBClient returns the underlying fasthttp.LBClient if the client was created with one.
func (c *Client) LBClient() *fasthttp.LBClient {
	if client, ok := c.transport.(*lbClientTransport); ok {
		return client.client
	}
	return nil
}

// R creates a new Request associated with the client.
func (c *Client) R() *Request {
	return AcquireRequest().SetClient(c)
}

// ownedRequest acquires a Request for a client helper call; Response.Close releases it.
func (c *Client) ownedRequest() *Request {
	req := AcquireRequest().SetClient(c)
	req.clientOwned = true
	return req
}

// RequestHook returns a copy of the user-defined request hooks.
func (c *Client) RequestHook() []RequestHook {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.userRequestHooks)
}

// AddRequestHook adds user-defined request hooks.
func (c *Client) AddRequestHook(h ...RequestHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userRequestHooks = append(c.userRequestHooks, h...)
	return c
}

// FinalRequestHook returns a copy of the user-defined final request hooks.
func (c *Client) FinalRequestHook() []RequestHook {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.finalRequestHooks)
}

// AddFinalRequestHook registers hooks for the point after request serialization
// and immediately before transport execution.
func (c *Client) AddFinalRequestHook(h ...RequestHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.finalRequestHooks = append(c.finalRequestHooks, h...)
	return c
}

// ResponseHook returns a copy of the user-defined response hooks.
func (c *Client) ResponseHook() []ResponseHook {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return slices.Clone(c.userResponseHooks)
}

// AddResponseHook adds user-defined response hooks.
func (c *Client) AddResponseHook(h ...ResponseHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userResponseHooks = append(c.userResponseHooks, h...)
	return c
}

// JSONMarshal returns the JSON marshal function used by the client.
func (c *Client) JSONMarshal() utils.JSONMarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.jsonMarshal
}

// SetJSONMarshal sets the JSON marshal function to use.
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.jsonMarshal = f
	return c
}

// JSONUnmarshal returns the JSON unmarshal function used by the client.
func (c *Client) JSONUnmarshal() utils.JSONUnmarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.jsonUnmarshal
}

// SetJSONUnmarshal sets the JSON unmarshal function to use.
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.jsonUnmarshal = f
	return c
}

// XMLMarshal returns the XML marshal function used by the client.
func (c *Client) XMLMarshal() utils.XMLMarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.xmlMarshal
}

// SetXMLMarshal sets the XML marshal function to use.
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.xmlMarshal = f
	return c
}

// XMLUnmarshal returns the XML unmarshal function used by the client.
func (c *Client) XMLUnmarshal() utils.XMLUnmarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.xmlUnmarshal
}

// SetXMLUnmarshal sets the XML unmarshal function to use.
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.xmlUnmarshal = f
	return c
}

// CBORMarshal returns the CBOR marshal function used by the client.
func (c *Client) CBORMarshal() utils.CBORMarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cborMarshal
}

// SetCBORMarshal sets the CBOR marshal function to use.
func (c *Client) SetCBORMarshal(f utils.CBORMarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cborMarshal = f
	return c
}

// CBORUnmarshal returns the CBOR unmarshal function used by the client.
func (c *Client) CBORUnmarshal() utils.CBORUnmarshal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cborUnmarshal
}

// SetCBORUnmarshal sets the CBOR unmarshal function to use.
func (c *Client) SetCBORUnmarshal(f utils.CBORUnmarshal) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cborUnmarshal = f
	return c
}

// TLSConfig returns the client's TLS configuration.
// If none is set, it initializes a new one.
func (c *Client) TLSConfig() *tls.Config {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cfg := c.currentTLSConfig(); cfg != nil {
		return cfg
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	c.applyTLSConfig(cfg)
	return cfg
}

// SetTLSConfig sets the TLS configuration for the client.
func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.applyTLSConfig(config)
	return c
}

// SetCertificates adds certificates to the client's TLS configuration.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	config := c.TLSConfig()
	config.Certificates = append(config.Certificates, certs...)
	return c
}

// panicf logs the message through the client's logger and panics with it. It
// panics without a log line when the logger was set to nil, and also when the
// logger's Panicf returns, so a setting that could not be applied is never
// reported as applied.
func (c *Client) panicf(format string, args ...any) {
	if logger := c.Logger(); logger != nil {
		logger.Panicf(format, args...)
	}
	panic(fmt.Sprintf(format, args...))
}

// SetRootCertificate adds one or more root certificates to the client's TLS configuration.
// It panics when the file cannot be read or holds no valid PEM certificate.
func (c *Client) SetRootCertificate(path string) *Client {
	cleanPath := filepath.Clean(path)
	file, err := os.Open(cleanPath)
	if err != nil {
		c.panicf("client: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			c.panicf("client: failed to close file: %v", closeErr)
		}
	}()

	pem, err := io.ReadAll(file)
	if err != nil {
		c.panicf("client: %v", err)
	}

	config := c.TLSConfig()
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	if !config.RootCAs.AppendCertsFromPEM(pem) {
		c.panicf("client: %v", ErrFailedToAppendCert)
	}

	return c
}

// SetRootCertificateFromString adds one or more root certificates from a string to the client's TLS configuration.
// It panics when the string holds no valid PEM certificate.
func (c *Client) SetRootCertificateFromString(pem string) *Client {
	config := c.TLSConfig()

	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	if !config.RootCAs.AppendCertsFromPEM([]byte(pem)) {
		c.panicf("client: %v", ErrFailedToAppendCert)
	}

	return c
}

// SetProxyURL sets the proxy URL for the client. This affects all subsequent requests.
func (c *Client) SetProxyURL(proxyURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build the fasthttp proxy dialer directly so invalid proxy URLs are returned
	// to callers instead of being swallowed by FasthttpHTTPDialer.
	dialer := fasthttpproxy.Dialer{
		Config: httpproxy.Config{HTTPProxy: proxyURL, HTTPSProxy: proxyURL},
	}
	dialFunc, err := dialer.GetDialFunc(false)
	if err != nil {
		return fmt.Errorf("client: invalid proxy URL: %w", err)
	}
	c.applyDial(dialFunc)
	return nil
}

// RetryConfig returns a copy of the current retry configuration.
func (c *Client) RetryConfig() *RetryConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.retryConfig == nil {
		return nil
	}

	cfg := *c.retryConfig
	return &cfg
}

// SetRetryConfig sets the retry configuration for the client.
func (c *Client) SetRetryConfig(config *RetryConfig) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.retryConfig = config
	return c
}

// BaseURL returns the client's base URL.
func (c *Client) BaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.baseURL
}

// SetBaseURL sets the base URL prefix for all requests made by the client.
func (c *Client) SetBaseURL(url string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.baseURL = url
	return c
}

// Header returns all header values associated with the provided key.
func (c *Client) Header(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.header.PeekMultiple(key)
}

// AddHeader adds a single header field and its value to the client. These headers apply to all requests.
func (c *Client) AddHeader(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.header.Add(key, val)
	return c
}

// SetHeader sets a single header field and its value in the client, overriding any previously set value.
func (c *Client) SetHeader(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.header.Del(key)
	c.header.Set(key, val)
	return c
}

// AddHeaders adds multiple header fields and their values to the client.
func (c *Client) AddHeaders(h map[string][]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.header.AddHeaders(h)
	return c
}

// SetHeaders method sets multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests created from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) SetHeaders(h map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.header.SetHeaders(h)
	return c
}

// Param returns all values of the specified query parameter.
func (c *Client) Param(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tmp := c.params.PeekMulti(key)
	res := make([]string, 0, len(tmp))
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}

	return res
}

// AddParam adds a single query parameter and its value to the client.
// These params will be applied to all requests created from this client instance.
func (c *Client) AddParam(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.params.Add(key, val)
	return c
}

// SetParam sets a single query parameter and its value in the client.
func (c *Client) SetParam(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.params.Set(key, val)
	return c
}

// AddParams adds multiple query parameters and their values to the client.
func (c *Client) AddParams(m map[string][]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.params.AddParams(m)
	return c
}

// SetParams sets multiple query parameters and their values in the client.
func (c *Client) SetParams(m map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.params.SetParams(m)
	return c
}

// SetParamsWithStruct sets multiple query parameters and their values using a struct.
func (c *Client) SetParamsWithStruct(v any) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.params.SetParamsWithStruct(v)
	return c
}

// DelParams deletes one or more query parameters and their values from the client.
func (c *Client) DelParams(key ...string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, v := range key {
		c.params.Del(v)
	}
	return c
}

// SetUserAgent sets the User-Agent header for the client.
func (c *Client) SetUserAgent(ua string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userAgent = ua
	return c
}

// SetReferer sets the Referer header for the client.
func (c *Client) SetReferer(r string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.referer = r
	return c
}

// DisablePathNormalizing reports whether path normalizing is disabled for the client.
func (c *Client) DisablePathNormalizing() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isPathNormalizingDisabled
}

// SetDisablePathNormalizing configures the client to disable or enable path normalizing.
//
// fasthttp normalizes the path of a request when it writes it, according to the
// client that sends it, so the setting is applied to the wrapped fasthttp client
// (every host client behind a load balancer): a client created with
// NewWithClient, NewWithHostClient or NewWithLBClient changes the one it wraps,
// for everyone who shares it. Call it before the first request, because a
// fasthttp.Client copies the setting into the host clients it has already
// created only when it creates them.
//
// The check that keeps a path parameter value inside its segment is lifted only
// when the wrapped client is known to send the path as built: a client from New,
// a fasthttp.Client handed to NewWithClient with the setting already on, or a
// host client or balancer whose host clients all have it on. Once a request has
// gone out through a fasthttp.Client that normalized, it stays on.
func (c *Client) SetDisablePathNormalizing(disable bool) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isPathNormalizingDisabled = disable
	if t, ok := c.transport.(pathNormalizingTransport); ok {
		t.SetDisablePathNormalizing(disable)
	}
	return c
}

// RedirectSensitiveHeaders returns a copy of the names set with SetRedirectSensitiveHeaders.
func (c *Client) RedirectSensitiveHeaders() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.redirectSensitiveHeaders.List()
}

// SetRedirectSensitiveHeaders names request headers, such as an X-Api-Key of your own, to drop with the
// built-in set when a redirect leaves the host a request started on. Names match case-insensitively;
// no names clears the list. Request.SetRedirectSensitiveHeaders adds names for one request.
func (c *Client) SetRedirectSensitiveHeaders(names ...string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.redirectSensitiveHeaders = crosshost.NewHeaderNames(names)
	return c
}

// redirectOptionsFor joins the names into a fresh list, which a recycled request cannot change.
func (c *Client) redirectOptionsFor(maxRedirects int, own crosshost.HeaderNames) redirectOptions {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return redirectOptions{maxRedirects: maxRedirects, sensitiveHeaders: c.redirectSensitiveHeaders.Join(own)}
}

// writesPathAsBuilt reports whether the wrapped fasthttp client sends the request
// path exactly as built, so nothing normalizes it after Fiber has put it together.
func (c *Client) writesPathAsBuilt() bool {
	t, ok := c.transport.(pathNormalizingTransport)
	return ok && t.DisablePathNormalizing()
}

// PathParam returns the value of the specified path parameter. Returns an empty string if it does not exist.
func (c *Client) PathParam(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := (*c.path)[key]; ok {
		return val
	}
	return ""
}

// SetPathParam sets a single path parameter and its value in the client.
func (c *Client) SetPathParam(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.path.SetParam(key, val)
	return c
}

// SetPathParams sets multiple path parameters and their values in the client.
func (c *Client) SetPathParams(m map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.path.SetParams(m)
	return c
}

// SetPathParamsWithStruct sets multiple path parameters and their values using a struct.
func (c *Client) SetPathParamsWithStruct(v any) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.path.SetParamsWithStruct(v)
	return c
}

// DelPathParams deletes one or more path parameters and their values from the client.
func (c *Client) DelPathParams(key ...string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.path.DelParams(key...)
	return c
}

// Cookie returns the value of the specified cookie. Returns an empty string if it does not exist.
func (c *Client) Cookie(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := (*c.cookies)[key]; ok {
		return val
	}
	return ""
}

// SetCookie sets a single cookie and its value in the client.
func (c *Client) SetCookie(key, val string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookies.SetCookie(key, val)
	return c
}

// SetCookies sets multiple cookies and their values in the client.
func (c *Client) SetCookies(m map[string]string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookies.SetCookies(m)
	return c
}

// SetCookiesWithStruct sets multiple cookies and their values using a struct.
func (c *Client) SetCookiesWithStruct(v any) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookies.SetCookiesWithStruct(v)
	return c
}

// DelCookies deletes one or more cookies and their values from the client.
func (c *Client) DelCookies(key ...string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookies.DelCookies(key...)
	return c
}

// SetTimeout sets the timeout value for the client. This applies to all requests unless overridden at the request level.
func (c *Client) SetTimeout(t time.Duration) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.timeout = t
	return c
}

// Debug enables debug-level logging output: after each response, the raw request and
// response, headers and bodies included, are logged through the client's logger.
func (c *Client) Debug() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isDebug = true
	return c
}

// DisableDebug disables debug-level logging output.
func (c *Client) DisableDebug() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isDebug = false
	return c
}

// StreamResponseBody returns the current StreamResponseBody setting.
func (c *Client) StreamResponseBody() bool {
	return c.transport.StreamResponseBody()
}

// SetStreamResponseBody enables or disables response body streaming.
// When enabled, the response body can be read as a stream using BodyStream()
// instead of being fully loaded into memory. This is useful for large responses
// or server-sent events.
func (c *Client) SetStreamResponseBody(enable bool) *Client {
	c.transport.SetStreamResponseBody(enable)
	return c
}

// MaxResponseBodySize returns the largest response body the client buffers, in
// bytes. Zero or less means no limit.
func (c *Client) MaxResponseBodySize() int {
	return c.transport.MaxResponseBodySize()
}

// SetMaxResponseBodySize sets the largest response body the client buffers, in
// bytes; a larger one fails with fasthttp.ErrBodyTooLarge. A client from New
// starts at 100 MiB, a wrapped fasthttp client keeps its own setting, and zero or
// less removes the limit. A streamed body is not limited. Call it before the
// first request: a fasthttp.Client gives each host the limit it has when it
// first serves that host.
func (c *Client) SetMaxResponseBodySize(size int) *Client {
	c.transport.SetMaxResponseBodySize(size)
	return c
}

// SetCookieJar sets the cookie jar for the client.
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookieJar = cookieJar
	return c
}

// Get sends a GET request to the specified URL, similar to axios.
func (c *Client) Get(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Get(url)
}

// Post sends a POST request to the specified URL, similar to axios.
func (c *Client) Post(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Post(url)
}

// Head sends a HEAD request to the specified URL, similar to axios.
func (c *Client) Head(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Head(url)
}

// Put sends a PUT request to the specified URL, similar to axios.
func (c *Client) Put(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Put(url)
}

// Delete sends a DELETE request to the specified URL, similar to axios.
func (c *Client) Delete(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Delete(url)
}

// Options sends an OPTIONS request to the specified URL, similar to axios.
func (c *Client) Options(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Options(url)
}

// Patch sends a PATCH request to the specified URL, similar to axios.
func (c *Client) Patch(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Patch(url)
}

// Query sends a QUERY request to the specified URL, similar to axios.
func (c *Client) Query(url string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Query(url)
}

// Custom sends a request with a custom method to the specified URL, similar to axios.
func (c *Client) Custom(url, method string, cfg ...Config) (*Response, error) {
	req := c.ownedRequest()
	setConfigToRequest(req, cfg...)
	return req.Custom(url, method)
}

// SetDial sets the custom dial function for the client.
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.applyDial(dial)
	return c
}

// SetLogger sets the logger instance used by the client.
func (c *Client) SetLogger(logger log.CommonLogger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger = logger
	return c
}

// Logger returns the logger instance used by the client.
func (c *Client) Logger() log.CommonLogger {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.logger
}

// Reset resets the client to its default state, clearing most configurations
// and replacing the underlying transport with a new fasthttp.Client so future
// requests resume with Fiber's standard transport settings.
func (c *Client) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.transport = newOwnedStandardClientTransport()
	c.baseURL = ""
	c.timeout = 0
	c.userAgent = ""
	c.referer = ""
	c.retryConfig = nil
	c.isDebug = false
	c.isPathNormalizingDisabled = false
	c.redirectSensitiveHeaders = crosshost.HeaderNames{}

	if c.cookieJar != nil {
		c.cookieJar.Release()
		c.cookieJar = nil
	}

	c.path.Reset()
	c.cookies.Reset()
	c.header.Reset()
	c.params.Reset()
}

// Config is used to easily set request parameters. Note that when setting a request body,
// JSON is used as the default serialization mechanism. The priority is:
// Body > FormData > File.
//
// RedirectSensitiveHeaders is passed to Request.SetRedirectSensitiveHeaders; an empty list changes nothing.
type Config struct {
	Ctx                      context.Context //nolint:containedctx // It's needed to be stored in the config.
	Body                     any
	Header                   map[string]string
	Param                    map[string]string
	Cookie                   map[string]string
	PathParam                map[string]string
	FormData                 map[string]string
	UserAgent                string
	Referer                  string
	File                     []*File
	RedirectSensitiveHeaders []string
	Timeout                  time.Duration
	MaxRedirects             int
	DisablePathNormalizing   bool
}

// setConfigToRequest sets the parameters passed via Config to the Request.
func setConfigToRequest(req *Request, config ...Config) {
	if len(config) == 0 {
		return
	}
	cfg := config[0]

	if cfg.Ctx != nil {
		req.SetContext(cfg.Ctx)
	}

	if cfg.UserAgent != "" {
		req.SetUserAgent(cfg.UserAgent)
	}

	if cfg.Referer != "" {
		req.SetReferer(cfg.Referer)
	}

	if cfg.Header != nil {
		req.SetHeaders(cfg.Header)
	}

	if cfg.Param != nil {
		req.SetParams(cfg.Param)
	}

	if cfg.Cookie != nil {
		req.SetCookies(cfg.Cookie)
	}

	if cfg.PathParam != nil {
		req.SetPathParams(cfg.PathParam)
	}

	if cfg.Timeout != 0 {
		req.SetTimeout(cfg.Timeout)
	}

	if cfg.MaxRedirects != 0 {
		req.SetMaxRedirects(cfg.MaxRedirects)
	}

	if len(cfg.RedirectSensitiveHeaders) > 0 {
		req.SetRedirectSensitiveHeaders(cfg.RedirectSensitiveHeaders...)
	}

	if cfg.DisablePathNormalizing {
		req.SetDisablePathNormalizing(true)
	}

	if cfg.Body != nil {
		switch v := cfg.Body.(type) {
		case []byte:
			req.SetRawBody(v)
		case string:
			req.SetRawBody([]byte(v))
		default:
			req.SetJSON(cfg.Body)
		}
		return
	}

	if cfg.FormData != nil {
		req.SetFormDataWithMap(cfg.FormData)
		return
	}

	if len(cfg.File) != 0 {
		req.AddFiles(cfg.File...)
		return
	}
}

var (
	defaultClient    atomic.Pointer[Client]
	replaceMu        = sync.Mutex{}
	defaultUserAgent = "fiber"
)

func init() {
	defaultClient.Store(New())
}

// New creates and returns a new Client object.
func New() *Client {
	// Follow-up performance optimizations:
	// Try to use a pool to reduce the memory allocation cost for the Fiber client and the fasthttp client.
	// If possible, also consider pooling other structs (e.g., request headers, cookies, query parameters, path parameters).
	return newClient(newOwnedStandardClientTransport())
}

// NewWithClient creates and returns a new Client object from an existing fasthttp.Client.
//
// Fiber cannot tell whether that client has served a host already, and a
// fasthttp.Client gives each host the path normalizing setting it had when it
// first served that host. So unless the client was created with
// DisablePathNormalizing set, SetDisablePathNormalizing changes what is sent to
// hosts not yet served, but the check that keeps a path parameter value inside
// its segment stays on.
func NewWithClient(c *fasthttp.Client) *Client {
	if c == nil {
		panic("fasthttp.Client must not be nil")
	}
	return newClient(newStandardClientTransport(c))
}

// NewWithHostClient creates and returns a new Client object from an existing fasthttp.HostClient.
func NewWithHostClient(c *fasthttp.HostClient) *Client {
	if c == nil {
		panic("fasthttp.HostClient must not be nil")
	}
	return newClient(newHostClientTransport(c))
}

// NewWithLBClient creates and returns a new Client object from an existing fasthttp.LBClient.
func NewWithLBClient(c *fasthttp.LBClient) *Client {
	if c == nil {
		panic("fasthttp.LBClient must not be nil")
	}
	return newClient(newLBClientTransport(c))
}

func newClient(transport httpClientTransport) *Client {
	return &Client{
		transport: transport,
		header: &Header{
			RequestHeader: &fasthttp.RequestHeader{},
		},
		params: &QueryParam{
			Args: fasthttp.AcquireArgs(),
		},
		cookies: &Cookie{},
		path:    &PathParam{},

		userRequestHooks:     []RequestHook{},
		builtinRequestHooks:  []RequestHook{parserRequestURL, parserRequestHeader, parserRequestBody},
		finalRequestHooks:    []RequestHook{},
		userResponseHooks:    []ResponseHook{},
		builtinResponseHooks: []ResponseHook{parserResponseCookie, logger},
		jsonMarshal:          json.Marshal,
		jsonUnmarshal:        json.Unmarshal,
		xmlMarshal:           xml.Marshal,
		cborMarshal:          cbor.Marshal,
		cborUnmarshal:        cbor.Unmarshal,
		xmlUnmarshal:         xml.Unmarshal,
		logger:               log.DefaultLogger[*stdlog.Logger](),
	}
}

// C returns the default client.
func C() *Client {
	return defaultClient.Load()
}

// Replace replaces the defaultClient with a new one, returning a function to restore the old client.
func Replace(c *Client) func() {
	replaceMu.Lock()
	defer replaceMu.Unlock()

	oldClient := defaultClient.Swap(c)

	return func() {
		replaceMu.Lock()
		defer replaceMu.Unlock()

		defaultClient.Store(oldClient)
	}
}

// Get sends a GET request using the default client.
func Get(url string, cfg ...Config) (*Response, error) {
	return C().Get(url, cfg...)
}

// Post sends a POST request using the default client.
func Post(url string, cfg ...Config) (*Response, error) {
	return C().Post(url, cfg...)
}

// Head sends a HEAD request using the default client.
func Head(url string, cfg ...Config) (*Response, error) {
	return C().Head(url, cfg...)
}

// Put sends a PUT request using the default client.
func Put(url string, cfg ...Config) (*Response, error) {
	return C().Put(url, cfg...)
}

// Delete sends a DELETE request using the default client.
func Delete(url string, cfg ...Config) (*Response, error) {
	return C().Delete(url, cfg...)
}

// Options sends an OPTIONS request using the default client.
func Options(url string, cfg ...Config) (*Response, error) {
	return C().Options(url, cfg...)
}

// Patch sends a PATCH request using the default client.
func Patch(url string, cfg ...Config) (*Response, error) {
	return C().Patch(url, cfg...)
}

// Query sends a QUERY request using the default client.
func Query(url string, cfg ...Config) (*Response, error) {
	return C().Query(url, cfg...)
}
