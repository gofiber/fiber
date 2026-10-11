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
	return bindingQuery
}

// Bind parses the request query and returns the result.
func (b *QueryBinding) Bind(reqCtx *fasthttp.Request, out any) error {
	args := reqCtx.URI().QueryArgs()
	data := acquireBindData(out, args.Len())
	defer releaseBindData(data)

	for key, val := range args.All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, true); err != nil {
			return err
		}
	}

	return data.parse(b.Name(), out)
}

// Reset resets the QueryBinding binder.
func (b *QueryBinding) Reset() {
	b.EnableSplitting = false
}
