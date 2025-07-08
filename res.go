package fiber

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

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

// ResFmt associates a Content Type to a fiber.Handler for c.Format
type ResFmt struct {
	Handler   func(Ctx) error
	MediaType string
}

//go:generate ifacemaker --file res.go --struct DefaultRes --iface Res --pkg fiber --output res_interface_gen.go --not-exported true --iface-comment "Res"
type DefaultRes struct {
	c Ctx
}

// App returns the *App reference to the instance of the Fiber application
func (r *DefaultRes) App() *App {
	return r.c.App()
}

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (r *DefaultRes) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := r.App().getString(r.Response().Header.Peek(field))
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
		r.Set(field, h)
	}
}

// Attachment sets the HTTP response Content-Disposition header field to attachment.
func (r *DefaultRes) Attachment(filename ...string) {
	if len(filename) > 0 {
		fname := filepath.Base(filename[0])
		r.Type(filepath.Ext(fname))
		var quoted string
		if r.App().isASCII(fname) {
			quoted = r.App().quoteString(fname)
		} else {
			quoted = r.App().quoteRawString(fname)
		}
		disp := `attachment; filename="` + quoted + `"`
		if !r.App().isASCII(fname) {
			disp += `; filename*=UTF-8''` + url.PathEscape(fname)
		}
		r.setCanonical(HeaderContentDisposition, disp)
		return
	}
	r.setCanonical(HeaderContentDisposition, "attachment")
}

