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
	calls     []stubRedirectCall
	callCount int
}

func (s *stubRedirectClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	_ = req
	s.callCount++
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

func (s *stubRedirectClient) CallCount() int { return s.callCount }

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

	require.Equal(t, int32(3), dialCount.Load())

	clientTLS := &tls.Config{ServerName: "standard", MinVersion: tls.VersionTLS12}
	client.TLSConfig = clientTLS

	cfg := transport.TLSConfig()
	require.Same(t, clientTLS, cfg)

	override := &tls.Config{ServerName: "override", MinVersion: tls.VersionTLS13}
	transport.SetTLSConfig(override)
	require.Equal(t, override, client.TLSConfig)
}

func TestHostClientTransportClientAccessor(t *testing.T) {
	t.Parallel()

	host := &fasthttp.HostClient{Addr: "example.com:80"}
	transport := newHostClientTransport(host)

	current, ok := transport.Client().(*fasthttp.HostClient)
	require.True(t, ok)
	require.Same(t, host, current)

	hostTLS := &tls.Config{ServerName: "host", MinVersion: tls.VersionTLS12}
	host.TLSConfig = hostTLS

	cfg := transport.TLSConfig()
	require.Same(t, hostTLS, cfg)

	override := &tls.Config{ServerName: "host-override", MinVersion: tls.VersionTLS13}
	transport.SetTLSConfig(override)
	require.Equal(t, override, host.TLSConfig)
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

	lb := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{
		stubBalancingClient{},
		hostWithoutOverrides,
		nestedDialLB,
		nestedTLSLB,
		multiLevelWrapper,
	}}

	transport := newLBClientTransport(lb)
	require.Same(t, lb, transport.Client())
	cfg := transport.TLSConfig()
	require.Same(t, nestedTLSHost.TLSConfig, cfg)

	overrideTLS := &tls.Config{ServerName: "override", MinVersion: tls.VersionTLS12}
	transport.SetTLSConfig(overrideTLS)
	require.Equal(t, overrideTLS, hostWithoutOverrides.TLSConfig)
	require.Equal(t, overrideTLS, nestedDialHost.TLSConfig)
	require.Equal(t, overrideTLS, nestedTLSHost.TLSConfig)
	require.Equal(t, overrideTLS, multiLevelHost.TLSConfig)
	cfg = transport.TLSConfig()
	require.Same(t, overrideTLS, cfg)
	cfg.ServerName = "mutated"
	require.Equal(t, "mutated", transport.TLSConfig().ServerName)

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
	req.Header.SetContentType("application/json")
	req.SetBodyString("payload")

	client := &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusMovedPermanently), location: ptrString("/redirect")}, {status: ptrInt(fasthttp.StatusOK)}}}
	require.NoError(t, doRedirectsWithClient(req, resp, -1, client))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Equal(t, "http://example.com/redirect", req.URI().String())
	require.Empty(t, req.Body())
	require.Empty(t, req.Header.ContentType())

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBodyString("payload")

	seeOtherClient := &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusSeeOther), location: ptrString("/see-other")}, {status: ptrInt(fasthttp.StatusOK)}}}
	require.NoError(t, doRedirectsWithClient(req, resp, -1, seeOtherClient))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Equal(t, "http://example.com/see-other", req.URI().String())
	require.Empty(t, req.Body())
	require.Empty(t, req.Header.ContentType())

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
	require.Equal(t, 1, singleCall.CallCount())
	require.Equal(t, fasthttp.StatusFound, resp.Header.StatusCode())

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

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusFound), location: ptrString("/bad\x00path")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrorInvalidURI)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: ptrInt(fasthttp.StatusMovedPermanently), location: ptrString("/loop")}, {status: ptrInt(fasthttp.StatusFound), location: ptrString("/final")}, {status: ptrInt(fasthttp.StatusOK)}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrTooManyRedirects)
}

func TestDoRedirectsWithClientDefaultLimit(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodPost)

	calls := make([]stubRedirectCall, 0, defaultRedirectLimit+1)
	for i := 0; i < defaultRedirectLimit+1; i++ {
		calls = append(calls, stubRedirectCall{status: ptrInt(fasthttp.StatusFound), location: ptrString("/loop")})
	}

	client := &stubRedirectClient{calls: calls}
	err := doRedirectsWithClient(req, resp, -1, client)
	require.ErrorIs(t, err, fasthttp.ErrTooManyRedirects)
	require.Equal(t, defaultRedirectLimit+1, client.CallCount())
}

