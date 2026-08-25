package client

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:    "an empty value collapses the placeholder when normalizing is off",
			uri:     "http://example.com/api/:id/x",
			params:  PathParam{"id": ""},
			want:    "http://example.com/api//x",
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

// FuzzPathParamTarget asserts the property that matters at the other end of
// the wire: whatever the value, the request either fails or targets exactly the
// path the template names. The string-level fuzz below cannot see this — the
// escaping is undone again by fasthttp's path normalizing, which is how
// "../../admin" used to reach "/admin/end".
func FuzzPathParamTarget(f *testing.F) {
	for _, seed := range []string{"", "..", ".", "a/b", "../../admin", "%2e%2e", "..%2f", `a\b`, "a;b", "\r\n", "//", "....//", ".%2e", "%252e%252e", "a?b", "a#b", "\u00fc"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		client := New()
		req := AcquireRequest().
			SetClient(client).
			SetURL("http://example.com/api/v1/:id/end").
			SetPathParam("id", val)
		defer ReleaseRequest(req)

		if err := parserRequestURL(client, req); err != nil {
			return
		}

		uri := req.RawRequest.URI()
		path := string(uri.Path())
		require.True(t, strings.HasPrefix(path, "/api/v1/"), "value left the template path: %q", path)
		require.True(t, strings.HasSuffix(path, "/end"), "value left the template path: %q", path)
		require.Equal(t, 4, strings.Count(path, "/"), "value changed the segment count: %q", path)
		require.Equal(t, "example.com", string(uri.Host()), "value changed the host")
	})
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
	for _, seed := range []string{"", "5", "a/b", "a?b", "a#b", "100%", ":name", "a b", "ünïcode", "%2F", "//", "?#/"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		t.Parallel()

		got, err := substitutePathParams("/a/:p/b", true, PathParam{"p": val})
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
