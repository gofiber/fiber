// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/utils/v2"
)

// Put fields related to mounting.
type mountFields struct {
	// Mounted and main apps
	appList map[string]*App
	// Prefix of app if it was mounted
	mountPath string
	// Apps mounted through a domain router. They are kept out of appList
	// because that map is keyed by path alone: two apps mounted at the same
	// path for different hosts would displace each other, and a lookup would
	// answer for hosts the domain pattern rejects.
	domainAppList []domainMountedApp
	// Routes that arrived from a mount which registers no automatic HEAD
	// routes, kept by route because a path can belong to a mount and to this
	// app at once
	noAutoHeadRoutes map[*Route]struct{}
	// Mounted app each cloned route came from, so the request that ran one
	// resolves its config directly instead of inferring an owner from the host
	// and the path
	routeOwners map[*Route]*App
	// Constraints each cloned route was parsed with, which are the ones of the
	// apps its path was composed by. Two sibling mounts can name one constraint
	// differently, and a route is only ever bound by the one its own app has.
	routeConstraints map[*Route][]CustomConstraint
	// Ordered keys of apps (sorted by key length for Render)
	appListKeys []string
	// guards one-time generation of appListKeys
	appListKeysOnce sync.Once
	// check added routes of sub-apps
	subAppsRoutesAdded sync.Once
	// check mounted sub-apps
	subAppsProcessed sync.Once
	// Whether this app's routes only answer for the hostnames the mount they
	// came from matches, which is true of the wrapper a domain mount builds
	hostScopedRoutes bool
}

// domainMountedApp is a sub-app mounted through a domain router. Its config —
// the ErrorHandler and the view engine — applies only to requests whose
// hostname matches every domain pattern the app was mounted behind.
type domainMountedApp struct {
	app *App
	// parser matches path when it carries parameters, and is nil otherwise: a
	// mount can be registered at "/:tenant" the same way a route can
	parser *routeParser
	// path is the full mount path, normalized by the app it is mounted on
	path string
	// matchers are the patterns of the domain routers this app was mounted
	// behind, outermost last. A request has to satisfy all of them.
	matchers []domainMatcher
	// ancestors are the apps this one is mounted inside, outermost first. They
	// are what its configuration is inherited from: a mount that merely covers
	// the same path encloses nothing.
	ancestors []*App
	// declared are the constraints of the apps whose mount prefixes the path is
	// composed of, which is where a constraint named in one of them is defined
	declared []CustomConstraint
}

// newDomainMount records a mount of mounted at path, normalized and parsed
// with the routing rules of the app it is being mounted on, so the lookups
// recognize it the same way the router recognizes its routes.
func (app *App) newDomainMount(mounted *App, path string, matchers []domainMatcher, ancestors []*App, declared []CustomConstraint) domainMountedApp {
	normalized := app.normalizePath(path)

	mount := domainMountedApp{
		app:       mounted,
		path:      normalized,
		matchers:  matchers,
		ancestors: ancestors,
		declared:  declared,
	}

	// A mount prefix is written in the terms of the app it was registered on,
	// and may name a constraint only that app knows. Those win where both
	// define a name, as they do when a mount's routes are re-parsed.
	constraints := mergeCustomConstraints(slices.Clone(declared), app.customConstraints)

	if parser := parseRoute(normalized, app.config.RegexHandler, constraints...); len(parser.params) > 0 {
		mount.parser = &parser
	}

	return mount
}

// covers reports whether this mount covers path, which is matched as a pattern
// when the mount path carries parameters.
func (m *domainMountedApp) covers(path string) bool {
	if m.parser != nil {
		var params [maxParams]string

		return m.parser.getMatch(path, path, &params, true)
	}

	return strings.HasPrefix(utils.AddTrailingSlashString(path), utils.AddTrailingSlashString(m.path))
}

// matchesHost reports whether hostname satisfies every domain pattern the app
// was mounted behind.
func (m *domainMountedApp) matchesHost(hostname string) bool {
	for i := range m.matchers {
		if matched, _ := m.matchers[i].match(hostname); !matched {
			return false
		}
	}

	return true
}

