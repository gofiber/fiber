// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	fasthttp "github.com/valyala/fasthttp"
)

// Version of Fiber
const Version = "1.8.2"

type (
	// App denotes the Fiber application.
	App struct {
		server   *fasthttp.Server // Fasthttp server settings
		routes   []*Route         // Route stack
		child    bool             // If current process is a child ( for prefork )
		recover  func(*Ctx)       // Deprecated, use middleware.Recover
		Settings *Settings        // Fiber settings
	}
	// Map defines a generic map of type `map[string]interface{}`.
	Map map[string]interface{}
	// Settings is a struct holding the server settings
	Settings struct {
		// This will spawn multiple Go processes listening on the same port
		Prefork bool `default:"false"`
		// Enable strict routing. When enabled, the router treats "/foo" and "/foo/" as different.
		StrictRouting bool `default:"false"`
		// Enable case sensitivity. When enabled, "/Foo" and "/foo" are different routes.
		CaseSensitive bool `default:"false"`
		// Enables the "Server: value" HTTP header.
		ServerHeader string `default:""`
		// Enables handler values to be immutable even if you return from handler
		Immutable bool `default:"false"`
		// Enables GZip / Deflate compression for all responses
		Compression bool `default:"false"`
		// Max body size that the server accepts
		BodyLimit int `default:"4 * 1024 * 1024"`
		// Folder containing template files
		TemplateFolder string `default:""`
		// Template engine: html, amber, handlebars , mustache or pug
		TemplateEngine string `default:""`
		// Extension for the template files
		TemplateExtension string `default:""`
	}
)

func init() {
	flag.Bool("prefork", false, "Use prefork")
	flag.Bool("child", false, "Is a child process")
}

// New : https://fiber.wiki/application#new
func New(settings ...*Settings) *App {
	var prefork, child bool
	// Loop trought args without using flag.Parse()
	for i := range os.Args[1:] {
		if os.Args[i] == "-prefork" {
			prefork = true
		} else if os.Args[i] == "-child" {
			child = true
		}
	}
	// Create default app
	app := &App{
		child: child,
		Settings: &Settings{
			Prefork:   prefork,
			BodyLimit: 4 * 1024 * 1024,
		},
	}
	// If settings exist, set some defaults
	if len(settings) > 0 {
		if !settings[0].Prefork { // Default to -prefork flag if false
			settings[0].Prefork = prefork
		}
		if settings[0].BodyLimit == 0 { // Default MaxRequestBodySize
			settings[0].BodyLimit = 4 * 1024 * 1024
		}
		if settings[0].Immutable { // Replace unsafe conversion funcs
			getString = func(b []byte) string { return string(b) }
			getBytes = func(s string) []byte { return []byte(s) }
		}
		app.Settings = settings[0] // Set custom settings
	}
	return app
}

// Group : https://fiber.wiki/application#group
func (app *App) Group(prefix string, handlers ...func(*Ctx)) *Group {
	if len(handlers) > 0 {
		app.registerMethod("USE", prefix, handlers...)
	}
	return &Group{
		prefix: prefix,
		app:    app,
	}
}

// Static : https://fiber.wiki/application#static
func (app *App) Static(prefix, root string) *App {
	app.registerStatic(prefix, root)
	return app
}

// Use : https://fiber.wiki/application#http-methods
func (app *App) Use(args ...interface{}) *App {
	var path = ""
	var handlers []func(*Ctx)
	for i := 0; i < len(args); i++ {
		switch arg := args[i].(type) {
		case string:
			path = arg
		case func(*Ctx):
			handlers = append(handlers, arg)
		default:
			log.Fatalf("Invalid handler: %v", reflect.TypeOf(arg))
		}
	}
	app.registerMethod("USE", path, handlers...)
	return app
}

// Connect : https://fiber.wiki/application#http-methods
func (app *App) Connect(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodConnect, path, handlers...)
	return app
}

// Put : https://fiber.wiki/application#http-methods
func (app *App) Put(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodPut, path, handlers...)
	return app
}

// Post : https://fiber.wiki/application#http-methods
func (app *App) Post(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodPost, path, handlers...)
	return app
}

// Delete : https://fiber.wiki/application#http-methods
func (app *App) Delete(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodDelete, path, handlers...)
	return app
}

// Head : https://fiber.wiki/application#http-methods
func (app *App) Head(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodHead, path, handlers...)
	return app
}

// Patch : https://fiber.wiki/application#http-methods
func (app *App) Patch(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodPatch, path, handlers...)
	return app
}

// Options : https://fiber.wiki/application#http-methods
func (app *App) Options(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodOptions, path, handlers...)
	return app
}

// Trace : https://fiber.wiki/application#http-methods
func (app *App) Trace(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodTrace, path, handlers...)
	return app
}

// Get : https://fiber.wiki/application#http-methods
func (app *App) Get(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod(http.MethodGet, path, handlers...)
	return app
}

// All : https://fiber.wiki/application#http-methods
func (app *App) All(path string, handlers ...func(*Ctx)) *App {
	app.registerMethod("ALL", path, handlers...)
	return app
}

