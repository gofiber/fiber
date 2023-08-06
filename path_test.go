// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

// go test -race -run Test_Path_parseRoute
func Test_Path_parseRoute(t *testing.T) {
	t.Parallel()
	var rp routeParser

	rp = parseRoute("/shop/product/::filter/color::color/size::size")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/shop/product/:", Length: 15},
			{IsParam: true, ParamName: "filter", ComparePart: "/color:", PartCount: 1},
			{Const: "/color:", Length: 7},
			{IsParam: true, ParamName: "color", ComparePart: "/size:", PartCount: 1},
			{Const: "/size:", Length: 6},
			{IsParam: true, ParamName: "size", IsLast: true},
		},
		params: []string{"filter", "color", "size"},
	}, rp)

	rp = parseRoute("/api/v1/:param/abc/*")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/api/v1/", Length: 8},
			{IsParam: true, ParamName: "param", ComparePart: "/abc", PartCount: 1},
			{Const: "/abc/", Length: 5, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, IsLast: true},
		},
		params:        []string{"param", "*1"},
		wildCardCount: 1,
	}, rp)

	rp = parseRoute("/v1/some/resource/name\\:customVerb")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/name:customVerb", Length: 33, IsLast: true},
		},
		params: nil,
	}, rp)

	rp = parseRoute("/v1/some/resource/:name\\:customVerb")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/", Length: 18},
			{IsParam: true, ParamName: "name", ComparePart: ":customVerb", PartCount: 1},
			{Const: ":customVerb", Length: 11, IsLast: true},
		},
		params: []string{"name"},
	}, rp)

	// heavy test with escaped charaters
	rp = parseRoute("/v1/some/resource/name\\\\:customVerb?\\?/:param/*")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/name:customVerb??/", Length: 36},
			{IsParam: true, ParamName: "param", ComparePart: "/", PartCount: 1},
			{Const: "/", Length: 1, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, IsLast: true},
		},
		params:        []string{"param", "*1"},
		wildCardCount: 1,
	}, rp)

	rp = parseRoute("/api/*/:param/:param2")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/api/", Length: 5, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, ComparePart: "/", PartCount: 2},
			{Const: "/", Length: 1},
			{IsParam: true, ParamName: "param", ComparePart: "/", PartCount: 1},
			{Const: "/", Length: 1},
			{IsParam: true, ParamName: "param2", IsLast: true},
		},
		params:        []string{"*1", "param", "param2"},
		wildCardCount: 1,
	}, rp)

	rp = parseRoute("/test:optional?:optional2?")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/test", Length: 5},
			{IsParam: true, ParamName: "optional", IsOptional: true, Length: 1},
			{IsParam: true, ParamName: "optional2", IsOptional: true, IsLast: true},
		},
		params: []string{"optional", "optional2"},
	}, rp)

	rp = parseRoute("/config/+.json")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/config/", Length: 8},
			{IsParam: true, ParamName: "+1", IsGreedy: true, IsOptional: false, ComparePart: ".json", PartCount: 1},
			{Const: ".json", Length: 5, IsLast: true},
		},
		params:    []string{"+1"},
		plusCount: 1,
	}, rp)

	rp = parseRoute("/api/:day.:month?.:year?")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/api/", Length: 5},
			{IsParam: true, ParamName: "day", IsOptional: false, ComparePart: ".", PartCount: 2},
			{Const: ".", Length: 1},
			{IsParam: true, ParamName: "month", IsOptional: true, ComparePart: ".", PartCount: 1},
			{Const: ".", Length: 1},
			{IsParam: true, ParamName: "year", IsOptional: true, IsLast: true},
		},
		params: []string{"day", "month", "year"},
	}, rp)

	rp = parseRoute("/*v1*/proxy")
	utils.AssertEqual(t, routeParser{
		segs: []*routeSegment{
			{Const: "/", Length: 1, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, ComparePart: "v1", PartCount: 1},
			{Const: "v1", Length: 2},
			{IsParam: true, ParamName: "*2", IsGreedy: true, IsOptional: true, ComparePart: "/proxy", PartCount: 1},
			{Const: "/proxy", Length: 6, IsLast: true},
		},
		params:        []string{"*1", "*2"},
		wildCardCount: 2,
	}, rp)
}