// equals reports whether two records describe the same mount: the same app,
// at the same path, behind the same domain patterns.
func (m *domainMountedApp) equals(other *domainMountedApp) bool {
	if m.app != other.app || m.path != other.path || len(m.matchers) != len(other.matchers) {
		return false
	}

	for i := range m.matchers {
		// parts carries the pattern the rest of the matcher is derived from.
		if !slices.Equal(m.matchers[i].parts, other.matchers[i].parts) {
			return false
		}
	}

	return true
}

// depth counts the path segments a mount path covers, so the deepest mount
// covering a request can be picked. The root mount covers none of its own,
// which leaves any named mount more specific than it.
func mountDepth(path string) int {
	if path == "" || path == "/" {
		return 0
	}

	return strings.Count(path, "/")
}

// depth counts the path segments this mount covers.
func (m *domainMountedApp) depth() int {
	return mountDepth(m.path)
}

// Create empty mountFields instance
func newMountFields(app *App) *mountFields {
	return &mountFields{
		appList:     map[string]*App{"": app},
		appListKeys: make([]string, 0),
	}
}

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount. The fiber's error handler and
// any of the fiber's sub apps are added to the application's error handlers
// to be invoked on errors that happen within the prefix route.
func (app *App) mount(prefix string, subApp *App) Router {
	prefix = utils.TrimRight(prefix, '/')
	if prefix == "" {
		prefix = "/"
	}

	app.mutex.Lock()
	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range subApp.mountFields.appList {
		path := getGroupPath(prefix, mountedPrefixes)

		subApp.mountFields.mountPath = path
		app.mountFields.appList[path] = subApp
	}
	app.mutex.Unlock()

	// register mounted group
	mountGroup := &Group{Prefix: prefix, app: subApp}
	app.register([]string{methodUse}, prefix, mountGroup, "")

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(app); err != nil {
		panic(err)
	}

	return app
}

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount.
func (grp *Group) mount(prefix string, subApp *App) Router {
	groupPath := getGroupPath(grp.Prefix, prefix)
	groupPath = utils.TrimRight(groupPath, '/')
	if groupPath == "" {
		groupPath = "/"
	}

	grp.app.mutex.Lock()
	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range subApp.mountFields.appList {
		path := getGroupPath(groupPath, mountedPrefixes)

		subApp.mountFields.mountPath = path
		grp.app.mountFields.appList[path] = subApp
	}
	grp.app.mutex.Unlock()

	// register mounted group
	mountGroup := &Group{Prefix: groupPath, app: subApp}
	// Advance the group's cursor onto the mount, matching App.Use. Leaving it
	// behind would retarget a chained helper at the route registered before it.
	atomic.StoreUint64(&grp.lastRegID, grp.app.register([]string{methodUse}, groupPath, mountGroup, ""))

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(grp.app); err != nil {
		panic(err)
	}

	return grp
}

// domainOwner is the domain-mounted app a request belongs to, and how deep the
// mount it was reached through is.
type domainOwner struct {
	app *App
	// ancestors are the apps the owner is mounted inside, outermost first, and
	// are what it inherits configuration from.
	ancestors []*App
	// depth is how many path segments the mount covers, which ranks the owner
	// against the plain mounts covering the same request.
	depth int
	// exact marks an owner taken from the route that ran rather than inferred
	// from the host and the path
	exact bool
}

// outranks reports whether this owner should answer for the request instead of
// a plain mount whose path is plainDepth segments deep.
//
// An owner taken from the route that ran always does: that route was served by
// it, and a plain mount is only ever ranked by depth because path is all there
// is to go on. An inferred one wins a tie, having matched the host as well, and
// gives way to a deeper plain mount as a mounted app gives way to one below it.
func (o domainOwner) outranks(plainDepth int) bool {
	return o.app != nil && (o.exact || plainDepth <= o.depth)
}

// supersedes reports whether this owner replaces a plain mount rather than
// inheriting from it. One at or below the owner's own depth did not serve the
// request; only the apps the owner is mounted inside enclose it, and those it
// inherits from however deep they are.
func (o domainOwner) supersedes(plainDepth int, plain *App) bool {
	return o.app != nil && plainDepth >= o.depth && !slices.Contains(o.ancestors, plain)
}

// appHasErrorHandler is the setting a request inherits from the domain mounts
// it was reached through.
func appHasErrorHandler(app *App) bool { return app.configured.ErrorHandler != nil }

