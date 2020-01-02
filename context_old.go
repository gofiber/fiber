package express

import (
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"sync"

	"github.com/valyala/fasthttp"
)

// Context :
type Context struct {
	next     bool
	params   *[]string
	values   *[]string]
	Fasthttp *fasthttp.RequestCtx
}

// CookieOptions :
type CookieOptions struct {
	domain   string
	expires  int
	httpOnly bool
	maxAge   int
	path     string
	secure   bool
	signed   bool
	sameSite string // "strict" or "laks"
}

var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Context)
	},
}
var valuesPool = sync.Pool{
	New: func() interface{} {
		return new([]string)
	},
}

func acquireCtx(fctx *fasthttp.RequestCtx) *Context {
	ctx := ctxPool.Get().(*Context)
	ctx.values = valuesPool.Get().(*[]string)
	ctx.Fasthttp = fctx
	return ctx
}

func releaseCtx(ctx *Context) {
	ctx.next = false
	if ctx.values != nil {
		valuesPool.Put(ctx.values)
	}
	ctx.values = nil
	ctx.params = nil
	ctx.Fasthttp = nil
	ctxPool.Put(ctx)
}

// Next :
func (ctx *Context) Next() {
	ctx.next = true
	if ctx.values != nil {
		valuesPool.Put(ctx.values)
	}
	ctx.values = nil
	ctx.params = nil
}

// // ParseRange https://expressjs.com/en/4x/api.html#req.ip
// func (ctx *Context) ParseRange() string {
// 	ctx.Fasthttp.Request.Header.Cookie("name")
// 	//fasthttp.ParseByteRange(string("Range: bytes=0-1023"), 5000)
// 	return ctx.Fasthttp.RemoteIP().String()
// }

// Cookies :
func (ctx *Context) Cookies(f func(key string, val string)) {
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		f(string(k), string(v))
	})
}

// ClearCookie https://expressjs.com/en/4x/api.html#req.ip
func (ctx *Context) ClearCookie(args ...interface{}) {
	if len(args) == 0 {
		// remove all cookies
		ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
			ctx.Fasthttp.Response.Header.DelClientCookie(string(k))
		})
	} else if len(args) == 1 {
		// remove specific cookie
		key, keyOk := args[0].(string)
		if !keyOk {
			panic("Invalid cookie key")
		}
		ctx.Fasthttp.Response.Header.DelClientCookie(key)
	}
}

// Cookie :
func (ctx *Context) Cookie(args ...interface{}) string {
	if len(args) == 1 {
		key, ok := args[0].(string)
		if !ok {
			panic("Invalid string")
		}
		return b2s(ctx.Fasthttp.Request.Header.Cookie(key))
	} else if len(args) > 1 {
		key, keyOk := args[0].(string)
		val, valOk := args[1].(string)
		if !keyOk || !valOk {
			panic("Invalid key or value")
		}
		cook := &fasthttp.Cookie{}
		cook.SetKey(key)
		cook.SetValue(val)
		if len(args) > 2 {
			fmt.Println(args[2])
			opt, optOk := args[2].(*CookieOptions)
			if !optOk {
				panic("Invalid cookie options")
			}
			// domain   string
			// expires  int
			// httpOnly bool
			// maxAge   int
			// path     string
			// secure   bool
			// signed   bool
			// sameSite string
			fmt.Println(opt)
			// if *opt.domain != "nil" {
			// 	cook.SetDomain(*opt.domain)
			// }
			// cook.SetExpire(time.Tuesday)
			// cook.SetHTTPOnly(true)
			// cook.SetMaxAge(10)
			// cook.SetPath("/")
			// cook.SetSecure(true)
			// cook.SetSameSite(true)
			//
			// if opt.domain != "" {
			// 	cook.SetDomain(opt.domain)
			// }
			// if opt.expires != 0 {
			//
			// 	cook.SetDomain(opt.domain)
			// }
			// if opt.httpOnly != "" {
			// 	cook.SetDomain(opt.domain)
			// }
			// if opt.domain != "" {
			// 	cook.SetDomain(opt.domain)
			// }
			// if opt.domain != "" {
			// 	cook.SetDomain(opt.domain)
			// }
			// if opt.domain != "" {
			// 	cook.SetDomain(opt.domain)
			// }
			// optional cookie parameters
		}
		ctx.Fasthttp.Response.Header.SetCookie(cook)
	}
	return ""
}

// ParseRange https://expressjs.com/en/4x/api.html#req.ip
func (ctx *Context) ParseRange() string {
	//fasthttp.ParseByteRange(string("Range: bytes=0-1023"), 5000)
	return ctx.Fasthttp.RemoteIP().String()
}

// Ip https://expressjs.com/en/4x/api.html#req.ip
func (ctx *Context) Ip() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// Url https://expressjs.com/en/4x/api.html#req.originalUrl
func (ctx *Context) Url() string {
	return b2s(ctx.Fasthttp.RequestURI())
}

