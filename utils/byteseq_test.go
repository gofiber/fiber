package utils

import (
	"bytes"
	"strings"
	"testing"
)

func Test_TrimRightBytes(t *testing.T) {
	t.Parallel()
	res := TrimRight([]byte("/test//////"), '/')
	AssertEqual(t, []byte("/test"), res)

	res = TrimRight([]byte("/test"), '/')
	AssertEqual(t, []byte("/test"), res)

	res = TrimRight([]byte(" "), ' ')
	AssertEqual(t, 0, len(res))

	res = TrimRight([]byte("  "), ' ')
	AssertEqual(t, 0, len(res))

	res = TrimRight([]byte(""), ' ')
	AssertEqual(t, 0, len(res))
}

func Benchmark_TrimRightBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimRight([]byte("foobar  "), ' ')
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

func Test_TrimLeftBytes(t *testing.T) {
	t.Parallel()
	res := TrimLeft([]byte("////test/"), '/')
	AssertEqual(t, []byte("test/"), res)

	res = TrimLeft([]byte("test/"), '/')
	AssertEqual(t, []byte("test/"), res)

	res = TrimLeft([]byte(" "), ' ')
	AssertEqual(t, 0, len(res))

	res = TrimLeft([]byte("  "), ' ')
	AssertEqual(t, 0, len(res))

	res = TrimLeft([]byte(""), ' ')
	AssertEqual(t, 0, len(res))
}

func Benchmark_TrimLeftBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = TrimLeft([]byte("  foobar"), ' ')
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

func Test_TrimBytes(t *testing.T) {
	t.Parallel()
	res := Trim([]byte("   test  "), ' ')
	AssertEqual(t, []byte("test"), res)

	res = Trim([]byte("test"), ' ')
	AssertEqual(t, []byte("test"), res)

	res = Trim([]byte(".test"), '.')
	AssertEqual(t, []byte("test"), res)

	res = Trim([]byte(" "), ' ')
	AssertEqual(t, 0, len(res))

	res = Trim([]byte("  "), ' ')
	AssertEqual(t, 0, len(res))

	res = Trim([]byte(""), ' ')
	AssertEqual(t, 0, len(res))
}

func Benchmark_TrimBytes(b *testing.B) {
	var res []byte

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = Trim([]byte("  foobar   "), ' ')
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

func Benchmark_EqualFoldBytes(b *testing.B) {
	left := []byte(upperStr)
	right := []byte(lowerStr)
	var res bool
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = EqualFold(left, right)
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

func Test_EqualFoldBytes(t *testing.T) {
	t.Parallel()
	res := EqualFold([]byte("/MY/NAME/IS/:PARAM/*"), []byte("/my/name/is/:param/*"))
	AssertEqual(t, true, res)
	res = EqualFold([]byte("/MY1/NAME/IS/:PARAM/*"), []byte("/MY1/NAME/IS/:PARAM/*"))
	AssertEqual(t, true, res)
	res = EqualFold([]byte("/my2/name/is/:param/*"), []byte("/my2/name"))
	AssertEqual(t, false, res)
	res = EqualFold([]byte("/dddddd"), []byte("eeeeee"))
	AssertEqual(t, false, res)
	res = EqualFold([]byte("\na"), []byte("*A"))
	AssertEqual(t, false, res)
	res = EqualFold([]byte("/MY3/NAME/IS/:PARAM/*"), []byte("/my3/name/is/:param/*"))
	AssertEqual(t, true, res)
	res = EqualFold([]byte("/MY4/NAME/IS/:PARAM/*"), []byte("/my4/nAME/IS/:param/*"))
	AssertEqual(t, true, res)
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
		{Expected: true, S1: "/MY1/NAME/IS/:PARAM/*", S2: "/MY1/NAME/IS/:PARAM/*"},
		{Expected: false, S1: "/my2/name/is/:param/*", S2: "/my2/name"},
		{Expected: false, S1: "/dddddd", S2: "eeeeee"},
		{Expected: false, S1: "\na", S2: "*A"},
		{Expected: true, S1: "/MY3/NAME/IS/:PARAM/*", S2: "/my3/name/is/:param/*"},
		{Expected: true, S1: "/MY4/NAME/IS/:PARAM/*", S2: "/my4/nAME/IS/:param/*"},
	}

	for _, tc := range testCases {
		res := EqualFold[string](tc.S1, tc.S2)
		AssertEqual(t, tc.Expected, res, "string")

		res = EqualFold[[]byte]([]byte(tc.S1), []byte(tc.S2))
		AssertEqual(t, tc.Expected, res, "bytes")
	}
}
