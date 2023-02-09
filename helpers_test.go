// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Utils_UniqueRouteStack(t *testing.T) {
	t.Parallel()
	route1 := &Route{}
	route2 := &Route{}
	route3 := &Route{}
	require.Equal(
		t,
		[]*Route{
			route1,
			route2,
			route3,
		},
		uniqueRouteStack([]*Route{
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
	require.Equal(t, "/v1/", res)

	res = getGroupPath("/v1/", "/")
	require.Equal(t, "/v1/", res)

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
	t.Parallel()
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
	t.Parallel()
	conn := &testConn{}
	require.Nil(t, conn.SetDeadline(time.Time{}))
	require.Nil(t, conn.SetReadDeadline(time.Time{}))
	require.Nil(t, conn.SetWriteDeadline(time.Time{}))
}

func Test_Utils_IsNoCache(t *testing.T) {
	t.Parallel()
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
