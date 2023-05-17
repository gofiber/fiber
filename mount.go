// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Put fields related to mounting.
type mountFields struct {
	// Mounted and main apps
	appList map[string]*App
	// Ordered keys of apps (sorted by key length for Render)
	appListKeys []string
	// check added routes of sub-apps
	subAppsRoutesAdded sync.Once
	// check mounted sub-apps
	subAppsProcessed sync.Once
	// Prefix of app if it was mounted
	mountPath string
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
func (app *App) Mount(prefix string, subApp *App) Router {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range subApp.mountFields.appList {
		path := getGroupPath(prefix, mountedPrefixes)

		subApp.mountFields.mountPath = path
		app.mountFields.appList[path] = subApp
	}

	// register mounted group
	mountGroup := &Group{Prefix: prefix, app: subApp}
	app.register(methodUse, prefix, mountGroup)

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(app); err != nil {
		panic(err)
	}

	return app
}

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount.
func (grp *Group) Mount(prefix string, subApp *App) Router {
	groupPath := getGroupPath(grp.Prefix, prefix)
	groupPath = strings.TrimRight(groupPath, "/")
	if groupPath == "" {
		groupPath = "/"
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range subApp.mountFields.appList {
		path := getGroupPath(groupPath, mountedPrefixes)

		subApp.mountFields.mountPath = path
		grp.app.mountFields.appList[path] = subApp
	}

	// register mounted group
	mountGroup := &Group{Prefix: groupPath, app: subApp}
	grp.app.register(methodUse, groupPath, mountGroup)

	// Execute onMount hooks
	if err := subApp.hooks.executeOnMountHooks(grp.app); err != nil {
		panic(err)
	}

	return grp
}

// The MountPath property contains one or more path patterns on which a sub-app was mounted.
func (app *App) MountPath() string {
	return app.mountFields.mountPath
}

// hasMountedApps Checks if there are any mounted apps in the current application.
func (app *App) hasMountedApps() bool {
	return len(app.mountFields.appList) > 1
}

// mountStartupProcess Handles the startup process of mounted apps by appending sub-app routes, generating app list keys, and processing sub-app routes.
func (app *App) mountStartupProcess() {
	if app.hasMountedApps() {
		// add routes of sub-apps
		app.mountFields.subAppsProcessed.Do(func() {
			app.appendSubAppLists(app.mountFields.appList)
			app.generateAppListKeys()
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

		// The first element of appList is always the app itself. If there are no other sub apps, we should skip appending nested apps.
		if len(subApp.mountFields.appList) > 1 {
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
	var routePos uint32
	// Iterate over the stack of the parent app
	for m := range app.stack {
		// Iterate over each route in the stack
		stackLen := len(app.stack[m])
		for i := 0; i < stackLen; i++ {
			route := app.stack[m][i]
			// Check if the route has a mounted app
			if !route.mount {
				routePos++
				// If not, update the route's position and continue
				route.pos = routePos
				if !route.use || (route.use && m == 0) {
					handlersCount += uint32(len(route.Handlers))
				}
				continue
			}

			// Create a slice to hold the sub-app's routes
			subRoutes := make([]*Route, len(route.group.app.stack[m]))

			// Iterate over the sub-app's routes
			for j, subAppRoute := range route.group.app.stack[m] {
				// Clone the sub-app's route
				subAppRouteClone := app.copyRoute(subAppRoute)

				// Add the parent route's path as a prefix to the sub-app's route
				app.addPrefixToRoute(route.path, subAppRouteClone)

				// Add the cloned sub-app's route to the slice of sub-app routes
				subRoutes[j] = subAppRouteClone
			}

			// Insert the sub-app's routes into the parent app's stack
			newStack := make([]*Route, len(app.stack[m])+len(subRoutes)-1)
			copy(newStack[:i], app.stack[m][:i])
			copy(newStack[i:i+len(subRoutes)], subRoutes)
			copy(newStack[i+len(subRoutes):], app.stack[m][i+1:])
			app.stack[m] = newStack

			// Decrease the parent app's route count to account for the mounted app's original route
			atomic.AddUint32(&app.routesCount, ^uint32(0))
			i--
			// Increase the parent app's route count to account for the sub-app's routes
			atomic.AddUint32(&app.routesCount, uint32(len(subRoutes)))

			// Mark the parent app's routes as refreshed
			app.routesRefreshed = true
			// update stackLen after appending subRoutes to app.stack[m]
			stackLen = len(app.stack[m])
		}
	}
	atomic.StoreUint32(&app.handlersCount, handlersCount)
}
