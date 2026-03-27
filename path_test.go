// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -race -run Test_Path_parseRoute
func Test_Path_parseRoute(t *testing.T) {
	t.Parallel()
	var rp routeParser

	rp = parseRoute("/shop/product/::filter/color::color/size::size")
	require.Equal(t, routeParser{
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
	require.Equal(t, routeParser{
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
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/name:customVerb", Length: 33, IsLast: true},
		},
		params: nil,
	}, rp)

	rp = parseRoute("/v1/some/resource/:name\\:customVerb")
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/", Length: 18},
			{IsParam: true, ParamName: "name", ComparePart: ":customVerb", PartCount: 1},
			{Const: ":customVerb", Length: 11, IsLast: true},
		},
		params: []string{"name"},
	}, rp)

	// heavy test with escaped characters
	rp = parseRoute("/v1/some/resource/name\\\\:customVerb?\\?/:param/*")
	require.Equal(t, routeParser{
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
	require.Equal(t, routeParser{
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
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/test", Length: 5},
			{IsParam: true, ParamName: "optional", IsOptional: true, Length: 1},
			{IsParam: true, ParamName: "optional2", IsOptional: true, IsLast: true},
		},
		params: []string{"optional", "optional2"},
	}, rp)

	rp = parseRoute("/config/+.json")
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/config/", Length: 8},
			{IsParam: true, ParamName: "+1", IsGreedy: true, IsOptional: false, ComparePart: ".json", PartCount: 1},
			{Const: ".json", Length: 5, IsLast: true},
		},
		params:    []string{"+1"},
		plusCount: 1,
	}, rp)

	rp = parseRoute("/api/:day.:month?.:year?")
	require.Equal(t, routeParser{
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
	require.Equal(t, routeParser{
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
			require.Equal(t, c.match, match, "route: '%s', url: '%s'", testCollection.pattern, c.url)
			if match && len(c.params) > 0 {
				require.Equal(t, c.params[0:len(c.params)], ctxParams[0:len(c.params)], "route: '%s', url: '%s'", testCollection.pattern, c.url)
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
			require.Equal(t, c.match, match, "route: '%s', url: '%s'", pattern, c.url)
		}
	}
	for _, testCase := range routeTestCases {
		testCaseFn(testCase.pattern, testCase.testCases)
	}
}

func TestHasPartialMatchBoundary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		path          string
		matchedLength int
		expected      bool
	}{
		{
			name:          "negative length",
			path:          "/demo",
			matchedLength: -1,
			expected:      false,
		},
		{
			name:          "greater than length",
			path:          "/demo",
			matchedLength: 6,
			expected:      false,
		},
		{
			name:          "exact match",
			path:          "/demo",
			matchedLength: len("/demo"),
			expected:      true,
		},
		{
			name:          "zero length",
			path:          "/demo",
			matchedLength: 0,
			expected:      false,
		},
		{
			name:          "previous rune slash",
			path:          "/demo/child",
			matchedLength: len("/demo/"),
			expected:      true,
		},
		{
			name:          "next rune slash",
			path:          "/demo/child",
			matchedLength: len("/demo"),
			expected:      true,
		},
		{
			name:          "no boundary",
			path:          "/demo/child",
			matchedLength: len("/dem"),
			expected:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.expected, hasPartialMatchBoundary(testCase.path, testCase.matchedLength))
		})
	}
}

func Test_Utils_GetTrimmedParam(t *testing.T) {
	t.Parallel()
	res := GetTrimmedParam("")
	require.Empty(t, res)
	res = GetTrimmedParam("*")
	require.Equal(t, "*", res)
	res = GetTrimmedParam(":param")
	require.Equal(t, "param", res)
	res = GetTrimmedParam(":param1?")
	require.Equal(t, "param1", res)
	res = GetTrimmedParam("noParam")
	require.Equal(t, "noParam", res)
}

func Test_Utils_RemoveEscapeChar(t *testing.T) {
	t.Parallel()
	res := RemoveEscapeChar(":test\\:bla")
	require.Equal(t, ":test:bla", res)
	res = RemoveEscapeChar("\\abc")
	require.Equal(t, "abc", res)
	res = RemoveEscapeChar("noEscapeChar")
	require.Equal(t, "noEscapeChar", res)
}

