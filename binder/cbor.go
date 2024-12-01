package binder

import (
	"github.com/gofiber/utils/v2"
)

// cborBinding is the CBOR binder for CBOR request body.
type cborBinding struct{}

// Name returns the binding name.
func (*cborBinding) Name() string {
	return "cbor"
}

// Bind parses the request body as CBOR and returns the result.
func (*cborBinding) Bind(body []byte, cborDecoder utils.CBORUnmarshal, out any) error {
	return cborDecoder(body, out)
}
