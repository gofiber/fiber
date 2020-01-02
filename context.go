package fiber

import (
	"fmt"

	"github.com/valyala/fasthttp"
)

// Next : Calls the next function that matches the route.
func (ctx *Context) Next() {
	ctx.next = true
	ctx.params = nil
	ctx.values = nil
}

// Params :
func (ctx *Context) Params(key string) string {
	if ctx.params == nil {
		return ""
	}
	for i := 0; i < len(*ctx.params); i++ {
		if (*ctx.params)[i] == key {
			return ctx.values[i]
		}
	}
	return ""
}

// Method https://expressjs.com/en/4x/api.html#req.method
func (ctx *Context) Method() string {
	return b2s(ctx.Fasthttp.Method())
}

// Path https://expressjs.com/en/4x/api.html#req.path
func (ctx *Context) Path() string {
	return b2s(ctx.Fasthttp.Path())
}

// Body :
func (ctx *Context) Body(args ...interface{}) string {
	if len(args) == 0 {
		return b2s(ctx.Fasthttp.Request.Body())
	}
	if len(args) == 1 {
		switch arg := args[0].(type) {
		case string:
			return b2s(ctx.Fasthttp.Request.PostArgs().Peek(arg))
		case func(string, string):
			ctx.Fasthttp.Request.PostArgs().VisitAll(func(k []byte, v []byte) {
				arg(b2s(k), b2s(v))
			})
		default:
			return b2s(ctx.Fasthttp.Request.Body())
		}
	}
	return ""
}

// Cookies :
func (ctx *Context) Cookies(args ...interface{}) string {
	if len(args) == 1 {
		switch arg := args[0].(type) {
		case string:
			return b2s(ctx.Fasthttp.Request.Header.Cookie(arg))
		case func(string, string):
			ctx.Fasthttp.Request.Header.VisitAllCookie(func(k, v []byte) {
				arg(b2s(k), b2s(v))
			})
		default:
			panic("Invalid argument")
		}
		return ""
	}
	if len(args) > 1 {
		key, keyOk := args[0].(string)
		val, valOk := args[1].(string)
		if !keyOk || !valOk {
			panic("Invalid key or value string")
		}
		cook := &fasthttp.Cookie{}
		cook.SetKey(key)
		cook.SetValue(val)
		if len(args) > 2 {
			switch arg := args[2].(type) {

			default:
				fmt.Printf("%T\n", arg)
			}
			// fmt.Println(args[2])
			// opt, optOk := args[2].(struct{})
			// if !optOk {
			// 	panic("Invalid cookie options")
			// }
			// fmt.Println(opt)
		}
		ctx.Fasthttp.Response.Header.SetCookie(cook)
	}
	return ""
}
