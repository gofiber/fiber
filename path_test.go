// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
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

func TestRouteParserResetBounds(t *testing.T) {
	t.Parallel()

	parser := &routeParser{
		segs:          make([]*routeSegment, 1, routeParserSegMaxCap+32),
		params:        make([]string, 1, routeParserParamMaxCap+32),
		wildCardCount: 3,
		plusCount:     2,
	}

	parser.segs[0] = &routeSegment{Const: "value"}
	parser.params[0] = "param"

	parser.reset()

	require.Zero(t, len(parser.segs))
	require.Zero(t, len(parser.params))
	require.Equal(t, routeParserSegDefaultCap, cap(parser.segs))
	require.Equal(t, routeParserParamDefaultCap, cap(parser.params))
	require.Zero(t, parser.wildCardCount)
	require.Zero(t, parser.plusCount)
}

func TestRouteSegmentPoolResetsState(t *testing.T) {
	t.Parallel()

	seg := acquireRouteSegment()
	seg.Const = "value"
	seg.ParamName = "param"
	seg.ComparePart = "compare"
	seg.Constraints = []*Constraint{{Name: "id", Data: []string{"1"}}}
	seg.PartCount = 3
	seg.Length = 42
	seg.IsParam = true
	seg.IsGreedy = true
	seg.IsOptional = true
	seg.IsLast = true
	seg.HasOptionalSlash = true

	releaseRouteSegment(seg)

	reused := acquireRouteSegment()
	require.Empty(t, reused.Const)
	require.Empty(t, reused.ParamName)
	require.Empty(t, reused.ComparePart)
	require.Nil(t, reused.Constraints)
	require.Zero(t, reused.PartCount)
	require.Zero(t, reused.Length)
	require.False(t, reused.IsParam)
	require.False(t, reused.IsGreedy)
	require.False(t, reused.IsOptional)
	require.False(t, reused.IsLast)
	require.False(t, reused.HasOptionalSlash)

	releaseRouteSegment(reused)
}

func TestRouteParserResetReleasesSegments(t *testing.T) {
	t.Parallel()

	parser := &routeParser{
		segs:   make([]*routeSegment, 0, 1),
		params: make([]string, 0, 1),
	}

	seg := acquireRouteSegment()
	seg.Const = "value"
	parser.segs = append(parser.segs, seg)
	parser.params = append(parser.params, "param")

	parser.reset()

	require.Empty(t, parser.segs)
	require.Empty(t, parser.params)
	require.Empty(t, seg.Const)
	require.False(t, seg.IsParam)
	require.False(t, seg.IsGreedy)
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
	require.Equal(t, "", res)
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
