package reflectunsafe

import (
	"reflect"
	"unsafe"
)

func ValueAndTypeID(v any) (reflect.Value, uintptr) {
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	return rv, (*[2]uintptr)(unsafe.Pointer(&rt))[1]
}
