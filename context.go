package fiber

import (
	"sync"

	"github.com/valyala/fasthttp"
)

// Context struct
type Context struct {
	next     bool
	params   *[]string
	values   []string
	Fasthttp *fasthttp.RequestCtx
}

// Context pool
var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Context)
	},
}

// Get new Context from pool
func acquireCtx(fctx *fasthttp.RequestCtx) *Context {
	ctx := ctxPool.Get().(*Context)
	ctx.Fasthttp = fctx
	return ctx
}

// Return Context to pool
func releaseCtx(ctx *Context) {
	ctx.next = false
	ctx.params = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	ctxPool.Put(ctx)
}

// Next : Call the next middleware function in the stack.
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
