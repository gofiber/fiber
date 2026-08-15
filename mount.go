// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
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
	// path is the full mount path, as appList would key it
	path string
	// matchers are the patterns of the domain routers this app was mounted
	// behind, outermost last. A request has to satisfy all of them.
	matchers []domainMatcher
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

// prefixParts counts the path segments of the mount path, so the deepest mount
// covering a request can be picked the same way appList entries are.
func (m *domainMountedApp) prefixParts() int {
	return strings.Count(m.path, "/") + 1
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

// domainMountedViews returns the view engine owner among the app's domain
// mounts for this request: the deepest mount whose path is part of the URL,
// whose patterns match the host, and which configures views. It returns nil
// when none does, leaving the plain mounts to answer.
func (app *App) domainMountedViews(c Ctx) *App {
	var (
		viewsApp   *App
		matchParts int
	)

	normalizedPath := utils.AddTrailingSlashString(c.Path())

	for i := range app.mountFields.domainAppList {
		mount := &app.mountFields.domainAppList[i]
		if mount.path == "" || mount.app.config.Views == nil {
			continue
		}
		// Matched on the path, as ErrorHandler matches the same entries. The
		// plain-mount scan in Render searches the whole URL instead, which also
		// answers for a mount path appearing in a query string.
		if !strings.HasPrefix(normalizedPath, utils.AddTrailingSlashString(mount.path)) || !mount.matchesHost(c.Hostname()) {
			continue
		}

		if parts := mount.prefixParts(); parts > matchParts {
			viewsApp, matchParts = mount.app, parts
		}
	}

	return viewsApp
}

// mountSkipsAutoHead reports whether path belongs to a mounted app that does
// not register automatic HEAD routes of its own, either because it does not
// serve HEAD at all or because it turned them off.
//
// Once a mount is expanded, its routes sit in this app's stack and are
// indistinguishable from its own, so the automatic-HEAD pass would synthesize
// a HEAD route the mounted app never wanted — and synthesize it from the GET
// route alone, leaving the middleware the mounted app registered for its own
// methods out of the chain.
func (app *App) mountSkipsAutoHead(path string) bool {
	normalizedPath := utils.AddTrailingSlashString(path)

	skips := func(mounted *App, mountPath string) bool {
		if mountPath == "" || !strings.HasPrefix(normalizedPath, utils.AddTrailingSlashString(mountPath)) {
			return false
		}
		if !mounted.config.DisableHeadAutoRegister && mounted.methodInt(MethodHead) >= 0 {
			return false
		}

		// Covering the path is not enough to own the route: a mount at "/"
		// covers every path this app registered itself. Ask the mounted app
		// whether the route is one of its own.
		return mounted.hasEndpoint(MethodGet, strings.TrimPrefix(path, utils.TrimRight(mountPath, '/')))
	}

	for mountPath, mounted := range app.mountFields.appList {
		if skips(mounted, mountPath) {
			return true
		}
	}

	for i := range app.mountFields.domainAppList {
		mount := &app.mountFields.domainAppList[i]
		if skips(mount.app, mount.path) {
			return true
		}
	}

	return false
}

// appendDomainMounts re-registers the domain mounts of a mounted app on the app
// above it, moving their paths under prefix. Their domain patterns travel with
// them: the host still has to match for their config to apply.
func appendDomainMounts(dst, mounts []domainMountedApp, prefix string) []domainMountedApp {
	for _, mount := range mounts {
		dst = append(dst, domainMountedApp{
			app:      mount.app,
			path:     getGroupPath(prefix, mount.path),
			matchers: mount.matchers,
		})
	}

	return dst
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
		app.mountFields.domainAppList = appendDomainMounts(app.mountFields.domainAppList, subApp.mountFields.domainAppList, prefix)

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

			// Create a slice to hold the sub-app's routes
			subRoutes := make([]*Route, len(subAppRoutes))

			// Iterate over the sub-app's routes
			for j, subAppRoute := range subAppRoutes {
				// Clone the sub-app's route
				subAppRouteClone := app.copyRoute(subAppRoute)

				// Add the parent route's path as a prefix to the sub-app's route
				app.addPrefixToRoute(route.path, subAppRouteClone, route.group.app.config.RegexHandler, route.group.app.customConstraints...)

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