// domainMountRender resolves what a domain mount contributes to a render: the
// nearest app configuring a view engine, and the layout of the nearest app at
// or below that one which configures a layout.
//
// The layout search stops where rendering does. An app with an engine of its
// own renders through it and takes no layout from further out, exactly as the
// scan over the ordinary mounts stops at the first engine it covers; only when
// nothing below configures an engine does the layout travel outward with the
// search for one.
func domainMountRender(owner domainOwner) (*App, string) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	if owner.app == nil {
		return nil, ""
	}

	layout := owner.app.config.ViewsLayout
	if owner.app.config.Views != nil {
		return owner.app, layout
	}

	for _, ancestor := range slices.Backward(owner.ancestors) {
		if layout == "" {
			layout = ancestor.config.ViewsLayout
		}

		if ancestor.config.Views != nil {
			return ancestor, layout
		}
	}

	return nil, layout
}

// domainMountConfig returns the app a setting is taken from: the owner when it
// configures one, and otherwise the deepest domain mount enclosing it that
// does — the inheritance an ordinarily nested mount gets from the app it sits
// in. Mounts at the owner's own depth are siblings that did not serve the
// request, and configure nothing on its behalf.
//
// There has to be an owner: nothing outranks a plain mount without one, so a
// caller reaches this having established it.
func domainMountConfig(owner domainOwner, want func(*App) bool) *App {
	if want(owner.app) {
		return owner.app
	}

	// Innermost first, so the nearest app configuring the setting answers. Only
	// the apps the owner is mounted inside are consulted: a mount that reaches
	// the same paths on the same hosts is a sibling, and configures nothing on
	// the owner's behalf.
	for _, ancestor := range slices.Backward(owner.ancestors) {
		if want(ancestor) {
			return ancestor
		}
	}

	return nil
}

// domainMountOwner returns the domain mount that owns this request: the app the
// route that ran was mounted from, or — for an error raised before any of them
// matched — the deepest mount covering the path whose patterns match the host,
// and among equally deep ones the first registered, matching the order their
// routes are matched in.
func (app *App) domainMountOwner(c Ctx) domainOwner {
	// The app the route that ran was mounted from, if it was mounted at all.
	// Its mount still has to match the request: a domain-wrapped handler that
	// declined the host left the route it belongs to as the one that ran, and
	// that app served nothing.
	routeApp := app.routeOwner(c.Route())

	var owner, byRoute domainOwner

	path := app.normalizePath(c.Path())

	for i := range app.mountFields.domainAppList {
		mount := &app.mountFields.domainAppList[i]
		if !mount.covers(path) || !mount.matchesHost(c.Hostname()) {
			continue
		}

		depth := mount.depth()

		// Two sub-apps mounted at one path behind overlapping patterns both
		// cover the request and both match the host. Only one of them ran.
		if mount.app == routeApp && (byRoute.app == nil || depth > byRoute.depth) {
			byRoute = domainOwner{app: mount.app, ancestors: mount.ancestors, depth: depth, exact: true}
		}

		if owner.app == nil || depth > owner.depth {
			owner.app, owner.depth, owner.ancestors = mount.app, depth, mount.ancestors
		}
	}

	if byRoute.app != nil {
		return byRoute
	}

	// A route that names an app none of these mounts describe was served by an
	// ordinary mount, or by one whose host these patterns reject. Either way
	// this request is not a domain mount's to answer for, and only one that ran
	// no mounted route at all — a 404, or an error raised above them — is left
	// to the mount covering its path.
	if routeApp != nil {
		return domainOwner{}
	}

	return owner
}

// mountedApps returns this app and every app mounted below it, plain or on a
// domain, for the operations that touch each mounted app's config.
//
// It walks the mount metadata itself rather than reading the flattened lists,
// which are only complete once the app has been through its startup process.
func (app *App) mountedApps() []*App {
	apps := []*App{app}

	for i := 0; i < len(apps); i++ {
		for _, mounted := range apps[i].mountedAppsSnapshot() {
			if !slices.Contains(apps, mounted) {
				apps = append(apps, mounted)
			}
		}
	}

	return apps
}

