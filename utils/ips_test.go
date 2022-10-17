// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsIPv4(t *testing.T) {
	t.Parallel()

	require.Equal(t, true, IsIPv4("174.23.33.100"))
	require.Equal(t, true, IsIPv4("127.0.0.1"))
	require.Equal(t, true, IsIPv4("0.0.0.0"))

	require.Equal(t, false, IsIPv4(".0.0.0"))
	require.Equal(t, false, IsIPv4("0.0.0."))
	require.Equal(t, false, IsIPv4("0.0.0"))
	require.Equal(t, false, IsIPv4(".0.0.0."))
	require.Equal(t, false, IsIPv4("0.0.0.0.0"))
	require.Equal(t, false, IsIPv4("0"))
	require.Equal(t, false, IsIPv4(""))
	require.Equal(t, false, IsIPv4("2345:0425:2CA1::0567:5673:23b5"))
	require.Equal(t, false, IsIPv4("invalid"))
}

// go test -v -run=^$ -bench=UnsafeString -benchmem -count=2

func Benchmark_IsIPv4(b *testing.B) {
	ip := "174.23.33.100"
	var res bool

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = IsIPv4(ip)
		}
		require.Equal(b, true, res)
	})

	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = net.ParseIP(ip) != nil
		}
		require.Equal(b, true, res)
	})
}

func Test_IsIPv6(t *testing.T) {
	t.Parallel()

	require.Equal(t, true, IsIPv6("9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"))
	require.Equal(t, true, IsIPv6("2345:0425:2CA1::0567:5673:23b5"))
	require.Equal(t, true, IsIPv6("2001:1:2:3:4:5:6:7"))

	require.Equal(t, false, IsIPv6("1.1.1.1"))
	require.Equal(t, false, IsIPv6("2001:1:2:3:4:5:6:"))
	require.Equal(t, false, IsIPv6(":1:2:3:4:5:6:"))
	require.Equal(t, false, IsIPv6("1:2:3:4:5:6:"))
	require.Equal(t, false, IsIPv6(""))
	require.Equal(t, false, IsIPv6("invalid"))
}

// go test -v -run=^$ -bench=UnsafeString -benchmem -count=2

func Benchmark_IsIPv6(b *testing.B) {
	ip := "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"
	var res bool

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = IsIPv6(ip)
		}
		require.Equal(b, true, res)
	})

	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = net.ParseIP(ip) != nil
		}
		require.Equal(b, true, res)
	})
}
