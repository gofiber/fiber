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
	Token() (token string)

	ReadBool(key, expect string, args ...string) bool

	ReadInt(key string, val int, args ...string) int

	ReadFloat(key string, val float64, args ...string) float64

	GetStr(key string, args ...string) string

	GetInt(key string, val int) int

	GetFloat(key string, val float64) float64

	PostStr(key string, args ...string) string

	PostInt(key string, val int) int

	PostFloat(key string, val float64) float64

	PostAll() (map[string]interface{}, error)

	// Read the POST first, if empty then read GET
	FetchStr(key string, args ...string) string

	FetchInt(key string, val int) int

	FetchFloat(key string, val float64) float64

	ParamStr(key string, args ...string) string

	ParamInt(key string, val int) int

	ParamFloat(key string, val float64) float64

	HeaderStr(key string, args ...string) string

	CookieStr(key string, args ...string) string
}
