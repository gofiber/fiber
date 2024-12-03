package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// cookieBinding is the cookie binder for cookie request body.
type cookieBinding struct{}

// Name returns the binding name.
func (*cookieBinding) Name() string {
	return "cookie"
}

// Bind parses the request cookie and returns the result.
func (b *cookieBinding) Bind(reqCtx *fasthttp.RequestCtx, out any) error {
	data := make(map[string][]string)
	var err error

	reqCtx.Request.Header.VisitAllCookie(func(key, val []byte) {
		if err != nil {
			return
		}

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

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}
