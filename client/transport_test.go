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
	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/fiber/v3/internal/fieldname"
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

// clientTestCWrapLB returns an lbBalancingClient around a balancer over clients:
// a nested balancer that only shows itself through LBClient.
func clientTestCWrapLB(clients ...fasthttp.BalancingClient) *lbBalancingClient {
	return &lbBalancingClient{client: &fasthttp.LBClient{Clients: clients}}
}

type stubRedirectCall struct {
	err      error
	status   *int
	location *string
}

type stubRedirectClient struct {
	onRequest func(req *fasthttp.Request)
	calls     []stubRedirectCall
	callCount int
}

func (s *stubRedirectClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	if s.onRequest != nil {
		s.onRequest(req)
	}
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

// clientTestCRedirectStub returns a stub that redirects once, with status to
// location, and then answers 200.
func clientTestCRedirectStub(status int, location string) *stubRedirectClient {
	return &stubRedirectClient{calls: []stubRedirectCall{
		{status: new(status), location: new(location)},
		{status: new(fasthttp.StatusOK)},
	}}
}

func TestStandardClientTransportCoverage(t *testing.T) {
	t.Parallel()
	var dialCount atomic.Int32
	client := &fasthttp.Client{}
	client.Dial = func(string) (net.Conn, error) {
		dialCount.Add(1)
		return nil, errors.New("dial error")
	}

	transport := newStandardClientTransport(client)

	req, resp := clientTestCReqResp(t)

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

	nestedDialHost.Dial = func(string) (net.Conn, error) { return nil, errors.New("original dial") }

	multiLevelHost.Dial = func(string) (net.Conn, error) { return nil, errors.New("multi-level dial") }

	nestedDialLB := clientTestCWrapLB(nestedDialHost)
	nestedTLSLB := clientTestCWrapLB(nestedTLSHost)
	multiLevelLeaf := clientTestCWrapLB(multiLevelHost)
	multiLevelWrapper := clientTestCWrapLB(multiLevelLeaf)

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
	overrideDial := func(string) (net.Conn, error) {
		overrideDialCalled.Store(true)
		return nil, errors.New("override dial")
	}
	transport.SetDial(overrideDial)
	for _, host := range []*fasthttp.HostClient{hostWithoutOverrides, nestedDialHost, multiLevelHost} {
		overrideDialCalled.Store(false)
		_, err := host.Dial(host.Addr)
		require.Error(t, err, host.Addr)
		require.True(t, overrideDialCalled.Load(), host.Addr)
	}
}

func TestExtractTLSConfigVariations(t *testing.T) {
	t.Parallel()
	require.Nil(t, extractTLSConfig(nil))
	require.Nil(t, extractTLSConfig([]fasthttp.BalancingClient{stubBalancingClient{}}))

	host := &fasthttp.HostClient{TLSConfig: &tls.Config{ServerName: "configured", MinVersion: tls.VersionTLS12}}
	require.Equal(t, host.TLSConfig, extractTLSConfig([]fasthttp.BalancingClient{host}))

	nested := &fasthttp.HostClient{TLSConfig: &tls.Config{ServerName: "nested", MinVersion: tls.VersionTLS12}}
	nestedLB := clientTestCWrapLB(nested)
	require.Equal(t, nested.TLSConfig, extractTLSConfig([]fasthttp.BalancingClient{nestedLB}))

	multiLayerLB := clientTestCWrapLB(nestedLB)
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
	nestedLB := clientTestCWrapLB(nested)
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
	type dropCase struct {
		name       string
		method     string
		wantMethod string
		status     int
	}

	statuses := []int{fasthttp.StatusMovedPermanently, fasthttp.StatusFound, fasthttp.StatusSeeOther}
	methods := []string{fasthttp.MethodPost, fasthttp.MethodPut, fasthttp.MethodPatch, fasthttp.MethodDelete, "QUERY"}
	var cases []dropCase
	for _, status := range statuses {
		for _, method := range methods {
			cases = append(cases, dropCase{
				name:   strconv.Itoa(status) + "/" + method,
				method: method, status: status, wantMethod: fasthttp.MethodGet,
			})
		}
	}
	// GET and HEAD keep their method: RFC 9110 Section 15.4.4 says GET *or* HEAD.
	// The body still goes, or a GET carrying one replays it to the target.
	for _, method := range []string{fasthttp.MethodGet, fasthttp.MethodHead} {
		cases = append(cases, dropCase{
			name: method + " keeps its method but not its body", method: method, wantMethod: method,
			status: fasthttp.StatusSeeOther,
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, resp := clientTestCReqResp(t)

			req.SetRequestURI("http://example.com/start")
			req.Header.SetMethod(tc.method)
			req.Header.SetContentType("application/json")
			req.Header.Set(fasthttp.HeaderContentEncoding, "gzip")
			req.Header.Set(fasthttp.HeaderTrailer, "X-Checksum")
			req.SetBodyString(`{"q":"secret"}`)

			client := clientTestCRedirectStub(tc.status, "http://other.example/next")
			require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: -1}, client))

			require.Equal(t, tc.wantMethod, string(req.Header.Method()))
			require.Empty(t, req.Body(), "the body must not reach the redirect target")
			require.Empty(t, req.Header.ContentType())
			require.Empty(t, req.Header.Peek(fasthttp.HeaderContentEncoding), "a coding without a body to decode")
			require.Empty(t, req.Header.Peek(fasthttp.HeaderTrailer), "a Trailer only frames a body that is gone")
			require.Empty(t, req.Header.Peek(fasthttp.HeaderTransferEncoding))
		})
	}

	// 307 and 308 exist to preserve the method and body, so they are untouched.
	for _, status := range []int{fasthttp.StatusTemporaryRedirect, fasthttp.StatusPermanentRedirect} {
		t.Run(strconv.Itoa(status)+"/preserved", func(t *testing.T) {
			t.Parallel()
			req, resp := clientTestCReqResp(t)

			req.SetRequestURI("http://example.com/start")
			req.Header.SetMethod(fasthttp.MethodPost)
			req.Header.SetContentType("application/json")
			req.SetBodyString("body-must-survive")

			client := clientTestCRedirectStub(status, "/next")
			require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: -1}, client))

			require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
			require.Equal(t, "body-must-survive", string(req.Body()))
		})
	}
}

