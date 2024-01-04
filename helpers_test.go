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
	utils.AssertEqual(t, "text/plain;a=1", getOffer("text/plain;a=1", acceptsOfferType, "text/plain;a=1"))
	utils.AssertEqual(t, "", getOffer("text/plain;a=1;b=2", acceptsOfferType, "text/plain;b=2"))

	// Spaces, quotes, out of order params, and case insensitivity
	utils.AssertEqual(t, "text/plain", getOffer("text/plain  ", acceptsOfferType, "text/plain"))
	utils.AssertEqual(t, "text/plain", getOffer("text/plain;q=0.4  ", acceptsOfferType, "text/plain"))
	utils.AssertEqual(t, "text/plain", getOffer("text/plain;q=0.4  ;", acceptsOfferType, "text/plain"))
	utils.AssertEqual(t, "text/plain", getOffer("text/plain;q=0.4  ; p=foo", acceptsOfferType, "text/plain"))
	utils.AssertEqual(t, "text/plain;b=2;a=1", getOffer("text/plain ;a=1;b=2", acceptsOfferType, "text/plain;b=2;a=1"))
	utils.AssertEqual(t, "text/plain;a=1", getOffer("text/plain;   a=1   ", acceptsOfferType, "text/plain;a=1"))
	utils.AssertEqual(t, `text/plain;a="1;b=2\",text/plain"`, getOffer(`text/plain;a="1;b=2\",text/plain";q=0.9`, acceptsOfferType, `text/plain;a=1;b=2`, `text/plain;a="1;b=2\",text/plain"`))
	utils.AssertEqual(t, "text/plain;A=CAPS", getOffer(`text/plain;a="caPs"`, acceptsOfferType, "text/plain;A=CAPS"))

	// Priority
	utils.AssertEqual(t, "text/plain", getOffer("text/plain", acceptsOfferType, "text/plain", "text/plain;a=1"))
	utils.AssertEqual(t, "text/plain;a=1", getOffer("text/plain", acceptsOfferType, "text/plain;a=1", "text/plain"))
	utils.AssertEqual(t, "text/plain;a=1", getOffer("text/plain,text/plain;a=1", acceptsOfferType, "text/plain", "text/plain;a=1"))
	utils.AssertEqual(t, "text/plain", getOffer("text/plain;q=0.899,text/plain;a=1;q=0.898", acceptsOfferType, "text/plain", "text/plain;a=1"))
	utils.AssertEqual(t, "text/plain;a=1;b=2", getOffer("text/plain,text/plain;a=1,text/plain;a=1;b=2", acceptsOfferType, "text/plain", "text/plain;a=1", "text/plain;a=1;b=2"))

	// Takes the last value specified
	utils.AssertEqual(t, "text/plain;a=1;b=2", getOffer("text/plain;a=1;b=1;B=2", acceptsOfferType, "text/plain;a=1;b=1", "text/plain;a=1;b=2"))

	utils.AssertEqual(t, "", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer))
	utils.AssertEqual(t, "", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer, "ascii"))
	utils.AssertEqual(t, "utf-8", getOffer("utf-8, iso-8859-1;q=0.5", acceptsOffer, "utf-8"))
	utils.AssertEqual(t, "iso-8859-1", getOffer("utf-8;q=0, iso-8859-1;q=0.5", acceptsOffer, "utf-8", "iso-8859-1"))

	utils.AssertEqual(t, "deflate", getOffer("gzip, deflate", acceptsOffer, "deflate"))
	utils.AssertEqual(t, "", getOffer("gzip, deflate;q=0", acceptsOffer, "deflate"))
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
			// 1 alloc:
			// The implementation uses a slice of length 2 allocated on the stack,
			// so a third parameters causes a heap allocation.
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

	for _, tc := range testCases {
		b.Run(tc.description, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				getOffer(tc.accept, acceptsOfferType, tc.offers...)
			}
		})
	}
}

