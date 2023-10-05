//go:build !go1.20

package utils

import (
	"reflect"
	"unsafe"
)

const MaxStringLen = 0x7fff0000 // Maximum string length for UnsafeBytes. (decimal: 2147418112)

// UnsafeBytes returns a byte pointer without allocation.
// String length shouldn't be more than 2147418112.
//
//nolint:gosec // unsafe is used for better performance here
func UnsafeBytes(s string) []byte {
	if s == "" {
		return nil
	}

	return (*[MaxStringLen]byte)(unsafe.Pointer(
		(*reflect.StringHeader)(unsafe.Pointer(&s)).Data),
	)[:len(s):len(s)]
}
