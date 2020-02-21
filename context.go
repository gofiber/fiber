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
	"strings"
	"sync"
	"time"

	// templates
	pug "github.com/Joker/jade"
	handlebars "github.com/aymerick/raymond"
	mustache "github.com/cbroglie/mustache"
	amber "github.com/eknkc/amber"
	// core
	websocket "github.com/fasthttp/websocket"
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
	Fasthttp *fasthttp.RequestCtx
	Socket   *websocket.Conn
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
	ctx.Fasthttp = nil
	ctx.Socket = nil
	poolCtx.Put(ctx)
}

// Conn https://godoc.org/github.com/gorilla/websocket#pkg-index
type Conn struct {
	params *[]string
	values []string
	*websocket.Conn
}

// Params : https://fiber.wiki/application#websocket
func (conn *Conn) Params(key string) string {
	if conn.params == nil {
		return ""
	}
	for i := 0; i < len(*conn.params); i++ {
		if (*conn.params)[i] == key {
			return conn.values[i]
		}
	}
	return ""
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
	conn.Close()
	conn.params = nil
	conn.values = nil
	conn.Conn = nil
	poolConn.Put(conn)
}

// Cookie : struct
type Cookie struct {
	Expire int // time.Unix(1578981376, 0)
	MaxAge int
	Domain string
	Path   string

	HTTPOnly bool
	Secure   bool
	SameSite string
}

