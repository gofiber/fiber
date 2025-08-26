package binder

import (
	utils "github.com/gofiber/utils/v2"
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
	for key, val := range req.Header.All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		_ = formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, false) //nolint:errcheck // always returns nil
	}

	return parse(b.Name(), out, data)
}

// Reset resets the HeaderBinding binder.
func (b *HeaderBinding) Reset() {
	b.EnableSplitting = false
}
