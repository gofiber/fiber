// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -race -run Test_Path_parseRoute
func Test_Path_parseRoute(t *testing.T) {
	t.Parallel()
	var rp routeParser

	rp = parseRoute("/shop/product/::filter/color::color/size::size", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/shop/product/:", Length: 15},
			{IsParam: true, ParamName: "filter", ComparePart: "/color:", PartCount: 1},
			{Const: "/color:", Length: 7},
			{IsParam: true, ParamName: "color", ComparePart: "/size:", PartCount: 1},
			{Const: "/size:", Length: 6},
			{IsParam: true, ParamName: "size", IsLast: true},
		},
		params:     []string{"filter", "color", "size"},
		minSlashes: 5,
		maxSlashes: 5,
		maxBounded: true,
		probe:      newConstProbe("/color:", 3),
	}, rp)

	rp = parseRoute("/api/v1/:param/abc/*", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/api/v1/", Length: 8},
			{IsParam: true, ParamName: "param", ComparePart: "/abc", PartCount: 1},
			{Const: "/abc/", Length: 5, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, IsLast: true},
		},
		params:        []string{"param", "*1"},
		wildCardCount: 1,
		minSlashes:    4,
		probe:         newConstProbe("/abc", 3),
	}, rp)

	rp = parseRoute("/v1/some/resource/name\\:customVerb", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/name:customVerb", Length: 33, IsLast: true},
		},
		params:     nil,
		minSlashes: 4,
		maxSlashes: 4,
		maxBounded: true,
	}, rp)

	rp = parseRoute("/v1/some/resource/:name\\:customVerb", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/", Length: 18},
			{IsParam: true, ParamName: "name", ComparePart: ":customVerb", PartCount: 1},
			{Const: ":customVerb", Length: 11, IsLast: true},
		},
		params:     []string{"name"},
		minSlashes: 4,
		maxSlashes: 4,
		maxBounded: true,
	}, rp)

	// heavy test with escaped characters
	rp = parseRoute("/v1/some/resource/name\\\\:customVerb?\\?/:param/*", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/v1/some/resource/name:customVerb??/", Length: 36},
			{IsParam: true, ParamName: "param", ComparePart: "/", PartCount: 1},
			{Const: "/", Length: 1, HasOptionalSlash: true},
			{IsParam: true, ParamName: "*1", IsGreedy: true, IsOptional: true, IsLast: true},
		},
		params:        []string{"param", "*1"},
		wildCardCount: 1,
		minSlashes:    5,
	}, rp)

	rp = parseRoute("/api/*/:param/:param2", regexp.MustCompile)
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
		minSlashes:    3,
	}, rp)

	rp = parseRoute("/test:optional?:optional2?", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/test", Length: 5},
			{IsParam: true, ParamName: "optional", IsOptional: true, Length: 1},
			{IsParam: true, ParamName: "optional2", IsOptional: true, IsLast: true},
		},
		params:     []string{"optional", "optional2"},
		minSlashes: 1,
	}, rp)

	rp = parseRoute("/config/+.json", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/config/", Length: 8},
			{IsParam: true, ParamName: "+1", IsGreedy: true, IsOptional: false, ComparePart: ".json", PartCount: 1},
			{Const: ".json", Length: 5, IsLast: true},
		},
		params:     []string{"+1"},
		plusCount:  1,
		minSlashes: 2,
	}, rp)

	rp = parseRoute("/api/:day.:month?.:year?", regexp.MustCompile)
	require.Equal(t, routeParser{
		segs: []*routeSegment{
			{Const: "/api/", Length: 5},
			{IsParam: true, ParamName: "day", IsOptional: false, ComparePart: ".", PartCount: 2},
			{Const: ".", Length: 1},
			{IsParam: true, ParamName: "month", IsOptional: true, ComparePart: ".", PartCount: 1},
			{Const: ".", Length: 1},
			{IsParam: true, ParamName: "year", IsOptional: true, IsLast: true},
		},
		params:     []string{"day", "month", "year"},
		minSlashes: 2,
	}, rp)

	rp = parseRoute("/*v1*/proxy", regexp.MustCompile)
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
		minSlashes:    1,
	}, rp)
}

