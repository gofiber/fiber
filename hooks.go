package fiber

import (
	"github.com/gofiber/fiber/v2/log"
)

// OnRouteHandler Handlers define a function to create hooks for Fiber.
type (
	OnRouteHandler     = func(Route) error
	OnNameHandler      = OnRouteHandler
	OnGroupHandler     = func(Group) error
	OnGroupNameHandler = OnGroupHandler
	OnListenHandler    = func(ListenData) error
	OnShutdownHandler  = func() error
	OnForkHandler      = func(int) error
	OnMountHandler     = func(*App) error
)

// Hooks is a struct to use it with App.
type Hooks struct {
	// Embed app
	app *App

	// Hooks
	onRoute     []OnRouteHandler
	onName      []OnNameHandler
	onGroup     []OnGroupHandler
	onGroupName []OnGroupNameHandler
	onListen    []OnListenHandler
	onShutdown  []OnShutdownHandler
	onFork      []OnForkHandler
	onMount     []OnMountHandler
}

// ListenData is a struct to use it with OnListenHandler
type ListenData struct {
	Host string
	Port string
	TLS  bool
}

func newHooks(app *App) *Hooks {
	return &Hooks{
		app:         app,
		onRoute:     make([]OnRouteHandler, 0),
		onGroup:     make([]OnGroupHandler, 0),
		onGroupName: make([]OnGroupNameHandler, 0),
		onName:      make([]OnNameHandler, 0),
		onListen:    make([]OnListenHandler, 0),
		onShutdown:  make([]OnShutdownHandler, 0),
		onFork:      make([]OnForkHandler, 0),
		onMount:     make([]OnMountHandler, 0),
	}
}

// OnRoute is a hook to execute user functions on each route registeration.
// Also you can get route properties by route parameter.
func (h *Hooks) OnRoute(handler ...OnRouteHandler) {
	h.app.mutex.Lock()
	h.onRoute = append(h.onRoute, handler...)
	h.app.mutex.Unlock()
}

// OnName is a hook to execute user functions on each route naming.
// Also you can get route properties by route parameter.
//
// WARN: OnName only works with naming routes, not groups.
func (h *Hooks) OnName(handler ...OnNameHandler) {
	h.app.mutex.Lock()
	h.onName = append(h.onName, handler...)
	h.app.mutex.Unlock()
}

// OnGroup is a hook to execute user functions on each group registeration.
// Also you can get group properties by group parameter.
func (h *Hooks) OnGroup(handler ...OnGroupHandler) {
	h.app.mutex.Lock()
	h.onGroup = append(h.onGroup, handler...)
	h.app.mutex.Unlock()
}

// OnGroupName is a hook to execute user functions on each group naming.
// Also you can get group properties by group parameter.
//
// WARN: OnGroupName only works with naming groups, not routes.
func (h *Hooks) OnGroupName(handler ...OnGroupNameHandler) {
	h.app.mutex.Lock()
	h.onGroupName = append(h.onGroupName, handler...)
	h.app.mutex.Unlock()
}

// OnListen is a hook to execute user functions on Listen, ListenTLS, Listener.
func (h *Hooks) OnListen(handler ...OnListenHandler) {
	h.app.mutex.Lock()
	h.onListen = append(h.onListen, handler...)
	h.app.mutex.Unlock()
}

// OnShutdown is a hook to execute user functions after Shutdown.
func (h *Hooks) OnShutdown(handler ...OnShutdownHandler) {
	h.app.mutex.Lock()
	h.onShutdown = append(h.onShutdown, handler...)
	h.app.mutex.Unlock()
}

// OnFork is a hook to execute user function after fork process.
func (h *Hooks) OnFork(handler ...OnForkHandler) {
	h.app.mutex.Lock()
	h.onFork = append(h.onFork, handler...)
	h.app.mutex.Unlock()
}

// OnMount is a hook to execute user function after mounting process.
// The mount event is fired when sub-app is mounted on a parent app. The parent app is passed as a parameter.
// It works for app and group mounting.
func (h *Hooks) OnMount(handler ...OnMountHandler) {
	h.app.mutex.Lock()
	h.onMount = append(h.onMount, handler...)
	h.app.mutex.Unlock()
}

func (h *Hooks) executeOnRouteHooks(route Route) error {
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

func (h *Hooks) executeOnNameHooks(route Route) error {
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

func (h *Hooks) executeOnGroupHooks(group Group) error {
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

func (h *Hooks) executeOnGroupNameHooks(group Group) error {
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

func (h *Hooks) executeOnListenHooks(listenData ListenData) error {
	for _, v := range h.onListen {
		if err := v(listenData); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnShutdownHooks() {
	for _, v := range h.onShutdown {
		if err := v(); err != nil {
			log.Errorf("failed to call shutdown hook: %v", err)
		}
	}
}

func (h *Hooks) executeOnForkHooks(pid int) {
	for _, v := range h.onFork {
		if err := v(pid); err != nil {
			log.Errorf("failed to call fork hook: %v", err)
		}
	}
}

func (h *Hooks) executeOnMountHooks(app *App) error {
	for _, v := range h.onMount {
		if err := v(app); err != nil {
			return err
		}
	}

	return nil
}
