package client

import (
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Test_SubstitutePathParams covers the placeholder grammar directly: what counts
// as a placeholder, where one ends, and what happens to a value on the way in.
func Test_SubstitutePathParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		uri    string
		params PathParam
		want   string
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
			name:   "a value cannot add a segment",
			uri:    "http://example.com/api/:id",
			params: PathParam{"id": "a/b"},
			want:   "http://example.com/api/a%2Fb",
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
			name:   "an empty value collapses the placeholder",
			uri:    "http://example.com/api/:id/x",
			params: PathParam{"id": ""},
			want:   "http://example.com/api//x",
		},
		{
			name:   "no placeholder, no work",
			uri:    "http://example.com/api/users",
			params: PathParam{"id": "5"},
			want:   "http://example.com/api/users",
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
			require.Equal(t, tc.want, substitutePathParams(tc.uri, tc.params))
		})
	}
}

// Test_SubstitutePathParams_SourceOrder pins that the first source wins, which is
// what keeps request parameters overriding the client's.
func Test_SubstitutePathParams_SourceOrder(t *testing.T) {
	t.Parallel()

	got := substitutePathParams(
		"http://example.com/api/:id/:name",
		PathParam{"id": "request"},
		PathParam{"id": "client", "name": "client"},
	)
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

// Test_PathParam_RoundTrip_SeparatorInValue records what a "/" inside a value
// does: the escape is written, then fasthttp's path normalizing decodes it, so
// the value reaches the server as two segments and the route no longer matches.
func Test_PathParam_RoundTrip_SeparatorInValue(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{UnescapePath: true})
	app.Get("/api/:id/end", func(c fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})

	ch := make(chan struct{})
	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
		close(ch)
	}()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("timeout when waiting for server close")
		}
	})

	client := New().SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })

	resp, err := AcquireRequest().
		SetClient(client).
		SetPathParam("id", "a/b").
		Get("http://example.com/api/:id/end")
	require.NoError(t, err)
	defer resp.Close()

	require.Equal(t, fiber.StatusNotFound, resp.StatusCode())
}

// FuzzSubstitutePathParams asserts the property the old ReplaceAll could not
// hold: whatever the value contains, it stays one path segment and survives a
// decode unchanged.
func FuzzSubstitutePathParams(f *testing.F) {
	for _, seed := range []string{"", "5", "a/b", "a?b", "a#b", "100%", ":name", "a b", "ünïcode", "%2F", "//", "?#/"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, val string) {
		got := substitutePathParams("/a/:p/b", PathParam{"p": val})

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
