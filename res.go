package fiber

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

//go:generate ifacemaker --file res.go --struct DefaultRes --iface Res --pkg fiber --output res_interface_gen.go --not-exported true --iface-comment "Res"
type DefaultRes struct {
	c Ctx
}

// App returns the *App reference to the instance of the Fiber application
func (r *DefaultRes) App() *App {
	return r.c.App()
}

// Accepts checks if the specified extensions or content types are acceptable.
func (r *DefaultRes) Accepts(offers ...string) string {
	return getOffer(r.Request().Header.Peek(HeaderAccept), acceptsOfferType, offers...)
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

		r.setCanonical(HeaderContentDisposition, `attachment; filename="`+r.App().quoteString(fname)+`"`)
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
	r.Request().Header.VisitAllCookie(func(k, _ []byte) {
		r.Response().Header.DelClientCookieBytes(k)
	})
}

// Cookie sets a cookie by passing a cookie struct.
func (r *DefaultRes) Cookie(cookie *Cookie) {
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

	r.Response().Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultRes) Cookies(key string, defaultValue ...string) string {
	return defaultString(r.App().getString(r.Request().Header.Cookie(key)), defaultValue)
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
	r.setCanonical(HeaderContentDisposition, `attachment; filename="`+r.App().quoteString(fname)+`"`)
	return r.SendFile(file)
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
	accept := r.Accepts(types...)

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
	accept := r.Accepts("html", "json", "txt", "xml")
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

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (r *DefaultRes) RequestCtx() *fasthttp.RequestCtx {
	return r.c.RequestCtx()
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (r *DefaultRes) Request() *fasthttp.Request {
	return r.c.Request()
}

// Response return the *fasthttp.Response object
// This allows you to use all fasthttp response methods
// https://godoc.org/github.com/valyala/fasthttp#Response
func (r *DefaultRes) Response() *fasthttp.Response {
	return r.c.Response()
}

// Release is a method to reset Res fields when to use ReleaseCtx()
func (r *DefaultRes) release() {
	r.c = nil
}

// GetRespHeader returns the HTTP response header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultRes) Get(key string, defaultValue ...string) string {
	return defaultString(r.App().getString(r.Response().Header.Peek(key)), defaultValue)
}

// GetRespHeaders returns the HTTP response headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultRes) GetRespHeaders() map[string][]string {
	headers := make(map[string][]string)
	r.Response().Header.VisitAll(func(k, v []byte) {
		key := r.App().getString(k)
		headers[key] = append(headers[key], r.App().getString(v))
	})
	return headers
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
		r.Request().Header.Del(HeaderAcceptEncoding)
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
	defer r.Request().SetRequestURI(originalURL)

	// Set new URI for fileHandler
	r.Request().SetRequestURI(file)

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

func (r *DefaultReq) getPathOriginal() string {
	return r.c.getPathOriginal()
}
