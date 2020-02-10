// 🚀 Fiber is an Express.js inspired web framework written in Go with 💖
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 Links: https://github.com/gofiber/fiber, https://fiber.wiki

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @erikdubbelboer, @savsgio, @julienschmidt, @koddr

package fiber

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

const (
	// Version : Fiber version
	Version = "1.4.2"
	website = "https://fiber.wiki"
	banner  = ` ______   __     ______     ______     ______
/\  ___\ /\ \   /\  == \   /\  ___\   /\  == \
\ \  __\ \ \ \  \ \  __<   \ \  __\   \ \  __<
 \ \_\    \ \_\  \ \_____\  \ \_____\  \ \_\ \_\
  \/_/     \/_/   \/_____/   \/_____/   \/_/ /_/

%sFiber %s listening on %s, visit %s
`
)

var (
	prefork = flag.Bool("prefork", false, "use prefork")
	child   = flag.Bool("child", false, "is child process")
)

// Fiber structure
type Fiber struct {
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
func New() *Fiber {
	flag.Parse()
	return &Fiber{
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
	path  string
	fiber *Fiber
}

// Group :
func (f *Fiber) Group(path string) *Group {
	return &Group{
		path:  path,
		fiber: f,
	}
}

// Connect establishes a tunnel to the server
// identified by the target resource.
func (f *Fiber) Connect(args ...interface{}) *Fiber {
	f.register("CONNECT", args...)
	return f
}

// Connect for group
func (g *Group) Connect(args ...interface{}) *Group {
	g.register("CONNECT", args...)
	return g
}

// Put replaces all current representations
// of the target resource with the request payload.
func (f *Fiber) Put(args ...interface{}) *Fiber {
	f.register("PUT", args...)
	return f
}

// Put for group
func (g *Group) Put(args ...interface{}) *Group {
	g.register("PUT", args...)
	return g
}

// Post is used to submit an entity to the specified resource,
// often causing a change in state or side effects on the server.
func (f *Fiber) Post(args ...interface{}) *Fiber {
	f.register("POST", args...)
	return f
}

// Post for group
func (g *Group) Post(args ...interface{}) *Group {
	g.register("POST", args...)
	return g
}

// Delete deletes the specified resource.
func (f *Fiber) Delete(args ...interface{}) *Fiber {
	f.register("DELETE", args...)
	return f
}

// Delete for group
func (g *Group) Delete(args ...interface{}) *Group {
	g.register("DELETE", args...)
	return g
}

// Head asks for a response identical to that of a GET request,
// but without the response body.
func (f *Fiber) Head(args ...interface{}) *Fiber {
	f.register("HEAD", args...)
	return f
}

// Head for group
func (g *Group) Head(args ...interface{}) *Group {
	g.register("HEAD", args...)
	return g
}

// Patch is used to apply partial modifications to a resource.
func (f *Fiber) Patch(args ...interface{}) *Fiber {
	f.register("PATCH", args...)
	return f
}

// Patch for group
func (g *Group) Patch(args ...interface{}) *Group {
	g.register("PATCH", args...)
	return g
}

// Options is used to describe the communication options
// for the target resource.
func (f *Fiber) Options(args ...interface{}) *Fiber {
	f.register("OPTIONS", args...)
	return f
}

// Options for group
func (g *Group) Options(args ...interface{}) *Group {
	g.register("OPTIONS", args...)
	return g
}

// Trace performs a message loop-back test
// along the path to the target resource.
func (f *Fiber) Trace(args ...interface{}) *Fiber {
	f.register("TRACE", args...)
	return f
}

// Trace for group
func (g *Group) Trace(args ...interface{}) *Group {
	g.register("TRACE", args...)
	return g
}

// Get requests a representation of the specified resource.
// Requests using GET should only retrieve data.
func (f *Fiber) Get(args ...interface{}) *Fiber {
	f.register("GET", args...)
	return f
}

// Get for group
func (g *Group) Get(args ...interface{}) *Group {
	g.register("GET", args...)
	return g
}

// All matches any HTTP method
func (f *Fiber) All(args ...interface{}) *Fiber {
	f.register("ALL", args...)
	return f
}

// All for group
func (g *Group) All(args ...interface{}) *Group {
	g.register("ALL", args...)
	return g
}

// Use only matches the starting path
func (f *Fiber) Use(args ...interface{}) *Fiber {
	f.register("USE", args...)
	return f
}

// Use for group
func (g *Group) Use(args ...interface{}) *Group {
	g.register("USE", args...)
	return g
}

// Static https://fiber.wiki/application#static
func (f *Fiber) Static(args ...string) {
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
			f.routes = append(f.routes, &Route{"GET", prefix, wildcard, false, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}

		// Add the route + SendFile(filepath) to routes
		f.routes = append(f.routes, &Route{"GET", path, wildcard, false, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}

// Listen : https://fiber.wiki/application#listen
func (f *Fiber) Listen(address interface{}, tls ...string) {
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
	f.httpServer = f.setupServer()

	out := colorable.NewColorableStdout()

	// Prefork enabled
	if f.Prefork && runtime.NumCPU() > 1 {
		if f.Banner && f.child {
			//cores := fmt.Sprintf("%s\x1b[1;30m %v cores", host, runtime.NumCPU())
			fmt.Fprintf(out, "\x1b[1;32m"+banner, "\x1b[1;30m", "\x1b[1;32mv"+Version+"\x1b[1;30m", "\x1b[1;32m"+host+"\x1b[1;30m", "\x1b[1;32mfiber.wiki")
		}
		f.prefork(host, tls...)
	}

	// Prefork disabled
	if f.Banner {
		fmt.Fprintf(out, "\x1b[1;32m"+banner, "\x1b[1;30m", "\x1b[1;32mv"+Version+"\x1b[1;30m", "\x1b[1;32m"+host+"\x1b[1;30m", "\x1b[1;32mfiber.wiki")
	}

	ln, err := net.Listen("tcp4", host)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// enable TLS/HTTPS
	if len(tls) > 1 {
		if err := f.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen: ", err)
		}
	}

	if err := f.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen: ", err)
	}
}

// Shutdown server gracefully
func (f *Fiber) Shutdown() error {
	if f.httpServer == nil {
		return fmt.Errorf("Server is not running")
	}
	return f.httpServer.Shutdown()
}

// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (f *Fiber) prefork(host string, tls ...string) {
	// Master proc
	if !f.child {
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
		if err := f.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen-prefork: ", err)
		}
	}

	if err := f.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen-prefork: ", err)
	}
}

