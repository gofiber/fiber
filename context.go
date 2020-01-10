package fiber

import (
	"encoding/base64"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Next :
func (ctx *Ctx) Next() {
	ctx.next = true
	ctx.params = nil
	ctx.values = nil
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

// Query :
func (ctx *Ctx) Query(key string) string {
	return b2s(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Method :
func (ctx *Ctx) Method() string {
	return b2s(ctx.Fasthttp.Request.Header.Method())
}

// Path :
func (ctx *Ctx) Path() string {
	return b2s(ctx.Fasthttp.URI().Path())
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

// MultipartForm :
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// FormValue :
func (ctx *Ctx) FormValue(key string) string {
	return b2s(ctx.Fasthttp.FormValue(key))
}

// FormFile :
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// SaveFile :
func (ctx *Ctx) SaveFile(fh *multipart.FileHeader, path string) {
	fasthttp.SaveMultipartFile(fh, path)
}

// // FormValue :
// func (ctx *Ctx) FormValues(key string) (values []string) {
// 	form, err := ctx.Fasthttp.MultipartForm()
// 	if err != nil {
// 		return values
// 	}
// 	return form.Value[key]
// }
//
// // FormFile :
// func (ctx *Ctx) FormFiles(key string) (files []*multipart.FileHeader) {
// 	form, err := ctx.Fasthttp.MultipartForm()
// 	if err != nil {
// 		return files
// 	}
// 	files = form.File[key]
// 	return files
// }

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

// ClearCookies :
func (ctx *Ctx) ClearCookies(args ...string) {
	if len(args) == 0 {
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			ctx.Fasthttp.Response.Header.DelClientCookie(b2s(k))
		})
	}
	if len(args) == 1 {
		ctx.Fasthttp.Response.Header.DelClientCookie(args[0])
	}
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

// SendString internal use
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// SendByte internal use
func (ctx *Ctx) SendByte(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(b2s(body))
}

// Write :
func (ctx *Ctx) Write(args ...interface{}) {
	if len(args) != 1 {
		panic("To many arguments!")
	}
	switch body := args[0].(type) {
	case string:
		ctx.Fasthttp.Response.AppendBodyString(body)
	case []byte:
		ctx.Fasthttp.Response.AppendBodyString(b2s(body))
	default:
		panic("body must be a string or []byte")
	}
}

// Set :
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.SetCanonical(s2b(key), s2b(val))
}

// Get :
func (ctx *Ctx) Get(key string) string {
	// https://en.wikipedia.org/wiki/HTTP_referer
	if key == "referrer" {
		key = "referer"
	}
	return b2s(ctx.Fasthttp.Request.Header.Peek(key))
}

// Json :
func (ctx *Ctx) Json(v interface{}) error {
	raw, err := json.Marshal(&v)
	if err != nil {
		return err
	}
	ctx.Set("Content-Type", "application/json")
	ctx.SendByte(raw)
	return nil
}

// Redirect :
func (ctx *Ctx) Redirect(args ...interface{}) {
	if len(args) == 1 {
		ctx.Set("Location", args[0].(string))
		ctx.Status(302)
	}
	if len(args) == 2 {
		ctx.Set("Location", args[1].(string))
		ctx.Status(args[0].(int))
	}
}

// Status :
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
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

// Hostname :
func (ctx *Ctx) Hostname() string {
	return b2s(ctx.Fasthttp.URI().Host())
}

// OriginalUrl :
func (ctx *Ctx) OriginalUrl() string {
	return b2s(ctx.Fasthttp.Request.Header.RequestURI())
}

// Protocol :
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Secure :
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Ip :
func (ctx *Ctx) Ip() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// Xhr :
func (ctx *Ctx) Xhr() bool {
	return ctx.Get("X-Requested-With") == "XMLHttpRequest"
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

// Attachment :
func (ctx *Ctx) Attachment(args ...string) {
	if len(args) == 1 {
		filename := filepath.Base(args[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set("Content-Disposition", "attachment")
}

// Download :
func (ctx *Ctx) Download(args ...string) {
	if len(args) == 0 {
		panic("Missing filename")
	}
	file := args[0]
	filename := filepath.Base(file)
	if len(args) > 1 {
		filename = args[1]
	}
	ctx.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SendFile(file)
}

// SendFile :
func (ctx *Ctx) SendFile(file string) {
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	fasthttp.ServeFile(ctx.Fasthttp, file)
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// Location :
func (ctx *Ctx) Location(path string) {
	ctx.Set("Location", path)
}

// Subdomains :
func (ctx *Ctx) Subdomains() (subs []string) {
	subs = strings.Split(ctx.Hostname(), ".")
	subs = subs[:len(subs)-2]
	return subs
}

// Ips https://expressjs.com/en/4x/api.html#req.ips
func (ctx *Ctx) Ips() []string {
	ips := strings.Split(ctx.Get("X-Forwarded-For"), " ")
	return ips
}

// Jsonp TODO https://expressjs.com/en/4x/api.html#res.jsonp
func (ctx *Ctx) Jsonp(args ...interface{}) error {
	jsonp := "callback("
	if len(args) == 1 {
		raw, err := json.Marshal(&args[0])
		if err != nil {
			return err
		}
		jsonp += b2s(raw) + ");"
	} else if len(args) == 2 {
		jsonp = args[0].(string) + "("
		raw, err := json.Marshal(&args[0])
		if err != nil {
			return err
		}
		jsonp += b2s(raw) + ");"
	} else {
		panic("Missing interface{}")
	}
	ctx.Set("X-Content-Type-Options", "nosniff")
	ctx.Set("Content-Type", "text/javascript")
	ctx.SendString(jsonp)
	return nil
}

// Vary TODO https://expressjs.com/en/4x/api.html#res.vary
func (ctx *Ctx) Vary() {

}

// Links TODO https://expressjs.com/en/4x/api.html#res.links
func (ctx *Ctx) Links() {

}

// Append TODO https://expressjs.com/en/4x/api.html#res.append
func (ctx *Ctx) Append(field, val string) {
	prev := ctx.Get(field)
	value := val
	if prev != "" {
		value = prev + "; " + val
	}
	ctx.Set(field, value)
}

// Accepts TODO https://expressjs.com/en/4x/api.html#req.accepts
func (ctx *Ctx) Accepts() bool {
	return true
}

// Range TODO https://expressjs.com/en/4x/api.html#req.range
func (ctx *Ctx) Range() bool {
	return true
}

// Fresh TODO https://expressjs.com/en/4x/api.html#req.fresh
func (ctx *Ctx) Fresh() bool {
	return true
}

// Stale TODO https://expressjs.com/en/4x/api.html#req.fresh
func (ctx *Ctx) Stale() bool {
	return true
}
