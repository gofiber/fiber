// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	schema "github.com/gorilla/schema"
	"github.com/valyala/bytebufferpool"
	fasthttp "github.com/valyala/fasthttp"
)

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
type Ctx struct {
	// Internal fields
	app    *App     // Reference to *App
	route  *Route   // Reference to *Route
	index  int      // Index of the current handler in the stack
	method string   // HTTP method
	path   string   // HTTP path
	values []string // Route parameter values
	err    error    // Contains error if caught

	// External fields
	Fasthttp *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
}

// Range struct
type Range struct {
	Type   string
	Ranges []struct {
		Start int
		End   int
	}
}

// Cookie struct
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	Expires  time.Time
	Secure   bool
	HTTPOnly bool
	SameSite string
}

// Global variables
var schemaDecoderForm = schema.NewDecoder()
var schemaDecoderQuery = schema.NewDecoder()
var cacheControlNoCacheRegexp, _ = regexp.Compile(`/(?:^|,)\s*?no-cache\s*?(?:,|$)/`)

var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Ctx)
	},
}

// AcquireCtx from pool
func AcquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := ctxPool.Get().(*Ctx)
	// Set stack index
	ctx.index = -1
	// Set path
	ctx.path = getString(fctx.URI().Path())
	// Set method
	ctx.method = getString(fctx.Request.Header.Method())
	// Attach fasthttp request to ctx
	ctx.Fasthttp = fctx
	return ctx
}

// ReleaseCtx to pool
func ReleaseCtx(ctx *Ctx) {
	// Reset values
	ctx.route = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	ctx.err = nil
	ctxPool.Put(ctx)
}

