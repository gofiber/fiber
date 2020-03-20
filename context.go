// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"encoding/xml"
	"fmt"
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

	websocket "github.com/fasthttp/websocket"
	template "github.com/gofiber/template"
	jsoniter "github.com/json-iterator/go"
	fasthttp "github.com/valyala/fasthttp"
)

// Ctx represents the Context which hold the HTTP request and response.
// It has methods for the request query string, parameters, body, HTTP headers and so on.
// For more information please visit our documentation: https://fiber.wiki/context
type Ctx struct {
	app      *App                 // Reference to *App
	route    *Route               // Reference to *Route
	index    int                  // Index of the current stack
	method   string               // HTTP method
	path     string               // HTTP path
	values   []string             // Route parameter values
	compress bool                 // If the response needs to be compressed
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
	ctx.compress = false
	ctx.Fasthttp = nil
	ctx.err = nil
	poolCtx.Put(ctx)
}

// Conn https://godoc.org/github.com/gorilla/websocket#pkg-index
type Conn struct {
	*websocket.Conn
}

// Conn pool
var poolConn = sync.Pool{
	New: func() interface{} {
		return new(Conn)
	},
}

// Acquire Conn from pool
func acquireConn(fconn *websocket.Conn) *Conn {
	conn := poolConn.Get().(*Conn)
	conn.Conn = fconn
	return conn
}

// Return Conn to pool
func releaseConn(conn *Conn) {
	conn.Conn = nil
	poolConn.Put(conn)
}

// Checks, if the specified extensions or content types are acceptable.
//
// https://fiber.wiki/context#accepts
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

// Checks, if the specified charset is acceptable.
//
// https://fiber.wiki/context#accepts
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

// Checks, if the specified encoding is acceptable.
//
// https://fiber.wiki/context#accepts
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

// Checks, if the specified language is acceptable.
//
// https://fiber.wiki/context#accepts
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

// Appends the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
//
// https://fiber.wiki/context#append
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

// Sets the HTTP response Content-Disposition header field to attachment.
//
// https://fiber.wiki/context#attachment
func (ctx *Ctx) Attachment(name ...string) {
	if len(name) > 0 {
		filename := filepath.Base(name[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set(HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set(HeaderContentDisposition, "attachment")
}

// Returns base URL (protocol + host) as a string.
//
// https://fiber.wiki/context#baseurl
func (ctx *Ctx) BaseURL() string {
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// Contains the raw body submitted in a POST request.
// If a key is provided, it returns the form value
//
// https://fiber.wiki/context#body
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

// Binds the request body to a struct.
// BodyParser supports decoding the following content types based on the Content-Type header:
// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
//
// https://fiber.wiki/context#bodyparser
func (ctx *Ctx) BodyParser(out interface{}) error {
	// TODO : Query Params
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
		return schemaDecoder.Decode(out, data)
	}
	// multipart/form-data
	if strings.HasPrefix(ctype, MIMEMultipartForm) {
		data, err := ctx.Fasthttp.MultipartForm()
		if err != nil {
			return err
		}
		return schemaDecoder.Decode(out, data.Value)

	}
	return fmt.Errorf("BodyParser: cannot parse content-type: %v", ctype)
}

// Expire a client cookie (or all cookies if left empty)
//
// https://fiber.wiki/context#clearcookie
func (ctx *Ctx) ClearCookie(key ...string) {
	if len(key) > 0 {
		for i := range key {
			//ctx.Fasthttp.Request.Header.DelAllCookies()
			ctx.Fasthttp.Response.Header.DelClientCookie(key[i])
		}
		return
	}
	//ctx.Fasthttp.Response.Header.DelAllCookies()
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		ctx.Fasthttp.Response.Header.DelClientCookie(getString(k))
	})
}

// Set cookie by passing a cookie struct
//
// https://fiber.wiki/context#cookie
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

// Get cookie value by key
//
// https://fiber.wiki/context#cookies
func (ctx *Ctx) Cookies(key ...string) (value string) {
	if len(key) == 0 {
		return ctx.Get(HeaderCookie)
	}
	return getString(ctx.Fasthttp.Request.Header.Cookie(key[0]))
}

// Transfers the file from path as an attachment.
// Typically, browsers will prompt the user for download.
// By default, the Content-Disposition header filename= parameter is the filepath (this typically appears in the browser dialog).
// Override this default with the filename parameter.
//
// Download : https://fiber.wiki/context#download
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)

	if len(name) > 0 {
		filename = name[0]
	}

	ctx.Set(HeaderContentDisposition, "attachment; filename="+filename)
	ctx.SendFile(file)
}

// This contains the error information that thrown by a panic or passed via the Next(err) method.
//
// https://fiber.wiki/context#error
func (ctx *Ctx) Error() error {
	return ctx.err
}

// Performs content-negotiation on the Accept HTTP header. It uses Accepts to select a proper format.
// If the header is not specified or there is no proper format, text/plain is used.
//
// https://fiber.wiki/context#format
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
			log.Println("Format: error serializing json ", err)
		}
	default:
		ctx.SendString(b)
	}
}

