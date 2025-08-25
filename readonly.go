package fiber

import "unsafe"

//go:linkname runtime_rodata runtime.rodata
var runtime_rodata byte

//go:linkname runtime_erodata runtime.erodata
var runtime_erodata byte

func isReadOnly(p unsafe.Pointer) bool {
	start := uintptr(unsafe.Pointer(&runtime_rodata)) //nolint:gosec // converting runtime symbols
	end := uintptr(unsafe.Pointer(&runtime_erodata))  //nolint:gosec // converting runtime symbols
	addr := uintptr(p)                                //nolint:gosec // pointer arithmetic for rodata check
	return addr >= start && addr < end
}
