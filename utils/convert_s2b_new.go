//go:build go1.20
// +build go1.20

package utils

import (
	"unsafe" //nolint:depguard // unsafe is used for better performance
)

// UnsafeBytes returns a byte pointer without allocation.
func UnsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
