package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/utils/v2"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

var ErrFailedToAppendCert = errors.New("failed to append certificate")

// Client is used to create a Fiber client with client-level settings that
// apply to all requests made by the client.
//
// The Fiber client also provides an option to override or merge most of the
// client settings at the request level.
type Client struct {
	logger   log.CommonLogger
	fasthttp *fasthttp.Client

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

	cookieJar            *CookieJar
	retryConfig          *RetryConfig
	baseURL              string
	userAgent            string
	referer              string
	userRequestHooks     []RequestHook
	builtinRequestHooks  []RequestHook
	userResponseHooks    []ResponseHook
	builtinResponseHooks []ResponseHook

	timeout time.Duration
	mu      sync.RWMutex
	debug   bool
}

// R creates a new Request associated with the client.
func (c *Client) R() *Request {
	return AcquireRequest().SetClient(c)
}

// RequestHook returns the user-defined request hooks.
func (c *Client) RequestHook() []RequestHook {
	return c.userRequestHooks
}

// AddRequestHook adds user-defined request hooks.
func (c *Client) AddRequestHook(h ...RequestHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userRequestHooks = append(c.userRequestHooks, h...)
	return c
}

// ResponseHook returns the user-defined response hooks.
func (c *Client) ResponseHook() []ResponseHook {
	return c.userResponseHooks
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
	return c.jsonMarshal
}

// SetJSONMarshal sets the JSON marshal function to use.
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client {
	c.jsonMarshal = f
	return c
}

// JSONUnmarshal returns the JSON unmarshal function used by the client.
func (c *Client) JSONUnmarshal() utils.JSONUnmarshal {
	return c.jsonUnmarshal
}

// SetJSONUnmarshal sets the JSON unmarshal function to use.
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client {
	c.jsonUnmarshal = f
	return c
}

// XMLMarshal returns the XML marshal function used by the client.
func (c *Client) XMLMarshal() utils.XMLMarshal {
	return c.xmlMarshal
}

// SetXMLMarshal sets the XML marshal function to use.
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client {
	c.xmlMarshal = f
	return c
}

// XMLUnmarshal returns the XML unmarshal function used by the client.
func (c *Client) XMLUnmarshal() utils.XMLUnmarshal {
	return c.xmlUnmarshal
}

// SetXMLUnmarshal sets the XML unmarshal function to use.
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client {
	c.xmlUnmarshal = f
	return c
}

// CBORMarshal returns the CBOR marshal function used by the client.
func (c *Client) CBORMarshal() utils.CBORMarshal {
	return c.cborMarshal
}

// SetCBORMarshal sets the CBOR marshal function to use.
func (c *Client) SetCBORMarshal(f utils.CBORMarshal) *Client {
	c.cborMarshal = f
	return c
}

// CBORUnmarshal returns the CBOR unmarshal function used by the client.
func (c *Client) CBORUnmarshal() utils.CBORUnmarshal {
	return c.cborUnmarshal
}

// SetCBORUnmarshal sets the CBOR unmarshal function to use.
func (c *Client) SetCBORUnmarshal(f utils.CBORUnmarshal) *Client {
	c.cborUnmarshal = f
	return c
}

// TLSConfig returns the client's TLS configuration.
// If none is set, it initializes a new one.
func (c *Client) TLSConfig() *tls.Config {
	if c.fasthttp.TLSConfig == nil {
		c.fasthttp.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return c.fasthttp.TLSConfig
}

// SetTLSConfig sets the TLS configuration for the client.
func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	c.fasthttp.TLSConfig = config
	return c
}

// SetCertificates adds certificates to the client's TLS configuration.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	config := c.TLSConfig()
	config.Certificates = append(config.Certificates, certs...)
	return c
}

// SetRootCertificate adds one or more root certificates to the client's TLS configuration.
func (c *Client) SetRootCertificate(path string) *Client {
	cleanPath := filepath.Clean(path)
	file, err := os.Open(cleanPath)
	if err != nil {
		c.logger.Panicf("client: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			c.logger.Panicf("client: failed to close file: %v", err)
		}
	}()

	pem, err := io.ReadAll(file)
	if err != nil {
		c.logger.Panicf("client: %v", err)
	}

	config := c.TLSConfig()
	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	if !config.RootCAs.AppendCertsFromPEM(pem) {
		c.logger.Panicf("client: %v", ErrFailedToAppendCert)
	}

	return c
}

