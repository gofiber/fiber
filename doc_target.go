package fiber

import (
	"maps"
	"sync/atomic"
)

// docTarget is what App, Group, domainRouter and Registering share to document the route they registered last.
type docTarget[R any] struct {
	// cursor is the latest registration, shared by copies of the owner; nil on App.
	cursor *atomic.Uint64
	app    *App
	self   R
}

func (t *docTarget[R]) document(apply func(route *Route)) R {
	t.app.applyToCursor(t.cursor, apply)
	return t.self
}

// Annotate replaces the value stored under key on the routes of the latest registration with
// update(current), where current is the value stored before or nil. A stored value must not be changed
// afterwards: copies of a route, such as the ones GetRoutes returns, share it. Packages that keep their own
// metadata on a route, like the OpenAPI middleware, use an unexported key type so their keys cannot clash.
func (t *docTarget[R]) Annotate(key any, update func(current any) any) {
	t.document(func(route *Route) {
		next := make(map[any]any, len(route.annotations)+1)
		maps.Copy(next, route.annotations)
		next[key] = update(route.annotations[key])
		route.annotations = next
	})
}

func newGroup(app *App, prefix string, parent *Group) *Group {
	grp := &Group{Prefix: prefix, parentGroup: parent}
	grp.docTarget = docTarget[Router]{cursor: new(atomic.Uint64), app: app, self: grp}
	return grp
}

//nolint:gocritic // hugeParam: the matcher is stored by value.
func newDomainRouter(app *App, group *Group, matcher domainMatcher, lastRegID uint64) *domainRouter {
	d := &domainRouter{group: group, matcher: matcher}
	d.docTarget = docTarget[Router]{cursor: new(atomic.Uint64), app: app, self: d}
	d.cursor.Store(lastRegID)
	return d
}

func newRegistering(app *App, group *Group, domain *domainRouter, path string) *Registering {
	r := &Registering{group: group, domain: domain, path: path}
	r.docTarget = docTarget[Register]{cursor: new(atomic.Uint64), app: app, self: r}
	return r
}