func TestDoRedirectsWithClientBranches(t *testing.T) {
	t.Parallel()
	req, resp := clientTestCReqResp(t)

	req.SetRequestURI("http://example.com/start")

	// The steps run in order on one request: each pass sends it as a POST again
	// from wherever the previous redirect left it.
	for _, tc := range []struct {
		location string
		wantURL  string
		status   int
	}{
		{status: fasthttp.StatusMovedPermanently, location: "/redirect", wantURL: "http://example.com/redirect"},
		{status: fasthttp.StatusSeeOther, location: "/see-other", wantURL: "http://example.com/see-other"},
	} {
		resp.Reset()
		req.Header.SetMethod(fasthttp.MethodPost)
		req.Header.SetContentType("application/json")
		req.SetBodyString("payload")

		client := clientTestCRedirectStub(tc.status, tc.location)
		require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: -1}, client),
			"status %d", tc.status)
		require.Equal(t, fasthttp.MethodGet, string(req.Header.Method()), "status %d", tc.status)
		require.Equal(t, tc.wantURL, req.URI().String(), "status %d", tc.status)
		require.Empty(t, req.Body(), "status %d", tc.status)
		require.Empty(t, req.Header.ContentType(), "status %d", tc.status)
	}

	resp.Reset()
	req.Header.SetMethod(fasthttp.MethodPost)
	req.SetRequestURI("http://example.com/again")
	req.SetBodyString("payload")

	// A limit of zero issues the request and counts the first redirect against
	// it, as fasthttp's own DoRedirects does. Returning nil made a request that
	// finished indistinguishable from one that stopped at a hop, and the request
	// is left as it was sent, since the error comes before any rewrite.
	singleCall := &stubRedirectClient{calls: []stubRedirectCall{{status: new(fasthttp.StatusFound), location: new("/ignored")}}}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 0}, singleCall),
		fasthttp.ErrTooManyRedirects)
	require.Equal(t, fasthttp.StatusFound, resp.StatusCode())
	require.Equal(t, fasthttp.MethodPost, string(req.Header.Method()))
	require.Equal(t, "http://example.com/again", req.URI().String())
	require.Equal(t, "payload", string(req.Body()))
	require.Equal(t, 1, singleCall.CallCount())
	require.Equal(t, fasthttp.StatusFound, resp.Header.StatusCode())

	// A redirect that cannot be followed, with a limit of one.
	for _, tc := range []struct {
		wantErr error
		name    string
		calls   []stubRedirectCall
	}{
		{
			name:    "a redirect with no Location",
			calls:   []stubRedirectCall{{status: new(fasthttp.StatusFound)}},
			wantErr: fasthttp.ErrMissingLocation,
		},
		{
			name:    "a Location with a foreign scheme",
			calls:   []stubRedirectCall{{status: new(fasthttp.StatusMovedPermanently), location: new("ftp://example.com")}},
			wantErr: fasthttp.ErrorInvalidURI,
		},
		{
			name:    "a Location with a control character",
			calls:   []stubRedirectCall{{status: new(fasthttp.StatusFound), location: new("/bad\x00path")}},
			wantErr: fasthttp.ErrorInvalidURI,
		},
		{
			name: "more redirects than the limit",
			calls: []stubRedirectCall{
				{status: new(fasthttp.StatusMovedPermanently), location: new("/loop")},
				{status: new(fasthttp.StatusFound), location: new("/final")},
				{status: new(fasthttp.StatusOK)},
			},
			wantErr: fasthttp.ErrTooManyRedirects,
		},
	} {
		resp.Reset()
		req.Header.SetMethod(fasthttp.MethodPost)
		req.SetRequestURI("http://example.com/start")

		client := &stubRedirectClient{calls: tc.calls}
		require.ErrorIs(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 1}, client),
			tc.wantErr, tc.name)
	}
}

