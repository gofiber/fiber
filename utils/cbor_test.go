package utils

import (
	"encoding/hex"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func Test_GolangCBOREncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &sampleStructure{
			ImportantString: "Hello World",
		}
		importantString             = `a170696d706f7274616e745f737472696e676b48656c6c6f20576f726c64`
		cborEncoder     CBORMarshal = cbor.Marshal
	)

	raw, err := cborEncoder(ss)
	AssertEqual(t, err, nil)

	AssertEqual(t, hex.EncodeToString([]byte(raw)), importantString)
}

func Test_DefaultCBOREncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &sampleStructure{
			ImportantString: "Hello World",
		}
		importantString             = `a170696d706f7274616e745f737472696e676b48656c6c6f20576f726c64`
		cborEncoder     CBORMarshal = cbor.Marshal
	)

	raw, err := cborEncoder(ss)
	AssertEqual(t, err, nil)

	AssertEqual(t, hex.EncodeToString([]byte(raw)), importantString)
}

func Test_DefaultCBORDecoder(t *testing.T) {
	t.Parallel()

	var (
		ss                 sampleStructure
		importantString, _               = hex.DecodeString("a170696d706f7274616e745f737472696e676b48656c6c6f20576f726c64")
		cborDecoder        CBORUnmarshal = cbor.Unmarshal
	)

	err := cborDecoder(importantString, &ss)
	AssertEqual(t, err, nil)
	AssertEqual(t, "Hello World", ss.ImportantString)
}
