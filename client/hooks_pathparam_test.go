package client

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Test_SubstitutePathParams covers the placeholder grammar directly: what counts
// as a placeholder, where one ends, and what happens to a value on the way in.
func Test_SubstitutePathParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		params  PathParam
		name    string
		uri     string
		want    string
		keepEsc bool
	}{
		{
			name:   "plain substitution",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/5",
		},
		{
			name:   "a longer placeholder is not matched by a shorter name",
			uri:    "http://example.com/api/:idx",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/:idx",
		},
		{
			name:   "both names resolve independently",
			uri:    "http://example.com/api/:idx/:id",
			params: PathParam{"id": "5", "idx": "9"},
			want:   "http://example.com/api/9/5",
		},
		{
			name:   "a dot ends a placeholder, as it does in a route",
			uri:    "http://example.com/api/:id.json",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/5.json",
		},
		{
			name:   "a dash ends a placeholder, as it does in a route",
			uri:    "http://example.com/api/:id-suffix",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/5-suffix",
		},
		{
			name:   "a name holding a dash resolves, not just its prefix",
			uri:    "http://example.com/api/:api-version/users",
			params: PathParam{"api-version": "v2"},
			want:   "http://example.com/api/v2/users",
		},
		{
			name:   "a name holding a dot resolves, not just its prefix",
			uri:    "http://example.com/api/:v1.0/x",
			params: PathParam{"v1.0": "beta"},
			want:   "http://example.com/api/beta/x",
		},
		{
			name:   "a name holding a colon resolves in the path",
			uri:    "http://example.com/api/:ns:key",
			params: PathParam{"ns:key": "v"},
			want:   "http://example.com/api/v",
		},
		{
			name:   "the longer name wins when both are set",
			uri:    "http://example.com/api/:id-suffix",
			params: PathParam{"id": "5", "id-suffix": "full"},
			want:   "http://example.com/api/full",
		},
		{
			name:   "adjacent placeholders",
			uri:    "http://example.com/api/:a:b",
			params: PathParam{"a": "1", "b": "2"},
			want:   "http://example.com/api/12",
		},
		{
			name:   "the same placeholder twice",
			uri:    "http://example.com/api/:id/sub/:id",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/5/sub/5",
		},
		{
			name:   "an unknown placeholder is left alone",
			uri:    "http://example.com/api/:missing",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/:missing",
		},
		{
			name:   "the scheme separator is not a placeholder",
			uri:    "http://example.com/api",
			params: PathParam{"": "x", "//example": "y"},
			want:   "http://example.com/api",
		},
		{
			// A digits-only name in the authority is the port. Substituting it
			// would take the ":" with it and fold the port into the host, which
			// is what the old ReplaceAll did ("example.com9090").
			name:   "a port is not a placeholder",
			uri:    "http://example.com:8080/api/:id",
			params: PathParam{"id": "5", "8080": "9090"},
			want:   "http://example.com:8080/api/5",
		},
		{
			name:   "a host can still be templated",
			uri:    "http://:tenant.example.com:8080/api/:id",
			params: PathParam{"tenant": "acme", "id": "5"},
			want:   "http://acme.example.com:8080/api/5",
		},
		{
			name:   "a digits-only name still substitutes in the path",
			uri:    "http://example.com:8080/api/:8080",
			params: PathParam{"8080": "9090"},
			want:   "http://example.com:8080/api/9090",
		},
		{
			name:   "a relative URI has no authority to protect",
			uri:    "/api/:8080",
			params: PathParam{"8080": "9090"},
			want:   "/api/9090",
		},
		{
			name:   "a port with no matching parameter is left alone",
			uri:    "http://example.com:8080/api/:id",
			params: PathParam{"id": "5"},
			want:   "http://example.com:8080/api/5",
		},
		{
			name:   "an IPv6 host is left intact",
			uri:    "http://[::1]:8080/api/:id",
			params: PathParam{"id": "5"},
			want:   "http://[::1]:8080/api/5",
		},
		{
			name:   "a value cannot open a query",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a?b"},
			want:   "http://example.com/api/a%3Fb",
		},
		{
			name:   "a value cannot open a fragment",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a#b"},
			want:   "http://example.com/api/a%23b",
		},
		{
			// Path normalizing would decode this "%2F" back into a separator,
			// so the escape only holds when normalizing is off.
			name:    "a value cannot add a segment",
			uri:     "http://example.com/api/:id",
			params:  PathParam{"id": "a/b"},
			want:    "http://example.com/api/a%2Fb",
			keepEsc: true,
		},
		{
			name:    "a separator in a value is rejected when normalizing will decode it",
			uri:     "http://example.com/api/:id",
			params:  PathParam{"id": "a/b"},
			wantErr: ErrPathParamInPath,
		},
		{
			// Normalizing decodes the path before it collapses "..", so this
			// used to leave the path the template set: "/api/v1/:id/end" with
			// "../../admin" went out as "/admin/end".
			name:    "a value cannot traverse out of its segment",
			uri:     "http://example.com/api/v1/:id/end",
			params:  PathParam{"id": "../../admin"},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a dot-dot value is rejected",
			uri:     "http://example.com/api/v1/:id/end",
			params:  PathParam{"id": ".."},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a dot value is rejected",
			uri:     "http://example.com/api/v1/:id/end",
			params:  PathParam{"id": "."},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a backslash in a value is rejected",
			uri:     "http://example.com/api/:id",
			params:  PathParam{"id": `a\b`},
			wantErr: ErrPathParamInPath,
		},
		{
			// Disabling normalizing is a caller asking for the raw path, so the
			// escape is emitted as written. Whether it survives routing is then
			// up to the server: one that unescapes paths decodes it again.
			name:    "traversal is allowed to stay escaped when normalizing is off",
			uri:     "http://example.com/api/v1/:id/end",
			params:  PathParam{"id": "../../admin"},
			want:    "http://example.com/api/v1/..%2F..%2Fadmin/end",
			keepEsc: true,
		},
		{
			// An escape cannot hide a dot segment: sent as written, "/files/pub/../meta"
			// is resolved by the server or a proxy to "/files/meta".
			name:    "a dot-dot value is rejected when normalizing is off",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": ".."},
			wantErr: ErrPathParamInPath,
			keepEsc: true,
		},
		{
			name:    "a dot value is rejected when normalizing is off",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": "."},
			wantErr: ErrPathParamInPath,
			keepEsc: true,
		},
		{
			name:    "a value of three dots is an ordinary segment when normalizing is off",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": "..."},
			want:    "http://example.com/files/pub/.../meta",
			keepEsc: true,
		},
		{
			// Normalizing writes the ";" raw, and a servlet container strips it
			// before it resolves the dot segment: "/files/pub/..;/meta" is "/files/meta".
			name:    "a dot-dot value carrying path parameters is rejected",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": "..;"},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a dot-dot value with a named path parameter is rejected",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": "..;jsessionid=x"},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a dot value carrying path parameters is rejected",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": ".;"},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a value that is only path parameters is rejected",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": ";x"},
			wantErr: ErrPathParamInPath,
		},
		{
			name:    "a dot-dot value carrying path parameters is rejected when normalizing is off",
			uri:     "http://example.com/files/pub/:name/meta",
			params:  PathParam{"name": "..;"},
			wantErr: ErrPathParamInPath,
			keepEsc: true,
		},
		{
			name:   "a segment that merely carries path parameters is kept",
			uri:    "http://example.com/files/pub/:name/meta",
			params: PathParam{"name": "a;jsessionid=x"},
			want:   "http://example.com/files/pub/a%3Bjsessionid=x/meta",
		},
		{
			// A "%2F" the caller typed is escaped to "%252F" and decodes to a
			// literal "%2F", so it never becomes a separator.
			name:   "an already-escaped separator in a value is not a separator",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a%2Fb"},
			want:   "http://example.com/api/a%252Fb",
		},
		{
			name:   "a dot inside a longer value is fine",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a..b"},
			want:   "http://example.com/api/a..b",
		},
		{
			name:   "a percent in a value is encoded, not passed through",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "100%"},
			want:   "http://example.com/api/100%25",
		},
		{
			name:   "a substituted value is not scanned for placeholders",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": ":name", "name": "fiber"},
			want:   "http://example.com/api/:name",
		},
		{
			// The "//" this leaves is collapsed by path normalizing, so the
			// segment disappears and "/api/:id/x" is sent as "/api/x".
			name:    "an empty value is rejected",
			uri:     "http://example.com/api/:id/x",
			params:  PathParam{"id": ""},
			wantErr: ErrPathParamInPath,
		},
		{
			// Sent as written, "/api//x" is read as "/api/x" by a server that
			// merges slashes, so the segment is gone there too.
			name:    "an empty value is rejected when normalizing is off",
			uri:     "http://example.com/api/:id/x",
			params:  PathParam{"id": ""},
			wantErr: ErrPathParamInPath,
			keepEsc: true,
		},
		{
			name:   "no placeholder, no work",
			uri:    "http://example.com/api/users",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/users",
		},
		{
			name:   "a relative URI with no colon at all short-circuits",
			uri:    "/api/users",
			params: PathParam{"id": "5"},
			want:   "/api/users",
		},
		{
			// "@" would end a userinfo section and hand the request to
			// evil.com. Escaping it is not an option — a "%XX" below 0x80 in a
			// host makes fasthttp abandon the rest of the URI — so the request
			// fails instead.
			name:    "a value cannot introduce userinfo in the authority",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "a@evil.com"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:    "a value cannot introduce a port in the authority",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "evil.com:8080"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:    "a value that would need escaping in a host is rejected",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "a b"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:    "a value cannot add a segment to the authority",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "evil.com/"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:    "an empty authority value is rejected",
			uri:     "http://:host/api",
			params:  PathParam{"host": ""},
			wantErr: ErrPathParamInHost,
		},
		{
			// The sub-delims a host parse keeps verbatim, so they need no
			// escaping and are not a reason to fail the request.
			name:   "host-legal sub-delims are allowed in an authority value",
			uri:    "http://:tenant.example.com/api",
			params: PathParam{"tenant": "a$b&c+d=e"},
			want:   "http://a$b&c+d=e.example.com/api",
		},
		{
			// The bytes reach the Host header verbatim, and "\xc0\xaf" is the
			// overlong encoding of "/" — a separator to a lenient decoder.
			name:    "malformed UTF-8 in an authority value is rejected",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "\xc0\xaf"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:    "a lone continuation byte in an authority value is rejected",
			uri:     "http://:tenant.example.com/api",
			params:  PathParam{"tenant": "\xff"},
			wantErr: ErrPathParamInHost,
		},
		{
			name:   "an internationalized host label is passed through",
			uri:    "http://:tenant.example.com/api",
			params: PathParam{"tenant": "\u00fcn\u00efcode"},
			want:   "http://\u00fcn\u00efcode.example.com/api",
		},
		{
			// The same bytes in a path segment are harmless: net/url's
			// PathEscape leaves them, and nothing in a path reparses them.
			name:   "the same delimiters are left verbatim in the path",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a@b:c"},
			want:   "http://example.com/api/a@b:c",
		},
		{
			// The colons in "[2001:db8::1]" are the address, not placeholder
			// separators, so a "db8" parameter must not rewrite the host.
			name:   "an IPv6 literal is not a set of placeholders",
			uri:    "http://[2001:db8::1]/api/:id",
			params: PathParam{"db8": "PWN", "id": "5"},
			want:   "http://[2001:db8::1]/api/5",
		},
		{
			name:   "an IPv6 literal with a port is left intact",
			uri:    "http://[2001:db8::1]:8080/api/:id",
			params: PathParam{"db8": "PWN", "8080": "9090", "id": "5"},
			want:   "http://[2001:db8::1]:8080/api/5",
		},
		{
			// A digits-only name is the port and is skipped, but a named
			// placeholder in the port position is a normal substitution.
			name:   "a port can be templated under a non-numeric name",
			uri:    "http://example.com::port/api/:id",
			params: PathParam{"port": "8080", "id": "5"},
			want:   "http://example.com:8080/api/5",
		},
		{
			name:   "a placeholder after an IPv6 literal still resolves",
			uri:    "http://[::1]:8080/api/:id",
			params: PathParam{"id": "5"},
			want:   "http://[::1]:8080/api/5",
		},
		{
			name:   "a trailing colon is not a placeholder",
			uri:    "http://example.com/api/:",
			params: PathParam{"": "x"},
			want:   "http://example.com/api/:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := substitutePathParams(tc.uri, tc.keepEsc, tc.params)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_SubstitutePathParams_SourceOrder pins that the first source wins, which is
// what keeps request parameters overriding the client's.
func Test_SubstitutePathParams_SourceOrder(t *testing.T) {
	t.Parallel()

	got, err := substitutePathParams(
		"http://example.com/api/:id/:name",
		false,
		PathParam{"id": "request"},
		PathParam{"id": "client", "name": "client"},
	)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/api/request/client", got)
}

// Test_PathParam_RoundTrip sends each value through a real server and checks the
// handler reads back exactly what was set: escaping on the way out has to be
// undone by the server's own decoding, not just look right in the URL.
func Test_PathParam_RoundTrip(t *testing.T) {
	t.Parallel()

	values := []string{
		"plain",
		"a b",
		"a?b",
		"a#b",
		"a&b=c",
		"100%",
		"a%2Fb",
		"a:b",
		"a.b",
		"a-b",
		"\u00fcn\u00efcode",
		"5",
	}

	ln := fasthttputil.NewInmemoryListener()
	// UnescapePath makes the server decode the segment once, which is the half
	// of the round trip that proves the escaping was correct rather than lossy.
	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Get("/api/:id/end", func(c fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	ch := make(chan struct{})
	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
		close(ch)
	}()
	// Cleanup rather than defer: it runs after the parallel subtests below have
	// finished, where a defer would tear the server down while they still run.
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("timeout when waiting for server close")
		}
	})

	client := New().SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })

	for _, val := range values {
		t.Run(val, func(t *testing.T) {
			t.Parallel()

			resp, err := AcquireRequest().
				SetClient(client).
				SetPathParam("id", val).
				Get("http://example.com/api/:id/end")
			require.NoError(t, err)
			defer resp.Close()

			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, val, resp.String())
		})
	}
}

