// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package base

type IBaseResponse interface {
	// JSON converts any interface or string to JSON using Jsoniter.
	// This method also sets the content header to application/json.
	JSON(data interface{}) error

	// JSONP sends a JSON response with JSONP support.
	// This method is identical to JSON, except that it opts-in to JSONP callback support.
	// By default, the callback name is simply callback.
	JSONP(data interface{}, callback ...string) error

	// Send formatted string
	Printf(format string, args ...interface{}) error

	// Send sets the HTTP response body. The Send body can be of any type.
	Send(bodies ...interface{})

	// Status sets the HTTP status for the response.
	// This method is chainable.
	SetStatus(status int)

	// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
	SetType(ext string)

	// Write appends any input to the HTTP body response.
	Write(bodies ...interface{})
}

type ImplResponse interface {
	Jsonify(format string, args ...interface{}) error

	Errorf(servCode int, format string, args ...interface{}) error

	Abort(code int, data interface{}) error

	Deny(msg string) error

	Reply(data interface{}, metas ...int64) error
}
