package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type cookieBinding struct{}

func (*cookieBinding) Name() string {
	return "cookie"
}

func (b *cookieBinding) Bind(reqCtx *fasthttp.RequestCtx, out any) error {
	data := map[string][]string{}
	var err error

	reqCtx.Request.Header.VisitAllCookie(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		sliceDetected := strings.Contains(v, ",")

		if sliceDetected && equalFieldType(out, reflect.Slice, k) {
			values := strings.Split(v, ",")
			tempSlice := data[k]
			for i := 0; i < len(values); i++ {
				tempSlice = append(tempSlice, values[i])
			}
			data[k] = tempSlice
		} else {
			data[k] = append(data[k], v)
		}
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}
