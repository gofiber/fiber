package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
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

// TestDoRedirectsWithClient_DropsBodyForAnyMethod pins net/http's
// redirectBehavior: 301, 302 and 303 all turn a body-carrying method into GET
// and drop the body, for every method rather than just POST, and drop the body
// even where the method already was GET or HEAD.
//
// Fiber's client drives QUERY requests through this loop too (client/core.go),
// and both branches used to be gated on IsPost — so a QUERY kept its method and
// its body across the redirect and replayed that body to the new location,
// which may be a different host than the caller addressed.
func TestDoRedirectsWithClient_DropsBodyForAnyMethod(t *testing.T) {
	t.Parallel()

	for _, status := range []int{fasthttp.StatusMovedPermanently, fasthttp.StatusFound, fasthttp.StatusSeeOther} {
		for _, method := range []string{fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodPatch, fasthttp.MethodDelete, "QUERY"} {
			t.Run(strconv.Itoa(status)+"/"+method, func(t *testing.T) {
				t.Parallel()

				req := fasthttp.AcquireRequest()
				resp := fasthttp.AcquireResponse()
				defer fasthttp.ReleaseRequest(req)
				defer fasthttp.ReleaseResponse(resp)

				req.SetRequestURI("http://example.com/start")
				req.Header.SetMethod(method)
				req.Header.SetContentType("application/json")
				req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
				req.Header.Set(fasthttp.HeaderTrailer, "X-Checksum")
				req.SetBodyString(`{"q":"secret"}`)

				client := &stubRedirectClient{calls: []stubRedirectCall{
					{status: new(status), location: new("http://other.example/next")},
					{status: new(fasthttp.StatusOK)},
				}}
				require.NoError(t, doRedirectsWithClient(req, resp, -1, client))

				require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
				require.Empty(t, req.Body(), "the body must not reach the redirect target")
				require.Empty(t, req.Header.ContentType())
				require.Empty(t, req.Header.Peek(fasthttp.HeaderContentEncoding), "a coding without a body to decode")
				require.Empty(t, req.Header.Peek(fasthttp.HeaderTrailer), "a Trailer only frames a body that is gone")
				require.Empty(t, req.Header.Peek(fasthttp.HeaderTransferEncoding))
			})
		}
	}

	// 307 and 308 exist to preserve the method and body, so they are untouched.
	for _, status := range []int{fasthttp.StatusTemporaryRedirect, fasthttp.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(status)+"/preserved", func(t *testing.T) {
			t.Parallel()

			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)

			req.SetRequestURI("http://example.com/start")
			req.Header.SetMethod(fasthttp.MethodPost)
			req.Header.SetContentType("application/json")
			req.SetBodyString("body-must-survive")

			client := &stubRedirectClient{calls: []stubRedirectCall{
				{status: new(status), location: new("/next")},
				{status: new(fasthttp.StatusOK)},
			}}
			require.NoError(t, doRedirectsWithClient(req, resp, -1, client))

			require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
			require.Equal(t, "body-must-survive", string(req.Body()))
		})
	}

	// GET and HEAD keep their method: RFC 9110 Section 15.4.4 says GET *or* HEAD.
	// The body still goes, or a GET carrying one replays it to the target.
	for _, method := range []string{fasthttp.MethodGet, fasthttp.MethodHead} {
		t.Run(method+" keeps its method but not its body", func(t *testing.T) {
			t.Parallel()

			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)

			req.SetRequestURI("http://example.com/start")
			req.Header.SetMethod(method)
			req.Header.SetContentType("application/json")
			req.SetBodyString(`{"q":"secret"}`)

			client := &stubRedirectClient{calls: []stubRedirectCall{
				{status: new(fasthttp.StatusSeeOther), location: new("http://other.example/next")},
				{status: new(fasthttp.StatusOK)},
			}}
			require.NoError(t, doRedirectsWithClient(req, resp, -1, client))

			require.Equal(t, method, string(req.Header.Method()))
			require.Empty(t, req.Body(), "the body must not reach the redirect target")
			require.Empty(t, req.Header.ContentType())
		})
	}
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

	client := &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusMovedPermanently), location: new("/redirect")}, {status: new(fasthttp.StatusOK)}}}
	require.NoError(t, doRedirectsWithClient(req, resp, -1, client))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Equal(t, "http://example.com/redirect", req.URI().String())
	require.Empty(t, req.Body())
	require.Empty(t, req.Header.ContentType())

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBodyString("payload")

	seeOtherClient := &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusSeeOther), location: new("/see-other")}, {status: new(fasthttp.StatusOK)}}}
	require.NoError(t, doRedirectsWithClient(req, resp, -1, seeOtherClient))
	require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()))
	require.Equal(t, "http://example.com/see-other", req.URI().String())
	require.Empty(t, req.Body())
	require.Empty(t, req.Header.ContentType())

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/again")
	req.SetBodyString("payload")

	// A limit of zero issues the request and counts the first redirect against
	// it, as fasthttp's own DoRedirects does. Returning nil made a request that
	// finished indistinguishable from one that stopped at a hop, and the request
	// is left as it was sent, since the error comes before any rewrite.
	singleCall := &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusFound), location: new("/ignored")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 0, singleCall), fasthttp.ErrTooManyRedirects)
	require.Equal(t, fasthttp.StatusFound, resp.StatusCode())
	require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
	require.Equal(t, "http://example.com/again", req.URI().String())
	require.Equal(t, "payload", string(req.Body()))
	require.Equal(t, 1, singleCall.CallCount())
	require.Equal(t, fasthttp.StatusFound, resp.Header.StatusCode())

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusFound)}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrMissingLocation)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusMovedPermanently), location: new("ftp://example.com")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrorInvalidURI)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusFound), location: new("/bad\x00path")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrorInvalidURI)

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/start")

	client = &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusMovedPermanently), location: new("/loop")}, {status: new(fasthttp.StatusFound), location: new("/final")}, {status: new(fasthttp.StatusOK)}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, 1, client), fasthttp.ErrTooManyRedirects)
}

