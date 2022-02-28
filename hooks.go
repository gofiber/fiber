package fiber

import (
	"github.com/valyala/fasthttp"
)

// Handler defines a function to create hooks for Fiber.
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

// OnGroupName is a hook to execute user functions on each group naming.
// Also you can get group properties by "group" key of map.
//
// WARN: OnGroupName only works with naming groups, not routes.
func (h *Hooks) OnGroupName(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.hookList["onGroupName"] = append(h.hookList["onGroupName"], handler...)
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

func (h *Hooks) executeOnGroupNameHooks(group Group) error {
	for _, v := range h.hookList["onGroupName"] {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"group": group}); err != nil {
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
