// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// ⚠️ This path parser was based on urlpath by @ucarion (MIT License).
// 💖 Modified for the Fiber router by @renanbastos93 & @renewerner87
// 🤖 ucarion/urlpath - renanbastos93/fastpath - renewerner87/fastpath

package fiber

import (
	"fmt"
	"reflect"
	"testing"
)

// params testing

type testCase struct {
	uri    string
	params []string
	ok     bool
}

<<<<<<< HEAD
func Test_With_Starting_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/*"),
		[]testCase{
			{uri: "/api/v1/entity", params: []string{"api/v1/entity"}, ok: true},
			{uri: "/api/v1/entity/", params: []string{"api/v1/entity/"}, ok: true},
			{uri: "/api/v1/entity/1", params: []string{"api/v1/entity/1"}, ok: true},
			{uri: "/", params: []string{""}, ok: true},
		},
	)
}

=======
>>>>>>> upstream/master
func Test_With_Param_And_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param/*"),
		[]testCase{
			{uri: "/api/v1/entity", params: []string{"entity", ""}, ok: true},
			{uri: "/api/v1/entity/", params: []string{"entity", ""}, ok: true},
			{uri: "/api/v1/entity/1", params: []string{"entity", "1"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
		},
	)
}

func Test_With_A_Param_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param?"),
		[]testCase{
			{uri: "/api/v1", params: []string{""}, ok: true},
			{uri: "/api/v1/", params: []string{""}, ok: true},
			{uri: "/api/v1/optional", params: []string{"optional"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/xyz", params: nil, ok: false},
		},
	)
}

func Test_With_With_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/*"),
		[]testCase{
			{uri: "/api/v1", params: []string{""}, ok: true},
			{uri: "/api/v1/", params: []string{""}, ok: true},
			{uri: "/api/v1/entity", params: []string{"entity"}, ok: true},
			{uri: "/api/v1/entity/1/2", params: []string{"entity/1/2"}, ok: true},
			{uri: "/api/v", params: nil, ok: false},
			{uri: "/api/v2", params: nil, ok: false},
			{uri: "/api/abc", params: nil, ok: false},
		},
	)
}
func Test_With_With_Param(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param"),
		[]testCase{
			{uri: "/api/v1/entity", params: []string{"entity"}, ok: true},
			{uri: "/api/v1", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
		},
	)
}

func Test_With_Without_A_Param_Or_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/const"),
		[]testCase{
			{uri: "/api/v1/const", params: []string{}, ok: true},
			{uri: "/api/v1", params: nil, ok: false},
			{uri: "/api/v1/", params: nil, ok: false},
			{uri: "/api/v1/something", params: nil, ok: false},
		},
	)
}
func Test_With_With_A_Param_And_Wildcard_Differents_Positions(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/v1/:param/abc/*"),
		[]testCase{
			{uri: "/api/v1/well/abc/wildcard", params: []string{"well", "wildcard"}, ok: true},
			{uri: "/api/v1/well/abc/", params: []string{"well", ""}, ok: true},
			{uri: "/api/v1/well/abc", params: []string{"well", ""}, ok: true},
			{uri: "/api/v1/well/ttt", params: nil, ok: false},
		},
	)
}
func Test_With_With_Params_And_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/:day/:month?/:year?"),
		[]testCase{
			{uri: "/api/1", params: []string{"1", "", ""}, ok: true},
			{uri: "/api/1/", params: []string{"1", "", ""}, ok: true},
			{uri: "/api/1/2", params: []string{"1", "2", ""}, ok: true},
			{uri: "/api/1/2/3", params: []string{"1", "2", "3"}, ok: true},
			{uri: "/api/", params: nil, ok: false},
		},
	)
}
func Test_With_With_Simple_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*"),
		[]testCase{
			{uri: "/api/", params: []string{""}, ok: true},
			{uri: "/api/joker", params: []string{"joker"}, ok: true},
			{uri: "/api", params: []string{""}, ok: true},
			{uri: "/api/v1/entity", params: []string{"v1/entity"}, ok: true},
			{uri: "/api2/v1/entity", params: nil, ok: false},
			{uri: "/api_ignore/v1/entity", params: nil, ok: false},
		},
	)
}
func Test_With_With_Wildcard_And_Optional(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param?"),
		[]testCase{
			{uri: "/api/", params: []string{"", ""}, ok: true},
			{uri: "/api/joker", params: []string{"joker", ""}, ok: true},
			{uri: "/api/joker/batman", params: []string{"joker", "batman"}, ok: true},
			{uri: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, ok: true},
			{uri: "/api", params: []string{"", ""}, ok: true},
		},
	)
}
func Test_With_With_Wildcard_And_Param(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param"),
		[]testCase{
			{uri: "/api/test/abc", params: []string{"test", "abc"}, ok: true},
			{uri: "/api/joker/batman", params: []string{"joker", "batman"}, ok: true},
			{uri: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, ok: true},
			{uri: "/api", params: nil, ok: false},
		},
	)
}
func Test_With_With_Wildcard_And_2Params(t *testing.T) {
	checkCases(
		t,
		parseParams("/api/*/:param/:param2"),
		[]testCase{
			{uri: "/api/test/abc", params: nil, ok: false},
			{uri: "/api/joker/batman", params: nil, ok: false},
			{uri: "/api/joker/batman/robin", params: []string{"joker", "batman", "robin"}, ok: true},
			{uri: "/api/joker/batman/robin/1", params: []string{"joker/batman", "robin", "1"}, ok: true},
			{uri: "/api/joker/batman/robin/1/2", params: []string{"joker/batman/robin", "1", "2"}, ok: true},
			{uri: "/api", params: nil, ok: false},
		},
	)
}
func Test_With_With_Simple_Path(t *testing.T) {
	checkCases(
		t,
		parseParams("/"),
		[]testCase{
			{uri: "/api", params: nil, ok: false},
			{uri: "", params: []string{}, ok: true},
			{uri: "/", params: []string{}, ok: true},
		},
	)
}

