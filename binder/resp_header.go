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
	return "respHeader"
}

// Bind parses the response header and returns the result.
func (b *RespHeaderBinding) Bind(resp *fasthttp.Response, out any) error {
	data := make(map[string][]string)
	var err error

	resp.Header.All()(func(key, val []byte) bool {
		if err != nil {
			return true
		}

		k := utils.UnsafeString(key)
		v := utils.UnsafeString(val)
		err = formatBindData(b.Name(), out, data, k, v, b.EnableSplitting, false)
		return true
	})

	if err != nil {
		return err
	}

	return parse(b.Name(), out, data)
}

// Reset resets the RespHeaderBinding binder.
func (b *RespHeaderBinding) Reset() {
	b.EnableSplitting = false
}