// TestComposeRedirectURL_RejectsHTTPSDowngrade verifies that redirects
// from HTTPS origins to plaintext HTTP targets are refused so cookies
// and credentials never leave the TLS boundary.
func TestComposeRedirectURL_RejectsHTTPSDowngrade(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr                                 error
		name, from, location, wantURL, wantHost string
	}{
		{
			name: "HTTPS to HTTP is refused", from: "https://example.com/login", location: "http://example.com/after",
			wantErr: ErrRedirectDowngrade,
		},
		{
			name: "same-origin HTTP to HTTP is fine",
			from: "http://example.com/a", location: "http://example.com/b",
			wantURL: "http://example.com/b", wantHost: "example.com",
		},
		{
			name: "HTTPS to HTTPS is fine",
			from: "https://example.com/a", location: "https://example.com/b",
			wantURL: "https://example.com/b", wantHost: "example.com",
		},
		{
			name: "HTTP to HTTPS upgrade is fine",
			from: "http://example.com/a", location: "https://example.com/b",
			wantURL: "https://example.com/b", wantHost: "example.com",
		},
		{
			// The new host is reported so the caller can drop origin-scoped
			// credentials.
			name: "a cross-host redirect reports the new host",
			from: "http://example.com/a", location: "http://other.example/b",
			wantURL: "http://other.example/b", wantHost: "other.example",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, host, err := composeRedirectURL(tc.from, []byte(tc.location), false)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantURL, out)
			require.Equal(t, tc.wantHost, host)
		})
	}
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

	// newReq returns a request for http://example.com/start that carries every
	// credential, and the response to fill.
	newReq := func(t *testing.T) (*fasthttp.Request, *fasthttp.Response) {
		t.Helper()

		req, resp := clientTestCReqResp(t)
		req.SetRequestURI("http://example.com/start")
		req.Header.Set(fasthttp.HeaderAuthorization, "Bearer secret")
		req.Header.Set(fasthttp.HeaderProxyAuthorization, "Basic secret")
		req.Header.Set(fasthttp.HeaderCookie, "session=secret")
		return req, resp
	}

	for _, tc := range []struct{ name, location string }{
		{name: "subdomain of the initial host keeps credentials", location: "http://www.example.com/next"},
		{name: "explicit default port keeps credentials", location: "http://example.com:80/next"},
		{name: "same host keeps credentials", location: "http://example.com/next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, resp := newReq(t)

			client := clientTestCRedirectStub(fasthttp.StatusFound, tc.location)
			require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 5}, client))
			require.Equal(t, "Bearer secret", string(req.Header.Peek(fasthttp.HeaderAuthorization)))
			require.Equal(t, "Basic secret", string(req.Header.Peek(fasthttp.HeaderProxyAuthorization)))
			require.Equal(t, "session=secret", string(req.Header.Peek(fasthttp.HeaderCookie)))
		})
	}

	t.Run("cross host drops credentials", func(t *testing.T) {
		t.Parallel()
		req, resp := newReq(t)

		client := clientTestCRedirectStub(fasthttp.StatusFound, "http://attacker.example/collect")
		require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 5}, client))
		require.Equal(t, "http://attacker.example/collect", req.URI().String())
		require.Empty(t, req.Header.Peek(fasthttp.HeaderAuthorization))
		require.Empty(t, req.Header.Peek(fasthttp.HeaderProxyAuthorization))
		require.Empty(t, req.Header.Peek(fasthttp.HeaderCookie))
	})
}

