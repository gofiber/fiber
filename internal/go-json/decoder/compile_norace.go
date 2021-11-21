//go:build !race
// +build !race

package decoder

import (
	"unsafe"

	"github.com/gofiber/fiber/v2/internal/go-json/runtime"
)

func CompileToGetDecoder(typ *runtime.Type) (Decoder, error) {
	typeptr := uintptr(unsafe.Pointer(typ))
	if typeptr > typeAddr.MaxTypeAddr {
		return compileToGetDecoderSlowPath(typeptr, typ)
	}

	index := (typeptr - typeAddr.BaseTypeAddr) >> typeAddr.AddrShift
	if dec := cachedDecoder[index]; dec != nil {
		return dec, nil
	}

	dec, err := compileHead(typ, map[uintptr]Decoder{})
	if err != nil {
		return nil, err
	}
	cachedDecoder[index] = dec
	return dec, nil
}
