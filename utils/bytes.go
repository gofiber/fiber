// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

// ToLowerBytes converts ascii slice to lower-case in-place.
// Explanation : if string(77) = M, then string(77+32) = m
func ToLowerBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] + 32
		}
	}
	return b
}

// ToUpperBytes converts ascii slice to upper-case in-place.
// Explanation : if string(97) = a, then string(97-32) = A
func ToUpperBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] = b[i] - 32
		}
	}
	return b
}

// TrimRightBytes is the equivalent of bytes.TrimRight
func TrimRightBytes(b []byte, cutset byte) []byte {
	lenStr := len(b)
	for lenStr > 0 && b[lenStr-1] == cutset {
		lenStr--
	}
	return b[:lenStr]
}

// TrimLeftBytes is the equivalent of bytes.TrimLeft
func TrimLeftBytes(b []byte, cutset byte) []byte {
	lenStr, start := len(b), 0
	for start < lenStr && b[start] == cutset {
		start++
	}
	return b[start:]
}

// TrimBytes is the equivalent of bytes.Trim
func TrimBytes(b []byte, cutset byte) []byte {
	i, j := 0, len(b)-1
	for ; i <= j; i++ {
		if b[i] != cutset {
			break
		}
	}
	for ; i < j; j-- {
		if b[j] != cutset {
			break
		}
	}

	return b[i : j+1]
}

// EqualFoldBytes tests ascii slices for equality case-insensitively
func EqualFoldBytes(b, s []byte) bool {
	if len(b) != len(s) {
		return false
	}
	for i := len(b) - 1; i >= 0; i-- {
        // Check only for lowe case bytes
		if b[i] >= 'a' && b[i] <= 'z' && b[i]+32 != s[i]+32 {
			return false
		}
	}
	return true
}
