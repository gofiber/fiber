package encoder

import (
	"testing"
	"unsafe"
)

func TestDumpOpcode(t *testing.T) {
	ctx := TakeRuntimeContext()
	defer ReleaseRuntimeContext(ctx)
	var v interface{} = 1
	header := (*emptyInterface)(unsafe.Pointer(&v))
	typ := header.typ
	typeptr := uintptr(unsafe.Pointer(typ))
	codeSet, err := CompileToGetCodeSet(ctx, typeptr)
	if err != nil {
		t.Fatal(err)
	}
	codeSet.EscapeKeyCode.Dump()
}
