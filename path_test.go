// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"regexp"
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

	// heavy test with escaped charaters
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
	b.ResetTimer()
	var res string
	for n := 0; n < b.N; n++ {
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
				for i := 0; i < b.N; i++ {
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
				for i := 0; i < b.N; i++ {
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

func TestConstraint_CheckConstraint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		constraint     *Constraint
		param          string
		expectedResult bool
	}{
		{
			name:           "no constraint",
			constraint:     &Constraint{ID: noConstraint},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "int constraint valid",
			constraint:     &Constraint{ID: intConstraint},
			param:          "123",
			expectedResult: true,
		},
		{
			name:           "int constraint invalid",
			constraint:     &Constraint{ID: intConstraint},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "bool constraint valid",
			constraint:     &Constraint{ID: boolConstraint},
			param:          "true",
			expectedResult: true,
		},
		{
			name:           "bool constraint invalid",
			constraint:     &Constraint{ID: boolConstraint},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "float constraint valid",
			constraint:     &Constraint{ID: floatConstraint},
			param:          "1.23",
			expectedResult: true,
		},
		{
			name:           "float constraint invalid",
			constraint:     &Constraint{ID: floatConstraint},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "alpha constraint valid",
			constraint:     &Constraint{ID: alphaConstraint},
			param:          "abc",
			expectedResult: true,
		},
		{
			name:           "alpha constraint invalid",
			constraint:     &Constraint{ID: alphaConstraint},
			param:          "123",
			expectedResult: false,
		},
		{
			name:           "guid constraint valid",
			constraint:     &Constraint{ID: guidConstraint},
			param:          "123e4567-e89b-12d3-a456-426614174000",
			expectedResult: true,
		},
		{
			name:           "guid constraint invalid",
			constraint:     &Constraint{ID: guidConstraint},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "min length constraint valid",
			constraint:     &Constraint{ID: minLenConstraint, Data: []string{"3"}},
			param:          "abc",
			expectedResult: true,
		},
		{
			name:           "min length constraint invalid",
			constraint:     &Constraint{ID: minLenConstraint, Data: []string{"5"}},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "max length constraint valid",
			constraint:     &Constraint{ID: maxLenConstraint, Data: []string{"5"}},
			param:          "abc",
			expectedResult: true,
		},
		{
			name:           "max length constraint invalid",
			constraint:     &Constraint{ID: maxLenConstraint, Data: []string{"2"}},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "length constraint valid",
			constraint:     &Constraint{ID: lenConstraint, Data: []string{"3"}},
			param:          "abc",
			expectedResult: true,
		},
		{
			name:           "length constraint invalid",
			constraint:     &Constraint{ID: lenConstraint, Data: []string{"5"}},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "between length constraint valid",
			constraint:     &Constraint{ID: betweenLenConstraint, Data: []string{"2", "4"}},
			param:          "abc",
			expectedResult: true,
		},
		{
			name:           "between length constraint invalid",
			constraint:     &Constraint{ID: betweenLenConstraint, Data: []string{"4", "6"}},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "min constraint valid",
			constraint:     &Constraint{ID: minConstraint, Data: []string{"2"}},
			param:          "3",
			expectedResult: true,
		},
		{
			name:           "min constraint invalid",
			constraint:     &Constraint{ID: minConstraint, Data: []string{"4"}},
			param:          "3",
			expectedResult: false,
		},
		{
			name:           "max constraint valid",
			constraint:     &Constraint{ID: maxConstraint, Data: []string{"4"}},
			param:          "3",
			expectedResult: true,
		},
		{
			name:           "max constraint invalid",
			constraint:     &Constraint{ID: maxConstraint, Data: []string{"2"}},
			param:          "3",
			expectedResult: false,
		},
		{
			name:           "range constraint valid",
			constraint:     &Constraint{ID: rangeConstraint, Data: []string{"2", "4"}},
			param:          "3",
			expectedResult: true,
		},
		{
			name:           "range constraint invalid",
			constraint:     &Constraint{ID: rangeConstraint, Data: []string{"4", "6"}},
			param:          "3",
			expectedResult: false,
		},
		{
			name:           "datetime constraint valid",
			constraint:     &Constraint{ID: datetimeConstraint, Data: []string{"2006-01-02"}},
			param:          "2023-05-20",
			expectedResult: true,
		},
		{
			name:           "datetime constraint invalid",
			constraint:     &Constraint{ID: datetimeConstraint, Data: []string{"2006-01-02"}},
			param:          "2023/05/20",
			expectedResult: false,
		},
		{
			name:           "regex constraint valid",
			constraint:     &Constraint{ID: regexConstraint, Data: []string{`^\d+$`}, RegexCompiler: regexp.MustCompile(`^\d+$`)},
			param:          "123",
			expectedResult: true,
		},
		{
			name:           "regex constraint invalid",
			constraint:     &Constraint{ID: regexConstraint, RegexCompiler: regexp.MustCompile(`^\d+$`)},
			param:          "abc",
			expectedResult: false,
		},
		{
			name:           "custom constraint valid",
			constraint:     &Constraint{Name: "custom", customConstraints: []CustomConstraint{&mockCustomConstraint{}}},
			param:          "abc",
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := tc.constraint.CheckConstraint(tc.param)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

type mockCustomConstraint struct{}

func (*mockCustomConstraint) Name() string {
	return "custom"
}

func (*mockCustomConstraint) Execute(_ string, _ ...string) bool {
	return true
}