// SetRootCertificateFromString adds one or more root certificates from a string to the client's TLS configuration.
func (c *Client) SetRootCertificateFromString(pem string) *Client {
	config := c.TLSConfig()

	if config.RootCAs == nil {
		config.RootCAs = x509.NewCertPool()
	}

	if !config.RootCAs.AppendCertsFromPEM([]byte(pem)) {
		c.logger.Panicf("client: %v", ErrFailedToAppendCert)
	}

	return c
}

// SetProxyURL sets the proxy URL for the client. This affects all subsequent requests.
func (c *Client) SetProxyURL(proxyURL string) error {
	c.fasthttp.Dial = fasthttpproxy.FasthttpHTTPDialer(proxyURL)
	return nil
}

// RetryConfig returns the current retry configuration.
func (c *Client) RetryConfig() *RetryConfig {
	return c.retryConfig
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
	return c.baseURL
}

// SetBaseURL sets the base URL prefix for all requests made by the client.
func (c *Client) SetBaseURL(url string) *Client {
	c.baseURL = url
	return c
}

// Header returns all header values associated with the provided key.
func (c *Client) Header(key string) []string {
	return c.header.PeekMultiple(key)
}

// AddHeader adds a single header field and its value to the client. These headers apply to all requests.
func (c *Client) AddHeader(key, val string) *Client {
	c.header.Add(key, val)
	return c
}

// SetHeader sets a single header field and its value in the client.
func (c *Client) SetHeader(key, val string) *Client {
	c.header.Set(key, val)
	return c
}

// AddHeaders adds multiple header fields and their values to the client.
func (c *Client) AddHeaders(h map[string][]string) *Client {
	c.header.AddHeaders(h)
	return c
}

// SetHeaders method sets multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests created from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) SetHeaders(h map[string]string) *Client {
	c.header.SetHeaders(h)
	return c
}

// Param returns all values of the specified query parameter.
func (c *Client) Param(key string) []string {
	res := []string{}
	tmp := c.params.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}

	return res
}

// AddParam adds a single query parameter and its value to the client.
// These params will be applied to all requests created from this client instance.
func (c *Client) AddParam(key, val string) *Client {
	c.params.Add(key, val)
	return c
}

// SetParam sets a single query parameter and its value in the client.
func (c *Client) SetParam(key, val string) *Client {
	c.params.Set(key, val)
	return c
}

// AddParams adds multiple query parameters and their values to the client.
func (c *Client) AddParams(m map[string][]string) *Client {
	c.params.AddParams(m)
	return c
}

// SetParams sets multiple query parameters and their values in the client.
func (c *Client) SetParams(m map[string]string) *Client {
	c.params.SetParams(m)
	return c
}

// SetParamsWithStruct sets multiple query parameters and their values using a struct.
func (c *Client) SetParamsWithStruct(v any) *Client {
	c.params.SetParamsWithStruct(v)
	return c
}

// DelParams deletes one or more query parameters and their values from the client.
func (c *Client) DelParams(key ...string) *Client {
	for _, v := range key {
		c.params.Del(v)
	}
	return c
}

// SetUserAgent sets the User-Agent header for the client.
func (c *Client) SetUserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

// SetReferer sets the Referer header for the client.
func (c *Client) SetReferer(r string) *Client {
	c.referer = r
	return c
}

// PathParam returns the value of the specified path parameter. Returns an empty string if it does not exist.
func (c *Client) PathParam(key string) string {
	if val, ok := (*c.path)[key]; ok {
		return val
	}
	return ""
}

// SetPathParam sets a single path parameter and its value in the client.
func (c *Client) SetPathParam(key, val string) *Client {
	c.path.SetParam(key, val)
	return c
}

// SetPathParams sets multiple path parameters and their values in the client.
func (c *Client) SetPathParams(m map[string]string) *Client {
	c.path.SetParams(m)
	return c
}

// SetPathParamsWithStruct sets multiple path parameters and their values using a struct.
func (c *Client) SetPathParamsWithStruct(v any) *Client {
	c.path.SetParamsWithStruct(v)
	return c
}

// DelPathParams deletes one or more path parameters and their values from the client.
func (c *Client) DelPathParams(key ...string) *Client {
	c.path.DelParams(key...)
	return c
}

// Cookie returns the value of the specified cookie. Returns an empty string if it does not exist.
func (c *Client) Cookie(key string) string {
	if val, ok := (*c.cookies)[key]; ok {
		return val
	}
	return ""
}

// SetCookie sets a single cookie and its value in the client.
func (c *Client) SetCookie(key, val string) *Client {
	c.cookies.SetCookie(key, val)
	return c
}