func TestDoRedirectsWithClient_DropsBuiltInHeadersWhateverTheirCase(t *testing.T) {
	t.Parallel()

	names := []string{
		"authorization",
		"proxy-authorization",
		"proxy-authenticate",
		"www-authenticate",
		"cookie",
		"cookie2",
	}

	follow := func(t *testing.T, location string) *fasthttp.Request {
		t.Helper()

		req := fasthttp.AcquireRequest()
		t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
		req.SetRequestURI("http://example.com/start")
		// Header.Del is byte-exact and missed these under DisableNormalizing.
		req.Header.DisableNormalizing()
		for _, name := range names {
			req.Header.Add(name, "k=secret")
		}
		req.Header.Add("x-api-key", "key")
		for _, name := range names {
			require.Len(t, fieldname.Lines(&req.Header, name, false), 1, name)
		}

		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
		client := &stubRedirectClient{calls: []stubRedirectCall{
			{status: new(fasthttp.StatusFound), location: &location},
			{status: new(fasthttp.StatusOK)},
		}}
		require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 5}, client))
		return req
	}

	t.Run("another host drops every one", func(t *testing.T) {
		t.Parallel()

		req := follow(t, "http://attacker.example/collect")
		for _, name := range names {
			require.Empty(t, fieldname.Lines(&req.Header, name, false), name)
		}
		require.Len(t, fieldname.Lines(&req.Header, "x-api-key", false), 1)
	})

	t.Run("same host keeps every one", func(t *testing.T) {
		t.Parallel()

		req := follow(t, "http://example.com/next")
		for _, name := range names {
			require.Len(t, fieldname.Lines(&req.Header, name, false), 1, name)
		}
		require.Len(t, fieldname.Lines(&req.Header, "x-api-key", false), 1)
	})
}

func TestDoRedirectsWithClient_DropsListedHeadersCrossHost(t *testing.T) {
	t.Parallel()

	plain := func(req *fasthttp.Request) {
		req.Header.Set(fasthttp.HeaderAuthorization, "Bearer secret")
		req.Header.Set("X-Api-Key", "key")
		req.Header.Set("X-Auth-Token", "tok")
	}
	lowerCase := func(req *fasthttp.Request) {
		req.Header.DisableNormalizing()
		req.Header.Add("authorization", "Bearer secret")
		req.Header.Add("x-api-key", "key")
		req.Header.Add("X-API-KEY", "key")
		req.Header.Add("x-auth-token", "tok")
	}

	// carried counts the lines of each header that reach the transport.
	type carried struct{ authorization, apiKey, authToken int }

	var (
		asSent       = carried{authorization: 1, apiKey: 1, authToken: 1}
		twiceSent    = carried{authorization: 1, apiKey: 2, authToken: 1}
		listedGone   = carried{authToken: 1}
		builtInsGone = carried{apiKey: 1, authToken: 1}
	)

	for _, tc := range []struct {
		setup     func(req *fasthttp.Request)
		name      string
		listed    []string
		locations []string
		want      []carried
	}{
		{
			name: "a listed name is dropped on another host, an unlisted one is kept", setup: plain,
			listed: []string{"X-Api-Key"}, locations: []string{"http://attacker.example/collect"},
			want: []carried{asSent, listedGone},
		},
		{
			name: "every spelling a header is stored under is dropped", setup: lowerCase,
			listed: []string{"X-Api-Key"}, locations: []string{"http://attacker.example/collect"},
			want: []carried{twiceSent, listedGone},
		},
		{
			name: "another port of the same host keeps every header", setup: plain,
			listed: []string{"X-Api-Key"}, locations: []string{"http://example.com:8080/next"},
			want: []carried{asSent, asSent},
		},
		{
			name: "a subdomain keeps every header", setup: plain,
			listed: []string{"X-Api-Key"}, locations: []string{"http://www.example.com/next"},
			want: []carried{asSent, asSent},
		},
		{
			name: "nothing listed forwards the application's headers and drops the built-in set", setup: plain,
			locations: []string{"http://attacker.example/collect"},
			want:      []carried{asSent, builtInsGone},
		},
		{
			name: "they stay gone when a later hop returns to the initial host", setup: plain,
			listed: []string{"X-Api-Key"}, locations: []string{"http://attacker.example/collect", "http://example.com/back"},
			want: []carried{asSent, listedGone, listedGone},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)
			req.SetRequestURI("http://example.com/start")
			tc.setup(req)

			calls := make([]stubRedirectCall, 0, len(tc.locations)+1)
			for _, location := range tc.locations {
				calls = append(calls, stubRedirectCall{status: new(fasthttp.StatusFound), location: new(location)})
			}
			calls = append(calls, stubRedirectCall{status: new(fasthttp.StatusOK)})

			var got []carried
			client := &stubRedirectClient{calls: calls, onRequest: func(req *fasthttp.Request) {
				got = append(got, carried{
					authorization: len(fieldname.Lines(&req.Header, fasthttp.HeaderAuthorization, false)),
					apiKey:        len(fieldname.Lines(&req.Header, "X-Api-Key", false)),
					authToken:     len(fieldname.Lines(&req.Header, "X-Auth-Token", false)),
				})
			}}

			require.NoError(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 5, sensitiveHeaders: crosshost.NewHeaderNames(tc.listed)}, client))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestClient_DoRedirects_DropsClientListedHeaders(t *testing.T) {
	t.Parallel()

	server := startRedirectServer(t, func(c fiber.Ctx) error {
		return c.SendString(c.Get("X-Api-Key") + "|" + c.Get("X-Auth-Token"))
	})
	// Not a defer: the parallel subtests outlive this function.
	t.Cleanup(server.stop)

	do := func(t *testing.T, client *Client, path string) string {
		t.Helper()

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
		req.SetRequestURI("http://example.com" + path)
		req.Header.Set("X-Api-Key", "key")
		req.Header.Set("X-Auth-Token", "tok")

		require.NoError(t, client.DoRedirects(req, resp, 5))
		return string(resp.Body())
	}

	for name, newClient := range map[string]func() *Client{
		"standard": func() *Client { return New().SetDial(server.dial()) },
		"host": func() *Client {
			return NewWithHostClient(&fasthttp.HostClient{Addr: "example.com:80", Dial: server.dial()})
		},
		"balanced": func() *Client {
			return NewWithLBClient(&fasthttp.LBClient{Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: "example.com:80", Dial: server.dial()},
			}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := newClient()

			require.Equal(t, "key|tok", do(t, client, "/start"))

			client.SetRedirectSensitiveHeaders("x-api-key")
			require.Equal(t, "|tok", do(t, client, "/start"))
			require.Equal(t, "key|tok", do(t, client, "/same-host"))

			client.SetRedirectSensitiveHeaders()
			require.Equal(t, "key|tok", do(t, client, "/start"), "an empty call clears the list")
		})
	}
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
			req, resp := clientTestCReqResp(t)

			req.SetRequestURI(tc.base)
			client := &stubRedirectClient{calls: []stubRedirectCall{
				{status: new(fasthttp.StatusFound), location: new(tc.location)},
			}}
			require.ErrorIs(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 5}, client), tc.wantErr)
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
	server := startRedirectServer(t, func(c fiber.Ctx) error {
		return c.SendString(c.Get(fiber.HeaderAuthorization) + "|" + c.Get(fiber.HeaderCookie))
	})
	t.Cleanup(server.stop)
	client := New().SetDial(server.dial())
	credentials := Config{
		Header:       map[string]string{fiber.HeaderAuthorization: "Bearer secret"},
		Cookie:       map[string]string{"session": "secret"},
		MaxRedirects: 5,
	}

	// Control: a redirect that stays on the issuing host carries them through,
	// so the cross-host assertion below cannot pass vacuously.
	sameHost := clientTestBClientGet(t, client, "http://example.com/same-host", credentials)
	require.Equal(t, fiber.StatusOK, sameHost.StatusCode())
	require.Equal(t, "Bearer secret|session=secret", sameHost.String())

	resp := clientTestBClientGet(t, client, "http://example.com/start", credentials)

	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "|", resp.String())
}

