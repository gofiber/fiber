// 🔌 Fiber is an Expressjs inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"encoding/base64"
	"mime"
	"mime/multipart"
	"regexp"
	"strings"

	"github.com/valyala/fasthttp"
)

// Accepts : https://gofiber.github.io/fiber/#/context?id=accepts
func (ctx *Ctx) Accepts(offers ...string) string {
	if len(offers) == 0 {
		panic("You must provide atleast one content type string")
	}
	h := ctx.Get("Accept")
	if h == "" {
		return offers[0]
	}
	specs := strings.Split(h, ",")
	for _, offer := range offers {
		mimetype := mime.TypeByExtension("." + offer)
		if mimetype != "" {
			mimetype = strings.Split(mimetype, ";")[0]
		} else {
			mimetype = offer
		}
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

// AcceptsCharsets : https://gofiber.github.io/fiber/#/context?id=acceptscharsets
func (ctx *Ctx) AcceptsCharsets(offers ...string) string {
	if len(offers) == 0 {
		panic("You must provide atleast one content type string")
	}
	h := ctx.Get("Accept-Charset")
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

// AcceptsEncodings : https://gofiber.github.io/fiber/#/context?id=acceptsencodings
func (ctx *Ctx) AcceptsEncodings(offers ...string) string {
	if len(offers) == 0 {
		panic("You must provide atleast one content type string")
	}
	h := ctx.Get("Accept-Encoding")
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

// AcceptsLanguages : https://gofiber.github.io/fiber/#/context?id=acceptslanguages
func (ctx *Ctx) AcceptsLanguages(offers ...string) string {
	if len(offers) == 0 {
		panic("You must provide atleast one content type string")
	}
	h := ctx.Get("Accept-Language")
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

// BaseUrl : https://gofiber.github.io/fiber/#/context?id=baseurl
func (ctx *Ctx) BaseUrl() string {
	return ctx.Protocol() + "://" + ctx.Hostname()
}

// BasicAuth : https://gofiber.github.io/fiber/#/context?id=basicauth
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

// Body : https://gofiber.github.io/fiber/#/context?id=body
// curl -X POST \
//   http://localhost:8080 \
//   -H 'Content-Type: application/x-www-form-urlencoded' \
//   -d john=doe
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

// Cookies : https://gofiber.github.io/fiber/#/context?id=cookies
func (ctx *Ctx) Cookies(args ...interface{}) string {
	if len(args) == 0 {
		//return b2s(ctx.Fasthttp.Response.Header.Peek("Cookie"))
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

// FormFile : https://gofiber.github.io/fiber/#/context?id=formfile
func (ctx *Ctx) FormFile(key string) (*multipart.FileHeader, error) {
	return ctx.Fasthttp.FormFile(key)
}

// FormValue : https://gofiber.github.io/fiber/#/context?id=formvalue
func (ctx *Ctx) FormValue(key string) string {
	return b2s(ctx.Fasthttp.FormValue(key))
}

// Fresh : https://gofiber.github.io/fiber/#/context?id=fresh
func (ctx *Ctx) Fresh() bool {
	return true
}

// Get : https://gofiber.github.io/fiber/#/context?id=get
func (ctx *Ctx) Get(key string) string {
	// https://en.wikipedia.org/wiki/HTTP_referer
	if key == "referrer" {
		key = "referer"
	}
	return b2s(ctx.Fasthttp.Request.Header.Peek(key))
}

// Hostname : https://gofiber.github.io/fiber/#/context?id=hostname
func (ctx *Ctx) Hostname() string {
	return b2s(ctx.Fasthttp.URI().Host())
}

// Ip : https://gofiber.github.io/fiber/#/context?id=Ip
func (ctx *Ctx) Ip() string {
	return ctx.Fasthttp.RemoteIP().String()
}

// Ips : https://gofiber.github.io/fiber/#/context?id=ips
func (ctx *Ctx) Ips() []string {
	ips := strings.Split(ctx.Get("X-Forwarded-For"), ",")
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
	}
	return ips
}

// Is : https://gofiber.github.io/fiber/#/context?id=is
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

// Locals : https://gofiber.github.io/fiber/#/context?id=locals
func (ctx *Ctx) Locals(key string, val ...interface{}) interface{} {
	if len(val) == 0 {
		return ctx.Fasthttp.UserValue(key)
	} else {
		ctx.Fasthttp.SetUserValue(key, val[0])
	}
	return nil
}

// Method : https://gofiber.github.io/fiber/#/context?id=method
func (ctx *Ctx) Method() string {
	return b2s(ctx.Fasthttp.Request.Header.Method())
}

// MultipartForm : https://gofiber.github.io/fiber/#/context?id=multipartform
func (ctx *Ctx) MultipartForm() (*multipart.Form, error) {
	return ctx.Fasthttp.MultipartForm()
}

// OriginalUrl : https://gofiber.github.io/fiber/#/context?id=originalurl
func (ctx *Ctx) OriginalUrl() string {
	return b2s(ctx.Fasthttp.Request.Header.RequestURI())
}

// Params : https://gofiber.github.io/fiber/#/context?id=params
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

// Path : https://gofiber.github.io/fiber/#/context?id=path
func (ctx *Ctx) Path() string {
	return b2s(ctx.Fasthttp.URI().Path())
}

// Protocol : https://gofiber.github.io/fiber/#/context?id=protocol
func (ctx *Ctx) Protocol() string {
	if ctx.Fasthttp.IsTLS() {
		return "https"
	}
	return "http"
}

// Query : https://gofiber.github.io/fiber/#/context?id=query
func (ctx *Ctx) Query(key string) string {
	return b2s(ctx.Fasthttp.QueryArgs().Peek(key))
}

// Range : https://gofiber.github.io/fiber/#/context?id=range
func (ctx *Ctx) Range() {

}

// Route : https://gofiber.github.io/fiber/#/context?id=route
func (ctx *Ctx) Route() (s struct {
	Method   string
	Path     string
	Wildcard bool
	Regex    *regexp.Regexp
	Params   []string
	Values   []string
	Handler  func(*Ctx)
}) {
	s.Method = ctx.route.method
	s.Path = ctx.route.path
	s.Wildcard = ctx.route.wildcard
	s.Regex = ctx.route.regex
	s.Params = ctx.route.params
	s.Values = ctx.values
	s.Handler = ctx.route.handler
	return
}

// SaveFile : https://gofiber.github.io/fiber/#/context?id=secure
func (ctx *Ctx) SaveFile(fh *multipart.FileHeader, path string) {
	fasthttp.SaveMultipartFile(fh, path)
}

// Secure : https://gofiber.github.io/fiber/#/context?id=secure
func (ctx *Ctx) Secure() bool {
	return ctx.Fasthttp.IsTLS()
}

// SignedCookies : https://gofiber.github.io/fiber/#/context?id=signedcookies
func (ctx *Ctx) SignedCookies() {

}

// Stale : https://gofiber.github.io/fiber/#/context?id=stale
func (ctx *Ctx) Stale() bool {
	return true
}

// Subdomains : https://gofiber.github.io/fiber/#/context?id=subdomains
func (ctx *Ctx) Subdomains() (subs []string) {
	subs = strings.Split(ctx.Hostname(), ".")
	subs = subs[:len(subs)-2]
	return subs
}

// Xhr : https://gofiber.github.io/fiber/#/context?id=xhr
func (ctx *Ctx) Xhr() bool {
	return ctx.Get("X-Requested-With") == "XMLHttpRequest"
}
