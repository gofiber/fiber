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
	data := acquireBindData(out, 0)
	defer releaseBindData(data)
	if data.mode != bindPairs {
		// A map files each name once, so it holds fewer names than the cookies only when one repeats: until then no
		// name is checked.
		taken := 0
		for key, val := range req.Header.Cookies() {
			taken++
			if err := data.bind(b.Name(), out, utils.UnsafeString(key), utils.UnsafeString(val), b.EnableSplitting, false); err != nil {
				return err
			}
		}
		if taken > data.filedNames() {
			data.reset()
			if err := b.bindFirst(req, out, data); err != nil {
				return err
			}
		}
		return data.parse(b.Name(), out)
	}
	if err := b.bindFirst(req, out, data); err != nil {
		return err
	}
	return data.parse(b.Name(), out)
}

// bindFirst files the first cookie of each name in data, as Ctx.Cookies returns it.
func (b *CookieBinding) bindFirst(req *fasthttp.Request, out any, data *bindData) error {
	for key, val := range req.Header.Cookies() {
		if k := utils.UnsafeString(key); data.firstOf(k) {
			if err := data.bind(b.Name(), out, k, utils.UnsafeString(val), b.EnableSplitting, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reset resets the CookieBinding binder.
func (b *CookieBinding) Reset() {
	b.EnableSplitting = false
}
