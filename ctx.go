// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	utils "github.com/gofiber/utils"
	schema "github.com/gorilla/schema"
	bytebufferpool "github.com/valyala/bytebufferpool"
	fasthttp "github.com/valyala/fasthttp"
)

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
type Ctx struct {
	app          *App                 // Reference to *App
	route        *Route               // Reference to *Route
	indexRoute   int                  // Index of the current route
	indexHandler int                  // Index of the current handler
	method       string               // HTTP method
	methodINT    int                  // HTTP method INT equivalent
	path         string               // Prettified HTTP path
	treePath     string               // Path for the search in the tree
	pathOriginal string               // Original HTTP path
	values       []string             // Route parameter values
	err          error                // Contains error if passed to Next
	Fasthttp     *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	matched      bool                 // Non use route matched
}

// Range data for ctx.Range
type Range struct {
	Type   string
	Ranges []struct {
		Start int
		End   int
	}
}

// Cookie data for ctx.Cookie
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path"`
	Domain   string    `json:"domain"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
	SameSite string    `json:"same_site"`
}

// Views is the interface that wraps the Render function.
type Views interface {
	Load() error
	Render(io.Writer, string, interface{}, ...string) error
}

// AcquireCtx retrieves a new Ctx from the pool.
func (app *App) AcquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := app.pool.Get().(*Ctx)
	// Set app reference
	ctx.app = app
	// Reset route and handler index
	ctx.indexRoute = -1
	ctx.indexHandler = 0
	// Reset matched flag
	ctx.matched = false
	// Set paths
	ctx.path = getString(fctx.URI().PathOriginal())
	ctx.pathOriginal = ctx.path
	// Set method
	ctx.method = getString(fctx.Request.Header.Method())
	ctx.methodINT = methodInt(ctx.method)
	// Attach *fasthttp.RequestCtx to ctx
	ctx.Fasthttp = fctx
	// Prettify path
	ctx.prettifyPath()
	return ctx
}

// ReleaseCtx releases the ctx back into the pool.
func (app *App) ReleaseCtx(ctx *Ctx) {
	// Reset values
	ctx.route = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	ctx.err = nil
	app.pool.Put(ctx)
}

// Accepts checks if the specified extensions or content types are acceptable.
func (ctx *Ctx) Accepts(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	header := ctx.Get(HeaderAccept)
	if header == "" {
		return offers[0]
	}

	spec, commaPos := "", 0
	for len(header) > 0 && commaPos != -1 {
		commaPos = strings.IndexByte(header, ',')
		if commaPos != -1 {
			spec = utils.Trim(header[:commaPos], ' ')
		} else {
			spec = header
		}
		if factorSign := strings.IndexByte(spec, ';'); factorSign != -1 {
			spec = spec[:factorSign]
		}

		for _, offer := range offers {
			mimetype := utils.GetMIME(offer)
			if len(spec) > 2 && spec[len(spec)-2:] == "/*" {
				if strings.HasPrefix(spec[:len(spec)-2], strings.Split(mimetype, "/")[0]) {
					return offer
				} else if spec == "*/*" {
					return offer
				}
			} else if strings.HasPrefix(spec, mimetype) {
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
func (ctx *Ctx) AcceptsCharsets(offers ...string) string {
	return getOffer(ctx.Get(HeaderAcceptCharset), offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (ctx *Ctx) AcceptsEncodings(offers ...string) string {
	return getOffer(ctx.Get(HeaderAcceptEncoding), offers...)
}

// AcceptsLanguages checks if the specified language is acceptable.
func (ctx *Ctx) AcceptsLanguages(offers ...string) string {
	return getOffer(ctx.Get(HeaderAcceptLanguage), offers...)
}

// App returns the *App reference to access Settings or ErrorHandler
func (ctx *Ctx) App() *App {
	return ctx.app
}

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (ctx *Ctx) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := getString(ctx.Fasthttp.Response.Header.Peek(field))
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
		ctx.Set(field, h)
	}
}

// Attachment sets the HTTP response Content-Disposition header field to attachment.
func (ctx *Ctx) Attachment(filename ...string) {
	if len(filename) > 0 {
		fname := filepath.Base(filename[0])
		ctx.Type(filepath.Ext(fname))

		ctx.Set(HeaderContentDisposition, `attachment; filename="`+quoteString(fname)+`"`)
		return
	}
	ctx.Set(HeaderContentDisposition, "attachment")
}

// BaseURL returns (protocol + host + base path).
func (ctx *Ctx) BaseURL() string {
	// TODO: Could be improved: 53.8 ns/op  32 B/op  1 allocs/op
	// Should work like https://codeigniter.com/user_guide/helpers/url_helper.html
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// Body contains the raw body submitted in a POST request.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (ctx *Ctx) Body() string {
	return getString(ctx.Fasthttp.Request.Body())
}

// decoderPool helps to improve BodyParser's and QueryParser's performance
var decoderPool = &sync.Pool{New: func() interface{} {
	var decoder = schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	return decoder
}}

// BodyParser binds the request body to a struct.
// It supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
func (ctx *Ctx) BodyParser(out interface{}) error {
	// Get decoder from pool
	schemaDecoder := decoderPool.Get().(*schema.Decoder)
	defer decoderPool.Put(schemaDecoder)

	// Get content-type
	ctype := getString(ctx.Fasthttp.Request.Header.ContentType())

	// Parse body accordingly
	if strings.HasPrefix(ctype, MIMEApplicationJSON) {
		schemaDecoder.SetAliasTag("json")
		return json.Unmarshal(ctx.Fasthttp.Request.Body(), out)
	} else if strings.HasPrefix(ctype, MIMEApplicationForm) {
		schemaDecoder.SetAliasTag("form")
		data := make(map[string][]string)
		ctx.Fasthttp.PostArgs().VisitAll(func(key []byte, val []byte) {
			data[getString(key)] = append(data[getString(key)], getString(val))
		})
		return schemaDecoder.Decode(out, data)
	} else if strings.HasPrefix(ctype, MIMEMultipartForm) {
		schemaDecoder.SetAliasTag("form")
		data, err := ctx.Fasthttp.MultipartForm()
		if err != nil {
			return err
		}
		return schemaDecoder.Decode(out, data.Value)
	} else if strings.HasPrefix(ctype, MIMETextXML) || strings.HasPrefix(ctype, MIMEApplicationXML) {
		schemaDecoder.SetAliasTag("xml")
		return xml.Unmarshal(ctx.Fasthttp.Request.Body(), out)
	}

	// Query params in BodyParser is deprecated
	if ctx.Fasthttp.QueryArgs().Len() > 0 {
		fmt.Println("bodyparser: parsing query strings is deprecated since v1.12.7, please use `ctx.QueryParser` instead")
		return ctx.QueryParser(out)
	}

	// No suitable content type found
	return fmt.Errorf("bodyparser: cannot parse content-type: %v", ctype)
}

// QueryParser binds the query string to a struct.
func (ctx *Ctx) QueryParser(out interface{}) error {
	if ctx.Fasthttp.QueryArgs().Len() < 1 {
		return nil
	}
	// Get decoder from pool
	var decoder = decoderPool.Get().(*schema.Decoder)
	defer decoderPool.Put(decoder)

	// Set correct alias tag
	decoder.SetAliasTag("query")

	data := make(map[string][]string)
	ctx.Fasthttp.QueryArgs().VisitAll(func(key []byte, val []byte) {
		data[getString(key)] = append(data[getString(key)], getString(val))
	})

	return decoder.Decode(out, data)
}

// ClearCookie expires a specific cookie by key on the client side.
// If no key is provided it expires all cookies that came with the request.
func (ctx *Ctx) ClearCookie(key ...string) {
	if len(key) > 0 {
		for i := range key {
			ctx.Fasthttp.Response.Header.DelClientCookie(key[i])
		}
		return
	}
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		ctx.Fasthttp.Response.Header.DelClientCookieBytes(k)
	})
}

// Context returns context.Context that carries a deadline, a cancellation signal,
// and other values across API boundaries.
func (ctx *Ctx) Context() context.Context {
	return ctx.Fasthttp
}

// Cookie sets a cookie by passing a cookie struct.
func (ctx *Ctx) Cookie(cookie *Cookie) {
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(cookie.Name)
	fcookie.SetValue(cookie.Value)
	fcookie.SetPath(cookie.Path)
	fcookie.SetDomain(cookie.Domain)
	fcookie.SetExpire(cookie.Expires)
	fcookie.SetSecure(cookie.Secure)
	fcookie.SetHTTPOnly(cookie.HTTPOnly)

	switch utils.ToLower(cookie.SameSite) {
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	}

	ctx.Fasthttp.Response.Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

// Cookies is used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (ctx *Ctx) Cookies(key string, defaultValue ...string) string {
	return defaultString(getString(ctx.Fasthttp.Request.Header.Cookie(key)), defaultValue)
}

// Download transfers the file from path as an attachment.
// Typically, browsers will prompt the user for download.
// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
// Override this default with the filename parameter.
func (ctx *Ctx) Download(file string, filename ...string) error {
	fname := filepath.Base(file)
	if len(filename) > 0 {
		fname = filename[0]
	}
	ctx.Set(HeaderContentDisposition, `attachment; filename="`+quoteString(fname)+`"`)
	return ctx.SendFile(file)
}

// Error contains the error information passed via the Next(err) method.
func (ctx *Ctx) Error() error {
	return ctx.err
}

// Format performs content-negotiation on the Accept HTTP header.
// It uses Accepts to select a proper format.
// If the header is not specified or there is no proper format, text/plain is used.
func (ctx *Ctx) Format(body interface{}) {
	// Get accepted content type
	accept := ctx.Accepts("html", "json", "txt", "xml")
	// Set accepted content type
	ctx.Type(accept)

	// Type convert provided body
	var b string
	switch val := body.(type) {
	case string:
		b = val
	case []byte:
		b = getString(val)
	default:
		b = fmt.Sprintf("%v", val)
	}

	// Format based on the accept content type
	switch accept {
	case "html":
		ctx.SendString("<p>" + b + "</p>")
	case "json":
		if err := ctx.JSON(body); err != nil {
			ctx.Send(body) // Fallback
			log.Println("Format: error serializing json ", err)
		}
	case "txt":
		ctx.SendString(b)
	case "xml":
		raw, err := xml.Marshal(body)
		if err != nil {
			ctx.Send(body) // Fallback
			log.Println("Format: error serializing xml ", err)
		} else {
			ctx.Fasthttp.Response.SetBodyRaw(raw)
		}
	default:
		ctx.SendString(b)
	}
}

// FormFile returns the first file by key from a MultipartForm.
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (ctx *Ctx) FormValue(key string) (value string) {
	return getString(ctx.Fasthttp.FormValue(key))
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L33
func (ctx *Ctx) Fresh() bool {
	// fields
	var modifiedSince = ctx.Get(HeaderIfModifiedSince)
	var noneMatch = ctx.Get(HeaderIfNoneMatch)

	// unconditional request
	if modifiedSince == "" && noneMatch == "" {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://tools.ietf.org/html/rfc2616#section-14.9.4
	cacheControl := ctx.Get(HeaderCacheControl)
	if cacheControl != "" && isNoCache(cacheControl) {
		return false
	}

	// if-none-match
	if noneMatch != "" && noneMatch != "*" {
		var etag = getString(ctx.Fasthttp.Response.Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if isEtagStale(etag, getBytes(noneMatch)) {
			return false
		}

		if modifiedSince != "" {
			var lastModified = getString(ctx.Fasthttp.Response.Header.Peek(HeaderLastModified))
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
func (ctx *Ctx) Get(key string, defaultValue ...string) string {
	return defaultString(getString(ctx.Fasthttp.Request.Header.Peek(key)), defaultValue)
}

// Hostname contains the hostname derived from the Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (ctx *Ctx) Hostname() string {
	return getString(ctx.Fasthttp.URI().Host())
}

// IP returns the remote IP address of the request.
func (ctx *Ctx) IP() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
func (ctx *Ctx) IPs() (ips []string) {
	header := ctx.Fasthttp.Request.Header.Peek(HeaderXForwardedFor)
	if len(header) == 0 {
		return
	}
	ips = make([]string, bytes.Count(header, []byte(","))+1)
	var commaPos, i int
	for {
		commaPos = bytes.IndexByte(header, ',')
		if commaPos != -1 {
			ips[i] = getString(header[:commaPos])
			header, i = header[commaPos+2:], i+1
		} else {
			ips[i] = getString(header)
			return
		}
	}
}

// Is returns the matching content type,
// if the incoming request's Content-Type HTTP header field matches the MIME type specified by the type parameter
func (ctx *Ctx) Is(extension string) bool {
	extensionHeader := utils.GetMIME(extension)
	if extensionHeader == "" {
		return false
	}

	return strings.HasPrefix(
		utils.TrimLeft(utils.GetString(ctx.Fasthttp.Request.Header.ContentType()), ' '),
		extensionHeader,
	)
}

// JSON converts any interface or string to JSON.
// This method also sets the content header to application/json.
func (ctx *Ctx) JSON(data interface{}) error {
	raw, err := json.Marshal(data)
	// Check for errors
	if err != nil {
		return err
	}
	// Set http headers
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	ctx.Fasthttp.Response.SetBodyRaw(raw)

	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (ctx *Ctx) JSONP(data interface{}, callback ...string) error {
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

	result = cb + "(" + getString(raw) + ");"

	ctx.Set(HeaderXContentTypeOptions, "nosniff")
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScriptCharsetUTF8)
	ctx.SendString(result)

	return nil
}

// Links joins the links followed by the property to populate the response's Link HTTP header field.
func (ctx *Ctx) Links(link ...string) {
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
	ctx.Set(HeaderLink, utils.TrimRight(getString(bb.Bytes()), ','))
	bytebufferpool.Put(bb)
}

// Locals makes it possible to pass interface{} values under string keys scoped to the request
// and therefore available to all following routes that match the request.
func (ctx *Ctx) Locals(key string, value ...interface{}) (val interface{}) {
	if len(value) == 0 {
		return ctx.Fasthttp.UserValue(key)
	}
	ctx.Fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Location sets the response Location HTTP header to the specified path parameter.
func (ctx *Ctx) Location(path string) {
	ctx.Set(HeaderLocation, path)
}

// Method contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
func (ctx *Ctx) Method(override ...string) string {
	if len(override) > 0 {
		method := utils.ToUpper(override[0])
		mINT := methodInt(method)
		if mINT == -1 {
			return ctx.method
		}
		ctx.method = method
		ctx.methodINT = mINT
	}
	return ctx.method
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key the value will be a string slice.
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// Next executes the next method in the stack that matches the current route.
// You can pass an optional error for custom error handling.
func (ctx *Ctx) Next(err ...error) {
	if len(err) > 0 {
		ctx.err = err[0]
		ctx.app.Settings.ErrorHandler(ctx, ctx.err)
		return
	}

	// Increment handler index
	ctx.indexHandler++
	// Did we executed all route handlers?
	if ctx.indexHandler < len(ctx.route.Handlers) {
		// Continue route stack
		ctx.route.Handlers[ctx.indexHandler](ctx)
	} else {
		// Continue handler stack
		ctx.app.next(ctx)
	}
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (ctx *Ctx) OriginalURL() string {
	return getString(ctx.Fasthttp.Request.Header.RequestURI())
}

// Params is used to get the route parameters.
// Defaults to empty string "" if the param doesn't exist.
// If a default value is given, it will return that value if the param doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (ctx *Ctx) Params(key string, defaultValue ...string) string {
	if key == "*" || key == "+" {
		key += "1"
	}
	for i := range ctx.route.Params {
		if len(key) != len(ctx.route.Params[i]) {
			continue
		}
		if ctx.route.Params[i] == key {
			// in case values are not here
			if len(ctx.values) <= i || len(ctx.values[i]) == 0 {
				break
			}
			return ctx.values[i]
		}
	}
	return defaultString("", defaultValue)
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
func (ctx *Ctx) Path(override ...string) string {
	if len(override) != 0 && ctx.path != override[0] {
		// Set new path to context
		ctx.path = override[0]
		ctx.pathOriginal = ctx.path
		// Set new path to request context
		ctx.Fasthttp.Request.URI().SetPath(ctx.pathOriginal)
		// Prettify path
		ctx.prettifyPath()
	}
	return ctx.pathOriginal
}

// Protocol contains the request protocol string: http or https for TLS requests.
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	scheme := "http"
	ctx.Fasthttp.Request.Header.VisitAll(func(key, val []byte) {
		if len(key) < 12 {
			return // X-Forwarded-
		} else if bytes.HasPrefix(key, []byte("X-Forwarded-")) {
			if bytes.Equal(key, []byte(HeaderXForwardedProto)) {
				scheme = getString(val)
			} else if bytes.Equal(key, []byte(HeaderXForwardedProtocol)) {
				scheme = getString(val)
			} else if bytes.Equal(key, []byte(HeaderXForwardedSsl)) && bytes.Equal(val, []byte("on")) {
				scheme = "https"
			}
		} else if bytes.Equal(key, []byte(HeaderXUrlScheme)) {
			scheme = getString(val)
		}
	})
	return scheme
}

// Query returns the query string parameter in the url.
// Defaults to empty string "" if the query doesn't exist.
// If a default value is given, it will return that value if the query doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (ctx *Ctx) Query(key string, defaultValue ...string) string {
	return defaultString(getString(ctx.Fasthttp.QueryArgs().Peek(key)), defaultValue)
}

var (
	ErrRangeMalformed     = errors.New("range: malformed range header string")
	ErrRangeUnsatisfiable = errors.New("range: unsatisfiable range")
)

// Range returns a struct containing the type and a slice of ranges.
func (ctx *Ctx) Range(size int) (rangeData Range, err error) {
	rangeStr := ctx.Get(HeaderRange)
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
func (ctx *Ctx) Redirect(location string, status ...int) {
	ctx.Set(HeaderLocation, location)
	if len(status) > 0 {
		ctx.Status(status[0])
	} else {
		ctx.Status(StatusFound)
	}
}

// Render a template with data and sends a text/html response.
// We support the following engines: html, amber, handlebars, mustache, pug
func (ctx *Ctx) Render(name string, bind interface{}, layouts ...string) (err error) {
	// Get new buffer from pool
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	if ctx.app.Settings.Views != nil {
		// Render template from Views
		if err := ctx.app.Settings.Views.Render(buf, name, bind, layouts...); err != nil {
			return err
		}
	} else {
		// Render raw template using 'name' as filepath if no engine is set
		var tmpl *template.Template
		if _, err = readContent(buf, name); err != nil {
			return err
		}
		// Parse template
		if tmpl, err = template.New("").Parse(getString(buf.Bytes())); err != nil {
			return err
		}
		buf.Reset()
		// Render template
		if err = tmpl.Execute(buf, bind); err != nil {
			return err
		}
	}
	// Set Content-Type to text/html
	ctx.Fasthttp.Response.Header.SetContentType(MIMETextHTMLCharsetUTF8)
	// Set rendered template to body
	ctx.Fasthttp.Response.SetBody(buf.Bytes())
	// Return err if exist
	return
}

// Route returns the matched Route struct.
func (ctx *Ctx) Route() *Route {
	if ctx.route == nil {
		// Fallback for fasthttp error handler
		return &Route{
			path:     ctx.pathOriginal,
			Path:     ctx.pathOriginal,
			Method:   ctx.method,
			Handlers: make([]Handler, 0),
		}
	}
	return ctx.route
}

// SaveFile saves any multipart file to disk.
func (ctx *Ctx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// Secure returns a boolean property, that is true, if a TLS connection is established.
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Send sets the HTTP response body. The input can be of any type, io.Reader is also supported.
func (ctx *Ctx) Send(bodies ...interface{}) {
	// Reset response body
	ctx.SendString("")
	// Write response body
	ctx.Write(bodies...)
}

// SendBytes sets the HTTP response body for []byte types without copying it.
// From this point onward the body argument must not be changed.
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyRaw(body)
}

var fsOnce sync.Once
var sendFileFS *fasthttp.FS
var sendFileHandler fasthttp.RequestHandler

// SendFile transfers the file from the given path.
// The file is not compressed by default, enable this by passing a 'true' argument
// Sets the Content-Type response HTTP header field based on the filenames extension.
func (ctx *Ctx) SendFile(file string, compress ...bool) error {
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	fsOnce.Do(func() {
		sendFileFS = &fasthttp.FS{
			Root:                 "/",
			GenerateIndexPages:   false,
			AcceptByteRange:      true,
			Compress:             true,
			CompressedFileSuffix: ctx.app.Settings.CompressedFileSuffix,
			CacheDuration:        10 * time.Second,
			IndexNames:           []string{"index.html"},
			PathNotFound: func(ctx *fasthttp.RequestCtx) {
				ctx.Response.SetStatusCode(StatusNotFound)
			},
		}
		sendFileHandler = sendFileFS.NewRequestHandler()
	})

	// Keep original path for mutable params
	ctx.pathOriginal = utils.ImmutableString(ctx.pathOriginal)
	// Disable compression
	if len(compress) <= 0 || !compress[0] {
		// https://github.com/valyala/fasthttp/blob/master/fs.go#L46
		ctx.Fasthttp.Request.Header.Del(HeaderAcceptEncoding)
	}
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L85
	if len(file) == 0 || file[0] != '/' {
		hasTrailingSlash := len(file) > 0 && file[len(file)-1] == '/'
		var err error
		if file, err = filepath.Abs(file); err != nil {
			return err
		}
		if hasTrailingSlash {
			file += "/"
		}
	}
	// Set new URI for fileHandler
	ctx.Fasthttp.Request.SetRequestURI(file)
	// Save status code
	status := ctx.Fasthttp.Response.StatusCode()
	// Serve file
	sendFileHandler(ctx.Fasthttp)
	// Get the status code which is set by fasthttp
	fsStatus := ctx.Fasthttp.Response.StatusCode()
	// Set the status code set by the user if it is different from the fasthttp status code and 200
	if status != fsStatus && status != StatusOK {
		ctx.Status(status)
	}
	// Check for error
	if status != StatusNotFound && fsStatus == StatusNotFound {
		return fmt.Errorf("sendfile: file %s not found", file)
	}
	return nil
}

// SendStatus sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
func (ctx *Ctx) SendStatus(status int) {
	ctx.Status(status)
	// Only set status body when there is no response body
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.SendString(utils.StatusMessage(status))
	}
}

// SendString sets the HTTP response body for string types.
// This means no type assertion, recommended for faster performance
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// SendStream sets response body stream and optional body size.
func (ctx *Ctx) SendStream(stream io.Reader, size ...int) {
	if len(size) > 0 && size[0] >= 0 {
		ctx.Fasthttp.Response.SetBodyStream(stream, size[0])
	} else {
		ctx.Fasthttp.Response.SetBodyStream(stream, -1)
		ctx.Set(HeaderContentLength, strconv.Itoa(len(ctx.Fasthttp.Response.Body())))
	}
}

// Set sets the response's HTTP header field to the specified key, value.
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.Set(key, val)
}

// Subdomains returns a string slice of subdomains in the domain name of the request.
// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
func (ctx *Ctx) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(ctx.Hostname(), ".")
	l := len(subdomains) - o
	// Check index to avoid slice bounds out of range panic
	if l < 0 {
		l = len(subdomains)
	}
	subdomains = subdomains[:l]
	return subdomains
}

// Stale is not implemented yet, pull requests are welcome!
func (ctx *Ctx) Stale() bool {
	return !ctx.Fresh()
}

// Status sets the HTTP status for the response.
// This method is chainable.
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
}

// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
func (ctx *Ctx) Type(extension string, charset ...string) *Ctx {
	if len(charset) > 0 {
		ctx.Fasthttp.Response.Header.SetContentType(utils.GetMIME(extension) + "; charset=" + charset[0])
	} else {
		ctx.Fasthttp.Response.Header.SetContentType(utils.GetMIME(extension))
	}
	return ctx
}

// Vary adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
func (ctx *Ctx) Vary(fields ...string) {
	ctx.Append(HeaderVary, fields...)
}

// Write appends any input to the HTTP body response, io.Reader is also supported as input.
func (ctx *Ctx) Write(bodies ...interface{}) {
	for i := range bodies {
		switch body := bodies[i].(type) {
		case string:
			ctx.Fasthttp.Response.AppendBodyString(body)
		case []byte:
			ctx.Fasthttp.Response.AppendBodyString(getString(body))
		case int:
			ctx.Fasthttp.Response.AppendBodyString(strconv.Itoa(body))
		case bool:
			ctx.Fasthttp.Response.AppendBodyString(strconv.FormatBool(body))
		case io.Reader:
			ctx.Fasthttp.Response.SetBodyStream(body, -1)
			ctx.Set(HeaderContentLength, strconv.Itoa(len(ctx.Fasthttp.Response.Body())))
		default:
			ctx.Fasthttp.Response.AppendBodyString(fmt.Sprintf("%v", body))
		}
	}
}

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (ctx *Ctx) XHR() bool {
	return strings.EqualFold(ctx.Get(HeaderXRequestedWith), "xmlhttprequest")
}

// prettifyPath ...
func (ctx *Ctx) prettifyPath() {
	// If UnescapePath enabled, we decode the path
	if ctx.app.Settings.UnescapePath {
		pathBytes := getBytes(ctx.path)
		pathBytes = fasthttp.AppendUnquotedArg(pathBytes[:0], pathBytes)
		ctx.path = getString(pathBytes)
	}
	// If CaseSensitive is disabled, we lowercase the original path
	if !ctx.app.Settings.CaseSensitive {
		// We are making a copy here to keep access to the original path
		ctx.path = utils.ToLower(ctx.path)
	}
	// If StrictRouting is disabled, we strip all trailing slashes
	if !ctx.app.Settings.StrictRouting && len(ctx.path) > 1 && ctx.path[len(ctx.path)-1] == '/' {
		ctx.path = utils.TrimRight(ctx.path, '/')
	}

	ctx.treePath = ctx.treePath[0:0]
	if len(ctx.path) >= 3 {
		ctx.treePath = ctx.path[:3]
	}
}