// Accepts : https://fiber.wiki/context#accepts
func (ctx *Ctx) Accepts(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(fasthttp.HeaderAccept)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, offer := range offers {
		mimetype := getType(offer)
		// if mimetype != "" {
		// 	mimetype = strings.Split(mimetype, ";")[0]
		// } else {
		// 	mimetype = offer
		// }
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*/*") {
				return offer
			}

			if strings.HasPrefix(spec, mimetype) {
				return offer
			}

			if strings.Contains(spec, "/*") {
				if strings.HasPrefix(spec, strings.Split(mimetype, "/")[0]) {
					return offer
				}
			}
		}
	}
	return ""
}

// AcceptsCharsets : https://fiber.wiki/context#acceptscharsets
func (ctx *Ctx) AcceptsCharsets(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(fasthttp.HeaderAcceptCharset)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, offer := range offers {
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return offer
			}
			if strings.HasPrefix(spec, offer) {
				return offer
			}
		}
	}
	return ""
}

// AcceptsEncodings : https://fiber.wiki/context#acceptsencodings
func (ctx *Ctx) AcceptsEncodings(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}

	h := ctx.Get(fasthttp.HeaderAcceptEncoding)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, offer := range offers {
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return offer
			}
			if strings.HasPrefix(spec, offer) {
				return offer
			}
		}
	}
	return ""
}

// AcceptsLanguages : https://fiber.wiki/context#acceptslanguages
func (ctx *Ctx) AcceptsLanguages(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	h := ctx.Get(fasthttp.HeaderAcceptLanguage)
	if h == "" {
		return offers[0]
	}

	specs := strings.Split(h, ",")
	for _, offer := range offers {
		for _, spec := range specs {
			spec = strings.TrimSpace(spec)
			if strings.HasPrefix(spec, "*") {
				return offer
			}
			if strings.HasPrefix(spec, offer) {
				return offer
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
func (ctx *Ctx) Body(args ...interface{}) string {
	if len(args) == 0 {
		return getString(ctx.Fasthttp.Request.Body())
	}

	if len(args) == 1 {
		switch arg := args[0].(type) {
		case string:
			return getString(ctx.Fasthttp.Request.PostArgs().Peek(arg))
		case []byte:
			return getString(ctx.Fasthttp.Request.PostArgs().PeekBytes(arg))
		case func(string, string):
			ctx.Fasthttp.Request.PostArgs().VisitAll(func(k []byte, v []byte) {
				arg(getString(k), getString(v))
			})
		default:
			return getString(ctx.Fasthttp.Request.Body())
		}
	}
	return ""
}

// BodyParser : https://fiber.wiki/context#bodyparser
func (ctx *Ctx) BodyParser(v interface{}) error {
	ctype := getString(ctx.Fasthttp.Request.Header.ContentType())
	// application/json
	if strings.HasPrefix(ctype, MIMEApplicationJSON) {
		return jsoniter.Unmarshal(ctx.Fasthttp.Request.Body(), v)
	}
	// application/xml text/xml
	if strings.HasPrefix(ctype, MIMEApplicationXML) || strings.HasPrefix(ctype, MIMETextXML) {
		return xml.Unmarshal(ctx.Fasthttp.Request.Body(), v)
	}
	// application/x-www-form-urlencoded
	if strings.HasPrefix(ctype, MIMEApplicationForm) {
		data, err := url.ParseQuery(getString(ctx.Fasthttp.PostBody()))
		if err != nil {
			return err
		}
		return schemaDecoder.Decode(v, data)
	}
	// multipart/form-data
	if strings.HasPrefix(ctype, MIMEMultipartForm) {
		data, err := ctx.Fasthttp.MultipartForm()
		if err != nil {
			return err
		}
		return schemaDecoder.Decode(v, data.Value)

	}
	return fmt.Errorf("cannot parse content-type: %v", ctype)
}

// ClearCookie : https://fiber.wiki/context#clearcookie
func (ctx *Ctx) ClearCookie(name ...string) {
	if len(name) > 0 {
		for i := range name {
			//ctx.Fasthttp.Request.Header.DelAllCookies()
			ctx.Fasthttp.Response.Header.DelClientCookie(name[i])
		}
		return
	}
	//ctx.Fasthttp.Response.Header.DelAllCookies()
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		ctx.Fasthttp.Response.Header.DelClientCookie(getString(k))
	})
}

// Cookie : https://fiber.wiki/context#cookie
func (ctx *Ctx) Cookie(key, value string, options ...interface{}) {
	cook := &fasthttp.Cookie{}

	cook.SetKey(key)
	cook.SetValue(value)

	if len(options) > 0 {
		switch opt := options[0].(type) {
		case *Cookie:
			if opt.Expire > 0 {
				cook.SetExpire(time.Unix(int64(opt.Expire), 0))
			}
			if opt.MaxAge > 0 {
				cook.SetMaxAge(opt.MaxAge)
			}
			if opt.Domain != "" {
				cook.SetDomain(opt.Domain)
			}
			if opt.Path != "" {
				cook.SetPath(opt.Path)
			}
			if opt.HTTPOnly {
				cook.SetHTTPOnly(opt.HTTPOnly)
			}
			if opt.Secure {
				cook.SetSecure(opt.Secure)
			}
			if opt.SameSite != "" {
				sameSite := fasthttp.CookieSameSiteDefaultMode
				if strings.EqualFold(opt.SameSite, "lax") {
					sameSite = fasthttp.CookieSameSiteLaxMode
				} else if strings.EqualFold(opt.SameSite, "strict") {
					sameSite = fasthttp.CookieSameSiteStrictMode
				} else if strings.EqualFold(opt.SameSite, "none") {
					sameSite = fasthttp.CookieSameSiteNoneMode
				}
				// } else {
				// 	sameSite = fasthttp.CookieSameSiteDisabled
				// }
				cook.SetSameSite(sameSite)
			}
		default:
			log.Println("Cookie: Invalid &Cookie{} struct")
		}
	}

	ctx.Fasthttp.Response.Header.SetCookie(cook)
}

// Cookies : https://fiber.wiki/context#cookies
func (ctx *Ctx) Cookies(args ...interface{}) string {
	if len(args) == 0 {
		return ctx.Get(fasthttp.HeaderCookie)
	}

	switch arg := args[0].(type) {
	case string:
		return getString(ctx.Fasthttp.Request.Header.Cookie(arg))
	case []byte:
		return getString(ctx.Fasthttp.Request.Header.CookieBytes(arg))
	case func(string, string):
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			arg(getString(k), getString(v))
		})
	default:
		return ctx.Get(fasthttp.HeaderCookie)
	}

	return ""
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
func (ctx *Ctx) Format(args ...interface{}) {
	var body string

	accept := ctx.Accepts("html", "json")

	for i := range args {
		switch arg := args[i].(type) {
		case string:
			body = arg
		case []byte:
			body = getString(arg)
		default:
			body = fmt.Sprintf("%v", arg)
		}
		switch accept {
		case "html":
			ctx.SendString("<p>" + body + "</p>")
		case "json":
			if err := ctx.JSON(body); err != nil {
				log.Println("Format: error serializing json ", err)
			}
		default:
			ctx.SendString(body)
		}
	}
}

// FormFile : https://fiber.wiki/context#formfile
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// FormValue : https://fiber.wiki/context#formvalue
func (ctx *Ctx) FormValue(key string) string {
	return getString(ctx.Fasthttp.FormValue(key))
}

// Fresh : https://fiber.wiki/context#fresh
func (ctx *Ctx) Fresh() bool {
	return false
}

// Get : https://fiber.wiki/context#get
func (ctx *Ctx) Get(key string) string {
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
func (ctx *Ctx) Is(ext string) bool {
	if ext[0] != '.' {
		ext = "." + ext
	}

	exts, _ := mime.ExtensionsByType(ctx.Get(fasthttp.HeaderContentType))
	if len(exts) > 0 {
		for _, item := range exts {
			if item == ext {
				return true
			}
		}
	}
	return false
}

// JSON : https://fiber.wiki/context#json
func (ctx *Ctx) JSON(v interface{}) error {
	ctx.Fasthttp.Response.Header.SetContentType(MIMEApplicationJSON)
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		ctx.Fasthttp.Response.SetBodyString("")
		return err
	}
	ctx.Fasthttp.Response.SetBodyString(getString(raw))

	return nil
}

// JSONP : https://fiber.wiki/context#jsonp
func (ctx *Ctx) JSONP(v interface{}, cb ...string) error {
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		return err
	}

	str := "callback("
	if len(cb) > 0 {
		str = cb[0] + "("
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
func (ctx *Ctx) Locals(key string, val ...interface{}) interface{} {
	if len(val) == 0 {
		return ctx.Fasthttp.UserValue(key)
	}
	ctx.Fasthttp.SetUserValue(key, val[0])
	return nil
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
func (ctx *Ctx) Params(key string) string {
	if ctx.params == nil {
		return ""
	}
	for i := 0; i < len(*ctx.params); i++ {
		if (*ctx.params)[i] == key {
			return ctx.values[i]
		}
	}
	return ""
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
func (ctx *Ctx) Query(key string) string {
	return getString(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Range : https://fiber.wiki/context#range
func (ctx *Ctx) Range() {
	// https://expressjs.com/en/api.html#req.range
	// https://github.com/jshttp/range-parser/blob/master/index.js
	// r := ctx.Fasthttp.Request.Header.Peek(fasthttp.HeaderRange)
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
func (ctx *Ctx) Render(file string, data interface{}, e ...string) error {
	var err error
	var raw []byte
	var html string
	var engine string

	if len(e) > 0 {
		engine = e[0]
	} else if ctx.app.Settings.ViewEngine != "" {
		engine = ctx.app.Settings.ViewEngine
	} else {
		engine = filepath.Ext(file)[1:]
	}
	if ctx.app.Settings.ViewFolder != "" {
		file = filepath.Join(ctx.app.Settings.ViewFolder, file)
	}
	if ctx.app.Settings.ViewExtension != "" {
		file = file + ctx.app.Settings.ViewExtension
	}
	if raw, err = ioutil.ReadFile(filepath.Clean(file)); err != nil {
		return err
	}

	switch engine {
	case "amber": // https://github.com/eknkc/amber
		var buf bytes.Buffer
		var tmpl *template.Template

		if tmpl, err = amber.Compile(getString(raw), amber.DefaultOptions); err != nil {
			return err
		}
		if err = tmpl.Execute(&buf, data); err != nil {
			return err
		}
		html = buf.String()

	case "handlebars": // https://github.com/aymerick/raymond
		if html, err = handlebars.Render(getString(raw), data); err != nil {
			return err
		}
	case "mustache": // https://github.com/cbroglie/mustache
		if html, err = mustache.Render(getString(raw), data); err != nil {
			return err
		}
	case "pug": // https://github.com/Joker/jade
		var parsed string
		var buf bytes.Buffer
		var tmpl *template.Template
		if parsed, err = pug.Parse("", raw); err != nil {
			return err
		}
		if tmpl, err = template.New("").Parse(parsed); err != nil {
			return err
		}
		if err = tmpl.Execute(&buf, data); err != nil {
			return err
		}
		html = buf.String()

	default: // https://golang.org/pkg/text/template/
		var buf bytes.Buffer
		var tmpl *template.Template

		if tmpl, err = template.New("").Parse(getString(raw)); err != nil {
			return err
		}
		if err = tmpl.Execute(&buf, data); err != nil {
			return err
		}
		html = buf.String()
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
func (ctx *Ctx) SaveFile(fh *multipart.FileHeader, path string) error {
	return fasthttp.SaveMultipartFile(fh, path)
}

// Secure : https://fiber.wiki/context#secure
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Send : https://fiber.wiki/context#send
func (ctx *Ctx) Send(args ...interface{}) {
	if len(args) == 0 {
		return
	}

	switch body := args[0].(type) {
	case string:
		ctx.Fasthttp.Response.SetBodyString(body)
	case []byte:
		ctx.Fasthttp.Response.SetBodyString(getString(body))
	default:
		ctx.Fasthttp.Response.SetBodyString(fmt.Sprintf("%v", body))
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
func (ctx *Ctx) Subdomains(offset ...int) (subs []string) {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subs = strings.Split(ctx.Hostname(), ".")
	subs = subs[:len(subs)-o]
	return subs
}

// SignedCookies : https://fiber.wiki/context#signedcookies
func (ctx *Ctx) SignedCookies() {

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
func (ctx *Ctx) Write(args ...interface{}) {
	for i := range args {
		switch body := args[i].(type) {
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
