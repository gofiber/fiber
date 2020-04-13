// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	schema "github.com/gorilla/schema"
	jsoniter "github.com/json-iterator/go"
	fasthttp "github.com/valyala/fasthttp"
)

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
type Ctx struct {
	app      *App                 // Reference to *App
	route    *Route               // Reference to *Route
	index    int                  // Index of the current stack
	method   string               // HTTP method
	path     string               // HTTP path
	values   []string             // Route parameter values
	Fasthttp *fasthttp.RequestCtx // Reference to *fasthttp.RequestCtx
	err      error                // Contains error if catched
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
var jsonParser = jsoniter.ConfigCompatibleWithStandardLibrary
var schemaDecoderForm = schema.NewDecoder()
var schemaDecoderQuery = schema.NewDecoder()

// Ctx pool
var poolCtx = sync.Pool{
	New: func() interface{} {
		return new(Ctx)
	},
}

// Acquire Ctx from pool
func acquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := poolCtx.Get().(*Ctx)
	ctx.index = -1
	ctx.path = getString(fctx.URI().Path())
	ctx.method = getString(fctx.Request.Header.Method())
	ctx.Fasthttp = fctx
	return ctx
}

// Return Ctx to pool
func releaseCtx(ctx *Ctx) {
	ctx.route = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	ctx.err = nil
	poolCtx.Put(ctx)
}

