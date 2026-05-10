package binder

import (
	"errors"

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

var errUnimplementedCBOR = errors.New("must explicitly set up CBOR, please check docs: https://docs.gofiber.io/next/guide/advance-format#cbor")

// UnimplementedCborMarshal returns an error to signal that a CBOR marshaler
// must be configured before CBOR support can be used.
func UnimplementedCborMarshal(_ any) ([]byte, error) {
	return nil, errUnimplementedCBOR
}

// UnimplementedCborUnmarshal returns an error to signal that a CBOR unmarshaler
// must be configured before CBOR support can be used.
func UnimplementedCborUnmarshal(_ []byte, _ any) error {
	return errUnimplementedCBOR
}
