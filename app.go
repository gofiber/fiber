// https://fiber.wiki

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
const Version = "1.0.0"

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

var prefork = flag.Bool("fiber-prefork", false, "use prefork")
var child = flag.Bool("fiber-child", false, "is child process")

// New ...
func New(settings ...*Settings) (app *App) {
	app = &App{
		child: *child,
	}
	if len(settings) > 0 {
		opt := settings[0]
		if !opt.Prefork {
			opt.Prefork = *prefork
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
		Prefork:            *prefork,
		Concurrency:        256 * 1024,
		ReadBufferSize:     4096,
		WriteBufferSize:    4096,
		MaxRequestBodySize: 4 * 1024 * 1024,
	}
	return
}

// Static ...
func (app *App) Static(args ...string) *App {
	app.registerStatic("/", args...)
	return app
}

// WebSocket ...
func (app *App) WebSocket(args ...interface{}) *App {
	app.register("GET", "", args...)
	return app
}

// Connect ...
func (app *App) Connect(args ...interface{}) *App {
	app.register("CONNECT", "", args...)
	return app
}

// Put ...
func (app *App) Put(args ...interface{}) *App {
	app.register("PUT", "", args...)
	return app
}

// Post ...
func (app *App) Post(args ...interface{}) *App {
	app.register("POST", "", args...)
	return app
}

// Delete ...
func (app *App) Delete(args ...interface{}) *App {
	app.register("DELETE", "", args...)
	return app
}

// Head ...
func (app *App) Head(args ...interface{}) *App {
	app.register("HEAD", "", args...)
	return app
}

// Patch ...
func (app *App) Patch(args ...interface{}) *App {
	app.register("PATCH", "", args...)
	return app
}

// Options ...
func (app *App) Options(args ...interface{}) *App {
	app.register("OPTIONS", "", args...)
	return app
}

// Trace ...
func (app *App) Trace(args ...interface{}) *App {
	app.register("TRACE", "", args...)
	return app
}

// Get ...
func (app *App) Get(args ...interface{}) *App {
	app.register("GET", "", args...)
	return app
}

// All ...
func (app *App) All(args ...interface{}) *App {
	app.register("ALL", "", args...)
	return app
}

// Use ...
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
	// Print banner
	// if app.Settings.Banner && !app.child {
	// 	fmt.Printf("Fiber-%s is listening on %s\n", Version, addr)
	// }
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

// Shutdown server gracefully
func (app *App) Shutdown() error {
	if app.server == nil {
		return fmt.Errorf("Server is not running")
	}
	return app.server.Shutdown()
}

// Test takes a http.Request and execute a fake connection to the application
// It returns a http.Response when the connection was successful
func (app *App) Test(req *http.Request) (*http.Response, error) {
	// Get raw http request
	reqRaw, err := httputil.DumpRequest(req, true)
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
	resp, err := http.ReadResponse(buffer, req)
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
		childs := make([]*exec.Cmd, runtime.NumCPU()/2)

		// #nosec G204
		for i := range childs {
			childs[i] = exec.Command(os.Args[0], "-fiber-prefork", "-fiber-child")
			childs[i].Stdout = os.Stdout
			childs[i].Stderr = os.Stderr
			childs[i].ExtraFiles = []*os.File{fl}
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
