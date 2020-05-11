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
	testCase := func(r, u string, p []string, m bool) {
		parser := getParams(r)
		params, match := parser.getMatch(u)
		assertEqual(t, p, params)
		assertEqual(t, m, match)
	}
	testCase("/api/v1/:param/*", "/api/v1/entity", []string{"entity", ""}, true)
}

// func Test_Utils_getTrimmedParam(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getCharPos(t *testing.T) {
// 	// TODO
// }
