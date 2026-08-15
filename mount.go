// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
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
	// Ordered keys of apps (sorted by key length for Render)
	appListKeys []string
	// guards one-time generation of appListKeys
	appListKeysOnce sync.Once
	// check added routes of sub-apps
	subAppsRoutesAdded sync.Once
	// check mounted sub-apps
	subAppsProcessed sync.Once
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
}

// newDomainMount records a mount of mounted at path, normalized and parsed
// with the routing rules of the app it is being mounted on, so the lookups
// recognize it the same way the router recognizes its routes.
func (app *App) newDomainMount(mounted *App, path string, matchers []domainMatcher) domainMountedApp {
	normalized := app.normalizePath(path)

	mount := domainMountedApp{
		app:      mounted,
		path:     normalized,
		matchers: matchers,
	}

	if parser := parseRoute(normalized, app.config.RegexHandler, app.customConstraints...); len(parser.params) > 0 {
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
	app.register([]string{methodUse}, prefix, mountGroup)

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
	grp.app.register([]string{methodUse}, groupPath, mountGroup)

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(grp.app); err != nil {
		panic(err)
	}

	return grp
}

// domainMountedViews returns the view engine of the domain mount that owns this
// request: the deepest one covering the path whose patterns match the host, and
// among equally deep ones the first registered, matching the order their routes
// are matched in.
//
// It returns nil when the owner configures no views, leaving the plain mounts
// and the root to answer — rather than borrowing the engine of a mount that did
// not serve the request.
func (app *App) domainMountedViews(c Ctx) (*App, int) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	owner, depth := app.domainMountOwner(c)
	if owner == nil || owner.config.Views == nil {
		return nil, 0
	}

	return owner, depth
}

// domainMountOwner returns the domain mount that owns this request and the
// depth it matched at: the deepest one covering the path whose patterns match
// the host, and among equally deep ones the first registered, matching the
// order their routes are matched in.
func (app *App) domainMountOwner(c Ctx) (*App, int) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	var (
		owner *App
		best  int
	)

	path := app.normalizePath(c.Path())

	for i := range app.mountFields.domainAppList {
		mount := &app.mountFields.domainAppList[i]
		if !mount.covers(path) || !mount.matchesHost(c.Hostname()) {
			continue
		}

		if depth := mount.depth(); owner == nil || depth > best {
			owner, best = mount.app, depth
		}
	}

	return owner, best
}

// mountedApps returns this app and every app mounted below it, plain or on a
// domain, for the operations that touch each mounted app's config.
//
// It walks the mount metadata itself rather than reading the flattened lists,
// which are only complete once the app has been through its startup process.
func (app *App) mountedApps() []*App {
	apps := []*App{app}

	for i := 0; i < len(apps); i++ {
		current := apps[i]
		if current.mountFields == nil {
			continue
		}

		for _, mounted := range current.mountFields.appList {
			if !slices.Contains(apps, mounted) {
				apps = append(apps, mounted)
			}
		}

		for j := range current.mountFields.domainAppList {
			if mounted := current.mountFields.domainAppList[j].app; !slices.Contains(apps, mounted) {
				apps = append(apps, mounted)
			}
		}
	}

	return apps
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

// appendDomainMounts re-registers the domain mounts of a mounted app on the app
// above it, moving their paths under prefix. Their domain patterns travel with
// them: the host still has to match for their config to apply.
func (app *App) appendDomainMounts(dst, mounts []domainMountedApp, prefix string) []domainMountedApp {
	for i := range mounts {
		mount := &mounts[i]
		dst = addDomainMount(dst, app.newDomainMount(mount.app, getGroupPath(prefix, mount.path), mount.matchers))
	}

	return dst
}

// addDomainMount appends a domain mount unless the same one is already
// recorded. An app list holds the descendants of the apps it lists, so the
// startup walk reaches the same mount once per path that leads to it.
func addDomainMount(dst []domainMountedApp, mount domainMountedApp) []domainMountedApp {
	for i := range dst {
		if dst[i].equals(&mount) {
			return dst
		}
	}

	return append(dst, mount)
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
		app.mountFields.domainAppList = app.appendDomainMounts(app.mountFields.domainAppList, subApp.mountFields.domainAppList, prefix)

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

			// Read the sub-app's routes by method rather than by stack index:
			// an app is free to configure its own RequestMethods, and the two
			// tables then neither line up nor have to be the same length.
			subAppRoutes := route.group.app.routesForMethod(app.method(m))

			// The sub-app's routes are about to be re-parsed against this app,
			// so its constraints have to be resolvable here too.
			app.customConstraints = mergeCustomConstraints(app.customConstraints, route.group.app.customConstraints)

			// The prefix is this app's, and may name a constraint only this
			// app knows; the routes are the sub-app's, whose constraints win
			// where both define a name.
			constraints := mergeCustomConstraints(slices.Clone(route.group.app.customConstraints), app.customConstraints)

			// A sub-app that registers no automatic HEAD routes of its own
			// must not be given them here either: its middleware is
			// registered for the methods it serves, and a HEAD route
			// synthesized from the GET one would run without it.
			skipsAutoHead := route.group.app.config.DisableHeadAutoRegister || route.group.app.methodInt(MethodHead) < 0

			// Create a slice to hold the sub-app's routes
			subRoutes := make([]*Route, len(subAppRoutes))

			// Iterate over the sub-app's routes
			for j, subAppRoute := range subAppRoutes {
				// Clone the sub-app's route
				subAppRouteClone := app.copyRoute(subAppRoute)

				// Add the parent route's path as a prefix to the sub-app's route
				app.addPrefixToRoute(route.path, subAppRouteClone, route.group.app.config.RegexHandler, constraints...)

				// Carry the sub-app's stance on automatic HEAD routes over to
				// the clone, so this app's own routes at the same path keep
				// theirs and the sub-app's descendants keep skipping.
				if skipsAutoHead || route.group.app.skipsAutoHeadFor(subAppRoute) {
					app.markSkipAutoHead(subAppRouteClone)
				}

				// Add the cloned sub-app's route to the slice of sub-app routes
				subRoutes[j] = subAppRouteClone
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
			// update stackLen after appending subRoutes to app.stack[m]
			stackLen = len(app.stack[m])
		}
	}
	atomic.StoreUint32(&app.handlersCount, handlersCount)
}
