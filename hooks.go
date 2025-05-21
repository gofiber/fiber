package fiber

import (
	"github.com/gofiber/fiber/v3/log"
)

// OnRouteHandler Handlers define a function to create hooks for Fiber.
type (
	OnRouteHandler[TCtx CtxGeneric[TCtx]]     func(Route[TCtx]) error
	OnNameHandler[TCtx CtxGeneric[TCtx]]      func(Route[TCtx]) error
	OnGroupHandler[TCtx CtxGeneric[TCtx]]     func(Group[TCtx]) error
	OnGroupNameHandler[TCtx CtxGeneric[TCtx]] func(Group[TCtx]) error
	OnListenHandler                           func(ListenData) error
	OnPreShutdownHandler                      func() error
	OnPostShutdownHandler                     func(error) error
	OnForkHandler                             func(int) error
	OnMountHandler[TCtx CtxGeneric[TCtx]]     func(*App[TCtx]) error
)

// Hooks is a struct to use it with App.
type Hooks[TCtx CtxGeneric[TCtx]] struct {
	// Embed app
	app *App[TCtx]

	// Hooks
	onRoute        []OnRouteHandler[TCtx]
	onName         []OnNameHandler[TCtx]
	onGroup        []OnGroupHandler[TCtx]
	onGroupName    []OnGroupNameHandler[TCtx]
	onListen       []OnListenHandler
	onPreShutdown  []OnPreShutdownHandler
	onPostShutdown []OnPostShutdownHandler
	onFork         []OnForkHandler
	onMount        []OnMountHandler[TCtx]
}

// ListenData is a struct to use it with OnListenHandler
type ListenData struct {
	Host string
	Port string
	TLS  bool
}

func newHooks[TCtx CtxGeneric[TCtx]](app *App[TCtx]) *Hooks[TCtx] {
	return &Hooks[TCtx]{
		app:            app,
		onRoute:        make([]OnRouteHandler[TCtx], 0),
		onGroup:        make([]OnGroupHandler[TCtx], 0),
		onGroupName:    make([]OnGroupNameHandler[TCtx], 0),
		onName:         make([]OnNameHandler[TCtx], 0),
		onListen:       make([]OnListenHandler, 0),
		onPreShutdown:  make([]OnPreShutdownHandler, 0),
		onPostShutdown: make([]OnPostShutdownHandler, 0),
		onFork:         make([]OnForkHandler, 0),
		onMount:        make([]OnMountHandler[TCtx], 0),
	}
}

// OnRoute is a hook to execute user functions on each route registration.
// Also you can get route properties by route parameter.
func (h *Hooks[TCtx]) OnRoute(handler ...OnRouteHandler[TCtx]) {
	h.app.mutex.Lock()
	h.onRoute = append(h.onRoute, handler...)
	h.app.mutex.Unlock()
}

// OnName is a hook to execute user functions on each route naming.
// Also you can get route properties by route parameter.
//
// WARN: OnName only works with naming routes, not groups.
func (h *Hooks[TCtx]) OnName(handler ...OnNameHandler[TCtx]) {
	h.app.mutex.Lock()
	h.onName = append(h.onName, handler...)
	h.app.mutex.Unlock()
}

// OnGroup is a hook to execute user functions on each group registration.
// Also you can get group properties by group parameter.
func (h *Hooks[TCtx]) OnGroup(handler ...OnGroupHandler[TCtx]) {
	h.app.mutex.Lock()
	h.onGroup = append(h.onGroup, handler...)
	h.app.mutex.Unlock()
}

// OnGroupName is a hook to execute user functions on each group naming.
// Also you can get group properties by group parameter.
//
// WARN: OnGroupName only works with naming groups, not routes.
func (h *Hooks[TCtx]) OnGroupName(handler ...OnGroupNameHandler[TCtx]) {
	h.app.mutex.Lock()
	h.onGroupName = append(h.onGroupName, handler...)
	h.app.mutex.Unlock()
}