// ClearCookie expires a specific cookie by key on the client side.
// If no key is provided it expires all cookies that came with the request.
func (r *DefaultRes) ClearCookie(key ...string) {
	if len(key) > 0 {
		for i := range key {
			r.Response().Header.DelClientCookie(key[i])
		}
		return
	}
	for k := range r.c.Request().Header.Cookies() {
		r.Response().Header.DelClientCookieBytes(k)
	}
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (r *DefaultRes) RequestCtx() *fasthttp.RequestCtx {
	return r.c.RequestCtx()
}

// Cookie sets a cookie by passing a cookie struct.
func (r *DefaultRes) Cookie(cookie *Cookie) {
	if cookie.Path == "" {
		cookie.Path = "/"
	}

	if cookie.SessionOnly {
		cookie.MaxAge = 0
		cookie.Expires = time.Time{}
	}

	var sameSite http.SameSite

	switch utils.ToLower(cookie.SameSite) {
	case CookieSameSiteStrictMode:
		sameSite = http.SameSiteStrictMode
	case CookieSameSiteNoneMode:
		sameSite = http.SameSiteNoneMode
	case CookieSameSiteDisabled:
		sameSite = 0
	case CookieSameSiteLaxMode:
		sameSite = http.SameSiteLaxMode
	default:
		sameSite = http.SameSiteLaxMode
	}

	// create/validate cookie using net/http
	hc := &http.Cookie{
		Name:        cookie.Name,
		Value:       cookie.Value,
		Path:        cookie.Path,
		Domain:      cookie.Domain,
		Expires:     cookie.Expires,
		MaxAge:      cookie.MaxAge,
		Secure:      cookie.Secure,
		HttpOnly:    cookie.HTTPOnly,
		SameSite:    sameSite,
		Partitioned: cookie.Partitioned,
	}

	if err := hc.Valid(); err != nil {
		// invalid cookies are ignored, same approach as net/http
		return
	}

	// create fasthttp cookie
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(hc.Name)
	fcookie.SetValue(hc.Value)
	fcookie.SetPath(hc.Path)
	fcookie.SetDomain(hc.Domain)

	if !cookie.SessionOnly {
		fcookie.SetMaxAge(hc.MaxAge)
		fcookie.SetExpire(hc.Expires)
	}

	fcookie.SetSecure(hc.Secure)
	fcookie.SetHTTPOnly(hc.HttpOnly)

	switch sameSite {
	case http.SameSiteLaxMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	case http.SameSiteStrictMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case http.SameSiteNoneMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	case http.SameSiteDefaultMode:
		fcookie.SetSameSite(fasthttp.CookieSameSiteDefaultMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteDisabled)
	}

	fcookie.SetPartitioned(hc.Partitioned)

	// Set resp header
	r.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

// Download transfers the file from path as an attachment.
// Typically, browsers will prompt the user for download.
// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
// Override this default with the filename parameter.
func (r *DefaultRes) Download(file string, filename ...string) error {
	var fname string
	if len(filename) > 0 {
		fname = filename[0]
	} else {
		fname = filepath.Base(file)
	}
	var quoted string
	if r.App().isASCII(fname) {
		quoted = r.App().quoteString(fname)
	} else {
		quoted = r.App().quoteRawString(fname)
	}
	disp := `attachment; filename="` + quoted + `"`
	if !r.App().isASCII(fname) {
		disp += `; filename*=UTF-8''` + url.PathEscape(fname)
	}
	r.setCanonical(HeaderContentDisposition, disp)
	return r.SendFile(file)
}

// Response return the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
func (r *DefaultRes) Response() *fasthttp.Response {
	return r.c.Response()
}

// Format performs content-negotiation on the Accept HTTP header.
// It uses Accepts to select a proper format and calls the matching
// user-provided handler function.
// If no accepted format is found, and a format with MediaType "default" is given,
// that default handler is called. If no format is found and no default is given,
// StatusNotAcceptable is sent.
func (r *DefaultRes) Format(handlers ...ResFmt) error {
	if len(handlers) == 0 {
		return ErrNoHandlers
	}

	r.Vary(HeaderAccept)

	if r.c.Get(HeaderAccept) == "" {
		r.Response().Header.SetContentType(handlers[0].MediaType)
		return handlers[0].Handler(r.c)
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
	accept := r.c.Accepts(types...)

	if accept == "" {
		if defaultHandler == nil {
			return r.SendStatus(StatusNotAcceptable)
		}
		return defaultHandler(r.c)
	}

	for _, h := range handlers {
		if h.MediaType == accept {
			r.Response().Header.SetContentType(h.MediaType)
			return h.Handler(r.c)
		}
	}

	return fmt.Errorf("%w: format: an Accept was found but no handler was called", errUnreachable)
}

// AutoFormat performs content-negotiation on the Accept HTTP header.
// It uses Accepts to select a proper format.
// The supported content types are text/html, text/plain, application/json, and application/xml.
// For more flexible content negotiation, use Format.
// If the header is not specified or there is no proper format, text/plain is used.
func (r *DefaultRes) AutoFormat(body any) error {
	// Get accepted content type
	accept := r.c.Accepts("html", "json", "txt", "xml")
	// Set accepted content type
	r.Type(accept)
	// Type convert provided body
	var b string
	switch val := body.(type) {
	case string:
		b = val
	case []byte:
		b = r.App().getString(val)
	default:
		b = fmt.Sprintf("%v", val)
	}

	// Format based on the accept content type
	switch accept {
	case "html":
		return r.SendString("<p>" + b + "</p>")
	case "json":
		return r.JSON(body)
	case "txt":
		return r.SendString(b)
	case "xml":
		return r.XML(body)
	}
	return r.SendString(b)
}

// Get (a.k.a. GetRespHeader) returns the HTTP response header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultRes) Get(key string, defaultValue ...string) string {
	return defaultString(r.App().getString(r.Response().Header.Peek(key)), defaultValue)
}

// GetHeaders returns the HTTP response headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultRes) GetHeaders() map[string][]string {
	headers := make(map[string][]string)
	for k, v := range r.Response().Header.All() {
		key := r.App().getString(k)
		headers[key] = append(headers[key], r.App().getString(v))
	}
	return headers
}

// JSON converts any interface or string to JSON.
// Array and slice values encode as JSON arrays,
// except that []byte encodes as a base64-encoded string,
// and a nil slice encodes as the null JSON value.
// If the ctype parameter is given, this method will set the
// Content-Type header equal to ctype. If ctype is not given,
// The Content-Type header will be set to application/json.
func (r *DefaultRes) JSON(data any, ctype ...string) error {
	raw, err := r.App().config.JSONEncoder(data)
	if err != nil {
		return err
	}
	r.Response().SetBodyRaw(raw)
	if len(ctype) > 0 {
		r.Response().Header.SetContentType(ctype[0])
	} else {
		r.Response().Header.SetContentType(MIMEApplicationJSON)
	}
	return nil
}

// CBOR converts any interface or string to CBOR encoded bytes.
// If the ctype parameter is given, this method will set the
// Content-Type header equal to ctype. If ctype is not given,
// The Content-Type header will be set to application/cbor.
func (r *DefaultRes) CBOR(data any, ctype ...string) error {
	raw, err := r.App().config.CBOREncoder(data)
	if err != nil {
		return err
	}
	r.Response().SetBodyRaw(raw)
	if len(ctype) > 0 {
		r.Response().Header.SetContentType(ctype[0])
	} else {
		r.Response().Header.SetContentType(MIMEApplicationCBOR)
	}
	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (r *DefaultRes) JSONP(data any, callback ...string) error {
	raw, err := r.App().config.JSONEncoder(data)
	if err != nil {
		return err
	}

	var result, cb string

	if len(callback) > 0 {
		cb = callback[0]
	} else {
		cb = "callback"
	}

	result = cb + "(" + r.App().getString(raw) + ");"

	r.setCanonical(HeaderXContentTypeOptions, "nosniff")
	r.Response().Header.SetContentType(MIMETextJavaScriptCharsetUTF8)
	return r.SendString(result)
}

// XML converts any interface or string to XML.
// This method also sets the content header to application/xml.
func (r *DefaultRes) XML(data any) error {
	raw, err := r.App().config.XMLEncoder(data)
	if err != nil {
		return err
	}
	r.Response().SetBodyRaw(raw)
	r.Response().Header.SetContentType(MIMEApplicationXML)
	return nil
}

// Links joins the links followed by the property to populate the response's Link HTTP header field.
func (r *DefaultRes) Links(link ...string) {
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
	r.setCanonical(HeaderLink, utils.TrimRight(r.App().getString(bb.Bytes()), ','))
	bytebufferpool.Put(bb)
}

// Location sets the response Location HTTP header to the specified path parameter.
func (r *DefaultRes) Location(path string) {
	r.setCanonical(HeaderLocation, path)
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultRes) OriginalURL() string {
	return r.c.OriginalURL()
}

// Redirect returns the Redirect reference.
// Use Redirect().Status() to set custom redirection status code.
// If status is not specified, status defaults to 303 See Other.
// You can use Redirect().To(), Redirect().Route() and Redirect().Back() for redirection.
func (r *DefaultRes) Redirect() *Redirect {
	return r.c.Redirect()
}

// ViewBind Add vars to default view var map binding to template engine.
// Variables are read by the Render method and may be overwritten.
func (r *DefaultRes) ViewBind(vars Map) error {
	return r.c.ViewBind(vars)
}

// getLocationFromRoute get URL location from route using parameters
func (r *DefaultRes) getLocationFromRoute(route Route, params Map) (string, error) {
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
			isSame := key == segment.ParamName || (!r.App().config.CaseSensitive && utils.EqualFold(key, segment.ParamName))
			isGreedy := segment.IsGreedy && len(key) == 1 && bytes.IndexByte(greedyParameters, key[0]) != -1
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
func (r *DefaultRes) GetRouteURL(routeName string, params Map) (string, error) {
	return r.getLocationFromRoute(r.App().GetRoute(routeName), params)
}

// Render a template with data and sends a text/html response.
// We support the following engines: https://github.com/gofiber/template
func (r *DefaultRes) Render(name string, bind any, layouts ...string) error {
	// Get new buffer from pool
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	// Initialize empty bind map if bind is nil
	if bind == nil {
		bind = make(Map)
	}

	// Pass-locals-to-views, bind, appListKeys
	r.renderExtensions(bind)

	var rendered bool
	for i := len(r.App().mountFields.appListKeys) - 1; i >= 0; i-- {
		prefix := r.App().mountFields.appListKeys[i]
		app := r.App().mountFields.appList[prefix]
		if prefix == "" || strings.Contains(r.OriginalURL(), prefix) {
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
		tmpl, err := template.New("").Parse(r.App().getString(buf.Bytes()))
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
	r.Response().Header.SetContentType(MIMETextHTMLCharsetUTF8)
	// Set rendered template to body
	r.Response().SetBody(buf.Bytes())

	return nil
}

func (r *DefaultRes) renderExtensions(bind any) {
	r.c.renderExtensions(bind)
}

// Send sets the HTTP response body without copying it.
// From this point onward the body argument must not be changed.
func (r *DefaultRes) Send(body []byte) error {
	// Write response body
	r.Response().SetBodyRaw(body)
	return nil
}

// SendFile transfers the file from the specified path.
// By default, the file is not compressed. To enable compression, set SendFile.Compress to true.
// The Content-Type response HTTP header field is set based on the file's extension.
// If the file extension is missing or invalid, the Content-Type is detected from the file's format.
func (r *DefaultRes) SendFile(file string, config ...SendFile) error {
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

	r.App().sendfilesMutex.RLock()
	for _, sf := range r.App().sendfiles {
		if sf.compareConfig(cfg) {
			fsHandler = sf.handler
			cacheControlValue = sf.cacheControlValue
			break
		}
	}
	r.App().sendfilesMutex.RUnlock()

	if fsHandler == nil {
		fasthttpFS := &fasthttp.FS{
			Root:                   "",
			FS:                     cfg.FS,
			AllowEmptyRoot:         true,
			GenerateIndexPages:     false,
			AcceptByteRange:        cfg.ByteRange,
			Compress:               cfg.Compress,
			CompressBrotli:         cfg.Compress,
			CompressZstd:           cfg.Compress,
			CompressedFileSuffixes: r.App().config.CompressedFileSuffixes,
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

		r.App().sendfilesMutex.Lock()
		r.App().sendfiles = append(r.App().sendfiles, sf)
		r.App().sendfilesMutex.Unlock()
	}

	// Keep original path for mutable params
	r.c.keepOriginalPath()

	// Delete the Accept-Encoding header if compression is disabled
	if !cfg.Compress {
		// https://github.com/valyala/fasthttp/blob/7cc6f4c513f9e0d3686142e0a1a5aa2f76b3194a/fs.go#L55
		r.c.Request().Header.Del(HeaderAcceptEncoding)
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
	originalURL := utils.CopyString(r.OriginalURL())
	defer r.c.Request().SetRequestURI(originalURL)

	// Set new URI for fileHandler
	r.c.Request().SetRequestURI(file)

	// Save status code
	status := r.Response().StatusCode()

	// Serve file
	fsHandler(r.RequestCtx())

	// Sets the response Content-Disposition header to attachment if the Download option is true
	if cfg.Download {
		r.Attachment()
	}

	// Get the status code which is set by fasthttp
	fsStatus := r.Response().StatusCode()

	// Check for error
	if status != StatusNotFound && fsStatus == StatusNotFound {
		return NewError(StatusNotFound, fmt.Sprintf("sendfile: file %s not found", filename))
	}

	// Set the status code set by the user if it is different from the fasthttp status code and 200
	if status != fsStatus && status != StatusOK {
		r.Status(status)
	}

	// Apply cache control header
	if status != StatusNotFound && status != StatusForbidden {
		if len(cacheControlValue) > 0 {
			r.RequestCtx().Response.Header.Set(HeaderCacheControl, cacheControlValue)
		}

		return nil
	}

	return nil
}

// SendStatus sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
func (r *DefaultRes) SendStatus(status int) error {
	r.Status(status)

	// Only set status body when there is no response body
	if len(r.Response().Body()) == 0 {
		return r.SendString(utils.StatusMessage(status))
	}

	return nil
}

// SendString sets the HTTP response body for string types.
// This means no type assertion, recommended for faster performance
func (r *DefaultRes) SendString(body string) error {
	r.Response().SetBodyString(body)

	return nil
}

// SendStream sets response body stream and optional body size.
func (r *DefaultRes) SendStream(stream io.Reader, size ...int) error {
	if len(size) > 0 && size[0] >= 0 {
		r.Response().SetBodyStream(stream, size[0])
	} else {
		r.Response().SetBodyStream(stream, -1)
	}

	return nil
}

// SendStreamWriter sets response body stream writer
func (r *DefaultRes) SendStreamWriter(streamWriter func(*bufio.Writer)) error {
	r.Response().SetBodyStreamWriter(fasthttp.StreamWriter(streamWriter))

	return nil
}

// Set sets the response's HTTP header field to the specified key, value.
func (r *DefaultRes) Set(key, val string) {
	r.Response().Header.Set(key, val)
}

func (r *DefaultRes) setCanonical(key, val string) {
	r.Response().Header.SetCanonical(utils.UnsafeBytes(key), utils.UnsafeBytes(val))
}

// Status sets the HTTP status for the response.
// This method is chainable.
func (r *DefaultRes) Status(status int) Ctx {
	r.Response().SetStatusCode(status)
	return r.c
}

// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
func (r *DefaultRes) Type(extension string, charset ...string) Ctx {
	if len(charset) > 0 {
		r.Response().Header.SetContentType(utils.GetMIME(extension) + "; charset=" + charset[0])
	} else {
		r.Response().Header.SetContentType(utils.GetMIME(extension))
	}
	return r.c
}

// Vary adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
func (r *DefaultRes) Vary(fields ...string) {
	r.Append(HeaderVary, fields...)
}

// Write appends p into response body.
func (r *DefaultRes) Write(p []byte) (int, error) {
	r.Response().AppendBody(p)
	return len(p), nil
}

// Writef appends f & a into response body writer.
func (r *DefaultRes) Writef(f string, a ...any) (int, error) {
	//nolint:wrapcheck // This must not be wrapped
	return fmt.Fprintf(r.Response().BodyWriter(), f, a...)
}

// WriteString appends s to response body.
func (r *DefaultRes) WriteString(s string) (int, error) {
	r.Response().AppendBodyString(s)
	return len(s), nil
}

// Release is a method to reset Res fields when to use ReleaseCtx()
func (r *DefaultRes) release() {
	r.c = nil
}

// Drop closes the underlying connection without sending any response headers or body.
// This can be useful for silently terminating client connections, such as in DDoS mitigation
// or when blocking access to sensitive endpoints.
func (r *DefaultRes) Drop() error {
	//nolint:wrapcheck // error wrapping is avoided to keep the operation lightweight and focused on connection closure.
	return r.RequestCtx().Conn().Close()
}

// End immediately flushes the current response and closes the underlying connection.
func (r *DefaultRes) End() error {
	ctx := r.RequestCtx()
	conn := ctx.Conn()

	bw := bufio.NewWriter(conn)
	if err := ctx.Response.Write(bw); err != nil {
		return err
	}

	if err := bw.Flush(); err != nil {
		return err //nolint:wrapcheck // unnecessary to wrap it
	}

	return conn.Close() //nolint:wrapcheck // unnecessary to wrap it
}
