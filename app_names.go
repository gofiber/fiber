package fiber

func (app *App) applyToLatest(apply func(route *Route)) {
	app.mutex.Lock()
	if app.applyToRegIDLocked(app.latestRegID, apply) {
		app.bumpRoutesRevision()
	}
	app.mutex.Unlock()
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

// nameRegistrationLocked names the entries of regID and returns a snapshot of the
// last for the OnName hooks, or nil. The caller must hold app.mutex.
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

// fireOnNameHooks runs the OnName hooks, panicking on error like registration. Callers must not hold app.mutex.
func (app *App) fireOnNameHooks(named *Route) {
	if named == nil {
		return
	}
	if err := app.hooks.executeOnNameHooks(named); err != nil {
		panic(err)
	}
}

func (app *App) applyToRegistration(regID uint64, apply func(route *Route)) {
	if regID == 0 || apply == nil {
		return
	}
	app.mutex.Lock()
	if app.applyToRegIDLocked(regID, apply) {
		app.bumpRoutesRevision()
	}
	app.mutex.Unlock()
}

// applyToRegIDLocked applies a mutation to every entry of regID except mount
// placeholders, and reports whether any was touched. The caller must hold app.mutex.
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

// nameRoutesLocked names the entries of regID and the automatic HEAD twin of each
// GET, returning the last named. The caller must hold app.mutex.
func (app *App) nameRoutesLocked(regID uint64, name string) *Route {
	var (
		gets  []*Route
		named *Route
	)
	app.applyToRegIDLocked(regID, func(route *Route) {
		route.Name = name
		if route.group != nil {
			route.Name = route.group.name + route.Name
		}
		if route.Method == MethodGet && !route.use {
			gets = append(gets, route)
		}
		named = route
	})
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

// namedRouteIndex is an immutable by-name snapshot of the routes at one revision;
// the first route in stack order wins for each name.
type namedRouteIndex struct {
	routes   map[string]*Route
	small    []*Route // the same snapshots in registration order when there are few, to scan instead of hash
	revision uint64
}

// namedRoute returns the shared snapshot of the route called name, or nil; callers
// must not modify it. The index is rebuilt under the lock after a change and read
// lock-free, as published snapshots are never mutated.
func (app *App) namedRoute(name string) *Route {
	index := app.namedRoutes.Load()
	if index == nil || index.revision != app.routesRevision.Load() {
		index = app.indexNamedRoutes()
	}
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

// indexNamedRoutes rebuilds the index unless another goroutine already has. Revision
// bumps happen under the lock, so a build under it matches its revision.
func (app *App) indexNamedRoutes() *namedRouteIndex {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	revision := app.routesRevision.Load()
	if index := app.namedRoutes.Load(); index != nil && index.revision == revision {
		return index
	}

	index := &namedRouteIndex{revision: revision, routes: make(map[string]*Route)}
	var inOrder []*Route
	for _, routes := range app.stack {
		for _, route := range routes {
			if _, taken := index.routes[route.Name]; taken {
				continue
			}
			snapshot := new(Route)
			app.copyRouteInto(snapshot, route)
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

// GetRoute returns a copy of the route with the given name. Its documentation metadata is
// cloned; Handlers and Params still share their backing arrays with the app and must not be modified.
func (app *App) GetRoute(name string) (found Route) { //nolint:nonamedreturns // the named result is what keeps this to a single struct move
	snapshot := app.namedRoute(name)
	if snapshot == nil {
		return found
	}
	found = *snapshot
	found.group = nil
	if snapshot.isDocumented() {
		app.cloneRouteDocInto(&found, snapshot)
	}
	return found
}