// OnListen is a hook to execute user functions on Listen, ListenTLS, Listener.
func (h *Hooks[TCtx]) OnListen(handler ...OnListenHandler) {
	h.app.mutex.Lock()
	h.onListen = append(h.onListen, handler...)
	h.app.mutex.Unlock()
}

// OnPreShutdown is a hook to execute user functions before Shutdown.
func (h *Hooks) OnPreShutdown(handler ...OnPreShutdownHandler) {
	h.app.mutex.Lock()
	h.onPreShutdown = append(h.onPreShutdown, handler...)
	h.app.mutex.Unlock()
}

// OnPostShutdown is a hook to execute user functions after Shutdown.
func (h *Hooks[TCtx]) OnPostShutdown(handler ...OnPostShutdownHandler) {
	h.app.mutex.Lock()
	h.onPostShutdown = append(h.onPostShutdown, handler...)
	h.app.mutex.Unlock()
}

// OnFork is a hook to execute user function after fork process.
func (h *Hooks[TCtx]) OnFork(handler ...OnForkHandler) {
	h.app.mutex.Lock()
	h.onFork = append(h.onFork, handler...)
	h.app.mutex.Unlock()
}

// OnMount is a hook to execute user function after mounting process.
// The mount event is fired when sub-app is mounted on a parent app. The parent app is passed as a parameter.
// It works for app and group mounting.
func (h *Hooks[TCtx]) OnMount(handler ...OnMountHandler[TCtx]) {
	h.app.mutex.Lock()
	h.onMount = append(h.onMount, handler...)
	h.app.mutex.Unlock()
}

func (h *Hooks[TCtx]) executeOnRouteHooks(route Route[TCtx]) error {
	// Check mounting
	if h.app.mountFields.mountPath != "" {
		route.path = h.app.mountFields.mountPath + route.path
		route.Path = route.path
	}

	for _, v := range h.onRoute {
		if err := v(route); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks[TCtx]) executeOnNameHooks(route Route[TCtx]) error {
	// Check mounting
	if h.app.mountFields.mountPath != "" {
		route.path = h.app.mountFields.mountPath + route.path
		route.Path = route.path
	}

	for _, v := range h.onName {
		if err := v(route); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks[TCtx]) executeOnGroupHooks(group Group[TCtx]) error {
	// Check mounting
	if h.app.mountFields.mountPath != "" {
		group.Prefix = h.app.mountFields.mountPath + group.Prefix
	}

	for _, v := range h.onGroup {
		if err := v(group); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks[TCtx]) executeOnGroupNameHooks(group Group[TCtx]) error {
	// Check mounting
	if h.app.mountFields.mountPath != "" {
		group.Prefix = h.app.mountFields.mountPath + group.Prefix
	}

	for _, v := range h.onGroupName {
		if err := v(group); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks[TCtx]) executeOnListenHooks(listenData ListenData) error {
	for _, v := range h.onListen {
		if err := v(listenData); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks[TCtx]) executeOnPreShutdownHooks() {
	for _, v := range h.onPreShutdown {
		if err := v(); err != nil {
			log.Errorf("failed to call pre shutdown hook: %v", err)
		}
	}
}

func (h *Hooks) executeOnPostShutdownHooks(err error) {
	for _, v := range h.onPostShutdown {
		if err := v(err); err != nil {
			log.Errorf("failed to call post shutdown hook: %v", err)
		}
	}
}

func (h *Hooks[TCtx]) executeOnForkHooks(pid int) {
	for _, v := range h.onFork {
		if err := v(pid); err != nil {
			log.Errorf("failed to call fork hook: %v", err)
		}
	}
}

func (h *Hooks[TCtx]) executeOnMountHooks(app *App[TCtx]) error {
	for _, v := range h.onMount {
		if err := v(app); err != nil {
			return err
		}
	}

	return nil
}
