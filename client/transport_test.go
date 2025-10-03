package client

import (
	"crypto/tls"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type stubBalancingClient struct{}

func (stubBalancingClient) DoDeadline(*fasthttp.Request, *fasthttp.Response, time.Time) error {
	return nil
}
func (stubBalancingClient) PendingRequests() int { return 0 }

type lbBalancingClient struct {
	client *fasthttp.LBClient
}

func (l *lbBalancingClient) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	if l.client == nil {
		return nil
	}
	return l.client.DoDeadline(req, resp, deadline)
}

func (*lbBalancingClient) PendingRequests() int { return 0 }

func (l *lbBalancingClient) LBClient() *fasthttp.LBClient { return l.client }

type stubRedirectCall struct {
	err      error
	status   *int
	location *string
}

func ptrInt(v int) *int { return &v }

func ptrString(v string) *string { return &v }

type stubRedirectClient struct {
	calls []stubRedirectCall
}

func (s *stubRedirectClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	_ = req
	if len(s.calls) == 0 {
		resp.Reset()
		resp.Header.SetStatusCode(fasthttp.StatusOK)
		return nil
	}

	call := s.calls[0]
	s.calls = s.calls[1:]

	resp.Reset()
	if call.status != nil {
		resp.Header.SetStatusCode(*call.status)
	}
	if call.location != nil {
		resp.Header.Set("Location", *call.location)
	}
	return call.err
}

func TestStandardClientTransportCoverage(t *testing.T) {
	t.Parallel()

	var dialCount atomic.Int32
	client := &fasthttp.Client{}
	client.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		dialCount.Add(1)
		return nil, errors.New("dial error")
	}

	transport := newStandardClientTransport(client)

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://example.com/")
	require.Error(t, transport.Do(req, resp))

	req.SetRequestURI("http://example.com/")
	require.Error(t, transport.DoTimeout(req, resp, time.Millisecond))

	req.SetRequestURI("http://example.com/")
	require.Error(t, transport.DoDeadline(req, resp, time.Now().Add(time.Second)))

	transport.CloseIdleConnections()

	underlying, ok := transport.Client().(*fasthttp.Client)
	require.True(t, ok)
	require.Same(t, client, underlying)

	transport.Reset()
	resetClient, ok := transport.Client().(*fasthttp.Client)
	require.True(t, ok)
	require.NotSame(t, client, resetClient)
	require.Equal(t, int32(3), dialCount.Load())
}

func TestHostClientTransportClientAccessor(t *testing.T) {
	t.Parallel()

	host := &fasthttp.HostClient{Addr: "example.com:80"}
	transport := newHostClientTransport(host)

	current, ok := transport.Client().(*fasthttp.HostClient)
	require.True(t, ok)
	require.Same(t, host, current)

	transport.Reset()
	afterReset, ok := transport.Client().(*fasthttp.HostClient)
	require.True(t, ok)
	require.NotSame(t, host, afterReset)
}

func TestLBClientTransportAccessorsAndOverrides(t *testing.T) {
	t.Parallel()

	hostWithoutOverrides := &fasthttp.HostClient{Addr: "example.com:80"}
	nestedDialHost := &fasthttp.HostClient{Addr: "example.org:80"}
	nestedTLSHost := &fasthttp.HostClient{Addr: "example.net:80", TLSConfig: &tls.Config{ServerName: "example", MinVersion: tls.VersionTLS12}}
	multiLevelHost := &fasthttp.HostClient{Addr: "example.edu:80"}

	nestedDialHost.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		return nil, errors.New("original dial")
	}

	multiLevelHost.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		return nil, errors.New("multi-level dial")
	}

	nestedDialLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nestedDialHost}}}
	nestedTLSLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nestedTLSHost}}}
	multiLevelLeaf := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{multiLevelHost}}}
	multiLevelWrapper := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{multiLevelLeaf}}}

	lb := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{stubBalancingClient{}, hostWithoutOverrides, nestedDialLB, nestedTLSLB, multiLevelWrapper}}

	transport := newLBClientTransport(lb)
	require.Same(t, lb, transport.Client())
	require.Equal(t, nestedTLSHost.TLSConfig, transport.tlsConfig)
	require.Equal(t, nestedTLSHost.TLSConfig, transport.TLSConfig())
	require.NotNil(t, transport.dial)

	overrideTLS := &tls.Config{ServerName: "override", MinVersion: tls.VersionTLS12}
	transport.SetTLSConfig(overrideTLS)
	require.Equal(t, overrideTLS, hostWithoutOverrides.TLSConfig)
	require.Equal(t, overrideTLS, nestedDialHost.TLSConfig)
	require.Equal(t, overrideTLS, nestedTLSHost.TLSConfig)
	require.Equal(t, overrideTLS, multiLevelHost.TLSConfig)
	require.Equal(t, overrideTLS, transport.TLSConfig())

	overrideDialCalled := atomic.Bool{}
	overrideDial := func(addr string) (net.Conn, error) {
		_ = addr
		overrideDialCalled.Store(true)
		return nil, errors.New("override dial")
	}
	transport.SetDial(overrideDial)
	overrideDialCalled.Store(false)
	_, err := hostWithoutOverrides.Dial("example.com:80")
	require.Error(t, err)
	require.True(t, overrideDialCalled.Load())

	overrideDialCalled.Store(false)
	_, err = nestedDialHost.Dial("example.org:80")
	require.Error(t, err)
	require.True(t, overrideDialCalled.Load())

	overrideDialCalled.Store(false)
	_, err = multiLevelHost.Dial("example.edu:80")
	require.Error(t, err)
	require.True(t, overrideDialCalled.Load())

	transport.Reset()
	require.Nil(t, transport.tlsConfig)
	require.Nil(t, transport.dial)
	resetClient, ok := transport.Client().(*fasthttp.LBClient)
	require.True(t, ok)
	require.NotSame(t, lb, resetClient)
}

