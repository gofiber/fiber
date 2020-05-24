// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package contrib

import (
	"fmt"
	"net/http"
	"unsafe"

	"github.com/gofiber/fiber/base"
)

// Quickly to string
func ToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func GetServiceCode(statusCode int) int {
	if statusCode == 200 {
		return 200
	}
	return statusCode
}

// Give the difference of reponse body or reponse data
type ReplyBody base.Map

type ResponseMixin struct {
	// JSON converts any interface or string to JSON using Jsoniter.
	// This method also sets the content header to application/json.
	JSON func(data interface{}) error

	// JSONP sends a JSON response with JSONP support.
	// This method is identical to JSON, except that it opts-in to JSONP callback support.
	// By default, the callback name is simply callback.
	JSONP func(data interface{}, callback ...string) error

	// Send formatted string
	Printf func(format string, args ...interface{}) error

	// Send sets the HTTP response body. The Send body can be of any type.
	Send func(bodies ...interface{})

	// Status sets the HTTP status for the response.
	// This method is chainable.
	SetStatus func(status int)

	// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
	SetType func(ext string)

	// Write appends any input to the HTTP body response.
	Write func(bodies ...interface{})
}

func NewResponseMixin(ctx base.IBaseResponse) *ResponseMixin {
	r := new(ResponseMixin)
	r.JSON, r.JSONP = ctx.JSON, ctx.JSONP
	r.Printf, r.Send = ctx.Printf, ctx.Send
	r.SetStatus, r.SetType = ctx.SetStatus, ctx.SetType
	r.Write = ctx.Write
	return r
}

func (r *ResponseMixin) Jsonify(format string, args ...interface{}) error {
	r.SetType("json")
	return r.Printf(format, args...)
}

func (r *ResponseMixin) Errorf(servCode int, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	body := base.Map{"code": servCode, "message": msg}
	r.SetStatus(http.StatusOK)
	return r.JSON(body)
}

func (r *ResponseMixin) Abort(code int, data interface{}) error {
	r.SetStatus(code)
	if data != nil {
		return r.JSON(data)
	}
	return nil
}

func (r *ResponseMixin) Deny(msg string) error {
	code := http.StatusForbidden
	servCode := GetServiceCode(code)
	return r.Abort(code, base.Map{"code": servCode, "message": msg})
}

func (r *ResponseMixin) Reply(data interface{}, metas ...int64) error {
	var body base.Map
	if rbody, ok := data.(ReplyBody); ok { // Map as response body
		body = base.Map(rbody)
	} else { // Map as data in response body
		servCode := GetServiceCode(http.StatusOK)
		body = base.Map{"code": servCode, "data": data}
	}
	if len(metas) >= 1 {
		body["total"] = metas[0]
	}
	return r.JSON(body)
}
