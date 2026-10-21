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

// lookupsFor returns the cache the up-front check may use for requests through cli. Only the shared
// client may: registered before its first request, as WithClient asks, it has its dials checked again
// where they connect. A per-call client may have dialed before the guard reached it, and one with a
// Transport dials by itself, so those get a fresh lookup.
func lookupsFor(cli *fasthttp.Client) *lookupCache {
	if cli.Transport == nil && cli == sharedClient.Load() {
		return upfrontLookups
	}
	return nil
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