// TestComposeRedirectURL_RejectsHTTPSDowngrade verifies that redirects
// from HTTPS origins to plaintext HTTP targets are refused so cookies
// and credentials never leave the TLS boundary.
func TestComposeRedirectURL_RejectsHTTPSDowngrade(t *testing.T) {
	t.Parallel()
	_, _, err := composeRedirectURL("https://example.com/login", []byte("http://example.com/after"), false)
	require.ErrorIs(t, err, ErrRedirectDowngrade)

	// Same-origin HTTP→HTTP is fine.
	out, host, err := composeRedirectURL("http://example.com/a", []byte("http://example.com/b"), false)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/b", out)
	require.Equal(t, "example.com", host)

	// HTTPS→HTTPS is fine.
	out, host, err = composeRedirectURL("https://example.com/a", []byte("https://example.com/b"), false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/b", out)
	require.Equal(t, "example.com", host)

	// HTTP→HTTPS upgrade is fine.
	out, host, err = composeRedirectURL("http://example.com/a", []byte("https://example.com/b"), false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/b", out)
	require.Equal(t, "example.com", host)

	// A cross-host redirect reports the new host so the caller can drop
	// origin-scoped credentials.
	out, host, err = composeRedirectURL("http://example.com/a", []byte("http://other.example/b"), false)
	require.NoError(t, err)
	require.Equal(t, "http://other.example/b", out)
	require.Equal(t, "other.example", host)
}

// TestTrustedRedirectTarget_NoInitialHostname pins the degenerate case: with no
// initial hostname there is no origin for the target to be a subdomain of, and
// every host is a suffix match for the empty string — so the subdomain test has
// to be skipped entirely rather than letting a trailing dot pass for one.
func TestTrustedRedirectTarget_NoInitialHostname(t *testing.T) {
	t.Parallel()

	require.False(t, trustedRedirectTarget("evil.com.", ""))
	require.False(t, trustedRedirectTarget("evil.com", ""))
	require.False(t, trustedRedirectTarget(".", ""))
	// Both empty is not a host change, so credentials still apply.
	require.True(t, trustedRedirectTarget("", ""))
}

// TestHostnameWithoutPort covers the host normalization the credential checks
// are built on. Its output feeds the same-origin and suffix tests in
// trustedRedirectTarget, so two different spellings must never fold to the same
// name — hence unwrapping exactly one matched bracket pair rather than trimming
// every bracket off both ends.
func TestHostnameWithoutPort(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com"},
		{"[::1]", "::1"},
		{"[::1]:8080", "::1"},
		{"[2001:db8::1]:443", "2001:db8::1"},
		{"", ""},
		{":8080", ""},
		// An IPv6 literal written without brackets: the last colon belongs to
		// the address, not to a port. Truncating there would fold distinct
		// hosts onto one name.
		{"fe80::1", "fe80::1"},
		{"fe80::2", "fe80::2"},
		{"2001:db8::1", "2001:db8::1"},
		// Not a bracketed literal: one stray bracket is left where it is
		// rather than silently folded away.
		{"[[x]]", "[x]"},
		{"[x", "[x"},
		{"x]", "x]"},
	} {
		require.Equal(t, tc.want, hostnameWithoutPort(tc.in), "input %q", tc.in)
	}
}

// TestDoRedirectsWithClient_StripsCredentialsCrossHost verifies that
// origin-scoped credentials are dropped as soon as a redirect leaves the host
// they were issued for, and survive redirects that stay on it.
func TestDoRedirectsWithClient_StripsCredentialsCrossHost(t *testing.T) {
	t.Parallel()

	newReq := func() *fasthttp.Request {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI("http://example.com/start")
		req.Header.Set(fasthttp.HeaderAuthorization, "Bearer secret")
		req.Header.Set(fasthttp.HeaderProxyAuthorization, "Basic secret")
		req.Header.Set(fasthttp.HeaderCookie, "session=secret")
		return req
	}

	t.Run("subdomain of the initial host keeps credentials", func(t *testing.T) {
		t.Parallel()

		req := newReq()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		client := &stubRedirectClient{calls: []stubRedirectCall{
			{status: new(fasthttp.StatusFound), location: new("http://www.example.com/next")},
			{status: new(fasthttp.StatusOK)},
		}}
		require.NoError(t, doRedirectsWithClient(req, resp, 5, client))
		require.Equal(t, "Bearer secret", string(req.Header.Peek(fasthttp.HeaderAuthorization)))
	})

	t.Run("explicit default port keeps credentials", func(t *testing.T) {
		t.Parallel()

		req := newReq()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		client := &stubRedirectClient{calls: []stubRedirectCall{
			{status: new(fasthttp.StatusFound), location: new("http://example.com:80/next")},
			{status: new(fasthttp.StatusOK)},
		}}
		require.NoError(t, doRedirectsWithClient(req, resp, 5, client))
		require.Equal(t, "Bearer secret", string(req.Header.Peek(fasthttp.HeaderAuthorization)))
	})

	t.Run("same host keeps credentials", func(t *testing.T) {
		t.Parallel()

		req := newReq()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		client := &stubRedirectClient{calls: []stubRedirectCall{
			{status: new(fasthttp.StatusFound), location: new("http://example.com/next")},
			{status: new(fasthttp.StatusOK)},
		}}
		require.NoError(t, doRedirectsWithClient(req, resp, 5, client))
		require.Equal(t, "Bearer secret", string(req.Header.Peek(fasthttp.HeaderAuthorization)))
		require.Equal(t, "Basic secret", string(req.Header.Peek(fasthttp.HeaderProxyAuthorization)))
		require.Equal(t, "session=secret", string(req.Header.Peek(fasthttp.HeaderCookie)))
	})

	t.Run("cross host drops credentials", func(t *testing.T) {
		t.Parallel()

		req := newReq()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		client := &stubRedirectClient{calls: []stubRedirectCall{
			{status: new(fasthttp.StatusFound), location: new("http://attacker.example/collect")},
			{status: new(fasthttp.StatusOK)},
		}}
		require.NoError(t, doRedirectsWithClient(req, resp, 5, client))
		require.Equal(t, "http://attacker.example/collect", req.URI().String())
		require.Empty(t, req.Header.Peek(fasthttp.HeaderAuthorization))
		require.Empty(t, req.Header.Peek(fasthttp.HeaderProxyAuthorization))
		require.Empty(t, req.Header.Peek(fasthttp.HeaderCookie))
	})
}

// TestDoRedirectsWithClient_ValidatesTargets covers the target validation that
// every transport now shares through doRedirectsWithClient.
func TestDoRedirectsWithClient_ValidatesTargets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr  error
		name     string
		base     string
		location string
	}{
		{name: "https downgrade", base: "https://example.com/login", location: "http://example.com/after", wantErr: ErrRedirectDowngrade},
		{name: "foreign scheme", base: "http://example.com/a", location: "ftp://example.com/b", wantErr: fasthttp.ErrorInvalidURI},
		{name: "control character", base: "http://example.com/a", location: "/b\x00c", wantErr: fasthttp.ErrorInvalidURI},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)

			req.SetRequestURI(tc.base)
			client := &stubRedirectClient{calls: []stubRedirectCall{
				{status: new(fasthttp.StatusFound), location: new(tc.location)},
			}}
			require.ErrorIs(t, doRedirectsWithClient(req, resp, 5, client), tc.wantErr)
		})
	}
}

