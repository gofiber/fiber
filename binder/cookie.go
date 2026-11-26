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
	return bindingCookie
}

// Bind parses the request cookie and returns the result.
func (b *CookieBinding) Bind(req *fasthttp.Request, out any) error {
	mode, target := bindTarget(out)
	data := acquireBindDataMode(mode)
	defer releaseBindData(data)
	names := cookieFieldCache.cookieNames(target)

	for key, val := range req.Header.Cookies() {
		k := utils.UnsafeString(key)
		// A field takes the cookie Ctx.Cookies returns: its exact name, and the first of a repeated one.
		switch i := names.index(k); {
		case i >= 0 && i < 64:
			if data.bound&(1<<i) != 0 {
				continue
			}
			data.bound |= 1 << i
		case names == nil:
			if !data.firstOf(k) {
				continue
			}
		case names.unknown(k):
			// It binds no field, so it need not be the first.
		case !names.takes(k, i, data):
			continue
		}
		v := utils.UnsafeString(val)
		if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, false); err != nil {
			return err
		}
	}

	return data.parse(b.Name(), out)
}

// Reset resets the CookieBinding binder.
func (b *CookieBinding) Reset() {
	b.EnableSplitting = false
}
