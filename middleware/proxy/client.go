package proxy

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// defaultMaxResponseBodySize bounds the body the proxy buffers from an
// upstream response, so one that streams without end does not take the
// memory; it is the bound the client package applies too.
const defaultMaxResponseBodySize = 100 << 20

// defaultRequestTimeout bounds a request the default client makes to an upstream,
// connect, write and read of the whole response included, so one that stalls does
// not hold the handler and its connection. It is the request's bound, which
// DoTimeout and DoDeadline set themselves, rather than the client's read and
// write timeouts: those cap the time a request may be given and, being set per
// attempt, had fasthttp re-send an idempotent request to a stalled upstream until
// its attempts ran out. A variable for the test that shortens it.
var defaultRequestTimeout = 30 * time.Second

// defaultClient buffers at most defaultMaxResponseBodySize of a response and gives
// each request defaultRequestTimeout, see doActionWithPolicy; set other bounds with
// WithClient, or on one request with DoTimeout or DoDeadline.
var defaultClient = &fasthttp.Client{
	NoDefaultUserAgentHeader: true,
	DisablePathNormalizing:   true,
	MaxConnsPerHost:          defaultMaxConnsPerHost,
	MaxResponseBodySize:      defaultMaxResponseBodySize,
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
