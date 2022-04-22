//go:build !purego && !appengine
// +build !purego,!appengine

package msgp

import (
	"reflect"
	"unsafe"
)

// NOTE:
// all of the definition in this file
// should be repeated in appengine.go,
// but without using unsafe

const (
	// spec says int and uint are always
	// the same size, but that int/uint
	// size may not be machine word size
	smallint = unsafe.Sizeof(int(0)) == 4
)

// UnsafeString returns the byte slice as a volatile string
// THIS SHOULD ONLY BE USED BY THE CODE GENERATOR.
// THIS IS EVIL CODE.
// YOU HAVE BEEN WARNED.
func UnsafeString(b []byte) string {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	return *(*string)(unsafe.Pointer(&reflect.StringHeader{Data: sh.Data, Len: sh.Len}))
}

// UnsafeBytes returns the string as a byte slice
// THIS SHOULD ONLY BE USED BY THE CODE GENERATOR.
// THIS IS EVIL CODE.
// YOU HAVE BEEN WARNED.
func UnsafeBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Len:  len(s),
		Cap:  len(s),
		Data: (*(*reflect.StringHeader)(unsafe.Pointer(&s))).Data,
	}))
}
