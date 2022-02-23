package fiber

import (
	"github.com/valyala/fasthttp"
)

// Handler defines a function to create hooks for Fibe.
type HookHandler = func(*Ctx, Map) error

type Hooks struct {
	app      *App
	hookList map[string][]HookHandler
}

// OnRoute is a hook to execute user functions on each route registeration.
// Also you can get route properties by "route" key of map.
func (h *Hooks) OnRoute(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onRoute"] = append(h.hookList["onRoute"], handler...)
	h.app.mutex.Unlock()
}

// OnName is a hook to execute user functions on each route naming.
// Also you can get route properties by "route" key of map.
//
// WARN: OnName only works with naming routes, not groups.
func (h *Hooks) OnName(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onName"] = append(h.hookList["onName"], handler...)
	h.app.mutex.Unlock()
}

// OnListen is a hook to execute user functions on Listen, ListenTLS, Listener.
func (h *Hooks) OnListen(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onListen"] = append(h.hookList["onListen"], handler...)
	h.app.mutex.Unlock()
}

// OnShutdown is a hook to execute user functions after Shutdown.
func (h *Hooks) OnShutdown(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onShutdown"] = append(h.hookList["onShutdown"], handler...)
	h.app.mutex.Unlock()
}

// OnResponse is a hook to execute user functions after a response.
//
// WARN: You can't edit response with OnResponse hook.
func (h *Hooks) OnResponse(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onResponse"] = append(h.hookList["onResponse"], handler...)
	h.app.mutex.Unlock()
}

// OnRequest is a hook to execute user functions after a request.
//
// WARN: You can edit response with OnRequest hook.
func (h *Hooks) OnRequest(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onRequest"] = append(h.hookList["onRequest"], handler...)
	h.app.mutex.Unlock()
}

func (h *Hooks) executeOnRouteHooks(route Route) error {
	for _, v := range h.hookList["onRoute"] {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"route": route}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnNameHooks(route Route) error {
	for _, v := range h.hookList["onName"] {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"route": route}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnListenHooks() error {
	for _, v := range h.hookList["onListen"] {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnShutdownHooks() {
	if len(h.hookList["onShutdown"]) > 0 {
		for _, v := range h.hookList["onShutdown"] {
			ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
			defer h.app.ReleaseCtx(ctx)

			_ = v(ctx, Map{})
		}
	}
}

func (h *Hooks) executeOnRequestHooks(c *Ctx) error {
	for _, v := range h.hookList["onRequest"] {
		if err := v(c, Map{}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnResponseHooks(c *Ctx) {
	for _, v := range h.hookList["onResponse"] {
		_ = v(c, Map{})
	}
}
