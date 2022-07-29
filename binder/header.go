package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3/utils"
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

		if strings.Contains(v, ",") && equalFieldType(out, reflect.Slice, k) {
			values := strings.Split(v, ",")
			for i := 0; i < len(values); i++ {
				data[k] = append(data[k], values[i])
			}
		} else {
			data[k] = append(data[k], v)
		}
	})

	return parse(b.Name(), out, data)
}
