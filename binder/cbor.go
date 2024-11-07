package binder

import (
	"github.com/gofiber/utils/v2"
)

type cborBinding struct{}

func (*cborBinding) Name() string {
	return "cbor"
}

func (*cborBinding) Bind(body []byte, cborDecoder utils.CBORUnmarshal, out any) error {
	return cborDecoder(body, out)
}
