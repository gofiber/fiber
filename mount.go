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
func (app *App) Mount(prefix string, fiber *App) Router {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range fiber.mountFields.appList {
		path := getGroupPath(prefix, mountedPrefixes)

		subApp.mountFields.mountPath = path
		app.mountFields.appList[path] = subApp
	}

	// Execute onMount hooks
	if err := fiber.hooks.executeOnMountHooks(app); err != nil {
		panic(err)
	}

	return app
}

// Mount attaches another app instance as a sub-router along a routing path.
// It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount.
func (grp *Group) Mount(prefix string, fiber *App) Router {
	groupPath := getGroupPath(grp.Prefix, prefix)
	groupPath = strings.TrimRight(groupPath, "/")
	if groupPath == "" {
		groupPath = "/"
	}

	// Support for configs of mounted-apps and sub-mounted-apps
	for mountedPrefixes, subApp := range fiber.mountFields.appList {
		path := getGroupPath(groupPath, mountedPrefixes)

		subApp.mountFields.mountPath = path
		grp.app.mountFields.appList[path] = subApp
	}

	// Execute onMount hooks
	if err := fiber.hooks.executeOnMountHooks(grp.app); err != nil {
		panic(err)
	}

	return grp
}

// The MountPath property contains one or more path patterns on which a sub-app was mounted.
func (app *App) MountPath() string {
	return app.mountFields.mountPath
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
	for prefix, subApp := range appList {
		// skip real app
		if prefix == "" {
			continue
		}

		if len(parent) > 0 {
			prefix = getGroupPath(parent[0], prefix)
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

// addSubAppsRoutes adds routes of sub apps nestedly when to start the server
func (app *App) addSubAppsRoutes(appList map[string]*App, parent ...string) {
	for prefix, subApp := range appList {
		// skip real app
		if prefix == "" {
			continue
		}

		if len(parent) > 0 {
			prefix = getGroupPath(parent[0], prefix)
		}

		// add routes
		stack := subApp.stack
		for m := range stack {
			for r := range stack[m] {
				route := app.copyRoute(stack[m][r])
				app.addRoute(route.Method, app.addPrefixToRoute(prefix, route), true)
			}
		}

		atomic.AddUint32(&app.handlersCount, subApp.handlersCount)
	}
}
