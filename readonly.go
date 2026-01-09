//go:build !s390x && !ppc64 && !ppc64le

package fiber

import (
	"unsafe"
)

//go:linkname runtimeRodata runtime.rodata
var runtimeRodata byte

//go:linkname runtimeErodata runtime.erodata
var runtimeErodata byte

func isReadOnly(p unsafe.Pointer) bool {
	start := uintptr(unsafe.Pointer(&runtimeRodata)) //nolint:gosec // converting runtime symbols
	end := uintptr(unsafe.Pointer(&runtimeErodata))  //nolint:gosec // converting runtime symbols
	addr := uintptr(p)
	return addr >= start && addr < end
}