// TestClient_DoRedirects_RejectsHTTPSDowngrade drives the transport New()
// actually builds over a real TLS hop. It used to delegate to fasthttp's loop,
// which follows an HTTPS-to-HTTP redirect and, the host being unchanged, keeps
// the credentials on it.
//
// Only the plaintext hop is left undialable: reaching it produces a dial error
// rather than ErrRedirectDowngrade, so the assertion cannot pass vacuously.
func TestClient_DoRedirects_RejectsHTTPSDowngrade(t *testing.T) {
	t.Parallel()

	cert, err := tls.LoadX509KeyPair("../.github/testdata/ssl.pem", "../.github/testdata/ssl.key")
	require.NoError(t, err)

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New()
	app.Get("/start", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusFound).To("http://example.com/collect")
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, app.Listener(
			tls.NewListener(ln, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}),
			fiber.ListenConfig{DisableStartupMessage: true},
		))
	}()
	defer func() {
		require.NoError(t, app.Shutdown())
		<-done
	}()

	client := New().
		SetDial(func(addr string) (net.Conn, error) {
			if strings.HasSuffix(addr, ":443") {
				return ln.Dial()
			}
			return nil, errors.New("the plaintext hop must never be dialed")
		}).
		SetTLSConfig(&tls.Config{InsecureSkipVerify: true}) // self-signed test certificate

	_, err = client.Get("https://example.com/start", Config{
		Header:       map[string]string{fiber.HeaderAuthorization: "Bearer secret"},
		MaxRedirects: 5,
	})
	require.ErrorIs(t, err, ErrRedirectDowngrade)
}

