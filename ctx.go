// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

type DefaultCtx struct {
	app                 *App                 // Reference to *App
	route               *Route               // Reference to *Route
	indexRoute          int                  // Index of the current route
	indexHandler        int                  // Index of the current handler
	method              string               // HTTP method
	methodINT           int                  // HTTP method INT equivalent
	baseURI             string               // HTTP base uri
	path                string               // HTTP path with the modifications by the configuration -> string copy from pathBuffer
	pathBuffer          []byte               // HTTP path buffer
	detectionPath       string               // Route detection path                                  -> string copy from detectionPathBuffer
	detectionPathBuffer []byte               // HTTP detectionPath buffer
	treePath            string               // Path for the search in the tree
	pathOriginal        string               // Original HTTP path
	values              [maxParams]string    // Route parameter values
	fasthttp            *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	matched             bool                 // Non use route matched
	viewBindMap         sync.Map             // Default view map to bind template engine
	bind                *Bind                // Default bind reference
	redirect            *Redirect            // Default redirect reference
	redirectionMessages []string             // Messages of the previous redirect
}

// TLSHandler object
type TLSHandler struct {
	clientHelloInfo *tls.ClientHelloInfo
}

// GetClientInfo Callback function to set ClientHelloInfo
// Must comply with the method structure of https://cs.opensource.google/go/go/+/refs/tags/go1.20:src/crypto/tls/common.go;l=554-563
// Since we overlay the method of the tls config in the listener method
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
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Path        string    `json:"path"`
	Domain      string    `json:"domain"`
	MaxAge      int       `json:"max_age"`
	Expires     time.Time `json:"expires"`
	Secure      bool      `json:"secure"`
	HTTPOnly    bool      `json:"http_only"`
	SameSite    string    `json:"same_site"`
	SessionOnly bool      `json:"session_only"`
}

// Views is the interface that wraps the Render function.
type Views interface {
	Load() error
	Render(io.Writer, string, any, ...string) error
}

// ResFmt associates a Content Type to a fiber.Handler for c.Format
type ResFmt struct {
	MediaType string
	Handler   func(Ctx) error
}

// Accepts checks if the specified extensions or content types are acceptable.
func (c *DefaultCtx) Accepts(offers ...string) string {
	return getOffer(c.Get(HeaderAccept), acceptsOfferType, offers...)
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (c *DefaultCtx) AcceptsCharsets(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptCharset), acceptsOffer, offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (c *DefaultCtx) AcceptsEncodings(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptEncoding), acceptsOffer, offers...)
}

// AcceptsLanguages checks if the specified language is acceptable.
func (c *DefaultCtx) AcceptsLanguages(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptLanguage), acceptsOffer, offers...)
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
	if c.app.config.Immutable {
		return utils.CopyBytes(c.fasthttp.Request.Body())
	}
	return c.fasthttp.Request.Body()
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

	// faster than peek
	c.Request().Header.VisitAll(func(key, value []byte) {
		if c.app.getString(key) == HeaderContentEncoding {
			headerEncoding = c.app.getString(value)
		}
	})

	// Split and get the encodings list, in order to attend the
	// rule defined at: https://www.rfc-editor.org/rfc/rfc9110#section-8.4-5
	encodingOrder = getSplicedStrList(headerEncoding, encodingOrder)
	if len(encodingOrder) == 0 {
		if c.app.config.Immutable {
			return utils.CopyBytes(c.fasthttp.Request.Body())
		}
		return c.fasthttp.Request.Body()
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
	c.fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		c.fasthttp.Response.Header.DelClientCookieBytes(k)
	})
}

// Context returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (c *DefaultCtx) Context() *fasthttp.RequestCtx {
	return c.fasthttp
}

// UserContext returns a context implementation that was set by
// user earlier or returns a non-nil, empty context,if it was not set earlier.
func (c *DefaultCtx) UserContext() context.Context {
	ctx, ok := c.fasthttp.UserValue(userContextKey).(context.Context)
	if !ok {
		ctx = context.Background()
		c.SetUserContext(ctx)
	}

	return ctx
}

