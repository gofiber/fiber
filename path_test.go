// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"strings"
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
	type testparams struct {
		url          string
		params       []string
		match        bool
		partialCheck bool
	}
	var ctxParams [maxParams]string
	testCase := func(r string, cases []testparams) {
		parser := parseRoute(r)
		for _, c := range cases {
			match := parser.getMatch(c.url, c.url, &ctxParams, c.partialCheck)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			if match && len(c.params) > 0 {
				utils.AssertEqual(t, c.params[0:len(c.params)], ctxParams[0:len(c.params)], fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			}
		}
	}
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/1", params: []string{"entity", "1"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:param/+", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/entity/", params: nil, match: false},
		{url: "/api/v1/entity/1", params: []string{"entity", "1"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:param?", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/optional", params: []string{"optional"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/xyz", params: nil, match: false},
	})
	testCase("/v1/some/resource/name\\:customVerb", []testparams{
		{url: "/v1/some/resource/name:customVerb", params: nil, match: true},
		{url: "/v1/some/resource/name:test", params: nil, match: false},
	})
	testCase("/v1/some/resource/:name\\:customVerb", []testparams{
		{url: "/v1/some/resource/test:customVerb", params: []string{"test"}, match: true},
		{url: "/v1/some/resource/test:test", params: nil, match: false},
	})
	testCase("/v1/some/resource/name\\\\:customVerb?\\?/:param/*", []testparams{
		{url: "/v1/some/resource/name:customVerb??/test/optionalWildCard/character", params: []string{"test", "optionalWildCard/character"}, match: true},
		{url: "/v1/some/resource/name:customVerb??/test", params: []string{"test", ""}, match: true},
	})
	testCase("/api/v1/*", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/1/2", params: []string{"entity/1/2"}, match: true},
		{url: "/api/v1/Entity/1/2", params: []string{"Entity/1/2"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/abc", params: nil, match: false},
	})
	testCase("/api/v1/:param", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/8728382", params: nil, match: false},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:param-:param2", []testparams{
		{url: "/api/v1/entity-entity2", params: []string{"entity", "entity2"}, match: true},
		{url: "/api/v1/entity/8728382", params: nil, match: false},
		{url: "/api/v1/entity-8728382", params: []string{"entity", "8728382"}, match: true},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:filename.:extension", []testparams{
		{url: "/api/v1/test.pdf", params: []string{"test", "pdf"}, match: true},
		{url: "/api/v1/test/pdf", params: nil, match: false},
		{url: "/api/v1/test-pdf", params: nil, match: false},
		{url: "/api/v1/test_pdf", params: nil, match: false},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/const", []testparams{
		{url: "/api/v1/const", params: []string{}, match: true},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
		{url: "/api/v1/something", params: nil, match: false},
	})
	testCase("/api/:param/fixedEnd", []testparams{
		{url: "/api/abc/fixedEnd", params: []string{"abc"}, match: true},
		{url: "/api/abc/def/fixedEnd", params: nil, match: false},
	})
	testCase("/shop/product/::filter/color::color/size::size", []testparams{
		{url: "/shop/product/:test/color:blue/size:xs", params: []string{"test", "blue", "xs"}, match: true},
		{url: "/shop/product/test/color:blue/size:xs", params: nil, match: false},
	})
	testCase("/::param?", []testparams{
		{url: "/:hello", params: []string{"hello"}, match: true},
		{url: "/:", params: []string{""}, match: true},
		{url: "/", params: nil, match: false},
	})
	// successive parameters, each take one character and the last parameter gets everything
	testCase("/test:sign:param", []testparams{
		{url: "/test-abc", params: []string{"-", "abc"}, match: true},
		{url: "/test", params: nil, match: false},
	})
	// optional parameters are not greedy
	testCase("/:param1:param2?:param3", []testparams{
		{url: "/abbbc", params: []string{"a", "b", "bbc"}, match: true},
		// {url: "/ac", params: []string{"a", "", "c"}, match: true}, // TODO: fix it
		{url: "/test", params: []string{"t", "e", "st"}, match: true},
	})
	testCase("/test:optional?:mandatory", []testparams{
		// {url: "/testo", params: []string{"", "o"}, match: true}, // TODO: fix it
		{url: "/testoaaa", params: []string{"o", "aaa"}, match: true},
		{url: "/test", params: nil, match: false},
	})
	testCase("/test:optional?:optional2?", []testparams{
		{url: "/testo", params: []string{"o", ""}, match: true},
		{url: "/testoaaa", params: []string{"o", "aaa"}, match: true},
		{url: "/test", params: []string{"", ""}, match: true},
		{url: "/tes", params: nil, match: false},
	})
	testCase("/foo:param?bar", []testparams{
		{url: "/foofaselbar", params: []string{"fasel"}, match: true},
		{url: "/foobar", params: []string{""}, match: true},
		{url: "/fooba", params: nil, match: false},
		{url: "/fobar", params: nil, match: false},
	})
	testCase("/foo*bar", []testparams{
		{url: "/foofaselbar", params: []string{"fasel"}, match: true},
		{url: "/foobar", params: []string{""}, match: true},
		{url: "/", params: nil, match: false},
	})
	testCase("/foo+bar", []testparams{
		{url: "/foofaselbar", params: []string{"fasel"}, match: true},
		{url: "/foobar", params: nil, match: false},
		{url: "/", params: nil, match: false},
	})
	testCase("/a*cde*g/", []testparams{
		{url: "/abbbcdefffg", params: []string{"bbb", "fff"}, match: true},
		{url: "/acdeg", params: []string{"", ""}, match: true},
		{url: "/", params: nil, match: false},
	})
	testCase("/*v1*/proxy", []testparams{
		{url: "/customer/v1/cart/proxy", params: []string{"customer/", "/cart"}, match: true},
		{url: "/v1/proxy", params: []string{"", ""}, match: true},
		{url: "/v1/", params: nil, match: false},
	})
	// successive wildcard -> first wildcard is greedy
	testCase("/foo***bar", []testparams{
		{url: "/foo*abar", params: []string{"*a", "", ""}, match: true},
		{url: "/foo*bar", params: []string{"*", "", ""}, match: true},
		{url: "/foobar", params: []string{"", "", ""}, match: true},
		{url: "/fooba", params: nil, match: false},
	})
	// chars in front of an parameter
	testCase("/name::name", []testparams{
		{url: "/name:john", params: []string{"john"}, match: true},
	})
	testCase("/@:name", []testparams{
		{url: "/@john", params: []string{"john"}, match: true},
	})
	testCase("/-:name", []testparams{
		{url: "/-john", params: []string{"john"}, match: true},
	})
	testCase("/.:name", []testparams{
		{url: "/.john", params: []string{"john"}, match: true},
	})
	testCase("/api/v1/:param/abc/*", []testparams{
		{url: "/api/v1/well/abc/wildcard", params: []string{"well", "wildcard"}, match: true},
		{url: "/api/v1/well/abc/", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/abc", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/ttt", params: nil, match: false},
	})
	testCase("/api/:day/:month?/:year?", []testparams{
		{url: "/api/1", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/", params: []string{"1", "", ""}, match: true},
		{url: "/api/1//", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/-/", params: []string{"1", "-", ""}, match: true},
		{url: "/api/1-", params: []string{"1-", "", ""}, match: true},
		{url: "/api/1.", params: []string{"1.", "", ""}, match: true},
		{url: "/api/1/2", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1/2/3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/:day.:month?.:year?", []testparams{
		{url: "/api/1", params: nil, match: false},
		{url: "/api/1/", params: nil, match: false},
		{url: "/api/1.", params: nil, match: false},
		{url: "/api/1..", params: []string{"1", "", ""}, match: true},
		{url: "/api/1.2", params: nil, match: false},
		{url: "/api/1.2.", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1.2.3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/:day-:month?-:year?", []testparams{
		{url: "/api/1", params: nil, match: false},
		{url: "/api/1/", params: nil, match: false},
		{url: "/api/1-", params: nil, match: false},
		{url: "/api/1--", params: []string{"1", "", ""}, match: true},
		{url: "/api/1-/", params: nil, match: false},
		// {url: "/api/1-/-", params: nil, match: false}, // TODO: fix this part
		{url: "/api/1-2", params: nil, match: false},
		{url: "/api/1-2-", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1-2-3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/*", []testparams{
		{url: "/api/", params: []string{""}, match: true},
		{url: "/api/joker", params: []string{"joker"}, match: true},
		{url: "/api", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"v1/entity"}, match: true},
		{url: "/api2/v1/entity", params: nil, match: false},
		{url: "/api_ignore/v1/entity", params: nil, match: false},
	})
	testCase("/partialCheck/foo/bar/:param", []testparams{
		{url: "/partialCheck/foo/bar/test", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar/test/test2", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar", params: nil, match: false, partialCheck: true},
		{url: "/partiaFoo", params: nil, match: false, partialCheck: true},
	})
	testCase("/", []testparams{
		{url: "/api", params: nil, match: false},
		{url: "", params: []string{}, match: true},
		{url: "/", params: []string{}, match: true},
	})
	testCase("/config/abc.json", []testparams{
		{url: "/config/abc.json", params: []string{}, match: true},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config/efg.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/config/*.json", []testparams{
		{url: "/config/abc.json", params: []string{"abc"}, match: true},
		{url: "/config/efg.json", params: []string{"efg"}, match: true},
		{url: "/config/.json", params: []string{""}, match: true},
		{url: "/config/efg.csv", params: nil, match: false},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/config/+.json", []testparams{
		{url: "/config/abc.json", params: []string{"abc"}, match: true},
		{url: "/config/.json", params: nil, match: false},
		{url: "/config/efg.json", params: []string{"efg"}, match: true},
		{url: "/config/efg.csv", params: nil, match: false},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/xyz", []testparams{
		{url: "xyz", params: nil, match: false},
		{url: "xyz/", params: nil, match: false},
	})
	testCase("/api/*/:param?", []testparams{
		{url: "/api/", params: []string{"", ""}, match: true},
		{url: "/api/joker", params: []string{"joker", ""}, match: true},
		{url: "/api/joker/batman", params: []string{"joker", "batman"}, match: true},
		{url: "/api/joker//batman", params: []string{"joker/", "batman"}, match: true},
		{url: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api/joker/batman/robin/1/", params: []string{"joker/batman/robin/1", ""}, match: true},
		{url: "/api/joker-batman/robin/1", params: []string{"joker-batman/robin", "1"}, match: true},
		{url: "/api/joker-batman-robin/1", params: []string{"joker-batman-robin", "1"}, match: true},
		{url: "/api/joker-batman-robin-1", params: []string{"joker-batman-robin-1", ""}, match: true},
		{url: "/api", params: []string{"", ""}, match: true},
	})
	testCase("/api/*/:param", []testparams{
		{url: "/api/test/abc", params: []string{"test", "abc"}, match: true},
		{url: "/api/joker/batman", params: []string{"joker", "batman"}, match: true},
		{url: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api/joker/batman-robin/1", params: []string{"joker/batman-robin", "1"}, match: true},
		{url: "/api/joker-batman-robin-1", params: nil, match: false},
		{url: "/api", params: nil, match: false},
	})
	testCase("/api/+/:param", []testparams{
		{url: "/api/test/abc", params: []string{"test", "abc"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api/joker", params: nil, match: false},
		{url: "/api", params: nil, match: false},
	})
	testCase("/api/*/:param/:param2", []testparams{
		{url: "/api/test/abc/1", params: []string{"test", "abc", "1"}, match: true},
		{url: "/api/joker/batman", params: nil, match: false},
		{url: "/api/joker/batman-robin/1", params: []string{"joker", "batman-robin", "1"}, match: true},
		{url: "/api/joker-batman-robin-1", params: nil, match: false},
		{url: "/api/test/abc", params: nil, match: false},
		{url: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, match: true},
		{url: "/api/joker/batman/robin/1/2", params: []string{"joker/batman/robin", "1", "2"}, match: true},
		{url: "/api", params: nil, match: false},
		{url: "/api/:test", params: nil, match: false},
	})
	testCase("/api/v1/:param<int>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<bool>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/true", params: []string{"true"}, match: true},
	})
	testCase("/api/v1/:param<float>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/8728382.5", params: []string{"8728382.5"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<alpha>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/#!?", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
	})
	testCase("/api/v1/:param<guid>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/f0fa66cc-d22e-445b-866d-1d76e776371d", params: []string{"f0fa66cc-d22e-445b-866d-1d76e776371d"}, match: true},
	})
	testCase("/api/v1/:param<minLen>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
	})
	testCase("/api/v1/:param<minLen(5)>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/123", params: nil, match: false},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	testCase("/api/v1/:param<maxLen(5)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/ent", params: []string{"ent"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	testCase("/api/v1/:param<len(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/123", params: nil, match: false},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	testCase("/api/v1/:param<betweenLen(1)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/ent", params: nil, match: false},
	})
	testCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", params: nil, match: false},
		{url: "/api/v1/en", params: []string{"en"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	testCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", params: nil, match: false},
		{url: "/api/v1/en", params: []string{"en"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	testCase("/api/v1/:param<min(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/1", params: nil, match: false},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
	})
	testCase("/api/v1/:param<max(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/1", params: []string{"1"}, match: true},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
		{url: "/api/v1/15", params: nil, match: false},
	})
	testCase("/api/v1/:param<range(5,10)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/9", params: []string{"9"}, match: true},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
		{url: "/api/v1/15", params: nil, match: false},
	})
	testCase("/api/v1/:param<datetime(2006\\-01\\-02)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/2005-11-01", params: []string{"2005-11-01"}, match: true},
	})
	testCase("/api/v1/:param<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/15", params: nil, match: false},
		{url: "/api/v1/peach", params: []string{"peach"}, match: true},
		{url: "/api/v1/p34ch", params: nil, match: false},
	})
	testCase("/api/v1/:param<regex(^[a-z0-9]([a-z0-9-]{1,61}[a-z0-9])?$)>", []testparams{
		{url: "/api/v1/12", params: nil, match: false},
		{url: "/api/v1/xy", params: nil, match: false},
		{url: "/api/v1/test", params: []string{"test"}, match: true},
		{url: "/api/v1/" + strings.Repeat("a", 64), params: nil, match: false},
	})
	testCase("/api/v1/:param<regex(\\d{4}-\\d{2}-\\d{2})}>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/15", params: nil, match: false},
		{url: "/api/v1/2022-08-27", params: []string{"2022-08-27"}, match: true},
		{url: "/api/v1/2022/08-27", params: nil, match: false},
	})
	testCase("/api/v1/:param<int;bool((>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<int;max(3000)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<int;maxLen(10)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<int;range(10,30)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:param<int\\;range(10,30)>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/87283827683", params: []string{"87283827683"}, match: true},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/true", params: []string{"true"}, match: true},
	})
	testCase("/api/v1/:param<range(10\\,30,1500)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/1200", params: []string{"1200"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	testCase("/api/v1/:lang<len(2)>/videos/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/videos/200", params: nil, match: false},
		{url: "/api/v1/tr/videos/1800", params: nil, match: false},
		{url: "/api/v1/tr/videos/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/videos/10", params: nil, match: false},
	})
	testCase("/api/v1/:lang<len(2)>/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", params: nil, match: false},
		{url: "/api/v1/tr/1800", params: nil, match: false},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	testCase("/api/v1/:lang/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", params: []string{"try", "200"}, match: true},
		{url: "/api/v1/tr/1800", params: nil, match: false},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	testCase("/api/v1/:lang<len(2)>/:page", []testparams{
		{url: "/api/v1/try/200", params: nil, match: false},
		{url: "/api/v1/tr/1800", params: []string{"tr", "1800"}, match: true},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	testCase("/api/v1/:date<datetime(2006\\-01\\-02)>/:regex<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/2005-11-01/a", params: nil, match: false},
		{url: "/api/v1/2005-1101/paach", params: nil, match: false},
		{url: "/api/v1/2005-11-01/peach", params: []string{"2005-11-01", "peach"}, match: true},
	})
	testCase("/api/v1/:param<int>?", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
		{url: "/api/v1/", params: []string{""}, match: true},
	})
}

