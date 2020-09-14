// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"reflect"
	"unsafe"
)

// #nosec G103
// GetString returns a string pointer without allocation
func GetString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// #nosec G103
// GetBytes returns a byte pointer without allocation
func GetBytes(s string) (bs []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return
}

// ImmutableString copies a string to make it immutable
func ImmutableString(s string) string {
	return string(GetBytes(s))
}
