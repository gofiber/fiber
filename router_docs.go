package fiber

// copyRoute clones a route without its group, so Name() does not prefix it with the source app's group name.
// The omission is load-bearing for a mount placeholder, which domainRouter.cloneRoutesForDomain expands instead.
func (app *App) copyRoute(route *Route) *Route {
	copied := new(Route)
	app.copyRouteInto(copied, route)
	return copied
}

// copyRouteInto copies route into dst; Handlers, Params and annotations are shared.
func (*App) copyRouteInto(dst, route *Route) {
	*dst = *route
	dst.group = nil
}

// copyRouteBase is copyRoute without what was attached to the route after it was registered: its annotations
// and the handlers it documents.
func (app *App) copyRouteBase(route *Route) *Route {
	copied := new(Route)
	app.copyRouteInto(copied, route)
	copied.docHandlers = nil
	copied.annotations = nil
	return copied
}
