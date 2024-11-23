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

	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/utils/v2"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
)

var ErrFailedToAppendCert = errors.New("failed to append certificate")

// The Client is used to create a Fiber Client with
// client-level settings that apply to all requests
// raise from the client.
//
// Fiber Client also provides an option to override
// or merge most of the client settings at the request.
type Client struct {
	// logger
	logger log.CommonLogger

	fasthttp *fasthttp.Client

	header  *Header
	params  *QueryParam
	cookies *Cookie
	path    *PathParam

	jsonMarshal   utils.JSONMarshal
	jsonUnmarshal utils.JSONUnmarshal
	xmlMarshal    utils.XMLMarshal
	xmlUnmarshal  utils.XMLUnmarshal

	cookieJar *CookieJar

	// retry
	retryConfig *RetryConfig

	baseURL   string
	userAgent string
	referer   string

	// user defined request hooks
	userRequestHooks []RequestHook

	// client package defined request hooks
	builtinRequestHooks []RequestHook

	// user defined response hooks
	userResponseHooks []ResponseHook

	// client package defined response hooks
	builtinResponseHooks []ResponseHook

	timeout time.Duration

	mu sync.RWMutex

	debug bool
}

// R raise a request from the client.
func (c *Client) R() *Request {
	return AcquireRequest().SetClient(c)
}

// RequestHook Request returns user-defined request hooks.
func (c *Client) RequestHook() []RequestHook {
	return c.userRequestHooks
}

// AddRequestHook Add user-defined request hooks.
func (c *Client) AddRequestHook(h ...RequestHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userRequestHooks = append(c.userRequestHooks, h...)
	return c
}

// ResponseHook return user-define response hooks.
func (c *Client) ResponseHook() []ResponseHook {
	return c.userResponseHooks
}

// AddResponseHook Add user-defined response hooks.
func (c *Client) AddResponseHook(h ...ResponseHook) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.userResponseHooks = append(c.userResponseHooks, h...)
	return c
}

// JSONMarshal returns json marshal function in Core.
func (c *Client) JSONMarshal() utils.JSONMarshal {
	return c.jsonMarshal
}

// SetJSONMarshal sets the JSON encoder.
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client {
	c.jsonMarshal = f
	return c
}

// JSONUnmarshal returns json unmarshal function in Core.
func (c *Client) JSONUnmarshal() utils.JSONUnmarshal {
	return c.jsonUnmarshal
}

// Set json decoder.
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client {
	c.jsonUnmarshal = f
	return c
}

// XMLMarshal returns xml marshal function in Core.
func (c *Client) XMLMarshal() utils.XMLMarshal {
	return c.xmlMarshal
}

// SetXMLMarshal Set xml encoder.
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client {
	c.xmlMarshal = f
	return c
}

// XMLUnmarshal returns xml unmarshal function in Core.
func (c *Client) XMLUnmarshal() utils.XMLUnmarshal {
	return c.xmlUnmarshal
}

// SetXMLUnmarshal Set xml decoder.
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client {
	c.xmlUnmarshal = f
	return c
}

// TLSConfig returns tlsConfig in client.
// If client don't have tlsConfig, this function will init it.
func (c *Client) TLSConfig() *tls.Config {
	if c.fasthttp.TLSConfig == nil {
		c.fasthttp.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return c.fasthttp.TLSConfig
}

// SetTLSConfig sets tlsConfig in client.
func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	c.fasthttp.TLSConfig = config
	return c
}

// SetCertificates method sets client certificates into client.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	config := c.TLSConfig()
	config.Certificates = append(config.Certificates, certs...)
	return c
}

// SetRootCertificate adds one or more root certificates into client.
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

// SetRootCertificateFromString method adds one or more root certificates into client.
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

// SetProxyURL sets proxy url in client. It will apply via core to hostclient.
func (c *Client) SetProxyURL(proxyURL string) error {
	c.fasthttp.Dial = fasthttpproxy.FasthttpHTTPDialer(proxyURL)

	return nil
}

// RetryConfig returns retry config in client.
func (c *Client) RetryConfig() *RetryConfig {
	return c.retryConfig
}

// SetRetryConfig sets retry config in client which is impl by addon/retry package.
func (c *Client) SetRetryConfig(config *RetryConfig) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.retryConfig = config
	return c
}

// BaseURL returns baseurl in Client instance.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// SetBaseURL Set baseUrl which is prefix of real url.
func (c *Client) SetBaseURL(url string) *Client {
	c.baseURL = url
	return c
}

// Header method returns header value via key,
// this method will visit all field in the header,
// then sort them.
func (c *Client) Header(key string) []string {
	return c.header.PeekMultiple(key)
}

// AddHeader method adds a single header field and its value in the client instance.
// These headers will be applied to all requests raised from this client instance.
// Also, it can be overridden at request level header options.
func (c *Client) AddHeader(key, val string) *Client {
	c.header.Add(key, val)
	return c
}

