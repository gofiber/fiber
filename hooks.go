package fiber

import (
	"github.com/valyala/fasthttp"
)

// Handler defines a function to create hooks for Fiber.
type HookHandler = func(*Ctx, Map) error

type Hooks struct {
	// Embed app
	app *App

	// Hooks
	onRoute     []HookHandler
	onName      []HookHandler
	onGroupName []HookHandler
	onListen    []HookHandler
	onShutdown  []HookHandler
}

// OnRoute is a hook to execute user functions on each route registeration.
// Also you can get route properties by "route" key of map.
func (h *Hooks) OnRoute(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.onRoute = append(h.onRoute, handler...)
	h.app.mutex.Unlock()
}

// OnName is a hook to execute user functions on each route naming.
// Also you can get route properties by "route" key of map.
//
// WARN: OnName only works with naming routes, not groups.
func (h *Hooks) OnName(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.onName = append(h.onName, handler...)
	h.app.mutex.Unlock()
}

// OnGroupName is a hook to execute user functions on each group naming.
// Also you can get group properties by "group" key of map.
//
// WARN: OnGroupName only works with naming groups, not routes.
func (h *Hooks) OnGroupName(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.onGroupName = append(h.onGroupName, handler...)
	h.app.mutex.Unlock()
}

// OnListen is a hook to execute user functions on Listen, ListenTLS, Listener.
func (h *Hooks) OnListen(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.onListen = append(h.onListen, handler...)
	h.app.mutex.Unlock()
}

// OnShutdown is a hook to execute user functions after Shutdown.
func (h *Hooks) OnShutdown(handler ...HookHandler) {
	h.app.mutex.Lock()
	h.onShutdown = append(h.onShutdown, handler...)
	h.app.mutex.Unlock()
}

func (h *Hooks) executeOnRouteHooks(route Route) error {
	for _, v := range h.onRoute {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"route": route}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnNameHooks(route Route) error {
	for _, v := range h.onName {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"route": route}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnGroupNameHooks(group Group) error {
	for _, v := range h.onGroupName {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{"group": group}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnListenHooks() error {
	for _, v := range h.onListen {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		if err := v(ctx, Map{}); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnShutdownHooks() {
	for _, v := range h.onShutdown {
		ctx := h.app.AcquireCtx(&fasthttp.RequestCtx{})
		defer h.app.ReleaseCtx(ctx)

		_ = v(ctx, Map{})
	}
}
