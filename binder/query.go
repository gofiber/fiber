package binder

import (
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

	reqCtx.URI().QueryArgs().All()(func(key, val []byte) bool {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		err = formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, true)
		return err == nil // Stop iteration on the first error
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
