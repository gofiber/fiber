package gotiny

import (
	"reflect"
	"unsafe"
)

const (
	kindDirectIface = 1 << 5
)

// rtype is the common implementation of most values.
// It is embedded in other struct types.
//
// rtype must be kept in sync with reflect/type.go:/^type._type.
type rtype struct {
	_    uintptr
	_    uintptr // number of bytes in the type that can contain pointers
	_    uint32  // hash of type; avoids computation in hash tables
	_    uint8   // extra type information flags
	_    uint8   // alignment of variable with this type
	_    uint8   // alignment of struct field with this type
	kind uint8   // enumeration for C
	_    uintptr // algorithm table
	_    uintptr // garbage collection data
	_    int32   // string form
	_    int32   // type for pointer to this type, may be zero
}

// ifaceIndir reports whether t is stored indirectly in an interface value.
func ifaceDirect(t *rtype) bool {
	return t.kind&kindDirectIface != 0
}

func directType(rt *reflect.Type) bool {
	return ifaceDirect((*rtype)((*[2]unsafe.Pointer)(unsafe.Pointer(rt))[1]))
}

type refVal struct {
	_    unsafe.Pointer
	ptr  unsafe.Pointer
	flag flag
}

type flag uintptr

//go:linkname flagIndir reflect.flagIndir
const flagIndir flag = 1 << 7

func getUnsafePointer(rv *reflect.Value) unsafe.Pointer {
	vv := (*refVal)(unsafe.Pointer(rv))
	if vv.flag&flagIndir == 0 {
		return unsafe.Pointer(&vv.ptr)
	} else {
		return vv.ptr
	}
}