func Test_Utils_ForEachParameter(t *testing.T) {
	testCases := []struct {
		description    string
		paramStr       string
		expectedParams [][]string
	}{
		{
			description: "empty input",
			paramStr:    ``,
		},
		{
			description: "no parameters",
			paramStr:    `; `,
		},
		{
			description: "naked equals",
			paramStr:    `; = `,
		},
		{
			description: "no value",
			paramStr:    `;s=`,
		},
		{
			description: "no name",
			paramStr:    `;=bar`,
		},
		{
			description: "illegal characters in name",
			paramStr:    `; foo@bar=baz`,
		},
		{
			description: "value starts with illegal characters",
			paramStr:    `; foo=@baz; param=val`,
		},
		{
			description: "unterminated quoted value",
			paramStr:    `; foo="bar`,
		},
		{
			description: "illegal character after value terminates parsing",
			paramStr:    `; foo=bar@baz; param=val`,
			expectedParams: [][]string{
				{"foo", "bar"},
			},
		},
		{
			description: "parses parameters",
			paramStr:    `; foo=bar; PARAM=BAZ`,
			expectedParams: [][]string{
				{"foo", "bar"},
				{"PARAM", "BAZ"},
			},
		},
		{
			description: "stops parsing when functor returns false",
			paramStr:    `; foo=bar; end=baz; extra=unparsed`,
			expectedParams: [][]string{
				{"foo", "bar"},
				{"end", "baz"},
			},
		},
		{
			description: "stops parsing when encountering a non-parameter string",
			paramStr:    `; foo=bar; gzip; param=baz`,
			expectedParams: [][]string{
				{"foo", "bar"},
			},
		},
		{
			description: "quoted string with escapes and special characters",
			// Note: the sequence \\\" is effectively an escaped backslash \\ and
			// an escaped double quote \"
			paramStr: `;foo="20t\w,b\\\"b;s=k o"`,
			expectedParams: [][]string{
				{"foo", `20t\w,b\\\"b;s=k o`},
			},
		},
		{
			description: "complex",
			paramStr:    `  ;  foo=1  ; bar="\"value\"";  end="20tw,b\\\"b;s=k o" ; action=skip `,
			expectedParams: [][]string{
				{"foo", "1"},
				{"bar", `\"value\"`},
				{"end", `20tw,b\\\"b;s=k o`},
			},
		},
	}
	for _, tc := range testCases {
		n := 0
		forEachParameter(tc.paramStr, func(p, v string) bool {
			utils.AssertEqual(t, true, n < len(tc.expectedParams), "Received more parameters than expected: "+p+"="+v)
			utils.AssertEqual(t, tc.expectedParams[n][0], p, tc.description)
			utils.AssertEqual(t, tc.expectedParams[n][1], v, tc.description)
			n++

			// Stop parsing at the first parameter called "end"
			return p != "end"
		})
		utils.AssertEqual(t, len(tc.expectedParams), n, tc.description+": number of parameters differs")
	}
	// Check that we exited on the second parameter (bar)
}

// go test -v -run=^$ -bench=Benchmark_Utils_ForEachParameter -benchmem -count=4
func Benchmark_Utils_ForEachParameter(b *testing.B) {
	for n := 0; n < b.N; n++ {
		forEachParameter(`  ;  josua=1  ;   vermant="20tw\",bob;sack o" ; version=1; foo=bar;  `, func(s1, s2 string) bool {
			return true
		})
	}
}

func Test_Utils_ParamsMatch(t *testing.T) {
	testCases := []struct {
		description string
		accept      string
		offer       string
		match       bool
	}{
		{
			description: "empty accept and offer",
			accept:      "",
			offer:       "",
			match:       true,
		},
		{
			description: "accept is empty, offer has params",
			accept:      "",
			offer:       ";foo=bar",
			match:       true,
		},
		{
			description: "offer is empty, accept has params",
			accept:      ";foo=bar",
			offer:       "",
			match:       false,
		},
		{
			description: "accept has extra parameters",
			accept:      ";foo=bar;a=1",
			offer:       ";foo=bar",
			match:       false,
		},
		{
			description: "matches regardless of order",
			accept:      "; a=1; b=2",
			offer:       ";b=2;a=1",
			match:       true,
		},
		{
			description: "case insensitive",
			accept:      ";ParaM=FoO",
			offer:       ";pAram=foO",
			match:       true,
		},
		{
			description: "ignores q",
			accept:      ";q=0.42",
			offer:       "",
			match:       true,
		},
	}

	for _, tc := range testCases {
		utils.AssertEqual(t, tc.match, paramsMatch(tc.accept, tc.offer), tc.description)
	}
}

func Benchmark_Utils_ParamsMatch(b *testing.B) {
	var match bool
	for n := 0; n < b.N; n++ {
		match = paramsMatch(`; appLe=orange; param="foo"`, `;param=foo; apple=orange`)
	}
	utils.AssertEqual(b, true, match)
}

func Test_Utils_AcceptsOfferType(t *testing.T) {
	testCases := []struct {
		description string
		spec        string
		specParams  string
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
			specParams:  `; format=foo; version=1`,
			offerType:   "application/json;version=1;format=foo;q=0.1",
			accepts:     true,
		},
		{
			description: "spec has extra params",
			spec:        "text/html",
			specParams:  "; charset=utf-8",
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
			specParams:  `;format=foo; version=1`,
			offerType:   "application/json;  version=1 ;    format=foo   ",
			accepts:     true,
		},
		{
			description: "ignores optional whitespace",
			spec:        "application/json",
			specParams:  `;format="foo bar"; version=1`,
			offerType:   `application/json;version="1";format="foo bar"`,
			accepts:     true,
		},
	}
	for _, tc := range testCases {
		accepts := acceptsOfferType(tc.spec, tc.offerType, tc.specParams)
		utils.AssertEqual(t, tc.accepts, accepts, tc.description)
	}
}

func Test_Utils_GetSplicedStrList(t *testing.T) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			dst := make([]string, 10)
			result := getSplicedStrList(tc.headerValue, dst)
			utils.AssertEqual(t, tc.expectedList, result)
		})
	}
}

func Benchmark_Utils_GetSplicedStrList(b *testing.B) {
	destination := make([]string, 5)
	result := destination
	const input = `deflate, gzip,br,brotli`
	for n := 0; n < b.N; n++ {
		result = getSplicedStrList(input, destination)
	}
	utils.AssertEqual(b, []string{"deflate", "gzip", "br", "brotli"}, result)
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
		{spec: "application/json", quality: 0.999, specificity: 3, params: ";a=1", order: 11},
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
		{spec: "application/json", quality: 0.999, specificity: 3, params: ";a=1", order: 11},
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