// SetUserContext sets a context implementation by user.
func (c *DefaultCtx) SetUserContext(ctx context.Context) {
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
				return lastModifiedTime.Before(modifiedSinceTime)
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
	return defaultString(c.app.getString(c.fasthttp.Request.Header.Peek(key)), defaultValue)
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
// Make copies or use the Immutable setting instead.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
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
// Make copies or use the Immutable setting instead.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Hostname() string {
	addr, _ := parseAddr(c.Host())

	return addr
}

// Port returns the remote port of the request.
func (c *DefaultCtx) Port() string {
	tcpaddr, ok := c.fasthttp.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to *net.TCPAddr"))
	}
	return strconv.Itoa(tcpaddr.Port)
}

// IP returns the remote IP address of the request.
// If ProxyHeader and IP Validation is configured, it will parse that header and return the first valid IP address.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
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

		s := strings.TrimRight(headerValue[i:j], " ")

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

			s := strings.TrimRight(headerValue[i:j], " ")

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
		strings.TrimLeft(utils.UnsafeString(c.fasthttp.Request.Header.ContentType()), " "),
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
			_ = bb.WriteByte('<')          //nolint:errcheck // This will never fail
			_, _ = bb.WriteString(link[i]) //nolint:errcheck // This will never fail
			_ = bb.WriteByte('>')          //nolint:errcheck // This will never fail
		} else {
			_, _ = bb.WriteString(`; rel="` + link[i] + `",`) //nolint:errcheck // This will never fail
		}
	}
	c.setCanonical(HeaderLink, strings.TrimRight(c.app.getString(bb.Bytes()), ","))
	bytebufferpool.Put(bb)
}

// Locals makes it possible to pass any values under keys scoped to the request
// and therefore available to all following routes that match the request.
func (c *DefaultCtx) Locals(key any, value ...any) any {
	if len(value) == 0 {
		return c.fasthttp.UserValue(key)
	}
	c.fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Locals function utilizing Go's generics feature.
// This function allows for manipulating and retrieving local values within a request context with a more specific data type.
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
	var err error
	// Did we execute all route handlers?
	if c.indexHandler < len(c.route.Handlers) {
		// Continue route stack
		err = c.route.Handlers[c.indexHandler](c)
	} else {
		// Continue handler stack
		if c.app.newCtxFunc != nil {
			_, err = c.app.nextCustom(c)
		} else {
			_, err = c.app.next(c)
		}
	}
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
	for i := range c.route.Params {
		if len(key) != len(c.route.Params[i]) {
			continue
		}
		if c.route.Params[i] == key || (!c.app.config.CaseSensitive && utils.EqualFold(c.route.Params[i], key)) {
			// in case values are not here
			if len(c.values) <= i || len(c.values[i]) == 0 {
				break
			}
			return c.values[i]
		}
	}
	return defaultString("", defaultValue)
}