// Test_PathParam_Traversal_NeverDials proves the traversal never leaves the
// process: "../../admin" in a value used to be escaped, then decoded again by
// path normalizing, and "/api/v1/:id/end" went out as "/admin/end".
func Test_PathParam_Traversal_NeverDials(t *testing.T) {
	t.Parallel()

	var dialed atomic.Bool
	client := New().SetDial(func(_ string) (net.Conn, error) {
		dialed.Store(true)
		return nil, errors.New("dial must not happen")
	})

	_, err := AcquireRequest().
		SetClient(client).
		SetPathParam("id", "../../admin").
		Get("http://example.com/api/v1/:id/end")
	require.ErrorIs(t, err, ErrPathParamInPath)
	require.False(t, dialed.Load(), "the request was sent")
}

// pathRecorder is a server that records the request target exactly as it
// arrived. It is the only reliable view of what the client wrote: the URI the
// client holds is rewritten once more by fasthttp just before the request goes
// out, so what Fiber built and what is sent can differ.
type pathRecorder struct {
	dial   fasthttp.DialFunc
	target chan string
}

func startPathRecorder(t *testing.T) *pathRecorder {
	t.Helper()

	mem := fasthttputil.NewInmemoryListener()
	ln := &acceptSignalListener{Listener: mem, accepting: make(chan struct{})}
	rec := &pathRecorder{
		target: make(chan string, 16),
		dial:   func(string) (net.Conn, error) { return mem.Dial() },
	}
	server := &fasthttp.Server{Handler: func(ctx *fasthttp.RequestCtx) {
		rec.target <- string(ctx.RequestURI())
		ctx.SetStatusCode(fasthttp.StatusOK)
	}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, server.Serve(ln))
	}()
	// A test that expects the request to fail never reaches the server, so
	// without this Shutdown can run before Serve has registered its listener,
	// find nothing to close, and leave Serve blocked in Accept for good.
	<-ln.accepting
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown())
		<-done
	})

	return rec
}

