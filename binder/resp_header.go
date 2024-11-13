package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type respHeaderBinding struct{}

func (*respHeaderBinding) Name() string {
	return "respHeader"
}

func (b *respHeaderBinding) Bind(resp *fasthttp.Response, out any) error {
	data := make(map[string][]string)
	resp.Header.VisitAll(func(key, val []byte) {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		appendValue(data, v, out, k, b.Name())
	})

	return parse(b.Name(), out, data)
}