// mountedAppRef is an app reachable from another one: the path it is reachable
// at, and the domain patterns a request has to satisfy to get there.
type mountedAppRef struct {
	app       *App
	path      string
	matchers  []domainMatcher
	ancestors []*App
	declared  []CustomConstraint
}

// mountTree returns this app and every app mounted below it, plain or on a
// domain, with the paths and the domain patterns composed along the way.
//
// It walks the mount metadata of each app in turn rather than reading the
// flattened list of one: that list is filled in when an app is mounted, so an
// app mounted on a descendant afterwards is missing from it until startup
// repairs it — and a domain mount records its apps when it is registered.
func (app *App) mountTree() []mountedAppRef {
	walk := mountWalk{seen: make(map[mountWalkKey]int)}
	walk.add(app, "", nil, nil, nil)

	return walk.refs
}

// mountWalk carries the state of one walk over an app's mounts.
type mountWalk struct {
	// seen holds the longest chain each app has already been reached at, per
	// path. An app list holds the descendants of the apps it lists, so a
	// descendant is reachable through every combination of the lists leading to
	// it — walking one again pays for itself only when the chain is longer than
	// one it was reached with before, since only the longest is kept.
	seen map[mountWalkKey]int
	refs []mountedAppRef
}

// mountWalkKey identifies an app at the path it was reached at, behind the
// patterns it was reached through: the same app can be mounted at one path for
// two hostnames, and each of those is a mount of its own.
type mountWalkKey struct {
	app      *App
	path     string
	matchers string
}

// matchersKey identifies a set of domain patterns. Labels carry neither
// separator, so the parts of one pattern and the patterns of one mount cannot
// run together.
func matchersKey(matchers []domainMatcher) string {
	patterns := make([]string, len(matchers))
	for i := range matchers {
		patterns[i] = strings.Join(matchers[i].parts, ".")
	}

	return strings.Join(patterns, "\n")
}

// add records this app and its descendants, reached at prefix
// behind matchers. chain holds the apps above this one so a mount cycle
// terminates; it is a slice because it is only ever a few entries deep, and an
// app mounted on two branches is still reached through both.
func (w *mountWalk) add(app *App, prefix string, matchers []domainMatcher, declared []CustomConstraint, chain []*App) {
	if slices.Contains(chain, app) {
		return
	}

	key := mountWalkKey{app: app, path: prefix, matchers: matchersKey(matchers)}
	if reached, ok := w.seen[key]; ok && reached >= len(chain) {
		return
	}
	w.seen[key] = len(chain)

	w.refs = append(w.refs, mountedAppRef{
		app:       app,
		path:      prefix,
		matchers:  matchers,
		ancestors: slices.Clone(chain),
		declared:  declared,
	})
	chain = append(chain, app)

	// The prefixes below are written in this app's terms, so a constraint only
	// it knows travels with them. Its own win where a name is defined twice,
	// the way a mounted app's do when its routes are re-parsed.
	declared = mergeCustomConstraints(slices.Clone(app.customConstraints), declared)

	// Patterns compose innermost first, the order a mount records them in: a
	// request has to satisfy every one of them.
	for _, mount := range app.domainMountsSnapshot() {
		w.add(
			mount.app,
			getGroupPath(prefix, mount.path),
			append(slices.Clone(mount.matchers), matchers...),
			declared,
			chain,
		)
	}

	for mountPrefix, mounted := range app.plainMountsSnapshot() {
		if mounted == app {
			continue
		}

		w.add(mounted, getGroupPath(prefix, mountPrefix), matchers, declared, chain)
	}
}

// mountedAppsSnapshot returns the apps mounted directly on this one, plain or
// on a domain.
func (app *App) mountedAppsSnapshot() []*App {
	plain := app.plainMountsSnapshot()
	domain := app.domainMountsSnapshot()

	mounted := make([]*App, 0, len(plain)+len(domain))
	for _, sub := range plain {
		mounted = append(mounted, sub)
	}

	for i := range domain {
		mounted = append(mounted, domain[i].app)
	}

	return mounted
}

