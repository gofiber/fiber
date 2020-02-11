// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

const (
	// Version : Fiber version
	Version = "1.4.3"
	banner  = "\x1b[1;32m" + ` ______   __     ______     ______     ______
/\  ___\ /\ \   /\  == \   /\  ___\   /\  == \
\ \  __\ \ \ \  \ \  __<   \ \  __\   \ \  __<
 \ \_\    \ \_\  \ \_____\  \ \_____\  \ \_\ \_\
  \/_/     \/_/   \/_____/   \/_____/   \/_/ /_/

` + "\x1b[0mFiber \x1b[1;32mv%s\x1b[0m %s on \x1b[1;32m%s\x1b[0m, visit \x1b[1;32m%s\x1b[0m\n\n"
)

var (
	prefork = flag.Bool("prefork", false, "use prefork")
	child   = flag.Bool("child", false, "is child process")
)

// Application structure
type Application struct {
	// Server name header
	Server     string
	httpServer *fasthttp.Server
	// Show fiber banner
	Banner bool
	// https://github.com/valyala/fasthttp/blob/master/server.go#L150
	Engine *engine
	// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
	Prefork bool
	child   bool
	// Stores all routes
	routes []*Route
}

// Fasthttp settings
// https://github.com/valyala/fasthttp/blob/master/server.go#L150
type engine struct {
	Concurrency                        int
	DisableKeepAlive                   bool
	ReadBufferSize                     int
	WriteBufferSize                    int
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
	IdleTimeout                        time.Duration
	MaxConnsPerIP                      int
	MaxRequestsPerConn                 int
	TCPKeepalive                       bool
	TCPKeepalivePeriod                 time.Duration
	MaxRequestBodySize                 int
	ReduceMemoryUsage                  bool
	GetOnly                            bool
	DisableHeaderNamesNormalizing      bool
	SleepWhenConcurrencyLimitsExceeded time.Duration
	NoDefaultContentType               bool
	KeepHijackedConns                  bool
}

// New https://fiber.wiki/application#new
func New() *Application {
	flag.Parse()
	return &Application{
		Server:     "",
		httpServer: nil,
		Banner:     true,
		Prefork:    *prefork,
		child:      *child,
		Engine: &engine{
			Concurrency:                        256 * 1024,
			DisableKeepAlive:                   false,
			ReadBufferSize:                     4096,
			WriteBufferSize:                    4096,
			WriteTimeout:                       0,
			ReadTimeout:                        0,
			IdleTimeout:                        0,
			MaxConnsPerIP:                      0,
			MaxRequestsPerConn:                 0,
			TCPKeepalive:                       false,
			TCPKeepalivePeriod:                 0,
			MaxRequestBodySize:                 4 * 1024 * 1024,
			ReduceMemoryUsage:                  false,
			GetOnly:                            false,
			DisableHeaderNamesNormalizing:      false,
			SleepWhenConcurrencyLimitsExceeded: 0,
			NoDefaultContentType:               false,
			KeepHijackedConns:                  false,
		},
	}
}

// Group :
type Group struct {
	path string
	app  *Application
}

// Group :
func (app *Application) Group(path string) *Group {
	return &Group{
		path: path,
		app:  app,
	}
}

// Connect establishes a tunnel to the server
// identified by the target resource.
func (app *Application) Connect(args ...interface{}) *Application {
	app.register("CONNECT", args...)
	return app
}

// Connect for group
func (grp *Group) Connect(args ...interface{}) *Group {
	grp.register("CONNECT", args...)
	return grp
}

// Put replaces all current representations
// of the target resource with the request payload.
func (app *Application) Put(args ...interface{}) *Application {
	app.register("PUT", args...)
	return app
}

// Put for group
func (grp *Group) Put(args ...interface{}) *Group {
	grp.register("PUT", args...)
	return grp
}

// Post is used to submit an entity to the specified resource,
// often causing a change in state or side effects on the server.
func (app *Application) Post(args ...interface{}) *Application {
	app.register("POST", args...)
	return app
}

// Post for group
func (grp *Group) Post(args ...interface{}) *Group {
	grp.register("POST", args...)
	return grp
}

// Delete deletes the specified resource.
func (app *Application) Delete(args ...interface{}) *Application {
	app.register("DELETE", args...)
	return app
}