// WebSocket : https://fiber.wiki/application#websocket
func (app *App) WebSocket(path string, handle func(*Conn)) *App {
	app.registerWebSocket(http.MethodGet, path, handle)
	return app
}

// Recover : https://fiber.wiki/application#recover
func (app *App) Recover(handler func(*Ctx)) {
	log.Println("Warning: Recover(handler) is deprecated since v1.8.2, please use middleware.Recover(handler, error) instead.")
	app.recover = handler
}

// Listen : https://fiber.wiki/application#listen
func (app *App) Listen(address interface{}, tlsconfig ...*tls.Config) error {
	addr, ok := address.(string)
	if !ok {
		port, ok := address.(int)
		if !ok {
			return fmt.Errorf("Listen: Host must be an INT port or STRING address")
		}
		addr = strconv.Itoa(port)
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	// Create fasthttp server
	app.server = app.newServer()
	// Print listening message
	if !app.child {
		fmt.Printf("Fiber v%s listening on %s\n", Version, addr)
	}
	var ln net.Listener
	var err error
	// Prefork enabled
	if app.Settings.Prefork && runtime.NumCPU() > 1 {
		if ln, err = app.prefork(addr); err != nil {
			return err
		}
	} else {
		if ln, err = net.Listen("tcp4", addr); err != nil {
			return err
		}
	}

	// TLS config
	if len(tlsconfig) > 0 {
		ln = tls.NewListener(ln, tlsconfig[0])
	}
	return app.server.Serve(ln)
}

// Shutdown : TODO: Docs
// Shutsdown the server gracefully
func (app *App) Shutdown() error {
	if app.server == nil {
		return fmt.Errorf("Server is not running")
	}
	return app.server.Shutdown()
}

// Test : https://fiber.wiki/application#test
func (app *App) Test(request *http.Request) (*http.Response, error) {
	// Get raw http request
	reqRaw, err := httputil.DumpRequest(request, true)
	if err != nil {
		return nil, err
	}
	// Setup a fiber server struct
	app.server = app.newServer()
	// Create fake connection
	conn := &testConn{}
	// Pass HTTP request to conn
	_, err = conn.r.Write(reqRaw)
	if err != nil {
		return nil, err
	}
	// Serve conn to server
	channel := make(chan error)
	go func() {
		channel <- app.server.ServeConn(conn)
	}()
	// Wait for callback
	select {
	case err := <-channel:
		if err != nil {
			return nil, err
		}
		// Throw timeout error after 200ms
	case <-time.After(200 * time.Millisecond):
		return nil, fmt.Errorf("timeout")
	}
	// Get raw HTTP response
	respRaw, err := ioutil.ReadAll(&conn.w)
	if err != nil {
		return nil, err
	}
	// Create buffer
	reader := strings.NewReader(getString(respRaw))
	buffer := bufio.NewReader(reader)
	// Convert raw HTTP response to http.Response
	resp, err := http.ReadResponse(buffer, request)
	if err != nil {
		return nil, err
	}
	// Return *http.Response
	return resp, nil
}

// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (app *App) prefork(address string) (ln net.Listener, err error) {
	// Master proc
	if !app.child {
		addr, err := net.ResolveTCPAddr("tcp", address)
		if err != nil {
			return ln, err
		}
		tcplistener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return ln, err
		}
		fl, err := tcplistener.File()
		if err != nil {
			return ln, err
		}
		files := []*os.File{fl}
		childs := make([]*exec.Cmd, runtime.NumCPU()/2)
		// #nosec G204
		for i := range childs {
			childs[i] = exec.Command(os.Args[0], append(os.Args[1:], "-prefork", "-child")...)
			childs[i].Stdout = os.Stdout
			childs[i].Stderr = os.Stderr
			childs[i].ExtraFiles = files
			if err := childs[i].Start(); err != nil {
				return ln, err
			}
		}

		for _, child := range childs {
			if err := child.Wait(); err != nil {
				return ln, err
			}
		}
		os.Exit(0)
	} else {
		runtime.GOMAXPROCS(1)
		ln, err = net.FileListener(os.NewFile(3, ""))
	}
	return ln, err
}

type disableLogger struct{}

func (dl *disableLogger) Printf(format string, args ...interface{}) {
	// fmt.Println(fmt.Sprintf(format, args...))
}

func (app *App) newServer() *fasthttp.Server {
	return &fasthttp.Server{
		Handler:               app.handler,
		Name:                  app.Settings.ServerHeader,
		MaxRequestBodySize:    app.Settings.BodyLimit,
		NoDefaultServerHeader: app.Settings.ServerHeader == "",

		Logger:       &disableLogger{},
		LogAllErrors: false,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			if err.Error() == "body size exceeds the given limit" {
				ctx.Response.SetStatusCode(413)
				ctx.Response.SetBodyString("Request Entity Too Large")
			} else {
				ctx.Response.SetStatusCode(400)
				ctx.Response.SetBodyString("Bad Request")
			}
		},
	}
}
