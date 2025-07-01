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
	var err error

	req.Header.Cookies()(func(key, val []byte) bool {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		err = formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, false)
		return err == nil // Stop iteration on the first error
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}

// Reset resets the CookieBinding binder.
func (b *CookieBinding) Reset() {
	b.EnableSplitting = false
}