func TestDoRedirectsWithClientDefaultLimit(t *testing.T) {
	t.Parallel()
	req, resp := clientTestCReqResp(t)

	req.SetRequestURI("http://example.com/start")
	req.Header.SetMethod(fasthttp.MethodPost)

	calls := make([]stubRedirectCall, 0, defaultRedirectLimit+1)
	for range defaultRedirectLimit + 1 {
		calls = append(calls, stubRedirectCall{status: new(fasthttp.StatusFound), location: new("/loop")})
	}

	client := &stubRedirectClient{calls: calls}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: -1}, client),
		fasthttp.ErrTooManyRedirects)
	require.Equal(t, defaultRedirectLimit+1, client.CallCount())
}

func TestDoRedirectsWithClientDeadlineCoversTheChain(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/start")

	const budget, hop = 100 * time.Millisecond, 30 * time.Millisecond
	calls := make([]stubRedirectCall, 0, 20)
	for range 20 {
		calls = append(calls, stubRedirectCall{status: new(fasthttp.StatusFound), location: new("/loop")})
	}
	var timeouts []time.Duration
	client := &stubRedirectClient{calls: calls, onRequest: func(req *fasthttp.Request) {
		timeouts = append(timeouts, req.GetTimeOut())
		time.Sleep(hop)
	}}

	opts := &redirectOptions{maxRedirects: 20, deadline: time.Now().Add(budget)}
	require.ErrorIs(t, doRedirectsWithClient(req, resp, opts, client), fasthttp.ErrTimeout)

	require.LessOrEqual(t, len(timeouts), int(budget/hop)+1)
	left := budget
	for _, got := range timeouts {
		require.Positive(t, got)
		require.LessOrEqual(t, got, left)
		left = got - hop
	}
}

// clientTestCRunStreamResponseBody registers the scenarios of the streaming
// setting for a transport that wraps one fasthttp client. build returns a
// transport over a fresh client together with a reader of that client's
// StreamResponseBody field.
func clientTestCRunStreamResponseBody(t *testing.T, build func() (httpClientTransport, func() bool)) {
	t.Helper()

	t.Run("default value", func(t *testing.T) {
		t.Parallel()
		transport, _ := build()
		require.False(t, transport.StreamResponseBody())
	})

	t.Run("enable streaming", func(t *testing.T) {
		t.Parallel()
		transport, field := build()
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, field())
	})

	t.Run("disable streaming", func(t *testing.T) {
		t.Parallel()
		transport, field := build()
		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		transport.SetStreamResponseBody(false)
		require.False(t, transport.StreamResponseBody())
		require.False(t, field())
	})
}