// go test -race -run Test_Path_matchParams
func Test_Path_matchParams(t *testing.T) {
	t.Parallel()
	var ctxParams [maxParams]string
	testCaseFn := func(testCollection routeCaseCollection) {
		parser := parseRoute(testCollection.pattern, regexp.MustCompile)
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

// go test -race -run Test_RouteParser_SlashBounds
func Test_RouteParser_SlashBounds(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		pattern    string
		minSlashes int32
		maxSlashes int32
		maxBounded bool
	}{
		{pattern: "/", minSlashes: 0, maxSlashes: 1, maxBounded: true},
		{pattern: "/api/v1/const", minSlashes: 3, maxSlashes: 3, maxBounded: true},
		{pattern: "/api/v1/:param", minSlashes: 3, maxSlashes: 3, maxBounded: true},
		{pattern: "/api/v1/:param?", minSlashes: 2, maxSlashes: 3, maxBounded: true},
		{pattern: "/api/v1/:param/fixedEnd", minSlashes: 4, maxSlashes: 4, maxBounded: true},
		// greedy parameters match across '/', so no upper bound
		{pattern: "/api/*", minSlashes: 1},
		{pattern: "/api/+", minSlashes: 2},
		// optional segments drop their leading slashes from the lower bound
		{pattern: "/api/:day/:month?/:year?", minSlashes: 2, maxSlashes: 4, maxBounded: true},
		// single-byte compare parts have no slash guard in findParamLen,
		// so such parameters can swallow '/' and the max is unbounded
		{pattern: "/api/v1/:a-:b", minSlashes: 3},
		{pattern: "/api/:day.:month?.:year?", minSlashes: 2},
		// successive parameters consume one byte each, possibly a '/'
		{pattern: "/test:sign:param", minSlashes: 1},
		// multi-byte compare parts reject slashes, so bounds stay exact
		{pattern: "/shop/product/::filter/color::color/size::size", minSlashes: 5, maxSlashes: 5, maxBounded: true},
		{pattern: "/v1/some/resource/name\\:customVerb", minSlashes: 4, maxSlashes: 4, maxBounded: true},
	}
	for _, tc := range testCases {
		parser := parseRoute(tc.pattern, regexp.MustCompile)
		require.Equal(t, tc.minSlashes, parser.minSlashes, "route: '%s' minSlashes", tc.pattern)
		require.Equal(t, tc.maxSlashes, parser.maxSlashes, "route: '%s' maxSlashes", tc.pattern)
		require.Equal(t, tc.maxBounded, parser.maxBounded, "route: '%s' maxBounded", tc.pattern)
	}
}

// Test_Route_Match_SlashBoundsDifferential generatively proves the two
// slash-index filters in Route.match — the slash-count quick-reject and the
// probe compare — are transparent: for every generated pattern and path, the
// filtered Route.match must agree with a raw getMatch on the same input.
// Unlike the fixture-driven tests this needs no hand-authored expectations, so
// it also binds pattern shapes nobody thought to add to the fixture — if
// findParamLen ever lets a new shape swallow '/', or computeProbe ever admits
// a segment shape whose constant can move, this fails.
// go test -race -run Test_Route_Match_SlashBoundsDifferential
func Test_Route_Match_SlashBoundsDifferential(t *testing.T) {
	t.Parallel()

	segments := []string{
		"/api", "/foo/", "/:a", "/:b?", "/*", "/+", "/:a-:b", "/:f.:e?",
		":tail", "/::c", "/:x:y", "/name\\:verb", "/:p/fixed",
		// probe shapes: constants after parameters, longer than a word, with
		// optional slashes, and behind adjacent, optional or greedy parameters
		"/:a/:b/fixed", "/:p/verylongconstantsegment", "/x/:y/z/", "/:q/a/b/c",
		"/:a?/fixed", "/:x:y/fixed", "/*/fixed", "/:s<int>/x",
	}
	// patterns: every single segment and every ordered pair
	patterns := make([]string, 0, len(segments)*(len(segments)+1))
	for _, s1 := range segments {
		patterns = append(patterns, s1)
		for _, s2 := range segments {
			patterns = append(patterns, s1+s2)
		}
	}

	pieces := []string{
		"", "/a", "/a/b", "/a-b", "/a.b", "/x/y-z", "/enti/ty-x", "/a/",
		"/:c", "/api", "/api/foo/bar", "/name:verb", "/fixed",
		"/a/fixed", "/a/b/fixed", "/fixedx", "/verylongconstantsegment",
		"/verylongconstantsegmentx", "/x/1/z", "/x/1/z/", "/1/a/b/c", "//fixed",
	}
	// paths: every ordered pair of pieces (skipping the empty result)
	paths := make([]string, 0, len(pieces)*len(pieces))
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			if p1+p2 == "" {
				continue
			}
			paths = append(paths, p1+p2)
		}
	}

	for _, pattern := range patterns {
		parser := parseRoute(pattern, regexp.MustCompile)
		if len(parser.params) == 0 {
			// non-parametric routes never take the filtered getMatch path
			continue
		}
		route := &Route{
			routeParser: parser,
			Params:      parser.params,
			path:        pattern,
			Path:        pattern,
		}
		for _, use := range []bool{false, true} {
			route.use = use
			for _, path := range paths {
				// an empty index is the "unknown" state and must bypass the filters
				for _, slashes := range []*slashIndex{slashIndexOf(path), {}} {
					var filteredParams, rawParams [maxParams]string
					filtered := route.match(path, path, &filteredParams, slashes)
					raw := parser.getMatch(path, path, &rawParams, use)
					if filtered != raw {
						t.Fatalf("filter changed outcome: pattern %q, path %q, use %v, slashes %d: filtered=%v raw=%v",
							pattern, path, use, slashes.count, filtered, raw)
					}
					if raw {
						require.Equal(t, rawParams[:len(parser.params)], filteredParams[:len(parser.params)],
							"params diverged: pattern %q, path %q, use %v", pattern, path, use)
					}
				}
			}
		}
	}
}

