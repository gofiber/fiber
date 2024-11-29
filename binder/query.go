package binder

import (
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type queryBinding struct{}

func (*queryBinding) Name() string {
	return "query"
}

func (b *queryBinding) Bind(reqCtx *fasthttp.RequestCtx, out any) error {
	data := make(map[string][]string)
	var err error

	reqCtx.QueryArgs().VisitAll(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)

		if strings.Contains(k, "[") {
			k, err = parseParamSquareBrackets(k)
		}

		appendValue(data, v, out, k, b.Name())
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}
