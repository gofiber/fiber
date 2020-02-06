// 🔌 Fiber is an Express.js inspired web framework build on 🚀 Fasthttp.
// 📌 Please open an issue if you got suggestions or found a bug!
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

// Listen : https://gofiber.github.io/fiber/#/application?id=listen
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
	server := r.setupServer()

	// Prefork enabled
	if r.Prefork && runtime.NumCPU() > 1 {
		if r.Banner && !r.child {
			cores := fmt.Sprintf("%s\x1b[1;30m %v cores", host, runtime.NumCPU())
			fmt.Printf(banner, Version, " prefork", "Express on steroids", cores)
		}
		r.prefork(server, host, tls...)
	}

	// Prefork disabled
	if r.Banner {
		fmt.Printf(banner, Version, "", "Express on steroids", host)
	}

	ln, err := net.Listen("tcp4", host)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	// enable TLS/HTTPS
	if len(tls) > 1 {
		if err := server.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen: ", err)
		}
	}

	if err := server.Serve(ln); err != nil {
		log.Fatal("Listen: ", err)
	}
}

// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (r *Fiber) prefork(server *fasthttp.Server, host string, tls ...string) {
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
		if err := server.ServeTLS(ln, tls[0], tls[1]); err != nil {
			log.Fatal("Listen-prefork: ", err)
		}
	}

	if err := server.Serve(ln); err != nil {
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
