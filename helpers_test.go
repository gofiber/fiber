// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
)

// go test -v -run=^$ -bench=Benchmark_Utils_RemoveNewLines -benchmem -count=4
func Benchmark_Utils_RemoveNewLines(b *testing.B) {
	withNL := "foo\r\nSet-Cookie:%20SESSIONID=MaliciousValue\r\n"
	withoutNL := "foo  Set-Cookie:%20SESSIONID=MaliciousValue  "
	expected := utils.SafeString(withoutNL)
	var res string

	b.Run("withNewlines", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			res = removeNewLines(withNL)
		}
		utils.AssertEqual(b, expected, res)
	})
	b.Run("withoutNewlines", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			res = removeNewLines(withoutNL)
		}
		utils.AssertEqual(b, expected, res)
	})

}

// go test -v -run=Test_Utils_ -count=3
func Test_Utils_ETag(t *testing.T) {
	app := New()
	t.Run("Not Status OK", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		c.Status(201)
		setETag(c, false)
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("No Body", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		setETag(c, false)
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("Has HeaderIfNoneMatch", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		c.Request().Header.Set(HeaderIfNoneMatch, `"13-1831710635"`)
		setETag(c, false)
		utils.AssertEqual(t, 304, c.Response().StatusCode())
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
		utils.AssertEqual(t, "", string(c.Response().Body()))
	})

	t.Run("No HeaderIfNoneMatch", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		setETag(c, false)
		utils.AssertEqual(t, `"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})
}

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_Utils_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.SendString("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, false)
	}
	utils.AssertEqual(b, `"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

// go test -v -run=Test_Utils_ETag_Weak -count=1
func Test_Utils_ETag_Weak(t *testing.T) {
	app := New()
	t.Run("Set Weak", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		setETag(c, true)
		utils.AssertEqual(t, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("Match Weak ETag", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		c.Request().Header.Set(HeaderIfNoneMatch, `W/"13-1831710635"`)
		setETag(c, true)
		utils.AssertEqual(t, 304, c.Response().StatusCode())
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
		utils.AssertEqual(t, "", string(c.Response().Body()))
	})

	t.Run("Not Match Weak ETag", func(t *testing.T) {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		c.SendString("Hello, World!")
		c.Request().Header.Set(HeaderIfNoneMatch, `W/"13-1831710635xx"`)
		setETag(c, true)
		utils.AssertEqual(t, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})
}

func Test_Utils_UniqueRouteStack(t *testing.T) {
	route1 := &Route{}
	route2 := &Route{}
	route3 := &Route{}
	utils.AssertEqual(
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
		}),
	)
}

// go test -v -run=^$ -bench=Benchmark_App_ETag_Weak -benchmem -count=4
func Benchmark_Utils_ETag_Weak(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.SendString("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

func Test_Utils_getGroupPath(t *testing.T) {
	t.Parallel()
	res := getGroupPath("/v1", "/")
	utils.AssertEqual(t, "/v1", res)

	res = getGroupPath("/v1/", "/")
	utils.AssertEqual(t, "/v1/", res)

	res = getGroupPath("/v1", "/")
	utils.AssertEqual(t, "/v1", res)

	res = getGroupPath("/", "/")
	utils.AssertEqual(t, "/", res)

	res = getGroupPath("/v1/api/", "/")
	utils.AssertEqual(t, "/v1/api/", res)
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
	utils.AssertEqual(b, "/v1/api/register/:project", res)
}

func Benchmark_Utils_Unescape(b *testing.B) {
	unescaped := ""
	dst := make([]byte, 0)

	for n := 0; n < b.N; n++ {
		source := "/cr%C3%A9er"
		pathBytes := getBytes(source)
		pathBytes = fasthttp.AppendUnquotedArg(dst[:0], pathBytes)
		unescaped = getString(pathBytes)
	}

	utils.AssertEqual(b, "/créer", unescaped)
}

func Test_Utils_Parse_Address(t *testing.T) {
	testCases := []struct {
		addr, host, port string
	}{
		{"[::]:3000", "[::]", "3000"},
		{"127.0.0.1:3000", "127.0.0.1", "3000"},
		{"/path/to/unix/socket", "/path/to/unix/socket", ""},
	}

	for _, c := range testCases {
		host, port := parseAddr(c.addr)
		utils.AssertEqual(t, c.host, host, "addr host")
		utils.AssertEqual(t, c.port, port, "addr port")
	}
}

func Test_Utils_GetOffset(t *testing.T) {
	utils.AssertEqual(t, "", getOffer("hello"))
	utils.AssertEqual(t, "1", getOffer("", "1"))
	utils.AssertEqual(t, "", getOffer("2", "1"))
}

func Test_Utils_TestAddr_Network(t *testing.T) {
	var addr testAddr = "addr"
	utils.AssertEqual(t, "addr", addr.Network())
}

func Test_Utils_TestConn_Deadline(t *testing.T) {
	conn := &testConn{}
	utils.AssertEqual(t, nil, conn.SetDeadline(time.Time{}))
	utils.AssertEqual(t, nil, conn.SetReadDeadline(time.Time{}))
	utils.AssertEqual(t, nil, conn.SetWriteDeadline(time.Time{}))
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
		utils.AssertEqual(t, c.bool, ok,
			fmt.Sprintf("want %t, got isNoCache(%s)=%t", c.bool, c.string, ok))
	}
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsNoCache -benchmem -count=4
func Benchmark_Utils_IsNoCache(b *testing.B) {
	var ok bool
	for i := 0; i < b.N; i++ {
		ok = isNoCache("public")
		ok = isNoCache("no-cache")
		ok = isNoCache("public, no-cache, max-age=30")
		ok = isNoCache("public,no-cache")
		ok = isNoCache("no-cache, public")
		ok = isNoCache("max-age=30, no-cache,public")
	}
	utils.AssertEqual(b, true, ok)
}

func Test_Utils_lnMetadata(t *testing.T) {
	t.Run("closed listen", func(t *testing.T) {
		ln, err := net.Listen("tcp", ":0")
		utils.AssertEqual(t, nil, err)

		utils.AssertEqual(t, nil, ln.Close())

		addr, config := lnMetadata(ln)

		utils.AssertEqual(t, ln.Addr().String(), addr)
		utils.AssertEqual(t, true, config == nil)
	})

	t.Run("non tls", func(t *testing.T) {
		ln, err := net.Listen("tcp", ":0")

		utils.AssertEqual(t, nil, err)

		addr, config := lnMetadata(ln)

		utils.AssertEqual(t, ln.Addr().String(), addr)
		utils.AssertEqual(t, true, config == nil)
	})

	t.Run("tls", func(t *testing.T) {
		cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
		utils.AssertEqual(t, nil, err)

		config := &tls.Config{Certificates: []tls.Certificate{cer}}

		ln, err := net.Listen("tcp4", ":0")
		utils.AssertEqual(t, nil, err)

		ln = tls.NewListener(ln, config)

		addr, config := lnMetadata(ln)

		utils.AssertEqual(t, ln.Addr().String(), addr)
		utils.AssertEqual(t, true, config != nil)
	})
}
