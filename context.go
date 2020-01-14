package fiber

import (
	"encoding/base64"
	"mime"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

// Ctx struct
type Ctx struct {
	noCopy   noCopy
	next     bool
	params   *[]string
	values   []string
	locals   map[string]string
	Fasthttp *fasthttp.RequestCtx
}

// Ctx pool
var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Ctx)
	},
}

// Get new Ctx from pool
func acquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := ctxPool.Get().(*Ctx)
	ctx.Fasthttp = fctx
	return ctx
}

// Return Context to pool
func releaseCtx(ctx *Ctx) {
	ctx.next = false
	ctx.params = nil
	ctx.values = nil
	ctx.locals = nil
	ctx.Fasthttp = nil
	ctxPool.Put(ctx)
}

// Accepts :
func (ctx *Ctx) Accepts(typ string) bool {
	accept := ctx.Get("Accept-Charset")
	if strings.Contains(accept, typ) {
		return true
	}
	return false
}

// AcceptsCharsets :
func (ctx *Ctx) AcceptsCharsets(charset string) bool {
	accept := ctx.Get("Accept-Charset")
	if strings.Contains(accept, charset) {
		return true
	}
	return false
}

// AcceptsEncodings :
func (ctx *Ctx) AcceptsEncodings(encoding string) bool {
	accept := ctx.Get("Accept-Encoding")
	if strings.Contains(accept, encoding) {
		return true
	}
	return false
}

// AcceptsLanguages :
func (ctx *Ctx) AcceptsLanguages(lang string) bool {
	accept := ctx.Get("Accept-Language")
	if strings.Contains(accept, lang) {
		return true
	}
	return false
}

// Append :
func (ctx *Ctx) Append(field string, values ...string) {
	newVal := ctx.Get(field)
	if len(values) > 0 {
		for i := range values {
			newVal = newVal + ", " + values[i]
		}
	}
	ctx.Set(field, newVal)
}

// Attachment :
func (ctx *Ctx) Attachment(name ...string) {
	if len(name) > 0 {
		filename := filepath.Base(name[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set("Content-Disposition", "attachment")
}

// BaseUrl :
func (ctx *Ctx) BaseUrl() string {
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// BasicAuth :
func (ctx *Ctx) BasicAuth() (user, pass string, ok bool) {
	auth := ctx.Get("Authorization")
	if auth == "" {
		return
	}
	const prefix = "Basic "
	// Case insensitive prefix match.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := b2s(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

// Body :
func (ctx *Ctx) Body(args ...interface{}) string {
	if len(args) == 0 {
		return b2s(ctx.Fasthttp.Request.Body())
	}
	if len(args) == 1 {
		switch arg := args[0].(type) {
		case string:
			return b2s(ctx.Fasthttp.Request.PostArgs().Peek(arg))
		case func(string, string):
			ctx.Fasthttp.Request.PostArgs().VisitAll(func(k []byte, v []byte) {
				arg(b2s(k), b2s(v))
			})
		default:
			return b2s(ctx.Fasthttp.Request.Body())
		}
	}
	return ""
}

// ClearCookie :
func (ctx *Ctx) ClearCookie(name ...string) {
	if len(name) == 0 {
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			ctx.Fasthttp.Response.Header.DelClientCookie(b2s(k))
		})
	} else if len(name) > 0 {
		for i := range name {
			ctx.Fasthttp.Response.Header.DelClientCookie(name[i])
		}
	}
}

// Cookie :
func (ctx *Ctx) Cookie(name, value string, options ...interface{}) {
	cook := &fasthttp.Cookie{}
	if len(options) > 0 {
		// options
	}
	cook.SetKey(name)
	cook.SetValue(value)
	ctx.Fasthttp.Response.Header.SetCookie(cook)
}

// Cookies :
func (ctx *Ctx) Cookies(args ...interface{}) string {
	if len(args) == 0 {
		return ctx.Get("Cookie")
	}
	switch arg := args[0].(type) {
	case string:
		return b2s(ctx.Fasthttp.Request.Header.Cookie(arg))
	case func(string, string):
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			arg(b2s(k), b2s(v))
		})
	default:
		panic("Argument must be a string or func(string, string)")
	}
	return ""
}

// Download :
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)
	if len(name) > 0 {
		filename = name[0]
	}
	ctx.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SendFile(file)
}

