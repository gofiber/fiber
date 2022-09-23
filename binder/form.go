package binder

import (
	"reflect"
	"strings"

	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

type formBinding struct{}

func (*formBinding) Name() string {
	return "form"
}

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

	return parse(b.Name(), out, data)
}

func (b *formBinding) BindMultipart(reqCtx *fasthttp.RequestCtx, out any) error {
	data, err := reqCtx.MultipartForm()
	if err != nil {
		return err
	}

	return parse(b.Name(), out, data.Value)
}