// SetHeader method sets a single header field and its value in the client instance.
// These headers will be applied to all requests raised from this client instance.
// Also, it can be overridden at request level header options.
func (c *Client) SetHeader(key, val string) *Client {
	c.header.Set(key, val)
	return c
}

// AddHeaders method adds multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests raised from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) AddHeaders(h map[string][]string) *Client {
	c.header.AddHeaders(h)
	return c
}

// SetHeaders method sets multiple headers field and its values at one go in the client instance.
// These headers will be applied to all requests raised from this client instance. Also it can be
// overridden at request level headers options.
func (c *Client) SetHeaders(h map[string]string) *Client {
	c.header.SetHeaders(h)
	return c
}

// Param method returns params value via key,
// this method will visit all field in the query param.
func (c *Client) Param(key string) []string {
	res := []string{}
	tmp := c.params.PeekMulti(key)
	for _, v := range tmp {
		res = append(res, utils.UnsafeString(v))
	}

	return res
}

// AddParam method adds a single query param field and its value in the client instance.
// These params will be applied to all requests raised from this client instance.
// Also, it can be overridden at request level param options.
func (c *Client) AddParam(key, val string) *Client {
	c.params.Add(key, val)
	return c
}

// SetParam method sets a single query param field and its value in the client instance.
// These params will be applied to all requests raised from this client instance.
// Also, it can be overridden at request level param options.
func (c *Client) SetParam(key, val string) *Client {
	c.params.Set(key, val)
	return c
}

// AddParams method adds multiple query params field and its values at one go in the client instance.
// These params will be applied to all requests raised from this client instance. Also it can be
// overridden at request level params options.
func (c *Client) AddParams(m map[string][]string) *Client {
	c.params.AddParams(m)
	return c
}

// SetParams method sets multiple params field and its values at one go in the client instance.
// These params will be applied to all requests raised from this client instance. Also it can be
// overridden at request level params options.
func (c *Client) SetParams(m map[string]string) *Client {
	c.params.SetParams(m)
	return c
}

// SetParamsWithStruct method sets multiple params field and its values at one go in the client instance.
// These params will be applied to all requests raised from this client instance. Also it can be
// overridden at request level params options.
func (c *Client) SetParamsWithStruct(v any) *Client {
	c.params.SetParamsWithStruct(v)
	return c
}

// DelParams method deletes single or multiple params field and its values in client.
func (c *Client) DelParams(key ...string) *Client {
	for _, v := range key {
		c.params.Del(v)
	}
	return c
}

// SetUserAgent method sets userAgent field and its value in the client instance.
// This ua will be applied to all requests raised from this client instance.
// Also it can be overridden at request level ua options.
func (c *Client) SetUserAgent(ua string) *Client {
	c.userAgent = ua
	return c
}

// SetReferer method sets referer field and its value in the client instance.
// This referer will be applied to all requests raised from this client instance.
// Also it can be overridden at request level referer options.
func (c *Client) SetReferer(r string) *Client {
	c.referer = r
	return c
}

// PathParam returns the path param be set in request instance.
// if path param doesn't exist, return empty string.
func (c *Client) PathParam(key string) string {
	if val, ok := (*c.path)[key]; ok {
		return val
	}

	return ""
}

// SetPathParam method sets a single path param field and its value in the client instance.
// These path params will be applied to all requests raised from this client instance.
// Also it can be overridden at request level path params options.
func (c *Client) SetPathParam(key, val string) *Client {
	c.path.SetParam(key, val)
	return c
}

// SetPathParams method sets multiple path params field and its values at one go in the client instance.
// These path params will be applied to all requests raised from this client instance. Also it can be
// overridden at request level path params options.
func (c *Client) SetPathParams(m map[string]string) *Client {
	c.path.SetParams(m)
	return c
}

// SetPathParamsWithStruct method sets multiple path params field and its values at one go in the client instance.
// These path params will be applied to all requests raised from this client instance. Also it can be
// overridden at request level path params options.
func (c *Client) SetPathParamsWithStruct(v any) *Client {
	c.path.SetParamsWithStruct(v)
	return c
}

// DelPathParams method deletes single or multiple path params field and its values in client.
func (c *Client) DelPathParams(key ...string) *Client {
	c.path.DelParams(key...)
	return c
}

// Cookie returns the cookie be set in request instance.
// if cookie doesn't exist, return empty string.
func (c *Client) Cookie(key string) string {
	if val, ok := (*c.cookies)[key]; ok {
		return val
	}
	return ""
}

// SetCookie method sets a single cookie field and its value in the client instance.
// These cookies will be applied to all requests raised from this client instance.
// Also it can be overridden at request level cookie options.
func (c *Client) SetCookie(key, val string) *Client {
	c.cookies.SetCookie(key, val)
	return c
}

// SetCookies method sets multiple cookies field and its values at one go in the client instance.
// These cookies will be applied to all requests raised from this client instance. Also it can be
// overridden at request level cookie options.
func (c *Client) SetCookies(m map[string]string) *Client {
	c.cookies.SetCookies(m)
	return c
}

