package binder

import (
	"errors"

	"github.com/gofiber/utils/v2"
)

var ErrMsgpackNotConfigured = errors.New("msgpack is not configured: please check docs https://docs.gofiber.io/next/guide/advance-format#msgpack")

// MsgPackBinding is the MsgPack binder for MsgPack request body.
type MsgPackBinding struct {
	MsgPackDecoder utils.MsgPackUnmarshal
}

// Name returns the binding name.
func (*MsgPackBinding) Name() string {
	return "msgpack"
}

// Bind parses the request body as MsgPack and returns the result.
func (b *MsgPackBinding) Bind(body []byte, out any) error {
	return b.MsgPackDecoder(body, out)
}

// Reset resets the MsgPackBinding binder.
func (b *MsgPackBinding) Reset() {
	b.MsgPackDecoder = nil
}

// UnimplementedMsgpackMarshal returns an error to signal that a Msgpack marshaler must
// be configured before MsgPack support can be used.
func UnimplementedMsgpackMarshal(_ any) ([]byte, error) {
	return nil, ErrMsgpackNotConfigured
}

// UnimplementedMsgpackUnmarshal returns an error to signal that a Msgpack unmarshaler
// must be configured before MsgPack support can be used.
func UnimplementedMsgpackUnmarshal(_ []byte, _ any) error {
	return ErrMsgpackNotConfigured
}
