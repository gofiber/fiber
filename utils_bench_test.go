// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"testing"
)

// go test -v ./... -run=^$ -bench=Benchmark_CC_ -benchmem -count=3

// func Benchmark_Utils_assertEqual(b *testing.B) {
// 	// TODO
// }

func Benchmark_Utils_getGroupPath(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = getGroupPath("/v1", "/")
		_ = getGroupPath("/v1", "/api")
		_ = getGroupPath("/v1", "/api/register/:project")
		_ = getGroupPath("/v1/long/path/john/doe", "/why/this/name/is/so/awesome")
	}
}

func Benchmark_Utils_getMIME(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = getMIME(".json")
		_ = getMIME(".xml")
		_ = getMIME("xml")
		_ = getMIME("json")
	}
}

// func Benchmark_Utils_getArgument(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_parseTokenList(b *testing.B) {
// 	// TODO
// }

func Benchmark_Utils_getString(b *testing.B) {
	raw := []byte("Hello, World!")
	for n := 0; n < b.N; n++ {
		_ = getString(raw)
	}
}

func Benchmark_Utils_getStringImmutable(b *testing.B) {
	raw := []byte("Hello, World!")
	for n := 0; n < b.N; n++ {
		_ = getStringImmutable(raw)
	}
}

func Benchmark_Utils_getBytes(b *testing.B) {
	raw := "Hello, World!"
	for n := 0; n < b.N; n++ {
		_ = getBytes(raw)
	}
}

func Benchmark_Utils_getBytesImmutable(b *testing.B) {
	raw := "Hello, World!"
	for n := 0; n < b.N; n++ {
		_ = getBytesImmutable(raw)
	}
}

func Benchmark_Utils_methodINT(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = methodINT[MethodGet]
		_ = methodINT[MethodHead]
		_ = methodINT[MethodPost]
		_ = methodINT[MethodPut]
		_ = methodINT[MethodPatch]
		_ = methodINT[MethodDelete]
		_ = methodINT[MethodConnect]
		_ = methodINT[MethodOptions]
		_ = methodINT[MethodTrace]
	}
}

func Benchmark_Utils_statusMessage(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = statusMessage[100]
		_ = statusMessage[304]
		_ = statusMessage[423]
		_ = statusMessage[507]
	}
}

func Benchmark_Utils_extensionMIME(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = extensionMIME[".json"]
		_ = extensionMIME["json"]
		_ = extensionMIME["xspf"]
		_ = extensionMIME[".xspf"]
		_ = extensionMIME["avi"]
		_ = extensionMIME[".avi"]
	}
}

// func Benchmark_Utils_getParams(b *testing.B) {
// 	// TODO
// }

// func Benchmark_Utils_matchParams(b *testing.B) {
// 	// TODO
// }

func Benchmark_Utils_getTrimmedParam(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = getTrimmedParam(":param")
		_ = getTrimmedParam(":param?")
	}
}

// func Benchmark_Utils_getCharPos(b *testing.B) {
// 	// TODO
// }
