// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"testing"
)

func Test_Utils_ToLowerBytes(t *testing.T) {
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
	var path = []byte("/RePos/GoFiBer/FibEr/iSsues/187643/CoMmEnts")
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToLowerBytes(path)
		}
		AssertEqual(b, bytes.Equal(GetBytes("/repos/gofiber/fiber/issues/187643/comments"), res), true)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToLower(path)
		}
		AssertEqual(b, bytes.Equal(GetBytes("/repos/gofiber/fiber/issues/187643/comments"), res), true)
	})
}

func Test_Utils_ToUpperBytes(t *testing.T) {
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
	var path = []byte("/RePos/GoFiBer/FibEr/iSsues/187643/CoMmEnts")
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToUpperBytes(path)
		}
		AssertEqual(b, bytes.Equal(GetBytes("/REPOS/GOFIBER/FIBER/ISSUES/187643/COMMENTS"), res), true)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToUpper(path)
		}
		AssertEqual(b, bytes.Equal(GetBytes("/REPOS/GOFIBER/FIBER/ISSUES/187643/COMMENTS"), res), true)
	})
}

func Test_Utils_TrimRightBytes(t *testing.T) {
	t.Parallel()
	res := TrimRightBytes([]byte("/test//////"), '/')
	AssertEqual(t, []byte("/test"), res)

	res = TrimRightBytes([]byte("/test"), '/')
	AssertEqual(t, []byte("/test"), res)
}

func Benchmark_TrimRightBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimRightBytes([]byte("foobar  "), ' ')
		}
		AssertEqual(b, []byte("foobar"), res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.TrimRight([]byte("foobar  "), " ")
		}
		AssertEqual(b, []byte("foobar"), res)
	})
}

func Test_Utils_TrimLeftBytes(t *testing.T) {
	t.Parallel()
	res := TrimLeftBytes([]byte("////test/"), '/')
	AssertEqual(t, []byte("test/"), res)

	res = TrimLeftBytes([]byte("test/"), '/')
	AssertEqual(t, []byte("test/"), res)
}
func Benchmark_TrimLeftBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimLeftBytes([]byte("  foobar"), ' ')
		}
		AssertEqual(b, []byte("foobar"), res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.TrimLeft([]byte("  foobar"), " ")
		}
		AssertEqual(b, []byte("foobar"), res)
	})
}
func Test_Utils_TrimBytes(t *testing.T) {
	t.Parallel()
	res := TrimBytes([]byte("   test  "), ' ')
	AssertEqual(t, []byte("test"), res)

	res = TrimBytes([]byte("test"), ' ')
	AssertEqual(t, []byte("test"), res)

	res = TrimBytes([]byte(".test"), '.')
	AssertEqual(t, []byte("test"), res)
}
func Benchmark_TrimBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimBytes([]byte("  foobar   "), ' ')
		}
		AssertEqual(b, []byte("foobar"), res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.Trim([]byte("  foobar   "), " ")
		}
		AssertEqual(b, []byte("foobar"), res)
	})
}

func Benchmark_EqualFolds(b *testing.B) {
	var left = []byte("/RePos/GoFiBer/FibEr/iSsues/187643/CoMmEnts")
	var right = []byte("/RePos/goFiber/Fiber/issues/187643/COMMENTS")
	var res bool

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = EqualsFold(left, right)
		}
		AssertEqual(b, true, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.EqualFold(left, right)
		}
		AssertEqual(b, true, res)
	})
}

func Test_Utils_EqualsFold(t *testing.T) {
	t.Parallel()
	res := EqualsFold([]byte("/MY/NAME/IS/:PARAM/*"), []byte("/my/name/is/:param/*"))
	AssertEqual(t, true, res)
	res = EqualsFold([]byte("/MY1/NAME/IS/:PARAM/*"), []byte("/MY1/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, res)
	res = EqualsFold([]byte("/my2/name/is/:param/*"), []byte("/my2/name"))
	AssertEqual(t, false, res)
	res = EqualsFold([]byte("/dddddd"), []byte("eeeeee"))
	AssertEqual(t, false, res)
	res = EqualsFold([]byte("/MY3/NAME/IS/:PARAM/*"), []byte("/my3/name/is/:param/*"))
	AssertEqual(t, true, res)
	res = EqualsFold([]byte("/MY4/NAME/IS/:PARAM/*"), []byte("/my4/nAME/IS/:param/*"))
	AssertEqual(t, true, res)
}
