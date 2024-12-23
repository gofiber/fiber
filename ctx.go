// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// maxParams defines the maximum number of parameters per route.
const maxParams = 30

// The contextKey type is unexported to prevent collisions with context keys defined in
// other packages.
type contextKey int

// userContextKey define the key name for storing context.Context in *fasthttp.RequestCtx
const userContextKey contextKey = 0 // __local_user_context__

// DefaultCtx is the default implementation of the Ctx interface
// generation tool `go install github.com/vburenin/ifacemaker@975a95966976eeb2d4365a7fb236e274c54da64c`
// https://github.com/vburenin/ifacemaker/blob/975a95966976eeb2d4365a7fb236e274c54da64c/ifacemaker.go#L14-L30
//
//go:generate ifacemaker --file ctx.go --struct DefaultCtx --iface Ctx --pkg fiber --output ctx_interface_gen.go --not-exported true --iface-comment "Ctx represents the Context which hold the HTTP request and response.\nIt has methods for the request query string, parameters, body, HTTP headers and so on."
type DefaultCtx struct {
	app                 *App                 // Reference to *App
	route               *Route               // Reference to *Route
	fasthttp            *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	bind                *Bind                // Default bind reference
	redirect            *Redirect            // Default redirect reference
	values              [maxParams]string    // Route parameter values
	viewBindMap         sync.Map             // Default view map to bind template engine
	method              string               // HTTP method
	baseURI             string               // HTTP base uri
	path                string               // HTTP path with the modifications by the configuration -> string copy from pathBuffer
	detectionPath       string               // Route detection path                                  -> string copy from detectionPathBuffer
	treePath            string               // Path for the search in the tree
	pathOriginal        string               // Original HTTP path
	pathBuffer          []byte               // HTTP path buffer
	detectionPathBuffer []byte               // HTTP detectionPath buffer
	flashMessages       redirectionMsgs      // Flash messages
	indexRoute          int                  // Index of the current route
	indexHandler        int                  // Index of the current handler
	methodINT           int                  // HTTP method INT equivalent
	matched             bool                 // Non use route matched
}

