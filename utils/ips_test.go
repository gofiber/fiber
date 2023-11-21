// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"net"
	"testing"
)

func Test_IsIPv4(t *testing.T) {
	t.Parallel()

	AssertEqual(t, true, IsIPv4("174.23.33.100"))
	AssertEqual(t, true, IsIPv4("127.0.0.1"))
	AssertEqual(t, true, IsIPv4("127.255.255.255"))
	AssertEqual(t, true, IsIPv4("0.0.0.0"))

	AssertEqual(t, false, IsIPv4(".0.0.0"))
	AssertEqual(t, false, IsIPv4("0.0.0."))
	AssertEqual(t, false, IsIPv4("0.0.0"))
	AssertEqual(t, false, IsIPv4(".0.0.0."))
	AssertEqual(t, false, IsIPv4("0.0.0.0.0"))
	AssertEqual(t, false, IsIPv4("0"))
	AssertEqual(t, false, IsIPv4(""))
	AssertEqual(t, false, IsIPv4("2345:0425:2CA1::0567:5673:23b5"))
	AssertEqual(t, false, IsIPv4("invalid"))
	AssertEqual(t, false, IsIPv4("189.12.34.260"))
	AssertEqual(t, false, IsIPv4("189.12.260.260"))
	AssertEqual(t, false, IsIPv4("189.260.260.260"))
	AssertEqual(t, false, IsIPv4("999.999.999.999"))
	AssertEqual(t, false, IsIPv4("9999.9999.9999.9999"))
}

// go test -v -run=^$ -bench=UnsafeString -benchmem -count=2

func Benchmark_IsIPv4(b *testing.B) {
	ip := "174.23.33.100"
	var res bool

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = IsIPv4(ip)
		}
		AssertEqual(b, true, res)
	})

	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = net.ParseIP(ip) != nil
		}
		AssertEqual(b, true, res)
	})
}

func Test_IsIPv6(t *testing.T) {
	t.Parallel()

	AssertEqual(t, true, IsIPv6("9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"))
	AssertEqual(t, true, IsIPv6("2345:0425:2CA1::0567:5673:23b5"))
	AssertEqual(t, true, IsIPv6("2001:1:2:3:4:5:6:7"))

	AssertEqual(t, false, IsIPv6("1.1.1.1"))
	AssertEqual(t, false, IsIPv6("2001:1:2:3:4:5:6:"))
	AssertEqual(t, false, IsIPv6(":1:2:3:4:5:6:"))
	AssertEqual(t, false, IsIPv6("1:2:3:4:5:6:"))
	AssertEqual(t, false, IsIPv6(""))
	AssertEqual(t, false, IsIPv6("invalid"))
}

// go test -v -run=^$ -bench=UnsafeString -benchmem -count=2

func Benchmark_IsIPv6(b *testing.B) {
	ip := "9396:9549:b4f7:8ed0:4791:1330:8c06:e62d"
	var res bool

	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = IsIPv6(ip)
		}
		AssertEqual(b, true, res)
	})

	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = net.ParseIP(ip) != nil
		}
		AssertEqual(b, true, res)
	})
}
