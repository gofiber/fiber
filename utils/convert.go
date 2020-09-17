// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"reflect"
	"strconv"
	"strings"
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

const (
	uByte = 1 << (10 * iota)
	uKilobyte
	uMegabyte
	uGigabyte
	uTerabyte
	uPetabyte
	uExabyte
)

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
func ByteSize(bytes uint64) string {
	unit := ""
	value := float64(bytes)
	switch {
	case bytes >= uExabyte:
		unit = "E"
		value = value / uExabyte
	case bytes >= uPetabyte:
		unit = "P"
		value = value / uPetabyte
	case bytes >= uTerabyte:
		unit = "T"
		value = value / uTerabyte
	case bytes >= uGigabyte:
		unit = "G"
		value = value / uGigabyte
	case bytes >= uMegabyte:
		unit = "M"
		value = value / uMegabyte
	case bytes >= uKilobyte:
		unit = "K"
		value = value / uKilobyte
	case bytes >= uByte:
		unit = "B"
	default:
		return "0B"
	}
	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}
