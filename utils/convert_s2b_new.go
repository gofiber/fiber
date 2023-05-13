//go:build go1.20
// +build go1.20

package utils

import (
	"unsafe"
)

// UnsafeBytes returns a byte pointer without allocation.
//
//nolint:gosec // unsafe is used for better performance here
func UnsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