func Test_StandardClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()
	clientTestCRunStreamResponseBody(t, func() (httpClientTransport, func() bool) {
		client := &fasthttp.Client{}
		return newStandardClientTransport(client), func() bool { return client.StreamResponseBody }
	})
}

func Test_HostClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()
	clientTestCRunStreamResponseBody(t, func() (httpClientTransport, func() bool) {
		hostClient := &fasthttp.HostClient{}
		return newHostClientTransport(hostClient), func() bool { return hostClient.StreamResponseBody }
	})
}

func Test_LBClientTransport_StreamResponseBody(t *testing.T) {
	t.Parallel()
	t.Run("empty clients", func(t *testing.T) {
		t.Parallel()
		lbClient := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{}}
		transport := newLBClientTransport(lbClient)
		require.False(t, transport.StreamResponseBody())
	})

	t.Run("single host client", func(t *testing.T) {
		t.Parallel()
		clientTestCRunStreamResponseBody(t, func() (httpClientTransport, func() bool) {
			hostClient := &fasthttp.HostClient{Addr: "example.com:80"}
			lbClient := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hostClient}}
			return newLBClientTransport(lbClient), func() bool { return hostClient.StreamResponseBody }
		})
	})

	t.Run("multiple host clients", func(t *testing.T) {
		t.Parallel()
		hostClient1 := &fasthttp.HostClient{Addr: "example1.com:80"}
		hostClient2 := &fasthttp.HostClient{Addr: "example2.com:80"}
		lbClient := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hostClient1, hostClient2}}
		transport := newLBClientTransport(lbClient)

		transport.SetStreamResponseBody(true)
		require.True(t, transport.StreamResponseBody())
		require.True(t, hostClient1.StreamResponseBody)
		require.True(t, hostClient2.StreamResponseBody)

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
		{name: "standardClientTransport", transport: newStandardClientTransport(&fasthttp.Client{})},
		{name: "hostClientTransport", transport: newHostClientTransport(&fasthttp.HostClient{})},
		{
			name: "lbClientTransport",
			transport: newLBClientTransport(&fasthttp.LBClient{
				Clients: []fasthttp.BalancingClient{&fasthttp.HostClient{Addr: "example.com:80"}},
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

// Test_Transport_PathNormalizing covers the setting that decides what reaches the
// wire: the wrapped client's own field, read and written through every kind of
// transport.
func Test_Transport_PathNormalizing(t *testing.T) {
	t.Parallel()
	t.Run("fasthttp.Client the Fiber client made", func(t *testing.T) {
		t.Parallel()
		tr := newOwnedStandardClientTransport()
		fc := tr.client
		require.False(t, tr.DisablePathNormalizing())

		tr.SetDisablePathNormalizing(true)
		require.True(t, tr.DisablePathNormalizing())
		require.True(t, fc.DisablePathNormalizing)

		tr.SetDisablePathNormalizing(false)
		require.False(t, tr.DisablePathNormalizing())
		require.False(t, fc.DisablePathNormalizing)
	})

	t.Run("fasthttp.Client handed over normalizing", func(t *testing.T) {
		t.Parallel()
		fc := &fasthttp.Client{}
		tr := newStandardClientTransport(fc)
		require.False(t, tr.DisablePathNormalizing())

		// The field is written, so hosts not served yet get it, but the
		// transport cannot say the hosts served already follow.
		tr.SetDisablePathNormalizing(true)
		require.True(t, fc.DisablePathNormalizing)
		require.False(t, tr.DisablePathNormalizing())

		tr.SetDisablePathNormalizing(false)
		require.False(t, fc.DisablePathNormalizing)
		require.False(t, tr.DisablePathNormalizing())
	})

	t.Run("fasthttp.Client handed over with the setting on", func(t *testing.T) {
		t.Parallel()
		fc := &fasthttp.Client{DisablePathNormalizing: true}
		tr := newStandardClientTransport(fc)
		require.True(t, tr.DisablePathNormalizing())

		tr.SetDisablePathNormalizing(false)
		require.False(t, tr.DisablePathNormalizing())
		require.False(t, fc.DisablePathNormalizing)

		// Still known: no request has gone out in between.
		tr.SetDisablePathNormalizing(true)
		require.True(t, tr.DisablePathNormalizing())
	})

	t.Run("fasthttp.HostClient", func(t *testing.T) {
		t.Parallel()
		hc := &fasthttp.HostClient{}
		tr := newHostClientTransport(hc)
		require.False(t, tr.DisablePathNormalizing())

		tr.SetDisablePathNormalizing(true)
		require.True(t, tr.DisablePathNormalizing())
		require.True(t, hc.DisablePathNormalizing)

		tr.SetDisablePathNormalizing(false)
		require.False(t, hc.DisablePathNormalizing)
	})

	t.Run("fasthttp.LBClient writes every host client, nested ones included", func(t *testing.T) {
		t.Parallel()
		direct := &fasthttp.HostClient{Addr: "a.example:80"}
		inner := &fasthttp.HostClient{Addr: "b.example:80"}
		deeper := &fasthttp.HostClient{Addr: "c.example:80"}
		tr := newLBClientTransport(&fasthttp.LBClient{Clients: []fasthttp.BalancingClient{
			direct,
			clientTestCWrapLB(inner, clientTestCWrapLB(deeper)),
		}})
		require.False(t, tr.DisablePathNormalizing())

		tr.SetDisablePathNormalizing(true)
		require.True(t, tr.DisablePathNormalizing())
		for _, hc := range []*fasthttp.HostClient{direct, inner, deeper} {
			require.True(t, hc.DisablePathNormalizing, hc.Addr)
		}

		tr.SetDisablePathNormalizing(false)
		for _, hc := range []*fasthttp.HostClient{direct, inner, deeper} {
			require.False(t, hc.DisablePathNormalizing, hc.Addr)
		}
	})

	t.Run("fasthttp.LBClient reports false while any client normalizes", func(t *testing.T) {
		t.Parallel()
		quiet := &fasthttp.HostClient{Addr: "a.example:80", DisablePathNormalizing: true}
		normalizing := &fasthttp.HostClient{Addr: "b.example:80"}

		tests := []struct {
			name    string
			clients []fasthttp.BalancingClient
			want    bool
		}{
			{name: "all leave the path alone", clients: []fasthttp.BalancingClient{quiet, quiet}, want: true},
			{name: "one normalizes", clients: []fasthttp.BalancingClient{quiet, normalizing}},
			{
				name:    "a nested balancer holds one that normalizes",
				clients: []fasthttp.BalancingClient{quiet, clientTestCWrapLB(quiet, normalizing)},
			},
			{
				name:    "a nested balancer where all leave it alone",
				clients: []fasthttp.BalancingClient{quiet, clientTestCWrapLB(quiet)},
				want:    true,
			},
			{name: "a nested balancer with no clients", clients: []fasthttp.BalancingClient{quiet, clientTestCWrapLB()}},
			{name: "a client that cannot be inspected", clients: []fasthttp.BalancingClient{quiet, stubBalancingClient{}}},
			{name: "a wrapper exposing no balancer", clients: []fasthttp.BalancingClient{quiet, &lbBalancingClient{}}},
			{name: "no clients", clients: nil},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				tr := newLBClientTransport(&fasthttp.LBClient{Clients: tc.clients})
				require.Equal(t, tc.want, tr.DisablePathNormalizing())
			})
		}
	})

	t.Run("setting an empty balancer is harmless", func(t *testing.T) {
		t.Parallel()
		tr := newLBClientTransport(&fasthttp.LBClient{})
		tr.SetDisablePathNormalizing(true)
		require.False(t, tr.DisablePathNormalizing(), "nothing behind it can be told")
	})
}

// Test_Client_SetDisablePathNormalizing_AppliesToTheWrappedClient checks the
// setter reaches the fasthttp client each constructor wraps, and that Reset
// returns to a default client that normalizes.
func Test_Client_SetDisablePathNormalizing_AppliesToTheWrappedClient(t *testing.T) {
	t.Parallel()
	host := &fasthttp.HostClient{Addr: "example.com:80"}
	lbHost := &fasthttp.HostClient{Addr: "example.com:80"}
	std := &fasthttp.Client{}

	tests := []struct {
		client *Client
		field  func() bool
		name   string
		// lifts is whether the setter alone makes the client send paths as built
		// as far as Fiber can tell. A fasthttp.Client handed to NewWithClient may
		// have served hosts already, so it does not.
		lifts bool
	}{
		{name: "New", client: New(), lifts: true},
		{name: "NewWithClient", client: NewWithClient(std), field: func() bool { return std.DisablePathNormalizing }},
		{name: "NewWithHostClient", client: NewWithHostClient(host), field: func() bool { return host.DisablePathNormalizing }, lifts: true},
		{
			name:   "NewWithLBClient",
			client: NewWithLBClient(&fasthttp.LBClient{Clients: []fasthttp.BalancingClient{lbHost}}),
			field:  func() bool { return lbHost.DisablePathNormalizing },
			lifts:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := tc.client
			field := tc.field
			if field == nil { // New: the client creates its own fasthttp client
				field = func() bool { return c.FasthttpClient().DisablePathNormalizing }
			}
			require.False(t, c.writesPathAsBuilt())

			c.SetDisablePathNormalizing(true)
			require.True(t, c.DisablePathNormalizing())
			require.True(t, field())
			require.Equal(t, tc.lifts, c.writesPathAsBuilt())

			c.SetDisablePathNormalizing(false)
			require.False(t, c.DisablePathNormalizing())
			require.False(t, field())
			require.False(t, c.writesPathAsBuilt())
		})
	}

	t.Run("NewWithClient created with the setting on", func(t *testing.T) {
		t.Parallel()
		std := &fasthttp.Client{DisablePathNormalizing: true}
		c := NewWithClient(std)
		require.True(t, c.writesPathAsBuilt())
		require.False(t, c.DisablePathNormalizing(), "the Fiber setting is not what turned it on")

		c.SetDisablePathNormalizing(false)
		require.False(t, std.DisablePathNormalizing)
		require.False(t, c.writesPathAsBuilt())
	})

	t.Run("Reset", func(t *testing.T) {
		t.Parallel()
		c := New().SetDisablePathNormalizing(true)
		require.True(t, c.writesPathAsBuilt())

		c.Reset()
		require.False(t, c.DisablePathNormalizing())
		require.False(t, c.writesPathAsBuilt())
		require.False(t, c.FasthttpClient().DisablePathNormalizing)
	})
}

