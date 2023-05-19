//go:build !go1.20
// +build !go1.20

package utils

import (
	"unsafe"
)

// UnsafeString returns a string pointer without allocation
//
//nolint:gosec // unsafe is used for better performance here
func UnsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