// Accepts checks if the specified extensions or content types are acceptable.
func (ctx *Ctx) Accepts(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(HeaderAccept)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for i := range offers {
		mimetype := getMIME(offers[i])
		for k := range specs {
			spec := strings.TrimSpace(specs[k])
			if strings.HasPrefix(spec, "*/*") {
				return offers[i]
			}

			if strings.HasPrefix(spec, mimetype) {
				return offers[i]
			}

			if strings.Contains(spec, "/*") {
				if strings.HasPrefix(spec, strings.Split(mimetype, "/")[0]) {
					return offers[i]
				}
			}
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

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (ctx *Ctx) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := ctx.Fasthttp.Response.Header.Peek(field)
	for i := range values {
		var value = getBytes(values[i])
		if len(h) == 0 {
			h = append(h, value...)
		} else if 0 != bytes.Compare(h, value) && !bytes.HasSuffix(h, append([]byte{' '}, value...)) &&
			!bytes.Contains(h, append(append([]byte{}, value...), ',')) {
			h = append(append(h, ',', ' '), value...)
		}
	}
	ctx.Fasthttp.Response.Header.SetBytesV(field, h)
}

// Attachment sets the HTTP response Content-Disposition header field to attachment.
func (ctx *Ctx) Attachment(name ...string) {
	if len(name) > 0 {
		filename := filepath.Base(name[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set(HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set(HeaderContentDisposition, "attachment")
}

// BaseURL returns (protocol + host).
func (ctx *Ctx) BaseURL() string {
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// Body contains the raw body submitted in a POST request.
// If a key is provided, it returns the form value
func (ctx *Ctx) Body(key ...string) string {
	// Return request body
	if len(key) == 0 {
		return getString(ctx.Fasthttp.Request.Body())
	}
	// Return post value by key
	if len(key) > 0 {
		fmt.Println("DEPRECATED: c.Body(\"" + key[0] + "\") is deprecated, please use c.FormValue(\"" + key[0] + "\") instead.")
		return getString(ctx.Fasthttp.Request.PostArgs().Peek(key[0]))
	}
	return ""
}

// BodyParser binds the request body to a struct.
// It supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
func (ctx *Ctx) BodyParser(out interface{}) error {
	// get content type
	ctype := getString(ctx.Fasthttp.Request.Header.ContentType())
	// application/json
	if strings.HasPrefix(ctype, MIMEApplicationJSON) {
		return json.Unmarshal(ctx.Fasthttp.Request.Body(), out)
	}
	// application/xml text/xml
	if strings.HasPrefix(ctype, MIMEApplicationXML) || strings.HasPrefix(ctype, MIMETextXML) {
		return xml.Unmarshal(ctx.Fasthttp.Request.Body(), out)
	}
	// application/x-www-form-urlencoded
	if strings.HasPrefix(ctype, MIMEApplicationForm) {
		data, err := url.ParseQuery(getString(ctx.Fasthttp.PostBody()))
		if err != nil {
			return err
		}
		return schemaDecoderForm.Decode(out, data)
	}
	// multipart/form-data
	if strings.HasPrefix(ctype, MIMEMultipartForm) {
		data, err := ctx.Fasthttp.MultipartForm()
		if err != nil {
			return err
		}
		return schemaDecoderForm.Decode(out, data.Value)
	}
	// query params
	if ctx.Fasthttp.QueryArgs().Len() > 0 {
		data := make(map[string][]string)
		ctx.Fasthttp.QueryArgs().VisitAll(func(key []byte, val []byte) {
			data[getString(key)] = append(data[getString(key)], getString(val))
		})
		return schemaDecoderQuery.Decode(out, data)
	}

	return fmt.Errorf("BodyParser: cannot parse content-type: %v", ctype)
}

// ClearCookie expires a specific cookie by key.
// If no key is provided it expires all cookies.
func (ctx *Ctx) ClearCookie(key ...string) {
	if len(key) > 0 {
		for i := range key {
			ctx.Fasthttp.Response.Header.DelClientCookie(key[i])
		}
		return
	}
	//ctx.Fasthttp.Response.Header.DelAllCookies()
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		ctx.Fasthttp.Response.Header.DelClientCookie(getString(k))
	})
}

// Cookie sets a cookie by passing a cookie struct
func (ctx *Ctx) Cookie(cookie *Cookie) {
	fcookie := fasthttp.AcquireCookie()
	fcookie.SetKey(cookie.Name)
	fcookie.SetValue(cookie.Value)
	fcookie.SetPath(cookie.Path)
	fcookie.SetDomain(cookie.Domain)
	fcookie.SetExpire(cookie.Expires)
	fcookie.SetSecure(cookie.Secure)
	if cookie.Secure {
		// Secure must be paired with SameSite=None
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
	}
	fcookie.SetHTTPOnly(cookie.HTTPOnly)
	switch strings.ToLower(cookie.SameSite) {
	case "lax":
		fcookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	case "strict":
		fcookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	case "none":
		fcookie.SetSameSite(fasthttp.CookieSameSiteNoneMode)
		// Secure must be paired with SameSite=None
		fcookie.SetSecure(true)
	default:
		fcookie.SetSameSite(fasthttp.CookieSameSiteDisabled)
	}
	ctx.Fasthttp.Response.Header.SetCookie(fcookie)
	fasthttp.ReleaseCookie(fcookie)
}

// Cookies is used for getting a cookie value by key
func (ctx *Ctx) Cookies(key ...string) (value string) {
	if len(key) == 0 {
		fmt.Println("DEPRECATED: c.Cookies() without a key is deprecated, please use c.Get(\"Cookies\") to get the cookie header instead.")
		return ctx.Get(HeaderCookie)
	}
	return getString(ctx.Fasthttp.Request.Header.Cookie(key[0]))
}

// Download transfers the file from path as an attachment.
// Typically, browsers will prompt the user for download.
// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
// Override this default with the filename parameter.
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)

	if len(name) > 0 {
		filename = name[0]
	}

	ctx.Set(HeaderContentDisposition, "attachment; filename="+filename)
	ctx.SendFile(file)
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
	case "text":
		ctx.SendString(b)
	case "xml":
		raw, err := xml.Marshal(body)
		if err != nil {
			ctx.Send(body) // Fallback
			log.Println("Format: error serializing xml ", err)
		} else {
			ctx.SendString(getString(raw))
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
func (ctx *Ctx) FormValue(key string) (value string) {
	return getString(ctx.Fasthttp.FormValue(key))
}

// Fresh When the response is still “fresh” in the client’s cache true is returned,
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
	var cacheControl = ctx.Get(HeaderCacheControl)
	if cacheControl != "" && cacheControlNoCacheRegexp.MatchString(cacheControl) {
		return false
	}

	// if-none-match
	if noneMatch != "" && noneMatch != "*" {
		var etag = getString(ctx.Fasthttp.Response.Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		var etagStal = true
		var matches = parseTokenList(getBytes(noneMatch))
		for i := range matches {
			match := matches[i]
			if match == etag || match == "W/"+etag || "W/"+match == etag {
				etagStal = false
				break
			}
		}
		if etagStal {
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
func (ctx *Ctx) Get(key string) (value string) {
	if key == "referrer" {
		key = "referer"
	}
	return getString(ctx.Fasthttp.Request.Header.Peek(key))
}

// Hostname contains the hostname derived from the Host HTTP header.
func (ctx *Ctx) Hostname() string {
	return getString(ctx.Fasthttp.URI().Host())
}

// IP returns the remote IP address of the request.
func (ctx *Ctx) IP() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// IPs returns an string slice of IP addresses specified in the X-Forwarded-For request header.
func (ctx *Ctx) IPs() []string {
	ips := strings.Split(ctx.Get(HeaderXForwardedFor), ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// Is returns the matching content type,
// if the incoming request’s Content-Type HTTP header field matches the MIME type specified by the type parameter
func (ctx *Ctx) Is(extension string) (match bool) {
	if extension[0] != '.' {
		extension = "." + extension
	}

	exts, _ := mime.ExtensionsByType(ctx.Get(HeaderContentType))
	if len(exts) > 0 {
		for i := range exts {
			if exts[i] == extension {
				return true
			}
		}
	}
	return
}

// JSON converts any interface or string to JSON using Jsoniter.
// This method also sets the content header to application/json.
func (ctx *Ctx) JSON(data interface{}) error {
	raw, err := json.Marshal(&data)
	// Check for errors
	if err != nil {
		return err
	}
	// Set http headers
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	ctx.Fasthttp.Response.SetBodyString(getString(raw))
	// Success!
	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (ctx *Ctx) JSONP(data interface{}, callback ...string) error {
	raw, err := json.Marshal(&data)

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

	ctx.Fasthttp.Response.Header.Set(HeaderXContentTypeOptions, "nosniff")
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScript)
	ctx.Fasthttp.Response.SetBodyString(result)

	return nil
}

// Links joins the links followed by the property to populate the response’s Link HTTP header field.
// #nosec G104
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
			_, _ = bb.WriteString(`; rel="`)
			_, _ = bb.WriteString(link[i])
			_, _ = bb.WriteString(`",`)
		}
	}
	ctx.Fasthttp.Response.Header.Set(HeaderLink, strings.TrimSuffix(bb.String(), ","))
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
		ctx.method = strings.ToUpper(override[0])
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
	if ctx.app == nil {
		return
	}
	ctx.route = nil
	ctx.values = nil
	if len(err) > 0 {
		ctx.err = err[0]
	}
	ctx.app.nextRoute(ctx)
}

// OriginalURL contains the original request URL.
func (ctx *Ctx) OriginalURL() string {
	return getString(ctx.Fasthttp.Request.Header.RequestURI())
}

// Params is used to get the route parameters.
// Defaults to empty string "", if the param doesn't exist.
func (ctx *Ctx) Params(key string) string {
	for i := range ctx.route.Params {
		if len(key) != len(ctx.route.Params[i]) {
			continue
		}
		if ctx.route.Params[i] == key {
			return ctx.values[i]
		}
	}
	return ""
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
func (ctx *Ctx) Path(override ...string) string {
	if len(override) > 0 && ctx.app != nil {
		// Non strict routing
		if !ctx.app.Settings.StrictRouting && len(override[0]) > 1 {
			override[0] = strings.TrimRight(override[0], "/")
		}
		// Not case sensitive
		if !ctx.app.Settings.CaseSensitive {
			override[0] = strings.ToLower(override[0])
		}
		ctx.path = override[0]
	}
	return ctx.path
}

// Protocol contains the request protocol string: http or https for TLS requests.
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Query returns the query string parameter in the url.
func (ctx *Ctx) Query(key string) (value string) {
	return getString(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Range returns a struct containing the type and a slice of ranges.
func (ctx *Ctx) Range(size int) (rangeData Range, err error) {
	rangeStr := getString(ctx.Fasthttp.Request.Header.Peek(HeaderRange))
	if rangeStr == "" || !strings.Contains(rangeStr, "=") {
		return rangeData, fmt.Errorf("malformed range header string")
	}
	data := strings.Split(rangeStr, "=")
	rangeData.Type = data[0]
	arr := strings.Split(data[1], ",")
	for i := 0; i < len(arr); i++ {
		item := strings.Split(arr[i], "-")
		if len(item) == 1 {
			return rangeData, fmt.Errorf("malformed range header string")
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
		return rangeData, fmt.Errorf("unsatisfiable range")
	}
	return rangeData, nil
}

// Redirect to the URL derived from the specified path, with specified status.
// If status is not specified, status defaults to 302 Found
func (ctx *Ctx) Redirect(path string, status ...int) {
	ctx.Fasthttp.Response.Header.Set(HeaderLocation, path)
	if len(status) > 0 {
		ctx.Fasthttp.Response.SetStatusCode(status[0])
	} else {
		ctx.Fasthttp.Response.SetStatusCode(StatusFound)
	}
}

// Render a template with data and sends a text/html response.
// We support the following engines: html, amber, handlebars, mustache, pug
func (ctx *Ctx) Render(file string, bind interface{}) error {
	var err error
	var raw []byte
	var html string
	if ctx.app != nil {
		if ctx.app.Settings.TemplateFolder != "" {
			file = filepath.Join(ctx.app.Settings.TemplateFolder, file)
		}
		if ctx.app.Settings.TemplateExtension != "" {
			file = file + ctx.app.Settings.TemplateExtension
		}
		if raw, err = ioutil.ReadFile(filepath.Clean(file)); err != nil {
			return err
		}
	}
	if ctx.app != nil && ctx.app.Settings.TemplateEngine != nil {
		// Custom template engine
		// https://github.com/gofiber/template
		if html, err = ctx.app.Settings.TemplateEngine(getString(raw), bind); err != nil {
			return err
		}
	} else {
		// Default template engine
		// https://golang.org/pkg/text/template/
		var buf bytes.Buffer
		var tmpl *template.Template

		if tmpl, err = template.New("").Parse(getString(raw)); err != nil {
			return err
		}
		if err = tmpl.Execute(&buf, bind); err != nil {
			return err
		}
		html = buf.String()
	}
	ctx.Set("Content-Type", "text/html")
	ctx.SendString(html)
	return err
}

// Route returns the matched Route struct.
func (ctx *Ctx) Route() *Route {
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

// Send sets the HTTP response body. The Send body can be of any type.
func (ctx *Ctx) Send(bodies ...interface{}) {
	if len(bodies) > 0 {
		ctx.Fasthttp.Response.SetBodyString("")
	}
	ctx.Write(bodies...)
}

// SendBytes sets the HTTP response body for []byte types
// This means no type assertion, recommended for faster performance
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(getString(body))
}

// SendFile transfers the file from the given path.
// The file is compressed by default
// Sets the Content-Type response HTTP header field based on the filenames extension.
func (ctx *Ctx) SendFile(file string, noCompression ...bool) {
	// Disable gzipping
	if len(noCompression) > 0 && noCompression[0] {
		fasthttp.ServeFileUncompressed(ctx.Fasthttp, file)
		return
	}
	fasthttp.ServeFile(ctx.Fasthttp, file)
}

// SendStatus sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
func (ctx *Ctx) SendStatus(status int) {
	ctx.Fasthttp.Response.SetStatusCode(status)
	// Only set status body when there is no response body
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.Fasthttp.Response.SetBodyString(statusMessage[status])
	}
}

// SendString sets the HTTP response body for string types
// This means no type assertion, recommended for faster performance
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// Set sets the response’s HTTP header field to the specified key, value.
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
	subdomains = subdomains[:len(subdomains)-o]
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
func (ctx *Ctx) Type(ext string) *Ctx {
	ctx.Fasthttp.Response.Header.SetContentType(getMIME(ext))
	return ctx
}

// Vary adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
func (ctx *Ctx) Vary(fields ...string) {
	ctx.Append(HeaderVary, fields...)
}

// Write appends any input to the HTTP body response.
func (ctx *Ctx) Write(bodies ...interface{}) {
	for i := range bodies {
		switch body := bodies[i].(type) {
		case string:
			ctx.Fasthttp.Response.AppendBodyString(body)
		case []byte:
			ctx.Fasthttp.Response.AppendBodyString(getString(body))
		default:
			ctx.Fasthttp.Response.AppendBodyString(fmt.Sprintf("%v", body))
		}
	}
}

// XHR returns a Boolean property, that is true, if the request’s X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (ctx *Ctx) XHR() bool {
	return strings.EqualFold(ctx.Get(HeaderXRequestedWith), "xmlhttprequest")
}