// Test_Route_Match_SlashBoundsConsistency proves the slash-count quick-reject in
// Route.match never flips the outcome of the exhaustive matching fixture. Only
// parametric patterns are checked, since only they take the filtered path.
// go test -race -run Test_Route_Match_SlashBoundsConsistency
func Test_Route_Match_SlashBoundsConsistency(t *testing.T) {
	t.Parallel()
	for _, testCollection := range routeTestCases {
		parser := parseRoute(testCollection.pattern, regexp.MustCompile)
		if len(parser.params) == 0 {
			continue
		}
		route := &Route{
			routeParser: parser,
			Params:      parser.params,
			path:        testCollection.pattern,
			Path:        testCollection.pattern,
		}
		for _, c := range testCollection.testCases {
			route.use = c.partialCheck
			var ctxParams [maxParams]string
			match := route.match(c.url, c.url, &ctxParams, slashIndexOf(c.url))
			require.Equal(t, c.match, match, "route: '%s', url: '%s'", testCollection.pattern, c.url)
			if match && len(c.params) > 0 {
				require.Equal(t, c.params[0:len(c.params)], ctxParams[0:len(c.params)], "route: '%s', url: '%s'", testCollection.pattern, c.url)
			}
		}
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
			constraint: *newConstraint(minLenConstraintType{}, ConstraintMinLen, []string{"abc"}),
			param:      "abcd",
		},
		{
			name:       "maxLen invalid metadata",
			constraint: *newConstraint(maxLenConstraintType{}, ConstraintMaxLen, []string{"abc"}),
			param:      "abcd",
		},
		{
			name:       "len invalid metadata",
			constraint: *newConstraint(lenConstraintType{}, ConstraintLen, []string{"abc"}),
			param:      "abcd",
		},
		{
			name:       "betweenLen invalid first metadata",
			constraint: *newConstraint(betweenLenConstraintType{}, ConstraintBetweenLen, []string{"abc", "5"}),
			param:      "abcd",
		},
		{
			name:       "betweenLen invalid second metadata",
			constraint: *newConstraint(betweenLenConstraintType{}, ConstraintBetweenLen, []string{"1", "abc"}),
			param:      "abcd",
		},
		{
			name:       "min invalid metadata",
			constraint: *newConstraint(minConstraintType{}, ConstraintMin, []string{"abc"}),
			param:      "10",
		},
		{
			name:       "max invalid metadata",
			constraint: *newConstraint(maxConstraintType{}, ConstraintMax, []string{"abc"}),
			param:      "10",
		},
		{
			name:       "range invalid first metadata",
			constraint: *newConstraint(rangeConstraintType{}, ConstraintRange, []string{"abc", "10"}),
			param:      "7",
		},
		{
			name:       "range invalid second metadata",
			constraint: *newConstraint(rangeConstraintType{}, ConstraintRange, []string{"1", "abc"}),
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

func Test_ConstraintCheckConstraint_NilRegexMatcher(t *testing.T) {
	t.Parallel()

	constraint := *newConstraint(regexConstraintType{}, ConstraintRegex, []string{"("})

	require.NotPanics(t, func() {
		require.False(t, constraint.CheckConstraint("123"))
	})
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
		parser := parseRoute(testCollection.pattern, regexp.MustCompile)
		for _, c := range testCollection.testCases {
			var matchRes bool
			state := "match"
			if !c.match {
				state = "not match"
			}
			t.Run(testCollection.pattern+"_"+state+"_"+c.url, func(b *testing.B) {
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
func Benchmark_ConstraintExecution(b *testing.B) {
	var ctxParams [maxParams]string
	var match bool

	constraintPatterns := []struct {
		name    string
		pattern string
		url     string
	}{
		{"int", "/api/:id<int>", "/api/12345"},
		{"bool", "/api/:flag<bool>", "/api/true"},
		{"float", "/api/:val<float>", "/api/3.14"},
		{"alpha", "/api/:name<alpha>", "/api/hello"},
		{"guid", "/api/:id<guid>", "/api/12345678-1234-1234-1234-123456789abc"},
		{"minLen", "/api/:name<minLen(3)>", "/api/hello"},
		{"maxLen", "/api/:name<maxLen(10)>", "/api/hello"},
		{"len", "/api/:name<len(5)>", "/api/hello"},
		{"betweenLen", "/api/:name<betweenLen(2,10)>", "/api/hello"},
		{"min", "/api/:id<min(5)>", "/api/10"},
		{"max", "/api/:id<max(100)>", "/api/10"},
		{"range", "/api/:id<range(1,20)>", "/api/10"},
		{"datetime", "/api/:date<datetime(2006-01-02)>", "/api/2024-01-15"},
		{"regex", "/api/:id<regex(^[0-9]+$)>", "/api/12345"},
	}

	for _, tc := range constraintPatterns {
		b.Run(tc.name, func(b *testing.B) {
			parser := parseRoute(tc.pattern, regexp.MustCompile)
			for b.Loop() {
				match = parser.getMatch(tc.url, tc.url, &ctxParams, false)
			}
		})
	}
	_ = match
}

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
			t.Run(testCollection.pattern+"_"+state+"_"+c.url, func(b *testing.B) {
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
			parseRoute(route, regexp.MustCompile)
		})
	})

	// Test with maxParams + 1 (31) - should panic
	t.Run("maxParams_plus_one", func(t *testing.T) {
		t.Parallel()
		route := paramsRoute(t, maxParams+1)
		require.PanicsWithValue(t, "Route '"+route+"' has 31 parameters, which exceeds the maximum of 30", func() {
			parseRoute(route, regexp.MustCompile)
		})
	})

	// Test with 35 params - should panic
	t.Run("35_params", func(t *testing.T) {
		t.Parallel()
		route := paramsRoute(t, maxParams+5)
		require.PanicsWithValue(t, "Route '"+route+"' has 35 parameters, which exceeds the maximum of 30", func() {
			parseRoute(route, regexp.MustCompile)
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

// Test_RegexHandler_Default verifies Fiber defaults RegexHandler to regexp.MustCompile.
func Test_RegexHandler_Default(t *testing.T) {
	t.Parallel()

	app := New()

	require.NotNil(t, app.config.RegexHandler)
	require.Equal(t, reflect.ValueOf(regexp.MustCompile).Pointer(), reflect.ValueOf(app.config.RegexHandler).Pointer())

	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

// mockRegexCompiler is a mock implementation of regex matching for testing
type mockRegexCompiler struct {
	*regexp.Regexp
	matchCalled bool
}

func (m *mockRegexCompiler) MatchString(s string) bool {
	m.matchCalled = true
	return m.Regexp.MatchString(s)
}

type matchOnlyRegexCompiler struct {
	re *regexp.Regexp
}

func (m *matchOnlyRegexCompiler) MatchString(s string) bool {
	return m.re.MatchString(s)
}

type regexPattern string

// mockRegexHandler is a mock regex handler function for testing
func mockRegexHandler(lastPattern *string, compileCalled *bool) any {
	return func(pattern string) regexMatcher {
		*compileCalled = true
		*lastPattern = pattern
		return &mockRegexCompiler{
			Regexp: regexp.MustCompile(pattern),
		}
	}
}

// Test_RegexHandler_Custom verifies that a custom regex handler can be used
func Test_RegexHandler_Custom(t *testing.T) {
	t.Parallel()

	var lastPattern string
	var compileCalled bool

	// Create app with custom regex handler
	app := New(Config{
		RegexHandler: mockRegexHandler(&lastPattern, &compileCalled),
	})

	// Register a route with regex constraint
	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	// Verify the mock handler was used during route registration
	require.True(t, compileCalled, "RegexHandler should have been called")
	require.Equal(t, `\d+`, lastPattern, "Pattern should match")

	// Test the route
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Test with non-matching pattern
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/abc", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
}

// Test_RegexHandler_MatchOnlyCompiler verifies Fiber accepts compilers that only implement MatchString.
func Test_RegexHandler_MatchOnlyCompiler(t *testing.T) {
	t.Parallel()

	var compileCalled bool

	app := New(Config{
		RegexHandler: func(pattern string) *matchOnlyRegexCompiler {
			compileCalled = true
			return &matchOnlyRegexCompiler{re: regexp.MustCompile(pattern)}
		},
	})

	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	require.True(t, compileCalled)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
}

// Test_RegexHandler_DefaultCompilerPreservesConstraintField verifies stdlib handlers still populate the exported regexp field.
func Test_RegexHandler_DefaultCompilerPreservesConstraintField(t *testing.T) {
	t.Parallel()

	parser := parseRoute("/api/:id<regex(\\d+)>", regexp.MustCompile)
	require.Len(t, parser.segs, 2)
	require.Len(t, parser.segs[1].Constraints, 1)
	require.NotNil(t, parser.segs[1].Constraints[0].RegexCompiler)
	require.True(t, parser.segs[1].Constraints[0].matchConstraint("123"))
	require.False(t, parser.segs[1].Constraints[0].matchConstraint("abc"))
}

func Test_RegexHandler_CustomCompilerUsesSegmentMatcher(t *testing.T) {
	t.Parallel()

	parser := parseRoute("/api/:id<regex(\\d+)>", func(pattern string) *matchOnlyRegexCompiler {
		return &matchOnlyRegexCompiler{re: regexp.MustCompile(pattern)}
	})
	require.Len(t, parser.segs, 2)
	require.Len(t, parser.segs[1].Constraints, 1)
	require.True(t, parser.segs[1].Constraints[0].matchConstraint("123"))
	require.False(t, parser.segs[1].Constraints[0].matchConstraint("abc"))
}

// Test_RoutePatternMatch_WithRegex verifies RoutePatternMatch works with regex constraints
func Test_RoutePatternMatch_WithRegex(t *testing.T) {
	t.Parallel()

	// Test with default handler
	require.True(t, RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>"))
	require.False(t, RoutePatternMatch("/api/abc", "/api/:id<regex(\\d+)>"))

	// Test with custom config
	var lastPattern string
	var compileCalled bool
	require.True(t, RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>", Config{
		RegexHandler: mockRegexHandler(&lastPattern, &compileCalled),
	}))
	require.True(t, compileCalled, "RegexHandler should have been called")
}

// Test_RegexHandler_NilDefaultsToStdlib verifies that nil RegexHandler defaults to stdlib
func Test_RegexHandler_NilDefaultsToStdlib(t *testing.T) {
	t.Parallel()

	// Create app without specifying RegexHandler (should default)
	app := New()

	// Verify it's set to the default
	require.NotNil(t, app.config.RegexHandler)

	// Register a route with regex constraint
	app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
		return c.SendString("matched")
	})

	// Test the route works
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/123", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

// Test_RegexHandler_ComplexPattern tests complex regex patterns
func Test_RegexHandler_ComplexPattern(t *testing.T) {
	t.Parallel()

	app := New()

	// Test date pattern
	app.Get("/date/:date<regex(\\d{4}-\\d{2}-\\d{2})>", func(c Ctx) error {
		return c.SendString("date: " + c.Params("date"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/date/2024-01-15", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/date/2024-1-5", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

// Test_RegexHandler_InvalidConfigurationPanics verifies invalid handlers fail fast.
func Test_RegexHandler_InvalidConfigurationPanics(t *testing.T) {
	t.Parallel()

	t.Run("typed_nil_function", func(t *testing.T) {
		t.Parallel()

		var handler func(string) *regexp.Regexp
		require.PanicsWithValue(t, "fiber: Config.RegexHandler must be a non-nil function", func() {
			New(Config{RegexHandler: handler})
		})
	})

	t.Run("non_function", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fiber: Config.RegexHandler must be a non-nil function", func() {
			New(Config{RegexHandler: "invalid"})
		})
	})

	t.Run("invalid_signature", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fiber: Config.RegexHandler must have signature func(string) T", func() {
			New(Config{RegexHandler: func(int) *regexp.Regexp { return nil }})
		})
	})

	t.Run("named_string_parameter", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fiber: Config.RegexHandler must have signature func(string) T", func() {
			New(Config{RegexHandler: func(regexPattern) *regexp.Regexp { return nil }})
		})
	})

	t.Run("invalid_return_type", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fiber: Config.RegexHandler return type must support MatchString(string) bool", func() {
			New(Config{RegexHandler: func(string) string { return "" }})
		})
	})
}