func (f *Fiber) setupServer() *fasthttp.Server {
	return &fasthttp.Server{
		Handler:                            f.handler,
		Name:                               f.Server,
		Concurrency:                        f.Engine.Concurrency,
		DisableKeepalive:                   f.Engine.DisableKeepAlive,
		ReadBufferSize:                     f.Engine.ReadBufferSize,
		WriteBufferSize:                    f.Engine.WriteBufferSize,
		ReadTimeout:                        f.Engine.ReadTimeout,
		WriteTimeout:                       f.Engine.WriteTimeout,
		IdleTimeout:                        f.Engine.IdleTimeout,
		MaxConnsPerIP:                      f.Engine.MaxConnsPerIP,
		MaxRequestsPerConn:                 f.Engine.MaxRequestsPerConn,
		TCPKeepalive:                       f.Engine.TCPKeepalive,
		TCPKeepalivePeriod:                 f.Engine.TCPKeepalivePeriod,
		MaxRequestBodySize:                 f.Engine.MaxRequestBodySize,
		ReduceMemoryUsage:                  f.Engine.ReduceMemoryUsage,
		GetOnly:                            f.Engine.GetOnly,
		DisableHeaderNamesNormalizing:      f.Engine.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: f.Engine.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              f.Server == "",
		NoDefaultContentType:               f.Engine.NoDefaultContentType,
		KeepHijackedConns:                  f.Engine.KeepHijackedConns,
	}
}
