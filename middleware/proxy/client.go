package proxy

import (
	"errors"
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

// defaultClient sets no timeouts or size cap; bound them with WithClient or DoTimeout/DoDeadline.
var defaultClient = &fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
	MaxConnsPerHost:          defaultMaxConnsPerHost,
}

// sharedClient is the client used when none is passed per call.
var sharedClient atomic.Pointer[fasthttp.Client]

var (
	errNilProxyClientOverride = errors.New("proxy: nil client override passed to Do/Forward")
	errNilGlobalProxyClient   = errors.New("proxy: global client is nil, set a non-nil client with proxy.WithClient")
)

func init() {
	ensureClientGuarded(defaultClient)
	sharedClient.Store(defaultClient)
}

// WithClient sets the global proxy client.
// This function should be called before Do and Forward — doing so installs
// the dial-time SSRF guard (via the client's ConfigureClient hook,
// composing with any hook it already carries) before the client dials any
// host, so requests dispatched through it re-validate the resolved IP at
// connect time, matching the default client's behavior.
func WithClient(cli *fasthttp.Client) {
	if cli == nil {
		panic("proxy: WithClient requires a non-nil *fasthttp.Client")
	}

	ensureClientGuarded(cli)
	sharedClient.Store(cli)
}

func selectClient(globalClient *fasthttp.Client, clients ...*fasthttp.Client) (*fasthttp.Client, error) {
	if len(clients) != 0 {
		if clients[0] == nil {
			return nil, errNilProxyClientOverride
		}

		return clients[0], nil
	}

	if globalClient == nil {
		return nil, errNilGlobalProxyClient
	}

	return globalClient, nil
}
