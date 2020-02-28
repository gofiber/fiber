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
	app      *App
	route    *Route
	next     bool
	error    error
	params   *[]string
	values   []string
	compress int
	Fasthttp *fasthttp.RequestCtx
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
	ctx.Fasthttp = fctx
	return ctx
}

// Return Ctx to pool
func releaseCtx(ctx *Ctx) {
	ctx.route = nil
	ctx.next = false
	ctx.error = nil
	ctx.params = nil
	ctx.values = nil
	ctx.compress = 0
	ctx.Fasthttp = nil
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

// Cookie : struct
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	Expires  time.Time
	Secure   bool
	HTTPOnly bool
}

// Accepts : https://fiber.wiki/context#accepts
func (ctx *Ctx) Accepts(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(fasthttp.HeaderAccept)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, value := range offers {
		mimetype := getType(value)
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

// AcceptsCharsets : https://fiber.wiki/context#acceptscharsets
func (ctx *Ctx) AcceptsCharsets(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(fasthttp.HeaderAcceptCharset)
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

// AcceptsEncodings : https://fiber.wiki/context#acceptsencodings
func (ctx *Ctx) AcceptsEncodings(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(fasthttp.HeaderAcceptEncoding)
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

// AcceptsLanguages : https://fiber.wiki/context#acceptslanguages
func (ctx *Ctx) AcceptsLanguages(offers ...string) (offer string) {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(fasthttp.HeaderAcceptLanguage)
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

// Append : https://fiber.wiki/context#append
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

// Attachment : https://fiber.wiki/context#attachment
func (ctx *Ctx) Attachment(name ...string) {
	if len(name) > 0 {
		filename := filepath.Base(name[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set(fasthttp.HeaderContentDisposition, `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set(fasthttp.HeaderContentDisposition, "attachment")
}

// BaseURL : https://fiber.wiki/context#baseurl
func (ctx *Ctx) BaseURL() string {
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// Body : https://fiber.wiki/context#body
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

// BodyParser : https://fiber.wiki/context#bodyparser
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

// ClearCookie : https://fiber.wiki/context#clearcookie
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

// Compress : https://fiber.wiki/context#compress
func (ctx *Ctx) Compress(level ...int) {
	ctx.compress = 1
	if len(level) > 0 {
		ctx.compress = level[0]
	}
}

// Cookie : https://fiber.wiki/context#cookie
func (ctx *Ctx) Cookie(cookie *Cookie) {
	fcookie := &fasthttp.Cookie{}
	fcookie.SetKey(cookie.Name)
	fcookie.SetValue(cookie.Value)
	fcookie.SetPath(cookie.Path)
	fcookie.SetDomain(cookie.Domain)
	fcookie.SetExpire(cookie.Expires)
	fcookie.SetSecure(cookie.Secure)
	fcookie.SetHTTPOnly(cookie.HTTPOnly)
	ctx.Fasthttp.Response.Header.SetCookie(fcookie)
}

// Cookies : https://fiber.wiki/context#cookies
func (ctx *Ctx) Cookies(key ...string) (value string) {
	if len(key) == 0 {
		return ctx.Get(fasthttp.HeaderCookie)
	}
	return getString(ctx.Fasthttp.Request.Header.Cookie(key[0]))
}

// Download : https://fiber.wiki/context#download
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)

	if len(name) > 0 {
		filename = name[0]
	}

	ctx.Set(fasthttp.HeaderContentDisposition, "attachment; filename="+filename)
	ctx.SendFile(file)
}

// Error returns err that is passed via Next(err)
func (ctx *Ctx) Error() error {
	return ctx.error
}

// Format : https://fiber.wiki/context#format
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

// FormFile : https://fiber.wiki/context#formfile
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// FormValue : https://fiber.wiki/context#formvalue
func (ctx *Ctx) FormValue(key string) (value string) {
	return getString(ctx.Fasthttp.FormValue(key))
}

// Fresh : https://fiber.wiki/context#fresh
func (ctx *Ctx) Fresh() bool {
	return false
}

// Get : https://fiber.wiki/context#get
func (ctx *Ctx) Get(key string) (value string) {
	if key == "referrer" {
		key = "referer"
	}
	return getString(ctx.Fasthttp.Request.Header.Peek(key))
}

// Hostname : https://fiber.wiki/context#hostname
func (ctx *Ctx) Hostname() string {
	return getString(ctx.Fasthttp.URI().Host())
}

// IP : https://fiber.wiki/context#Ip
func (ctx *Ctx) IP() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// IPs : https://fiber.wiki/context#ips
func (ctx *Ctx) IPs() []string {
	ips := strings.Split(ctx.Get(fasthttp.HeaderXForwardedFor), ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// Is : https://fiber.wiki/context#is
func (ctx *Ctx) Is(extension string) (match bool) {
	if extension[0] != '.' {
		extension = "." + extension
	}

	exts, _ := mime.ExtensionsByType(ctx.Get(fasthttp.HeaderContentType))
	if len(exts) > 0 {
		for _, item := range exts {
			if item == extension {
				return true
			}
		}
	}
	return
}

// JSON : https://fiber.wiki/context#json
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

// JSONP : https://fiber.wiki/context#jsonp
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

	ctx.Set(fasthttp.HeaderXContentTypeOptions, "nosniff")
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJavaScript)
	ctx.Fasthttp.Response.SetBodyString(str)

	return nil
}

// Links : https://fiber.wiki/context#links
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
		ctx.Set(fasthttp.HeaderLink, h)
	}
}

// Locals : https://fiber.wiki/context#locals
func (ctx *Ctx) Locals(key string, value ...interface{}) (val interface{}) {
	if len(value) == 0 {
		return ctx.Fasthttp.UserValue(key)
	}
	ctx.Fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Location : https://fiber.wiki/context#location
func (ctx *Ctx) Location(path string) {
	ctx.Set(fasthttp.HeaderLocation, path)
}

// Method : https://fiber.wiki/context#method
func (ctx *Ctx) Method() string {
	return getString(ctx.Fasthttp.Request.Header.Method())
}

// MultipartForm : https://fiber.wiki/context#multipartform
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// Next : https://fiber.wiki/context#next
func (ctx *Ctx) Next(err ...error) {
	ctx.route = nil
	ctx.next = true
	ctx.params = nil
	ctx.values = nil
	if len(err) > 0 {
		ctx.error = err[0]
	}
}

// OriginalURL : https://fiber.wiki/context#originalurl
func (ctx *Ctx) OriginalURL() string {
	return getString(ctx.Fasthttp.Request.Header.RequestURI())
}

// Params : https://fiber.wiki/context#params
func (ctx *Ctx) Params(key string) (value string) {
	if ctx.params == nil {
		return
	}
	for i := 0; i < len(*ctx.params); i++ {
		if (*ctx.params)[i] == key {
			return ctx.values[i]
		}
	}
	return
}

// Path : https://fiber.wiki/context#path
func (ctx *Ctx) Path() string {
	return getString(ctx.Fasthttp.URI().Path())
}

// Protocol : https://fiber.wiki/context#protocol
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Query : https://fiber.wiki/context#query
func (ctx *Ctx) Query(key string) (value string) {
	return getString(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Range : https://fiber.wiki/context#range
func (ctx *Ctx) Range() {
	// https://expressjs.com/en/api.html#req.range
	// https://github.com/jshttp/range-parser/blob/master/index.js
	// r := ctx.Fasthttp.Request.Header.Peek(HeaderRange)
	// *magic*
}

// Redirect : https://fiber.wiki/context#redirect
func (ctx *Ctx) Redirect(path string, status ...int) {
	code := 302
	if len(status) > 0 {
		code = status[0]
	}

	ctx.Set(fasthttp.HeaderLocation, path)
	ctx.Fasthttp.Response.SetStatusCode(code)
}

// Render : https://fiber.wiki/context#render
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

// Route : https://fiber.wiki/context#route
func (ctx *Ctx) Route() *Route {
	return ctx.route
}

// SaveFile : https://fiber.wiki/context#secure
func (ctx *Ctx) SaveFile(fileheader *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fileheader, path)
}

// Secure : https://fiber.wiki/context#secure
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Send : https://fiber.wiki/context#send
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

// SendBytes : https://fiber.wiki/context#sendbytes
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(getString(body))
}

// SendFile : https://fiber.wiki/context#sendfile
func (ctx *Ctx) SendFile(file string, gzip ...bool) {
	// Disable gzipping
	if len(gzip) > 0 && !gzip[0] {
		fasthttp.ServeFileUncompressed(ctx.Fasthttp, file)
		return
	}
	fasthttp.ServeFile(ctx.Fasthttp, file)
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// SendStatus : https://fiber.wiki/context#sendstatus
func (ctx *Ctx) SendStatus(status int) {
	ctx.Fasthttp.Response.SetStatusCode(status)
	// Only set status body when there is no response body
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		ctx.Fasthttp.Response.SetBodyString(getStatus(status))
	}
}

// SendString : https://fiber.wiki/context#sendstring
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// Set : https://fiber.wiki/context#set
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.SetCanonical(getBytes(key), getBytes(val))
}

// Subdomains : https://fiber.wiki/context#subdomains
func (ctx *Ctx) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(ctx.Hostname(), ".")
	subdomains = subdomains[:len(subdomains)-o]
	return subdomains
}

// Stale : https://fiber.wiki/context#stale
func (ctx *Ctx) Stale() bool {
	return !ctx.Fresh()
}

// Status : https://fiber.wiki/context#status
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
}

// Type : https://fiber.wiki/context#type
func (ctx *Ctx) Type(ext string) *Ctx {
	ctx.Fasthttp.Response.Header.SetContentType(getType(ext))
	return ctx
}

// Vary : https://fiber.wiki/context#vary
func (ctx *Ctx) Vary(fields ...string) {
	if len(fields) == 0 {
		return
	}

	h := getString(ctx.Fasthttp.Response.Header.Peek(fasthttp.HeaderVary))
	for i := range fields {
		if h == "" {
			h += fields[i]
		} else {
			h += ", " + fields[i]
		}
	}

	ctx.Set(fasthttp.HeaderVary, h)
}

// Write : https://fiber.wiki/context#write
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

// XHR : https://fiber.wiki/context#xhr
func (ctx *Ctx) XHR() bool {
	return ctx.Get(fasthttp.HeaderXRequestedWith) == "XMLHttpRequest"
}
