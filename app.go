// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	fasthttp "github.com/valyala/fasthttp"
)

// Version of Fiber
const Version = "1.8.0"

type (
	// App denotes the Fiber application.
	App struct {
		server   *fasthttp.Server
		routes   []*Route
		child    bool
		recover  func(*Ctx)
		Settings *Settings
	}
	// Map defines a generic map of type `map[string]interface{}`.
	Map map[string]interface{}
	// Settings is a struct holding the server settings
	Settings struct {
		// fiber settings
		Prefork bool `default:"false"`
		// Enable strict routing. When enabled, the router treats "/foo" and "/foo/" as different. Otherwise, the router treats "/foo" and "/foo/" as the same.
		StrictRouting bool `default:"false"`
		// Enable case sensitivity. When enabled, "/Foo" and "/foo" are different routes. When disabled, "/Foo" and "/foo" are treated the same.
		CaseSensitive bool `default:"false"`
		// Enables the "Server: value" HTTP header.
		ServerHeader string `default:""`
		// Enables handler values to be immutable even if you return from handler
		Immutable bool `default:"false"`
		// fasthttp settings
		GETOnly              bool          `default:"false"`
		IdleTimeout          time.Duration `default:"0"`
		Concurrency          int           `default:"256 * 1024"`
		ReadTimeout          time.Duration `default:"0"`
		WriteTimeout         time.Duration `default:"0"`
		TCPKeepalive         bool          `default:"false"`
		MaxConnsPerIP        int           `default:"0"`
		ReadBufferSize       int           `default:"4096"`
		WriteBufferSize      int           `default:"4096"`
		ConcurrencySleep     time.Duration `default:"0"`
		DisableKeepAlive     bool          `default:"false"`
		ReduceMemoryUsage    bool          `default:"false"`
		MaxRequestsPerConn   int           `default:"0"`
		TCPKeepalivePeriod   time.Duration `default:"0"`
		MaxRequestBodySize   int           `default:"4 * 1024 * 1024"`
		NoHeaderNormalizing  bool          `default:"false"`
		NoDefaultContentType bool          `default:"false"`
		// template settings
		ViewCache     bool   `default:"false"`
		ViewFolder    string `default:""`
		ViewEngine    string `default:""`
		ViewExtension string `default:""`
	}
)

func init() {
	flag.Bool("prefork", false, "Use prefork")
	flag.Bool("child", false, "Is a child process")
}

// New : https://fiber.wiki/application#new
func New(settings ...*Settings) (app *App) {
	var prefork bool
	var child bool
	for _, arg := range os.Args[1:] {
		if arg == "-prefork" {
			prefork = true
		} else if arg == "-child" {
			child = true
		}
	}
	app = &App{
		child: child,
	}
	if len(settings) > 0 {
		opt := settings[0]
		if !opt.Prefork {
			opt.Prefork = prefork
		}
		if opt.Immutable {
			getString = func(b []byte) string {
				return string(b)
			}
		}
		if opt.Concurrency == 0 {
			opt.Concurrency = 256 * 1024
		}
		if opt.ReadBufferSize == 0 {
			opt.ReadBufferSize = 4096
		}
		if opt.WriteBufferSize == 0 {
			opt.WriteBufferSize = 4096
		}
		if opt.MaxRequestBodySize == 0 {
			opt.MaxRequestBodySize = 4 * 1024 * 1024
		}
		app.Settings = opt
		return
	}
	app.Settings = &Settings{
		Prefork:            prefork,
		Concurrency:        256 * 1024,
		ReadBufferSize:     4096,
		WriteBufferSize:    4096,
		MaxRequestBodySize: 4 * 1024 * 1024,
	}
	return
}

// Recover : https://fiber.wiki/application#recover
func (app *App) Recover(callback func(*Ctx)) {
	app.recover = callback
}

// Recover : https://fiber.wiki/application#recover
func (grp *Group) Recover(callback func(*Ctx)) {
	grp.app.recover = callback
}

// Static : https://fiber.wiki/application#static
func (app *App) Static(args ...string) *App {
	app.registerStatic("/", args...)
	return app
}