// acceptSignalListener closes accepting the first time it is asked to accept,
// which is when the server has registered the listener.
type acceptSignalListener struct {
	net.Listener
	accepting chan struct{}
	once      sync.Once
}

func (l *acceptSignalListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.accepting) })
	return l.Listener.Accept() //nolint:wrapcheck // a transparent wrapper
}

// next returns the next request target the server saw.
func (r *pathRecorder) next(t *testing.T) string {
	t.Helper()

	select {
	case got := <-r.target:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("the server saw no request")
		return ""
	}
}

// pathClientKinds lists every way of wrapping a fasthttp client. wrapped is the
// DisablePathNormalizing the fasthttp client itself is created with, the only
// way to have it before the Fiber client's own setting is applied. setterLifts
// says whether SetDisablePathNormalizing alone lifts the path parameter check
// for a client of that kind: NewWithClient cannot tell whether the fasthttp
// client it is given has served a host already, so it needs wrapped as well.
var pathClientKinds = []struct {
	build       func(dial fasthttp.DialFunc, wrapped bool) *Client
	name        string
	setterLifts bool
}{
	{name: "New", setterLifts: true, build: func(dial fasthttp.DialFunc, wrapped bool) *Client {
		c := New().SetDial(dial)
		c.FasthttpClient().DisablePathNormalizing = wrapped
		return c
	}},
	{name: "NewWithClient", build: func(dial fasthttp.DialFunc, wrapped bool) *Client {
		return NewWithClient(&fasthttp.Client{Dial: dial, DisablePathNormalizing: wrapped})
	}},
	{name: "NewWithHostClient", setterLifts: true, build: func(dial fasthttp.DialFunc, wrapped bool) *Client {
		return NewWithHostClient(&fasthttp.HostClient{Addr: "example.com:80", Dial: dial, DisablePathNormalizing: wrapped})
	}},
	{name: "NewWithLBClient", setterLifts: true, build: func(dial fasthttp.DialFunc, wrapped bool) *Client {
		return NewWithLBClient(&fasthttp.LBClient{
			Timeout: 5 * time.Second,
			Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: "example.com:80", Dial: dial, DisablePathNormalizing: wrapped},
			},
		})
	}},
}