// Delete for group
func (grp *Group) Delete(args ...interface{}) *Group {
	grp.register("DELETE", args...)
	return grp
}

// Head asks for a response identical to that of a GET request,
// but without the response body.
func (app *Application) Head(args ...interface{}) *Application {
	app.register("HEAD", args...)
	return app
}

// Head for group
func (grp *Group) Head(args ...interface{}) *Group {
	grp.register("HEAD", args...)
	return grp
}

// Patch is used to apply partial modifications to a resource.
func (app *Application) Patch(args ...interface{}) *Application {
	app.register("PATCH", args...)
	return app
}

// Patch for group
func (grp *Group) Patch(args ...interface{}) *Group {
	grp.register("PATCH", args...)
	return grp
}

// Options is used to describe the communication options
// for the target resource.
func (app *Application) Options(args ...interface{}) *Application {
	app.register("OPTIONS", args...)
	return app
}

// Options for group
func (grp *Group) Options(args ...interface{}) *Group {
	grp.register("OPTIONS", args...)
	return grp
}

// Trace performs a message loop-back test
// along the path to the target resource.
func (app *Application) Trace(args ...interface{}) *Application {
	app.register("TRACE", args...)
	return app
}

// Trace for group
func (grp *Group) Trace(args ...interface{}) *Group {
	grp.register("TRACE", args...)
	return grp
}

// Get requests a representation of the specified resource.
// Requests using GET should only retrieve data.
func (app *Application) Get(args ...interface{}) *Application {
	app.register("GET", args...)
	return app
}

// Get for group
func (grp *Group) Get(args ...interface{}) *Group {
	grp.register("GET", args...)
	return grp
}

// All matches any HTTP method
func (app *Application) All(args ...interface{}) *Application {
	app.register("ALL", args...)
	return app
}

// All for group
func (grp *Group) All(args ...interface{}) *Group {
	grp.register("ALL", args...)
	return grp
}

// Use only matches the starting path
func (app *Application) Use(args ...interface{}) *Application {
	app.register("USE", args...)
	return app
}

// Use for group
func (grp *Group) Use(args ...interface{}) *Group {
	grp.register("USE", args...)
	return grp
}

