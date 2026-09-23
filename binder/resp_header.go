package binder

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// RespHeaderBinding is the respHeader binder for response header.
type RespHeaderBinding struct {
	EnableSplitting bool
}

// Name returns the binding name.
func (*RespHeaderBinding) Name() string {
	return bindingRespHeader
}

// Bind parses the response header and returns the result.
func (b *RespHeaderBinding) Bind(resp *fasthttp.Response, out any) error {
	data := acquireBindData(out)
	defer releaseBindData(data)

	for key, val := range resp.Header.All() {
		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		if err := data.bind(b.Name(), out, k, v, b.EnableSplitting, false); err != nil {
			return err
		}
	}

	return data.parse(b.Name(), out)
}

// Reset resets the RespHeaderBinding binder.
func (b *RespHeaderBinding) Reset() {
	b.EnableSplitting = false
}