// go test -race -run Test_RoutePatternMatch
func Test_RoutePatternMatch(t *testing.T) {
	t.Parallel()
	type testparams struct {
		url   string
		match bool
	}
	testCase := func(pattern string, cases []testparams) {
		for _, c := range cases {
			match := RoutePatternMatch(c.url, pattern)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", pattern, c.url))
		}
	}
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/", match: true},
		{url: "/api/v1/entity/1", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/v1/", match: false},
	})
	testCase("/api/v1/:param/+", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/entity/", match: false},
		{url: "/api/v1/entity/1", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/v1/", match: false},
	})
	testCase("/api/v1/:param?", []testparams{
		{url: "/api/v1", match: true},
		{url: "/api/v1/", match: true},
		{url: "/api/v1/optional", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/xyz", match: false},
	})
	testCase("/v1/some/resource/name\\:customVerb", []testparams{
		{url: "/v1/some/resource/name:customVerb", match: true},
		{url: "/v1/some/resource/name:test", match: false},
	})
	testCase("/v1/some/resource/:name\\:customVerb", []testparams{
		{url: "/v1/some/resource/test:customVerb", match: true},
		{url: "/v1/some/resource/test:test", match: false},
	})
	testCase("/v1/some/resource/name\\\\:customVerb?\\?/:param/*", []testparams{
		{url: "/v1/some/resource/name:customVerb??/test/optionalWildCard/character", match: true},
		{url: "/v1/some/resource/name:customVerb??/test", match: true},
	})
	testCase("/api/v1/*", []testparams{
		{url: "/api/v1", match: true},
		{url: "/api/v1/", match: true},
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/1/2", match: true},
		{url: "/api/v1/Entity/1/2", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/abc", match: false},
	})
	testCase("/api/v1/:param", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/8728382", match: false},
		{url: "/api/v1", match: false},
		{url: "/api/v1/", match: false},
	})
	testCase("/api/v1/:param-:param2", []testparams{
		{url: "/api/v1/entity-entity2", match: true},
		{url: "/api/v1/entity/8728382", match: false},
		{url: "/api/v1/entity-8728382", match: true},
		{url: "/api/v1", match: false},
		{url: "/api/v1/", match: false},
	})
	testCase("/api/v1/:filename.:extension", []testparams{
		{url: "/api/v1/test.pdf", match: true},
		{url: "/api/v1/test/pdf", match: false},
		{url: "/api/v1/test-pdf", match: false},
		{url: "/api/v1/test_pdf", match: false},
		{url: "/api/v1", match: false},
		{url: "/api/v1/", match: false},
	})
	testCase("/api/v1/const", []testparams{
		{url: "/api/v1/const", match: true},
		{url: "/api/v1", match: false},
		{url: "/api/v1/", match: false},
		{url: "/api/v1/something", match: false},
	})
	testCase("/api/:param/fixedEnd", []testparams{
		{url: "/api/abc/fixedEnd", match: true},
		{url: "/api/abc/def/fixedEnd", match: false},
	})
	testCase("/shop/product/::filter/color::color/size::size", []testparams{
		{url: "/shop/product/:test/color:blue/size:xs", match: true},
		{url: "/shop/product/test/color:blue/size:xs", match: false},
	})
	testCase("/::param?", []testparams{
		{url: "/:hello", match: true},
		{url: "/:", match: true},
		{url: "/", match: false},
	})
	// successive parameters, each take one character and the last parameter gets everything
	testCase("/test:sign:param", []testparams{
		{url: "/test-abc", match: true},
		{url: "/test", match: false},
	})
	// optional parameters are not greedy
	testCase("/:param1:param2?:param3", []testparams{
		{url: "/abbbc", match: true},
		// {url: "/ac", params: []string{"a", "", "c"}, match: true}, // TODO: fix it
		{url: "/test", match: true},
	})
	testCase("/test:optional?:mandatory", []testparams{
		// {url: "/testo", params: []string{"", "o"}, match: true}, // TODO: fix it
		{url: "/testoaaa", match: true},
		{url: "/test", match: false},
	})
	testCase("/test:optional?:optional2?", []testparams{
		{url: "/testo", match: true},
		{url: "/testoaaa", match: true},
		{url: "/test", match: true},
		{url: "/tes", match: false},
	})
	testCase("/foo:param?bar", []testparams{
		{url: "/foofaselbar", match: true},
		{url: "/foobar", match: true},
		{url: "/fooba", match: false},
		{url: "/fobar", match: false},
	})
	testCase("/foo*bar", []testparams{
		{url: "/foofaselbar", match: true},
		{url: "/foobar", match: true},
		{url: "/", match: false},
	})
	testCase("/foo+bar", []testparams{
		{url: "/foofaselbar", match: true},
		{url: "/foobar", match: false},
		{url: "/", match: false},
	})
	testCase("/a*cde*g/", []testparams{
		{url: "/abbbcdefffg", match: true},
		{url: "/acdeg", match: true},
		{url: "/", match: false},
	})
	testCase("/*v1*/proxy", []testparams{
		{url: "/customer/v1/cart/proxy", match: true},
		{url: "/v1/proxy", match: true},
		{url: "/v1/", match: false},
	})
	// successive wildcard -> first wildcard is greedy
	testCase("/foo***bar", []testparams{
		{url: "/foo*abar", match: true},
		{url: "/foo*bar", match: true},
		{url: "/foobar", match: true},
		{url: "/fooba", match: false},
	})
	// chars in front of an parameter
	testCase("/name::name", []testparams{
		{url: "/name:john", match: true},
	})
	testCase("/@:name", []testparams{
		{url: "/@john", match: true},
	})
	testCase("/-:name", []testparams{
		{url: "/-john", match: true},
	})
	testCase("/.:name", []testparams{
		{url: "/.john", match: true},
	})
	testCase("/api/v1/:param/abc/*", []testparams{
		{url: "/api/v1/well/abc/wildcard", match: true},
		{url: "/api/v1/well/abc/", match: true},
		{url: "/api/v1/well/abc", match: true},
		{url: "/api/v1/well/ttt", match: false},
	})
	testCase("/api/:day/:month?/:year?", []testparams{
		{url: "/api/1", match: true},
		{url: "/api/1/", match: true},
		{url: "/api/1//", match: true},
		{url: "/api/1/-/", match: true},
		{url: "/api/1-", match: true},
		{url: "/api/1.", match: true},
		{url: "/api/1/2", match: true},
		{url: "/api/1/2/3", match: true},
		{url: "/api/", match: false},
	})
	testCase("/api/:day.:month?.:year?", []testparams{
		{url: "/api/1", match: false},
		{url: "/api/1/", match: false},
		{url: "/api/1.", match: false},
		{url: "/api/1..", match: true},
		{url: "/api/1.2", match: false},
		{url: "/api/1.2.", match: true},
		{url: "/api/1.2.3", match: true},
		{url: "/api/", match: false},
	})
	testCase("/api/:day-:month?-:year?", []testparams{
		{url: "/api/1", match: false},
		{url: "/api/1/", match: false},
		{url: "/api/1-", match: false},
		{url: "/api/1--", match: true},
		{url: "/api/1-/", match: false},
		// {url: "/api/1-/-", params: nil, match: false}, // TODO: fix this part
		{url: "/api/1-2", match: false},
		{url: "/api/1-2-", match: true},
		{url: "/api/1-2-3", match: true},
		{url: "/api/", match: false},
	})
	testCase("/api/*", []testparams{
		{url: "/api/", match: true},
		{url: "/api/joker", match: true},
		{url: "/api", match: true},
		{url: "/api/v1/entity", match: true},
		{url: "/api2/v1/entity", match: false},
		{url: "/api_ignore/v1/entity", match: false},
	})
	testCase("/", []testparams{
		{url: "/api", match: false},
		{url: "", match: true},
		{url: "/", match: true},
	})
	testCase("/config/abc.json", []testparams{
		{url: "/config/abc.json", match: true},
		{url: "config/abc.json", match: false},
		{url: "/config/efg.json", match: false},
		{url: "/config", match: false},
	})
	testCase("/config/*.json", []testparams{
		{url: "/config/abc.json", match: true},
		{url: "/config/efg.json", match: true},
		{url: "/config/.json", match: true},
		{url: "/config/efg.csv", match: false},
		{url: "config/abc.json", match: false},
		{url: "/config", match: false},
	})
	testCase("/config/+.json", []testparams{
		{url: "/config/abc.json", match: true},
		{url: "/config/.json", match: false},
		{url: "/config/efg.json", match: true},
		{url: "/config/efg.csv", match: false},
		{url: "config/abc.json", match: false},
		{url: "/config", match: false},
	})
	testCase("/xyz", []testparams{
		{url: "xyz", match: false},
		{url: "xyz/", match: false},
	})
	testCase("/api/*/:param?", []testparams{
		{url: "/api/", match: true},
		{url: "/api/joker", match: true},
		{url: "/api/joker/batman", match: true},
		{url: "/api/joker//batman", match: true},
		{url: "/api/joker/batman/robin", match: true},
		{url: "/api/joker/batman/robin/1", match: true},
		{url: "/api/joker/batman/robin/1/", match: true},
		{url: "/api/joker-batman/robin/1", match: true},
		{url: "/api/joker-batman-robin/1", match: true},
		{url: "/api/joker-batman-robin-1", match: true},
		{url: "/api", match: true},
	})
	testCase("/api/*/:param", []testparams{
		{url: "/api/test/abc", match: true},
		{url: "/api/joker/batman", match: true},
		{url: "/api/joker/batman/robin", match: true},
		{url: "/api/joker/batman/robin/1", match: true},
		{url: "/api/joker/batman-robin/1", match: true},
		{url: "/api/joker-batman-robin-1", match: false},
		{url: "/api", match: false},
	})
	testCase("/api/+/:param", []testparams{
		{url: "/api/test/abc", match: true},
		{url: "/api/joker/batman/robin/1", match: true},
		{url: "/api/joker", match: false},
		{url: "/api", match: false},
	})
	testCase("/api/*/:param/:param2", []testparams{
		{url: "/api/test/abc/1", match: true},
		{url: "/api/joker/batman", match: false},
		{url: "/api/joker/batman-robin/1", match: true},
		{url: "/api/joker-batman-robin-1", match: false},
		{url: "/api/test/abc", match: false},
		{url: "/api/joker/batman/robin", match: true},
		{url: "/api/joker/batman/robin/1", match: true},
		{url: "/api/joker/batman/robin/1/2", match: true},
		{url: "/api", match: false},
		{url: "/api/:test", match: false},
	})
	testCase("/api/v1/:param<int>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<bool>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/true", match: true},
	})
	testCase("/api/v1/:param<float>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/8728382.5", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<alpha>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/#!?", match: false},
		{url: "/api/v1/8728382", match: false},
	})
	testCase("/api/v1/:param<guid>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/f0fa66cc-d22e-445b-866d-1d76e776371d", match: true},
	})
	testCase("/api/v1/:param<minLen>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
	})
	testCase("/api/v1/:param<minLen(5)>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/123", match: false},
		{url: "/api/v1/12345", match: true},
	})
	testCase("/api/v1/:param<maxLen(5)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/ent", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	testCase("/api/v1/:param<len(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/123", match: false},
		{url: "/api/v1/12345", match: true},
	})
	testCase("/api/v1/:param<betweenLen(1)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/ent", match: false},
	})
	testCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", match: false},
		{url: "/api/v1/en", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	testCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", match: false},
		{url: "/api/v1/en", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	testCase("/api/v1/:param<min(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/1", match: false},
		{url: "/api/v1/5", match: true},
	})
	testCase("/api/v1/:param<max(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/1", match: true},
		{url: "/api/v1/5", match: true},
		{url: "/api/v1/15", match: false},
	})
	testCase("/api/v1/:param<range(5,10)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/9", match: true},
		{url: "/api/v1/5", match: true},
		{url: "/api/v1/15", match: false},
	})
	testCase("/api/v1/:param<datetime(2006\\-01\\-02)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/2005-11-01", match: true},
	})
	testCase("/api/v1/:param<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/15", match: false},
		{url: "/api/v1/peach", match: true},
		{url: "/api/v1/p34ch", match: false},
	})
	testCase("/api/v1/:param<regex(\\d{4}-\\d{2}-\\d{2})}>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/15", match: false},
		{url: "/api/v1/2022-08-27", match: true},
		{url: "/api/v1/2022/08-27", match: false},
	})
	testCase("/api/v1/:param<int;bool((>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<int;max(3000)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<int;maxLen(10)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<int;range(10,30)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:param<int\\;range(10,30)>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/87283827683", match: true},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/true", match: true},
	})
	testCase("/api/v1/:param<range(10\\,30,1500)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/1200", match: true},
		{url: "/api/v1/true", match: false},
	})
	testCase("/api/v1/:lang<len(2)>/videos/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/videos/200", match: false},
		{url: "/api/v1/tr/videos/1800", match: false},
		{url: "/api/v1/tr/videos/100", match: true},
		{url: "/api/v1/e/videos/10", match: false},
	})
	testCase("/api/v1/:lang<len(2)>/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", match: false},
		{url: "/api/v1/tr/1800", match: false},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	testCase("/api/v1/:lang/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", match: true},
		{url: "/api/v1/tr/1800", match: false},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	testCase("/api/v1/:lang<len(2)>/:page", []testparams{
		{url: "/api/v1/try/200", match: false},
		{url: "/api/v1/tr/1800", match: true},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	testCase("/api/v1/:date<datetime(2006\\-01\\-02)>/:regex<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/2005-11-01/a", match: false},
		{url: "/api/v1/2005-1101/paach", match: false},
		{url: "/api/v1/2005-11-01/peach", match: true},
	})
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

