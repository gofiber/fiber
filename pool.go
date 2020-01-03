package fiber

import (
	"sync"

	"github.com/valyala/fasthttp"
)

// Context struct
type Ctx struct {
	next     bool
	params   *[]string
	values   []string
	Fasthttp *fasthttp.RequestCtx
}

// Context pool
var ctxPool = sync.Pool{
	New: func() interface{} {
		return new(Ctx)
	},
}

// Get new Context from pool
func acquireCtx(fctx *fasthttp.RequestCtx) *Ctx {
	ctx := ctxPool.Get().(*Ctx)
	ctx.Fasthttp = fctx
	return ctx
}

// Return Context to pool
func releaseCtx(ctx *Ctx) {
	ctx.next = false
	ctx.params = nil
	ctx.values = nil
	ctx.Fasthttp = nil
	ctxPool.Put(ctx)
}
