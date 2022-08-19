// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"testing"
)

func Test_ToLowerBytes(t *testing.T) {
	t.Parallel()
	res := ToLowerBytes([]byte("/MY/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/my/name/is/:param/*"), res))
	res = ToLowerBytes([]byte("/MY1/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/my1/name/is/:param/*"), res))
	res = ToLowerBytes([]byte("/MY2/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/my2/name/is/:param/*"), res))
	res = ToLowerBytes([]byte("/MY3/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/my3/name/is/:param/*"), res))
	res = ToLowerBytes([]byte("/MY4/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/my4/name/is/:param/*"), res))
}

func Benchmark_ToLowerBytes(b *testing.B) {
	path := []byte(largeStr)
	want := []byte(lowerStr)
	var res []byte
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToLowerBytes(path)
		}
		AssertEqual(b, bytes.Equal(want, res), true)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToLower(path)
		}
		AssertEqual(b, bytes.Equal(want, res), true)
	})
}

func Test_ToUpperBytes(t *testing.T) {
	t.Parallel()
	res := ToUpperBytes([]byte("/my/name/is/:param/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/MY/NAME/IS/:PARAM/*"), res))
	res = ToUpperBytes([]byte("/my1/name/is/:param/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/MY1/NAME/IS/:PARAM/*"), res))
	res = ToUpperBytes([]byte("/my2/name/is/:param/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/MY2/NAME/IS/:PARAM/*"), res))
	res = ToUpperBytes([]byte("/my3/name/is/:param/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/MY3/NAME/IS/:PARAM/*"), res))
	res = ToUpperBytes([]byte("/my4/name/is/:param/*"))
	AssertEqual(t, true, bytes.Equal([]byte("/MY4/NAME/IS/:PARAM/*"), res))
}

func Benchmark_ToUpperBytes(b *testing.B) {
	path := []byte(largeStr)
	want := []byte(upperStr)
	var res []byte
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToUpperBytes(path)
		}
		AssertEqual(b, bytes.Equal(want, res), true)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToUpper(path)
		}
		AssertEqual(b, bytes.Equal(want, res), true)
	})
}