// SetCookies sets multiple cookies and their values in the client.
func (c *Client) SetCookies(m map[string]string) *Client {
	c.cookies.SetCookies(m)
	return c
}

// SetCookiesWithStruct sets multiple cookies and their values using a struct.
func (c *Client) SetCookiesWithStruct(v any) *Client {
	c.cookies.SetCookiesWithStruct(v)
	return c
}

// DelCookies deletes one or more cookies and their values from the client.
func (c *Client) DelCookies(key ...string) *Client {
	c.cookies.DelCookies(key...)
	return c
}

// SetTimeout sets the timeout value for the client. This applies to all requests unless overridden at the request level.
func (c *Client) SetTimeout(t time.Duration) *Client {
	c.timeout = t
	return c
}

// Debug enables debug-level logging output.
func (c *Client) Debug() *Client {
	c.debug = true
	return c
}

// DisableDebug disables debug-level logging output.
func (c *Client) DisableDebug() *Client {
	c.debug = false
	return c
}

// SetCookieJar sets the cookie jar for the client.
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client {
	c.cookieJar = cookieJar
	return c
}

// Get sends a GET request to the specified URL, similar to axios.
func (c *Client) Get(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Get(url)
}

// Post sends a POST request to the specified URL, similar to axios.
func (c *Client) Post(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Post(url)
}

// Head sends a HEAD request to the specified URL, similar to axios.
func (c *Client) Head(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Head(url)
}

// Put sends a PUT request to the specified URL, similar to axios.
func (c *Client) Put(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Put(url)
}

// Delete sends a DELETE request to the specified URL, similar to axios.
func (c *Client) Delete(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Delete(url)
}

// Options sends an OPTIONS request to the specified URL, similar to axios.
func (c *Client) Options(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Options(url)
}

// Patch sends a PATCH request to the specified URL, similar to axios.
func (c *Client) Patch(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Patch(url)
}

// Custom sends a request with a custom method to the specified URL, similar to axios.
func (c *Client) Custom(url, method string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)
	return req.Custom(url, method)
}

// SetDial sets the custom dial function for the client.
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fasthttp.Dial = dial
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
	return c.logger
}

// Reset resets the client to its default state, clearing most configurations.
func (c *Client) Reset() {
	c.fasthttp = &fasthttp.Client{}
	c.baseURL = ""
	c.timeout = 0
	c.userAgent = ""
	c.referer = ""
	c.retryConfig = nil
	c.debug = false

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
type Config struct {
	Ctx          context.Context //nolint:containedctx // It's needed to be stored in the config.
	Body         any
	Header       map[string]string
	Param        map[string]string
	Cookie       map[string]string
	PathParam    map[string]string
	FormData     map[string]string
	UserAgent    string
	Referer      string
	File         []*File
	Timeout      time.Duration
	MaxRedirects int
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

	if cfg.Body != nil {
		req.SetJSON(cfg.Body)
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
	defaultClient    *Client
	replaceMu        = sync.Mutex{}
	defaultUserAgent = "fiber"
)

func init() {
	defaultClient = New()
}

// New creates and returns a new Client object.
func New() *Client {
	// Follow-up performance optimizations:
	// Try to use a pool to reduce the memory allocation cost for the Fiber client and the fasthttp client.
	// If possible, also consider pooling other structs (e.g., request headers, cookies, query parameters, path parameters).
	return NewWithClient(&fasthttp.Client{})
}

// NewWithClient creates and returns a new Client object from an existing fasthttp.Client.
func NewWithClient(c *fasthttp.Client) *Client {
	if c == nil {
		panic("fasthttp.Client must not be nil")
	}
	return &Client{
		fasthttp: c,
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
		userResponseHooks:    []ResponseHook{},
		builtinResponseHooks: []ResponseHook{parserResponseCookie, logger},
		jsonMarshal:          json.Marshal,
		jsonUnmarshal:        json.Unmarshal,
		xmlMarshal:           xml.Marshal,
		cborMarshal:          cbor.Marshal,
		cborUnmarshal:        cbor.Unmarshal,
		xmlUnmarshal:         xml.Unmarshal,
		logger:               log.DefaultLogger(),
	}
}

// C returns the default client.
func C() *Client {
	return defaultClient
}

// Replace replaces the defaultClient with a new one, returning a function to restore the old client.
func Replace(c *Client) func() {
	replaceMu.Lock()
	defer replaceMu.Unlock()

	oldClient := defaultClient
	defaultClient = c

	return func() {
		replaceMu.Lock()
		defer replaceMu.Unlock()

		defaultClient = oldClient
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
