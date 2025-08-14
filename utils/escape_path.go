// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	gutils "github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// EscapePath returns a URL-escaped path while preserving slashes.
// It pre-allocates extra capacity to minimize allocations.
func EscapePath(path string) string {
	b := gutils.UnsafeBytes(path)
	buf := make([]byte, 0, len(b)+len(b)/2)
	start := 0
	for i, c := range b {
		if c == '/' {
			buf = fasthttp.AppendQuotedArg(buf, b[start:i])
			buf = append(buf, '/')
			start = i + 1
		}
	}
	buf = fasthttp.AppendQuotedArg(buf, b[start:])
	return gutils.UnsafeString(buf)
}