// Test_Client_DisablePathNormalizing_Wire checks the path a server receives.
// The Fiber setting used to be written to the request's URI only, and fasthttp
// overwrites that field with its own client's just before it writes the
// request, so "/a//b/./c%2Fd" went out as "/a/b/c/d" however it was set.
func Test_Client_DisablePathNormalizing_Wire(t *testing.T) {
	t.Parallel()

	const (
		target     = "http://example.com/a//b/./c%2Fd"
		asWritten  = "/a//b/./c%2Fd"
		normalized = "/a/b/c/d"
	)

	cases := []struct {
		setup func(c *Client) *Client
		name  string
		want  string
		cfg   Config
		// wrapped is DisablePathNormalizing on the fasthttp client itself.
		wrapped bool
	}{
		{name: "normalized by default", want: normalized},
		{
			name:  "the client setting sends the path as written",
			setup: func(c *Client) *Client { return c.SetDisablePathNormalizing(true) },
			want:  asWritten,
		},
		{
			name:  "turning the client setting off again normalizes",
			setup: func(c *Client) *Client { return c.SetDisablePathNormalizing(true).SetDisablePathNormalizing(false) },
			want:  normalized,
		},
		{name: "a fasthttp client created not to normalize sends the path as written", wrapped: true, want: asWritten},
		// A request cannot change a client that other requests share, so on its
		// own the request setting does not reach the wire.
		{name: "the request setting alone leaves the wire normalized", cfg: Config{DisablePathNormalizing: true}, want: normalized},
	}

	for _, kind := range pathClientKinds {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					rec := startPathRecorder(t)
					c := kind.build(rec.dial, tc.wrapped)
					if tc.setup != nil {
						c = tc.setup(c)
					}

					resp, err := c.Get(target, tc.cfg)
					require.NoError(t, err)
					resp.Close()
					require.Equal(t, tc.want, rec.next(t))
				})
			}
		})
	}
}

