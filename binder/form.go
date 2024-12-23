package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// formBinding is the form binder for form request body.
type formBinding struct{}

// Name returns the binding name.
func (*formBinding) Name() string {
	return "form"
}

// Bind parses the request body and returns the result.
func (b *formBinding) Bind(reqCtx *fasthttp.RequestCtx, out any) error {
	data := make(map[string][]string)
	var err error

	reqCtx.PostArgs().VisitAll(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		if strings.Contains(k, "[") {
			k, err = parseParamSquareBrackets(k)
		}

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

// BindMultipart parses the request body and returns the result.
func (b *formBinding) BindMultipart(reqCtx *fasthttp.RequestCtx, out any) error {
	data, err := reqCtx.MultipartForm()
	if err != nil {
		return err
	}

	for key, values := range data.Value {
		if strings.Contains(key, "[") {
			k, err := parseParamSquareBrackets(key)
			if err != nil {
				return err
			}
			data.Value[k] = values
			delete(data.Value, key) // Remove bracket notation and use dot instead
		}

		for _, v := range values {
			if strings.Contains(v, ",") && equalFieldType(out, reflect.Slice, key) {
				delete(data.Value, key)

				values := strings.Split(v, ",")
				for i := 0; i < len(values); i++ {
					data.Value[key] = append(data.Value[key], values[i])
				}
			}
		}
	}

	return parse(b.Name(), out, data.Value)
}
