// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"

	"github.com/valyala/fasthttp"
)

type CustomCtx[T any] interface {
	CtxGeneric[T]

	// Reset is a method to reset context fields by given request when to use server handlers.
	Reset(fctx *fasthttp.RequestCtx)

	// Methods to use with next stack.
	getMethodInt() int
	getIndexRoute() int
	getTreePathHash() int
	getDetectionPath() string
	getPathOriginal() string
	getValues() *[maxParams]string
	getMatched() bool
	setIndexHandler(handler int)
	setIndexRoute(route int)
	setMatched(matched bool)
	setRoute(route *Route[T])
}

func NewDefaultCtx[TCtx *DefaultCtx](app *App[*DefaultCtx]) TCtx {
	// return ctx
	ctx := &DefaultCtx{
		// Set app reference
		app: app,
	}
	ctx.req = &DefaultReq{ctx: ctx}
	ctx.res = &DefaultRes{ctx: ctx}

	return ctx
}

func (app *App[TCtx]) newCtx() TCtx {
	var c TCtx

	// TODO: fix this with generics ?
	if app.newCtxFunc != nil {
		c = app.newCtxFunc(app)
	} else {
		c = any(NewDefaultCtx[*DefaultCtx](app)).(TCtx)
	}

	return c
}

// AcquireCtx retrieves a new Ctx from the pool.
func (app *App[TCtx]) AcquireCtx(fctx *fasthttp.RequestCtx) TCtx {
	ctx, ok := app.pool.Get().(TCtx)

	if !ok {
		panic(errors.New("failed to type-assert to Ctx"))
	}
	ctx.Reset(fctx)

	return ctx
}

// ReleaseCtx releases the ctx back into the pool.
func (app *App[TCtx]) ReleaseCtx(c TCtx) {
	c.release()
	app.pool.Put(c)
}
