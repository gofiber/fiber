package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// headerBinding is the header binder for header request body.
type headerBinding struct{}

// Name returns the binding name.
func (*headerBinding) Name() string {
	return "header"
}

// Bind parses the request header and returns the result.
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
