//go:build go1.20
// +build go1.20

package utils

import (
	"unsafe" //nolint:depguard // unsafe is used for better performance
)

// UnsafeString returns a string pointer without allocation
func UnsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
