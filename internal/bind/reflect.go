package bind

import (
	"reflect"
	"unsafe"
)

func ValueAndTypeID(v any) (reflect.Value, uintptr) {
	header := (*emptyInterface)(unsafe.Pointer(&v))

	rv := reflect.ValueOf(v)
	return rv, header.typeID
}

type emptyInterface struct {
	typeID  uintptr
	dataPtr unsafe.Pointer
}
