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

// func Test_Utils_getString(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getStringImmutable(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getBytes(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getBytesImmutable(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_methodINT(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_statusMessage(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_extensionMIME(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getParams(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_matchParams(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getTrimmedParam(t *testing.T) {
// 	// TODO
// }

// func Test_Utils_getCharPos(t *testing.T) {
// 	// TODO
// }
