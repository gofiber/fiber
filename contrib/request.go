// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package contrib

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/base"
)

const (
	COOKIE_TOKEN_KEY = "access_token"
	HEADER_TOKEN_KEY = "x-token"
)

// 找出第一个参数
func GetFirstStrArg(args []string) (val string) {
	if len(args) > 0 {
		val = args[0]
	}
	return
}

type RequestMixin struct {
	// BaseURL returns (protocol + host + base path).
	BaseURL func() string

	// Body contains the raw body submitted in a POST request.
	Body func() string

	// BodyParser binds the request body to a struct.
	// It supports decoding the following content types based on the Content-Type header:
	// application/json, application/xml, application/x-www-form-urlencoded, multipart/form-data
	BodyParser func(out interface{}) error

	// Common method read data of GET/POST/PARAM/HEADER/COOKIE
	Read func(key, val string, methods ...string) string
}

func NewRequestMixin(ctx base.IBaseRequest) *RequestMixin {
	r := new(RequestMixin)
	r.Body, r.BodyParser = ctx.Body, ctx.BodyParser
	r.Read, r.BaseURL = ctx.Read, ctx.BaseURL
	return r
}

func (r *RequestMixin) Token() (token string) {
	if token = r.CookieStr(COOKIE_TOKEN_KEY); token == "" {
		token = r.HeaderStr(HEADER_TOKEN_KEY)
	}
	return
}

func (r *RequestMixin) ReadBool(key, expect string, args ...string) bool {
	if value := r.Read(key, "", args...); value != "" {
		return strings.TrimSpace(value) == expect
	}
	return false
}

func (r *RequestMixin) ReadInt(key string, val int, args ...string) int {
	if value := r.Read(key, "", args...); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return val
}

func (r *RequestMixin) ReadFloat(key string, val float64, args ...string) float64 {
	if value := r.Read(key, "", args...); value != "" {
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	}
	return val
}

func (r *RequestMixin) GetStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "GET")
}

func (r *RequestMixin) GetInt(key string, val int) int {
	return r.ReadInt(key, val, "GET")
}

func (r *RequestMixin) GetFloat(key string, val float64) float64 {
	return r.ReadFloat(key, val, "GET")
}

func (r *RequestMixin) PostStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "POST")
}

func (r *RequestMixin) PostInt(key string, val int) int {
	return r.ReadInt(key, val, "POST")
}

func (r *RequestMixin) PostFloat(key string, val float64) float64 {
	return r.ReadFloat(key, val, "POST")
}

func (r *RequestMixin) PostAll() (map[string]interface{}, error) {
	data := make(map[string]interface{})
	values, err := url.ParseQuery(r.Body())
	if err != nil {
		return data, err
	}
	for key, vals := range values {
		data[key] = strings.Join(vals, ",")
	}
	return data, nil
}

// Read the POST first, if empty then read GET
func (r *RequestMixin) FetchStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "POST", "GET")
}

func (r *RequestMixin) FetchInt(key string, val int) int {
	return r.ReadInt(key, val, "POST", "GET")
}

func (r *RequestMixin) FetchFloat(key string, val float64) float64 {
	return r.ReadFloat(key, val, "POST", "GET")
}

func (r *RequestMixin) ParamStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "PARAM")
}

func (r *RequestMixin) ParamInt(key string, val int) int {
	return r.ReadInt(key, val, "PARAM")
}

func (r *RequestMixin) ParamFloat(key string, val float64) float64 {
	return r.ReadFloat(key, val, "PARAM")
}

func (r *RequestMixin) HeaderStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "HEADER")
}

func (r *RequestMixin) CookieStr(key string, args ...string) string {
	return r.Read(key, GetFirstStrArg(args), "COOKIE")
}
