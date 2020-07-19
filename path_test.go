// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"fmt"
	"sync/atomic"
	"testing"

	utils "github.com/gofiber/utils"
)

// go test -race -run Test_Path_matchParams
func Test_Path_matchParams(t *testing.T) {
	t.Parallel()
	type testparams struct {
		url          string
		params       []string
		match        bool
		partialCheck bool
	}
	testCase := func(r string, cases []testparams) {
		parser := parseRoute(r)
		for _, c := range cases {
			paramsPos, match := parser.getMatch(c.url, c.partialCheck)
			utils.AssertEqual(t, c.match, match, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			if match && paramsPos != nil {
				utils.AssertEqual(t, c.params, parser.paramsForPos(c.url, paramsPos), fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
			} else {
				utils.AssertEqual(t, true, nil == paramsPos, fmt.Sprintf("route: '%s', url: '%s'", r, c.url))
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
	testCase("/api/v1/:param?", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/optional", params: []string{"optional"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/xyz", params: nil, match: false},
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
		{url: "/ac", params: []string{"a", "", "c"}, match: true},
		{url: "/test", params: nil, match: false},
	})
	testCase("/test:optional?:mandatory/", []testparams{
		{url: "/testo", params: []string{"", "o"}, match: true},
		{url: "/testoaaa", params: []string{"o", "aaa"}, match: true},
		{url: "/test", params: nil, match: false},
	})
	testCase("/test:optional?:optional2?/", []testparams{
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
		{url: "/", params: []string{""}, match: false},
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
		{url: "/api/1", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/", params: nil, match: false},
		{url: "/api/1.", params: []string{"1", "", ""}, match: true},
		{url: "/api/1.2", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1.2.3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/:day-:month?-:year?", []testparams{
		{url: "/api/1", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/", params: nil, match: false},
		{url: "/api/1-", params: []string{"1", "", ""}, match: true},
		{url: "/api/1-/", params: nil, match: false},
		{url: "/api/1-/-", params: nil, match: false},
		{url: "/api/1-2", params: []string{"1", "2", ""}, match: true},
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
	testCase("/api/*/:param/:param2", []testparams{
		{url: "/api/test/abc/1", params: []string{"test", "abc", "1"}, match: true},
		{url: "/api/joker/batman", params: nil, match: false},
		{url: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, match: true},
		{url: "/api/joker/batman/robin/2/1", params: []string{"joker/batman/robin", "2", "1"}, match: true},
		{url: "/api/joker/batman-robin/1", params: []string{"joker", "batman-robin", "1"}, match: true},
		{url: "/api/joker-batman-robin-1", params: nil, match: false},
		{url: "/api", params: nil, match: false},
	})
	testCase("/partialCheck/foo/bar/:param", []testparams{
		{url: "/partialCheck/foo/bar/test", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar/test/test2", params: []string{"test"}, match: true, partialCheck: true},
		{url: "/partialCheck/foo/bar", params: nil, match: false, partialCheck: true},
		{url: "/partiaFoo", params: nil, match: false, partialCheck: true},
	})
	testCase("/api/*/:param/:param2", []testparams{
		{url: "/api/test/abc", params: nil, match: false},
		{url: "/api/joker/batman", params: nil, match: false},
		{url: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, match: true},
		{url: "/api/joker/batman/robin/1/2", params: []string{"joker/batman/robin", "1", "2"}, match: true},
		{url: "/api", params: nil, match: false},
		{url: "/api/:test", params: nil, match: false},
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
		{url: "/config/efg.csv", params: nil, match: false},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/xyz", []testparams{
		{url: "xyz", params: nil, match: false},
		{url: "xyz/", params: nil, match: false},
	})
}

// go test -race -run Test_Reset_StartParamPosList
func Test_Reset_StartParamPosList(t *testing.T) {
	atomic.StoreUint32(&startParamPosList, uint32(len(paramsPosDummy))-10)

	getAllocFreeParamsPos(5)

	utils.AssertEqual(t, uint32(5), startParamPosList)
}

// go test -race -run Test_Reset_startParamList
func Test_Reset_startParamList(t *testing.T) {
	atomic.StoreUint32(&startParamList, uint32(len(paramsDummy))-10)

	getAllocFreeParams(5)

	utils.AssertEqual(t, uint32(5), startParamList)
}
