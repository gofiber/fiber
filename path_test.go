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
		minSlashes int
		maxSlashes int
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

// Test_Route_Match_SlashBoundsDifferential generatively proves the slash-count
// quick-reject in Route.match is transparent: for every generated pattern and
// path, the filtered Route.match must agree with a raw getMatch on the same
// input. Unlike the fixture-driven tests this needs no hand-authored
// expectations, so it also binds pattern shapes nobody thought to add to the
// fixture — if findParamLen ever lets a new shape swallow '/', this fails.
// go test -race -run Test_Route_Match_SlashBoundsDifferential
func Test_Route_Match_SlashBoundsDifferential(t *testing.T) {
	t.Parallel()

	segments := []string{
		"/api", "/foo/", "/:a", "/:b?", "/*", "/+", "/:a-:b", "/:f.:e?",
		":tail", "/::c", "/:x:y", "/name\\:verb", "/:p/fixed",
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
				// 0 is the "count unknown" state and must bypass the filter
				for _, pathSlashes := range []int{strings.Count(path, "/"), 0} {
					var filteredParams, rawParams [maxParams]string
					filtered := route.match(path, path, &filteredParams, pathSlashes)
					raw := parser.getMatch(path, path, &rawParams, use)
					if filtered != raw {
						t.Fatalf("filter changed outcome: pattern %q, path %q, use %v, pathSlashes %d: filtered=%v raw=%v",
							pattern, path, use, pathSlashes, filtered, raw)
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
			match := route.match(c.url, c.url, &ctxParams, strings.Count(c.url, "/"))
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
func Test_RoutePatternMatch_InvalidRegexHandlerPanics(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "fiber: Config.RegexHandler must be a non-nil function", func() {
		RoutePatternMatch("/api/123", "/api/:id<regex(\\d+)>", Config{RegexHandler: "invalid"})
	})
}
