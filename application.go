// 🔌 Fiber is an Expressjs inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"flag"
	"time"
	// "github.com/tidwall/gjson"
)

const (
	// Version for debugging
	Version = "1.1.0"
	// https://play.golang.org/p/r6GNeV1gbH
	banner = "" +
		" \x1b[1;32m _____ _ _\n" +
		" \x1b[1;32m|   __|_| |_ ___ ___\n" +
		" \x1b[1;32m|   __| | . | -_|  _|\n" +
		" \x1b[1;32m|__|  |_|___|___|_|\x1b[1;30m%s\x1b[1;32m%s\n" +
		" \x1b[1;30m%s\x1b[1;32m%v\x1b[0000m\n\n"
)

var (
	prefork = flag.Bool("prefork", false, "use prefork")
	child   = flag.Bool("child", false, "is child process")
)

// Fiber structure
type Fiber struct {
	// Server name header
	Server string
	// Disable the fiber banner on launch
	Banner bool
	// Fasthttp server settings
	Engine *engine
	// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
	Prefork bool
	// Stores all routes
	routes []*route
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

// New creates a Fiber instance
func New() *Fiber {
	// Parse flags
	flag.Parse()
	return &Fiber{
		// No server header is sent when set empty ""
		Server: "",
		// Fiber banner is printed by default
		// Disable if it's a child process (when preforking)
		Banner: true,
		// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
		// Prefork can be set within code, or with flag -prefork
		Prefork: *prefork,
		// Default fasthttp settings
		// https://github.com/valyala/fasthttp/blob/master/server.go#L150
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
