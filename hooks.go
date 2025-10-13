package fiber

import (
	"fmt"
	"sort"

	"github.com/gofiber/fiber/v3/log"
)

type (
	// OnRouteHandler defines the hook signature invoked whenever a route is registered.
	OnRouteHandler = func(Route) error
	// OnNameHandler shares the OnRouteHandler signature for route naming callbacks.
	OnNameHandler = OnRouteHandler
	// OnGroupHandler defines the hook signature invoked whenever a group is registered.
	OnGroupHandler = func(Group) error
	// OnGroupNameHandler shares the OnGroupHandler signature for group naming callbacks.
	OnGroupNameHandler = OnGroupHandler
	// OnListenHandler runs when the application begins listening and receives the listener details.
	OnListenHandler = func(ListenData) error
	// OnPreStartupMessageHandler runs before Fiber prints the startup banner.
	OnPreStartupMessageHandler = func(*PreStartupMessageData) error
	// OnPostStartupMessageHandler runs after Fiber prints (or skips) the startup banner.
	OnPostStartupMessageHandler = func(PostStartupMessageData) error
	// OnPreShutdownHandler runs before the application shuts down.
	OnPreShutdownHandler = func() error
	// OnPostShutdownHandler runs after shutdown and receives the shutdown result.
	OnPostShutdownHandler = func(error) error
	// OnForkHandler runs inside a forked worker process and receives the worker ID.
	OnForkHandler = func(int) error
	// OnMountHandler runs after a sub-application mounts to a parent and receives the parent app reference.
	OnMountHandler = func(*App) error
)

// Hooks is a struct to use it with App.
type Hooks struct {
	// Embed app
	app *App

	// Hooks
	onRoute        []OnRouteHandler
	onName         []OnNameHandler
	onGroup        []OnGroupHandler
	onGroupName    []OnGroupNameHandler
	onListen       []OnListenHandler
	onPreStartup   []OnPreStartupMessageHandler
	onPostStartup  []OnPostStartupMessageHandler
	onPreShutdown  []OnPreShutdownHandler
	onPostShutdown []OnPostShutdownHandler
	onFork         []OnForkHandler
	onMount        []OnMountHandler
}

// startupMessageEntry represents a single line of startup message information.
type startupMessageEntry struct {
	key   string
	value string
}

// ListenData contains the listener metadata provided to OnListenHandler.
type ListenData struct {
	ColorScheme Colors
	Host        string
	Port        string
	Version     string
	AppName     string

	ChildPIDs []int

	HandlerCount int
	ProcessCount int
	PID          int

	TLS     bool
	Prefork bool
}

// PreStartupMessageData contains metadata exposed to OnPreStartupMessage hooks.
type PreStartupMessageData struct {
	PrimaryInfo   Map
	SecondaryInfo Map

	Header string

	ColorScheme Colors
	Host        string
	Port        string
	Version     string
	AppName     string

	ChildPIDs []int

	HandlerCount int
	ProcessCount int
	PID          int

	TLS            bool
	Prefork        bool
	PreventDefault bool
	HeaderSet      bool
}

func newPreStartupMessageData(listenData ListenData) *PreStartupMessageData {
	var childPIDs []int
	if len(listenData.ChildPIDs) > 0 {
		childPIDs = append(childPIDs, listenData.ChildPIDs...)
	}

	return &PreStartupMessageData{
		Host:         listenData.Host,
		Port:         listenData.Port,
		Version:      listenData.Version,
		AppName:      listenData.AppName,
		ColorScheme:  listenData.ColorScheme,
		ChildPIDs:    childPIDs,
		HandlerCount: listenData.HandlerCount,
		ProcessCount: listenData.ProcessCount,
		PID:          listenData.PID,
		TLS:          listenData.TLS,
		Prefork:      listenData.Prefork,
	}
}

// PostStartupMessageData contains metadata exposed to OnPostStartupMessage hooks.
type PostStartupMessageData struct {
	Host    string
	Port    string
	Version string
	AppName string

	ColorScheme Colors
	ChildPIDs   []int

	HandlerCount int
	ProcessCount int
	PID          int

	TLS       bool
	Prefork   bool
	Printed   bool
	Disabled  bool
	Prevented bool
	IsChild   bool
}