// Test_RegexHandler_NilReturnPanics verifies a nil compiled matcher is rejected.
func Test_RegexHandler_NilReturnPanics(t *testing.T) {
	t.Parallel()

	app := New(Config{
		RegexHandler: func(string) *regexp.Regexp { return nil },
	})

	require.PanicsWithValue(t, "fiber: Config.RegexHandler must not return nil", func() {
		app.Get("/api/:id<regex(\\d+)>", func(c Ctx) error {
			return c.SendString("matched")
		})
	})
}

// Test_RoutePatternMatch_InvalidRegexHandlerPanics verifies RoutePatternMatch also validates RegexHandler configuration.
// Test_RoutePatternMatch_NormalizesInputs covers the guards that run before any
// parsing: an empty path and an empty or slash-less pattern are all coerced to
// the forms the router itself registers, so callers do not have to pre-normalize.
func Test_RoutePatternMatch_NormalizesInputs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{name: "empty pattern becomes root", path: "/", pattern: "", want: true},
		{name: "empty pattern does not match a child", path: "/a", pattern: "", want: false},
		{name: "empty path becomes root", path: "", pattern: "/", want: true},
		{name: "both empty", path: "", pattern: "", want: true},
		{name: "pattern gains a leading slash", path: "/a", pattern: "a", want: true},
		{name: "slash-less pattern with a param", path: "/1", pattern: ":id", want: true},
		{name: "slash-less pattern that should not match", path: "/b", pattern: "a", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RoutePatternMatch(tc.path, tc.pattern),
				"path=%q pattern=%q", tc.path, tc.pattern)
		})
	}
}