// SendFile defines configuration options when to transfer file with SendFile.
type SendFile struct {
	// FS is the file system to serve the static files from.
	// You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc.
	//
	// Optional. Default: nil
	FS fs.FS

	// When set to true, the server tries minimizing CPU usage by caching compressed files.
	// This works differently than the github.com/gofiber/compression middleware.
	// You have to set Content-Encoding header to compress the file.
	// Available compression methods are gzip, br, and zstd.
	//
	// Optional. Default: false
	Compress bool `json:"compress"`

	// When set to true, enables byte range requests.
	//
	// Optional. Default: false
	ByteRange bool `json:"byte_range"`

	// When set to true, enables direct download.
	//
	// Optional. Default: false
	Download bool `json:"download"`

	// Expiration duration for inactive file handlers.
	// Use a negative time.Duration to disable it.
	//
	// Optional. Default: 10 * time.Second
	CacheDuration time.Duration `json:"cache_duration"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default: 0
	MaxAge int `json:"max_age"`
}

// sendFileStore is used to keep the SendFile configuration and the handler.
type sendFileStore struct {
	handler           fasthttp.RequestHandler
	cacheControlValue string
	config            SendFile
}

// compareConfig compares the current SendFile config with the new one
// and returns true if they are different.
//
// Here we don't use reflect.DeepEqual because it is quite slow compared to manual comparison.
func (sf *sendFileStore) compareConfig(cfg SendFile) bool {
	if sf.config.FS != cfg.FS {
		return false
	}

	if sf.config.Compress != cfg.Compress {
		return false
	}

	if sf.config.ByteRange != cfg.ByteRange {
		return false
	}

	if sf.config.Download != cfg.Download {
		return false
	}

	if sf.config.CacheDuration != cfg.CacheDuration {
		return false
	}

	if sf.config.MaxAge != cfg.MaxAge {
		return false
	}

	return true
}

// TLSHandler object
type TLSHandler struct {
	clientHelloInfo *tls.ClientHelloInfo
}

// GetClientInfo Callback function to set ClientHelloInfo
// Must comply with the method structure of https://cs.opensource.google/go/go/+/refs/tags/go1.20:src/crypto/tls/common.go;l=554-563
// Since we overlay the method of the TLS config in the listener method
func (t *TLSHandler) GetClientInfo(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	t.clientHelloInfo = info
	return nil, nil //nolint:nilnil // Not returning anything useful here is probably fine
}

// Range data for c.Range
type Range struct {
	Type   string
	Ranges []RangeSet
}

// RangeSet represents a single content range from a request.
type RangeSet struct {
	Start int
	End   int
}

// Cookie data for c.Cookie
type Cookie struct {
	Expires     time.Time `json:"expires"`      // The expiration date of the cookie
	Name        string    `json:"name"`         // The name of the cookie
	Value       string    `json:"value"`        // The value of the cookie
	Path        string    `json:"path"`         // Specifies a URL path which is allowed to receive the cookie
	Domain      string    `json:"domain"`       // Specifies the domain which is allowed to receive the cookie
	SameSite    string    `json:"same_site"`    // Controls whether or not a cookie is sent with cross-site requests
	MaxAge      int       `json:"max_age"`      // The maximum age (in seconds) of the cookie
	Secure      bool      `json:"secure"`       // Indicates that the cookie should only be transmitted over a secure HTTPS connection
	HTTPOnly    bool      `json:"http_only"`    // Indicates that the cookie is accessible only through the HTTP protocol
	Partitioned bool      `json:"partitioned"`  // Indicates if the cookie is stored in a partitioned cookie jar
	SessionOnly bool      `json:"session_only"` // Indicates if the cookie is a session-only cookie
}

// Views is the interface that wraps the Render function.
type Views interface {
	Load() error
	Render(out io.Writer, name string, binding any, layout ...string) error
}

// ResFmt associates a Content Type to a fiber.Handler for c.Format
type ResFmt struct {
	Handler   func(Ctx) error
	MediaType string
}

// Accepts checks if the specified extensions or content types are acceptable.
func (c *DefaultCtx) Accepts(offers ...string) string {
	return getOffer(c.fasthttp.Request.Header.Peek(HeaderAccept), acceptsOfferType, offers...)
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (c *DefaultCtx) AcceptsCharsets(offers ...string) string {
	return getOffer(c.fasthttp.Request.Header.Peek(HeaderAcceptCharset), acceptsOffer, offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (c *DefaultCtx) AcceptsEncodings(offers ...string) string {
	return getOffer(c.fasthttp.Request.Header.Peek(HeaderAcceptEncoding), acceptsOffer, offers...)
}

// AcceptsLanguages checks if the specified language is acceptable.
func (c *DefaultCtx) AcceptsLanguages(offers ...string) string {
	return getOffer(c.fasthttp.Request.Header.Peek(HeaderAcceptLanguage), acceptsOffer, offers...)
}

// App returns the *App reference to the instance of the Fiber application
func (c *DefaultCtx) App() *App {
	return c.app
}

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (c *DefaultCtx) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := c.app.getString(c.fasthttp.Response.Header.Peek(field))
	originalH := h
	for _, value := range values {
		if len(h) == 0 {
			h = value
		} else if h != value && !strings.HasPrefix(h, value+",") && !strings.HasSuffix(h, " "+value) &&
			!strings.Contains(h, " "+value+",") {
			h += ", " + value
		}
	}
	if originalH != h {
		c.Set(field, h)
	}
}

// Attachment sets the HTTP response Content-Disposition header field to attachment.
func (c *DefaultCtx) Attachment(filename ...string) {
	if len(filename) > 0 {
		fname := filepath.Base(filename[0])
		c.Type(filepath.Ext(fname))

		c.setCanonical(HeaderContentDisposition, `attachment; filename="`+c.app.quoteString(fname)+`"`)
		return
	}
	c.setCanonical(HeaderContentDisposition, "attachment")
}

// BaseURL returns (protocol + host + base path).
func (c *DefaultCtx) BaseURL() string {
	// TODO: Could be improved: 53.8 ns/op  32 B/op  1 allocs/op
	// Should work like https://codeigniter.com/user_guide/helpers/url_helper.html
	if c.baseURI != "" {
		return c.baseURI
	}
	c.baseURI = c.Scheme() + "://" + c.Host()
	return c.baseURI
}

// BodyRaw contains the raw body submitted in a POST request.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) BodyRaw() []byte {
	return c.getBody()
}

func (c *DefaultCtx) tryDecodeBodyInOrder(
	originalBody *[]byte,
	encodings []string,
) ([]byte, uint8, error) {
	var (
		err             error
		body            []byte
		decodesRealized uint8
	)

	for index, encoding := range encodings {
		decodesRealized++
		switch encoding {
		case StrGzip:
			body, err = c.fasthttp.Request.BodyGunzip()
		case StrBr, StrBrotli:
			body, err = c.fasthttp.Request.BodyUnbrotli()
		case StrDeflate:
			body, err = c.fasthttp.Request.BodyInflate()
		case StrZstd:
			body, err = c.fasthttp.Request.BodyUnzstd()
		default:
			decodesRealized--
			if len(encodings) == 1 {
				body = c.fasthttp.Request.Body()
			}
			return body, decodesRealized, nil
		}

		if err != nil {
			return nil, decodesRealized, err
		}

		// Only execute body raw update if it has a next iteration to try to decode
		if index < len(encodings)-1 && decodesRealized > 0 {
			if index == 0 {
				tempBody := c.fasthttp.Request.Body()
				*originalBody = make([]byte, len(tempBody))
				copy(*originalBody, tempBody)
			}
			c.fasthttp.Request.SetBodyRaw(body)
		}
	}

	return body, decodesRealized, nil
}

// Body contains the raw body submitted in a POST request.
// This method will decompress the body if the 'Content-Encoding' header is provided.
// It returns the original (or decompressed) body data which is valid only within the handler.
// Don't store direct references to the returned data.
// If you need to keep the body's data later, make a copy or use the Immutable option.
func (c *DefaultCtx) Body() []byte {
	var (
		err                error
		body, originalBody []byte
		headerEncoding     string
		encodingOrder      = []string{"", "", ""}
	)

	// Get Content-Encoding header
	headerEncoding = utils.UnsafeString(c.Request().Header.ContentEncoding())

	// If no encoding is provided, return the original body
	if len(headerEncoding) == 0 {
		return c.getBody()
	}

	// Split and get the encodings list, in order to attend the
	// rule defined at: https://www.rfc-editor.org/rfc/rfc9110#section-8.4-5
	encodingOrder = getSplicedStrList(headerEncoding, encodingOrder)
	if len(encodingOrder) == 0 {
		return c.getBody()
	}

	var decodesRealized uint8
	body, decodesRealized, err = c.tryDecodeBodyInOrder(&originalBody, encodingOrder)

	// Ensure that the body will be the original
	if originalBody != nil && decodesRealized > 0 {
		c.fasthttp.Request.SetBodyRaw(originalBody)
	}
	if err != nil {
		return []byte(err.Error())
	}

	if c.app.config.Immutable {
		return utils.CopyBytes(body)
	}
	return body
}

// ClearCookie expires a specific cookie by key on the client side.
// If no key is provided it expires all cookies that came with the request.
func (c *DefaultCtx) ClearCookie(key ...string) {
	if len(key) > 0 {
		for i := range key {
			c.fasthttp.Response.Header.DelClientCookie(key[i])
		}
		return
	}
	c.fasthttp.Request.Header.VisitAllCookie(func(k, _ []byte) {
		c.fasthttp.Response.Header.DelClientCookieBytes(k)
	})
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (c *DefaultCtx) RequestCtx() *fasthttp.RequestCtx {
	return c.fasthttp
}

// Context returns a context implementation that was set by
// user earlier or returns a non-nil, empty context,if it was not set earlier.
func (c *DefaultCtx) Context() context.Context {
	ctx, ok := c.fasthttp.UserValue(userContextKey).(context.Context)
	if !ok {
		ctx = context.Background()
		c.SetContext(ctx)
	}

	return ctx
}

// SetContext sets a context implementation by user.
func (c *DefaultCtx) SetContext(ctx context.Context) {
	c.fasthttp.SetUserValue(userContextKey, ctx)
}

// Cookie sets a cookie by passing a cookie struct.
func (c *DefaultCtx) Cookie(cookie *Cookie) {
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(cookie.Name)
	fcookie.SetValue(cookie.Value)
	fcookie.SetPath(cookie.Path)
	fcookie.SetDomain(cookie.Domain)
	// only set max age and expiry when SessionOnly is false
	// i.e. cookie supposed to last beyond browser session
	// refer: https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies#define_the_lifetime_of_a_cookie
	if !cookie.SessionOnly {
		fcookie.SetMaxAge(cookie.MaxAge)
		fcookie.SetExpire(cookie.Expires)
	}
	fcookie.SetSecure(cookie.Secure)
	fcookie.SetHTTPOnly(cookie.HTTPOnly)

	switch utils.ToLower(cookie.SameSite) {
	case CookieSameSiteStrictMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case CookieSameSiteNoneMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	case CookieSameSiteDisabled:
		fcookie.SetSameSite(fasthttp.CookieSameSiteDisabled)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	// CHIPS allows to partition cookie jar by top-level site.
	// refer: https://developers.google.com/privacy-sandbox/3pcd/chips
	fcookie.SetPartitioned(cookie.Partitioned)

	c.fasthttp.Response.Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) Cookies(key string, defaultValue ...string) string {
	return defaultString(c.app.getString(c.fasthttp.Request.Header.Cookie(key)), defaultValue)
}

// Download transfers the file from path as an attachment.
// Typically, browsers will prompt the user for download.
// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
// Override this default with the filename parameter.
func (c *DefaultCtx) Download(file string, filename ...string) error {
	var fname string
	if len(filename) > 0 {
		fname = filename[0]
	} else {
		fname = filepath.Base(file)
	}
	c.setCanonical(HeaderContentDisposition, `attachment; filename="`+c.app.quoteString(fname)+`"`)
	return c.SendFile(file)
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (c *DefaultCtx) Request() *fasthttp.Request {
	return &c.fasthttp.Request
}

// Response return the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
func (c *DefaultCtx) Response() *fasthttp.Response {
	return &c.fasthttp.Response
}

// Format performs content-negotiation on the Accept HTTP header.
// It uses Accepts to select a proper format and calls the matching
// user-provided handler function.
// If no accepted format is found, and a format with MediaType "default" is given,
// that default handler is called. If no format is found and no default is given,
// StatusNotAcceptable is sent.
func (c *DefaultCtx) Format(handlers ...ResFmt) error {
	if len(handlers) == 0 {
		return ErrNoHandlers
	}

	c.Vary(HeaderAccept)

	if c.Get(HeaderAccept) == "" {
		c.Response().Header.SetContentType(handlers[0].MediaType)
		return handlers[0].Handler(c)
	}

	// Using an int literal as the slice capacity allows for the slice to be
	// allocated on the stack. The number was chosen arbitrarily as an
	// approximation of the maximum number of content types a user might handle.
	// If the user goes over, it just causes allocations, so it's not a problem.
	types := make([]string, 0, 8)
	var defaultHandler Handler
	for _, h := range handlers {
		if h.MediaType == "default" {
			defaultHandler = h.Handler
			continue
		}
		types = append(types, h.MediaType)
	}
	accept := c.Accepts(types...)

	if accept == "" {
		if defaultHandler == nil {
			return c.SendStatus(StatusNotAcceptable)
		}
		return defaultHandler(c)
	}

	for _, h := range handlers {
		if h.MediaType == accept {
			c.Response().Header.SetContentType(h.MediaType)
			return h.Handler(c)
		}
	}

	return fmt.Errorf("%w: format: an Accept was found but no handler was called", errUnreachable)
}

// AutoFormat performs content-negotiation on the Accept HTTP header.
// It uses Accepts to select a proper format.
// The supported content types are text/html, text/plain, application/json, and application/xml.
// For more flexible content negotiation, use Format.
// If the header is not specified or there is no proper format, text/plain is used.
func (c *DefaultCtx) AutoFormat(body any) error {
	// Get accepted content type
	accept := c.Accepts("html", "json", "txt", "xml")
	// Set accepted content type
	c.Type(accept)
	// Type convert provided body
	var b string
	switch val := body.(type) {
	case string:
		b = val
	case []byte:
		b = c.app.getString(val)
	default:
		b = fmt.Sprintf("%v", val)
	}

	// Format based on the accept content type
	switch accept {
	case "html":
		return c.SendString("<p>" + b + "</p>")
	case "json":
		return c.JSON(body)
	case "txt":
		return c.SendString(b)
	case "xml":
		return c.XML(body)
	}
	return c.SendString(b)
}

// FormFile returns the first file by key from a MultipartForm.
func (c *DefaultCtx) FormFile(key string) (*multipart.FileHeader, error) {
	return c.fasthttp.FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
// Search is performed in QueryArgs, PostArgs, MultipartForm and FormFile in this particular order.
// Defaults to the empty string "" if the form value doesn't exist.
// If a default value is given, it will return that value if the form value does not exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) FormValue(key string, defaultValue ...string) string {
	return defaultString(c.app.getString(c.fasthttp.FormValue(key)), defaultValue)
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L33
func (c *DefaultCtx) Fresh() bool {
	// fields
	modifiedSince := c.Get(HeaderIfModifiedSince)
	noneMatch := c.Get(HeaderIfNoneMatch)

	// unconditional request
	if modifiedSince == "" && noneMatch == "" {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://tools.ietf.org/html/rfc2616#section-14.9.4
	cacheControl := c.Get(HeaderCacheControl)
	if cacheControl != "" && isNoCache(cacheControl) {
		return false
	}

	// if-none-match
	if noneMatch != "" && noneMatch != "*" {
		etag := c.app.getString(c.fasthttp.Response.Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if c.app.isEtagStale(etag, c.app.getBytes(noneMatch)) {
			return false
		}

		if modifiedSince != "" {
			lastModified := c.app.getString(c.fasthttp.Response.Header.Peek(HeaderLastModified))
			if lastModified != "" {
				lastModifiedTime, err := http.ParseTime(lastModified)
				if err != nil {
					return false
				}
				modifiedSinceTime, err := http.ParseTime(modifiedSince)
				if err != nil {
					return false
				}
				return lastModifiedTime.Compare(modifiedSinceTime) != 1
			}
		}
	}
	return true
}

// Get returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) Get(key string, defaultValue ...string) string {
	return GetReqHeader(c, key, defaultValue...)
}

// GetReqHeader returns the HTTP request header specified by filed.
// This function is generic and can handle different headers type values.
func GetReqHeader[V GenericType](c Ctx, key string, defaultValue ...V) V {
	var v V
	return genericParseType[V](c.App().getString(c.Request().Header.Peek(key)), v, defaultValue...)
}

// GetRespHeader returns the HTTP response header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetRespHeader(key string, defaultValue ...string) string {
	return defaultString(c.app.getString(c.fasthttp.Response.Header.Peek(key)), defaultValue)
}

// GetRespHeaders returns the HTTP response headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetRespHeaders() map[string][]string {
	headers := make(map[string][]string)
	c.Response().Header.VisitAll(func(k, v []byte) {
		key := c.app.getString(k)
		headers[key] = append(headers[key], c.app.getString(v))
	})
	return headers
}

// GetReqHeaders returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) GetReqHeaders() map[string][]string {
	headers := make(map[string][]string)
	c.Request().Header.VisitAll(func(k, v []byte) {
		key := c.app.getString(k)
		headers[key] = append(headers[key], c.app.getString(v))
	})
	return headers
}

// Host contains the host derived from the X-Forwarded-Host or Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// In a network context, `Host` refers to the combination of a hostname and potentially a port number used for connecting,
// while `Hostname` refers specifically to the name assigned to a device on a network, excluding any port information.
// Example: URL: https://example.com:8080 -> Host: example.com:8080
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Host() string {
	if c.IsProxyTrusted() {
		if host := c.Get(HeaderXForwardedHost); len(host) > 0 {
			commaPos := strings.Index(host, ",")
			if commaPos != -1 {
				return host[:commaPos]
			}
			return host
		}
	}
	return c.app.getString(c.fasthttp.Request.URI().Host())
}

// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header using the c.Host() method.
// Returned value is only valid within the handler. Do not store any references.
// Example: URL: https://example.com:8080 -> Hostname: example.com
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Hostname() string {
	addr, _ := parseAddr(c.Host())

	return addr
}

// Port returns the remote port of the request.
func (c *DefaultCtx) Port() string {
	tcpaddr, ok := c.fasthttp.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(errors.New("failed to type-assert to *net.TCPAddr"))
	}
	return strconv.Itoa(tcpaddr.Port)
}

// IP returns the remote IP address of the request.
// If ProxyHeader and IP Validation is configured, it will parse that header and return the first valid IP address.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) IP() string {
	if c.IsProxyTrusted() && len(c.app.config.ProxyHeader) > 0 {
		return c.extractIPFromHeader(c.app.config.ProxyHeader)
	}

	return c.fasthttp.RemoteIP().String()
}

// extractIPsFromHeader will return a slice of IPs it found given a header name in the order they appear.
// When IP validation is enabled, any invalid IPs will be omitted.
func (c *DefaultCtx) extractIPsFromHeader(header string) []string {
	// TODO: Reuse the c.extractIPFromHeader func somehow in here

	headerValue := c.Get(header)

	// We can't know how many IPs we will return, but we will try to guess with this constant division.
	// Counting ',' makes function slower for about 50ns in general case.
	const maxEstimatedCount = 8
	estimatedCount := len(headerValue) / maxEstimatedCount
	if estimatedCount > maxEstimatedCount {
		estimatedCount = maxEstimatedCount // Avoid big allocation on big header
	}

	ipsFound := make([]string, 0, estimatedCount)

	i := 0
	j := -1

iploop:
	for {
		var v4, v6 bool

		// Manually splitting string without allocating slice, working with parts directly
		i, j = j+1, j+2

		if j > len(headerValue) {
			break
		}

		for j < len(headerValue) && headerValue[j] != ',' {
			if headerValue[j] == ':' {
				v6 = true
			} else if headerValue[j] == '.' {
				v4 = true
			}
			j++
		}

		for i < j && (headerValue[i] == ' ' || headerValue[i] == ',') {
			i++
		}

		s := utils.TrimRight(headerValue[i:j], ' ')

		if c.app.config.EnableIPValidation {
			// Skip validation if IP is clearly not IPv4/IPv6, otherwise validate without allocations
			if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
				continue iploop
			}
		}

		ipsFound = append(ipsFound, s)
	}

	return ipsFound
}

// extractIPFromHeader will attempt to pull the real client IP from the given header when IP validation is enabled.
// currently, it will return the first valid IP address in header.
// when IP validation is disabled, it will simply return the value of the header without any inspection.
// Implementation is almost the same as in extractIPsFromHeader, but without allocation of []string.
func (c *DefaultCtx) extractIPFromHeader(header string) string {
	if c.app.config.EnableIPValidation {
		headerValue := c.Get(header)

		i := 0
		j := -1

	iploop:
		for {
			var v4, v6 bool

			// Manually splitting string without allocating slice, working with parts directly
			i, j = j+1, j+2

			if j > len(headerValue) {
				break
			}

			for j < len(headerValue) && headerValue[j] != ',' {
				if headerValue[j] == ':' {
					v6 = true
				} else if headerValue[j] == '.' {
					v4 = true
				}
				j++
			}

			for i < j && headerValue[i] == ' ' {
				i++
			}

			s := utils.TrimRight(headerValue[i:j], ' ')

			if c.app.config.EnableIPValidation {
				if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
					continue iploop
				}
			}

			return s
		}

		return c.fasthttp.RemoteIP().String()
	}

	// default behavior if IP validation is not enabled is just to return whatever value is
	// in the proxy header. Even if it is empty or invalid
	return c.Get(c.app.config.ProxyHeader)
}

// IPs returns a string slice of IP addresses specified in the X-Forwarded-For request header.
// When IP validation is enabled, only valid IPs are returned.
func (c *DefaultCtx) IPs() []string {
	return c.extractIPsFromHeader(HeaderXForwardedFor)
}

// Is returns the matching content type,
// if the incoming request's Content-Type HTTP header field matches the MIME type specified by the type parameter
func (c *DefaultCtx) Is(extension string) bool {
	extensionHeader := utils.GetMIME(extension)
	if extensionHeader == "" {
		return false
	}

	return strings.HasPrefix(
		utils.TrimLeft(utils.UnsafeString(c.fasthttp.Request.Header.ContentType()), ' '),
		extensionHeader,
	)
}

// JSON converts any interface or string to JSON.
// Array and slice values encode as JSON arrays,
// except that []byte encodes as a base64-encoded string,
// and a nil slice encodes as the null JSON value.
// If the ctype parameter is given, this method will set the
// Content-Type header equal to ctype. If ctype is not given,
// The Content-Type header will be set to application/json.
func (c *DefaultCtx) JSON(data any, ctype ...string) error {
	raw, err := c.app.config.JSONEncoder(data)
	if err != nil {
		return err
	}
	c.fasthttp.Response.SetBodyRaw(raw)
	if len(ctype) > 0 {
		c.fasthttp.Response.Header.SetContentType(ctype[0])
	} else {
		c.fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	}
	return nil
}

// CBOR converts any interface or string to CBOR encoded bytes.
// If the ctype parameter is given, this method will set the
// Content-Type header equal to ctype. If ctype is not given,
// The Content-Type header will be set to application/cbor.
func (c *DefaultCtx) CBOR(data any, ctype ...string) error {
	raw, err := c.app.config.CBOREncoder(data)
	if err != nil {
		return err
	}
	c.fasthttp.Response.SetBodyRaw(raw)
	if len(ctype) > 0 {
		c.fasthttp.Response.Header.SetContentType(ctype[0])
	} else {
		c.fasthttp.Response.Header.SetContentType(MIMEApplicationCBOR)
	}
	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (c *DefaultCtx) JSONP(data any, callback ...string) error {
	raw, err := c.app.config.JSONEncoder(data)
	if err != nil {
		return err
	}

	var result, cb string

	if len(callback) > 0 {
		cb = callback[0]
	} else {
		cb = "callback"
	}

	result = cb + "(" + c.app.getString(raw) + ");"

	c.setCanonical(HeaderXContentTypeOptions, "nosniff")
	c.fasthttp.Response.Header.SetContentType(MIMETextJavaScriptCharsetUTF8)
	return c.SendString(result)
}

// XML converts any interface or string to XML.
// This method also sets the content header to application/xml.
func (c *DefaultCtx) XML(data any) error {
	raw, err := c.app.config.XMLEncoder(data)
	if err != nil {
		return err
	}
	c.fasthttp.Response.SetBodyRaw(raw)
	c.fasthttp.Response.Header.SetContentType(MIMEApplicationXML)
	return nil
}

// Links joins the links followed by the property to populate the response's Link HTTP header field.
func (c *DefaultCtx) Links(link ...string) {
	if len(link) == 0 {
		return
	}
	bb := bytebufferpool.Get()
	for i := range link {
		if i%2 == 0 {
			bb.WriteByte('<')
			bb.WriteString(link[i])
			bb.WriteByte('>')
		} else {
			bb.WriteString(`; rel="` + link[i] + `",`)
		}
	}
	c.setCanonical(HeaderLink, utils.TrimRight(c.app.getString(bb.Bytes()), ','))
	bytebufferpool.Put(bb)
}

// Locals makes it possible to pass any values under keys scoped to the request
// and therefore available to all following routes that match the request.
//
// All the values are removed from ctx after returning from the top
// RequestHandler. Additionally, Close method is called on each value
// implementing io.Closer before removing the value from ctx.
func (c *DefaultCtx) Locals(key any, value ...any) any {
	if len(value) == 0 {
		return c.fasthttp.UserValue(key)
	}
	c.fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Locals function utilizing Go's generics feature.
// This function allows for manipulating and retrieving local values within a
// request context with a more specific data type.
//
// All the values are removed from ctx after returning from the top
// RequestHandler. Additionally, Close method is called on each value
// implementing io.Closer before removing the value from ctx.
func Locals[V any](c Ctx, key any, value ...V) V {
	var v V
	var ok bool
	if len(value) == 0 {
		v, ok = c.Locals(key).(V)
	} else {
		v, ok = c.Locals(key, value[0]).(V)
	}
	if !ok {
		return v // return zero of type V
	}
	return v
}

// Location sets the response Location HTTP header to the specified path parameter.
func (c *DefaultCtx) Location(path string) {
	c.setCanonical(HeaderLocation, path)
}

// Method returns the HTTP request method for the context, optionally overridden by the provided argument.
// If no override is given or if the provided override is not a valid HTTP method, it returns the current method from the context.
// Otherwise, it updates the context's method and returns the overridden method as a string.
func (c *DefaultCtx) Method(override ...string) string {
	if len(override) == 0 {
		// Nothing to override, just return current method from context
		return c.method
	}

	method := utils.ToUpper(override[0])
	mINT := c.app.methodInt(method)
	if mINT == -1 {
		// Provided override does not valid HTTP method, no override, return current method
		return c.method
	}

	c.method = method
	c.methodINT = mINT
	return c.method
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key the value will be a string slice.
func (c *DefaultCtx) MultipartForm() (*multipart.Form, error) {
	return c.fasthttp.MultipartForm()
}

// ClientHelloInfo return CHI from context
func (c *DefaultCtx) ClientHelloInfo() *tls.ClientHelloInfo {
	if c.app.tlsHandler != nil {
		return c.app.tlsHandler.clientHelloInfo
	}

	return nil
}

// Next executes the next method in the stack that matches the current route.
func (c *DefaultCtx) Next() error {
	// Increment handler index
	c.indexHandler++

	// Did we execute all route handlers?
	if c.indexHandler < len(c.route.Handlers) {
		// Continue route stack
		return c.route.Handlers[c.indexHandler](c)
	}

	// Continue handler stack
	if c.app.newCtxFunc != nil {
		_, err := c.app.nextCustom(c)
		return err
	}

	_, err := c.app.next(c)
	return err
}

// RestartRouting instead of going to the next handler. This may be useful after
// changing the request path. Note that handlers might be executed again.
func (c *DefaultCtx) RestartRouting() error {
	var err error

	c.indexRoute = -1
	if c.app.newCtxFunc != nil {
		_, err = c.app.nextCustom(c)
	} else {
		_, err = c.app.next(c)
	}
	return err
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) OriginalURL() string {
	return c.app.getString(c.fasthttp.Request.Header.RequestURI())
}

// Params is used to get the route parameters.
// Defaults to empty string "" if the param doesn't exist.
// If a default value is given, it will return that value if the param doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) Params(key string, defaultValue ...string) string {
	if key == "*" || key == "+" {
		key += "1"
	}

	route := c.Route()
	for i := range route.Params {
		if len(key) != len(c.route.Params[i]) {
			continue
		}
		if route.Params[i] == key || (!c.app.config.CaseSensitive && utils.EqualFold(route.Params[i], key)) {
			// in case values are not here
			if len(c.values) <= i || len(c.values[i]) == 0 {
				break
			}
			return c.values[i]
		}
	}
	return defaultString("", defaultValue)
}

// Params is used to get the route parameters.
// This function is generic and can handle different route parameters type values.
//
// Example:
//
// http://example.com/user/:user -> http://example.com/user/john
// Params[string](c, "user") -> returns john
//
// http://example.com/id/:id -> http://example.com/user/114
// Params[int](c, "id") ->  returns 114 as integer.
//
// http://example.com/id/:number -> http://example.com/id/john
// Params[int](c, "number", 0) -> returns 0 because can't parse 'john' as integer.
func Params[V GenericType](c Ctx, key string, defaultValue ...V) V {
	var v V
	return genericParseType(c.Params(key), v, defaultValue...)
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
func (c *DefaultCtx) Path(override ...string) string {
	if len(override) != 0 && c.path != override[0] {
		// Set new path to context
		c.pathOriginal = override[0]

		// Set new path to request context
		c.fasthttp.Request.URI().SetPath(c.pathOriginal)
		// Prettify path
		c.configDependentPaths()
	}
	return c.path
}

// Scheme contains the request protocol string: http or https for TLS requests.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Scheme() string {
	if c.fasthttp.IsTLS() {
		return schemeHTTPS
	}
	if !c.IsProxyTrusted() {
		return schemeHTTP
	}

	scheme := schemeHTTP
	const lenXHeaderName = 12
	c.fasthttp.Request.Header.VisitAll(func(key, val []byte) {
		if len(key) < lenXHeaderName {
			return // Neither "X-Forwarded-" nor "X-Url-Scheme"
		}
		switch {
		case bytes.HasPrefix(key, []byte("X-Forwarded-")):
			if bytes.Equal(key, []byte(HeaderXForwardedProto)) ||
				bytes.Equal(key, []byte(HeaderXForwardedProtocol)) {
				v := c.app.getString(val)
				commaPos := strings.Index(v, ",")
				if commaPos != -1 {
					scheme = v[:commaPos]
				} else {
					scheme = v
				}
			} else if bytes.Equal(key, []byte(HeaderXForwardedSsl)) && bytes.Equal(val, []byte("on")) {
				scheme = schemeHTTPS
			}

		case bytes.Equal(key, []byte(HeaderXUrlScheme)):
			scheme = c.app.getString(val)
		}
	})
	return scheme
}

// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
func (c *DefaultCtx) Protocol() string {
	return utils.UnsafeString(c.fasthttp.Request.Header.Protocol())
}

// Query returns the query string parameter in the url.
// Defaults to empty string "" if the query doesn't exist.
// If a default value is given, it will return that value if the query doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (c *DefaultCtx) Query(key string, defaultValue ...string) string {
	return Query[string](c, key, defaultValue...)
}

// Queries returns a map of query parameters and their values.
//
// GET /?name=alex&wanna_cake=2&id=
// Queries()["name"] == "alex"
// Queries()["wanna_cake"] == "2"
// Queries()["id"] == ""
//
// GET /?field1=value1&field1=value2&field2=value3
// Queries()["field1"] == "value2"
// Queries()["field2"] == "value3"
//
// GET /?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3
// Queries()["list_a"] == "3"
// Queries()["list_b[]"] == "3"
// Queries()["list_c"] == "1,2,3"
//
// GET /api/search?filters.author.name=John&filters.category.name=Technology&filters[customer][name]=Alice&filters[status]=pending
// Queries()["filters.author.name"] == "John"
// Queries()["filters.category.name"] == "Technology"
// Queries()["filters[customer][name]"] == "Alice"
// Queries()["filters[status]"] == "pending"
func (c *DefaultCtx) Queries() map[string]string {
	m := make(map[string]string, c.RequestCtx().QueryArgs().Len())
	c.RequestCtx().QueryArgs().VisitAll(func(key, value []byte) {
		m[c.app.getString(key)] = c.app.getString(value)
	})
	return m
}

// Query Retrieves the value of a query parameter from the request's URI.
// The function is generic and can handle query parameter values of different types.
// It takes the following parameters:
// - c: The context object representing the current request.
// - key: The name of the query parameter.
// - defaultValue: (Optional) The default value to return in case the query parameter is not found or cannot be parsed.
// The function performs the following steps:
//  1. Type-asserts the context object to *DefaultCtx.
//  2. Retrieves the raw query parameter value from the request's URI.
//  3. Parses the raw value into the appropriate type based on the generic type parameter V.
//     If parsing fails, the function checks if a default value is provided. If so, it returns the default value.
//  4. Returns the parsed value.
//
// If the generic type cannot be matched to a supported type, the function returns the default value (if provided) or the zero value of type V.
//
// Example usage:
//
//	GET /?search=john&age=8
//	name := Query[string](c, "search") // Returns "john"
//	age := Query[int](c, "age") // Returns 8
//	unknown := Query[string](c, "unknown", "default") // Returns "default" since the query parameter "unknown" is not found
func Query[V GenericType](c Ctx, key string, defaultValue ...V) V {
	var v V
	q := c.App().getString(c.RequestCtx().QueryArgs().Peek(key))

	return genericParseType[V](q, v, defaultValue...)
}

// Range returns a struct containing the type and a slice of ranges.
func (c *DefaultCtx) Range(size int) (Range, error) {
	var (
		rangeData Range
		ranges    string
	)
	rangeStr := c.Get(HeaderRange)

	i := strings.IndexByte(rangeStr, '=')
	if i == -1 || strings.Contains(rangeStr[i+1:], "=") {
		return rangeData, ErrRangeMalformed
	}
	rangeData.Type = rangeStr[:i]
	ranges = rangeStr[i+1:]

	var (
		singleRange string
		moreRanges  = ranges
	)
	for moreRanges != "" {
		singleRange = moreRanges
		if i := strings.IndexByte(moreRanges, ','); i >= 0 {
			singleRange = moreRanges[:i]
			moreRanges = moreRanges[i+1:]
		} else {
			moreRanges = ""
		}

		var (
			startStr, endStr string
			i                int
		)
		if i = strings.IndexByte(singleRange, '-'); i == -1 {
			return rangeData, ErrRangeMalformed
		}
		startStr = singleRange[:i]
		endStr = singleRange[i+1:]

		start, startErr := fasthttp.ParseUint(utils.UnsafeBytes(startStr))
		end, endErr := fasthttp.ParseUint(utils.UnsafeBytes(endStr))
		if startErr != nil { // -nnn
			start = size - end
			end = size - 1
		} else if endErr != nil { // nnn-
			end = size - 1
		}
		if end > size-1 { // limit last-byte-pos to current length
			end = size - 1
		}
		if start > end || start < 0 {
			continue
		}
		rangeData.Ranges = append(rangeData.Ranges, struct {
			Start int
			End   int
		}{
			Start: start,
			End:   end,
		})
	}
	if len(rangeData.Ranges) < 1 {
		return rangeData, ErrRangeUnsatisfiable
	}

	return rangeData, nil
}

// Redirect returns the Redirect reference.
// Use Redirect().Status() to set custom redirection status code.
// If status is not specified, status defaults to 302 Found.
// You can use Redirect().To(), Redirect().Route() and Redirect().Back() for redirection.
func (c *DefaultCtx) Redirect() *Redirect {
	if c.redirect == nil {
		c.redirect = AcquireRedirect()
		c.redirect.c = c
	}

	return c.redirect
}

// ViewBind Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (c *DefaultCtx) ViewBind(vars Map) error {
	// init viewBindMap - lazy map
	for k, v := range vars {
		c.viewBindMap.Store(k, v)
	}
	return nil
}

// getLocationFromRoute get URL location from route using parameters
func (c *DefaultCtx) getLocationFromRoute(route Route, params Map) (string, error) {
	buf := bytebufferpool.Get()
	for _, segment := range route.routeParser.segs {
		if !segment.IsParam {
			_, err := buf.WriteString(segment.Const)
			if err != nil {
				return "", fmt.Errorf("failed to write string: %w", err)
			}
			continue
		}

		for key, val := range params {
			isSame := key == segment.ParamName || (!c.app.config.CaseSensitive && utils.EqualFold(key, segment.ParamName))
			isGreedy := segment.IsGreedy && len(key) == 1 && isInCharset(key[0], greedyParameters)
			if isSame || isGreedy {
				_, err := buf.WriteString(utils.ToString(val))
				if err != nil {
					return "", fmt.Errorf("failed to write string: %w", err)
				}
			}
		}
	}
	location := buf.String()
	// release buffer
	bytebufferpool.Put(buf)
	return location, nil
}

// GetRouteURL generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"
func (c *DefaultCtx) GetRouteURL(routeName string, params Map) (string, error) {
	return c.getLocationFromRoute(c.App().GetRoute(routeName), params)
}

// Render a template with data and sends a text/html response.
// We support the following engines: https://github.com/gofiber/template
func (c *DefaultCtx) Render(name string, bind Map, layouts ...string) error {
	// Get new buffer from pool
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	// Initialize empty bind map if bind is nil
	if bind == nil {
		bind = make(Map)
	}

	// Pass-locals-to-views, bind, appListKeys
	c.renderExtensions(bind)

	var rendered bool
	for i := len(c.app.mountFields.appListKeys) - 1; i >= 0; i-- {
		prefix := c.app.mountFields.appListKeys[i]
		app := c.app.mountFields.appList[prefix]
		if prefix == "" || strings.Contains(c.OriginalURL(), prefix) {
			if len(layouts) == 0 && app.config.ViewsLayout != "" {
				layouts = []string{
					app.config.ViewsLayout,
				}
			}

			// Render template from Views
			if app.config.Views != nil {
				if err := app.config.Views.Render(buf, name, bind, layouts...); err != nil {
					return fmt.Errorf("failed to render: %w", err)
				}

				rendered = true
				break
			}
		}
	}

	if !rendered {
		// Render raw template using 'name' as filepath if no engine is set
		var tmpl *template.Template
		if _, err := readContent(buf, name); err != nil {
			return err
		}
		// Parse template
		tmpl, err := template.New("").Parse(c.app.getString(buf.Bytes()))
		if err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}
		buf.Reset()
		// Render template
		if err := tmpl.Execute(buf, bind); err != nil {
			return fmt.Errorf("failed to execute: %w", err)
		}
	}

	// Set Content-Type to text/html
	c.fasthttp.Response.Header.SetContentType(MIMETextHTMLCharsetUTF8)
	// Set rendered template to body
	c.fasthttp.Response.SetBody(buf.Bytes())

	return nil
}

func (c *DefaultCtx) renderExtensions(bind any) {
	if bindMap, ok := bind.(Map); ok {
		// Bind view map
		c.viewBindMap.Range(func(key, value any) bool {
			keyValue, ok := key.(string)
			if !ok {
				return true
			}
			if _, ok := bindMap[keyValue]; !ok {
				bindMap[keyValue] = value
			}
			return true
		})

		// Check if the PassLocalsToViews option is enabled (by default it is disabled)
		if c.app.config.PassLocalsToViews {
			// Loop through each local and set it in the map
			c.fasthttp.VisitUserValues(func(key []byte, val any) {
				// check if bindMap doesn't contain the key
				if _, ok := bindMap[c.app.getString(key)]; !ok {
					// Set the key and value in the bindMap
					bindMap[c.app.getString(key)] = val
				}
			})
		}
	}

	if len(c.app.mountFields.appListKeys) == 0 {
		c.app.generateAppListKeys()
	}
}

// Route returns the matched Route struct.
func (c *DefaultCtx) Route() *Route {
	if c.route == nil {
		// Fallback for fasthttp error handler
		return &Route{
			path:     c.pathOriginal,
			Path:     c.pathOriginal,
			Method:   c.method,
			Handlers: make([]Handler, 0),
			Params:   make([]string, 0),
		}
	}
	return c.route
}

// SaveFile saves any multipart file to disk.
func (*DefaultCtx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// SaveFileToStorage saves any multipart file to an external storage system.
func (*DefaultCtx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error {
	file, err := fileheader.Open()
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}
	defer file.Close() //nolint:errcheck // not needed

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	if err := storage.Set(path, content, 0); err != nil {
		return fmt.Errorf("failed to store: %w", err)
	}

	return nil
}

// Secure returns whether a secure connection was established.
func (c *DefaultCtx) Secure() bool {
	return c.Protocol() == schemeHTTPS
}

// Send sets the HTTP response body without copying it.
// From this point onward the body argument must not be changed.
func (c *DefaultCtx) Send(body []byte) error {
	// Write response body
	c.fasthttp.Response.SetBodyRaw(body)
	return nil
}

// SendFile transfers the file from the specified path.
// By default, the file is not compressed. To enable compression, set SendFile.Compress to true.
// The Content-Type response HTTP header field is set based on the file's extension.
// If the file extension is missing or invalid, the Content-Type is detected from the file's format.
func (c *DefaultCtx) SendFile(file string, config ...SendFile) error {
	// Save the filename, we will need it in the error message if the file isn't found
	filename := file

	var cfg SendFile
	if len(config) > 0 {
		cfg = config[0]
	}

	if cfg.CacheDuration == 0 {
		cfg.CacheDuration = 10 * time.Second
	}

	var fsHandler fasthttp.RequestHandler
	var cacheControlValue string

	c.app.sendfilesMutex.RLock()
	for _, sf := range c.app.sendfiles {
		if sf.compareConfig(cfg) {
			fsHandler = sf.handler
			cacheControlValue = sf.cacheControlValue
			break
		}
	}
	c.app.sendfilesMutex.RUnlock()

	if fsHandler == nil {
		fasthttpFS := &fasthttp.FS{
			Root:                   "",
			FS:                     cfg.FS,
			AllowEmptyRoot:         true,
			GenerateIndexPages:     false,
			AcceptByteRange:        cfg.ByteRange,
			Compress:               cfg.Compress,
			CompressBrotli:         cfg.Compress,
			CompressedFileSuffixes: c.app.config.CompressedFileSuffixes,
			CacheDuration:          cfg.CacheDuration,
			SkipCache:              cfg.CacheDuration < 0,
			IndexNames:             []string{"index.html"},
			PathNotFound: func(ctx *fasthttp.RequestCtx) {
				ctx.Response.SetStatusCode(StatusNotFound)
			},
		}

		if cfg.FS != nil {
			fasthttpFS.Root = "."
		}

		sf := &sendFileStore{
			config:  cfg,
			handler: fasthttpFS.NewRequestHandler(),
		}

		maxAge := cfg.MaxAge
		if maxAge > 0 {
			sf.cacheControlValue = "public, max-age=" + strconv.Itoa(maxAge)
		}

		// set vars
		fsHandler = sf.handler
		cacheControlValue = sf.cacheControlValue

		c.app.sendfilesMutex.Lock()
		c.app.sendfiles = append(c.app.sendfiles, sf)
		c.app.sendfilesMutex.Unlock()
	}

	// Keep original path for mutable params
	c.pathOriginal = utils.CopyString(c.pathOriginal)

	// Delete the Accept-Encoding header if compression is disabled
	if !cfg.Compress {
		// https://github.com/valyala/fasthttp/blob/7cc6f4c513f9e0d3686142e0a1a5aa2f76b3194a/fs.go#L55
		c.fasthttp.Request.Header.Del(HeaderAcceptEncoding)
	}

	// copy of https://github.com/valyala/fasthttp/blob/7cc6f4c513f9e0d3686142e0a1a5aa2f76b3194a/fs.go#L103-L121 with small adjustments
	if len(file) == 0 || (!filepath.IsAbs(file) && cfg.FS == nil) {
		// extend relative path to absolute path
		hasTrailingSlash := len(file) > 0 && (file[len(file)-1] == '/' || file[len(file)-1] == '\\')

		var err error
		file = filepath.FromSlash(file)
		if file, err = filepath.Abs(file); err != nil {
			return fmt.Errorf("failed to determine abs file path: %w", err)
		}
		if hasTrailingSlash {
			file += "/"
		}
	}

	// convert the path to forward slashes regardless the OS in order to set the URI properly
	// the handler will convert back to OS path separator before opening the file
	file = filepath.ToSlash(file)

	// Restore the original requested URL
	originalURL := utils.CopyString(c.OriginalURL())
	defer c.fasthttp.Request.SetRequestURI(originalURL)

	// Set new URI for fileHandler
	c.fasthttp.Request.SetRequestURI(file)

	// Save status code
	status := c.fasthttp.Response.StatusCode()

	// Serve file
	fsHandler(c.fasthttp)

	// Sets the response Content-Disposition header to attachment if the Download option is true
	if cfg.Download {
		c.Attachment()
	}

	// Get the status code which is set by fasthttp
	fsStatus := c.fasthttp.Response.StatusCode()

	// Check for error
	if status != StatusNotFound && fsStatus == StatusNotFound {
		return NewError(StatusNotFound, fmt.Sprintf("sendfile: file %s not found", filename))
	}

	// Set the status code set by the user if it is different from the fasthttp status code and 200
	if status != fsStatus && status != StatusOK {
		c.Status(status)
	}

	// Apply cache control header
	if status != StatusNotFound && status != StatusForbidden {
		if len(cacheControlValue) > 0 {
			c.RequestCtx().Response.Header.Set(HeaderCacheControl, cacheControlValue)
		}

		return nil
	}

	return nil
}

// SendStatus sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
func (c *DefaultCtx) SendStatus(status int) error {
	c.Status(status)

	// Only set status body when there is no response body
	if len(c.fasthttp.Response.Body()) == 0 {
		return c.SendString(utils.StatusMessage(status))
	}

	return nil
}

// SendString sets the HTTP response body for string types.
// This means no type assertion, recommended for faster performance
func (c *DefaultCtx) SendString(body string) error {
	c.fasthttp.Response.SetBodyString(body)

	return nil
}

// SendStream sets response body stream and optional body size.
func (c *DefaultCtx) SendStream(stream io.Reader, size ...int) error {
	if len(size) > 0 && size[0] >= 0 {
		c.fasthttp.Response.SetBodyStream(stream, size[0])
	} else {
		c.fasthttp.Response.SetBodyStream(stream, -1)
	}

	return nil
}

// SendStreamWriter sets response body stream writer
func (c *DefaultCtx) SendStreamWriter(streamWriter func(*bufio.Writer)) error {
	c.fasthttp.Response.SetBodyStreamWriter(fasthttp.StreamWriter(streamWriter))

	return nil
}

// Set sets the response's HTTP header field to the specified key, value.
func (c *DefaultCtx) Set(key, val string) {
	c.fasthttp.Response.Header.Set(key, val)
}

func (c *DefaultCtx) setCanonical(key, val string) {
	c.fasthttp.Response.Header.SetCanonical(utils.UnsafeBytes(key), utils.UnsafeBytes(val))
}

// Subdomains returns a string slice of subdomains in the domain name of the request.
// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
func (c *DefaultCtx) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(c.Host(), ".")
	l := len(subdomains) - o
	// Check index to avoid slice bounds out of range panic
	if l < 0 {
		l = len(subdomains)
	}
	subdomains = subdomains[:l]
	return subdomains
}

// Stale is not implemented yet, pull requests are welcome!
func (c *DefaultCtx) Stale() bool {
	return !c.Fresh()
}

// Status sets the HTTP status for the response.
// This method is chainable.
func (c *DefaultCtx) Status(status int) Ctx {
	c.fasthttp.Response.SetStatusCode(status)
	return c
}

// String returns unique string representation of the ctx.
//
// The returned value may be useful for logging.
func (c *DefaultCtx) String() string {
	// Get buffer from pool
	buf := bytebufferpool.Get()

	// Start with the ID, converting it to a hex string without fmt.Sprintf
	buf.WriteByte('#')
	// Convert ID to hexadecimal
	id := strconv.FormatUint(c.fasthttp.ID(), 16)
	// Pad with leading zeros to ensure 16 characters
	for i := 0; i < (16 - len(id)); i++ {
		buf.WriteByte('0')
	}
	buf.WriteString(id)
	buf.WriteString(" - ")

	// Add local and remote addresses directly
	buf.WriteString(c.fasthttp.LocalAddr().String())
	buf.WriteString(" <-> ")
	buf.WriteString(c.fasthttp.RemoteAddr().String())
	buf.WriteString(" - ")

	// Add method and URI
	buf.Write(c.fasthttp.Request.Header.Method())
	buf.WriteByte(' ')
	buf.Write(c.fasthttp.URI().FullURI())

	// Allocate string
	str := buf.String()

	// Reset buffer
	buf.Reset()
	bytebufferpool.Put(buf)

	return str
}

// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
func (c *DefaultCtx) Type(extension string, charset ...string) Ctx {
	if len(charset) > 0 {
		c.fasthttp.Response.Header.SetContentType(utils.GetMIME(extension) + "; charset=" + charset[0])
	} else {
		c.fasthttp.Response.Header.SetContentType(utils.GetMIME(extension))
	}
	return c
}

// Vary adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
func (c *DefaultCtx) Vary(fields ...string) {
	c.Append(HeaderVary, fields...)
}

// Write appends p into response body.
func (c *DefaultCtx) Write(p []byte) (int, error) {
	c.fasthttp.Response.AppendBody(p)
	return len(p), nil
}

// Writef appends f & a into response body writer.
func (c *DefaultCtx) Writef(f string, a ...any) (int, error) {
	//nolint:wrapcheck // This must not be wrapped
	return fmt.Fprintf(c.fasthttp.Response.BodyWriter(), f, a...)
}

// WriteString appends s to response body.
func (c *DefaultCtx) WriteString(s string) (int, error) {
	c.fasthttp.Response.AppendBodyString(s)
	return len(s), nil
}

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (c *DefaultCtx) XHR() bool {
	return utils.EqualFold(c.app.getBytes(c.Get(HeaderXRequestedWith)), []byte("xmlhttprequest"))
}

// configDependentPaths set paths for route recognition and prepared paths for the user,
// here the features for caseSensitive, decoded paths, strict paths are evaluated
func (c *DefaultCtx) configDependentPaths() {
	c.pathBuffer = append(c.pathBuffer[0:0], c.pathOriginal...)
	// If UnescapePath enabled, we decode the path and save it for the framework user
	if c.app.config.UnescapePath {
		c.pathBuffer = fasthttp.AppendUnquotedArg(c.pathBuffer[:0], c.pathBuffer)
	}
	c.path = c.app.getString(c.pathBuffer)

	// another path is specified which is for routing recognition only
	// use the path that was changed by the previous configuration flags
	c.detectionPathBuffer = append(c.detectionPathBuffer[0:0], c.pathBuffer...)
	// If CaseSensitive is disabled, we lowercase the original path
	if !c.app.config.CaseSensitive {
		c.detectionPathBuffer = utils.ToLowerBytes(c.detectionPathBuffer)
	}
	// If StrictRouting is disabled, we strip all trailing slashes
	if !c.app.config.StrictRouting && len(c.detectionPathBuffer) > 1 && c.detectionPathBuffer[len(c.detectionPathBuffer)-1] == '/' {
		c.detectionPathBuffer = utils.TrimRight(c.detectionPathBuffer, '/')
	}
	c.detectionPath = c.app.getString(c.detectionPathBuffer)

	// Define the path for dividing routes into areas for fast tree detection, so that fewer routes need to be traversed,
	// since the first three characters area select a list of routes
	c.treePath = c.treePath[0:0]
	const maxDetectionPaths = 3
	if len(c.detectionPath) >= maxDetectionPaths {
		c.treePath = c.detectionPath[:maxDetectionPaths]
	}
}

// IsProxyTrusted checks trustworthiness of remote ip.
// If Config.TrustProxy false, it returns true
// IsProxyTrusted can check remote ip by proxy ranges and ip map.
func (c *DefaultCtx) IsProxyTrusted() bool {
	if !c.app.config.TrustProxy {
		return true
	}

	ip := c.fasthttp.RemoteIP()

	if (c.app.config.TrustProxyConfig.Loopback && ip.IsLoopback()) ||
		(c.app.config.TrustProxyConfig.Private && ip.IsPrivate()) ||
		(c.app.config.TrustProxyConfig.LinkLocal && ip.IsLinkLocalUnicast()) {
		return true
	}

	if _, trusted := c.app.config.TrustProxyConfig.ips[ip.String()]; trusted {
		return true
	}

	for _, ipNet := range c.app.config.TrustProxyConfig.ranges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsFromLocal will return true if request came from local.
func (c *DefaultCtx) IsFromLocal() bool {
	return c.fasthttp.RemoteIP().IsLoopback()
}

// Bind You can bind body, cookie, headers etc. into the map, map slice, struct easily by using Binding method.
// It gives custom binding support, detailed binding options and more.
// Replacement of: BodyParser, ParamsParser, GetReqHeaders, GetRespHeaders, AllParams, QueryParser, ReqHeaderParser
func (c *DefaultCtx) Bind() *Bind {
	if c.bind == nil {
		c.bind = &Bind{
			ctx:            c,
			dontHandleErrs: true,
		}
	}
	return c.bind
}

// Reset is a method to reset context fields by given request when to use server handlers.
func (c *DefaultCtx) Reset(fctx *fasthttp.RequestCtx) {
	// Reset route and handler index
	c.indexRoute = -1
	c.indexHandler = 0
	// Reset matched flag
	c.matched = false
	// Set paths
	c.pathOriginal = c.app.getString(fctx.URI().PathOriginal())
	// Set method
	c.method = c.app.getString(fctx.Request.Header.Method())
	c.methodINT = c.app.methodInt(c.method)
	// Attach *fasthttp.RequestCtx to ctx
	c.fasthttp = fctx
	// reset base uri
	c.baseURI = ""
	// Prettify path
	c.configDependentPaths()
}

// Release is a method to reset context fields when to use ReleaseCtx()
func (c *DefaultCtx) release() {
	c.route = nil
	c.fasthttp = nil
	c.bind = nil
	c.flashMessages = c.flashMessages[:0]
	c.viewBindMap = sync.Map{}
	if c.redirect != nil {
		ReleaseRedirect(c.redirect)
		c.redirect = nil
	}
}

func (c *DefaultCtx) getBody() []byte {
	if c.app.config.Immutable {
		return utils.CopyBytes(c.fasthttp.Request.Body())
	}

	return c.fasthttp.Request.Body()
}

// Methods to use with next stack.
func (c *DefaultCtx) getMethodINT() int {
	return c.methodINT
}

func (c *DefaultCtx) getIndexRoute() int {
	return c.indexRoute
}

func (c *DefaultCtx) getTreePath() string {
	return c.treePath
}

func (c *DefaultCtx) getDetectionPath() string {
	return c.detectionPath
}

func (c *DefaultCtx) getPathOriginal() string {
	return c.pathOriginal
}

func (c *DefaultCtx) getValues() *[maxParams]string {
	return &c.values
}

func (c *DefaultCtx) getMatched() bool {
	return c.matched
}

func (c *DefaultCtx) setIndexHandler(handler int) {
	c.indexHandler = handler
}

func (c *DefaultCtx) setIndexRoute(route int) {
	c.indexRoute = route
}

func (c *DefaultCtx) setMatched(matched bool) {
	c.matched = matched
}

func (c *DefaultCtx) setRoute(route *Route) {
	c.route = route
}

// Drop closes the underlying connection without sending any response headers or body.
// This can be useful for silently terminating client connections, such as in DDoS mitigation
// or when blocking access to sensitive endpoints.
func (c *DefaultCtx) Drop() error {
	//nolint:wrapcheck // error wrapping is avoided to keep the operation lightweight and focused on connection closure.
	return c.RequestCtx().Conn().Close()
}