// End TODO
func (ctx *Ctx) End() {

}

// Format TODO
func (ctx *Ctx) Format() {

}

// FormFile :
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// FormValue :
func (ctx *Ctx) FormValue(key string) string {
	return b2s(ctx.Fasthttp.FormValue(key))
}

// Fresh TODO https://expressjs.com/en/4x/api.html#req.fresh
func (ctx *Ctx) Fresh() bool {
	return true
}

// Get :
func (ctx *Ctx) Get(key string) string {
	// https://en.wikipedia.org/wiki/HTTP_referer
	if key == "referrer" {
		key = "referer"
	}
	return b2s(ctx.Fasthttp.Request.Header.Peek(key))
}

// HeadersSent TODO
func (ctx *Ctx) HeadersSent() {

}

// Hostname :
func (ctx *Ctx) Hostname() string {
	return b2s(ctx.Fasthttp.URI().Host())
}

// Ip :
func (ctx *Ctx) Ip() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// Ips https://expressjs.com/en/4x/api.html#req.ips
func (ctx *Ctx) Ips() []string {
	ips := strings.Split(ctx.Get("X-Forwarded-For"), ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// Is :
func (ctx *Ctx) Is(ext string) bool {
	if ext[0] != '.' {
		ext = "." + ext
	}
	exts, _ := mime.ExtensionsByType(ctx.Get("Content-Type"))
	if len(exts) > 0 {
		for _, item := range exts {
			if item == ext {
				return true
			}
		}
	}
	return false
}

// Json :
func (ctx *Ctx) Json(v interface{}) error {
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		return err
	}
	ctx.Set("Content-Type", "application/json")
	ctx.Fasthttp.Response.SetBodyString(b2s(raw))
	return nil
}

// Jsonp :
func (ctx *Ctx) Jsonp(v interface{}, cb ...string) error {
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		return err
	}

	var builder strings.Builder
	if len(cb) > 0 {
		builder.Write(s2b(cb[0]))
	} else {
		builder.Write([]byte("callback"))
	}
	builder.Write([]byte("("))
	builder.Write(raw)
	builder.Write([]byte(");"))

	// Create buffer with length of json + cbname + ( );
	// buf := make([]byte, len(raw)+len(cbName)+3)
	//
	// count := 0
	// count += copy(buf[count:], cbName)
	// count += copy(buf[count:], "(")
	// count += copy(buf[count:], raw)
	// count += copy(buf[count:], ");")

	ctx.Set("X-Content-Type-Options", "nosniff")
	ctx.Set("Content-Type", "application/javascript")
	ctx.Fasthttp.Response.SetBodyString(builder.String())
	return nil
}

// Links :
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
		ctx.Set("Link", h)
	}
}

// Locals :
func (ctx *Ctx) Locals(key string, val ...string) string {
	if ctx.locals == nil {
		ctx.locals = make(map[string]string)
	}
	if len(val) == 0 {
		return ctx.locals[key]
	} else {
		ctx.locals[key] = val[0]
	}
	return ""
}

// Location :
func (ctx *Ctx) Location(path string) {
	ctx.Set("Location", path)
}

// Method :
func (ctx *Ctx) Method() string {
	return b2s(ctx.Fasthttp.Request.Header.Method())
}

// MultipartForm :
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// Next :
func (ctx *Ctx) Next() {
	ctx.next = true
	ctx.params = nil
	ctx.values = nil
}

// OriginalUrl :
func (ctx *Ctx) OriginalUrl() string {
	return b2s(ctx.Fasthttp.Request.Header.RequestURI())
}

// Params :
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

// Path :
func (ctx *Ctx) Path() string {
	return b2s(ctx.Fasthttp.URI().Path())
}