// Accepts checks if the specified extensions or content types are acceptable.
func (ctx *Ctx) Accepts(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(HeaderAccept)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, value := range offers {
		mimetype := getMIME(value)
		// if mimetype != "" {
		// 	mimetype = strings.Split(mimetype, ";")[0]
		// } else {
		// 	mimetype = offer
		// }
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*/*") {
				return value
			}

			if strings.HasPrefix(spec, mimetype) {
				return value
			}

			if strings.Contains(spec, "/*") {
				if strings.HasPrefix(spec, strings.Split(mimetype, "/")[0]) {
					return value
				}
			}
		}
	}
	return ""
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (ctx *Ctx) AcceptsCharsets(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(HeaderAcceptCharset)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, value := range offers {
		for _, spec := range specs {

			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return value
			}
			if strings.HasPrefix(spec, value) {
				return value
			}
		}
	}
	return ""
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (ctx *Ctx) AcceptsEncodings(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(HeaderAcceptEncoding)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, value := range offers {
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return value
			}
			if strings.HasPrefix(spec, value) {
				return value
			}
		}
	}
	return ""
}

// AcceptsLanguages checks if the specified language is acceptable.
func (ctx *Ctx) AcceptsLanguages(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(HeaderAcceptLanguage)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, value := range offers {
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return value
			}
			if strings.HasPrefix(spec, value) {
				return value
			}
		}
	}
	return ""
}

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (ctx *Ctx) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := getString(ctx.Fasthttp.Response.Header.Peek(field))
	for i := range values {
		if h == "" {
			h += values[i]
		} else {
			h += ", " + values[i]
		}
	}
	ctx.Set(field, h)
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
		return jsoniter.Unmarshal(ctx.Fasthttp.Request.Body(), out)
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
	// query Params
	if ctx.Fasthttp.QueryArgs().Len() > 0 {
		data := make(map[string][]string)
		ctx.Fasthttp.QueryArgs().VisitAll(func(key []byte, val []byte) {
			data[getString(key)] = []string{getString(val)}
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
	fcookie := &fasthttp.Cookie{}
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
}

// Cookies is used for getting a cookie value by key
func (ctx *Ctx) Cookies(key ...string) (value string) {
	if len(key) == 0 {
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
	var b string
	accept := ctx.Accepts("html", "json")

	switch val := body.(type) {
	case string:
		b = val
	case []byte:
		b = getString(val)
	default:
		b = fmt.Sprintf("%v", val)
	}
	switch accept {
	case "html":
		ctx.SendString("<p>" + b + "</p>")
	case "json":
		if err := ctx.JSON(body); err != nil {
			// Fix
			log.Println("Format: error serializing json ", err)
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

// Fresh is not implemented yet, pull requests are welcome!
func (ctx *Ctx) Fresh() bool {
	return false
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
		for _, item := range exts {
			if item == extension {
				return true
			}
		}
	}
	return
}

// JSON converts any interface or string to JSON using Jsoniter.
// This method also sets the content header to application/json.
func (ctx *Ctx) JSON(json interface{}) error {
	// Get stream from pool
	stream := jsonParser.BorrowStream(nil)
	defer jsonParser.ReturnStream(stream)
	// Write struct to stream
	stream.WriteVal(&json)
	// Check for errors
	if stream.Error != nil {
		return stream.Error
	}
	// Set http headers
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	ctx.Fasthttp.Response.SetBodyString(getString(stream.Buffer()))
	// Success!
	return nil
}

// JSONP sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
func (ctx *Ctx) JSONP(json interface{}, callback ...string) error {
	// Get stream from pool
	stream := jsonParser.BorrowStream(nil)
	defer jsonParser.ReturnStream(stream)
	// Write struct to stream
	stream.WriteVal(&json)
	// Check for errors
	if stream.Error != nil {
		return stream.Error
	}

	str := "callback("
	if len(callback) > 0 {
		str = callback[0] + "("
	}
	str += getString(stream.Buffer()) + ");"

	ctx.Set(HeaderXContentTypeOptions, "nosniff")
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScript)
	ctx.Fasthttp.Response.SetBodyString(str)

	return nil
}

// Links joins the links followed by the property to populate the response’s Link HTTP header field.
func (ctx *Ctx) Links(link ...string) {
	h := ""
	for i, l := range link {
		if i%2 == 0 {
			h += "<" + l + ">"
		} else {
			h += `; rel="` + l + `",`
		}
	}

	if len(link) > 0 {
		h = strings.TrimSuffix(h, ",")
		ctx.Set(HeaderLink, h)
	}
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
		ctx.method = override[0]
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
func (ctx *Ctx) Params(key string) (value string) {
	if ctx.route.Params == nil {
		return
	}
	for i := 0; i < len(ctx.route.Params); i++ {
		if (ctx.route.Params)[i] == key {
			return ctx.values[i]
		}
	}
	return
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
func (ctx *Ctx) Path(override ...string) string {
	if len(override) > 0 {
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
	rangeStr := string(ctx.Fasthttp.Request.Header.Peek(HeaderRange))
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
	code := 302
	if len(status) > 0 {
		code = status[0]
	}

	ctx.Set(HeaderLocation, path)
	ctx.Fasthttp.Response.SetStatusCode(code)
}

// Render a template with data and sends a text/html response.
// We support the following engines: html, amber, handlebars, mustache, pug
func (ctx *Ctx) Render(file string, bind interface{}) error {
	var err error
	var raw []byte
	var html string

	if ctx.app.Settings.TemplateFolder != "" {
		file = filepath.Join(ctx.app.Settings.TemplateFolder, file)
	}
	if ctx.app.Settings.TemplateExtension != "" {
		file = file + ctx.app.Settings.TemplateExtension
	}
	if raw, err = ioutil.ReadFile(filepath.Clean(file)); err != nil {
		return err
	}
	if ctx.app.Settings.TemplateEngine != nil {
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
		ctx.Fasthttp.Response.SetBodyString(statusMessages[status])
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

// Subdomains returns a string slive of subdomains in the domain name of the request.
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
	if len(fields) == 0 {
		return
	}

	h := getString(ctx.Fasthttp.Response.Header.Peek(HeaderVary))
	for i := range fields {
		if h == "" {
			h += fields[i]
		} else {
			h += ", " + fields[i]
		}
	}

	ctx.Set(HeaderVary, h)
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
	return strings.ToLower(ctx.Get(HeaderXRequestedWith)) == "xmlhttprequest"
}