// Test_Client_DisablePathNormalizing_PathParamGuard is the property the setting
// must never break: a path parameter value cannot change the path the template
// names. The check that holds a value inside its segment was lifted by the
// Fiber setting alone, which fasthttp ignored, so "x/../../admin" in
// "/api/users/:id" reached the server as "/api/admin".
func Test_Client_DisablePathNormalizing_PathParamGuard(t *testing.T) {
	t.Parallel()

	const (
		target    = "http://example.com/api/users/:id"
		traversal = "x/../../admin"
		oneSeg    = "/api/users/x%2F..%2F..%2Fadmin"
	)

	cases := []struct {
		setup func(c *Client) *Client
		name  string
		// want is the request target the server must see; empty means the
		// request has to fail with ErrPathParamInPath before anything is sent.
		want string
		cfg  Config
		// wrapped is DisablePathNormalizing on the fasthttp client itself.
		wrapped bool
		// bySetter marks a want that holds only for a client whose Fiber setting
		// alone lifts the check; any other kind has to reject the value.
		bySetter bool
	}{
		{name: "rejected by default"},
		{
			name: "rejected when only the request asks, because the path is still normalized",
			cfg:  Config{DisablePathNormalizing: true},
		},
		{
			name:     "kept in one segment when the client sends paths as written",
			setup:    func(c *Client) *Client { return c.SetDisablePathNormalizing(true) },
			want:     oneSeg,
			bySetter: true,
		},
		{
			name:  "rejected again once the client setting is turned off",
			setup: func(c *Client) *Client { return c.SetDisablePathNormalizing(true).SetDisablePathNormalizing(false) },
		},
		{
			name:    "kept in one segment when the request asks of a client that does not normalize",
			cfg:     Config{DisablePathNormalizing: true},
			wrapped: true,
			want:    oneSeg,
		},
		{name: "rejected when nobody asked, even if the client does not normalize", wrapped: true},
	}

	for _, kind := range pathClientKinds {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					rec := startPathRecorder(t)
					c := kind.build(rec.dial, tc.wrapped)
					if tc.setup != nil {
						c = tc.setup(c)
					}

					want := tc.want
					if tc.bySetter && !kind.setterLifts {
						want = ""
					}

					cfg := tc.cfg
					cfg.PathParam = map[string]string{"id": traversal}
					resp, err := c.Get(target, cfg)
					if want == "" {
						require.ErrorIs(t, err, ErrPathParamInPath)
						require.Empty(t, rec.target, "the request was sent")
						return
					}

					require.NoError(t, err)
					resp.Close()
					require.Equal(t, want, rec.next(t))
				})
			}
		})
	}
}