// TestClient_DoRedirects_StripsCredentialsCrossHost exercises the default
// transport end to end: the redirect validation must apply to the client
// callers actually get from New(), not only to the load-balanced transport.
func TestClient_DoRedirects_StripsCredentialsCrossHost(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/start", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://attacker.example/collect")
		})
		app.Get("/same-host", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://example.com/collect")
		})
		app.Get("/collect", func(c fiber.Ctx) error {
			return c.SendString(c.Get(fiber.HeaderAuthorization) + "|" + c.Get(fiber.HeaderCookie))
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	credentials := Config{
		Header:       map[string]string{fiber.HeaderAuthorization: "Bearer secret"},
		Cookie:       map[string]string{"session": "secret"},
		MaxRedirects: 5,
	}

	// Control: a redirect that stays on the issuing host carries them through,
	// so the cross-host assertion below cannot pass vacuously.
	sameHost, err := client.Get("http://example.com/same-host", credentials)
	require.NoError(t, err)
	defer sameHost.Close()
	require.Equal(t, fiber.StatusOK, sameHost.StatusCode())
	require.Equal(t, "Bearer secret|session=secret", sameHost.String())

	resp, err := client.Get("http://example.com/start", credentials)
	require.NoError(t, err)
	defer resp.Close()

	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "|", resp.String())
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
	for range defaultRedirectLimit + 1 {
		calls = append(calls, stubRedirectCall{status: new(fasthttp.StatusFound), location: new("/loop")})
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

// Test_Transport_DoRedirects_KeepsPathNormalizingDisabled covers a transport
// wrapping a client that asked for the path to be left alone.
//
// fasthttp's own DoRedirects sets the flag on req.URI() before serializing it.
// This loop serializes first, so without the same step the very first request
// went out with "/a//b" collapsed — the redirect implementation changing the
// path the caller asked for.
func Test_Transport_DoRedirects_KeepsPathNormalizingDisabled(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		wantPath string
		disable  bool
	}{
		{name: "disabled keeps the path as written", wantPath: "/a//b", disable: true},
		{name: "enabled normalizes as before", wantPath: "/a/b", disable: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ln := fasthttputil.NewInmemoryListener()
			seen := make(chan string, 4)
			server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
				seen <- string(ctx.Request.URI().PathOriginal())
				ctx.SetStatusCode(fasthttp.StatusOK)
			}}
			done := make(chan struct{})
			go func() {
				defer close(done)
				assert.NoError(t, server.Serve(ln))
			}()
			defer func() {
				require.NoError(t, server.Shutdown())
				<-done
			}()

			transport := newStandardClientTransport(&fasthttp.Client{
				DisablePathNormalizing: tc.disable,
				Dial:                   func(string) (net.Conn, error) { return ln.Dial() },
			})

			req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)
			req.SetRequestURI("http://example.com/a//b")

			require.NoError(t, transport.DoRedirects(req, resp, 3))
			require.Equal(t, fasthttp.StatusOK, resp.StatusCode())
			require.Equal(t, tc.wantPath, <-seen)
		})
	}
}

