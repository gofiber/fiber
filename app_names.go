package fiber

import (
	"sync/atomic"
)

// applyToCursor applies a documentation change to the registration cursor points at. A nil cursor is the
// app's own and targets its latest registration; a cursor still at 0 belongs to a router that registered
// nothing yet, and changes nothing rather than retargeting the app's latest route.
func (app *App) applyToCursor(cursor *atomic.Uint64, apply func(route *Route)) {
	if apply == nil {
		return
	}
	app.mutex.Lock()
	defer app.mutex.Unlock()

	regID := app.latestRegID
	if cursor != nil {
		regID = cursor.Load()
	}
	if regID == 0 {
		return
	}
	app.applyAndBumpLocked(regID, apply)
}

// applyAndBumpLocked applies a mutation to the entries of regID and bumps the routes revision; the caller must hold app.mutex.
func (app *App) applyAndBumpLocked(regID uint64, apply func(route *Route)) {
	if app.applyToRegIDLocked(regID, apply) {
		app.bumpRoutesRevision()
	}
}

func (app *App) applyNameToRegistration(regID uint64, name string) {
	if regID == 0 {
		return
	}

	app.mutex.Lock()
	named := app.nameRegistrationLocked(regID, name)
	app.mutex.Unlock()

	app.fireOnNameHooks(named)
}

// nameRegistrationLocked names the entries of regID and returns a snapshot of the last for the OnName hooks.
func (app *App) nameRegistrationLocked(regID uint64, name string) *Route {
	named := app.nameRoutesLocked(regID, name)
	if named == nil {
		return nil
	}
	app.bumpRoutesRevision()
	if len(app.hooks.onName) == 0 {
		return nil
	}
	// Snapshot under the lock: the hook runs without it.
	return app.copyRoute(named)
}

func (app *App) fireOnNameHooks(named *Route) {
	if named == nil {
		return
	}
	if err := app.hooks.executeOnNameHooks(named); err != nil {
		panic(err)
	}
}

// applyToRegIDLocked applies a mutation to every entry of regID except mount placeholders.
func (app *App) applyToRegIDLocked(regID uint64, apply func(route *Route)) bool {
	applied := false
	for _, route := range app.regEntries[regID] {
		if route.mount {
			continue
		}
		apply(route)
		applied = true
	}
	return applied
}

// nameRoutesLocked names the entries of regID and the auto-HEAD twin of each GET, returning the last named.
func (app *App) nameRoutesLocked(regID uint64, name string) *Route {
	var (
		gets  []*Route
		named *Route
	)
	setName := func(route *Route) {
		route.Name = name
		if route.group != nil {
			route.Name = route.group.name + name
		}
	}
	app.applyToRegIDLocked(regID, func(route *Route) {
		setName(route)
		if route.Method == MethodGet && !route.use {
			gets = append(gets, route)
		}
		named = route
	})
	// Naming a middleware registration names every middleware route at its path.
	if named != nil && named.use {
		for _, routes := range app.stack {
			for _, route := range routes {
				if route.mount || !route.use || route.Path != named.Path || route == named || matchersKey(route.hosts) != matchersKey(named.hosts) {
					continue
				}
				setName(route)
			}
		}
	}
	if len(gets) == 0 {
		return named
	}
	headIndex := app.methodInt(MethodHead)
	if headIndex == -1 {
		return named
	}
	for _, get := range gets {
		if _, twin := app.autoHeadTwinLocked(headIndex, app.autoHeadKey(get)); twin != nil {
			twin.Name = get.Name
		}
	}
	return named
}

// namedRouteIndex is an immutable by-name snapshot of the routes at one revision.
type namedRouteIndex struct {
	routes   map[string]*Route
	live     map[*Route]*Route // snapshot of a documented route -> the route it was taken from, for GetRoute to clone from
	small    []*Route          // the same snapshots in registration order when there are few, to scan instead of hash
	revision uint64
}

// currentNamedIndex returns the by-name index for the current routes revision, rebuilding it when stale.
func (app *App) currentNamedIndex() *namedRouteIndex {
	index := app.namedRoutes.Load()
	if index == nil || index.revision != app.routesRevision.Load() {
		index = app.indexNamedRoutes()
	}
	return index
}

// lookup returns the snapshot of the first route called name, or nil.
func (index *namedRouteIndex) lookup(name string) *Route {
	if index.small != nil {
		for _, route := range index.small {
			if route.Name == name {
				return route
			}
		}
		return nil
	}
	return index.routes[name]
}

// namedRoute returns the shared snapshot of the route called name, or nil; callers must not modify it.
func (app *App) namedRoute(name string) *Route {
	return app.currentNamedIndex().lookup(name)
}

// indexNamedRoutes rebuilds the index unless another goroutine already has.
func (app *App) indexNamedRoutes() *namedRouteIndex {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	return app.indexNamedRoutesLocked()
}

// indexNamedRoutesLocked is indexNamedRoutes for a caller holding app.mutex. A snapshot of a documented
// route carries no documentation, which only GetRoute reads and clones from the live route.
func (app *App) indexNamedRoutesLocked() *namedRouteIndex {
	revision := app.routesRevision.Load()
	if index := app.namedRoutes.Load(); index != nil && index.revision == revision {
		return index
	}

	index := &namedRouteIndex{revision: revision, routes: make(map[string]*Route), live: make(map[*Route]*Route)}
	var inOrder []*Route
	for _, routes := range app.stack {
		for _, route := range routes {
			if _, taken := index.routes[route.Name]; taken {
				continue
			}
			var snapshot *Route
			if route.isDocumented() {
				snapshot = app.copyRouteBase(route)
				index.live[snapshot] = route
			} else {
				snapshot = app.copyRoute(route)
			}
			index.routes[route.Name] = snapshot
			inOrder = append(inOrder, snapshot)
		}
	}
	if len(inOrder) <= smallIndexMax {
		index.small = inOrder
		if index.small == nil {
			index.small = []*Route{}
		}
	}
	app.namedRoutes.Store(index)
	return index
}

// GetRoute returns a copy of the route with the given name; Handlers and Params are shared and must not be modified.
func (app *App) GetRoute(name string) (found Route) { //nolint:nonamedreturns // the named result is what keeps this to a single struct move
	index := app.currentNamedIndex()
	snapshot := index.lookup(name)
	if snapshot == nil {
		return found
	}
	live := index.live[snapshot]
	if live == nil {
		return *snapshot
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.copyRouteInto(&found, live)
	return found
}
