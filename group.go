// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"log"
	"net/http"
	"reflect"
	"strings"
)

// Group ...
type Group struct {
	prefix string
	app    *App
}

// Group : https://fiber.wiki/application#group
func (grp *Group) Group(prefix string, handlers ...func(*Ctx)) *Group {
	if len(prefix) > 0 && prefix[0] != '/' && prefix[0] != '*' {
		prefix = "/" + prefix
	}
	// When grouping, always remove single slash
	if len(grp.prefix) > 0 && prefix == "/" {
		prefix = ""
	}
	// Prepent group prefix if exist
	prefix = grp.prefix + prefix
	// Clean path by removing double "//" => "/"
	prefix = strings.Replace(prefix, "//", "/", -1)
	if len(handlers) > 0 {
		grp.app.registerMethod("USE", prefix, handlers...)
	}
	return &Group{
		prefix: prefix,
		app:    grp.app,
	}
}

// Static : https://fiber.wiki/application#static
func (grp *Group) Static(args ...string) *Group {
	grp.app.registerStatic(grp.prefix, args...)
	return grp
}

func groupPaths(prefix, path string) string {
	// `/v1`+`/` => `/v1`+``
	if path == "/" {
		path = prefix
	} else {
		path = prefix + path
	}
	// Remove duplicate slashes `//`
	path = strings.Replace(path, "//", "/", -1)
	return path
}

// Use : https://fiber.wiki/application#http-methods
func (grp *Group) Use(args ...interface{}) *Group {
	var path = ""
	var handlers []func(*Ctx)
	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			path = arg
		case func(*Ctx):
			handlers = append(handlers, arg)
		default:
			log.Fatalf("Invalid handlerrrr: %v", reflect.TypeOf(arg))
		}
	}
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod("USE", path, handlers...)
	return grp
}

// Connect : https://fiber.wiki/application#http-methods
func (grp *Group) Connect(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodConnect, path, handlers...)
	return grp
}

// Put : https://fiber.wiki/application#http-methods
func (grp *Group) Put(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodPut, path, handlers...)
	return grp
}

// Post : https://fiber.wiki/application#http-methods
func (grp *Group) Post(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodPost, path, handlers...)
	return grp
}

// Delete : https://fiber.wiki/application#http-methods
func (grp *Group) Delete(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodDelete, path, handlers...)
	return grp
}

// Head : https://fiber.wiki/application#http-methods
func (grp *Group) Head(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodHead, path, handlers...)
	return grp
}

// Patch : https://fiber.wiki/application#http-methods
func (grp *Group) Patch(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodPatch, path, handlers...)
	return grp
}

// Options : https://fiber.wiki/application#http-methods
func (grp *Group) Options(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodOptions, path, handlers...)
	return grp
}

// Trace : https://fiber.wiki/application#http-methods
func (grp *Group) Trace(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodTrace, path, handlers...)
	return grp
}

// Get : https://fiber.wiki/application#http-methods
func (grp *Group) Get(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod(http.MethodGet, path, handlers...)
	return grp
}

// All : https://fiber.wiki/application#http-methods
func (grp *Group) All(path string, handlers ...func(*Ctx)) *Group {
	path = groupPaths(grp.prefix, path)
	grp.app.registerMethod("ALL", path, handlers...)
	return grp
}

// WebSocket : https://fiber.wiki/application#websocket
func (grp *Group) WebSocket(path string, handler func(*Conn)) *Group {
	grp.app.registerWebSocket(http.MethodGet, grp.prefix, path, handler)
	return grp
}

// Recover : https://fiber.wiki/application#recover
func (grp *Group) Recover(handler func(*Ctx)) {
	grp.app.recover = handler
}
