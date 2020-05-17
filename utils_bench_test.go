// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"testing"
)

// go test -v ./... -run=^$ -bench=Benchmark_Utils_toUpper -benchmem -count=3

// func Benchmark_Utils_assertEqual(b *testing.B) {
// 	// TODO
// }

func Benchmark_Utils_getGroupPath(b *testing.B) {
	var res string
	for n := 0; n < b.N; n++ {
		res = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
		res = getGroupPath("/v1", "/")
		res = getGroupPath("/v1", "/api")
		res = getGroupPath("/v1", "/api/register/:project")
	}
	assertEqual(b, "/v1/api/register/:project", res)
}

func Benchmark_Utils_getMIME(b *testing.B) {
	var res string
	for n := 0; n < b.N; n++ {
		res = getMIME(".json")
		res = getMIME(".xml")
		res = getMIME("xml")
		res = getMIME("json")
	}
	assertEqual(b, "application/json", res)
}

// func Benchmark_Utils_getArgument(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_parseTokenList(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_getParams(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_matchParams(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_getCharPos(b *testing.B) {
// 	// TODO
// }
