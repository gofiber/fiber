// +build !windows

package fiber

import (
	"net"
	"strings"

	tcplisten "github.com/valyala/tcplisten"
)

// reuseport provides TCP net.Listener with SO_REUSEPORT support.
//
// SO_REUSEPORT allows linear scaling server performance on multi-CPU servers.
// See https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/ for more details :)
//
// The package is based on https://github.com/kavu/go_reuseport .

// Listen returns TCP listener with SO_REUSEPORT option set.
//
// The returned listener tries enabling the following TCP options, which usually
// have positive impact on performance:
//
// - TCP_DEFER_ACCEPT. This option expects that the server reads from accepted
//   connections before writing to them.
//
// - TCP_FASTOPEN. See https://lwn.net/Articles/508865/ for details.
//
// Use https://github.com/valyala/tcplisten if you want customizing
// these options.
//
// Only tcp4 and tcp6 networks are supported.
//
// ErrNoReusePort error is returned if the system doesn't support SO_REUSEPORT.
func reuseport(network, addr string) (net.Listener, error) {
	cfg := &tcplisten.Config{
		ReusePort:   true,
		DeferAccept: true,
		FastOpen:    true,
	}
	ln, err := cfg.NewListener(network, addr)
	if err != nil && strings.Contains(err.Error(), "SO_REUSEPORT") {
		return nil, err
	}
	return ln, err
}