// Test_Client_PathNormalizing_UnknownTransport covers a transport that has no
// path normalizing setting to read: it counts as one that normalizes, so setting
// the flag does nothing to it and the path parameter check stays on.
func Test_Client_PathNormalizing_UnknownTransport(t *testing.T) {
	t.Parallel()
	c := newClient(newBlockingErrTransport(nil))
	require.NotPanics(t, func() { c.SetDisablePathNormalizing(true) })
	require.True(t, c.DisablePathNormalizing())
	require.False(t, c.writesPathAsBuilt())

	req := AcquireRequest().SetURL("http://example.com/api/:id").SetPathParam("id", "x/../../admin").
		SetDisablePathNormalizing(true)
	t.Cleanup(func() { ReleaseRequest(req) })

	require.ErrorIs(t, parserRequestURL(c, req), ErrPathParamInPath)
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
			seen := make(chan string, 4)
			dial := clientTestCServe(t, func(ctx *fasthttp.RequestCtx) {
				seen <- string(ctx.Request.URI().PathOriginal())
				ctx.SetStatusCode(fasthttp.StatusOK)
			})

			transport := newStandardClientTransport(&fasthttp.Client{DisablePathNormalizing: tc.disable, Dial: dial})

			req, resp := clientTestCReqResp(t)
			req.SetRequestURI("http://example.com/a//b")

			require.NoError(t, transport.DoRedirects(req, resp, &redirectOptions{maxRedirects: 3}))
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
	req := clientTestPooled(t, fasthttp.AcquireRequest, fasthttp.ReleaseRequest)
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
	req := clientTestPooled(t, fasthttp.AcquireRequest, fasthttp.ReleaseRequest)
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
	req := clientTestPooled(t, fasthttp.AcquireRequest, fasthttp.ReleaseRequest)
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
	hops := 0
	dial := clientTestCServe(t, func(ctx *fasthttp.RequestCtx) {
		hops++
		if string(ctx.Path()) == "/start" {
			ctx.Response.Header.Set(fiber.HeaderLocation, "/end")
			ctx.SetStatusCode(fasthttp.StatusFound)
			return
		}
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	transport := newStandardClientTransport(&fasthttp.Client{Dial: dial})

	req := clientTestPooled(t, fasthttp.AcquireRequest, fasthttp.ReleaseRequest)
	req.SetRequestURI("http://example.com/start")

	require.NotPanics(t, func() {
		require.NoError(t, transport.DoRedirects(req, nil, &redirectOptions{maxRedirects: 3}))
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
			req, resp := clientTestCReqResp(t)
			req.SetRequestURI(tc.uri)

			err := doRedirectsWithClient(req, resp, &redirectOptions{maxRedirects: 3}, stub)
			if tc.wantErr {
				require.ErrorIs(t, err, fasthttp.ErrorInvalidURI)
				require.Equal(t, 0, stub.CallCount(), "a malformed URI must be refused before anything is sent")
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
			var hops atomic.Int64
			dial := clientTestCServe(t, func(ctx *fasthttp.RequestCtx) {
				if hops.Add(1) == 1 {
					ctx.Response.Header.Set(fiber.HeaderLocation, tc.location)
					ctx.SetStatusCode(fasthttp.StatusFound)
					return
				}
				ctx.SetStatusCode(fasthttp.StatusOK)
			})

			transport := newStandardClientTransport(&fasthttp.Client{Dial: dial})

			req, resp := clientTestCReqResp(t)
			req.SetRequestURI("http://example.com/start")

			err := transport.DoRedirects(req, resp, &redirectOptions{maxRedirects: 3})
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
