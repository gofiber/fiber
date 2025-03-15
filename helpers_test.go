// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"strings"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Utils_GetOffer(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", getOffer([]byte("hello"), acceptsOffer))
	require.Equal(t, "1", getOffer([]byte(""), acceptsOffer, "1"))
	require.Equal(t, "", getOffer([]byte("2"), acceptsOffer, "1"))

	require.Equal(t, "", getOffer([]byte(""), acceptsOfferType))
	require.Equal(t, "", getOffer([]byte("text/html"), acceptsOfferType))
	require.Equal(t, "", getOffer([]byte("text/html"), acceptsOfferType, "application/json"))
	require.Equal(t, "", getOffer([]byte("text/html;q=0"), acceptsOfferType, "text/html"))
	require.Equal(t, "", getOffer([]byte("application/json, */*; q=0"), acceptsOfferType, "image/png"))
	require.Equal(t, "application/xml", getOffer([]byte("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"), acceptsOfferType, "application/xml", "application/json"))
	require.Equal(t, "text/html", getOffer([]byte("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"), acceptsOfferType, "text/html"))
	require.Equal(t, "application/pdf", getOffer([]byte("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000"), acceptsOfferType, "application/pdf", "application/json"))
	require.Equal(t, "application/pdf", getOffer([]byte("text/plain;q=0,application/pdf;q=0.9,*/*;q=0.000"), acceptsOfferType, "application/pdf", "application/json"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain;a=1"), acceptsOfferType, "text/plain;a=1"))
	require.Equal(t, "", getOffer([]byte("text/plain;a=1;b=2"), acceptsOfferType, "text/plain;b=2"))

	// Spaces, quotes, out of order params, and case insensitivity
	require.Equal(t, "text/plain", getOffer([]byte("text/plain  "), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  "), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  ;"), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.4  ; p=foo"), acceptsOfferType, "text/plain"))
	require.Equal(t, "text/plain;b=2;a=1", getOffer([]byte("text/plain ;a=1;b=2"), acceptsOfferType, "text/plain;b=2;a=1"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain;   a=1   "), acceptsOfferType, "text/plain;a=1"))
	require.Equal(t, `text/plain;a="1;b=2\",text/plain"`, getOffer([]byte(`text/plain;a="1;b=2\",text/plain";q=0.9`), acceptsOfferType, `text/plain;a=1;b=2`, `text/plain;a="1;b=2\",text/plain"`))
	require.Equal(t, "text/plain;A=CAPS", getOffer([]byte(`text/plain;a="caPs"`), acceptsOfferType, "text/plain;A=CAPS"))

	// Priority
	require.Equal(t, "text/plain", getOffer([]byte("text/plain"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain"), acceptsOfferType, "text/plain;a=1", "", "text/plain"))
	require.Equal(t, "text/plain;a=1", getOffer([]byte("text/plain,text/plain;a=1"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain", getOffer([]byte("text/plain;q=0.899,text/plain;a=1;q=0.898"), acceptsOfferType, "text/plain", "text/plain;a=1"))
	require.Equal(t, "text/plain;a=1;b=2", getOffer([]byte("text/plain,text/plain;a=1,text/plain;a=1;b=2"), acceptsOfferType, "text/plain", "text/plain;a=1", "text/plain;a=1;b=2"))

	// Takes the last value specified
	require.Equal(t, "text/plain;a=1;b=2", getOffer([]byte("text/plain;a=1;b=1;B=2"), acceptsOfferType, "text/plain;a=1;b=1", "text/plain;a=1;b=2"))

	require.Equal(t, "", getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer))
	require.Equal(t, "", getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer, "ascii"))
	require.Equal(t, "utf-8", getOffer([]byte("utf-8, iso-8859-1;q=0.5"), acceptsOffer, "utf-8"))
	require.Equal(t, "iso-8859-1", getOffer([]byte("utf-8;q=0, iso-8859-1;q=0.5"), acceptsOffer, "utf-8", "iso-8859-1"))

	require.Equal(t, "deflate", getOffer([]byte("gzip, deflate"), acceptsOffer, "deflate"))
	require.Equal(t, "", getOffer([]byte("gzip, deflate;q=0"), acceptsOffer, "deflate"))
}

// go test -v -run=^$ -bench=Benchmark_Utils_GetOffer -benchmem -count=4
func Benchmark_Utils_GetOffer(b *testing.B) {
	testCases := []struct {
		description string
		accept      string
		offers      []string
	}{
		{
			description: "simple",
			accept:      "application/json",
			offers:      []string{"application/json"},
		},
		{
			description: "6 offers",
			accept:      "text/plain",
			offers:      []string{"junk/a", "junk/b", "junk/c", "junk/d", "junk/e", "text/plain"},
		},
		{
			description: "1 parameter",
			accept:      "application/json; version=1",
			offers:      []string{"application/json;version=1"},
		},
		{
			description: "2 parameters",
			accept:      "application/json; version=1; foo=bar",
			offers:      []string{"application/json;version=1;foo=bar"},
		},
		{
			description: "3 parameters",
			accept:      "application/json; version=1; foo=bar; charset=utf-8",
			offers:      []string{"application/json;version=1;foo=bar;charset=utf-8"},
		},
		{
			description: "10 parameters",
			accept:      "text/plain;a=1;b=2;c=3;d=4;e=5;f=6;g=7;h=8;i=9;j=10",
			offers:      []string{"text/plain;a=1;b=2;c=3;d=4;e=5;f=6;g=7;h=8;i=9;j=10"},
		},
		{
			description: "6 offers w/params",
			accept:      "text/plain; format=flowed",
			offers: []string{
				"junk/a;a=b",
				"junk/b;b=c",
				"junk/c;c=d",
				"text/plain; format=justified",
				"text/plain; format=flat",
				"text/plain; format=flowed",
			},
		},
		{
			description: "mime extension",
			accept:      "utf-8, iso-8859-1;q=0.5",
			offers:      []string{"utf-8"},
		},
		{
			description: "mime extension",
			accept:      "utf-8, iso-8859-1;q=0.5",
			offers:      []string{"iso-8859-1"},
		},
		{
			description: "mime extension",
			accept:      "utf-8, iso-8859-1;q=0.5",
			offers:      []string{"iso-8859-1", "utf-8"},
		},
		{
			description: "mime extension",
			accept:      "gzip, deflate",
			offers:      []string{"deflate"},
		},
		{
			description: "web browser",
			accept:      "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			offers:      []string{"text/html", "application/xml", "application/xml+xhtml"},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for _, tc := range testCases {
		accept := []byte(tc.accept)
		b.Run(tc.description, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				getOffer(accept, acceptsOfferType, tc.offers...)
			}
		})
	}
}

func Test_Utils_ParamsMatch(t *testing.T) {
	testCases := []struct {
		description string
		accept      headerParams
		offer       string
		match       bool
	}{
		{
			description: "empty accept and offer",
			accept:      nil,
			offer:       "",
			match:       true,
		},
		{
			description: "accept is empty, offer has params",
			accept:      make(headerParams),
			offer:       ";foo=bar",
			match:       true,
		},
		{
			description: "offer is empty, accept has params",
			accept:      headerParams{"foo": []byte("bar")},
			offer:       "",
			match:       false,
		},
		{
			description: "accept has extra parameters",
			accept:      headerParams{"foo": []byte("bar"), "a": []byte("1")},
			offer:       ";foo=bar",
			match:       false,
		},
		{
			description: "matches regardless of order",
			accept:      headerParams{"b": []byte("2"), "a": []byte("1")},
			offer:       ";b=2;a=1",
			match:       true,
		},
		{
			description: "case insensitive",
			accept:      headerParams{"ParaM": []byte("FoO")},
			offer:       ";pAram=foO",
			match:       true,
		},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.match, paramsMatch(tc.accept, tc.offer), tc.description)
	}
}

func Benchmark_Utils_ParamsMatch(b *testing.B) {
	var match bool

	specParams := headerParams{
		"appLe": []byte("orange"),
		"param": []byte("foo"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		match = paramsMatch(specParams, `;param=foo; apple=orange`)
	}
	require.True(b, match)
}

func Test_Utils_AcceptsOfferType(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description string
		spec        string
		specParams  headerParams
		offerType   string
		accepts     bool
	}{
		{
			description: "no params, matching",
			spec:        "application/json",
			offerType:   "application/json",
			accepts:     true,
		},
		{
			description: "no params, mismatch",
			spec:        "application/json",
			offerType:   "application/xml",
			accepts:     false,
		},
		{
			description: "params match",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo"), "version": []byte("1")},
			offerType:   "application/json;version=1;format=foo;q=0.1",
			accepts:     true,
		},
		{
			description: "spec has extra params",
			spec:        "text/html",
			specParams:  headerParams{"charset": []byte("utf-8")},
			offerType:   "text/html",
			accepts:     false,
		},
		{
			description: "offer has extra params",
			spec:        "text/html",
			offerType:   "text/html;charset=utf-8",
			accepts:     true,
		},
		{
			description: "ignores optional whitespace",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo"), "version": []byte("1")},
			offerType:   "application/json;  version=1 ;    format=foo   ",
			accepts:     true,
		},
		{
			description: "ignores optional whitespace",
			spec:        "application/json",
			specParams:  headerParams{"format": []byte("foo bar"), "version": []byte("1")},
			offerType:   `application/json;version="1";format="foo bar"`,
			accepts:     true,
		},
	}
	for _, tc := range testCases {
		accepts := acceptsOfferType(tc.spec, tc.offerType, tc.specParams)
		require.Equal(t, tc.accepts, accepts, tc.description)
	}
}

func Test_Utils_GetSplicedStrList(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		description  string
		headerValue  string
		expectedList []string
	}{
		{
			description:  "normal case",
			headerValue:  "gzip, deflate,br",
			expectedList: []string{"gzip", "deflate", "br"},
		},
		{
			description:  "no matter the value",
			headerValue:  "   gzip,deflate, br, zip",
			expectedList: []string{"gzip", "deflate", "br", "zip"},
		},
		{
			description:  "headerValue is empty",
			headerValue:  "",
			expectedList: nil,
		},
		{
			description:  "has a comma without element",
			headerValue:  "gzip,",
			expectedList: []string{"gzip", ""},
		},
		{
			description:  "has a space between words",
			headerValue:  "  foo bar, hello  world",
			expectedList: []string{"foo bar", "hello  world"},
		},
		{
			description:  "single comma",
			headerValue:  ",",
			expectedList: []string{"", ""},
		},
		{
			description:  "multiple comma",
			headerValue:  ",,",
			expectedList: []string{"", "", ""},
		},
		{
			description:  "comma with space",
			headerValue:  ",  ,",
			expectedList: []string{"", "", ""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tc := tc // create a new 'tc' variable for the goroutine
			t.Parallel()
			dst := make([]string, 10)
			result := getSplicedStrList(tc.headerValue, dst)
			require.Equal(t, tc.expectedList, result)
		})
	}
}

func Benchmark_Utils_GetSplicedStrList(b *testing.B) {
	destination := make([]string, 5)
	result := destination
	const input = `deflate, gzip,br,brotli,zstd`
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		result = getSplicedStrList(input, destination)
	}
	require.Equal(b, []string{"deflate", "gzip", "br", "brotli", "zstd"}, result)
}

func Test_Utils_SortAcceptedTypes(t *testing.T) {
	t.Parallel()
	acceptedTypes := []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "application/json", quality: 0.999, specificity: 3, params: headerParams{"a": []byte("1")}, order: 11},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
	}
	sortAcceptedTypes(acceptedTypes)
	require.Equal(t, []acceptedType{
		{spec: "text/html", quality: 1, specificity: 3, order: 0},
		{spec: "application/xml", quality: 1, specificity: 3, order: 4},
		{spec: "application/pdf", quality: 1, specificity: 3, order: 5},
		{spec: "image/png", quality: 1, specificity: 3, order: 6},
		{spec: "image/jpeg", quality: 1, specificity: 3, order: 7},
		{spec: "image/gif", quality: 1, specificity: 3, order: 9},
		{spec: "text/plain", quality: 1, specificity: 3, order: 10},
		{spec: "image/*", quality: 1, specificity: 2, order: 8},
		{spec: "application/json", quality: 0.999, specificity: 3, params: headerParams{"a": []byte("1")}, order: 11},
		{spec: "application/json", quality: 0.999, specificity: 3, order: 3},
		{spec: "text/*", quality: 0.5, specificity: 2, order: 1},
		{spec: "*/*", quality: 0.1, specificity: 1, order: 2},
	}, acceptedTypes)
}

// go test -v -run=^$ -bench=Benchmark_Utils_SortAcceptedTypes_Sorted -benchmem -count=4
func Benchmark_Utils_SortAcceptedTypes_Sorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		acceptedTypes[0] = acceptedType{spec: "text/html", quality: 1, specificity: 1, order: 0}
		acceptedTypes[1] = acceptedType{spec: "text/*", quality: 0.5, specificity: 1, order: 1}
		acceptedTypes[2] = acceptedType{spec: "*/*", quality: 0.1, specificity: 1, order: 2}
		sortAcceptedTypes(acceptedTypes)
	}
	require.Equal(b, "text/html", acceptedTypes[0].spec)
	require.Equal(b, "text/*", acceptedTypes[1].spec)
	require.Equal(b, "*/*", acceptedTypes[2].spec)
}