func newPostStartupMessageData(listenData ListenData, printed, disabled, prevented, isChild bool) PostStartupMessageData {
	var childPIDs []int
	if len(listenData.ChildPIDs) > 0 {
		childPIDs = append(childPIDs, listenData.ChildPIDs...)
	}

	return PostStartupMessageData{
		Host:         listenData.Host,
		Port:         listenData.Port,
		Version:      listenData.Version,
		AppName:      listenData.AppName,
		ColorScheme:  listenData.ColorScheme,
		ChildPIDs:    childPIDs,
		HandlerCount: listenData.HandlerCount,
		ProcessCount: listenData.ProcessCount,
		PID:          listenData.PID,
		TLS:          listenData.TLS,
		Prefork:      listenData.Prefork,
		Printed:      printed,
		Disabled:     disabled,
		Prevented:    prevented,
		IsChild:      isChild,
	}
}

func mapToEntries(values Map) ([]startupMessageEntry, bool) {
	if len(values) == 0 {
		return nil, false
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	entries := make([]startupMessageEntry, 0, len(values))
	for _, key := range keys {
		entries = append(entries, startupMessageEntry{key: key, value: fmt.Sprint(values[key])})
	}

	return entries, true
}

func newHooks(app *App) *Hooks {
	return &Hooks{
		app:            app,
		onRoute:        make([]OnRouteHandler, 0),
		onGroup:        make([]OnGroupHandler, 0),
		onGroupName:    make([]OnGroupNameHandler, 0),
		onName:         make([]OnNameHandler, 0),
		onListen:       make([]OnListenHandler, 0),
		onPreStartup:   make([]OnPreStartupMessageHandler, 0),
		onPostStartup:  make([]OnPostStartupMessageHandler, 0),
		onPreShutdown:  make([]OnPreShutdownHandler, 0),
		onPostShutdown: make([]OnPostShutdownHandler, 0),
		onFork:         make([]OnForkHandler, 0),
		onMount:        make([]OnMountHandler, 0),
	}
}

// OnRoute is a hook to execute user functions on each route registration.
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

// OnGroup is a hook to execute user functions on each group registration.
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

// OnPreStartupMessage is a hook to execute user functions before the startup message is printed.
func (h *Hooks) OnPreStartupMessage(handler ...OnPreStartupMessageHandler) {
	h.app.mutex.Lock()
	h.onPreStartup = append(h.onPreStartup, handler...)
	h.app.mutex.Unlock()
}

// OnPostStartupMessage is a hook to execute user functions after the startup message is printed (or skipped).
func (h *Hooks) OnPostStartupMessage(handler ...OnPostStartupMessageHandler) {
	h.app.mutex.Lock()
	h.onPostStartup = append(h.onPostStartup, handler...)
	h.app.mutex.Unlock()
}

// OnPreShutdown is a hook to execute user functions before Shutdown.
func (h *Hooks) OnPreShutdown(handler ...OnPreShutdownHandler) {
	h.app.mutex.Lock()
	h.onPreShutdown = append(h.onPreShutdown, handler...)
	h.app.mutex.Unlock()
}

// OnPostShutdown is a hook to execute user functions after Shutdown.
func (h *Hooks) OnPostShutdown(handler ...OnPostShutdownHandler) {
	h.app.mutex.Lock()
	h.onPostShutdown = append(h.onPostShutdown, handler...)
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

func (h *Hooks) executeOnPreStartupMessageHooks(data *PreStartupMessageData) error {
	for _, handler := range h.onPreStartup {
		if err := handler(data); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnPostStartupMessageHooks(data PostStartupMessageData) error {
	for _, handler := range h.onPostStartup {
		if err := handler(data); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnPreShutdownHooks() {
	for _, v := range h.onPreShutdown {
		if err := v(); err != nil {
			log.Errorf("failed to call pre shutdown hook: %v", err)
		}
	}
}

func (h *Hooks) executeOnPostShutdownHooks(err error) {
	for _, v := range h.onPostShutdown {
		if hookErr := v(err); hookErr != nil {
			log.Errorf("failed to call post shutdown hook: %v", hookErr)
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
