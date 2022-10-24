// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import "strings"

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
	for mountedPrefixes, subApp := range fiber.appList {
		subApp.mountPath = prefix + mountedPrefixes
		app.appList[prefix+mountedPrefixes] = subApp
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
	for mountedPrefixes, subApp := range fiber.appList {
		subApp.mountPath = groupPath + mountedPrefixes
		grp.app.appList[groupPath+mountedPrefixes] = subApp
	}

	// Execute onMount hooks
	if err := fiber.hooks.executeOnMountHooks(grp.app); err != nil {
		panic(err)
	}

	return grp
}

// The MountPath property contains one or more path patterns on which a sub-app was mounted.
func (app *App) MountPath() string {
	return app.mountPath
}
