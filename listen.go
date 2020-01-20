// 🚀 Fiber, Express on Steriods
// 📌 Don't use in production until version 1.0.0
// 🖥 https://github.com/gofiber/fiber

// 🦸 Not all heroes wear capes, thank you to some amazing people
// 💖 @valyala, @dgrr, @erikdubbelboer, @savsgio, @julienschmidt

package fiber

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

// Listen : https://gofiber.github.io/fiber/#/application?id=listen
func (r *Fiber) Listen(address interface{}, tls ...string) {
	host := ""
	switch val := address.(type) {
	case int:
		// 8080 => ":8080"
		host = ":" + strconv.Itoa(val)
	case string:
		// Address needs to contain a semicolon
		if !strings.Contains(val, ":") {
			// "8080" => ":8080"
			val = ":" + val
		}
		host = val
	default:
		panic("Host must be an INT port or STRING address")
	}
	// Copy settings to fasthttp server
	server := &fasthttp.Server{
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
	// Print banner if enabled, ignore if child proccess
	if r.Banner && !*child {
		if r.Prefork {
			fmt.Printf(banner, Version, "-prefork", "Express on steriods", host)
		} else {
			fmt.Printf(banner, Version, "", "Express on steriods", host)
		}
	}
	// Create listener
	var listener net.Listener
	var err error
	// If prefork enabled & enough cores
	if r.Prefork && runtime.NumCPU() > 1 {
		listener, err = r.reuseport(host)
		if err != nil {
			panic(err)
		}
	} else {
		listener, err = net.Listen("tcp4", host)
		if err != nil {
			panic(err)
		}
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	// Check ssl files are provided
	if len(tls) > 1 {
		if err := server.ServeTLS(listener, tls[0], tls[1]); err != nil {
			panic(err)
		}
	}
	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}

// TODO: enable ipv6 support ~ tcp4 > tcp = tcp4+tcp6
// https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/
func (r *Fiber) reuseport(host string) (net.Listener, error) {
	var listener net.Listener
	if !*child {
		addr, err := net.ResolveTCPAddr("tcp4", host)
		if err != nil {
			return nil, err
		}
		tcplistener, err := net.ListenTCP("tcp4", addr)
		if err != nil {
			return nil, err
		}
		file, err := tcplistener.File()
		if err != nil {
			return nil, err
		}
		childs := make([]*exec.Cmd, runtime.NumCPU())
		for i := range childs {
			childs[i] = exec.Command(os.Args[0], append(os.Args[1:], "-child")...)
			childs[i].Stdout = os.Stdout
			childs[i].Stderr = os.Stderr
			childs[i].ExtraFiles = []*os.File{file}
			if err := childs[i].Start(); err != nil {
				return nil, err
			}
		}
		for _, child := range childs {
			if err := child.Wait(); err != nil {
				panic(err)
			}
		}
		os.Exit(0)
		panic("Problem with calling os.Exit(0)")
	} else {
		// fmt.Printf(" \x1b[1;30mChild \x1b[1;32m#%v\x1b[1;30m reuseport\x1b[1;32m%s\x1b[0000m\n", os.Getpid(), host)
		var err error
		listener, err = net.FileListener(os.NewFile(3, ""))
		if err != nil {
			return nil, err
		}
		runtime.GOMAXPROCS(1)
	}
	return listener, nil
}