// go test -race -run Test_Path_matchParams
func Benchmark_Path_matchParams(t *testing.B) {
	type testparams struct {
		url          string
		params       []string
		match        bool
		partialCheck bool
	}
	var ctxParams [maxParams]string
	benchCase := func(r string, cases []testparams) {
		parser := parseRoute(r)
		for _, c := range cases {
			var matchRes bool
			state := "match"
			if !c.match {
				state = "not match"
			}
			t.Run(r+" | "+state+" | "+c.url, func(b *testing.B) {
				for i := 0; i <= b.N; i++ {
					if match := parser.getMatch(c.url, c.url, &ctxParams, c.partialCheck); match {
						// Get params from the original path
						matchRes = true
					}
				}
				utils.AssertEqual(t, c.match, matchRes, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
				if matchRes && len(c.params) > 0 {
					utils.AssertEqual(t, c.params[0:len(c.params)-1], ctxParams[0:len(c.params)-1], fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
				}
			})
		}
	}
	benchCase("/api/:param/fixedEnd", []testparams{
		{url: "/api/abc/fixedEnd", params: []string{"abc"}, match: true},
		{url: "/api/abc/def/fixedEnd", params: nil, match: false},
	})
	benchCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/1", params: []string{"entity", "1"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	benchCase("/api/v1/:param", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/8728382", params: nil, match: false},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	benchCase("/api/v1", []testparams{
		{url: "/api/v1", params: []string{}, match: true},
		{url: "/api/v2", params: nil, match: false},
	})
	benchCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/", params: []string{"entity", ""}, match: true},
		{url: "/api/v1/entity/1", params: []string{"entity", "1"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<bool>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/true", params: []string{"true"}, match: true},
	})
	benchCase("/api/v1/:param<float>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/8728382.5", params: []string{"8728382.5"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<alpha>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/#!?", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
	})
	benchCase("/api/v1/:param<guid>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/f0fa66cc-d22e-445b-866d-1d76e776371d", params: []string{"f0fa66cc-d22e-445b-866d-1d76e776371d"}, match: true},
	})
	benchCase("/api/v1/:param<minLen>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
	})
	benchCase("/api/v1/:param<minLen(5)>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/123", params: nil, match: false},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	benchCase("/api/v1/:param<maxLen(5)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/ent", params: []string{"ent"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	benchCase("/api/v1/:param<len(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/123", params: nil, match: false},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	benchCase("/api/v1/:param<betweenLen(1)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/ent", params: nil, match: false},
	})
	benchCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", params: nil, match: false},
		{url: "/api/v1/en", params: []string{"en"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	benchCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", params: nil, match: false},
		{url: "/api/v1/en", params: []string{"en"}, match: true},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/12345", params: []string{"12345"}, match: true},
	})
	benchCase("/api/v1/:param<min(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/1", params: nil, match: false},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
	})
	benchCase("/api/v1/:param<max(5)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/1", params: []string{"1"}, match: true},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
		{url: "/api/v1/15", params: nil, match: false},
	})
	benchCase("/api/v1/:param<range(5,10)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/9", params: []string{"9"}, match: true},
		{url: "/api/v1/5", params: []string{"5"}, match: true},
		{url: "/api/v1/15", params: nil, match: false},
	})
	benchCase("/api/v1/:param<datetime(2006\\-01\\-02)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/2005-11-01", params: []string{"2005-11-01"}, match: true},
	})
	benchCase("/api/v1/:param<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/15", params: nil, match: false},
		{url: "/api/v1/peach", params: []string{"peach"}, match: true},
		{url: "/api/v1/p34ch", params: nil, match: false},
	})
	benchCase("/api/v1/:param<regex(\\d{4}-\\d{2}-\\d{2})}>", []testparams{
		{url: "/api/v1/ent", params: nil, match: false},
		{url: "/api/v1/15", params: nil, match: false},
		{url: "/api/v1/2022-08-27", params: []string{"2022-08-27"}, match: true},
		{url: "/api/v1/2022/08-27", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int;bool((>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int;max(3000)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int;maxLen(10)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/123", params: []string{"123"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int;range(10,30)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:param<int\\;range(10,30)>", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/87283827683", params: []string{"87283827683"}, match: true},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/true", params: []string{"true"}, match: true},
	})
	benchCase("/api/v1/:param<range(10\\,30,1500)>", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/87283827683", params: nil, match: false},
		{url: "/api/v1/25", params: []string{"25"}, match: true},
		{url: "/api/v1/1200", params: []string{"1200"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/videos/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/videos/200", params: nil, match: false},
		{url: "/api/v1/tr/videos/1800", params: nil, match: false},
		{url: "/api/v1/tr/videos/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/videos/10", params: nil, match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", params: nil, match: false},
		{url: "/api/v1/tr/1800", params: nil, match: false},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	benchCase("/api/v1/:lang/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", params: []string{"try", "200"}, match: true},
		{url: "/api/v1/tr/1800", params: nil, match: false},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/:page", []testparams{
		{url: "/api/v1/try/200", params: nil, match: false},
		{url: "/api/v1/tr/1800", params: []string{"tr", "1800"}, match: true},
		{url: "/api/v1/tr/100", params: []string{"tr", "100"}, match: true},
		{url: "/api/v1/e/10", params: nil, match: false},
	})
	benchCase("/api/v1/:date<datetime(2006\\-01\\-02)>/:regex<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/2005-11-01/a", params: nil, match: false},
		{url: "/api/v1/2005-1101/paach", params: nil, match: false},
		{url: "/api/v1/2005-11-01/peach", params: []string{"2005-11-01", "peach"}, match: true},
	})
	benchCase("/api/v1/:param<int>?", []testparams{
		{url: "/api/v1/entity", params: nil, match: false},
		{url: "/api/v1/8728382", params: []string{"8728382"}, match: true},
		{url: "/api/v1/true", params: nil, match: false},
		{url: "/api/v1/", params: []string{""}, match: true},
	})
}

// go test -race -run Test_RoutePatternMatch
func Benchmark_RoutePatternMatch(t *testing.B) {
	type testparams struct {
		url   string
		match bool
	}
	benchCase := func(pattern string, cases []testparams) {
		for _, c := range cases {
			var matchRes bool
			state := "match"
			if !c.match {
				state = "not match"
			}
			t.Run(pattern+" | "+state+" | "+c.url, func(b *testing.B) {
				for i := 0; i <= b.N; i++ {
					if match := RoutePatternMatch(c.url, pattern); match {
						// Get params from the original path
						matchRes = true
					}
				}
				utils.AssertEqual(t, c.match, matchRes, fmt.Sprintf("route: '%s', url: '%s'", pattern, c.url))
			})
		}
	}
	benchCase("/api/:param/fixedEnd", []testparams{
		{url: "/api/abc/fixedEnd", match: true},
		{url: "/api/abc/def/fixedEnd", match: false},
	})
	benchCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/", match: true},
		{url: "/api/v1/entity/1", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/v1/", match: false},
	})
	benchCase("/api/v1/:param", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/8728382", match: false},
		{url: "/api/v1", match: false},
		{url: "/api/v1/", match: false},
	})
	benchCase("/api/v1", []testparams{
		{url: "/api/v1", match: true},
		{url: "/api/v2", match: false},
	})
	benchCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/entity/", match: true},
		{url: "/api/v1/entity/1", match: true},
		{url: "/api/v", match: false},
		{url: "/api/v2", match: false},
		{url: "/api/v1/", match: false},
	})
	benchCase("/api/v1/:param<int>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<bool>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/true", match: true},
	})
	benchCase("/api/v1/:param<float>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/8728382.5", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<alpha>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/#!?", match: false},
		{url: "/api/v1/8728382", match: false},
	})
	benchCase("/api/v1/:param<guid>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/f0fa66cc-d22e-445b-866d-1d76e776371d", match: true},
	})
	benchCase("/api/v1/:param<minLen>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
	})
	benchCase("/api/v1/:param<minLen(5)>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/123", match: false},
		{url: "/api/v1/12345", match: true},
	})
	benchCase("/api/v1/:param<maxLen(5)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/ent", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	benchCase("/api/v1/:param<len(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/123", match: false},
		{url: "/api/v1/12345", match: true},
	})
	benchCase("/api/v1/:param<betweenLen(1)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/ent", match: false},
	})
	benchCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", match: false},
		{url: "/api/v1/en", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	benchCase("/api/v1/:param<betweenLen(2,5)>", []testparams{
		{url: "/api/v1/e", match: false},
		{url: "/api/v1/en", match: true},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/12345", match: true},
	})
	benchCase("/api/v1/:param<min(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/1", match: false},
		{url: "/api/v1/5", match: true},
	})
	benchCase("/api/v1/:param<max(5)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/1", match: true},
		{url: "/api/v1/5", match: true},
		{url: "/api/v1/15", match: false},
	})
	benchCase("/api/v1/:param<range(5,10)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/9", match: true},
		{url: "/api/v1/5", match: true},
		{url: "/api/v1/15", match: false},
	})
	benchCase("/api/v1/:param<datetime(2006\\-01\\-02)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/2005-11-01", match: true},
	})
	benchCase("/api/v1/:param<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/15", match: false},
		{url: "/api/v1/peach", match: true},
		{url: "/api/v1/p34ch", match: false},
	})
	benchCase("/api/v1/:param<regex(\\d{4}-\\d{2}-\\d{2})}>", []testparams{
		{url: "/api/v1/ent", match: false},
		{url: "/api/v1/15", match: false},
		{url: "/api/v1/2022-08-27", match: true},
		{url: "/api/v1/2022/08-27", match: false},
	})
	benchCase("/api/v1/:param<int;bool((>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<int;max(3000)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/8728382", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<int;maxLen(10)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/123", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<int;range(10,30)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:param<int\\;range(10,30)>", []testparams{
		{url: "/api/v1/entity", match: true},
		{url: "/api/v1/87283827683", match: true},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/true", match: true},
	})
	benchCase("/api/v1/:param<range(10\\,30,1500)>", []testparams{
		{url: "/api/v1/entity", match: false},
		{url: "/api/v1/87283827683", match: false},
		{url: "/api/v1/25", match: true},
		{url: "/api/v1/1200", match: true},
		{url: "/api/v1/true", match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/videos/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/videos/200", match: false},
		{url: "/api/v1/tr/videos/1800", match: false},
		{url: "/api/v1/tr/videos/100", match: true},
		{url: "/api/v1/e/videos/10", match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", match: false},
		{url: "/api/v1/tr/1800", match: false},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	benchCase("/api/v1/:lang/:page<range(100,1500)>", []testparams{
		{url: "/api/v1/try/200", match: true},
		{url: "/api/v1/tr/1800", match: false},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	benchCase("/api/v1/:lang<len(2)>/:page", []testparams{
		{url: "/api/v1/try/200", match: false},
		{url: "/api/v1/tr/1800", match: true},
		{url: "/api/v1/tr/100", match: true},
		{url: "/api/v1/e/10", match: false},
	})
	benchCase("/api/v1/:date<datetime(2006\\-01\\-02)>/:regex<regex(p([a-z]+)ch)>", []testparams{
		{url: "/api/v1/2005-11-01/a", match: false},
		{url: "/api/v1/2005-1101/paach", match: false},
		{url: "/api/v1/2005-11-01/peach", match: true},
	})
}
