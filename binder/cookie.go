package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// CookieBinding is the cookie binder for cookie request body.
type CookieBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*CookieBinding) Name() string {
	return "cookie"
}

// Bind parses the request cookie and returns the result.
func (b *CookieBinding) Bind(req *fasthttp.Request, out any) error {
	data := make(map[string][]string)

	for key, val := range req.Header.Cookies() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, false); err != nil {
			return err
		}
	}

	return parse(b.Name(), out, data)
}

// Reset resets the CookieBinding binder.
func (b *CookieBinding) Reset() {
	b.EnableSplitting = false
}
