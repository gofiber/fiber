package binder

import (
	"github.com/gofiber/utils/v2"
)

// CBORBinding is the CBOR binder for CBOR request body.
type CBORBinding struct {
	CBORDecoder utils.CBORUnmarshal
}

// Name returns the binding name.
func (*CBORBinding) Name() string {
	return "cbor"
}

// Bind parses the request body as CBOR and returns the result.
func (b *CBORBinding) Bind(body []byte, out any) error {
	return b.CBORDecoder(body, out)
}

// Reset resets the CBORBinding binder.
func (b *CBORBinding) Reset() {
	b.CBORDecoder = nil
}

// UnimplementedCborMarshal panics to signal that a CBOR marshaler must be
// configured before CBOR support can be used.
func UnimplementedCborMarshal(_ any) ([]byte, error) {
	panic("Must explicitly setup CBOR, please check docs: https://docs.gofiber.io/next/guide/advance-format#cbor")
}

// UnimplementedCborUnmarshal panics to signal that a CBOR unmarshaler must be
// configured before CBOR support can be used.
func UnimplementedCborUnmarshal(_ []byte, _ any) error {
	panic("Must explicitly setup CBOR, please check docs: https://docs.gofiber.io/next/guide/advance-format#cbor")
}
