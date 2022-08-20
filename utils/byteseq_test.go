package utils

import (
	"bytes"
	"strings"
	"testing"
)

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
