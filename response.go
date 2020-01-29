// 🔌 Fiber is an Expressjs inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt
package fiber

import (
	"encoding/xml"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/sjson"
	"github.com/valyala/fasthttp"
)

// Append : https://gofiber.github.io/fiber/#/context?id=append
func (ctx *Ctx) Append(field string, values ...string) {
	value := ctx.Get(field)
	if len(values) > 0 {
		for i := range values {
			value = fmt.Sprintf("%s, %s", value, values[i])
		}
	}
	ctx.Set(field, value)
}

// Attachment : https://gofiber.github.io/fiber/#/context?id=attachment
func (ctx *Ctx) Attachment(name ...string) {
	if len(name) > 0 {
		filename := filepath.Base(name[0])
		ctx.Type(filepath.Ext(filename))
		ctx.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		return
	}
	ctx.Set("Content-Disposition", "attachment")
}

// ClearCookie : https://gofiber.github.io/fiber/#/context?id=clearcookie
func (ctx *Ctx) ClearCookie(name ...string) {
	if len(name) > 0 {
		for i := range name {
			ctx.Fasthttp.Response.Header.DelClientCookie(name[i])
		}
		return
	}
	ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
		ctx.Fasthttp.Response.Header.DelClientCookie(B2S(k))
	})
}

// Cookie : https://gofiber.github.io/fiber/#/context?id=cookie
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
			if opt.HttpOnly {
				cook.SetHTTPOnly(opt.HttpOnly)
			}
			if opt.Secure {
				cook.SetSecure(opt.Secure)
			}
			if opt.SameSite != "" {
				sameSite := fasthttp.CookieSameSiteDisabled
				if strings.EqualFold(opt.SameSite, "lax") {
					sameSite = fasthttp.CookieSameSiteLaxMode
				} else if strings.EqualFold(opt.SameSite, "strict") {
					sameSite = fasthttp.CookieSameSiteStrictMode
				} else if strings.EqualFold(opt.SameSite, "none") {
					sameSite = fasthttp.CookieSameSiteNoneMode
				} else {
					sameSite = fasthttp.CookieSameSiteDefaultMode
				}
				cook.SetSameSite(sameSite)
			}
		default:
			panic("Invalid cookie options")
		}
	}
	ctx.Fasthttp.Response.Header.SetCookie(cook)
}

// Download : https://gofiber.github.io/fiber/#/context?id=download
func (ctx *Ctx) Download(file string, name ...string) {
	filename := filepath.Base(file)
	if len(name) > 0 {
		filename = name[0]
	}
	ctx.Set("Content-Disposition", "attachment; filename="+filename)
	ctx.SendFile(file)
}

// End : https://gofiber.github.io/fiber/#/context?id=end
func (ctx *Ctx) End() {

}

// Format : https://gofiber.github.io/fiber/#/context?id=format
func (ctx *Ctx) Format(args ...interface{}) {
	if len(args) == 0 {
		panic("Missing string or []byte body")
	}
	var body string
	switch b := args[0].(type) {
	case string:
		body = b
	case []byte:
		body = B2S(b)
	default:
		panic("Body must be a string or []byte")
	}
	accept := ctx.Accepts("html", "json")
	switch accept {
	case "html":
		ctx.SendString("<p>" + body + "</p>")
	case "json":
		ctx.Json(body)
	default:
		ctx.SendString(body)
	}
}

// HeadersSent : https://gofiber.github.io/fiber/#/context?id=headerssent
func (ctx *Ctx) HeadersSent() {

}

// Json : https://gofiber.github.io/fiber/#/context?id=json
func (ctx *Ctx) Json(v interface{}) error {

	raw, err := jsoniter.MarshalToString(&v)
	if err != nil {
		return err
	}
	ctx.Fasthttp.Response.Header.SetContentTypeBytes(applicationjson)
	ctx.Fasthttp.Response.SetBodyString(raw)
	return nil
}

// Jsonp : https://gofiber.github.io/fiber/#/context?id=jsonp
func (ctx *Ctx) Jsonp(v interface{}, cb ...string) error {
	raw, err := jsoniter.Marshal(&v)
	if err != nil {
		return err
	}

	var builder strings.Builder
	if len(cb) > 0 {
		builder.Write(S2B(cb[0]))
	} else {
		builder.Write([]byte("callback"))
	}
	builder.Write([]byte("("))
	builder.Write(raw)
	builder.Write([]byte(");"))

	ctx.Set("X-Content-Type-Options", "nosniff")
	ctx.Set("Content-Type", "application/javascript")
	ctx.Fasthttp.Response.SetBodyString(builder.String())
	return nil
}

// Links : https://gofiber.github.io/fiber/#/context?id=links
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

// Location : https://gofiber.github.io/fiber/#/context?id=location
func (ctx *Ctx) Location(path string) {
	ctx.Set("Location", path)
}

