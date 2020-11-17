// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import "testing"

func Test_GetString(t *testing.T) {
	t.Parallel()
	res := GetString([]byte("Hello, World!"))
	AssertEqual(t, "Hello, World!", res)
}

// go test -v -run=^$ -bench=GetString -benchmem -count=2

func Benchmark_GetString(b *testing.B) {
	var hello = []byte("Hello, World!")
	var res string
	b.Run("unsafe", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = GetString(hello)
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

func Test_GetBytes(t *testing.T) {
	t.Parallel()
	res := GetBytes("Hello, World!")
	AssertEqual(t, []byte("Hello, World!"), res)
}

// go test -v -run=^$ -bench=GetBytes -benchmem -count=4

func Benchmark_GetBytes(b *testing.B) {
	var hello = "Hello, World!"
	var res []byte
	b.Run("unsafe", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = GetBytes(hello)
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

func Test_ImmutableString(t *testing.T) {
	t.Parallel()
	res := ImmutableString("Hello, World!")
	AssertEqual(t, "Hello, World!", res)
}
