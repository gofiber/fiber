package binder

import (
	"github.com/gofiber/utils/v2"
)

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

func UnimplementedMsgpackMarshal(_ any) ([]byte, error) {
	panic("Must explicits setup Msgpack, please check docs: https://docs.gofiber.io/next/guide/advance-format#msgpack")
}

func UnimplementedMsgpackUnmarshal(_ []byte, _ any) error {
	panic("Must explicits setup Msgpack, please check docs: https://docs.gofiber.io/next/guide/advance-format#msgpack")
}