// plainMountsSnapshot copies this app's list of ordinarily mounted apps, read
// under its own lock: App.mount writes it under that lock, and a read left
// unguarded would race a concurrent mount. Only this app is locked, and only
// for the copy, so a caller may hold no other app's lock — mounting an app on
// itself would otherwise take one lock twice.
func (app *App) plainMountsSnapshot() map[string]*App {
	if app.mountFields == nil {
		return nil
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	return maps.Clone(app.mountFields.appList)
}

// domainMountsSnapshot copies this app's domain mount records, under the same
// lock and with the same restriction as plainMountsSnapshot.
func (app *App) domainMountsSnapshot() []domainMountedApp {
	if app.mountFields == nil {
		return nil
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	return slices.Clone(app.mountFields.domainAppList)
}

// mountCoversPath reports whether a mount registered at mountPath covers path.
//
// Both sides are put through this app's routing rules first: an app list is
// keyed by the path the caller spelled, so on a case-insensitive app a mount
// registered as "/API" has to cover a request for "/api/x" the same way its
// routes do. A parametric mount path is matched literally here — the app list
// keeps no parsed form of its keys.
func (app *App) mountCoversPath(mountPath, path string) bool {
	if mountPath == "" {
		return false
	}

	return strings.HasPrefix(
		utils.AddTrailingSlashString(app.normalizePath(path)),
		utils.AddTrailingSlashString(app.normalizePath(mountPath)),
	)
}

// markSkipAutoHead records that a route must not be given an automatic HEAD
// companion. Provenance is kept by route rather than by path: once a mount is
// expanded, its routes sit in this app's stack next to the app's own, and a
// path can belong to both.
func (app *App) markSkipAutoHead(route *Route) {
	if app.mountFields.noAutoHeadRoutes == nil {
		app.mountFields.noAutoHeadRoutes = make(map[*Route]struct{})
	}

	app.mountFields.noAutoHeadRoutes[route] = struct{}{}
}

// skipsAutoHeadFor reports whether route came from a mount that registers no
// automatic HEAD routes of its own.
func (app *App) skipsAutoHeadFor(route *Route) bool {
	_, ok := app.mountFields.noAutoHeadRoutes[route]

	return ok
}

// markRouteOwner records that route was cloned out of owner, the app it was
// mounted from. Ownership is kept by route for the same reason the automatic
// HEAD provenance is: host and path do not identify the app on their own, since
// two sub-apps can be mounted at one path behind patterns that both match.
//
// Ordinary mounts are recorded as well as domain ones. A sub-app that has
// already expanded its own mounts carries its children's routes in its stack,
// and the domain mount that clones them next would otherwise take them all for
// its own.
func (app *App) markRouteOwner(route *Route, owner *App) {
	if app.mountFields.routeOwners == nil {
		app.mountFields.routeOwners = make(map[*Route]*App)
	}

	app.mountFields.routeOwners[route] = owner
}

// markRouteConstraints records the constraints route was parsed with, so the
// next app to re-parse it uses those rather than a registry merged from every
// app it happens to have mounted.
func (app *App) markRouteConstraints(route *Route, constraints []CustomConstraint) {
	if len(constraints) == 0 {
		return
	}

	if app.mountFields.routeConstraints == nil {
		app.mountFields.routeConstraints = make(map[*Route][]CustomConstraint)
	}

	app.mountFields.routeConstraints[route] = constraints
}

// routeConstraintsFor returns the constraints route was parsed with, falling
// back to those of the app holding it — which is where its own routes get
// theirs from.
func (app *App) routeConstraintsFor(route *Route) []CustomConstraint {
	if recorded, ok := app.mountFields.routeConstraints[route]; ok {
		return recorded
	}

	return app.customConstraints
}

// routeOwner returns the mounted app route was cloned out of, or nil when the
// route did not come from a mount.
func (app *App) routeOwner(route *Route) *App {
	if route == nil || len(app.mountFields.routeOwners) == 0 {
		return nil
	}

	return app.mountFields.routeOwners[route]
}

// appendDomainMounts re-registers the domain mounts of a mounted app on the app
// above it, moving their paths under prefix. Their domain patterns travel with
// them: the host still has to match for their config to apply.
func (app *App) appendDomainMounts(dst, mounts []domainMountedApp, prefix string, owner *App) []domainMountedApp {
	for i := range mounts {
		mount := &mounts[i]

		// The app they were registered on encloses them from here: their own
		// ancestors are the apps between it and them, and it is the outermost.
		ancestors := append([]*App{owner}, mount.ancestors...)

		moved := app.newDomainMount(mount.app, getGroupPath(prefix, mount.path), mount.matchers, ancestors, mount.declared)
		dst = addDomainMount(dst, &moved)
	}

	return dst
}

// addDomainMount appends a domain mount unless the same one is already
// recorded. An app list holds the descendants of the apps it lists, so the
// startup walk reaches the same mount once per path that leads to it.
func addDomainMount(dst []domainMountedApp, mount *domainMountedApp) []domainMountedApp {
	for i := range dst {
		if !dst[i].equals(mount) {
			continue
		}

		// An app list holds the descendants of the apps it lists, so a mount is
		// reached both through the app it was registered on and as an entry of
		// the app above that one. Only the first walk passes through its real
		// parent; the other arrives with a prefix of that chain, and with the
		// constraints of the apps it skipped missing from its path — and which
		// one is seen first is up to map iteration order. The complete record
		// replaces the partial one whichever way round they arrive.
		if len(mount.ancestors) > len(dst[i].ancestors) {
			dst[i] = *mount
		}

		return dst
	}

	return append(dst, *mount)
}

// MountPath returns the route pattern where the current app instance was mounted as a sub-application.
func (app *App) MountPath() string {
	return app.mountFields.mountPath
}

// hasMountedApps Checks if there are any mounted apps in the current application.
func (app *App) hasMountedApps() bool {
	return len(app.mountFields.appList) > 1 || len(app.mountFields.domainAppList) > 0
}

// mountStartupProcess Handles the startup process of mounted apps by appending sub-app routes, generating app list keys, and processing sub-app routes.
func (app *App) mountStartupProcess() {
	if app.hasMountedApps() {
		// add routes of sub-apps
		app.mountFields.subAppsProcessed.Do(func() {
			app.appendSubAppLists(app.mountFields.appList)
			app.mountFields.appListKeysOnce.Do(app.generateAppListKeys)
		})
		// adds the routes of the sub-apps to the current application.
		app.mountFields.subAppsRoutesAdded.Do(func() {
			app.processSubAppsRoutes()
		})
	}
}

// generateAppListKeys generates app list keys for Render, should work after appendSubAppLists
func (app *App) generateAppListKeys() {
	for key := range app.mountFields.appList {
		app.mountFields.appListKeys = append(app.mountFields.appListKeys, key)
	}

	sort.Slice(app.mountFields.appListKeys, func(i, j int) bool {
		return len(app.mountFields.appListKeys[i]) < len(app.mountFields.appListKeys[j])
	})
}

// appendSubAppLists supports nested for sub apps
func (app *App) appendSubAppLists(appList map[string]*App, parent ...string) {
	// Optimize: Cache parent prefix
	parentPrefix := ""
	if len(parent) > 0 {
		parentPrefix = parent[0]
	}

	for prefix, subApp := range appList {
		// skip real app
		if prefix == "" {
			continue
		}

		if parentPrefix != "" {
			prefix = getGroupPath(parentPrefix, prefix)
		}

		if _, ok := app.mountFields.appList[prefix]; !ok {
			app.mountFields.appList[prefix] = subApp
		}

		// Domain mounts are kept out of appList, so collect them here instead.
		// Doing it at startup rather than when the sub-app was mounted also
		// picks up the ones registered in between.
		app.mountFields.domainAppList = app.appendDomainMounts(app.mountFields.domainAppList, subApp.mountFields.domainAppList, prefix, subApp)

		// The first element of appList is always the app itself. If there are no other sub apps, we should skip appending nested apps.
		if subApp.hasMountedApps() {
			app.appendSubAppLists(subApp.mountFields.appList, prefix)
		}
	}
}

// processSubAppsRoutes adds routes of sub-apps recursively when the server is started
func (app *App) processSubAppsRoutes() {
	for prefix, subApp := range app.mountFields.appList {
		// skip real app
		if prefix == "" {
			continue
		}
		// process the inner routes
		if subApp.hasMountedApps() {
			subApp.mountFields.subAppsRoutesAdded.Do(func() {
				subApp.processSubAppsRoutes()
			})
		}
	}
	var handlersCount uint32
	// Iterate over the stack of the parent app
	for m := range app.stack {
		// Iterate over each route in the stack
		stackLen := len(app.stack[m])
		for i := 0; i < stackLen; i++ {
			route := app.stack[m][i]
			// Check if the route has a mounted app
			if !route.mount {
				if !route.use || (route.use && m == 0) {
					handlersCount += uint32(len(route.Handlers)) //nolint:gosec // G115 - handler count is always small
				}
				continue
			}

			subApp := route.group.app

			// Snapshot under the sub-app's lock, then leave it: only clones
			// cross the boundary, so the re-parsing below never holds two
			// apps' locks. Read by method, not stack index: an app may
			// configure its own RequestMethods, so the tables need not line up.
			type sourceRoute struct {
				clone        *Route
				owner        *App
				constraints  []CustomConstraint
				skipAutoHead bool
			}

			subApp.mutex.Lock()
			subAppRoutes := subApp.routesForMethod(app.method(m))
			subConstraints := slices.Clone(subApp.customConstraints)
			// A sub-app that registers no automatic HEAD routes of its own
			// must not be given them here either: its middleware is
			// registered for the methods it serves, and a HEAD route
			// synthesized from the GET one would run without it.
			skipsAutoHead := subApp.config.DisableHeadAutoRegister || subApp.methodInt(MethodHead) < 0
			sources := make([]sourceRoute, len(subAppRoutes))
			for j, subAppRoute := range subAppRoutes {
				clone := app.copyRoute(subAppRoute)

				// The clone's registration ID comes from another counter and
				// could collide, so clear it rather than let helpers match.
				clone.regID = 0

				// Carry over the app each route came from: for a domain mount
				// the sub-app is a wrapper, and the apps behind it own the config.
				owner := subApp.routeOwner(subAppRoute)
				if owner == nil {
					owner = subApp
				}

				sources[j] = sourceRoute{
					clone:        clone,
					owner:        owner,
					constraints:  slices.Clone(subApp.routeConstraintsFor(subAppRoute)),
					skipAutoHead: skipsAutoHead || subApp.skipsAutoHeadFor(subAppRoute),
				}
			}
			subApp.mutex.Unlock()

			// The sub-app's routes are about to be re-parsed against this app,
			// so its constraints have to be resolvable here too.
			app.customConstraints = mergeCustomConstraints(app.customConstraints, subConstraints)

			// Create a slice to hold the sub-app's routes
			subRoutes := make([]*Route, len(sources))

			for j := range sources {
				source := &sources[j]

				// The prefix is this app's, and may name a constraint only this
				// app knows; the route brings the constraints of the apps that
				// composed its path, and those win where a name is defined
				// twice. Kept per route: two apps mounted side by side can name
				// one constraint differently, and neither binds the other.
				constraints := mergeCustomConstraints(source.constraints, app.customConstraints)

				// Add the parent route's path as a prefix to the sub-app's route
				app.addPrefixToRoute(route.path, source.clone, subApp.config.RegexHandler, constraints...)
				app.markRouteConstraints(source.clone, constraints)

				// Carry the sub-app's stance on automatic HEAD routes over to
				// the clone, so this app's own routes at the same path keep
				// theirs and the sub-app's descendants keep skipping.
				if source.skipAutoHead {
					app.markSkipAutoHead(source.clone)
				}
				app.markRouteOwner(source.clone, source.owner)

				// Add the cloned sub-app's route to the slice of sub-app routes
				subRoutes[j] = source.clone
			}

			// Insert the sub-app's routes into the parent app's stack
			newStack := make([]*Route, len(app.stack[m])+len(subRoutes)-1)
			copy(newStack[:i], app.stack[m][:i])
			copy(newStack[i:i+len(subRoutes)], subRoutes)
			copy(newStack[i+len(subRoutes):], app.stack[m][i+1:])
			app.stack[m] = newStack

			i--

			// Mark the parent app's routes as refreshed
			app.hasRoutesRefreshed = true
			app.bumpRoutesRevision()
			// update stackLen after appending subRoutes to app.stack[m]
			stackLen = len(app.stack[m])
		}
	}
	atomic.StoreUint32(&app.handlersCount, handlersCount)
}