func Test_StandardClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()

	t.Run("default value", func(t *testing.T) {
		t.Parallel()
		transport := newStandardClientTransport(&fasthttp.Client{})
		require.False(t, transport.StreamResponseBody())
	})

	t.Run("enable streaming", func(t *testing.T) {
		t.Parallel()
		client := &fasthttp.Client{}
		transport := newStandardClientTransport(client)
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, client.StreamResponseBody)
	})

	t.Run("disable streaming", func(t *testing.T) {
		t.Parallel()
		client := &fasthttp.Client{}
		transport := newStandardClientTransport(client)
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, client.StreamResponseBody)
	})
}

func Test_HostClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()

	t.Run("default value", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{}
		transport := newHostClientTransport(hostClient)
		require.False(t, transport.StreamResponseBody())
	})

	t.Run("enable streaming", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{}
		transport := newHostClientTransport(hostClient)
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, hostClient.StreamResponseBody)
	})

	t.Run("disable streaming", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{}
		transport := newHostClientTransport(hostClient)
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, hostClient.StreamResponseBody)
	})
}

func Test_LBClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()

	t.Run("empty clients", func(t *testing.T) {
		t.Parallel()
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{},
		}
		transport := newLBClientTransport(lbClient)
		require.False(t, transport.StreamResponseBody())
	})

	t.Run("single host client", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{Addr: "example.com:80"}
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{hostClient},
		}
		transport := newLBClientTransport(lbClient)

		// Test default
		require.False(t, transport.StreamResponseBody())

		// Enable streaming
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, hostClient.StreamResponseBody)

		// Disable streaming
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, hostClient.StreamResponseBody)
	})

	t.Run("multiple host clients", func(t *testing.T) {
		t.Parallel()
		hostClient1 := &fasthttp.HostClient{Addr: "example1.com:80"}
		hostClient2 := &fasthttp.HostClient{Addr: "example2.com:80"}
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{hostClient1, hostClient2},
		}
		transport := newLBClientTransport(lbClient)

		// Enable streaming on all clients
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, hostClient1.StreamResponseBody)
		require.True(t, hostClient2.StreamResponseBody)

		// Disable streaming on all clients
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, hostClient1.StreamResponseBody)
		require.False(t, hostClient2.StreamResponseBody)
	})

	t.Run("nested lb client", func(t *testing.T) {
		t.Parallel()
		hostClient1 := &fasthttp.HostClient{Addr: "example1.com:80"}
		hostClient2 := &fasthttp.HostClient{Addr: "example2.com:80"}
		nestedLB := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{hostClient1, hostClient2},
		}
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{&lbBalancingClient{client: nestedLB}},
		}
		transport := newLBClientTransport(lbClient)

		// Enable streaming on nested clients
		transport.SetStreamResponseBody(true)
		require.True(t, hostClient1.StreamResponseBody)
		require.True(t, hostClient2.StreamResponseBody)

		// Disable streaming on nested clients
		transport.SetStreamResponseBody(false)
		require.False(t, hostClient1.StreamResponseBody)
		require.False(t, hostClient2.StreamResponseBody)
	})

	t.Run("mixed clients with stub", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{Addr: "example.com:80"}
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{hostClient, stubBalancingClient{}},
		}
		transport := newLBClientTransport(lbClient)

		// Enable streaming
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, hostClient.StreamResponseBody)

		// Disable streaming
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, hostClient.StreamResponseBody)
	})
}

func Test_httpClientTransport_Interface(t *testing.T) {
	t.Parallel()

	transports := []struct {
		transport httpClientTransport
		name      string
	}{
		{
			name:      "standardClientTransport",
			transport: newStandardClientTransport(&fasthttp.Client{}),
		},
		{
			name:      "hostClientTransport",
			transport: newHostClientTransport(&fasthttp.HostClient{}),
		},
		{
			name: "lbClientTransport",
			transport: newLBClientTransport(&fasthttp.LBClient{
				Clients: []fasthttp.BalancingClient{
					&fasthttp.HostClient{Addr: "example.com:80"},
				},
			}),
		},
	}

	for _, tt := range transports {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transport := tt.transport
			require.NotNil(t, transport.Client())
			initialStream := transport.StreamResponseBody()
			transport.SetStreamResponseBody(!initialStream)
			require.Equal(t, !initialStream, transport.StreamResponseBody())
			transport.SetStreamResponseBody(initialStream)
			require.Equal(t, initialStream, transport.StreamResponseBody())
		})
	}
}
