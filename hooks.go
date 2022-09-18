package fiber

// OnRouteHandler Handlers define a function to create hooks for Fiber.
type OnRouteHandler = func(Route) error
type OnNameHandler = OnRouteHandler
type OnGroupHandler = func(Group) error
type OnGroupNameHandler = OnGroupHandler
type OnListenHandler = func() error
type OnShutdownHandler = OnListenHandler
type OnForkHandler = func(int) error

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

func (h *Hooks) executeOnRouteHooks(route Route) error {
	for _, v := range h.onRoute {
		if err := v(route); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnNameHooks(route Route) error {
	for _, v := range h.onName {
		if err := v(route); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnGroupHooks(group Group) error {
	for _, v := range h.onGroup {
		if err := v(group); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnGroupNameHooks(group Group) error {
	for _, v := range h.onGroupName {
		if err := v(group); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnListenHooks() error {
	for _, v := range h.onListen {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnShutdownHooks() {
	for _, v := range h.onShutdown {
		_ = v()
	}
}

func (h *Hooks) executeOnForkHooks(pid int) {
	for _, v := range h.onFork {
		_ = v(pid)
	}
}
