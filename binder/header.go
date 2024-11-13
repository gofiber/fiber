package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type headerBinding struct{}

func (*headerBinding) Name() string {
	return "header"
}

func (b *headerBinding) Bind(req *fasthttp.Request, out any) error {
	data := make(map[string][]string)
	req.Header.VisitAll(func(key, val []byte) {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		appendValue(data, v, out, k, b.Name())
	})

	return parse(b.Name(), out, data)
}
