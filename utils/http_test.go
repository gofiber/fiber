// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"mime"
	"net/http"
	"testing"
)

func Test_Utils_GetMIME(t *testing.T) {
	t.Parallel()
	res := GetMIME(".json")
	AssertEqual(t, "application/json", res)

	res = GetMIME(".xml")
	AssertEqual(t, "application/xml", res)

	res = GetMIME("xml")
	AssertEqual(t, "application/xml", res)

	res = GetMIME("unknown")
	AssertEqual(t, MIMEOctetStream, res)
	// empty case
	res = GetMIME("")
	AssertEqual(t, "", res)
}

// go test -v -run=^$ -bench=Benchmark_GetMIME -benchmem -count=2
func Benchmark_GetMIME(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = GetMIME(".xml")
			res = GetMIME(".txt")
			res = GetMIME(".png")
			res = GetMIME(".exe")
			res = GetMIME(".json")
		}
		AssertEqual(b, "application/json", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = mime.TypeByExtension(".xml")
			res = mime.TypeByExtension(".txt")
			res = mime.TypeByExtension(".png")
			res = mime.TypeByExtension(".exe")
			res = mime.TypeByExtension(".json")
		}
		AssertEqual(b, "application/json", res)
	})
}

func Test_Utils_StatusMessage(t *testing.T) {
	t.Parallel()
	res := StatusMessage(204)
	AssertEqual(t, "No Content", res)

	res = StatusMessage(404)
	AssertEqual(t, "Not Found", res)

	res = StatusMessage(426)
	AssertEqual(t, "Upgrade Required", res)

	res = StatusMessage(511)
	AssertEqual(t, "Network Authentication Required", res)

	res = StatusMessage(1337)
	AssertEqual(t, "", res)

	res = StatusMessage(-1)
	AssertEqual(t, "", res)

	res = StatusMessage(0)
	AssertEqual(t, "", res)

	res = StatusMessage(600)
	AssertEqual(t, "", res)
}

// go test -run=^$ -bench=Benchmark_StatusMessage -benchmem -count=2
func Benchmark_StatusMessage(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = StatusMessage(http.StatusNotExtended)
		}
		AssertEqual(b, "Not Extended", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = http.StatusText(http.StatusNotExtended)
		}
		AssertEqual(b, "Not Extended", res)
	})
}