func TestExtractTLSConfigVariations(t *testing.T) {
	t.Parallel()

	require.Nil(t, extractTLSConfig(nil))
	require.Nil(t, extractTLSConfig([]fasthttp.BalancingClient{stubBalancingClient{}}))

	host := &fasthttp.HostClient{TLSConfig: &tls.Config{ServerName: "configured", MinVersion: tls.VersionTLS12}}
	require.Equal(t, host.TLSConfig, extractTLSConfig([]fasthttp.BalancingClient{host}))

	nested := &fasthttp.HostClient{TLSConfig: &tls.Config{ServerName: "nested", MinVersion: tls.VersionTLS12}}
	nestedLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nested}}}
	require.Equal(t, nested.TLSConfig, extractTLSConfig([]fasthttp.BalancingClient{nestedLB}))

	multiLayerLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nestedLB}}}
	require.Equal(t, nested.TLSConfig, extractTLSConfig([]fasthttp.BalancingClient{multiLayerLB}))
}

func TestExtractDialVariations(t *testing.T) {
	t.Parallel()

	require.Nil(t, extractDial(nil))
	require.Nil(t, extractDial([]fasthttp.BalancingClient{stubBalancingClient{}}))

	hostWithoutDial := &fasthttp.HostClient{}
	hostWithDial := &fasthttp.HostClient{}
	called := atomic.Bool{}
	hostWithDial.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		called.Store(true)
		return nil, errors.New("dial")
	}

	dialFn := extractDial([]fasthttp.BalancingClient{hostWithoutDial, hostWithDial})
	require.NotNil(t, dialFn)
	_, err := dialFn("example.com:80")
	require.Error(t, err)
	require.True(t, called.Load())

	nestedHost := &fasthttp.HostClient{}
	nestedCalled := atomic.Bool{}
	nestedHost.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		nestedCalled.Store(true)
		return nil, errors.New("nested dial")
	}
	nestedLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nestedHost}}}
	nestedDial := extractDial([]fasthttp.BalancingClient{nestedLB})
	require.NotNil(t, nestedDial)
	_, err = nestedDial("example.com:80")
	require.Error(t, err)
	require.True(t, nestedCalled.Load())

	multiNestedHost := &fasthttp.HostClient{}
	multiCalled := atomic.Bool{}
	multiNestedHost.Dial = func(addr string) (net.Conn, error) {
		_ = addr
		multiCalled.Store(true)
		return nil, errors.New("multi nested dial")
	}
	multiNestedLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{multiNestedHost}}}
	multiLayerWrapper := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{multiNestedLB}}}
	multiLayerDial := extractDial([]fasthttp.BalancingClient{multiLayerWrapper})
	require.NotNil(t, multiLayerDial)
	_, err = multiLayerDial("example.com:80")
	require.Error(t, err)
	require.True(t, multiCalled.Load())
}

func TestWalkBalancingClientWithBreak(t *testing.T) {
	t.Parallel()

	host := &fasthttp.HostClient{}
	require.True(t, walkBalancingClientWithBreak(host, func(*fasthttp.HostClient) bool { return true }))

	require.False(t, walkBalancingClientWithBreak(stubBalancingClient{}, func(*fasthttp.HostClient) bool {
		t.Fatal("unexpected call")
		return false
	}))

	nested := &fasthttp.HostClient{}
	nestedLB := &lbBalancingClient{client: &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{nested}}}
	require.True(t, walkBalancingClientWithBreak(nestedLB, func(*fasthttp.HostClient) bool { return true }))

	directNestedHost := &fasthttp.HostClient{}
	directNestedLB := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{directNestedHost}}
	require.True(t, walkBalancingClientWithBreak(directNestedLB, func(hc *fasthttp.HostClient) bool {
		require.Same(t, directNestedHost, hc)
		return true
	}))
}

func TestDoRedirectsWithClientBranches(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetBodyString("payload")

	client := &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusMovedPermanently), location: ptrString("/redirect")}, {status: ptrInt(fasthttp.StatusOK)}}}
	require.NoError(t, doRedirectsWithClient(req, resp, -1, client))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Equal(t, "http://example.com/redirect", req.URI().String())
	require.Empty(t, req.Body())

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/again")
	req.SetBodyString("payload")

	singleCall := &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusFound), location: ptrString("/ignored")}}}
	require.NoError(t, doRedirectsWithClient(req, resp, 0, singleCall))
	require.Equal(t, fasthttp.StatusFound, resp.StatusCode())
	require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
	require.Equal(t, "http://example.com/again", req.URI().String())
	require.Equal(t, "payload", string(req.Body()))

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusFound)}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrMissingLocation)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusMovedPermanently), location: ptrString("ftp://example.com")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrorInvalidURI)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusMovedPermanently), location: ptrString("/loop")}, {status: ptrInt(fasthttp.StatusFound), location: ptrString("/final")}, {status: ptrInt(fasthttp.StatusOK)}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrTooManyRedirects)
}