// WebSocket : https://fiber.wiki/application#websocket
func (app *App) WebSocket(args ...interface{}) *App {
	app.register(http.MethodGet, "", args...)
	return app
}

// Connect : https://fiber.wiki/application#http-methods
func (app *App) Connect(args ...interface{}) *App {
	app.register(http.MethodConnect, "", args...)
	return app
}

// Put : https://fiber.wiki/application#http-methods
func (app *App) Put(args ...interface{}) *App {
	app.register(http.MethodPut, "", args...)
	return app
}

// Post : https://fiber.wiki/application#http-methods
func (app *App) Post(args ...interface{}) *App {
	app.register(http.MethodPost, "", args...)
	return app
}

// Delete : https://fiber.wiki/application#http-methods
func (app *App) Delete(args ...interface{}) *App {
	app.register(http.MethodDelete, "", args...)
	return app
}

// Head : https://fiber.wiki/application#http-methods
func (app *App) Head(args ...interface{}) *App {
	app.register(http.MethodHead, "", args...)
	return app
}

// Patch : https://fiber.wiki/application#http-methods
func (app *App) Patch(args ...interface{}) *App {
	app.register(http.MethodPatch, "", args...)
	return app
}

// Options : https://fiber.wiki/application#http-methods
func (app *App) Options(args ...interface{}) *App {
	app.register(http.MethodOptions, "", args...)
	return app
}

// Trace : https://fiber.wiki/application#http-methods
func (app *App) Trace(args ...interface{}) *App {
	app.register(http.MethodTrace, "", args...)
	return app
}

// Get : https://fiber.wiki/application#http-methods
func (app *App) Get(args ...interface{}) *App {
	app.register(http.MethodGet, "", args...)
	return app
}

// All : https://fiber.wiki/application#http-methods
func (app *App) All(args ...interface{}) *App {
	app.register("ALL", "", args...)
	return app
}

// Use : https://fiber.wiki/application#http-methods
func (app *App) Use(args ...interface{}) *App {
	app.register("USE", "", args...)
	return app
}

// Listen : https://fiber.wiki/application#listen
func (app *App) Listen(address interface{}, tls ...string) error {
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

	// enable TLS/HTTPS
	if len(tls) > 1 {
		return app.server.ServeTLS(ln, tls[0], tls[1])
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
	case <-time.After(1000 * time.Millisecond):
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
		addr, err := net.ResolveTCPAddr("tcp4", address)
		if err != nil {
			return ln, err
		}
		tcplistener, err := net.ListenTCP("tcp4", addr)
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

func (app *App) newServer() *fasthttp.Server {
	return &fasthttp.Server{
		Handler: app.handler,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			ctx.Response.SetStatusCode(400)
			ctx.Response.SetBodyString("Bad Request")
		},
		Name:                               app.Settings.ServerHeader,
		Concurrency:                        app.Settings.Concurrency,
		SleepWhenConcurrencyLimitsExceeded: app.Settings.ConcurrencySleep,
		DisableKeepalive:                   app.Settings.DisableKeepAlive,
		ReadBufferSize:                     app.Settings.ReadBufferSize,
		WriteBufferSize:                    app.Settings.WriteBufferSize,
		ReadTimeout:                        app.Settings.ReadTimeout,
		WriteTimeout:                       app.Settings.WriteTimeout,
		IdleTimeout:                        app.Settings.IdleTimeout,
		MaxConnsPerIP:                      app.Settings.MaxConnsPerIP,
		MaxRequestsPerConn:                 app.Settings.MaxRequestsPerConn,
		TCPKeepalive:                       app.Settings.TCPKeepalive,
		TCPKeepalivePeriod:                 app.Settings.TCPKeepalivePeriod,
		MaxRequestBodySize:                 app.Settings.MaxRequestBodySize,
		ReduceMemoryUsage:                  app.Settings.ReduceMemoryUsage,
		GetOnly:                            app.Settings.GETOnly,
		DisableHeaderNamesNormalizing:      app.Settings.NoHeaderNormalizing,
		NoDefaultServerHeader:              app.Settings.ServerHeader == "",
		NoDefaultContentType:               app.Settings.NoDefaultContentType,
	}
}