// Test_Client_DisablePathNormalizing_ValuesNoSettingLetsThrough covers the
// values that cannot stay a segment however the path is sent. With the check
// lifted a "/" in a value travels as "%2F", but ".." is a dot segment that the
// server resolves, so "/files/pub/:name/meta" with ".." went out as
// "/files/pub/../meta" and reached "/files/meta". An empty value gave
// "/files/pub//meta", which a server that merges slashes reads as
// "/files/pub/meta".
func Test_Client_DisablePathNormalizing_ValuesNoSettingLetsThrough(t *testing.T) {
	t.Parallel()

	const target = "http://example.com/files/pub/:name/meta"

	cases := []struct {
		name string
		val  string
		// want is the request target the server must see; empty means the
		// request has to fail with ErrPathParamInPath before anything is sent.
		want string
	}{
		{name: "dot", val: "."},
		{name: "dot-dot", val: ".."},
		{name: "empty", val: ""},
		// The two below show the check is lifted for these clients: only the
		// values above are held back.
		{name: "separator", val: "a/b", want: "/files/pub/a%2Fb/meta"},
		{name: "three dots", val: "...", want: "/files/pub/.../meta"},
	}

	for _, kind := range pathClientKinds {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					rec := startPathRecorder(t)
					// The fasthttp client is created not to normalize, which
					// lifts the check for every kind.
					c := kind.build(rec.dial, true).SetDisablePathNormalizing(true)
					require.True(t, c.writesPathAsBuilt())

					resp, err := c.Get(target, Config{PathParam: map[string]string{"name": tc.val}})
					if tc.want == "" {
						require.ErrorIs(t, err, ErrPathParamInPath)
						require.Empty(t, rec.target, "the request was sent")
						return
					}

					require.NoError(t, err)
					resp.Close()
					require.Equal(t, tc.want, rec.next(t))
				})
			}
		})
	}
}

