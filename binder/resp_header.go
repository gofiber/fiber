package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// respHeaderBinding is the respHeader binder for response header.
type respHeaderBinding struct{}

// Name returns the binding name.
func (*respHeaderBinding) Name() string {
	return "respHeader"
}

// Bind parses the response header and returns the result.
func (b *respHeaderBinding) Bind(resp *fasthttp.Response, out any) error {
	data := make(map[string][]string)
	resp.Header.VisitAll(func(key, val []byte) {
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