// Test_DropRequestBody_RemovesExpect covers the 100-continue expectation, which
// may not be generated without content (RFC 9110 Section 10.1.1). Left on the
// bodyless follow-up, a strict target answers 417 instead of serving it.
func Test_DropRequestBody_RemovesExpect(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.Set(fasthttp.HeaderExpect, "100-continue")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")

	dropRequestBody(req)

	require.Empty(t, req.Header.Peek(fasthttp.HeaderExpect))
	require.Empty(t, req.Body())
}

// Test_DropRequestBody_ClearsTheParsedForm pins why the PostArgs reset is there:
// Request.Write falls back to the parsed form when the body is empty, so a
// caller that had read PostArgs would otherwise have sent the dropped body on
// the redirected GET.
func Test_DropRequestBody_ClearsTheParsedForm(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://example.com/y")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")
	require.Equal(t, "a=1", req.PostArgs().String(), "the caller reads the form first")

	req.Header.SetMethod(fasthttp.MethodGet)
	dropRequestBody(req)

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, req.Write(w))
	require.NoError(t, w.Flush())
	require.NotContains(t, buf.String(), "a=1", "the dropped form must not come back as the body")
}

// Test_DropRequestBody_ReusedRequestSendsTheNewBody covers the request a caller
// gets back and then reuses without resetting it.
//
// fasthttp clears its parsed-form flag beside the values here, which this cannot
// reach: the field is unexported and only Request.Reset clears it, which would
// take the URI and headers the redirect loop is still using. What survives that
// gap has to be the part that matters — the body actually sent, and the earlier
// form not reappearing in it or in a later read. An untouched fasthttp request
// is no better here: SetBody does not invalidate the parsed form either, so it
// answers a reused PostArgs with the previous body's values.
func Test_DropRequestBody_ReusedRequestSendsTheNewBody(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://example.com/y")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")
	require.Equal(t, "a=1", req.PostArgs().String(), "the caller reads the form first")

	dropRequestBody(req)

	// The caller reuses the request for a second POST.
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("b=2")

	require.NotContains(t, req.PostArgs().String(), "a=1",
		"a reused request must not report the previous body's form values")

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, req.Write(w))
	require.NoError(t, w.Flush())
	require.Contains(t, buf.String(), "b=2", "the reused request sends the body it was given")
	require.NotContains(t, buf.String(), "a=1")
}

// Test_Response_ResetDropsOversizedOriginBuffers covers the recorded origin on a
// pooled Response. A redirect target chooses that path, so an oversized buffer
// is kept out of the pool rather than carried by every later response.
func Test_Response_ResetDropsOversizedOriginBuffers(t *testing.T) {
	t.Parallel()

	resp := &Response{RawResponse: fasthttp.AcquireResponse()}
	defer fasthttp.ReleaseResponse(resp.RawResponse)

	var uri fasthttp.URI
	uri.SetHost("example.com")
	uri.SetPath("/" + strings.Repeat("a", maxOriginBuf))
	resp.setRespondedURI(&uri)
	require.Greater(t, cap(resp.respondedPath), maxOriginBuf)

	resp.Reset()
	require.Empty(t, resp.respondedPath)
	require.LessOrEqual(t, cap(resp.respondedPath), maxOriginBuf, "the oversized buffer is not pooled")

	// An ordinary one is kept, so the common path still reuses its buffer.
	uri.SetPath("/short")
	resp.setRespondedURI(&uri)
	before := cap(resp.respondedPath)
	resp.Reset()
	require.Equal(t, before, cap(resp.respondedPath), "a modest buffer survives for reuse")
}