// Test_Client_DisablePathNormalizing_ClientHandedOver covers a fasthttp.Client
// that someone else may have used. It gives each host the setting it had when it
// first served that host, so a wrapper made later cannot tell, from the field
// alone, whether turning normalizing off reaches the host. Before the wrapper
// kept the check on for such a client, "x/../../admin" in "/files/pub/:name"
// reached the server as "/files/admin" through a client that had served the
// host already.
func Test_Client_DisablePathNormalizing_ClientHandedOver(t *testing.T) {
	t.Parallel()

	const target = "http://example.com/files/pub/:name"

	get := func(c *Client, name string) (*Response, error) {
		return c.Get(target, Config{PathParam: map[string]string{"name": name}})
	}

	t.Run("a client that served the host already keeps the check on", func(t *testing.T) {
		t.Parallel()

		rec := startPathRecorder(t)
		fh := &fasthttp.Client{Dial: rec.dial}

		warm, err := NewWithClient(fh).Get("http://example.com/warm")
		require.NoError(t, err)
		warm.Close()
		require.Equal(t, "/warm", rec.next(t))

		c := NewWithClient(fh).SetDisablePathNormalizing(true)
		require.True(t, fh.DisablePathNormalizing, "the field is set for hosts not served yet")
		require.False(t, c.writesPathAsBuilt(), "nothing says the host client keeps up")

		_, err = get(c, "x/../../admin")
		require.ErrorIs(t, err, ErrPathParamInPath)
		require.Empty(t, rec.target, "the request was sent")
	})

	t.Run("a client not used yet cannot be told from one that was", func(t *testing.T) {
		t.Parallel()

		rec := startPathRecorder(t)
		c := NewWithClient(&fasthttp.Client{Dial: rec.dial}).SetDisablePathNormalizing(true)
		require.False(t, c.writesPathAsBuilt())

		// The path itself goes out as written, because the host is new.
		resp, err := c.Get("http://example.com/a//b/./c%2Fd")
		require.NoError(t, err)
		resp.Close()
		require.Equal(t, "/a//b/./c%2Fd", rec.next(t))

		_, err = get(c, "a/b")
		require.ErrorIs(t, err, ErrPathParamInPath)
	})

	t.Run("a client created with the setting on lifts the check, used or not", func(t *testing.T) {
		t.Parallel()

		rec := startPathRecorder(t)
		fh := &fasthttp.Client{Dial: rec.dial, DisablePathNormalizing: true}

		warm, err := NewWithClient(fh).Get("http://example.com/warm")
		require.NoError(t, err)
		warm.Close()
		require.Equal(t, "/warm", rec.next(t))

		c := NewWithClient(fh).SetDisablePathNormalizing(true)
		require.True(t, c.writesPathAsBuilt())

		resp, err := get(c, "a/b")
		require.NoError(t, err)
		resp.Close()
		require.Equal(t, "/files/pub/a%2Fb", rec.next(t))
	})

	t.Run("turning the setting off and on again keeps the check on", func(t *testing.T) {
		t.Parallel()

		rec := startPathRecorder(t)
		c := NewWithClient(&fasthttp.Client{Dial: rec.dial, DisablePathNormalizing: true})
		require.True(t, c.writesPathAsBuilt())

		c.SetDisablePathNormalizing(false)
		resp, err := c.Get("http://example.com/first")
		require.NoError(t, err)
		resp.Close()
		require.Equal(t, "/first", rec.next(t))

		c.SetDisablePathNormalizing(true)
		require.False(t, c.writesPathAsBuilt(), "the host client made for /first normalizes")
	})

	t.Run("a client from New is its own", func(t *testing.T) {
		t.Parallel()

		rec := startPathRecorder(t)
		c := New().SetDial(rec.dial).SetDisablePathNormalizing(true)
		require.True(t, c.writesPathAsBuilt())

		resp, err := get(c, "a/b")
		require.NoError(t, err)
		resp.Close()
		require.Equal(t, "/files/pub/a%2Fb", rec.next(t))
	})
}

// Test_Client_DisablePathNormalizing_AfterFirstRequest covers a client that has
// already sent a request while normalizing. fasthttp.Client hands each host the
// setting it has when it first creates that host's client and never updates it,
// so turning normalizing off afterwards does not reach the host already served.
// The check that keeps a path parameter in its segment has to stay, or "x/../../admin"
// would change the path for that host.
func Test_Client_DisablePathNormalizing_AfterFirstRequest(t *testing.T) {
	t.Parallel()

	rec := startPathRecorder(t)
	c := New().SetDial(rec.dial)

	resp, err := c.Get("http://example.com/first")
	require.NoError(t, err)
	resp.Close()
	require.Equal(t, "/first", rec.next(t))

	c.SetDisablePathNormalizing(true)
	require.False(t, c.writesPathAsBuilt(), "the host client already created still normalizes")

	resp, err = c.Get("http://example.com/a//b")
	require.NoError(t, err)
	resp.Close()
	require.Equal(t, "/a/b", rec.next(t), "the host kept the setting it was created with")

	_, err = c.Get("http://example.com/api/:id", Config{PathParam: map[string]string{"id": "x/../../admin"}})
	require.ErrorIs(t, err, ErrPathParamInPath)
	require.Empty(t, rec.target, "the request was sent")
}

// FuzzPathParamTarget asserts the property that matters at the other end of
// the wire: whatever the value, the request either fails or targets exactly the
// path the template names. The string-level fuzz below cannot see this — the
// escaping is undone again by fasthttp's path normalizing, which is how
// "../../admin" used to reach "/admin/end".
func FuzzPathParamTarget(f *testing.F) {
	for _, seed := range []string{"", "..", ".", "a/b", "../../admin", "%2e%2e", "..%2f", `a\b`, "a;b", "..;", "..;jsessionid=x", ".;", ";x", "\r\n", "//", "....//", ".%2e", "%252e%252e", "a?b", "a#b", "\u00fc"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		checkPathParamTarget(t, New(), val)
	})
}

// FuzzPathParamTargetAsWritten is the same property for a client that sends the
// path as built. An escape holds a "/" in its segment there, but not a dot
// segment, which the server resolves however the client writes it.
func FuzzPathParamTargetAsWritten(f *testing.F) {
	for _, seed := range []string{"", "..", ".", "...", "a/b", "../../admin", "%2e%2e", "..%2f", `a\b`, "a;b", "..;", "..;jsessionid=x", ".;", ";x", "\r\n", "//", "....//", ".%2e", "%252e%252e", "a?b", "a#b", "ü"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		checkPathParamTarget(t, New().SetDisablePathNormalizing(true), val)
	})
}

