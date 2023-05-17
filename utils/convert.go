// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// CopyString copies a string to make it immutable
func CopyString(s string) string {
	return string(UnsafeBytes(s))
}

// CopyBytes copies a slice to make it immutable
func CopyBytes(b []byte) []byte {
	tmp := make([]byte, len(b))
	copy(tmp, b)
	return tmp
}

const (
	uByte = 1 << (10 * iota) // 1 << 10 == 1024
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
		unit = "EB"
		value /= uExabyte
	case bytes >= uPetabyte:
		unit = "PB"
		value /= uPetabyte
	case bytes >= uTerabyte:
		unit = "TB"
		value /= uTerabyte
	case bytes >= uGigabyte:
		unit = "GB"
		value /= uGigabyte
	case bytes >= uMegabyte:
		unit = "MB"
		value /= uMegabyte
	case bytes >= uKilobyte:
		unit = "KB"
		value /= uKilobyte
	case bytes >= uByte:
		unit = "B"
	default:
		return "0B"
	}
	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// ToString Change arg to string
func ToString(arg interface{}, timeFormat ...string) string {
	tmp := reflect.Indirect(reflect.ValueOf(arg)).Interface()
	switch v := tmp.(type) {
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.Itoa(int(v))
	case uint8:
		return strconv.FormatInt(int64(v), 10)
	case uint16:
		return strconv.FormatInt(int64(v), 10)
	case uint32:
		return strconv.FormatInt(int64(v), 10)
	case uint64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	case []byte:
		return string(v)
	case bool:
		return strconv.FormatBool(v)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case time.Time:
		if len(timeFormat) > 0 {
			return v.Format(timeFormat[0])
		}
		return v.Format("2006-01-02 15:04:05")
	case reflect.Value:
		return ToString(v.Interface(), timeFormat...)
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}
