// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package base

type IBaseRequest interface {
	// BaseURL returns (protocol + host + base path).
	BaseURL() string

	// Body contains the raw body submitted in a POST request.
	Body() string

	// BodyParser binds the request body to a struct.
	// It supports decoding the following content types based on the Content-Type header:
	// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
	BodyParser(out interface{}) error

	// Common method read data of GET/POST/PARAM/HEADER/COOKIE
	Read(key, val string, methods ...string) string
}

type ImplRequest interface {
}