// go test -v -run=^$ -bench=Benchmark_Utils_SortAcceptedTypes_Unsorted -benchmem -count=4
func Benchmark_Utils_SortAcceptedTypes_Unsorted(b *testing.B) {
	acceptedTypes := make([]acceptedType, 11)
	b.ReportAllocs()
	b.ResetTimer()
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
		sortAcceptedTypes(acceptedTypes)
	}
	require.Equal(b, []acceptedType{
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
	}, acceptedTypes)
}

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
	b.ReportAllocs()
	b.ResetTimer()
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
	b.ReportAllocs()
	b.ResetTimer()
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
		{addr: "[::1]:3000", host: "[::1]", port: "3000"},
		{addr: "127.0.0.1:3000", host: "127.0.0.1", port: "3000"},
		{addr: "/path/to/unix/socket", host: "/path/to/unix/socket", port: ""},
	}

	for _, c := range testCases {
		host, port := parseAddr(c.addr)
		require.Equal(t, c.host, host, "addr host")
		require.Equal(t, c.port, port, "addr port")
	}
}

func Test_Utils_TestConn_Deadline(t *testing.T) {
	t.Parallel()
	conn := &testConn{}
	require.NoError(t, conn.SetDeadline(time.Time{}))
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
	require.NoError(t, conn.SetWriteDeadline(time.Time{}))
}

