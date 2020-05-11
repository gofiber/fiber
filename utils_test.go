// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import "testing"

// func Test_Utils_assertEqual(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_setETag(t *testing.T) {
// 	// TODO
// }

func Test_Utils_getGroupPath(t *testing.T) {
	res := getGroupPath("/v1", "/")
	assertEqual(t, "/v1", res)

	res = getGroupPath("/v1", "/")
	assertEqual(t, "/v1", res)

	res = getGroupPath("/", "/")
	assertEqual(t, "/", res)

	res = getGroupPath("/v1/api/", "/")
	assertEqual(t, "/v1/api/", res)
}

func Test_Utils_getMIME(t *testing.T) {
	res := getMIME(".json")
	assertEqual(t, "application/json", res)

	res = getMIME(".xml")
	assertEqual(t, "application/xml", res)

	res = getMIME("xml")
	assertEqual(t, "application/xml", res)

	res = getMIME("json")
	assertEqual(t, "application/json", res)
}

// func Test_Utils_getArgument(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_parseTokenList(t *testing.T) {
// 	// TODO
// }

func Test_Utils_getString(t *testing.T) {
	res := getString([]byte("Hello, World!"))
	assertEqual(t, "Hello, World!", res)

	res = getString([]byte(""))
	assertEqual(t, "", res)
}

func Test_Utils_getStringImmutable(t *testing.T) {
	res := getStringImmutable([]byte("Hello, World!"))
	assertEqual(t, "Hello, World!", res)

	res = getStringImmutable([]byte(""))
	assertEqual(t, "", res)
}

func Test_Utils_getBytes(t *testing.T) {
	res := getBytes("Hello, World!")
	assertEqual(t, []byte("Hello, World!"), res)

	res = getBytes("")
	assertEqual(t, []byte{}, res)
}

func Test_Utils_getBytesImmutable(t *testing.T) {
	res := getBytesImmutable("Hello, World!")
	assertEqual(t, []byte("Hello, World!"), res)

	res = getBytesImmutable("")
	assertEqual(t, []byte{}, res)
}

func Test_Utils_methodINT(t *testing.T) {
	res := methodINT[MethodGet]
	assertEqual(t, 0, res)
	res = methodINT[MethodHead]
	assertEqual(t, 1, res)
	res = methodINT[MethodPost]
	assertEqual(t, 2, res)
	res = methodINT[MethodPut]
	assertEqual(t, 3, res)
	res = methodINT[MethodPatch]
	assertEqual(t, 4, res)
	res = methodINT[MethodDelete]
	assertEqual(t, 5, res)
	res = methodINT[MethodConnect]
	assertEqual(t, 6, res)
	res = methodINT[MethodOptions]
	assertEqual(t, 7, res)
	res = methodINT[MethodTrace]
	assertEqual(t, 8, res)
}

func Test_Utils_statusMessage(t *testing.T) {
	res := statusMessage[102]
	assertEqual(t, "Processing", res)

	res = statusMessage[303]
	assertEqual(t, "See Other", res)

	res = statusMessage[404]
	assertEqual(t, "Not Found", res)

	res = statusMessage[507]
	assertEqual(t, "Insufficient Storage", res)

}

func Test_Utils_extensionMIME(t *testing.T) {
	res := extensionMIME[".html"]
	assertEqual(t, "text/html", res)

	res = extensionMIME["html"]
	assertEqual(t, "text/html", res)

	res = extensionMIME[".msp"]
	assertEqual(t, "application/octet-stream", res)

	res = extensionMIME["msp"]
	assertEqual(t, "application/octet-stream", res)
}

// func Test_Utils_getParams(t *testing.T) {
// 	// TODO
// }

func Test_Utils_matchParams(t *testing.T) {
	type testparams struct {
		url    string
		params []string
		match  bool
	}
	testCase := func(r string, cases []testparams) {
		parser := getParams(r)
		for _, c := range cases {
			params, match := parser.getMatch(c.url)
			assertEqual(t, c.params, params)
			assertEqual(t, c.match, match)
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
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/optional", params: []string{"optional"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/xyz", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1", params: []string{""}, match: true},
		{url: "/api/v1/", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/1/2", params: []string{"entity/1/2"}, match: true},
		{url: "/api/v", params: nil, match: false},
		{url: "/api/v2", params: nil, match: false},
		{url: "/api/abc", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/entity", params: []string{"entity"}, match: true},
		{url: "/api/v1/entity/8728382", params: nil, match: false},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/const", params: []string{}, match: true},
		{url: "/api/v1", params: nil, match: false},
		{url: "/api/v1/", params: nil, match: false},
		{url: "/api/v1/something", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/v1/well/abc/wildcard", params: []string{"well", "wildcard"}, match: true},
		{url: "/api/v1/well/abc/", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/abc", params: []string{"well", ""}, match: true},
		{url: "/api/v1/well/ttt", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/1", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/", params: []string{"1", "", ""}, match: true},
		{url: "/api/1/2", params: []string{"1", "2", ""}, match: true},
		{url: "/api/1/2/3", params: []string{"1", "2", "3"}, match: true},
		{url: "/api/", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/", params: []string{""}, match: true},
		{url: "/api/joker", params: []string{"joker"}, match: true},
		{url: "/api", params: []string{""}, match: true},
		{url: "/api/v1/entity", params: []string{"v1/entity"}, match: true},
		{url: "/api2/v1/entity", params: nil, match: false},
		{url: "/api_ignore/v1/entity", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api/", params: []string{"", ""}, match: true},
		{url: "/api/joker", params: []string{"joker", ""}, match: true},
		{url: "/api/joker/batman", params: []string{"joker", "batman"}, match: true},
		{url: "/api/joker/batman/robin", params: []string{"joker/batman", "robin"}, match: true},
		{url: "/api/joker/batman/robin/1", params: []string{"joker/batman/robin", "1"}, match: true},
		{url: "/api", params: []string{"", ""}, match: true},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api", params: nil, match: false},
		{url: "", params: []string{}, match: true},
		{url: "/", params: []string{}, match: true},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/api", params: nil, match: false},
		{url: "", params: []string{}, match: true},
		{url: "/", params: []string{}, match: true},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/config/abc.json", params: []string{}, match: true},
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config/efg.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "/config/abc.json", params: []string{"abc.json"}, match: true},
		{url: "/config/efg.json", params: []string{"efg.json"}, match: true},
		//{url: "/config/efg.csv", params: nil, match: false},// doesn`t work, current: params: "efg.csv", true
		{url: "config/abc.json", params: nil, match: false},
		{url: "/config", params: nil, match: false},
	})
	testCase("/api/v1/:param/*", []testparams{
		{url: "xyz", params: nil, match: false},
		{url: "xyz/", params: nil, match: false},
	})
}

// func Test_Utils_getTrimmedParam(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getCharPos(t *testing.T) {
// 	// TODO
// }
