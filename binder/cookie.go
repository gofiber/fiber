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
	if mode != bindPairs {
		// A map files each name once, so it holds fewer names than the cookies only when one repeats: until then no
		// name is checked.
		taken := 0
		for key, val := range req.Header.Cookies() {
			taken++
			k, v := utils.UnsafeString(key), utils.UnsafeString(val)
			if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, false); err != nil {
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

// bindFirst files the first cookie of each name in data.
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
