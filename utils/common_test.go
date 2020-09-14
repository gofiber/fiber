// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func Test_Utils_FunctionName(t *testing.T) {
	t.Parallel()
	AssertEqual(t, "github.com/gofiber/fiber/v2/utils.Test_Utils_UUID", FunctionName(Test_Utils_UUID))

	AssertEqual(t, "github.com/gofiber/fiber/v2/utils.Test_Utils_FunctionName.func1", FunctionName(func() {}))

	var dummyint = 20
	AssertEqual(t, "int", FunctionName(dummyint))
}

func Test_Utils_UUID(t *testing.T) {
	t.Parallel()
	res := UUID()
	AssertEqual(t, 36, len(res))
}

// go test -v -run=^$ -bench=Benchmark_UUID -benchmem -count=2

func Benchmark_UUID(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = UUID()
		}
		AssertEqual(b, 36, len(res))
	})
	b.Run("default", func(b *testing.B) {
		rnd := make([]byte, 16)
		_, _ = rand.Read(rnd)
		for n := 0; n < b.N; n++ {
			res = fmt.Sprintf("%x-%x-%x-%x-%x", rnd[0:4], rnd[4:6], rnd[6:8], rnd[8:10], rnd[10:])
		}
		AssertEqual(b, 36, len(res))
	})
}
