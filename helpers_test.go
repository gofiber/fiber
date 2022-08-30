// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Utils_UniqueRouteStack(t *testing.T) {
	route1 := &RouteInfo{}
	route2 := &RouteInfo{}
	route3 := &RouteInfo{}
	require.Equal(
		t,
		[]*RouteInfo{
			route1,
			route2,
			route3,
		},
		uniqueRouteStack([]*RouteInfo{
			route1,
			route1,
			route1,
			route2,
			route2,
			route2,
			route3,
			route3,
			route3,
			route1,
			route2,
			route3,
		}))

}

func Test_Utils_getGroupPath(t *testing.T) {
	t.Parallel()
	res := getGroupPath("/v1", "/")
	require.Equal(t, "/v1", res)

	res = getGroupPath("/v1/", "/")
	require.Equal(t, "/v1/", res)

	res = getGroupPath("/v1", "/")
	require.Equal(t, "/v1", res)

	res = getGroupPath("/", "/")
	require.Equal(t, "/", res)

	res = getGroupPath("/v1/api/", "/")
	require.Equal(t, "/v1/api/", res)

	res = getGroupPath("/v1/api", "group")
	require.Equal(t, "/v1/api/group", res)

	res = getGroupPath("/v1/api", "")
	require.Equal(t, "/v1/api", res)
}

// go test -v -run=^$ -bench=Benchmark_Utils_ -benchmem -count=3

func Benchmark_Utils_getGroupPath(b *testing.B) {
	var res string
	for n := 0; n < b.N; n++ {
		_ = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
		_ = getGroupPath("/v1", "/")
		_ = getGroupPath("/v1", "/api")
		res = getGroupPath("/v1", "/api/register/:project")
	}
	require.Equal(b, "/v1/api/register/:project", res)
}

func Benchmark_Utils_Unescape(b *testing.B) {
	unescaped := ""
	dst := make([]byte, 0)

	for n := 0; n < b.N; n++ {
		source := "/cr%C3%A9er"
		pathBytes := utils.UnsafeBytes(source)
		pathBytes = fasthttp.AppendUnquotedArg(dst[:0], pathBytes)
		unescaped = utils.UnsafeString(pathBytes)
	}

	require.Equal(b, "/créer", unescaped)
}

func Test_Utils_Parse_Address(t *testing.T) {
	testCases := []struct {
		addr, host, port string
	}{
		{"[::1]:3000", "[::1]", "3000"},
		{"127.0.0.1:3000", "127.0.0.1", "3000"},
		{"/path/to/unix/socket", "/path/to/unix/socket", ""},
	}

	for _, c := range testCases {
		host, port := parseAddr(c.addr)
		require.Equal(t, c.host, host, "addr host")
		require.Equal(t, c.port, port, "addr port")
	}
}

func Test_Utils_GetOffset(t *testing.T) {
	require.Equal(t, "", getOffer("hello"))
	require.Equal(t, "1", getOffer("", "1"))
	require.Equal(t, "", getOffer("2", "1"))
}

func Test_Utils_TestConn_Deadline(t *testing.T) {
	conn := &testConn{}
	require.Nil(t, conn.SetDeadline(time.Time{}))
	require.Nil(t, conn.SetReadDeadline(time.Time{}))
	require.Nil(t, conn.SetWriteDeadline(time.Time{}))
}

func Test_Utils_IsNoCache(t *testing.T) {
	testCases := []struct {
		string
		bool
	}{
		{"public", false},
		{"no-cache", true},
		{"public, no-cache, max-age=30", true},
		{"public,no-cache", true},
		{"public,no-cacheX", false},
		{"no-cache, public", true},
		{"Xno-cache, public", false},
		{"max-age=30, no-cache,public", true},
	}

	for _, c := range testCases {
		ok := isNoCache(c.string)
		require.Equal(t, c.bool, ok,
			fmt.Sprintf("want %t, got isNoCache(%s)=%t", c.bool, c.string, ok))

	}
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsNoCache -benchmem -count=4
func Benchmark_Utils_IsNoCache(b *testing.B) {
	var ok bool
	for i := 0; i < b.N; i++ {
		_ = isNoCache("public")
		_ = isNoCache("no-cache")
		_ = isNoCache("public, no-cache, max-age=30")
		_ = isNoCache("public,no-cache")
		_ = isNoCache("no-cache, public")
		ok = isNoCache("max-age=30, no-cache,public")
	}
	require.True(b, ok)
}

func Test_Utils_lnMetadata(t *testing.T) {
	t.Run("closed listen", func(t *testing.T) {
		ln, err := net.Listen(NetworkTCP, ":0")
		require.NoError(t, err)

		require.Nil(t, ln.Close())

		addr, config := lnMetadata(NetworkTCP, ln)

		require.Equal(t, ln.Addr().String(), addr)
		require.True(t, config == nil)
	})

	t.Run("non tls", func(t *testing.T) {
		ln, err := net.Listen(NetworkTCP, ":0")

		require.NoError(t, err)

		addr, config := lnMetadata(NetworkTCP4, ln)

		require.Equal(t, ln.Addr().String(), addr)
		require.True(t, config == nil)
	})

	t.Run("tls", func(t *testing.T) {
		cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
		require.NoError(t, err)

		config := &tls.Config{Certificates: []tls.Certificate{cer}}

		ln, err := net.Listen(NetworkTCP4, ":0")
		require.NoError(t, err)

		ln = tls.NewListener(ln, config)

		addr, config := lnMetadata(NetworkTCP4, ln)

		require.Equal(t, ln.Addr().String(), addr)
		require.True(t, config != nil)
	})
}

// go test -v -run=^$ -bench=Benchmark_SlashRecognition -benchmem -count=4
func Benchmark_SlashRecognition(b *testing.B) {
	search := "wtf/1234"
	var result bool
	b.Run("indexBytes", func(b *testing.B) {
		result = false
		for i := 0; i < b.N; i++ {
			if strings.IndexByte(search, slashDelimiter) != -1 {
				result = true
			}
		}
		require.True(b, result)
	})
	b.Run("forEach", func(b *testing.B) {
		result = false
		c := int32(slashDelimiter)
		for i := 0; i < b.N; i++ {
			for _, b := range search {
				if b == c {
					result = true
					break
				}
			}
		}
		require.True(b, result)
	})
	b.Run("IndexRune", func(b *testing.B) {
		result = false
		c := int32(slashDelimiter)
		for i := 0; i < b.N; i++ {
			result = IndexRune(search, c)
		}
		require.True(b, result)
	})
}

func IndexRune(str string, needle int32) bool {
	for _, b := range str {
		if b == needle {
			return true
		}
	}
	return false
}