// ParamsInt is used to get an integer from the route parameters
// it defaults to zero if the parameter is not found or if the
// parameter cannot be converted to an integer
// If a default value is given, it will return that value in case the param
// doesn't exist or cannot be converted to an integer
func (c *DefaultCtx) ParamsInt(key string, defaultValue ...int) (int, error) {
	// Use Atoi to convert the param to an int or return zero and an error
	value, err := strconv.Atoi(c.Params(key))
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}
		return 0, fmt.Errorf("failed to convert: %w", err)
	}

	return value, nil
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
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
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
	m := make(map[string]string, c.Context().QueryArgs().Len())
	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
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
func Query[V QueryType](c Ctx, key string, defaultValue ...V) V {
	ctx, ok := c.(*DefaultCtx)
	if !ok {
		panic(fmt.Errorf("failed to type-assert to *DefaultCtx"))
	}
	var v V
	q := ctx.app.getString(ctx.fasthttp.QueryArgs().Peek(key))

	switch any(v).(type) {
	case int:
		return queryParseInt[V](q, 32, func(i int64) V { return assertValueType[V, int](int(i)) }, defaultValue...)
	case int8:
		return queryParseInt[V](q, 8, func(i int64) V { return assertValueType[V, int8](int8(i)) }, defaultValue...)
	case int16:
		return queryParseInt[V](q, 16, func(i int64) V { return assertValueType[V, int16](int16(i)) }, defaultValue...)
	case int32:
		return queryParseInt[V](q, 32, func(i int64) V { return assertValueType[V, int32](int32(i)) }, defaultValue...)
	case int64:
		return queryParseInt[V](q, 64, func(i int64) V { return assertValueType[V, int64](i) }, defaultValue...)
	case uint:
		return queryParseUint[V](q, 32, func(i uint64) V { return assertValueType[V, uint](uint(i)) }, defaultValue...)
	case uint8:
		return queryParseUint[V](q, 8, func(i uint64) V { return assertValueType[V, uint8](uint8(i)) }, defaultValue...)
	case uint16:
		return queryParseUint[V](q, 16, func(i uint64) V { return assertValueType[V, uint16](uint16(i)) }, defaultValue...)
	case uint32:
		return queryParseUint[V](q, 32, func(i uint64) V { return assertValueType[V, uint32](uint32(i)) }, defaultValue...)
	case uint64:
		return queryParseUint[V](q, 64, func(i uint64) V { return assertValueType[V, uint64](i) }, defaultValue...)
	case float32:
		return queryParseFloat[V](q, 32, func(i float64) V { return assertValueType[V, float32](float32(i)) }, defaultValue...)
	case float64:
		return queryParseFloat[V](q, 64, func(i float64) V { return assertValueType[V, float64](i) }, defaultValue...)
	case bool:
		return queryParseBool[V](q, func(b bool) V { return assertValueType[V, bool](b) }, defaultValue...)
	case string:
		if q == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, string](q)
	case []byte:
		if q == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return assertValueType[V, []byte](ctx.app.getBytes(q))
	default:
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return v
	}
}

type QueryType interface {
	QueryTypeInteger | QueryTypeFloat | bool | string | []byte
}

type QueryTypeInteger interface {
	QueryTypeIntegerSigned | QueryTypeIntegerUnsigned
}

type QueryTypeIntegerSigned interface {
	int | int8 | int16 | int32 | int64
}

type QueryTypeIntegerUnsigned interface {
	uint | uint8 | uint16 | uint32 | uint64
}