// Query https://expressjs.com/en/4x/api.html#req.query
func (ctx *Context) Query(key string) string {
	return b2s(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Params https://expressjs.com/en/4x/api.html#req.params
func (ctx *Context) Params(key string) string {
	if ctx.params == nil {
		return ""
	}
	length := len(*ctx.params)
	fmt.Println(length)
	for i := 0; i < length; i++ {
		if (*ctx.params)[i] == key {
			//return &ctx.values[i]
		}
	}
	return ""
}

// Method https://expressjs.com/en/4x/api.html#req.method
func (ctx *Context) Method() string {
	return b2s(ctx.Fasthttp.Method())
}

// Path https://expressjs.com/en/4x/api.html#req.path
func (ctx *Context) Path() string {
	return b2s(ctx.Fasthttp.Path())
}

// Secure https://expressjs.com/en/4x/api.html#req.secure
func (ctx *Context) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// Xhr https://expressjs.com/en/4x/api.html#req.xhr
func (ctx *Context) Xhr() bool {
	return ctx.Get("X-Requested-With") == "XMLHttpRequest"
}

// Protocol https://expressjs.com/en/4x/api.html#req.protocol
func (ctx *Context) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Is https://expressjs.com/en/4x/api.html#req.is
func (ctx *Context) Is(ext string) bool {
	if ext[0] != '.' {
		ext = "." + ext
	}
	extensions, _ := mime.ExtensionsByType(ctx.Get("Content-Type"))
	if len(extensions) > 0 {
		for _, item := range extensions {
			if item == ext {
				return true
			}
		}
	}
	return false
}

// Type https://expressjs.com/en/4x/api.html#res.type
func (ctx *Context) Type(ext string) {
	if ext[0] != '.' {
		ext = "." + ext
	}
	m := mime.TypeByExtension(ext)
	ctx.Fasthttp.Response.Header.Set("Content-Type", m)
}

// Attachment https://expressjs.com/en/4x/api.html#res.attachment
func (ctx *Context) Attachment(args ...interface{}) {
	if len(args) == 1 {
		filename, ok := args[0].(string)
		if !ok {
			panic("Invalid string")
		}
		ctx.Type(filepath.Ext(filename))
		ctx.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	} else {
		ctx.Set("Content-Disposition", "attachment")
	}
}

// Set https://expressjs.com/en/4x/api.html#res.set
func (ctx *Context) Set(key string, value string) {
	ctx.Fasthttp.Response.Header.Set(key, value)
}

// Get https://expressjs.com/en/4x/api.html#res.get
func (ctx *Context) Get(key string) string {
	return b2s(ctx.Fasthttp.Response.Header.Peek(key))
}

// Redirect https://expressjs.com/en/4x/api.html#res.redirect
func (ctx *Context) Redirect(args ...interface{}) *Context {
	if len(args) == 1 {
		str, ok := args[0].(string)
		if ok {
			ctx.Fasthttp.Redirect(str, 302)
		} else {
			panic("Invalid string url")
		}
	} else if len(args) == 2 {
		str, sOk := args[1].(string)
		code, cOk := args[0].(int)
		if sOk && cOk {
			ctx.Fasthttp.Redirect(str, code)
		} else {
			panic("Invalid statuscode or string")
		}
	} else {
		panic("You cannot have more than 1 argument")
	}
	return ctx
}

// Status https://expressjs.com/en/4x/api.html#res.status
func (ctx *Context) Status(code int) *Context {
	ctx.Fasthttp.SetStatusCode(code)
	return ctx
}

// Send https://expressjs.com/en/4x/api.html#res.send
func (ctx *Context) Send(args ...interface{}) {
	if len(args) > 2 {
		panic("To many arguments")
	}
	if len(args) == 1 {
		str, ok := args[0].(string)
		if ok {
			ctx.Fasthttp.SetBodyString(str)
			return
		}
		byt, ok := args[0].([]byte)
		if ok {
			ctx.Fasthttp.SetBody(byt)
			return
		}
		panic("Invalid string or []byte")
	} else if len(args) == 2 {
		reader, rOk := args[0].(io.Reader)
		bodysize, bOk := args[0].(int)
		if rOk && bOk {
			ctx.Fasthttp.SetBodyStream(reader, bodysize)
		} else {
			panic("Invalid io.Reader or bodysize(int)")
		}
	} else {
		panic("You cannot have more than 2 arguments")
	}
}

// SendByte https://expressjs.com/en/4x/api.html#res.send
func (ctx *Context) SendByte(b []byte) {
	ctx.Fasthttp.SetBody(b)
}

// SendString https://expressjs.com/en/4x/api.html#res.send
func (ctx *Context) SendString(s string) {
	ctx.Fasthttp.SetBodyString(s)
}

// SendFile https://expressjs.com/en/4x/api.html#res.sendFile
func (ctx *Context) SendFile(path string) {
	ctx.Type(filepath.Ext(path))
	// Shit doesnt work correctly,
	ctx.Fasthttp.SendFile(path)
}

// Write https://nodejs.org/docs/v0.4.7/api/all.html#response.write
func (ctx *Context) Write(args ...interface{}) {
	if len(args) > 2 {
		panic("To many arguments")
	}
	if len(args) == 1 {
		str, ok := args[0].(string)
		if ok {
			ctx.Fasthttp.WriteString(str)
			return
		}
		byt, ok := args[0].([]byte)
		if ok {
			ctx.Fasthttp.Write(byt)
			return
		}
		panic("Invalid string or []byte")
	} else {
		panic("You cannot have more than 1 argument")
	}
}

// WriteBytes https://nodejs.org/docs/v0.4.7/api/all.html#response.write
func (ctx *Context) WriteBytes(b []byte) {
	ctx.Fasthttp.Write(b)
}

// WriteString https://nodejs.org/docs/v0.4.7/api/all.html#response.write
func (ctx *Context) WriteString(s string) {
	ctx.Fasthttp.WriteString(s)
}
