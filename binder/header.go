package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// HeaderBinding is the binder implementation used to populate values from HTTP headers.
type HeaderBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*HeaderBinding) Name() string {
	return bindingHeader
}

// Bind parses the request header and returns the result.
func (b *HeaderBinding) Bind(req *fasthttp.Request, out any) error {
	data := acquireBindData(out)
	defer releaseBindData(data)

	for key, val := range req.Header.All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, false); err != nil {
			return err
		}
	}

	return data.parse(b.Name(), out)
}

// Reset resets the HeaderBinding binder.
func (b *HeaderBinding) Reset() {
	b.EnableSplitting = false
}