// SetCookiesWithStruct method sets multiple cookies field and its values at one go in the client instance.
// These cookies will be applied to all requests raised from this client instance. Also it can be
// overridden at request level cookies options.
func (c *Client) SetCookiesWithStruct(v any) *Client {
	c.cookies.SetCookiesWithStruct(v)
	return c
}

// DelCookies method deletes single or multiple cookies field and its values in client.
func (c *Client) DelCookies(key ...string) *Client {
	c.cookies.DelCookies(key...)
	return c
}

// SetTimeout method sets timeout val in client instance.
// This value will be applied to all requests raised from this client instance.
// Also, it can be overridden at request level timeout options.
func (c *Client) SetTimeout(t time.Duration) *Client {
	c.timeout = t
	return c
}

// Debug enable log debug level output.
func (c *Client) Debug() *Client {
	c.debug = true
	return c
}

// DisableDebug disables log debug level output.
func (c *Client) DisableDebug() *Client {
	c.debug = false
	return c
}

// SetCookieJar sets cookie jar in client instance.
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client {
	c.cookieJar = cookieJar
	return c
}

// Get provide an API like axios which send get request.
func (c *Client) Get(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Get(url)
}

// Post provide an API like axios which send post request.
func (c *Client) Post(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Post(url)
}

// Head provide a API like axios which send head request.
func (c *Client) Head(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Head(url)
}

// Put provide an API like axios which send put request.
func (c *Client) Put(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Put(url)
}

// Delete provide an API like axios which send delete request.
func (c *Client) Delete(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Delete(url)
}

// Options provide an API like axios which send options request.
func (c *Client) Options(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Options(url)
}

// Patch provide an API like axios which send patch request.
func (c *Client) Patch(url string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Patch(url)
}

// Custom provide an API like axios which send custom request.
func (c *Client) Custom(url, method string, cfg ...Config) (*Response, error) {
	req := AcquireRequest().SetClient(c)
	setConfigToRequest(req, cfg...)

	return req.Custom(url, method)
}

// SetDial sets dial function in client.
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fasthttp.Dial = dial
	return c
}

// SetLogger sets logger instance in client.
func (c *Client) SetLogger(logger log.CommonLogger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger = logger
	return c
}

// Logger returns logger instance of client.
func (c *Client) Logger() log.CommonLogger {
	return c.logger
}

// Reset clears the Client object
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

// Config for easy to set the request parameters, it should be
// noted that when setting the request body will use JSON as
// the default serialization mechanism, while the priority of
// Body is higher than FormData, and the priority of FormData
// is higher than File.
type Config struct {
	Ctx context.Context //nolint:containedctx // It's needed to be stored in the config.

	Body      any
	Header    map[string]string
	Param     map[string]string
	Cookie    map[string]string
	PathParam map[string]string

	FormData map[string]string

	UserAgent string
	Referer   string
	File      []*File

	Timeout      time.Duration
	MaxRedirects int
}

// setConfigToRequest Set the parameters passed via Config to Request.
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
		req.SetFormDatas(cfg.FormData)
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

// init acquire a default client.
func init() {
	defaultClient = New()
}

// New creates and returns a new Client object.
func New() *Client {
	// FOllOW-UP performance optimization
	// trie to use a pool to reduce the cost of memory allocation
	// for the fiber client and the fasthttp client
	// if possible also for other structs -> request header, cookie, query param, path param...
	return NewWithClient(&fasthttp.Client{})
}

// NewWithClient creates and returns a new Client object from an eisting client object.
func NewWithClient(c *fasthttp.Client) *Client {
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
		xmlUnmarshal:         xml.Unmarshal,
		logger:               log.DefaultLogger(),
	}
}

// C get default client.
func C() *Client {
	return defaultClient
}

// Replace the defaultClient, the returned function can undo.
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

// Get send a get request use defaultClient, a convenient method.
func Get(url string, cfg ...Config) (*Response, error) {
	return C().Get(url, cfg...)
}

// Post send a post request use defaultClient, a convenient method.
func Post(url string, cfg ...Config) (*Response, error) {
	return C().Post(url, cfg...)
}

// Head send a head request use defaultClient, a convenient method.
func Head(url string, cfg ...Config) (*Response, error) {
	return C().Head(url, cfg...)
}

// Put send a put request use defaultClient, a convenient method.
func Put(url string, cfg ...Config) (*Response, error) {
	return C().Put(url, cfg...)
}

// Delete send a delete request use defaultClient, a convenient method.
func Delete(url string, cfg ...Config) (*Response, error) {
	return C().Delete(url, cfg...)
}

// Options send a options request use defaultClient, a convenient method.
func Options(url string, cfg ...Config) (*Response, error) {
	return C().Options(url, cfg...)
}

// Patch send a patch request use defaultClient, a convenient method.
func Patch(url string, cfg ...Config) (*Response, error) {
	return C().Patch(url, cfg...)
}
