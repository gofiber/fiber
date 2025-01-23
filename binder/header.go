package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// v is the header binder for header request body.
type HeaderBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*HeaderBinding) Name() string {
	return "header"
}

// Bind parses the request header and returns the result.
func (b *HeaderBinding) Bind(req *fasthttp.Request, out any) error {
	data := make(map[string][]string)
	var err error
	req.Header.VisitAll(func(key, val []byte) {
		if err != nil {
			return
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		err = formatBindData(out, data, k, v, b.EnableSplitting, false)
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}

// Reset resets the HeaderBinding binder.
func (b *HeaderBinding) Reset() {
	b.EnableSplitting = false
}