// Next : https://gofiber.github.io/fiber/#/context?id=next
func (ctx *Ctx) Next() {
	ctx.next = true
	ctx.params = nil
	ctx.values = nil
}

// Redirect : https://gofiber.github.io/fiber/#/context?id=redirect
func (ctx *Ctx) Redirect(path string, status ...int) {
	ctx.Set("Location", path)
	if len(status) > 0 {
		ctx.Status(status[0])
	} else {
		ctx.Status(302)
	}
}

// Render : https://gofiber.github.io/fiber/#/context?id=render
func (ctx *Ctx) Render() {

}

// Send : https://gofiber.github.io/fiber/#/context?id=send
func (ctx *Ctx) Send(args ...interface{}) {

	// https://github.com/valyala/fasthttp/blob/master/http.go#L490
	if len(args) == 0 {
		panic("Missing string or []byte body")
	}
	switch body := args[0].(type) {
	case string:
		//ctx.Fasthttp.Response.SetBodyRaw(S2B(body))
		ctx.Fasthttp.Response.SetBodyString(body)
	case []byte:
		//ctx.Fasthttp.Response.SetBodyRaw(body)
		ctx.Fasthttp.Response.SetBodyString(B2S(body))
	default:
		panic("body must be a string or []byte")
	}
}

// SendBytes : https://gofiber.github.io/fiber/#/context?id=sendbytes
func (ctx *Ctx) SendBytes(body []byte) {
	ctx.Fasthttp.Response.SetBodyString(B2S(body))
}

// SendFile : https://gofiber.github.io/fiber/#/context?id=sendfile
func (ctx *Ctx) SendFile(file string, gzip ...bool) {
	// Disable gzipping
	if len(gzip) > 0 && !gzip[0] {
		fasthttp.ServeFileUncompressed(ctx.Fasthttp, file)
	}
	fasthttp.ServeFile(ctx.Fasthttp, file)
	// https://github.com/valyala/fasthttp/blob/master/fs.go#L81
	//ctx.Type(filepath.Ext(path))
	//ctx.Fasthttp.SendFile(path)
}

// SendStatus : https://gofiber.github.io/fiber/#/context?id=sendstatus
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

// SendString : https://gofiber.github.io/fiber/#/context?id=sendstring
func (ctx *Ctx) SendString(body string) {
	ctx.Fasthttp.Response.SetBodyString(body)
}

// Set : https://gofiber.github.io/fiber/#/context?id=set
func (ctx *Ctx) Set(key string, val string) {
	ctx.Fasthttp.Response.Header.SetCanonical(S2B(key), S2B(val))
}

// Sjson : https://github.com/tidwall/sjson
func (ctx *Ctx) Sjson(json, path string, value interface{}) error {
	raw, err := sjson.SetBytesOptions(S2B(json), path, value, &sjson.Options{
		Optimistic: true,
	})
	if err != nil {
		return err
	}
	ctx.Fasthttp.Response.Header.SetContentTypeBytes(applicationjson)
	ctx.Fasthttp.Response.SetBodyString(B2S(raw))
	return nil
}

// SjsonString : https://github.com/tidwall/sjson
func (ctx *Ctx) SjsonStr(json, path, value string) error {
	raw, err := sjson.SetBytesOptions(S2B(json), path, S2B(value), &sjson.Options{
		Optimistic: true,
	})
	if err != nil {
		return err
	}
	ctx.Fasthttp.Response.Header.SetContentTypeBytes(applicationjson)
	ctx.Fasthttp.Response.SetBodyString(B2S(raw))
	return nil
}

// Status : https://gofiber.github.io/fiber/#/context?id=status
func (ctx *Ctx) Status(status int) *Ctx {
	ctx.Fasthttp.Response.SetStatusCode(status)
	return ctx
}

// Type : https://gofiber.github.io/fiber/#/context?id=type
func (ctx *Ctx) Type(ext string) *Ctx {
	if ext[0] != '.' {
		ext = "." + ext
	}
	m := mime.TypeByExtension(ext)
	ctx.Set("Content-Type", m)
	return ctx
}

// Vary : https://gofiber.github.io/fiber/#/context?id=vary
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

// Write : https://gofiber.github.io/fiber/#/context?id=write
func (ctx *Ctx) Write(args ...interface{}) {
	if len(args) == 0 {
		panic("Missing string or []byte body")
	}
	switch body := args[0].(type) {
	case string:
		ctx.Fasthttp.Response.SetBodyString(body)
	case []byte:
		ctx.Fasthttp.Response.AppendBodyString(B2S(body))
	default:
		panic("body must be a string or []byte")
	}
}

// Xml : https://gofiber.github.io/fiber/#/context?id=xml
func (ctx *Ctx) Xml(v interface{}) error {
	raw, err := xml.Marshal(v)
	if err != nil {
		return err
	}
	ctx.Set("Content-Type", "application/xml")
	ctx.Fasthttp.Response.SetBody(raw)
	return nil
}
