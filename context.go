package fiber

import (
	"bytes"
	"encoding/base64"
	"mime"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/valyala/fasthttp"
)

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
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

// Form :
func (ctx *Ctx) MultipartForm() *multipart.Form {
	form, err := ctx.Fasthttp.MultipartForm()
	if err != nil {
		return nil
	}
	return form
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

// SaveFile :
func (ctx *Ctx) SaveFile(fh *multipart.FileHeader, path string) {
	fasthttp.SaveMultipartFile(fh, path)
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

// Cookies :
func (ctx *Ctx) Cookies(args ...interface{}) string {
	if len(args) == 0 {
		return ctx.Get("Cookie")
	}
	if len(args) == 1 {
		str, strOk := args[0].(string)
		if strOk {
			return ctx.Get(str)
		}
		fnc, fncOk := args[0].(func(string, string))
		if fncOk {
			ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
				fnc(b2s(k), b2s(v))
			})
			return ""
		}
		panic("Invalid parameters")
	}
	if len(args) > 1 {
		cook := &fasthttp.Cookie{}
		cook.SetKey(args[0].(string))
		cook.SetValue(args[1].(string))
		if len(args) > 2 {
			// Do something with cookie options (args[2])
			// Dont forget to finish this
		}
		ctx.Fasthttp.Response.Header.SetCookie(cook)
	}
	return ""
}

// ClearCookies :
func (ctx *Ctx) ClearCookies(args ...interface{}) {
	if len(args) == 0 {
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			ctx.Fasthttp.Response.Header.DelClientCookie(string(k))
		})
	}
	if len(args) == 1 {
		ctx.Fasthttp.Response.Header.DelClientCookie(args[0].(string))
	}
}

// Send :
func (ctx *Ctx) Send(args ...interface{}) {
	if len(args) == 1 {
		str, ok := args[0].(string)
		if ok {
			ctx.Fasthttp.Response.SetBodyString(str)
			return
		}
		ctx.Fasthttp.Response.SetBodyString(b2s(args[0].([]byte)))
		return
	}
	panic("To many arguments!")
}

// Write :
func (ctx *Ctx) Write(args ...interface{}) {
	if len(args) == 1 {
		str, ok := args[0].(string)
		if ok {
			ctx.Fasthttp.Response.AppendBodyString(str)
			return
		}
		ctx.Fasthttp.Response.AppendBodyString(b2s(args[0].([]byte)))
		return
	}
	panic("To many arguments!")
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
	ctx.Set("Content-Type", "application/json")
	b := bytes.NewBuffer(nil)
	enc := ffjson.NewEncoder(b)
	err := enc.Encode(v)
	ctx.Send(b.Bytes())
	return err
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

// OriginalURL :
func (ctx *Ctx) OriginalURL() string {
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

// IP :
func (ctx *Ctx) IP() string {
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
func (ctx *Ctx) Attachment(args ...interface{}) {
	if len(args) == 1 {
		filename := filepath.Base(args[0].(string))
		ctx.Type(filepath.Ext(filename))
		ctx.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set("Content-Disposition", "attachment")
}

// Download :
func (ctx *Ctx) Download(args ...interface{}) {
	var file string
	var filename string
	if len(args) == 1 {
		file = args[0].(string)
		filename = filepath.Base(file)
	}
	if len(args) == 2 {
		file = args[0].(string)
		filename = args[1].(string)
	}
	ctx.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SendFile(file)
}

// SendFile :
func (ctx *Ctx) SendFile(file string) {
	fasthttp.ServeFile(ctx.Fasthttp, file)
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// Location :
func (ctx *Ctx) Location(path string) {
	ctx.Set("Location", path)
}
