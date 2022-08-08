// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
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

	"github.com/gofiber/fiber/v3/utils"
	"github.com/savsgio/dictpool"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// maxParams defines the maximum number of parameters per route.
const maxParams = 30

// userContextKey define the key name for storing context.Context in *fasthttp.RequestCtx
const userContextKey = "__local_user_context__"

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
	viewBindMap         *dictpool.Dict       // Default view map to bind template engine
	bind                *Bind                // Default bind reference
}

// Range data for c.Range
type Range struct {
	Type   string
	Ranges []struct {
		Start int
		End   int
	}
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

// Accepts checks if the specified extensions or content types are acceptable.
func (c *DefaultCtx) Accepts(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	header := c.Get(HeaderAccept)
	if header == "" {
		return offers[0]
	}

	spec, commaPos := "", 0
	for len(header) > 0 && commaPos != -1 {
		commaPos = strings.IndexByte(header, ',')
		if commaPos != -1 {
			spec = utils.Trim(header[:commaPos], ' ')
		} else {
			spec = utils.TrimLeft(header, ' ')
		}
		if factorSign := strings.IndexByte(spec, ';'); factorSign != -1 {
			spec = spec[:factorSign]
		}

		var mimetype string
		for _, offer := range offers {
			if len(offer) == 0 {
				continue
				// Accept: */*
			} else if spec == "*/*" {
				return offer
			}

			if strings.IndexByte(offer, '/') != -1 {
				mimetype = offer // MIME type
			} else {
				mimetype = utils.GetMIME(offer) // extension
			}

			if spec == mimetype {
				// Accept: <MIME_type>/<MIME_subtype>
				return offer
			}

			s := strings.IndexByte(mimetype, '/')
			// Accept: <MIME_type>/*
			if strings.HasPrefix(spec, mimetype[:s]) && (spec[s:] == "/*" || mimetype[s:] == "/*") {
				return offer
			}
		}
		if commaPos != -1 {
			header = header[commaPos+1:]
		}
	}

	return ""
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (c *DefaultCtx) AcceptsCharsets(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptCharset), offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (c *DefaultCtx) AcceptsEncodings(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptEncoding), offers...)
}

// AcceptsLanguages checks if the specified language is acceptable.
func (c *DefaultCtx) AcceptsLanguages(offers ...string) string {
	return getOffer(c.Get(HeaderAcceptLanguage), offers...)
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
	c.baseURI = c.Scheme() + "://" + c.Hostname()
	return c.baseURI
}

// Body contains the raw body submitted in a POST request.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *DefaultCtx) Body() []byte {
	var err error
	var encoding string
	var body []byte
	// faster than peek
	c.Request().Header.VisitAll(func(key, value []byte) {
		if utils.UnsafeString(key) == HeaderContentEncoding {
			encoding = utils.UnsafeString(value)
		}
	})

	switch encoding {
	case StrGzip:
		body, err = c.fasthttp.Request.BodyGunzip()
	case StrBr, StrBrotli:
		body, err = c.fasthttp.Request.BodyUnbrotli()
	case StrDeflate:
		body, err = c.fasthttp.Request.BodyInflate()
	default:
		body = c.fasthttp.Request.Body()
	}

	if err != nil {
		return []byte(err.Error())
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

// Cookies is used for getting a cookie value by key.
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
// It uses Accepts to select a proper format.
// If the header is not specified or there is no proper format, text/plain is used.
func (c *DefaultCtx) Format(body any) error {
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
		raw, err := xml.Marshal(body)
		if err != nil {
			return fmt.Errorf("error serializing xml: %v", body)
		}
		c.fasthttp.Response.SetBody(raw)
		return nil
	}
	return c.SendString(b)
}

// FormFile returns the first file by key from a MultipartForm.
func (c *DefaultCtx) FormFile(key string) (*multipart.FileHeader, error) {
	return c.fasthttp.FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
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

// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Hostname() string {
	if c.IsProxyTrusted() {
		if host := c.Get(HeaderXForwardedHost); len(host) > 0 {
			return host
		}
	}
	return c.app.getString(c.fasthttp.Request.URI().Host())
}

// Port returns the remote port of the request.
func (c *DefaultCtx) Port() string {
	port := c.fasthttp.RemoteAddr().(*net.TCPAddr).Port
	return strconv.Itoa(port)
}

// IP returns the remote IP address of the request.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) IP() string {
	if c.IsProxyTrusted() && len(c.app.config.ProxyHeader) > 0 {
		return c.Get(c.app.config.ProxyHeader)
	}

	return c.fasthttp.RemoteIP().String()
}

// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
func (c *DefaultCtx) IPs() (ips []string) {
	header := c.fasthttp.Request.Header.Peek(HeaderXForwardedFor)
	if len(header) == 0 {
		return
	}
	ips = make([]string, bytes.Count(header, []byte(","))+1)
	var commaPos, i int
	for {
		commaPos = bytes.IndexByte(header, ',')
		if commaPos != -1 {
			ips[i] = utils.Trim(c.app.getString(header[:commaPos]), ' ')
			header, i = header[commaPos+1:], i+1
		} else {
			ips[i] = utils.Trim(c.app.getString(header), ' ')
			return
		}
	}
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
// This method also sets the content header to application/json.
func (c *DefaultCtx) JSON(data any) error {
	raw, err := c.app.config.JSONEncoder(data)
	if err != nil {
		return err
	}
	c.fasthttp.Response.SetBodyRaw(raw)
	c.fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (c *DefaultCtx) JSONP(data any, callback ...string) error {
	raw, err := json.Marshal(data)
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
	c.fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScriptCharsetUTF8)
	return c.SendString(result)
}

// Links joins the links followed by the property to populate the response's Link HTTP header field.
func (c *DefaultCtx) Links(link ...string) {
	if len(link) == 0 {
		return
	}
	bb := bytebufferpool.Get()
	for i := range link {
		if i%2 == 0 {
			_ = bb.WriteByte('<')
			_, _ = bb.WriteString(link[i])
			_ = bb.WriteByte('>')
		} else {
			_, _ = bb.WriteString(`; rel="` + link[i] + `",`)
		}
	}
	c.setCanonical(HeaderLink, utils.TrimRight(c.app.getString(bb.Bytes()), ','))
	bytebufferpool.Put(bb)
}

// Locals makes it possible to pass any values under string keys scoped to the request
// and therefore available to all following routes that match the request.
func (c *DefaultCtx) Locals(key string, value ...any) (val any) {
	if len(value) == 0 {
		return c.fasthttp.UserValue(key)
	}
	c.fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Location sets the response Location HTTP header to the specified path parameter.
func (c *DefaultCtx) Location(path string) {
	c.setCanonical(HeaderLocation, path)
}

// Method contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
func (c *DefaultCtx) Method(override ...string) string {
	if len(override) > 0 {
		method := utils.ToUpper(override[0])
		mINT := methodInt(method)
		if mINT == -1 {
			return c.method
		}
		c.method = method
		c.methodINT = mINT
	}
	return c.method
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key the value will be a string slice.
func (c *DefaultCtx) MultipartForm() (*multipart.Form, error) {
	return c.fasthttp.MultipartForm()
}

// Next executes the next method in the stack that matches the current route.
func (c *DefaultCtx) Next() (err error) {
	// Increment handler index
	c.indexHandler++
	// Did we executed all route handlers?
	if c.indexHandler < len(c.route.Handlers) {
		// Continue route stack
		err = c.route.Handlers[c.indexHandler](c)
	} else {
		// Continue handler stack
		_, err = c.app.next(c, c.app.newCtxFunc != nil)
	}
	return err
}

// RestartRouting instead of going to the next handler. This may be usefull after
// changing the request path. Note that handlers might be executed again.
func (c *DefaultCtx) RestartRouting() error {
	c.indexRoute = -1
	_, err := c.app.next(c, c.app.newCtxFunc != nil)
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
		if c.route.Params[i] == key {
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
		} else {
			return 0, err
		}
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

// Scheme contains the request scheme string: http or https for TLS requests.
// Use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (c *DefaultCtx) Scheme() string {
	if c.fasthttp.IsTLS() {
		return "https"
	}
	scheme := "http"
	if !c.IsProxyTrusted() {
		return scheme
	}
	c.fasthttp.Request.Header.VisitAll(func(key, val []byte) {
		if len(key) < 12 {
			return // X-Forwarded-
		} else if bytes.HasPrefix(key, []byte("X-Forwarded-")) {
			if bytes.Equal(key, []byte(HeaderXForwardedProto)) {
				scheme = c.app.getString(val)
			} else if bytes.Equal(key, []byte(HeaderXForwardedProtocol)) {
				scheme = c.app.getString(val)
			} else if bytes.Equal(key, []byte(HeaderXForwardedSsl)) && bytes.Equal(val, []byte("on")) {
				scheme = "https"
			}
		} else if bytes.Equal(key, []byte(HeaderXUrlScheme)) {
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
	return defaultString(c.app.getString(c.fasthttp.QueryArgs().Peek(key)), defaultValue)
}

func parseParamSquareBrackets(k string) (string, error) {
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	kbytes := []byte(k)

	for i, b := range kbytes {

		if b == '[' && kbytes[i+1] != ']' {
			if err := bb.WriteByte('.'); err != nil {
				return "", err
			}
		}

		if b == '[' || b == ']' {
			continue
		}

		if err := bb.WriteByte(b); err != nil {
			return "", err
		}
	}

	return bb.String(), nil
}

// Range returns a struct containing the type and a slice of ranges.
func (c *DefaultCtx) Range(size int) (rangeData Range, err error) {
	rangeStr := c.Get(HeaderRange)
	if rangeStr == "" || !strings.Contains(rangeStr, "=") {
		err = ErrRangeMalformed
		return
	}
	data := strings.Split(rangeStr, "=")
	if len(data) != 2 {
		err = ErrRangeMalformed
		return
	}
	rangeData.Type = data[0]
	arr := strings.Split(data[1], ",")
	for i := 0; i < len(arr); i++ {
		item := strings.Split(arr[i], "-")
		if len(item) == 1 {
			err = ErrRangeMalformed
			return
		}
		start, startErr := strconv.Atoi(item[0])
		end, endErr := strconv.Atoi(item[1])
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
		err = ErrRangeUnsatisfiable
		return
	}

	return
}

// Redirect to the URL derived from the specified path, with specified status.
// If status is not specified, status defaults to 302 Found.
func (c *DefaultCtx) Redirect(location string, status ...int) error {
	c.setCanonical(HeaderLocation, location)
	if len(status) > 0 {
		c.Status(status[0])
	} else {
		c.Status(StatusFound)
	}
	return nil
}

// Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (c *DefaultCtx) BindVars(vars Map) error {
	// init viewBindMap - lazy map
	if c.viewBindMap == nil {
		c.viewBindMap = dictpool.AcquireDict()
	}
	for k, v := range vars {
		c.viewBindMap.Set(k, v)
	}

	return nil
}

// getLocationFromRoute get URL location from route using parameters
func (c *DefaultCtx) getLocationFromRoute(route Route, params Map) (string, error) {
	buf := bytebufferpool.Get()
	for _, segment := range route.routeParser.segs {
		for key, val := range params {
			if segment.IsParam && (key == segment.ParamName || (segment.IsGreedy && len(key) == 1 && isInCharset(key[0], greedyParameters))) {
				_, err := buf.WriteString(utils.ToString(val))
				if err != nil {
					return "", err
				}
			}
		}
		if !segment.IsParam {
			_, err := buf.WriteString(segment.Const)
			if err != nil {
				return "", err
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

// RedirectToRoute to the Route registered in the app with appropriate parameters
// If status is not specified, status defaults to 302 Found.
// If you want to send queries to route, you must add "queries" key typed as map[string]string to params.
func (c *DefaultCtx) RedirectToRoute(routeName string, params Map, status ...int) error {
	location, err := c.getLocationFromRoute(c.App().GetRoute(routeName), params)
	if err != nil {
		return err
	}

	// Check queries
	if queries, ok := params["queries"].(map[string]string); ok {
		queryText := bytebufferpool.Get()
		defer bytebufferpool.Put(queryText)

		i := 1
		for k, v := range queries {
			_, _ = queryText.WriteString(k + "=" + v)

			if i != len(queries) {
				_, _ = queryText.WriteString("&")
			}
			i++
		}

		return c.Redirect(location+"?"+queryText.String(), status...)
	}
	return c.Redirect(location, status...)
}

// RedirectBack to the URL to referer
// If status is not specified, status defaults to 302 Found.
func (c *DefaultCtx) RedirectBack(fallback string, status ...int) error {
	location := c.Get(HeaderReferer)
	if location == "" {
		location = fallback
	}
	return c.Redirect(location, status...)
}

// Render a template with data and sends a text/html response.
// We support the following engines: https://github.com/gofiber/template
func (c *DefaultCtx) Render(name string, bind Map, layouts ...string) error {
	var err error
	// Get new buffer from pool
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	// Pass-locals-to-views & bind
	c.renderExtensions(bind)

	rendered := false
	for prefix, app := range c.app.appList {
		if prefix == "" || strings.Contains(c.OriginalURL(), prefix) {
			if len(layouts) == 0 && app.config.ViewsLayout != "" {
				layouts = []string{
					app.config.ViewsLayout,
				}
			}

			// Render template from Views
			if app.config.Views != nil {
				if err := app.config.Views.Render(buf, name, bind, layouts...); err != nil {
					return err
				}

				rendered = true
				break
			}
		}
	}

	if !rendered {
		// Render raw template using 'name' as filepath if no engine is set
		var tmpl *template.Template
		if _, err = readContent(buf, name); err != nil {
			return err
		}
		// Parse template
		if tmpl, err = template.New("").Parse(c.app.getString(buf.Bytes())); err != nil {
			return err
		}
		buf.Reset()
		// Render template
		if err = tmpl.Execute(buf, bind); err != nil {
			return err
		}
	}

	// Set Content-Type to text/html
	c.fasthttp.Response.Header.SetContentType(MIMETextHTMLCharsetUTF8)
	// Set rendered template to body
	c.fasthttp.Response.SetBody(buf.Bytes())
	// Return err if exist
	return err
}

func (c *DefaultCtx) renderExtensions(bind Map) {
	// Bind view map
	if c.viewBindMap != nil {
		for _, v := range c.viewBindMap.D {
			bind[v.Key] = v.Value
		}
	}

	// Check if the PassLocalsToViews option is enabled (by default it is disabled)
	if c.app.config.PassLocalsToViews {
		// Loop through each local and set it in the map
		c.fasthttp.VisitUserValues(func(key []byte, val any) {
			// check if bindMap doesn't contain the key
			if _, ok := bind[utils.UnsafeString(key)]; !ok {
				// Set the key and value in the bindMap
				bind[utils.UnsafeString(key)] = val
			}
		})
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
func (c *DefaultCtx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// SaveFileToStorage saves any multipart file to an external storage system.
func (c *DefaultCtx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error {
	file, err := fileheader.Open()
	if err != nil {
		return err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return storage.Set(path, content, 0)
}

// Secure returns a boolean property, that is true, if a TLS connection is established.
func (c *DefaultCtx) Secure() bool {
	return c.fasthttp.IsTLS()
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
		sendFileFS = &fasthttp.FS{
			Root:                 "",
			AllowEmptyRoot:       true,
			GenerateIndexPages:   false,
			AcceptByteRange:      true,
			Compress:             true,
			CompressedFileSuffix: c.app.config.CompressedFileSuffix,
			CacheDuration:        10 * time.Second,
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
			return err
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
		c.setCanonical(HeaderContentLength, strconv.Itoa(len(c.fasthttp.Response.Body())))
	}

	return nil
}

// Set sets the response's HTTP header field to the specified key, value.
func (c *DefaultCtx) Set(key string, val string) {
	c.fasthttp.Response.Header.Set(key, val)
}

func (c *DefaultCtx) setCanonical(key string, val string) {
	c.fasthttp.Response.Header.SetCanonical(utils.UnsafeBytes(key), utils.UnsafeBytes(val))
}

// Subdomains returns a string slice of subdomains in the domain name of the request.
// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
func (c *DefaultCtx) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(c.Hostname(), ".")
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
	return fmt.Sprintf(
		"#%016X - %s <-> %s - %s %s",
		c.fasthttp.ID(),
		c.fasthttp.LocalAddr(),
		c.fasthttp.RemoteAddr(),
		c.fasthttp.Request.Header.Method(),
		c.fasthttp.URI().FullURI(),
	)
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
	return utils.EqualFoldBytes(utils.UnsafeBytes(c.Get(HeaderXRequestedWith)), []byte("xmlhttprequest"))
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
		c.detectionPathBuffer = utils.TrimRightBytes(c.detectionPathBuffer, '/')
	}
	c.detectionPath = c.app.getString(c.detectionPathBuffer)

	// Define the path for dividing routes into areas for fast tree detection, so that fewer routes need to be traversed,
	// since the first three characters area select a list of routes
	c.treePath = c.treePath[0:0]
	if len(c.detectionPath) >= 3 {
		c.treePath = c.detectionPath[:3]
	}
}

// IsProxyTrusted checks trustworthiness of remote ip.
// If EnableTrustedProxyCheck false, it returns true
// IsProxyTrusted can check remote ip by proxy ranges and ip map.
func (c *DefaultCtx) IsProxyTrusted() bool {
	if !c.app.config.EnableTrustedProxyCheck {
		return true
	}

	_, trusted := c.app.config.trustedProxiesMap[c.fasthttp.RemoteIP().String()]
	if trusted {
		return trusted
	}

	for _, ipNet := range c.app.config.trustedProxyRanges {
		if ipNet.Contains(c.fasthttp.RemoteIP()) {
			return true
		}
	}

	return false
}

// IsLocalHost will return true if address is a localhost address.
func (c *DefaultCtx) isLocalHost(address string) bool {
	localHosts := []string{"127.0.0.1", "0.0.0.0", "::1"}
	for _, h := range localHosts {
		if strings.Contains(address, h) {
			return true
		}
	}
	return false
}

// IsFromLocal will return true if request came from local.
func (c *DefaultCtx) IsFromLocal() bool {
	ips := c.IPs()
	if len(ips) == 0 {
		ips = append(ips, c.IP())
	}
	return c.isLocalHost(ips[0])
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
