package fiber

import (
	"fmt"
	"sort"
	"sync"

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

// startupMessageState stores customization data for the startup message.
type startupMessageState struct {
	header       string
	hasHeader    bool
	primary      []startupMessageEntry
	hasPrimary   bool
	secondary    []startupMessageEntry
	hasSecondary bool
	prevent      bool

	afterPrint chan struct{}
	closeOnce  sync.Once
}

func newStartupMessageState() *startupMessageState {
	return &startupMessageState{
		afterPrint: make(chan struct{}),
	}
}

func (s *startupMessageState) setHeader(header string) {
	s.header = header
	s.hasHeader = true
}

func (s *startupMessageState) setPrimary(values Map) {
	s.primary, s.hasPrimary = mapToEntries(values)
}

func (s *startupMessageState) setSecondary(values Map) {
	s.secondary, s.hasSecondary = mapToEntries(values)
}

func (s *startupMessageState) preventDefault() {
	s.prevent = true
}

func (s *startupMessageState) closeAfterPrint() {
	s.closeOnce.Do(func() {
		close(s.afterPrint)
	})
}

// ListenData contains the listener metadata provided to OnListenHandler.
type ListenData struct {
	Host string
	Port string
	TLS  bool

	Version      string
	AppName      string
	HandlerCount int
	ProcessCount int
	PID          int
	Prefork      bool
	ChildPIDs    []int
	ColorScheme  Colors

	startupMessage *startupMessageState
}

// PreventDefault stops Fiber from printing the default startup message.
func (l ListenData) PreventDefault() {
	if l.startupMessage == nil {
		return
	}

	l.startupMessage.preventDefault()
}

// UseHeader overrides the startup message header. Provide a value that includes any desired
// newlines or separators. The default ASCII art is used when this method is not called.
func (l ListenData) UseHeader(header string) {
	if l.startupMessage == nil {
		return
	}

	l.startupMessage.setHeader(header)
}

// UsePrimaryInfoMap replaces the default primary startup information lines with the provided map.
// Keys are rendered in lexicographical order for deterministic output.
func (l ListenData) UsePrimaryInfoMap(values Map) {
	if l.startupMessage == nil {
		return
	}

	l.startupMessage.setPrimary(values)
}

// UseSecondaryInfoMap replaces the default secondary startup information lines with the provided map.
// Keys are rendered in lexicographical order for deterministic output.
func (l ListenData) UseSecondaryInfoMap(values Map) {
	if l.startupMessage == nil {
		return
	}

	l.startupMessage.setSecondary(values)
}

// AfterPrint returns a channel that is closed once the startup message has been printed (or skipped).
func (l ListenData) AfterPrint() <-chan struct{} {
	if l.startupMessage == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return l.startupMessage.afterPrint
}

func (l ListenData) finishStartupMessage() {
	if l.startupMessage == nil {
		return
	}

	l.startupMessage.closeAfterPrint()
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

func (h *Hooks) executeOnRouteHooks(route *Route) error {
	if route == nil {
		return nil
	}

	cloned := *route

	// Check mounting
	if h.app.mountFields.mountPath != "" {
		cloned.path = h.app.mountFields.mountPath + cloned.path
		cloned.Path = cloned.path
	}

	for _, v := range h.onRoute {
		if err := v(cloned); err != nil {
			return err
		}
	}

	return nil
}

func (h *Hooks) executeOnNameHooks(route *Route) error {
	if route == nil {
		return nil
	}

	cloned := *route

	// Check mounting
	if h.app.mountFields.mountPath != "" {
		cloned.path = h.app.mountFields.mountPath + cloned.path
		cloned.Path = cloned.path
	}

	for _, v := range h.onName {
		if err := v(cloned); err != nil {
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