// Static https://fiber.wiki/application#static
func (app *Application) Static(args ...string) {
	prefix := "/"
	root := "./"
	wildcard := false
	// enable / disable gzipping somewhere?
	// todo v2.0.0
	gzip := true

	if len(args) == 1 {
		root = args[0]
	} else if len(args) == 2 {
		prefix = args[0]
		root = args[1]
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
	}

	// Check if wildcard for single files
	// app.Static("*", "./public/index.html")
	// app.Static("/*", "./public/index.html")
	if prefix == "*" || prefix == "/*" {
		wildcard = true
	}

	// Check if root exists
	if _, err := os.Lstat(root); err != nil {
		log.Fatal("Static: ", err)
	}

	// Lets get all files from root
	files, _, err := getFiles(root)
	if err != nil {
		log.Fatal("Static: ", err)
	}

	// ./static/compiled => static/compiled
	mount := filepath.Clean(root)

	// Loop over all files
	for _, file := range files {
		// Ignore the .gzipped files by fasthttp
		if strings.Contains(file, ".fasthttp.gz") {
			continue
		}

		// Time to create a fake path for the route match
		// static/index.html => /index.html
		path := filepath.Join(prefix, strings.Replace(file, mount, "", 1))

		// Store original file path to use in ctx handler
		filePath := file

		// If the file is an index.html, bind the prefix to index.html directly
		if filepath.Base(filePath) == "index.html" || filepath.Base(filePath) == "index.htm" {
			app.routes = append(app.routes, &Route{"GET", prefix, wildcard, false, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}

		// Add the route + SendFile(filepath) to routes
		app.routes = append(app.routes, &Route{"GET", path, wildcard, false, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}

// Listen : https://fiber.wiki/application#listen
func (app *Application) Listen(address interface{}, tls ...string) {
	host := ""
	switch val := address.(type) {
	case int:
		host = ":" + strconv.Itoa(val) // 8080 => ":8080"
	case string:
		if !strings.Contains(val, ":") {
			val = ":" + val // "8080" => ":8080"
		}
		host = val
	default:
		log.Fatal("Listen: Host must be an INT port or STRING address")
	}
	// Create fasthttp server
	app.httpServer = app.setupServer()

	// Prefork enabled
	if app.Prefork && runtime.NumCPU() > 1 {
		if app.Banner && !app.child {
			fmt.Printf(banner, Version, "preforking", host, "fiber.wiki")
		}
		app.prefork(host, tls...)
	}

	// Prefork disabled
	if app.Banner {
		fmt.Printf(banner, Version, "listening", host, "fiber.wiki")
	}

	ln, err := net.Listen("tcp4", host)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// enable TLS/HTTPS
	if len(tls) > 1 {
		if err := app.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen: ", err)
		}
	}

	if err := app.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen: ", err)
	}
}

// Shutdown server gracefully
func (app *Application) Shutdown() error {
	if app.httpServer == nil {
		return fmt.Errorf("Server is not running")
	}
	return app.httpServer.Shutdown()
}

// Test takes a http.Request and execute a fake connection to the application
// It returns a http.Response when the connection was successful
func (app *Application) Test(req *http.Request) (*http.Response, error) {
	// Get raw http request
	reqRaw, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	// Setup a fiber server struct
	app.httpServer = app.setupServer()
	// Create fake connection
	conn := &conn{}
	// Pass HTTP request to conn
	_, err = conn.r.Write(reqRaw)
	if err != nil {
		return nil, err
	}
	// Serve conn to server
	channel := make(chan error)
	go func() {
		channel <- app.httpServer.ServeConn(conn)
	}()
	// Wait for callback
	select {
	case err := <-channel:
		if err != nil {
			return nil, err
		}
		// Throw timeout error after 200ms
	case <-time.After(500 * time.Millisecond):
		return nil, fmt.Errorf("Timeout")
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
func (app *Application) prefork(host string, tls ...string) {
	// Master proc
	if !app.child {
		// Create babies
		childs := make([]*exec.Cmd, runtime.NumCPU())

		// #nosec G204
		for i := range childs {
			childs[i] = exec.Command(os.Args[0], "-prefork", "-child")
			childs[i].Stdout = os.Stdout
			childs[i].Stderr = os.Stderr
			if err := childs[i].Start(); err != nil {
				log.Fatal("Listen-prefork: ", err)
			}
		}

		for _, child := range childs {
			if err := child.Wait(); err != nil {
				log.Fatal("Listen-prefork: ", err)
			}

		}

		os.Exit(0)
	}

	// Child proc
	runtime.GOMAXPROCS(1)

	ln, err := reuseport.Listen("tcp4", host)
	if err != nil {
		log.Fatal("Listen-prefork: ", err)
	}

	// enable TLS/HTTPS
	if len(tls) > 1 {
		if err := app.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen-prefork: ", err)
		}
	}

	if err := app.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen-prefork: ", err)
	}
}

func (app *Application) setupServer() *fasthttp.Server {
	return &fasthttp.Server{
		Handler:                            app.handler,
		Name:                               app.Server,
		Concurrency:                        app.Engine.Concurrency,
		DisableKeepalive:                   app.Engine.DisableKeepAlive,
		ReadBufferSize:                     app.Engine.ReadBufferSize,
		WriteBufferSize:                    app.Engine.WriteBufferSize,
		ReadTimeout:                        app.Engine.ReadTimeout,
		WriteTimeout:                       app.Engine.WriteTimeout,
		IdleTimeout:                        app.Engine.IdleTimeout,
		MaxConnsPerIP:                      app.Engine.MaxConnsPerIP,
		MaxRequestsPerConn:                 app.Engine.MaxRequestsPerConn,
		TCPKeepalive:                       app.Engine.TCPKeepalive,
		TCPKeepalivePeriod:                 app.Engine.TCPKeepalivePeriod,
		MaxRequestBodySize:                 app.Engine.MaxRequestBodySize,
		ReduceMemoryUsage:                  app.Engine.ReduceMemoryUsage,
		GetOnly:                            app.Engine.GetOnly,
		DisableHeaderNamesNormalizing:      app.Engine.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: app.Engine.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              app.Server == "",
		NoDefaultContentType:               app.Engine.NoDefaultContentType,
		KeepHijackedConns:                  app.Engine.KeepHijackedConns,
	}
}
