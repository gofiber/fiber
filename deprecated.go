// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"log"
	"regexp"
	"strings"

	websocket "github.com/fasthttp/websocket"
	fasthttp "github.com/valyala/fasthttp"
)

// These variables are deprecated since v1.8.2!
var compressResponse = fasthttp.CompressHandlerLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressDefaultCompression)
var websocketUpgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(fctx *fasthttp.RequestCtx) bool {
		return true
	},
}

// This function is deprecated since v1.8.2!
// Please us github.com/gofiber/compression
func (ctx *Ctx) Compress(enable ...bool) {
	log.Println("Warning: c.Compress() is deprecated since v1.8.2, please use github.com/gofiber/compression instead.")
	ctx.compress = true
	if len(enable) > 0 {
		ctx.compress = enable[0]
	}
}

// This function is deprecated since v1.8.2!
// Please us github.com/gofiber/websocket
func (app *App) WebSocket(path string, handle func(*Ctx)) *App {
	log.Println("Warning: app.WebSocket() is deprecated since v1.8.2, please use github.com/gofiber/websocket instead.")
	app.registerWebSocket(fasthttp.MethodGet, path, handle)
	return app
}

// This function is deprecated since v1.8.2!
// Please us github.com/gofiber/websocket
func (grp *Group) WebSocket(path string, handle func(*Ctx)) *Group {
	log.Println("Warning: app.WebSocket() is deprecated since v1.8.2, please use github.com/gofiber/websocket instead.")
	grp.app.registerWebSocket(fasthttp.MethodGet, groupPaths(grp.prefix, path), handle)
	return grp
}

// This function is deprecated since v1.8.2!
// Please us github.com/gofiber/recover
func (app *App) Recover(handler func(*Ctx)) {
	log.Println("Warning: app.Recover() is deprecated since v1.8.2, please use github.com/gofiber/recover instead.")
	app.recover = handler
}

// This function is deprecated since v1.8.2!
// Please us github.com/gofiber/recover
func (grp *Group) Recover(handler func(*Ctx)) {
	log.Println("Warning: Recover() is deprecated since v1.8.2, please use github.com/gofiber/recover instead.")
	grp.app.recover = handler
}

func (app *App) registerWebSocket(method, path string, handle func(*Ctx)) {
	// Cannot have an empty path
	if path == "" {
		path = "/"
	}
	// Path always start with a '/' or '*'
	if path[0] != '/' && path[0] != '*' {
		path = "/" + path
	}
	// Store original path to strip case sensitive params
	original := path
	// Case sensitive routing, all to lowercase
	if !app.Settings.CaseSensitive {
		path = strings.ToLower(path)
	}
	// Strict routing, remove last `/`
	if !app.Settings.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}

	var isWebSocket = true

	var isStar = path == "*" || path == "/*"
	var isSlash = path == "/"
	var isRegex = false
	// Route properties
	var Params = getParams(original)
	var Regexp *regexp.Regexp
	// Params requires regex pattern
	if len(Params) > 0 {
		regex, err := getRegex(path)
		if err != nil {
			log.Fatal("Router: Invalid path pattern: " + path)
		}
		isRegex = true
		Regexp = regex
	}
	app.routes = append(app.routes, &Route{
		isWebSocket: isWebSocket,
		isStar:      isStar,
		isSlash:     isSlash,
		isRegex:     isRegex,

		Method:    method,
		Path:      path,
		Params:    Params,
		Regexp:    Regexp,
		HandleCtx: handle,
	})
}
