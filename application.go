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
	Version = "1.4.0"
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

// Connect establishes a tunnel to the server
// identified by the target resource.
func (r *Fiber) Connect(args ...interface{}) {
	r.register("CONNECT", args...)
}

// Put replaces all current representations
// of the target resource with the request payload.
func (r *Fiber) Put(args ...interface{}) {
	r.register("PUT", args...)
}

// Post is used to submit an entity to the specified resource,
// often causing a change in state or side effects on the server.
func (r *Fiber) Post(args ...interface{}) {
	r.register("POST", args...)
}

// Delete deletes the specified resource.
func (r *Fiber) Delete(args ...interface{}) {
	r.register("DELETE", args...)
}

// Head asks for a response identical to that of a GET request,
// but without the response body.
func (r *Fiber) Head(args ...interface{}) {
	r.register("HEAD", args...)
}

// Patch is used to apply partial modifications to a resource.
func (r *Fiber) Patch(args ...interface{}) {
	r.register("PATCH", args...)
}

// Options is used to describe the communication options
// for the target resource.
func (r *Fiber) Options(args ...interface{}) {
	r.register("OPTIONS", args...)
}

// Trace performs a message loop-back test
// along the path to the target resource.
func (r *Fiber) Trace(args ...interface{}) {
	r.register("TRACE", args...)
}

// Get requests a representation of the specified resource.
// Requests using GET should only retrieve data.
func (r *Fiber) Get(args ...interface{}) {
	r.register("GET", args...)
}

// All matches any HTTP method
func (r *Fiber) All(args ...interface{}) {
	r.register("ALL", args...)
}

// Use only matches the starting path
func (r *Fiber) Use(args ...interface{}) {
	r.register("MIDWARE", args...)
}

// Static https://fiber.wiki/application#static
func (r *Fiber) Static(args ...string) {
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
			r.routes = append(r.routes, &Route{"GET", prefix, wildcard, false, nil, nil, func(c *Ctx) {
				c.SendFile(filePath, gzip)
			}})
		}

		// Add the route + SendFile(filepath) to routes
		r.routes = append(r.routes, &Route{"GET", path, wildcard, false, nil, nil, func(c *Ctx) {
			c.SendFile(filePath, gzip)
		}})
	}
}

// Listen : https://fiber.wiki/application#listen
func (r *Fiber) Listen(address interface{}, tls ...string) {
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
	r.httpServer = r.setupServer()

	out := colorable.NewColorableStdout()

	// Prefork enabled
	if r.Prefork && runtime.NumCPU() > 1 {
		if r.Banner && !r.child {
			//cores := fmt.Sprintf("%s\x1b[1;30m %v cores", host, runtime.NumCPU())
			fmt.Fprintf(out, "\x1b[1;32m"+banner, "\x1b[1;30m", "\x1b[1;32mv"+Version+"\x1b[1;30m", "\x1b[1;32m"+host+"\x1b[1;30m", "\x1b[1;32mfiber.wiki")
		}
		r.prefork(host, tls...)
	}

	// Prefork disabled
	if r.Banner {
		fmt.Fprintf(out, "\x1b[1;32m"+banner, "\x1b[1;30m", "\x1b[1;32mv"+Version+"\x1b[1;30m", "\x1b[1;32m"+host+"\x1b[1;30m", "\x1b[1;32mfiber.wiki")
	}

	ln, err := net.Listen("tcp4", host)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// enable TLS/HTTPS
	if len(tls) > 1 {
		if err := r.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen: ", err)
		}
	}

	if err := r.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen: ", err)
	}
}

// Shutdown server gracefully
func (r *Fiber) Shutdown() error {
	if r.httpServer == nil {
		return fmt.Errorf("Server is not running")
	}
	return r.httpServer.Shutdown()
}

// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (r *Fiber) prefork(host string, tls ...string) {
	// Master proc
	if !r.child {
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
		if err := r.httpServer.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen-prefork: ", err)
		}
	}

	if err := r.httpServer.Serve(ln); err != nil {
		log.Fatal("Listen-prefork: ", err)
	}
}

func (r *Fiber) setupServer() *fasthttp.Server {
	return &fasthttp.Server{
		Handler:                            r.handler,
		Name:                               r.Server,
		Concurrency:                        r.Engine.Concurrency,
		DisableKeepalive:                   r.Engine.DisableKeepAlive,
		ReadBufferSize:                     r.Engine.ReadBufferSize,
		WriteBufferSize:                    r.Engine.WriteBufferSize,
		ReadTimeout:                        r.Engine.ReadTimeout,
		WriteTimeout:                       r.Engine.WriteTimeout,
		IdleTimeout:                        r.Engine.IdleTimeout,
		MaxConnsPerIP:                      r.Engine.MaxConnsPerIP,
		MaxRequestsPerConn:                 r.Engine.MaxRequestsPerConn,
		TCPKeepalive:                       r.Engine.TCPKeepalive,
		TCPKeepalivePeriod:                 r.Engine.TCPKeepalivePeriod,
		MaxRequestBodySize:                 r.Engine.MaxRequestBodySize,
		ReduceMemoryUsage:                  r.Engine.ReduceMemoryUsage,
		GetOnly:                            r.Engine.GetOnly,
		DisableHeaderNamesNormalizing:      r.Engine.DisableHeaderNamesNormalizing,
		SleepWhenConcurrencyLimitsExceeded: r.Engine.SleepWhenConcurrencyLimitsExceeded,
		NoDefaultServerHeader:              r.Server == "",
		NoDefaultContentType:               r.Engine.NoDefaultContentType,
		KeepHijackedConns:                  r.Engine.KeepHijackedConns,
	}
}
