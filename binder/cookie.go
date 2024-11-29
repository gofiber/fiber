package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type cookieBinding struct{}

func (*cookieBinding) Name() string {
	return "cookie"
}

func (b *cookieBinding) Bind(reqCtx *fasthttp.RequestCtx, out any) error {
	data := make(map[string][]string)
	var err error

	reqCtx.Request.Header.VisitAllCookie(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		appendValue(data, v, out, k, b.Name())
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}
