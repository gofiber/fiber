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
	res := ToUpper("/my/name/is/:param/*")
	AssertEqual(t, "/MY/NAME/IS/:PARAM/*", res)
}

const (
	largeStr = "/RePos/GoFiBer/FibEr/iSsues/187643/CoMmEnts/RePos/GoFiBer/FibEr/iSsues/CoMmEnts"
	upperStr = "/REPOS/GOFIBER/FIBER/ISSUES/187643/COMMENTS/REPOS/GOFIBER/FIBER/ISSUES/COMMENTS"
	lowerStr = "/repos/gofiber/fiber/issues/187643/comments/repos/gofiber/fiber/issues/comments"
)

var (
	largeBytes = []byte(largeStr)
	upperBytes = []byte(upperStr)
	lowerBytes = []byte(lowerStr)
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
	res := ToLower("/MY/NAME/IS/:PARAM/*")
	AssertEqual(t, "/my/name/is/:param/*", res)
	res = ToLower("/MY1/NAME/IS/:PARAM/*")
	AssertEqual(t, "/my1/name/is/:param/*", res)
	res = ToLower("/MY2/NAME/IS/:PARAM/*")
	AssertEqual(t, "/my2/name/is/:param/*", res)
	res = ToLower("/MY3/NAME/IS/:PARAM/*")
	AssertEqual(t, "/my3/name/is/:param/*", res)
	res = ToLower("/MY4/NAME/IS/:PARAM/*")
	AssertEqual(t, "/my4/name/is/:param/*", res)
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

func Test_TrimRight(t *testing.T) {
	t.Parallel()
	res := TrimRight("/test//////", '/')
	AssertEqual(t, "/test", res)

	res = TrimRight("/test", '/')
	AssertEqual(t, "/test", res)

	res = TrimRight(" ", ' ')
	AssertEqual(t, "", res)

	res = TrimRight("  ", ' ')
	AssertEqual(t, "", res)

	res = TrimRight("", ' ')
	AssertEqual(t, "", res)
}

func Benchmark_TrimRight(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimRight("foobar  ", ' ')
		}
		AssertEqual(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimRight("foobar  ", " ")
		}
		AssertEqual(b, "foobar", res)
	})
}

func Test_TrimLeft(t *testing.T) {
	t.Parallel()
	res := TrimLeft("////test/", '/')
	AssertEqual(t, "test/", res)

	res = TrimLeft("test/", '/')
	AssertEqual(t, "test/", res)

	res = TrimLeft(" ", ' ')
	AssertEqual(t, "", res)

	res = TrimLeft("  ", ' ')
	AssertEqual(t, "", res)

	res = TrimLeft("", ' ')
	AssertEqual(t, "", res)
}

func Benchmark_TrimLeft(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimLeft("  foobar", ' ')
		}
		AssertEqual(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimLeft("  foobar", " ")
		}
		AssertEqual(b, "foobar", res)
	})
}

func Test_Trim(t *testing.T) {
	t.Parallel()
	res := Trim("   test  ", ' ')
	AssertEqual(t, "test", res)

	res = Trim("test", ' ')
	AssertEqual(t, "test", res)

	res = Trim(".test", '.')
	AssertEqual(t, "test", res)

	res = Trim(" ", ' ')
	AssertEqual(t, "", res)

	res = Trim("  ", ' ')
	AssertEqual(t, "", res)

	res = Trim("", ' ')
	AssertEqual(t, "", res)
}

func Benchmark_Trim(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = Trim("  foobar   ", ' ')
		}
		AssertEqual(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.Trim("  foobar   ", " ")
		}
		AssertEqual(b, "foobar", res)
	})
	b.Run("default.trimspace", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimSpace("  foobar   ")
		}
		AssertEqual(b, "foobar", res)
	})
}

// go test -v -run=^$ -bench=Benchmark_EqualFold -benchmem -count=4 ./utils/
func Benchmark_EqualFold(b *testing.B) {
	var res bool
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = EqualFold(upperStr, lowerStr)
		}
		AssertEqual(b, true, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.EqualFold(upperStr, lowerStr)
		}
		AssertEqual(b, true, res)
	})
}

func Test_EqualFold(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		Expected bool
		S1       string
		S2       string
	}{
		{Expected: true, S1: "/MY/NAME/IS/:PARAM/*", S2: "/my/name/is/:param/*"},
		{Expected: true, S1: "/MY/NAME/IS/:PARAM/*", S2: "/my/name/is/:param/*"},
		{true, "/MY1/NAME/IS/:PARAM/*", "/MY1/NAME/IS/:PARAM/*"},
		{false, "/my2/name/is/:param/*", "/my2/name"},
		{false, "/dddddd", "eeeeee"},
		{false, "\na", "*A"},
		{true, "/MY3/NAME/IS/:PARAM/*", "/my3/name/is/:param/*"},
		{true, "/MY4/NAME/IS/:PARAM/*", "/my4/nAME/IS/:param/*"},
	}

	for _, tc := range testCases {
		res := EqualFold[string](tc.S1, tc.S2)
		AssertEqual(t, tc.Expected, res, "string")

		res = EqualFold[[]byte]([]byte(tc.S1), []byte(tc.S2))
		AssertEqual(t, tc.Expected, res, "bytes")
	}
}
