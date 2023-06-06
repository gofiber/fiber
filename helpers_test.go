// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
)

// go test -v -run=Test_Utils_ -count=3
func Test_Utils_ETag(t *testing.T) {
	t.Parallel()
	app := New()
	t.Run("Not Status OK", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		c.Status(201)
		setETag(c, false)
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("No Body", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		setETag(c, false)
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("Has HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		c.Request().Header.Set(HeaderIfNoneMatch, `"13-1831710635"`)
		setETag(c, false)
		utils.AssertEqual(t, 304, c.Response().StatusCode())
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
		utils.AssertEqual(t, "", string(c.Response().Body()))
	})

	t.Run("No HeaderIfNoneMatch", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		setETag(c, false)
		utils.AssertEqual(t, `"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})
}

func Test_Utils_GetOffer(t *testing.T) {
	t.Parallel()
	utils.AssertEqual(t, "", getOffer("hello", acceptsOffer))
	utils.AssertEqual(t, "1", getOffer("", acceptsOffer, "1"))
	utils.AssertEqual(t, "", getOffer("2", acceptsOffer, "1"))

	utils.AssertEqual(t, "", getOffer("", acceptsOfferType))
	utils.AssertEqual(t, "", getOffer("text/html", acceptsOfferType))
	utils.AssertEqual(t, "", getOffer("text/html", acceptsOfferType, "application/json"))
	utils.AssertEqual(t, "", getOffer("text/html;q=0", acceptsOfferType, "text/html"))
	utils.AssertEqual(t, "", getOffer("application/json, */*; q=0", acceptsOfferType, "image/png"))
	utils.AssertEqual(t, "application/xml", getOffer("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", acceptsOfferType, "application/xml", "application/json"))
	utils.AssertEqual(t, "text/html", getOffer("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", acceptsOfferType, "text/html"))
	utils.AssertEqual(t, "application/pdf", getOffer("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000", acceptsOfferType, "application/pdf", "application/json"))
	utils.AssertEqual(t, "application/pdf", getOffer("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000", acceptsOfferType, "application/pdf", "application/json"))

	utils.AssertEqual(t, "", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer))
	utils.AssertEqual(t, "", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer, "ascii"))
	utils.AssertEqual(t, "utf-8", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer, "utf-8"))
	utils.AssertEqual(t, "iso-8859-1", getOffer("utf-8;q=0, iso-8859-1;q=0.5", acceptsOffer, "utf-8", "iso-8859-1"))

	utils.AssertEqual(t, "deflate", getOffer("gzip, deflate", acceptsOffer, "deflate"))
	utils.AssertEqual(t, "", getOffer("gzip, deflate;q=0", acceptsOffer, "deflate"))
}

func Benchmark_Utils_GetOffer(b *testing.B) {
	headers := []string{
		"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"application/json",
		"utf-8, iso-8859-1;q=0.5",
		"gzip, deflate",
	}
	offers := [][]string{
		{"text/html", "application/xml", "application/xml+xhtml"},
		{"application/json"},
		{"utf-8"},
		{"deflate"},
	}
	for n := 0; n < b.N; n++ {
		for i, header := range headers {
			getOffer(header, acceptsOfferType, offers[i]...)
		}
	}
}

func Test_Utils_SortAcceptedTypes(t *testing.T) {
	t.Parallel()
	acceptedTypes := []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
	}
	sortAcceptedTypes(&acceptedTypes)
	utils.AssertEqual(t, acceptedTypes, []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
	})
}

// go test -v -run=^$ -bench=Benchmark_Utils_SortAcceptedTypes_Sorted -benchmem -count=4
func Benchmark_Utils_SortAcceptedTypes_Sorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 3)
	for n := 0; n < b.N; n++ {
		acceptedTypes[0] = acceptedType{spec: "text/html", quality: 1, specificity: 1, order: 0}
		acceptedTypes[1] = acceptedType{spec: "text/*", quality: 0.5, specificity: 1, order: 1}
		acceptedTypes[2] = acceptedType{spec: "*/*", quality: 0.1, specificity: 1, order: 2}
		sortAcceptedTypes(&acceptedTypes)
	}
	utils.AssertEqual(b, "text/html", acceptedTypes[0].spec)
	utils.AssertEqual(b, "text/*", acceptedTypes[1].spec)
	utils.AssertEqual(b, "*/*", acceptedTypes[2].spec)
}

// go test -v -run=^$ -bench=Benchmark_Utils_SortAcceptedTypes_Unsorted -benchmem -count=4
func Benchmark_Utils_SortAcceptedTypes_Unsorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 11)
	for n := 0; n < b.N; n++ {
		acceptedTypes[0] = acceptedType{spec: "text/html", quality: 1, specificity: 3, order: 0}
		acceptedTypes[1] = acceptedType{spec: "text/*", quality: 0.5, specificity: 2, order: 1}
		acceptedTypes[2] = acceptedType{spec: "*/*", quality: 0.1, specificity: 1, order: 2}
		acceptedTypes[3] = acceptedType{spec: "application/json", quality: 0.999, specificity: 3, order: 3}
		acceptedTypes[4] = acceptedType{spec: "application/xml", quality: 1, specificity: 3, order: 4}
		acceptedTypes[5] = acceptedType{spec: "application/pdf", quality: 1, specificity: 3, order: 5}
		acceptedTypes[6] = acceptedType{spec: "image/png", quality: 1, specificity: 3, order: 6}
		acceptedTypes[7] = acceptedType{spec: "image/jpeg", quality: 1, specificity: 3, order: 7}
		acceptedTypes[8] = acceptedType{spec: "image/*", quality: 1, specificity: 2, order: 8}
		acceptedTypes[9] = acceptedType{spec: "image/gif", quality: 1, specificity: 3, order: 9}
		acceptedTypes[10] = acceptedType{spec: "text/plain", quality: 1, specificity: 3, order: 10}
		sortAcceptedTypes(&acceptedTypes)
	}
	utils.AssertEqual(b, acceptedTypes, []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
	})
}

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_Utils_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.SendString("Hello, World!")
	utils.AssertEqual(b, nil, err)
	for n := 0; n < b.N; n++ {
		setETag(c, false)
	}
	utils.AssertEqual(b, `"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

// go test -v -run=Test_Utils_ETag_Weak -count=1
func Test_Utils_ETag_Weak(t *testing.T) {
	t.Parallel()
	app := New()
	t.Run("Set Weak", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		setETag(c, true)
		utils.AssertEqual(t, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})

	t.Run("Match Weak ETag", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		c.Request().Header.Set(HeaderIfNoneMatch, `W/"13-1831710635"`)
		setETag(c, true)
		utils.AssertEqual(t, 304, c.Response().StatusCode())
		utils.AssertEqual(t, "", string(c.Response().Header.Peek(HeaderETag)))
		utils.AssertEqual(t, "", string(c.Response().Body()))
	})

	t.Run("Not Match Weak ETag", func(t *testing.T) {
		t.Parallel()
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(c)
		err := c.SendString("Hello, World!")
		utils.AssertEqual(t, nil, err)
		c.Request().Header.Set(HeaderIfNoneMatch, `W/"13-1831710635xx"`)
		setETag(c, true)
		utils.AssertEqual(t, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
	})
}

func Test_Utils_UniqueRouteStack(t *testing.T) {
	t.Parallel()
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
	err := c.SendString("Hello, World!")
	utils.AssertEqual(b, nil, err)
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

func Test_Utils_getGroupPath(t *testing.T) {
	t.Parallel()
	res := getGroupPath("/v1", "/")
	utils.AssertEqual(t, "/v1/", res)

	res = getGroupPath("/v1/", "/")
	utils.AssertEqual(t, "/v1/", res)

	res = getGroupPath("/", "/")
	utils.AssertEqual(t, "/", res)

	res = getGroupPath("/v1/api/", "/")
	utils.AssertEqual(t, "/v1/api/", res)

	res = getGroupPath("/v1/api", "group")
	utils.AssertEqual(t, "/v1/api/group", res)

	res = getGroupPath("/v1/api", "")
	utils.AssertEqual(t, "/v1/api", res)
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
		pathBytes := utils.UnsafeBytes(source)
		pathBytes = fasthttp.AppendUnquotedArg(dst[:0], pathBytes)
		unescaped = utils.UnsafeString(pathBytes)
	}

	utils.AssertEqual(b, "/créer", unescaped)
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
		utils.AssertEqual(t, c.host, host, "addr host")
		utils.AssertEqual(t, c.port, port, "addr port")
	}
}

func Test_Utils_TestConn_Deadline(t *testing.T) {
	t.Parallel()
	conn := &testConn{}
	utils.AssertEqual(t, nil, conn.SetDeadline(time.Time{}))
	utils.AssertEqual(t, nil, conn.SetReadDeadline(time.Time{}))
	utils.AssertEqual(t, nil, conn.SetWriteDeadline(time.Time{}))
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
		utils.AssertEqual(t, c.bool, ok,
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
	utils.AssertEqual(b, true, ok)
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
		utils.AssertEqual(b, true, result)
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
		utils.AssertEqual(b, true, result)
	})
	b.Run("IndexRune", func(b *testing.B) {
		result = false
		c := int32(slashDelimiter)
		for i := 0; i < b.N; i++ {
			result = IndexRune(search, c)
		}
		utils.AssertEqual(b, true, result)
	})
}
