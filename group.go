// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"net/http"
	"strings"
)

// Group ...
type Group struct {
	prefix string
	app    *App
}

// Group : https://fiber.wiki/application#group
func (app *App) Group(prefix string, handlers ...interface{}) *Group {
	if len(handlers) > 0 {
		app.register("USE", prefix, handlers...)
	}
	return &Group{
		prefix: prefix,
		app:    app,
	}
}

// Group : https://fiber.wiki/application#group
func (grp *Group) Group(prefix string, handlers ...interface{}) *Group {
	var oldPrefix = grp.prefix
	if len(prefix) > 0 && prefix[0] != '/' && prefix[0] != '*' {
		prefix = "/" + prefix
	}
	// When grouping, always remove single slash
	if len(oldPrefix) > 0 && prefix == "/" {
		prefix = ""
	}
	// Prepent group prefix if exist
	newPrefix := oldPrefix + prefix
	// Clean path by removing double "//" => "/"
	newPrefix = strings.Replace(newPrefix, "//", "/", -1)
	if len(handlers) > 0 {
		grp.app.register("USE", newPrefix, handlers...)
	}
	return &Group{
		prefix: newPrefix,
		app:    grp.app,
	}
}

// Static : https://fiber.wiki/application#static
func (grp *Group) Static(args ...string) *Group {
	grp.app.registerStatic(grp.prefix, args...)
	return grp
}

// WebSocket : https://fiber.wiki/application#websocket
func (grp *Group) WebSocket(args ...interface{}) *Group {
	grp.app.register(http.MethodGet, grp.prefix, args...)
	return grp
}

// Connect : https://fiber.wiki/application#http-methods
func (grp *Group) Connect(args ...interface{}) *Group {
	grp.app.register(http.MethodConnect, grp.prefix, args...)
	return grp
}

// Put : https://fiber.wiki/application#http-methods
func (grp *Group) Put(args ...interface{}) *Group {
	grp.app.register(http.MethodPut, grp.prefix, args...)
	return grp
}

// Post : https://fiber.wiki/application#http-methods
func (grp *Group) Post(args ...interface{}) *Group {
	grp.app.register(http.MethodPost, grp.prefix, args...)
	return grp
}

// Delete : https://fiber.wiki/application#http-methods
func (grp *Group) Delete(args ...interface{}) *Group {
	grp.app.register(http.MethodDelete, grp.prefix, args...)
	return grp
}

// Head : https://fiber.wiki/application#http-methods
func (grp *Group) Head(args ...interface{}) *Group {
	grp.app.register(http.MethodHead, grp.prefix, args...)
	return grp
}

// Patch : https://fiber.wiki/application#http-methods
func (grp *Group) Patch(args ...interface{}) *Group {
	grp.app.register(http.MethodPatch, grp.prefix, args...)
	return grp
}

// Options : https://fiber.wiki/application#http-methods
func (grp *Group) Options(args ...interface{}) *Group {
	grp.app.register(http.MethodOptions, grp.prefix, args...)
	return grp
}

// Trace : https://fiber.wiki/application#http-methods
func (grp *Group) Trace(args ...interface{}) *Group {
	grp.app.register(http.MethodTrace, grp.prefix, args...)
	return grp
}

// Get : https://fiber.wiki/application#http-methods
func (grp *Group) Get(args ...interface{}) *Group {
	grp.app.register(http.MethodGet, grp.prefix, args...)
	return grp
}

// All : https://fiber.wiki/application#http-methods
func (grp *Group) All(args ...interface{}) *Group {
	grp.app.register("ALL", grp.prefix, args...)
	return grp
}

// Use : https://fiber.wiki/application#http-methods
func (grp *Group) Use(args ...interface{}) *Group {
	grp.app.register("USE", grp.prefix, args...)
	return grp
}