func Test_Utils_TestConn_ReadWrite(t *testing.T) {
	t.Parallel()
	conn := &testConn{}

	// Verify read of request
	_, err := conn.r.Write([]byte("Request"))
	require.NoError(t, err)

	req := make([]byte, 7)
	_, err = conn.Read(req)
	require.NoError(t, err)
	require.Equal(t, []byte("Request"), req)

	// Verify write of response
	_, err = conn.Write([]byte("Response"))
	require.NoError(t, err)

	res := make([]byte, 8)
	_, err = conn.w.Read(res)
	require.NoError(t, err)
	require.Equal(t, []byte("Response"), res)
}

func Test_Utils_TestConn_Closed_Write(t *testing.T) {
	t.Parallel()
	conn := &testConn{}

	// Verify write of response
	_, err := conn.Write([]byte("Response 1\n"))
	require.NoError(t, err)

	// Close early, write should fail
	conn.Close() //nolint:errcheck,revive // It is fine to ignore the error here
	_, err = conn.Write([]byte("Response 2\n"))
	require.ErrorIs(t, err, errTestConnClosed)

	res := make([]byte, 11)
	_, err = conn.w.Read(res)
	require.NoError(t, err)
	require.Equal(t, []byte("Response 1\n"), res)
}

func Test_Utils_IsNoCache(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		string
		bool
	}{
		{string: "public", bool: false},
		{string: "no-cache", bool: true},
		{string: "public, no-cache, max-age=30", bool: true},
		{string: "public,no-cache", bool: true},
		{string: "public,no-cacheX", bool: false},
		{string: "no-cache, public", bool: true},
		{string: "Xno-cache, public", bool: false},
		{string: "max-age=30, no-cache,public", bool: true},
	}

	for _, c := range testCases {
		ok := isNoCache(c.string)
		require.Equal(t, c.bool, ok, "want %t, got isNoCache(%s)=%t", c.bool, c.string, ok)
	}
}

// go test -v -run=^$ -bench=Benchmark_Utils_IsNoCache -benchmem -count=4
func Benchmark_Utils_IsNoCache(b *testing.B) {
	var ok bool
	b.ReportAllocs()
	b.ResetTimer()
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
		b.ReportAllocs()
		b.ResetTimer()
		result = false
		for i := 0; i < b.N; i++ {
			if strings.IndexByte(search, slashDelimiter) != -1 {
				result = true
			}
		}
		require.True(b, result)
	})
	b.Run("forEach", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
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
	b.Run("strings.ContainsRune", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		result = false
		c := int32(slashDelimiter)
		for i := 0; i < b.N; i++ {
			result = strings.ContainsRune(search, c)
		}
		require.True(b, result)
	})
}
