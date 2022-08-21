// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"strings"
	"testing"
)

func Test_ToUpper(t *testing.T) {
	t.Parallel()
	AssertEqual(t, "/MY/NAME/IS/:PARAM/*", ToUpper("/my/name/is/:param/*"))
}

const (
	largeStr = "/RePos/GoFiBer/FibEr/iSsues/187643/CoMmEnts/RePos/GoFiBer/FibEr/iSsues/CoMmEnts"
	upperStr = "/REPOS/GOFIBER/FIBER/ISSUES/187643/COMMENTS/REPOS/GOFIBER/FIBER/ISSUES/COMMENTS"
	lowerStr = "/repos/gofiber/fiber/issues/187643/comments/repos/gofiber/fiber/issues/comments"
)

func Benchmark_ToUpper(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToUpper(largeStr)
		}
		AssertEqual(b, upperStr, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.ToUpper(largeStr)
		}
		AssertEqual(b, upperStr, res)
	})
}

func Test_ToLower(t *testing.T) {
	t.Parallel()
	AssertEqual(t, "/my/name/is/:param/*", ToLower("/MY/NAME/IS/:PARAM/*"))
	AssertEqual(t, "/my1/name/is/:param/*", ToLower("/MY1/NAME/IS/:PARAM/*"))
	AssertEqual(t, "/my2/name/is/:param/*", ToLower("/MY2/NAME/IS/:PARAM/*"))
	AssertEqual(t, "/my3/name/is/:param/*", ToLower("/MY3/NAME/IS/:PARAM/*"))
	AssertEqual(t, "/my4/name/is/:param/*", ToLower("/MY4/NAME/IS/:PARAM/*"))
}

func Benchmark_ToLower(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToLower(largeStr)
		}
		AssertEqual(b, lowerStr, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.ToLower(largeStr)
		}
		AssertEqual(b, lowerStr, res)
	})
}
