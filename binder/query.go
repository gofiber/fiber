package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// QueryBinding is the query binder for query request body.
type QueryBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*QueryBinding) Name() string {
	return "query"
}

// Bind parses the request query and returns the result.
func (b *QueryBinding) Bind(reqCtx *fasthttp.Request, out any) error {
	data := make(map[string][]string)
	var err error

	reqCtx.URI().QueryArgs().VisitAll(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		if strings.Contains(k, "[") {
			k, err = parseParamSquareBrackets(k)
		}

		if b.EnableSplitting && strings.Contains(v, ",") && equalFieldType(out, reflect.Slice, k) {
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

// Reset resets the QueryBinding binder.
func (b *QueryBinding) Reset() {
	b.EnableSplitting = false
}
