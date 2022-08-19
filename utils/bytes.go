// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

// ToLowerBytes converts ascii slice to lower-case
func ToLowerBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		b[i] = toLowerTable[b[i]]
	}
	return b
}

// ToUpperBytes converts ascii slice to upper-case
func ToUpperBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		b[i] = toUpperTable[b[i]]
	}
	return b
}