// MultipartForm files can be retrieved by name, the first file from the given key is returned.
//
// https://fiber.wiki/context#formfile
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// MultipartForm values can be retrieved by name, the first value from the given key is returned.
//
// https://fiber.wiki/context#formvalue
func (ctx *Ctx) FormValue(key string) (value string) {
	return getString(ctx.Fasthttp.FormValue(key))
}

// Not implemented yet, pull requests are welcome!
//
// https://fiber.wiki/context#fresh
func (ctx *Ctx) Fresh() bool {
	return false
}

// Returns the HTTP request header specified by field.
// Field names are case-insensitive
//
// https://fiber.wiki/context#get
func (ctx *Ctx) Get(key string) (value string) {
	if key == "referrer" {
		key = "referer"
	}
	return getString(ctx.Fasthttp.Request.Header.Peek(key))
}

// Contains the hostname derived from the Host HTTP header.
//
// https://fiber.wiki/context#hostname
func (ctx *Ctx) Hostname() string {
	return getString(ctx.Fasthttp.URI().Host())
}

// Returns the remote IP address of the request.
//
// https://fiber.wiki/context#Ip
func (ctx *Ctx) IP() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// Returns an string slice of IP addresses specified in the X-Forwarded-For request header.
//
// https://fiber.wiki/context#ips
func (ctx *Ctx) IPs() []string {
	ips := strings.Split(ctx.Get(HeaderXForwardedFor), ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// Returns the matching content type,
// if the incoming request’s Content-Type HTTP header field matches the MIME type specified by the type parameter.
//
// https://fiber.wiki/context#is
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

// Converts any interface or string to JSON using Jsoniter.
// This method also sets the content header to application/json.
//
// https://fiber.wiki/context#json
func (ctx *Ctx) JSON(json interface{}) error {
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	raw, err := jsoniter.Marshal(&json)
	if err != nil {
		ctx.Fasthttp.Response.SetBodyString("")
		return err
	}
	ctx.Fasthttp.Response.SetBodyString(getString(raw))

	return nil
}

// Sends a JSON response with JSONP support.
// This method is identical to JSON, except that it opts-in to JSONP callback support.
// By default, the callback name is simply callback.
//
// https://fiber.wiki/context#jsonp
func (ctx *Ctx) JSONP(json interface{}, callback ...string) error {
	raw, err := jsoniter.Marshal(&json)
	if err != nil {
		return err
	}

	str := "callback("
	if len(callback) > 0 {
		str = callback[0] + "("
	}
	str += getString(raw) + ");"

	ctx.Set(HeaderXContentTypeOptions, "nosniff")
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScript)
	ctx.Fasthttp.Response.SetBodyString(str)

	return nil
}

// Joins the links followed by the property to populate the response’s Link HTTP header field.
//
// https://fiber.wiki/context#links
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

// You can pass interface{} values under string keys scoped to the request
// and therefore available to all routes that match the request.
//
// https://fiber.wiki/context#locals
func (ctx *Ctx) Locals(key string, value ...interface{}) (val interface{}) {
	if len(value) == 0 {
		return ctx.Fasthttp.UserValue(key)
	}
	ctx.Fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Sets the response Location HTTP header to the specified path parameter.
//
// https://fiber.wiki/context#location
func (ctx *Ctx) Location(path string) {
	ctx.Set(HeaderLocation, path)
}

// Contains a string corresponding to the HTTP method of the request: GET, POST, PUT and so on.
//
// https://fiber.wiki/context#method
func (ctx *Ctx) Method(override ...string) string {
	if len(override) > 0 {
		ctx.method = override[0]
	}
	return ctx.method
}

// Access multipart form entries, you can parse the binary with MultipartForm().
// This returns a map[string][]string, so given a key the value will be a string slice.
//
// https://fiber.wiki/context#multipartform
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// Next executes the next method in the stack that matches the current route.
// You can pass an optional error for custom error handling.
//
// https://fiber.wiki/context#next
func (ctx *Ctx) Next(err ...error) {
	ctx.route = nil
	ctx.values = nil
	if len(err) > 0 {
		ctx.err = err[0]
	}
	ctx.app.nextRoute(ctx)
}

// Contains the original request URL.
//
// https://fiber.wiki/context#originalurl
func (ctx *Ctx) OriginalURL() string {
	return getString(ctx.Fasthttp.Request.Header.RequestURI())
}

// Used to get the route parameters.
// Defaults to empty string "", if the param doesn't exist.
//
// https://fiber.wiki/context#params
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

// Returns the path part of the request URL.
// Optionally, you could override the path.
//
// https://fiber.wiki/context#path
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

// Contains the request protocol string: http or https for TLS requests.
//
// https://fiber.wiki/context#protocol
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Returns the query string parameter in the url.
//
// https://fiber.wiki/context#query
func (ctx *Ctx) Query(key string) (value string) {
	return getString(ctx.Fasthttp.QueryArgs().Peek(key))
}

// An struct containing the type and a slice of ranges will be returned.
//
// https://fiber.wiki/context#range
func (ctx *Ctx) Range(size int) (rangeData Range, err error) {
	rangeStr := string(ctx.Fasthttp.Request.Header.Peek("range"))
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

// Redirects to the URL derived from the specified path, with specified status.
// If status is not specified, status defaults to 302 Found
//
// https://fiber.wiki/context#redirect
func (ctx *Ctx) Redirect(path string, status ...int) {
	code := 302
	if len(status) > 0 {
		code = status[0]
	}

	ctx.Set(HeaderLocation, path)
	ctx.Fasthttp.Response.SetStatusCode(code)
}

// Renders a template with data and sends a text/html response.
// We support the following engines: html, amber, handlebars, mustache, pug
//
// https://fiber.wiki/context#render
func (ctx *Ctx) Render(file string, bind interface{}, engine ...string) error {
	var err error
	var raw []byte
	var html string
	var e string

	if len(engine) > 0 {
		e = engine[0]
	} else if ctx.app.Settings.TemplateEngine != "" {
		e = ctx.app.Settings.TemplateEngine
	} else {
		e = filepath.Ext(file)[1:]
	}
	if ctx.app.Settings.TemplateFolder != "" {
		file = filepath.Join(ctx.app.Settings.TemplateFolder, file)
	}
	if ctx.app.Settings.TemplateExtension != "" {
		file = file + ctx.app.Settings.TemplateExtension
	}
	if raw, err = ioutil.ReadFile(filepath.Clean(file)); err != nil {
		return err
	}

	switch e {
	case "amber": // https://github.com/eknkc/amber
		if html, err = template.Amber(getString(raw), bind); err != nil {
			return err
		}
	case "handlebars": // https://github.com/aymerick/raymond
		if html, err = template.Handlebars(getString(raw), bind); err != nil {
			return err
		}
	case "mustache": // https://github.com/cbroglie/mustache
		if html, err = template.Mustache(getString(raw), bind); err != nil {
			return err
		}
	case "pug": // https://github.com/Joker/jade
		if html, err = template.Pug(getString(raw), bind); err != nil {
			return err
		}
	default: // https://golang.org/pkg/text/template/
		if html, err = template.HTML(getString(raw), bind); err != nil {
			return err
		}
	}
	ctx.Set("Content-Type", "text/html")
	ctx.SendString(html)
	return err
}

// Returns the matched Route struct.
//
// https://fiber.wiki/context#route
func (ctx *Ctx) Route() *Route {
	return ctx.route
}

// Save any multipart file to disk.
//
// https://fiber.wiki/context#secure
func (ctx *Ctx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// A boolean property, that is true, if a TLS connection is established.
//
// https://fiber.wiki/context#secure
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Sets the HTTP response body. The Send body can be of any type.
//
// https://fiber.wiki/context#send
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

// Sets the HTTP response body for []byte types
// This means no type assertion, recommended for faster performance
//
// https://fiber.wiki/context#send
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(getString(body))
}

// Transfers the file from the given path.
// Sets the Content-Type response HTTP header field based on the filenames extension.
//
// https://fiber.wiki/context#sendfile
func (ctx *Ctx) SendFile(file string, compress ...bool) {
	// Disable gzipping
	if len(compress) > 0 && !compress[0] {
		fasthttp.ServeFileUncompressed(ctx.Fasthttp, file)
		return
	}
	fasthttp.ServeFile(ctx.Fasthttp, file)
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// Sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
//
// https://fiber.wiki/context#sendstatus
func (ctx *Ctx) SendStatus(status int) {
	ctx.Fasthttp.Response.SetStatusCode(status)
	// Only set status body when there is no response body
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.Fasthttp.Response.SetBodyString(statusMessages[status])
	}
}

// Sets the HTTP response body for string types
// This means no type assertion, recommended for faster performance
//
// https://fiber.wiki/context#send
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// Sets the response’s HTTP header field to the specified key, value.
//
// https://fiber.wiki/context#set
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.Set(key, val)
}

// Returns a string slive of subdomains in the domain name of the request.
// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
//
// https://fiber.wiki/context#subdomains
func (ctx *Ctx) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(ctx.Hostname(), ".")
	subdomains = subdomains[:len(subdomains)-o]
	return subdomains
}

// Not implemented yet, pull requests are welcome!
//
// https://fiber.wiki/context#stale
func (ctx *Ctx) Stale() bool {
	return !ctx.Fresh()
}

// Sets the HTTP status for the response.
// This method is chainable.
//
// https://fiber.wiki/context#status
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
}

// Sets the Content-Type HTTP header to the MIME type specified by the file extension.
//
// https://fiber.wiki/context#type
func (ctx *Ctx) Type(ext string) *Ctx {
	ctx.Fasthttp.Response.Header.SetContentType(getMIME(ext))
	return ctx
}

// Adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
//
// https://fiber.wiki/context#vary
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

// Appends any input to the HTTP body response.
//
// https://fiber.wiki/context#write
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

// A Boolean property, that is true, if the request’s X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
//
// https://fiber.wiki/context#xhr
func (ctx *Ctx) XHR() bool {
	return ctx.Get(HeaderXRequestedWith) == "XMLHttpRequest"
}