type QueryTypeFloat interface {
	float32 | float64
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
			start,
			end,
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

// Bind Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (c *DefaultCtx) BindVars(vars Map) error {
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

var (
	sendFileOnce    sync.Once
	sendFileFS      *fasthttp.FS
	sendFileHandler fasthttp.RequestHandler
)

// SendFile transfers the file from the given path.
// The file is not compressed by default, enable this by passing a 'true' argument
// Sets the Content-Type response HTTP header field based on the filenames extension.
func (c *DefaultCtx) SendFile(file string, compress ...bool) error {
	// Save the filename, we will need it in the error message if the file isn't found
	filename := file

	// https://github.com/valyala/fasthttp/blob/c7576cc10cabfc9c993317a2d3f8355497bea156/fs.go#L129-L134
	sendFileOnce.Do(func() {
		const cacheDuration = 10 * time.Second
		sendFileFS = &fasthttp.FS{
			Root:                 "",
			AllowEmptyRoot:       true,
			GenerateIndexPages:   false,
			AcceptByteRange:      true,
			Compress:             true,
			CompressedFileSuffix: c.app.config.CompressedFileSuffix,
			CacheDuration:        cacheDuration,
			IndexNames:           []string{"index.html"},
			PathNotFound: func(ctx *fasthttp.RequestCtx) {
				ctx.Response.SetStatusCode(StatusNotFound)
			},
		}
		sendFileHandler = sendFileFS.NewRequestHandler()
	})

	// Keep original path for mutable params
	c.pathOriginal = utils.CopyString(c.pathOriginal)
	// Disable compression
	if len(compress) == 0 || !compress[0] {
		// https://github.com/valyala/fasthttp/blob/7cc6f4c513f9e0d3686142e0a1a5aa2f76b3194a/fs.go#L55
		c.fasthttp.Request.Header.Del(HeaderAcceptEncoding)
	}
	// copy of https://github.com/valyala/fasthttp/blob/7cc6f4c513f9e0d3686142e0a1a5aa2f76b3194a/fs.go#L103-L121 with small adjustments
	if len(file) == 0 || !filepath.IsAbs(file) {
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
	sendFileHandler(c.fasthttp)
	// Get the status code which is set by fasthttp
	fsStatus := c.fasthttp.Response.StatusCode()
	// Set the status code set by the user if it is different from the fasthttp status code and 200
	if status != fsStatus && status != StatusOK {
		c.Status(status)
	}
	// Check for error
	if status != StatusNotFound && fsStatus == StatusNotFound {
		return NewError(StatusNotFound, fmt.Sprintf("sendfile: file %s not found", filename))
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
	buf.WriteByte('#') //nolint:errcheck // It is fine to ignore the error
	// Convert ID to hexadecimal
	id := strconv.FormatUint(c.fasthttp.ID(), 16)
	// Pad with leading zeros to ensure 16 characters
	for i := 0; i < (16 - len(id)); i++ {
		buf.WriteByte('0') //nolint:errcheck // It is fine to ignore the error
	}
	buf.WriteString(id)    //nolint:errcheck // It is fine to ignore the error
	buf.WriteString(" - ") //nolint:errcheck // It is fine to ignore the error

	// Add local and remote addresses directly
	buf.WriteString(c.fasthttp.LocalAddr().String())  //nolint:errcheck // It is fine to ignore the error
	buf.WriteString(" <-> ")                          //nolint:errcheck // It is fine to ignore the error
	buf.WriteString(c.fasthttp.RemoteAddr().String()) //nolint:errcheck // It is fine to ignore the error
	buf.WriteString(" - ")                            //nolint:errcheck // It is fine to ignore the error

	// Add method and URI
	buf.Write(c.fasthttp.Request.Header.Method()) //nolint:errcheck // It is fine to ignore the error
	buf.WriteByte(' ')                            //nolint:errcheck // It is fine to ignore the error
	buf.Write(c.fasthttp.URI().FullURI())         //nolint:errcheck // It is fine to ignore the error

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
		c.detectionPathBuffer = bytes.TrimRight(c.detectionPathBuffer, "/")
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
// If EnableTrustedProxyCheck false, it returns true
// IsProxyTrusted can check remote ip by proxy ranges and ip map.
func (c *DefaultCtx) IsProxyTrusted() bool {
	if !c.app.config.EnableTrustedProxyCheck {
		return true
	}

	ip := c.fasthttp.RemoteIP()

	if _, trusted := c.app.config.trustedProxiesMap[ip.String()]; trusted {
		return true
	}

	for _, ipNet := range c.app.config.trustedProxyRanges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

var localHosts = [...]string{"127.0.0.1", "::1"}

// IsLocalHost will return true if address is a localhost address.
func (*DefaultCtx) isLocalHost(address string) bool {
	for _, h := range localHosts {
		if address == h {
			return true
		}
	}
	return false
}

// IsFromLocal will return true if request came from local.
func (c *DefaultCtx) IsFromLocal() bool {
	return c.isLocalHost(c.fasthttp.RemoteIP().String())
}

// You can bind body, cookie, headers etc. into the map, map slice, struct easily by using Binding method.
// It gives custom binding support, detailed binding options and more.
// Replacement of: BodyParser, ParamsParser, GetReqHeaders, GetRespHeaders, AllParams, QueryParser, ReqHeaderParser
func (c *DefaultCtx) Bind() *Bind {
	if c.bind == nil {
		c.bind = &Bind{
			ctx:    c,
			should: true,
		}
	}
	return c.bind
}