// Protocol :
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Query :
func (ctx *Ctx) Query(key string) string {
	return b2s(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Range TODO
func (ctx *Ctx) Range() {

}

// Redirect :
func (ctx *Ctx) Redirect(path string, status ...int) {
	ctx.Set("Location", path)
	if len(status) > 0 {
		ctx.Status(status[0])
	} else {
		ctx.Status(302)
	}
}

// Render TODO https://expressjs.com/en/4x/api.html#res.render
func (ctx *Ctx) Render() {

}

// Route : Only use in debugging
func (ctx *Ctx) Route(r *Fiber) (s struct {
	Method   string
	Path     string
	Wildcard bool
	Regex    *regexp.Regexp
	Params   []string
	Values   []string
	Handler  func(*Ctx)
}) {
	path := ctx.Path()
	method := ctx.Method()
	for _, route := range r.routes {
		if route.method != "*" && route.method != method {
			continue
		}
		if route.any || (route.path == path && route.params == nil) {
			s.Method = method
			s.Path = path
			s.Wildcard = route.any
			s.Regex = route.regex
			s.Params = route.params
			s.Values = ctx.values
			s.Handler = route.handler
			return
		}
		if route.regex == nil {
			continue
		}
		if !route.regex.MatchString(path) {
			continue
		}
		s.Method = method
		s.Path = path
		s.Wildcard = route.any
		s.Regex = route.regex
		s.Params = route.params
		s.Handler = route.handler
		return
	}
	return
}

// Secure :
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Send :
func (ctx *Ctx) Send(args ...interface{}) {

	// https://github.com/valyala/fasthttp/blob/master/http.go#L490
	if len(args) != 1 {
		panic("To many arguments!")
	}
	switch body := args[0].(type) {
	case string:
		//ctx.Fasthttp.Response.SetBodyRaw(s2b(body))
		ctx.Fasthttp.Response.SetBodyString(body)
	case []byte:
		//ctx.Fasthttp.Response.SetBodyRaw(body)
		ctx.Fasthttp.Response.SetBodyString(b2s(body))
	default:
		panic("body must be a string or []byte")
	}
}

// SendBytes : Same as Send() but without type assertion
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(b2s(body))
}

// SendFile :
func (ctx *Ctx) SendFile(file string) {
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	fasthttp.ServeFile(ctx.Fasthttp, file)
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// SendStatus :
func (ctx *Ctx) SendStatus(status int) {
	ctx.Status(status)
	// Only set status body when there is no response body
	if len(ctx.Fasthttp.Response.Body()) == 0 {
		msg := statusMessages[status]
		if msg != "" {
			ctx.Fasthttp.Response.SetBodyString(msg)
		}
	}
}

// SendString : Same as Send() but without type assertion
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// Set :
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.SetCanonical(s2b(key), s2b(val))
}

// SignedCookies TODO
func (ctx *Ctx) SignedCookies() {

}

// Stale TODO https://expressjs.com/en/4x/api.html#req.fresh
func (ctx *Ctx) Stale() bool {
	return true
}

// Status :
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
}

// Subdomains :
func (ctx *Ctx) Subdomains() (subs []string) {
	subs = strings.Split(ctx.Hostname(), ".")
	subs = subs[:len(subs)-2]
	return subs
}

// Type :
func (ctx *Ctx) Type(ext string) *Ctx {
	if ext[0] != '.' {
		ext = "." + ext
	}
	m := mime.TypeByExtension(ext)
	ctx.Set("Content-Type", m)
	return ctx
}

// Vary :
func (ctx *Ctx) Vary(field ...string) {
	vary := ctx.Get("Vary")
	for _, f := range field {
		if !strings.Contains(vary, f) {
			vary += ", " + f
		}
	}
	if len(field) > 0 {
		ctx.Set("Vary", vary)
	}
}

// Write :
func (ctx *Ctx) Write(args ...interface{}) {
	if len(args) == 0 {
		panic("Missing body")
	}
	switch body := args[0].(type) {
	case string:
		ctx.Fasthttp.Response.SetBodyString(body)
	case []byte:
		ctx.Fasthttp.Response.AppendBodyString(b2s(body))
	default:
		panic("body must be a string or []byte")
	}
}

// Xhr :
func (ctx *Ctx) Xhr() bool {
	return ctx.Get("X-Requested-With") == "XMLHttpRequest"
}