// Test_DoRedirects_NilResponse covers the response fasthttp's own DoRedirects
// documents as optional — "Response is ignored if resp is nil". Its loop reads
// resp.Header regardless and panics on it; this one acquires its own instead.
func Test_DoRedirects_NilResponse(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()
	hops := 0
	server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
		hops++
		if string(ctx.Path()) == "/start" {
			ctx.Response.Header.Set(fiber.HeaderLocation, "/end")
			ctx.SetStatusCode(fasthttp.StatusFound)
			return
		}
		ctx.SetStatusCode(fasthttp.StatusOK)
	}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, server.Serve(ln))
	}()
	defer func() {
		require.NoError(t, server.Shutdown())
		<-done
	}()

	transport := newStandardClientTransport(&fasthttp.Client{
		Dial: func(string) (net.Conn, error) { return ln.Dial() },
	})

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://example.com/start")

	require.NotPanics(t, func() {
		require.NoError(t, transport.DoRedirects(req, nil, 3))
	})
	require.Equal(t, 2, hops, "the redirect was still followed")
}

// Test_Transport_DoRedirects_RefusesUnparsableInitialURI covers the URI the
// caller supplies, as against the redirect targets covered below.
//
// Request.URI swallows the parse error the same way UpdateBytes does, so the
// loop read a half-parsed URI: "http://example.com:abc/x" keeps the malformed
// authority as its host and loses its path. A HostClient dials its fixed Addr
// whatever the URI says, so that host would have gone out as the Host header of
// a request the caller never wrote. fasthttp's own loop calls the
// error-returning parser after every SetRequestURI; this one now does too.
func Test_Transport_DoRedirects_RefusesUnparsableInitialURI(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{name: "invalid port", uri: "http://example.com:abc/x", wantErr: true},
		{name: "unclosed bracket", uri: "http://[::1/x", wantErr: true},
		{name: "a URI that parses is sent", uri: "http://example.com/x", wantErr: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stub := &stubRedirectClient{}
			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)
			req.SetRequestURI(tc.uri)

			err := doRedirectsWithClient(req, resp, 3, stub)
			if tc.wantErr {
				require.ErrorIs(t, err, fasthttp.ErrorInvalidURI)
				require.Equal(t, 0, stub.CallCount(),
					"a malformed URI must be refused before anything is sent")
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, stub.CallCount())
		})
	}
}

// Test_Transport_DoRedirects_RefusesUnparsableTarget covers a Location fasthttp
// cannot parse. URI.UpdateBytes discards that error and keeps whatever it read,
// so "http://example.com:abc/x" left the host "example.com:abc" and lost the
// path — and a HostClient, which connects to its fixed Addr regardless, would
// have sent a second request carrying that Host. fasthttp's own redirect loop
// returns the error instead, so this one does too.
func Test_Transport_DoRedirects_RefusesUnparsableTarget(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		location string
		wantErr  bool
	}{
		{name: "invalid port", location: "http://example.com:abc/x", wantErr: true},
		{name: "space in the host", location: "http://exa mple.com/x", wantErr: true},
		{name: "unclosed bracket", location: "http://[::1/x", wantErr: true},
		{name: "a target that parses is followed", location: "/next", wantErr: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ln := fasthttputil.NewInmemoryListener()
			var hops atomic.Int64
			server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
				if hops.Add(1) == 1 {
					ctx.Response.Header.Set(fiber.HeaderLocation, tc.location)
					ctx.SetStatusCode(fasthttp.StatusFound)
					return
				}
				ctx.SetStatusCode(fasthttp.StatusOK)
			}}
			done := make(chan struct{})
			go func() {
				defer close(done)
				assert.NoError(t, server.Serve(ln))
			}()
			defer func() {
				require.NoError(t, server.Shutdown())
				<-done
			}()

			transport := newStandardClientTransport(&fasthttp.Client{
				Dial: func(string) (net.Conn, error) { return ln.Dial() },
			})

			req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(resp)
			req.SetRequestURI("http://example.com/start")

			err := transport.DoRedirects(req, resp, 3)
			if tc.wantErr {
				require.ErrorIs(t, err, fasthttp.ErrorInvalidURI)
				require.Equal(t, int64(1), hops.Load(), "the malformed target must not be requested")
				return
			}
			require.NoError(t, err)
			require.Equal(t, fasthttp.StatusOK, resp.StatusCode())
			require.Equal(t, int64(2), hops.Load())
		})
	}
}
