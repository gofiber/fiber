// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"testing"
)

func Test_UnsafeString(t *testing.T) {
	t.Parallel()
	res := UnsafeString([]byte("Hello, World!"))
	AssertEqual(t, "Hello, World!", res)
}

// go test -v -run=^$ -bench=UnsafeString -benchmem -count=2

func Benchmark_UnsafeString(b *testing.B) {
	hello := []byte("Hello, World!")
	var res string
	b.Run("unsafe", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = UnsafeString(hello)
		}
		AssertEqual(b, "Hello, World!", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = string(hello)
		}
		AssertEqual(b, "Hello, World!", res)
	})
}

func Test_UnsafeBytes(t *testing.T) {
	t.Parallel()
	res := UnsafeBytes("Hello, World!")
	AssertEqual(t, []byte("Hello, World!"), res)
}

// go test -v -run=^$ -bench=UnsafeBytes -benchmem -count=4

func Benchmark_UnsafeBytes(b *testing.B) {
	hello := "Hello, World!"
	var res []byte
	b.Run("unsafe", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = UnsafeBytes(hello)
		}
		AssertEqual(b, []byte("Hello, World!"), res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = []byte(hello)
		}
		AssertEqual(b, []byte("Hello, World!"), res)
	})
}

func Test_CopyString(t *testing.T) {
	t.Parallel()
	res := CopyString("Hello, World!")
	AssertEqual(t, "Hello, World!", res)
}

func Test_ToString(t *testing.T) {
	t.Parallel()
	res := ToString([]byte("Hello, World!"))
	AssertEqual(t, "Hello, World!", res)
	res = ToString(true)
	AssertEqual(t, "true", res)
	res = ToString(uint(100))
	AssertEqual(t, "100", res)
}

// go test -v -run=^$ -bench=ToString -benchmem -count=2
func Benchmark_ToString(b *testing.B) {
	hello := []byte("Hello, World!")
	for n := 0; n < b.N; n++ {
		ToString(hello)
	}
}