func Test_ConstraintCheckConstraint_InvalidMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		param      string
		constraint Constraint
	}{
		{
			name:       "minLen invalid metadata",
			constraint: Constraint{ID: minLenConstraint, Data: []string{"abc"}},
			param:      "abcd",
		},
		{
			name:       "maxLen invalid metadata",
			constraint: Constraint{ID: maxLenConstraint, Data: []string{"abc"}},
			param:      "abcd",
		},
		{
			name:       "len invalid metadata",
			constraint: Constraint{ID: lenConstraint, Data: []string{"abc"}},
			param:      "abcd",
		},
		{
			name:       "betweenLen invalid first metadata",
			constraint: Constraint{ID: betweenLenConstraint, Data: []string{"abc", "5"}},
			param:      "abcd",
		},
		{
			name:       "betweenLen invalid second metadata",
			constraint: Constraint{ID: betweenLenConstraint, Data: []string{"1", "abc"}},
			param:      "abcd",
		},
		{
			name:       "min invalid metadata",
			constraint: Constraint{ID: minConstraint, Data: []string{"abc"}},
			param:      "10",
		},
		{
			name:       "max invalid metadata",
			constraint: Constraint{ID: maxConstraint, Data: []string{"abc"}},
			param:      "10",
		},
		{
			name:       "range invalid first metadata",
			constraint: Constraint{ID: rangeConstraint, Data: []string{"abc", "10"}},
			param:      "7",
		},
		{
			name:       "range invalid second metadata",
			constraint: Constraint{ID: rangeConstraint, Data: []string{"1", "abc"}},
			param:      "7",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			require.False(t, testCase.constraint.CheckConstraint(testCase.param))
		})
	}
}

func Benchmark_Utils_RemoveEscapeChar(b *testing.B) {
	b.ReportAllocs()
	var res string
	for b.Loop() {
		res = RemoveEscapeChar(":test\\:bla")
	}

	require.Equal(b, ":test:bla", res)
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
				for b.Loop() {
					if match := parser.getMatch(c.url, c.url, &ctxParams, c.partialCheck); match {
						// Get testCases from the original path
						matchRes = true
					}
				}
				require.Equal(t, c.match, matchRes, "route: '%s', url: '%s'", testCollection.pattern, c.url)
				if matchRes && len(c.params) > 0 {
					require.Equal(t, c.params[0:len(c.params)-1], ctxParams[0:len(c.params)-1], "route: '%s', url: '%s'", testCollection.pattern, c.url)
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
				for b.Loop() {
					if match := RoutePatternMatch(c.url, testCollection.pattern); match {
						// Get testCases from the original path
						matchRes = true
					}
				}
				require.Equal(t, c.match, matchRes, "route: '%s', url: '%s'", testCollection.pattern, c.url)
			})
		}
	}

	for _, testCollection := range benchmarkCases {
		benchCaseFn(testCollection)
	}
}

func Test_Route_TooManyParams_Panic(t *testing.T) {
	t.Parallel()

	// Test with exactly maxParams (30) - should work
	t.Run("exactly_maxParams", func(t *testing.T) {
		t.Parallel()
		route := paramsRoute(t, maxParams)
		require.NotPanics(t, func() {
			parseRoute(route)
		})
	})

	// Test with maxParams + 1 (31) - should panic
	t.Run("maxParams_plus_one", func(t *testing.T) {
		t.Parallel()
		route := paramsRoute(t, maxParams+1)
		require.PanicsWithValue(t, "Route '"+route+"' has 31 parameters, which exceeds the maximum of 30", func() {
			parseRoute(route)
		})
	})

	// Test with 35 params - should panic
	t.Run("35_params", func(t *testing.T) {
		t.Parallel()
		route := paramsRoute(t, maxParams+5)
		require.PanicsWithValue(t, "Route '"+route+"' has 35 parameters, which exceeds the maximum of 30", func() {
			parseRoute(route)
		})
	})
}

func Test_App_Register_TooManyParams_Panic(t *testing.T) {
	t.Parallel()

	// Test registering a route with too many params via app
	t.Run("register_via_Get", func(t *testing.T) {
		t.Parallel()
		app := New()
		route := paramsRoute(t, maxParams+1)

		require.PanicsWithValue(t, "Route '"+route+"' has 31 parameters, which exceeds the maximum of 30", func() {
			app.Get(route, func(c Ctx) error {
				return c.SendString("test")
			})
		})
	})

	// Test registering a route with maxParams works
	t.Run("register_maxParams_works", func(t *testing.T) {
		t.Parallel()
		app := New()
		route := paramsRoute(t, maxParams)

		require.NotPanics(t, func() {
			app.Get(route, func(c Ctx) error {
				return c.SendString("test")
			})
		})
	})
}

// paramsRoute generates a route with n parameters for testing parseRoute maxParams condition.
// Returns a route in the format "/:p1/:p2/:p3/.../:pN"
func paramsRoute(t *testing.T, n int) string {
	t.Helper()
	params := make([]string, n)
	for i := range params {
		params[i] = fmt.Sprintf(":p%d", i+1)
	}
	return "/" + strings.Join(params, "/")
}