func Test_RoutePatternMatch_InvalidRegexHandlerPanics(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "fiber: Config.RegexHandler must be a non-nil function", func() {
		RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>", Config{RegexHandler: "invalid"})
	})
}

// Test_RouteParser_ConstParamShape pins which patterns the "/const/:param"
// specialization claims. A gate that silently stops firing turns the
// specialization into dead code, and one that fires too widely is a
// correctness bug, so both directions are asserted.
// go test -race -run Test_RouteParser_ConstParamShape
func Test_RouteParser_ConstParamShape(t *testing.T) {
	t.Parallel()

	specialized := []string{
		"/user/keys/:key_id", "/api/v1/:param", "/:param", "/a/:b",
		"/repos/:owner", "/x/:y",
	}
	generic := []string{
		"/", "/const", "/*", "/+", "/api/*", "/api/+",
		"/api/:a/:b",           // two params
		"/api/:a/fixed",        // param not last
		"/api/:a?",             // optional param (const carries the optional slash)
		"/apix:a?",             // optional param with no optional slash on the const
		"/api/:a<int>",         // constrained param
		"/api/:a-:b",           // adjacent params
		"/api/",                // no param at all
		"/user/keys/:id/extra", // trailing const
	}

	for _, pattern := range specialized {
		parser := parseRoute(pattern, regexp.MustCompile)
		require.True(t, parser.constParam, "expected specialization for %q", pattern)
	}
	for _, pattern := range generic {
		parser := parseRoute(pattern, regexp.MustCompile)
		require.False(t, parser.constParam, "unexpected specialization for %q", pattern)
	}
}

// slashIndexOf builds the slash index a request for path carries, already
// counted so the filters in Route.match are live; offsets are located from
// path on demand, exactly as in the router.
func slashIndexOf(path string) *slashIndex {
	return &slashIndex{count: strings.Count(path, "/")}
}