// checkPathParamTarget builds the URL of "/api/v1/:id/end" with val for id and
// requires that it either fails or still targets exactly that path, with val as
// one non-empty segment that is no dot segment, also once a servlet container
// has stripped its ";" parameters. The path is read from the request target,
// which is what goes out on the wire: the normalized path, or the original one
// when the client sends paths as written.
func checkPathParamTarget(t *testing.T, client *Client, val string) {
	t.Helper()

	req := AcquireRequest().
		SetClient(client).
		SetURL("http://example.com/api/v1/:id/end").
		SetPathParam("id", val)
	defer ReleaseRequest(req)

	if err := parserRequestURL(client, req); err != nil {
		return
	}

	uri := req.RawRequest.URI()
	path, _, _ := strings.Cut(string(uri.RequestURI()), "?")
	require.True(t, strings.HasPrefix(path, "/"), "value left the template path: %q", path)

	segments := strings.Split(path[1:], "/")
	require.Len(t, segments, 4, "value changed the segment count: %q", path)
	require.Equal(t, "api", segments[0], "value left the template path: %q", path)
	require.Equal(t, "v1", segments[1], "value left the template path: %q", path)
	require.Equal(t, "end", segments[3], "value left the template path: %q", path)
	require.NotEmpty(t, segments[2], "value dropped its segment: %q", path)
	require.NotContains(t, []string{".", ".."}, segments[2], "value made a dot segment: %q", path)
	name, _, _ := strings.Cut(segments[2], ";")
	require.NotEmpty(t, name, "value drops its segment once its path parameters are stripped: %q", path)
	require.NotContains(t, []string{".", ".."}, name, "value is a dot segment once its path parameters are stripped: %q", path)
	require.Equal(t, "example.com", string(uri.Host()), "value changed the host")
}

// FuzzPathParamHost is the same property for the authority: a value either
// fails or stays one label of the domain the template names. This is the one
// that has to hold, because getting it wrong sends the request elsewhere.
func FuzzPathParamHost(f *testing.F) {
	for _, seed := range []string{"", "a", "a@b", "a:1", "[::1]", "a.b", "a%2eb", "..", "a/b", "\u00fc", "a$b", "-", "0", "a\rb", "σ", "Σ", "\xc0\xaf", "\xff", "\xe2\x82"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		client := New()
		req := AcquireRequest().
			SetClient(client).
			SetURL("http://:tenant.example.com/api").
			SetPathParam("tenant", val)
		defer ReleaseRequest(req)

		if err := parserRequestURL(client, req); err != nil {
			return
		}

		uri := req.RawRequest.URI()
		// Nothing may transform an accepted value: it is one label of the
		// domain the template names, byte for byte, save for the ASCII
		// lowercasing fasthttp applies to every host. Not EqualFold, which
		// reads "σ" and "ς" as equal and would accept a mangled label.
		want := utilsstrings.ToLower(val + ".example.com")
		require.Equal(t, want, string(uri.Host()), "value did not land verbatim")
		require.Equal(t, "/api", string(uri.Path()), "value ate the path")
	})
}

// FuzzSubstitutePathParams asserts the property the old ReplaceAll could not
// hold: whatever the value contains, it stays one path segment and survives a
// decode unchanged.
func FuzzSubstitutePathParams(f *testing.F) {
	for _, seed := range []string{"", "5", "a/b", "a?b", "a#b", "100%", ":name", "a b", "ünïcode", "%2F", "//", "?#/", ".", "..", "..."} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		got, err := substitutePathParams("/a/:p/b", true, PathParam{"p": val})
		if cannotFillSegment(val) {
			require.ErrorIs(t, err, ErrPathParamInPath)
			return
		}
		require.NoError(t, err)

		parts := strings.Split(got, "/")
		require.Len(t, parts, 4, "value changed the segment count: %q", got)
		require.Empty(t, parts[0])
		require.Equal(t, "a", parts[1])
		require.Equal(t, "b", parts[3])
		require.NotContains(t, parts[2], "?", "value opened a query: %q", got)
		require.NotContains(t, parts[2], "#", "value opened a fragment: %q", got)

		decoded, err := url.PathUnescape(parts[2])
		require.NoError(t, err)
		require.Equal(t, val, decoded, "value did not survive a decode")
	})
}
