// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ToLowerBytes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []byte("/my/name/is/:param/*"), ToLowerBytes([]byte("/MY/NAME/IS/:PARAM/*")))
	require.Equal(t, []byte("/my1/name/is/:param/*"), ToLowerBytes([]byte("/MY1/NAME/IS/:PARAM/*")))
	require.Equal(t, []byte("/my2/name/is/:param/*"), ToLowerBytes([]byte("/MY2/NAME/IS/:PARAM/*")))
	require.Equal(t, []byte("/my3/name/is/:param/*"), ToLowerBytes([]byte("/MY3/NAME/IS/:PARAM/*")))
	require.Equal(t, []byte("/my4/name/is/:param/*"), ToLowerBytes([]byte("/MY4/NAME/IS/:PARAM/*")))
}

func Benchmark_ToLowerBytes(b *testing.B) {
	path := []byte(largeStr)
	want := []byte(lowerStr)
	var res []byte
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToLowerBytes(path)
		}
		require.Equal(b, bytes.Equal(want, res), true)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToLower(path)
		}
		require.Equal(b, bytes.Equal(want, res), true)
	})
}

func Test_ToUpperBytes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []byte("/MY/NAME/IS/:PARAM/*"), ToUpperBytes([]byte("/my/name/is/:param/*")))
	require.Equal(t, []byte("/MY1/NAME/IS/:PARAM/*"), ToUpperBytes([]byte("/my1/name/is/:param/*")))
	require.Equal(t, []byte("/MY2/NAME/IS/:PARAM/*"), ToUpperBytes([]byte("/my2/name/is/:param/*")))
	require.Equal(t, []byte("/MY3/NAME/IS/:PARAM/*"), ToUpperBytes([]byte("/my3/name/is/:param/*")))
	require.Equal(t, []byte("/MY4/NAME/IS/:PARAM/*"), ToUpperBytes([]byte("/my4/name/is/:param/*")))
}

func Benchmark_ToUpperBytes(b *testing.B) {
	path := []byte(largeStr)
	want := []byte(upperStr)
	var res []byte
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = ToUpperBytes(path)
		}
		require.Equal(b, want, res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = bytes.ToUpper(path)
		}
		require.Equal(b, want, res)
	})
}
