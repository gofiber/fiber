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