// Test_RouteParser_Probe pins computeProbe's gate: which segment shapes yield
// a probe, which constant it packs and at which slash it is tested.
// go test -race -run Test_RouteParser_Probe
func Test_RouteParser_Probe(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		pattern string
		want    constProbe
	}{
		// no parameter, or nothing after it
		{pattern: "/", want: constProbe{}},
		{pattern: "/api/v1/const", want: constProbe{}},
		{pattern: "/api/:id", want: constProbe{}},
		{pattern: "/api/:id/", want: constProbe{}},
		{pattern: "/repos/:owner/:repo", want: constProbe{}},
		// the first constant after a parameter, at the slash the constants
		// before it add up to
		{pattern: "/:p/fixed", want: newConstProbe("/fixed", 1)},
		{pattern: "/repos/:owner/:repo/issues", want: newConstProbe("/issues", 3)},
		{pattern: "/repos/:owner/:repo/issues/:number", want: newConstProbe("/issues/", 3)},
		{pattern: "/api/:a/b/:c/dd/:e", want: newConstProbe("/b/", 2)},
		{pattern: "/api/:a<int>/x", want: newConstProbe("/x", 2)},
		{pattern: "/v1/some/resource/:name/x", want: newConstProbe("/x", 4)},
		// longer than a word: the first word is the probe
		{pattern: "/api/:v/collaborators/:user", want: newConstProbe("/collabo", 2)},
		// a trailing '/' getMatch may drop is left out of the probe
		{pattern: "/user/:name/keys/", want: newConstProbe("/keys", 2)},
		{pattern: "/api/:a/x/:b?/y", want: newConstProbe("/x", 2)},
		// parameters that do not end at the next '/'
		{pattern: "/api/*/x", want: constProbe{}},
		{pattern: "/api/+/x", want: constProbe{}},
		{pattern: "/api/:a?/x", want: constProbe{}},
		{pattern: "/api/:a:b/x", want: constProbe{}},
		{pattern: "/api/:a-:b/x", want: constProbe{}},
		{pattern: "/files/:name.json/:x", want: constProbe{}},
		{pattern: "/v1/some/resource/:name\\:customVerb", want: constProbe{}},
		// the slash the probe would need is past what a slashIndex records
		{pattern: "/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/:x/y", want: constProbe{}},
		{pattern: "/a/b/c/d/e/f/g/h/i/j/k/l/m/n/:x/y", want: newConstProbe("/y", 15)},
	}
	for _, tc := range testCases {
		parser := parseRoute(tc.pattern, regexp.MustCompile)
		require.Equal(t, tc.want, parser.probe, "route: '%s'", tc.pattern)
	}

	// RoutePatternMatch's pooled parser never computes a probe, and reset
	// must not carry one over from parseRoute either.
	parser := parseRoute("/:p/fixed", regexp.MustCompile)
	require.NotZero(t, parser.probe.mask)
	parser.reset()
	require.Equal(t, constProbe{}, parser.probe)
}

// Test_ConstProbe_Rejects pins the probe compare itself, including the slash
// index states that have to stand it down.
// go test -race -run Test_ConstProbe_Rejects
func Test_ConstProbe_Rejects(t *testing.T) {
	t.Parallel()
	probe := newConstProbe("/issues/", 3)

	require.False(t, probe.rejects("/repos/a/b/issues/1", slashIndexOf("/repos/a/b/issues/1")))
	require.True(t, probe.rejects("/repos/a/b/pulls/1", slashIndexOf("/repos/a/b/pulls/1")))
	// the constant is compared in full, so a shorter tail cannot pass
	require.True(t, probe.rejects("/repos/a/b/issues", slashIndexOf("/repos/a/b/issues")))
	require.True(t, probe.rejects("/repos/a/b/issue", slashIndexOf("/repos/a/b/issue")))
	// the slash it needs is not there: getMatch decides
	require.False(t, probe.rejects("/repos/a/b", slashIndexOf("/repos/a/b")))

	// a probe shorter than a word masks the lanes past it
	short := newConstProbe("/x", 1)
	require.False(t, short.rejects("/a/x", slashIndexOf("/a/x")))
	require.False(t, short.rejects("/a/xyz/q", slashIndexOf("/a/xyz/q")))
	require.True(t, short.rejects("/a/y", slashIndexOf("/a/y")))
	require.True(t, short.rejects("/a/", slashIndexOf("/a/")))
}

// Test_wordAt pins the packed-word loads the probe compare reads: every
// offset of a few strings, against the obvious byte loop, including the
// overlapping tail load and the sub-word fallback.
// go test -race -run Test_wordAt
func Test_wordAt(t *testing.T) {
	t.Parallel()
	pack := func(s string) uint64 {
		var w uint64
		for i := 0; i < len(s) && i < 8; i++ {
			w |= uint64(s[i]) << (8 * i)
		}
		return w
	}
	for _, s := range []string{"", "/", "/a", "/abcdef", "/abcdefg", "/abcdefgh", "/abcdefghi", "/repos/a/b/issues/1"} {
		for i := 0; i <= len(s); i++ {
			require.Equal(t, pack(s[i:]), wordAt(s, i), "s=%q i=%d", s, i)
		}
	}
}

// Test_packConst pins the word and mask packConst derives, which both the
// prefix filter and the probes are built from.
// go test -race -run Test_packConst
func Test_packConst(t *testing.T) {
	t.Parallel()
	word, mask := packConst("")
	require.Zero(t, word)
	require.Zero(t, mask)

	word, mask = packConst("/ab")
	require.Equal(t, uint64('/')|uint64('a')<<8|uint64('b')<<16, word)
	require.Equal(t, uint64(0xFFFFFF), mask)

	word, mask = packConst("/abcdefghij")
	require.Equal(t, pathHeadWord("/abcdefg"), word)
	require.Equal(t, ^uint64(0), mask)
}

