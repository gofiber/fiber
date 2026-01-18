// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"github.com/valyala/fasthttp"
)

// CustomCtx extends Ctx with the additional methods required by Fiber's
// internals and middleware helpers.
type CustomCtx interface {
	Ctx

	// Reset is a method to reset context fields by given request when to use server handlers.
	Reset(fctx *fasthttp.RequestCtx)

	// release is called before returning the context to the pool.
	release()

	// Abandon marks the context as abandoned. An abandoned context will not be
	// returned to the pool when ReleaseCtx is called. This is used by the timeout
	// middleware to return immediately while the handler goroutine continues.
	// The cleanup goroutine must call ForceRelease when the handler finishes.
	Abandon()

	// IsAbandoned returns true if the context has been abandoned.
	IsAbandoned() bool

	// ForceRelease releases an abandoned context back to the pool.
	// Must only be called after the handler goroutine has completely finished.
	ForceRelease()

	// Methods to use with next stack.
	getMethodInt() int
	getIndexRoute() int
	getTreePathHash() int
	getDetectionPath() string
	getPathOriginal() string
	getValues() *[maxParams]string
	getMatched() bool
	getSkipNonUseRoutes() bool
	setIndexHandler(handler int)
	setIndexRoute(route int)
	setMatched(matched bool)
	setSkipNonUseRoutes(skip bool)
	setRoute(route *Route)
}

// NewDefaultCtx constructs the default context implementation bound to the
// provided application.
func NewDefaultCtx(app *App) *DefaultCtx {
	// return ctx
	ctx := &DefaultCtx{
		// Set app reference
		app: app,
	}
	ctx.DefaultReq.c = ctx
	ctx.DefaultRes.c = ctx

	return ctx
}

// AcquireCtx retrieves a new Ctx from the pool.
func (app *App) AcquireCtx(fctx *fasthttp.RequestCtx) CustomCtx {
	ctx, ok := app.pool.Get().(CustomCtx)

	if !ok {
		panic(errCustomCtxTypeAssertion)
	}

	if app.hasCustomCtx {
		if setter, ok := ctx.(interface{ setHandlerCtx(CustomCtx) }); ok {
			setter.setHandlerCtx(ctx)
		}
	}

	ctx.Reset(fctx)

	return ctx
}

// ReleaseCtx releases the ctx back into the pool.
// If the context was abandoned (e.g., by timeout middleware), this is a no-op.
// The abandoning code must call ForceRelease when the handler finishes.
func (app *App) ReleaseCtx(c CustomCtx) {
	if c.IsAbandoned() {
		return
	}
	c.release()
	app.pool.Put(c)
}
