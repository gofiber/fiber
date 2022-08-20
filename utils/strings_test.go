// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ToUpper(t *testing.T) {
	t.Parallel()
	res := ToUpper("/my/name/is/:param/*")
	require.Equal(t, "/MY/NAME/IS/:PARAM/*", res)
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
		require.Equal(b, upperStr, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.ToUpper(largeStr)
		}
		require.Equal(b, upperStr, res)
	})
}

func Test_ToLower(t *testing.T) {
	t.Parallel()
	res := ToLower("/MY/NAME/IS/:PARAM/*")
	require.Equal(t, "/my/name/is/:param/*", res)
	res = ToLower("/MY1/NAME/IS/:PARAM/*")
	require.Equal(t, "/my1/name/is/:param/*", res)
	res = ToLower("/MY2/NAME/IS/:PARAM/*")
	require.Equal(t, "/my2/name/is/:param/*", res)
	res = ToLower("/MY3/NAME/IS/:PARAM/*")
	require.Equal(t, "/my3/name/is/:param/*", res)
	res = ToLower("/MY4/NAME/IS/:PARAM/*")
	require.Equal(t, "/my4/name/is/:param/*", res)
}

func Benchmark_ToLower(b *testing.B) {
	var res string
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToLower(largeStr)
		}
		require.Equal(b, lowerStr, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.ToLower(largeStr)
		}
		require.Equal(b, lowerStr, res)
	})
}

func Test_TrimRight(t *testing.T) {
	t.Parallel()
	res := TrimRight("/test//////", '/')
	require.Equal(t, "/test", res)

	res = TrimRight("/test", '/')
	require.Equal(t, "/test", res)

	res = TrimRight(" ", ' ')
	require.Equal(t, "", res)

	res = TrimRight("  ", ' ')
	require.Equal(t, "", res)

	res = TrimRight("", ' ')
	require.Equal(t, "", res)
}

func Benchmark_TrimRight(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimRight("foobar  ", ' ')
		}
		require.Equal(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimRight("foobar  ", " ")
		}
		require.Equal(b, "foobar", res)
	})
}

func Test_TrimLeft(t *testing.T) {
	t.Parallel()
	res := TrimLeft("////test/", '/')
	require.Equal(t, "test/", res)

	res = TrimLeft("test/", '/')
	require.Equal(t, "test/", res)

	res = TrimLeft(" ", ' ')
	require.Equal(t, "", res)

	res = TrimLeft("  ", ' ')
	require.Equal(t, "", res)

	res = TrimLeft("", ' ')
	require.Equal(t, "", res)
}

func Benchmark_TrimLeft(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimLeft("  foobar", ' ')
		}
		require.Equal(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimLeft("  foobar", " ")
		}
		require.Equal(b, "foobar", res)
	})
}

func Test_Trim(t *testing.T) {
	t.Parallel()
	res := Trim("   test  ", ' ')
	require.Equal(t, "test", res)

	res = Trim("test", ' ')
	require.Equal(t, "test", res)

	res = Trim(".test", '.')
	require.Equal(t, "test", res)

	res = Trim(" ", ' ')
	require.Equal(t, "", res)

	res = Trim("  ", ' ')
	require.Equal(t, "", res)

	res = Trim("", ' ')
	require.Equal(t, "", res)
}

func Benchmark_Trim(b *testing.B) {
	var res string

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = Trim("  foobar   ", ' ')
		}
		require.Equal(b, "foobar", res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.Trim("  foobar   ", " ")
		}
		require.Equal(b, "foobar", res)
	})
	b.Run("default.trimspace", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.TrimSpace("  foobar   ")
		}
		require.Equal(b, "foobar", res)
	})
}

// go test -v -run=^$ -bench=Benchmark_EqualFold -benchmem -count=4
func Benchmark_EqualFold(b *testing.B) {
	var res bool
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = EqualFold(upperStr, lowerStr)
		}
		require.Equal(b, true, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = strings.EqualFold(upperStr, lowerStr)
		}
		require.Equal(b, true, res)
	})
}

func Test_EqualFold(t *testing.T) {
	t.Parallel()
	res := EqualFold("/MY/NAME/IS/:PARAM/*", "/my/name/is/:param/*")
	require.Equal(t, true, res)
	res = EqualFold("/MY1/NAME/IS/:PARAM/*", "/MY1/NAME/IS/:PARAM/*")
	require.Equal(t, true, res)
	res = EqualFold("/my2/name/is/:param/*", "/my2/name")
	require.Equal(t, false, res)
	res = EqualFold("/dddddd", "eeeeee")
	require.Equal(t, false, res)
	res = EqualFold("\na", "*A")
	require.Equal(t, false, res)
	res = EqualFold("/MY3/NAME/IS/:PARAM/*", "/my3/name/is/:param/*")
	require.Equal(t, true, res)
	res = EqualFold("/MY4/NAME/IS/:PARAM/*", "/my4/nAME/IS/:param/*")
	require.Equal(t, true, res)
}