// legacyFindParamLen is findParamLen as it stood before the slash-stop
// shortcut: a multi-byte compare part was located with a substring search, and
// a non-greedy parameter then rejected when a '/' preceded the hit.
func legacyFindParamLen(s string, segment *routeSegment) int {
	if segment.IsLast {
		return findParamLenForLastSegment(s, segment)
	}

	if segment.Length != 0 && len(s) >= segment.Length {
		return segment.Length
	} else if segment.IsGreedy {
		searchCount := strings.Count(s, segment.ComparePart)
		if searchCount > 1 {
			return findGreedyParamLen(s, searchCount, segment)
		}
	}

	if len(segment.ComparePart) == 1 {
		if constPosition := strings.IndexByte(s, segment.ComparePart[0]); constPosition != -1 {
			return constPosition
		}
	} else if constPosition := strings.Index(s, segment.ComparePart); constPosition != -1 {
		if !segment.IsGreedy && strings.IndexByte(s[:constPosition], slashDelimiter) != -1 {
			return 0
		}
		return constPosition
	}

	return len(s)
}

// legacyGetMatch is getMatch's generic segment walk with legacyFindParamLen in
// place of findParamLen and without the "/const/:param" specialization.
//
//nolint:revive // flag-parameter: mirrors getMatch's signature
func legacyGetMatch(parser *routeParser, detectionPath, path string, params *[maxParams]string, partialCheck bool) bool {
	originalDetectionPath := detectionPath
	var i, paramsIterator, partLen, offset int
	for _, segment := range parser.segs {
		partLen = len(detectionPath)
		if !segment.IsParam {
			i = segment.Length
			if segment.HasOptionalSlash && partLen == i-1 && detectionPath == segment.Const[:i-1] {
				i--
			} else if uint(i) > uint(len(detectionPath)) || detectionPath[:i] != segment.Const {
				return false
			}
		} else {
			i = legacyFindParamLen(detectionPath, segment)
			if !segment.IsOptional && i == 0 {
				return false
			}
			params[paramsIterator] = path[offset : offset+i]
			if !segment.IsOptional || i != 0 {
				for _, c := range segment.Constraints {
					if !c.matchConstraint(params[paramsIterator]) {
						return false
					}
				}
			}
			paramsIterator++
		}
		if partLen > 0 {
			detectionPath = detectionPath[i:]
			offset += i
		}
	}
	if detectionPath != "" {
		if !partialCheck {
			return false
		}
		if !hasPartialMatchBoundary(originalDetectionPath, len(originalDetectionPath)-len(detectionPath)) {
			return false
		}
	}
	return true
}

// Test_Path_FindParamLen_SlashStopDifferential proves the slash-stop shortcut
// in findParamLen is transparent: for every generated pattern and path, the
// current getMatch and a copy of the segment walk running the substring search
// it replaced agree on the outcome and the captured parameters. The shortcut
// answers 0 where the search answered len(s) for an absent compare part, so
// the equivalence holds at the match level, which is what is pinned here.
// go test -race -run Test_Path_FindParamLen_SlashStopDifferential
func Test_Path_FindParamLen_SlashStopDifferential(t *testing.T) {
	t.Parallel()

	segments := []string{
		"/api", "/foo/", "/:a", "/:b?", "/*", "/+", "/:a-:b", "/:f.:e?",
		"/::c", "/:x:y", "/:p/fixed", "/:p/keys/:k", "/:p/verylongconstantsegment",
		"/:q/a/", "/:r/.json", "/:s<int>/x", "/:o?/keys", "/*/keys", "/:m/k/:n/k",
	}
	patterns := make([]string, 0, len(segments)*(len(segments)+1))
	for _, s1 := range segments {
		patterns = append(patterns, s1)
		for _, s2 := range segments {
			patterns = append(patterns, s1+s2)
		}
	}

	pieces := []string{
		"", "/a", "/a/b", "/a/fixed", "/a/b/fixed", "/fixed", "/fixed/a",
		"/a/keys/1", "/a/b/keys/1", "/a/keys", "/a/keysx", "/a/xkeys",
		"/a/verylongconstantsegment", "/a/verylongconstantsegmentx",
		"/a-b", "/a.b", "/a/.json", "/a/b/.json", "/1/x", "/1/x/", "/a/",
		"//a", "/a//fixed", "/x/y/fixed/z", "/k/k/k", "/a/k/b/k",
	}
	paths := make([]string, 0, len(pieces)*len(pieces))
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			paths = append(paths, p1+p2)
		}
	}

	for _, pattern := range patterns {
		parser := parseRoute(pattern, regexp.MustCompile)
		if len(parser.params) == 0 {
			continue
		}
		for _, path := range paths {
			for _, partialCheck := range []bool{false, true} {
				var got, want [maxParams]string
				gotOK := parser.getMatch(path, path, &got, partialCheck)
				wantOK := legacyGetMatch(&parser, path, path, &want, partialCheck)
				require.Equal(t, wantOK, gotOK,
					"outcome diverged: pattern %q, path %q, partialCheck %v", pattern, path, partialCheck)
				if wantOK {
					require.Equal(t, want[:len(parser.params)], got[:len(parser.params)],
						"params diverged: pattern %q, path %q, partialCheck %v", pattern, path, partialCheck)
				}
			}
		}
	}
}