func Test_With_With_FileName(t *testing.T) {
	checkCases(
		t,
		parseParams("/config/abc.json"),
		[]testCase{
			{uri: "/config/abc.json", params: []string{}, ok: true},
			{uri: "config/abc.json", params: nil, ok: false},
			{uri: "/config/efg.json", params: nil, ok: false},
			{uri: "/config", params: nil, ok: false},
		},
	)
}

func Test_With_With_FileName_And_Wildcard(t *testing.T) {
	checkCases(
		t,
		parseParams("/config/*.json"),
		[]testCase{
			{uri: "/config/abc.json", params: []string{"abc.json"}, ok: true},
			{uri: "/config/efg.json", params: []string{"efg.json"}, ok: true},
			//{uri: "/config/efg.csv", params: nil, ok: false},// doesn`t work, current: params: "efg.csv", true
			{uri: "config/abc.json", params: nil, ok: false},
			{uri: "/config", params: nil, ok: false},
		},
	)
}

func Test_With_With_Simple_Path_And_NoMatch(t *testing.T) {
	checkCases(
		t,
		parseParams("/xyz"),
		[]testCase{
			{uri: "xyz", params: nil, ok: false},
			{uri: "xyz/", params: nil, ok: false},
		},
	)
}

func checkCases(tParent *testing.T, parser parsedParams, tcases []testCase) {
	for _, tcase := range tcases {
		tParent.Run(fmt.Sprintf("%+v", tcase), func(t *testing.T) {
			params, ok := parser.matchParams(tcase.uri)
			if !reflect.DeepEqual(params, tcase.params) {
				t.Errorf("Path.Match() got = %v, want %v", params, tcase.params)
			}
			if ok != tcase.ok {
				t.Errorf("Path.Match() got1 = %v, want %v", ok, tcase.ok)
			}
		})
	}
}
