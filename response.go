// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"encoding/xml"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

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

// Download : https://fiber.wiki/context#download
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)

	if len(name) > 0 {
		filename = name[0]
	}

	ctx.Set(fasthttp.HeaderContentDisposition, "attachment; filename="+filename)
	ctx.SendFile(file)
}

// End : https://fiber.wiki/context#end
func (ctx *Ctx) End() {

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

// HeadersSent : https://fiber.wiki/context#headerssent
func (ctx *Ctx) HeadersSent() {

}

// Json is deprecated, this will be removed in v2: Use c.JSON() instead
func (ctx *Ctx) Json(v interface{}) error {
	fmt.Println("Fiber deprecated c.Json(), this will be removed in v2: Use c.JSON() instead")
	return ctx.JSON(v)
}

// JSON : https://fiber.wiki/context#json
func (ctx *Ctx) JSON(v interface{}) error {
	ctx.Fasthttp.Response.Header.SetContentType(contentTypeJSON)
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		ctx.Fasthttp.Response.SetBodyString("")
		return err
	}
	ctx.Fasthttp.Response.SetBodyString(getString(raw))

	return nil
}

// JsonBytes is deprecated, this will be removed in v2: Use c.JSONBytes() instead
func (ctx *Ctx) JsonBytes(raw []byte) {
	fmt.Println("Fiber deprecated c.JsonBytes(), this will be removed in v2: Use c.JSONBytes() instead")
	ctx.JSONBytes(raw)
}

// JSONBytes : https://fiber.wiki/context#jsonbytes
func (ctx *Ctx) JSONBytes(raw []byte) {
	ctx.Fasthttp.Response.Header.SetContentType(contentTypeJSON)
	ctx.Fasthttp.Response.SetBodyString(getString(raw))
}

// Jsonp is deprecated, this will be removed in v2: Use c.JSONP() instead
func (ctx *Ctx) Jsonp(v interface{}, cb ...string) error {
	fmt.Println("Fiber deprecated c.Jsonp(), this will be removed in v2: Use c.JSONP() instead")
	return ctx.JSONP(v, cb...)
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
	ctx.Fasthttp.Response.Header.SetContentType(contentTypeJs)
	ctx.Fasthttp.Response.SetBodyString(str)

	return nil
}

// JsonString is deprecated, this will be removed in v2: Use c.JSONString() instead
func (ctx *Ctx) JsonString(raw string) {
	fmt.Println("Fiber deprecated c.JsonString(), this will be removed in v2: Use c.JSONString() instead")
	ctx.JSONString(raw)
}

// JSONString : https://fiber.wiki/context#json
func (ctx *Ctx) JSONString(raw string) {
	ctx.Fasthttp.Response.Header.SetContentType(contentTypeJSON)
	ctx.Fasthttp.Response.SetBodyString(raw)
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

// Location : https://fiber.wiki/context#location
func (ctx *Ctx) Location(path string) {
	ctx.Set(fasthttp.HeaderLocation, path)
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
func (ctx *Ctx) Render() {

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
		msg := getStatus(status)
		if msg != "" {
			ctx.Fasthttp.Response.SetBodyString(msg)
		}
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

// Xml is deprecated, this will be removed in v2: Use c.XML() instead
func (ctx *Ctx) Xml(v interface{}) error {
	fmt.Println("Fiber deprecated c.Xml(), this will be removed in v2: Use c.XML() instead")
	return ctx.XML(v)
}

// XML : https://fiber.wiki/context#xml
func (ctx *Ctx) XML(v interface{}) error {
	raw, err := xml.Marshal(v)
	if err != nil {
		return err
	}

	ctx.Fasthttp.Response.Header.SetContentType(contentTypeXML)
	ctx.Fasthttp.Response.SetBody(raw)

	return nil
}