// Test_RouteParser_ConstParamDifferential proves matchConstParam is a pure
// rewrite of the generic segment walk: for every specialized pattern and every
// path, the specialized and generic matchers must agree on both the outcome
// and the captured parameter. Running the two against each other is the whole
// safety argument for the specialization.
// go test -race -run Test_RouteParser_ConstParamDifferential
func Test_RouteParser_ConstParamDifferential(t *testing.T) {
	t.Parallel()

	prefixes := []string{"/", "/a/", "/user/keys/", "/api/v1/", "/verylongprefix/"}
	patterns := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		patterns = append(patterns, p+":id")
	}

	pieces := []string{
		"", "/", "/a", "/a/", "/a/b", "/a/b/c", "/user", "/user/",
		"/user/keys", "/user/keys/", "/user/keys/1337", "/user/keys/1337/x",
		"/api", "/api/v1", "/api/v1/", "/api/v1/x", "/api/v1/x/y",
		"/verylongprefix", "/verylongprefix/z", "/VERYLONGPREFIX/z",
	}
	paths := make([]string, 0, len(pieces)*len(pieces))
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			paths = append(paths, p1+p2)
		}
	}

	for _, pattern := range patterns {
		parser := parseRoute(pattern, regexp.MustCompile)
		require.True(t, parser.constParam, "pattern %q lost its specialization", pattern)

		// Same parser with the specialization switched off: segs is shared and
		// read-only, so this exercises the generic walk over identical data.
		fallback := parser
		fallback.constParam = false

		for _, path := range paths {
			for _, partialCheck := range []bool{false, true} {
				var got, want [maxParams]string
				gotOK := parser.getMatch(path, path, &got, partialCheck)
				wantOK := fallback.getMatch(path, path, &want, partialCheck)
				require.Equal(t, wantOK, gotOK,
					"outcome diverged: pattern %q, path %q, partialCheck %v", pattern, path, partialCheck)
				if wantOK {
					require.Equal(t, want[0], got[0],
						"param diverged: pattern %q, path %q, partialCheck %v", pattern, path, partialCheck)
				}
			}
		}
	}
}

// Test_RouteParser_ConstParamFixture runs the specialization against the
// exhaustive path-matching fixture, which covers pattern and URL shapes the
// generated set above does not reach.
// go test -race -run Test_RouteParser_ConstParamFixture
func Test_RouteParser_ConstParamFixture(t *testing.T) {
	t.Parallel()

	checked := 0
	for _, testCollection := range routeTestCases {
		parser := parseRoute(testCollection.pattern, regexp.MustCompile)
		if !parser.constParam {
			continue
		}
		fallback := parser
		fallback.constParam = false

		for _, c := range testCollection.testCases {
			var got, want [maxParams]string
			gotOK := parser.getMatch(c.url, c.url, &got, c.partialCheck)
			wantOK := fallback.getMatch(c.url, c.url, &want, c.partialCheck)
			require.Equal(t, wantOK, gotOK, "route: '%s', url: '%s'", testCollection.pattern, c.url)
			if wantOK {
				require.Equal(t, want[0], got[0], "route: '%s', url: '%s'", testCollection.pattern, c.url)
			}
			checked++
		}
	}
	require.NotZero(t, checked, "fixture exercised no specialized routes")
}

// Test_RoutePatternMatch_MatchesRouter is a differential test: RoutePatternMatch
// documents itself as "see logic in (*Route).match and (*App).register", so for
// every pattern/path/config combination it must agree with what the router
// actually does. It caught RoutePatternMatch trimming trailing slashes from the
// pattern but not from the path.
func Test_RoutePatternMatch_MatchesRouter(t *testing.T) {
	t.Parallel()

	patterns := []string{
		"/", "/a", "/a/b", "/:id", "/a/:id", "/a/:id?", "/a/*", "/*", "/+",
		"/a/+", "/:a/:b", "/a-:b", "/a.:b", "/:a?/b", "/api/v1/:id/x",
		"/a/*/b", "/:id<int>", "/:id<minLen(3)>", "/a/:b?/c", "/ab/*",
		// Case-sensitive constraints: these are evaluated against the value
		// getMatch slices out of the untouched path, not the detection path.
		"/:id<regex(^[a-z]+$)>", "/:id<regex(^[A-Z]+$)>", "/:id<regex(^[^a-z]+$)>",
		"/user/:n<regex(^[A-Z][a-z]+$)>",
		// Escaped specials: register strips the escapes before deriving the
		// root/star flags, so the helper has to as well.
		`/\*`, `/\:id`, `/a\-b`,
	}
	paths := []string{
		"/", "/a", "/a/", "/a/b", "/a/b/", "/1", "/a/1", "/a/b/c",
		"/a-b", "/a.b", "/b", "/api/v1/9/x", "/a/x/b", "/abc", "/ab",
		"/a/b/c/d", "//", "/a//b", "/ab/", "/A", "/A/",
		"/ABC", "/Abc", "/user/John", "/a%2Fb", "/a%20b", "/a%41b",
	}
	configs := []Config{
		{},
		{StrictRouting: true},
		{CaseSensitive: true},
		{StrictRouting: true, CaseSensitive: true},
		{UnescapePath: true},
		{UnescapePath: true, CaseSensitive: true},
		{UnescapePath: true, StrictRouting: true},
	}

	for _, cfg := range configs {
		name := fmt.Sprintf("strict=%v/casesensitive=%v/unescape=%v",
			cfg.StrictRouting, cfg.CaseSensitive, cfg.UnescapePath)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, pattern := range patterns {
				for _, path := range paths {
					app := New(cfg)
					matched := false
					app.Get(pattern, func(_ Ctx) error {
						matched = true
						return nil
					})

					_, err := app.Test(httptest.NewRequest(MethodGet, path, http.NoBody))
					require.NoError(t, err)

					require.Equal(t, matched, RoutePatternMatch(path, pattern, cfg),
						"pattern=%q path=%q", pattern, path)
				}
			}
		})
	}
}