// go test -race -run Test_Path_matchParams
func Test_Path_matchParams(t *testing.T) {
	t.Parallel()
	var ctxParams [maxParams]string
	testCaseFn := func(testCollection routeCaseCollection) {
		parser := parseRoute(testCollection.pattern)
		for _, c := range testCollection.testCases {
			match := parser.getMatch(c.url, c.url, &ctxParams, c.partialCheck)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", testCollection.pattern, c.url))
			if match && len(c.params) > 0 {
				utils.AssertEqual(t, c.params[0:len(c.params)], ctxParams[0:len(c.params)], fmt.Sprintf("route: '%s', url: '%s'", testCollection.pattern, c.url))
			}
		}
	}
	for _, testCaseCollection := range routeTestCases {
		testCaseFn(testCaseCollection)
	}
}

// go test -race -run Test_RoutePatternMatch
func Test_RoutePatternMatch(t *testing.T) {
	t.Parallel()
	testCaseFn := func(pattern string, cases []routeTestCase) {
		for _, c := range cases {
			// skip all cases for partial checks
			if c.partialCheck {
				continue
			}
			match := RoutePatternMatch(c.url, pattern)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", pattern, c.url))
		}
	}
	for _, testCase := range routeTestCases {
		testCaseFn(testCase.pattern, testCase.testCases)
	}
}

func Test_Utils_GetTrimmedParam(t *testing.T) {
	t.Parallel()
	res := GetTrimmedParam("")
	utils.AssertEqual(t, "", res)
	res = GetTrimmedParam("*")
	utils.AssertEqual(t, "*", res)
	res = GetTrimmedParam(":param")
	utils.AssertEqual(t, "param", res)
	res = GetTrimmedParam(":param1?")
	utils.AssertEqual(t, "param1", res)
	res = GetTrimmedParam("noParam")
	utils.AssertEqual(t, "noParam", res)
}

func Test_Utils_RemoveEscapeChar(t *testing.T) {
	t.Parallel()
	res := RemoveEscapeChar(":test\\:bla")
	utils.AssertEqual(t, ":test:bla", res)
	res = RemoveEscapeChar("\\abc")
	utils.AssertEqual(t, "abc", res)
	res = RemoveEscapeChar("noEscapeChar")
	utils.AssertEqual(t, "noEscapeChar", res)
}

func Benchmark_Utils_RemoveEscapeChar(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	var res string
	for n := 0; n < b.N; n++ {
		res = RemoveEscapeChar(":test\\:bla")
	}

	utils.AssertEqual(b, ":test:bla", res)
}

// go test -race -run Test_Path_matchParams
func Benchmark_Path_matchParams(t *testing.B) {
	var ctxParams [maxParams]string
	benchCaseFn := func(testCollection routeCaseCollection) {
		parser := parseRoute(testCollection.pattern)
		for _, c := range testCollection.testCases {
			var matchRes bool
			state := "match"
			if !c.match {
				state = "not match"
			}
			t.Run(testCollection.pattern+" | "+state+" | "+c.url, func(b *testing.B) {
				for i := 0; i <= b.N; i++ {
					if match := parser.getMatch(c.url, c.url, &ctxParams, c.partialCheck); match {
						// Get testCases from the original path
						matchRes = true
					}
				}
				utils.AssertEqual(t, c.match, matchRes, fmt.Sprintf("route: '%s', url: '%s'", testCollection.pattern, c.url))
				if matchRes && len(c.params) > 0 {
					utils.AssertEqual(t, c.params[0:len(c.params)-1], ctxParams[0:len(c.params)-1], fmt.Sprintf("route: '%s', url: '%s'", testCollection.pattern, c.url))
				}
			})
		}
	}

	for _, testCollection := range benchmarkCases {
		benchCaseFn(testCollection)
	}
}

// go test -race -run Test_RoutePatternMatch
func Benchmark_RoutePatternMatch(t *testing.B) {
	benchCaseFn := func(testCollection routeCaseCollection) {
		for _, c := range testCollection.testCases {
			// skip all cases for partial checks
			if c.partialCheck {
				continue
			}
			var matchRes bool
			state := "match"
			if !c.match {
				state = "not match"
			}
			t.Run(testCollection.pattern+" | "+state+" | "+c.url, func(b *testing.B) {
				for i := 0; i <= b.N; i++ {
					if match := RoutePatternMatch(c.url, testCollection.pattern); match {
						// Get testCases from the original path
						matchRes = true
					}
				}
				utils.AssertEqual(t, c.match, matchRes, fmt.Sprintf("route: '%s', url: '%s'", testCollection.pattern, c.url))
			})
		}
	}

	for _, testCollection := range benchmarkCases {
		benchCaseFn(testCollection)
	}
}
