package wiretarget

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// uriChanges are what a handler does to the URI of a request, by the name the rows of the tests below give them.
var uriChanges = map[string]func(req *fasthttp.Request){
	"read":               func(req *fasthttp.Request) { req.URI().QueryArgs().Peek("x") },
	"add":                func(req *fasthttp.Request) { req.URI().QueryArgs().Add("k", "v") },
	"set":                func(req *fasthttp.Request) { req.URI().QueryArgs().Set("x", "9") },
	"reset":              func(req *fasthttp.Request) { req.URI().QueryArgs().Reset() },
	"set query string":   func(req *fasthttp.Request) { req.URI().SetQueryString("z=1") },
	"clear query string": func(req *fasthttp.Request) { req.URI().SetQueryString("") },
	"keep the path":      func(req *fasthttp.Request) { req.URI().DisablePathNormalizing = true },
	"set a path with ?":  func(req *fasthttp.Request) { req.URI().SetPath("/x?b") },
	"keep and set a path with ?": func(req *fasthttp.Request) {
		req.URI().DisablePathNormalizing = true
		req.URI().SetPath("/x?b")
	},
}

// requestFor returns a request whose line is line, with the URI change of that name made to it, if there is one.
func requestFor(line, change string) *fasthttp.Request {
	req := &fasthttp.Request{}
	req.SetRequestURI(line)
	if apply := uriChanges[change]; apply != nil {
		apply(req)
	}
	return req
}

func Test_plainPath(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"/": true, "/a": true, "/a/": true, "/a/b-c_d.e~f": true, "/UPPER/lower09": true, "/!$&'()*+,=:@": true,
		"": false, "a": false, "*": false, "example.com:443": false,
		"//": false, "//a": false, "/a//b": false, "/a//": false,
		"/a;b": false, "/;": false, "/a%2Fb": false, "/%41": false, "/%": false, "/%zz": false, `/a\b`: false,
		"/a b": false, "/a|b": false, "/café": false, "/a\x00": false, "/a?b": false, "/a#b": false, "/a\"b": false,
	} {
		require.Equal(t, want, plainPath(path), path)
	}
}

// Test_plainPath_LetsEveryCheckThrough pins what lineIsTarget leaves out the scans for, over every path of up to five
// bytes of the ones the checks tell apart: a path that is plain needs no escape, and SegmentsAsRouted and
// SingleSpelling let it through.
func Test_plainPath_LetsEveryCheckThrough(t *testing.T) {
	t.Parallel()
	alphabet := "/.;%\\?a2F ~,\xc3"
	plain := 0
	var walk func(path string)
	walk = func(path string) {
		if plainPath(path) {
			plain++
			require.False(t, needsEscape(path), path)
			require.True(t, SegmentsAsRouted(path, AllowParameters), path)
			require.True(t, SingleSpelling(path), path)
			require.Equal(t, path, pathOf(path), path)
		}
		if len(path) == 6 {
			return
		}
		for i := range len(alphabet) {
			walk(path + alphabet[i:i+1])
		}
	}
	walk("/")
	require.Greater(t, plain, 1000)
}

func Test_lineIsTarget(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, line, path, change string
		want                     bool
	}{
		{name: "path", line: "/a/b-c_d.e~f", path: "/a/b-c_d.e~f", want: true},
		{name: "root", line: "/", path: "/", want: true},
		{name: "trailing slash", line: "/a/", path: "/a/", want: true},
		{name: "query", line: "/a?x=1&y=%20;z", path: "/a", want: true},
		{name: "query read", line: "/a?x=a%20b&c=;d", path: "/a", change: "read", want: true},
		{name: "query, path kept", line: "/a?x=1", path: "/a", change: "keep the path", want: true},
		{name: "arguments set", line: "/a?x=1", path: "/a", change: "set"},
		{name: "arguments removed", line: "/a?x=1", path: "/a", change: "reset"},
		{name: "arguments added", line: "/a", path: "/a", change: "add"},
		{name: "query string set", line: "/a", path: "/a", change: "set query string"},
		{name: "empty query", line: "/a?", path: "/a"},
		{name: "fragment after a query", line: "/a?x=1#f", path: "/a"},
		{name: "fragment", line: "/a#f", path: "/a"},
		{name: "path rewritten", line: "/a", path: "/b"},
		{name: "path longer than the line", line: "/a", path: "/a/b"},
		{name: "path ends inside the line", line: "/a/b", path: "/a"},
		{name: "path ends inside the line, query", line: "/a/b?x=1", path: "/a"},
		{name: "unrooted", line: "a", path: "a"},
		{name: "asterisk", line: "*", path: "*"},
		{name: "escape", line: "/a%20b", path: "/a%20b"},
		{name: "escaped slash", line: "/a%2Fb", path: "/a%2Fb"},
		{name: "path parameter", line: "/a;b", path: "/a;b"},
		{name: "empty segment", line: "/a//b", path: "/a//b"},
		{name: "backslash", line: `/a\b`, path: `/a\b`},
		{name: "byte that needs an escape", line: "/a b", path: "/a b"},
		{name: "byte of UTF-8", line: "/café", path: "/café"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, lineIsTarget(requestFor(tc.line, tc.change), tc.path, tc.line))
		})
	}
}

// fakeCtx is what serveWith reads of a Ctx, for a request and the path the router matched.
type fakeCtx struct {
	req  *fasthttp.Request
	path string
}

func (fakeCtx) App() struct{}                    { return struct{}{} }
func (c fakeCtx) OriginalURL() string            { return string(c.req.Header.RequestURI()) }
func (c fakeCtx) Path(...string) string          { return c.path }
func (c fakeCtx) Request() *fasthttp.Request     { return c.req }
func (fakeCtx) Response() *fasthttp.Response     { return nil }
func (fakeCtx) RequestCtx() *fasthttp.RequestCtx { return nil }
func (fakeCtx) IsAbandoned() bool                { return false }
func (fakeCtx) Locals(any, ...any) any           { return nil }

// Test_lineIsTarget_MeetsTheChecks pins what serveWith relies on: when lineIsTarget says that the line is the routed
// target, routedFor makes that very target of the path and the checks let it through, for every path of up to three
// bytes behind a slash, with and without a query, and whatever a handler did to the URI.
func Test_lineIsTarget_MeetsTheChecks(t *testing.T) {
	t.Parallel()
	const alphabet = "/.;%\\?a2F ~,"
	var paths []string
	var walk func(path string)
	walk = func(path string) {
		paths = append(paths, path)
		if len(path) == 4 {
			return
		}
		for i := range len(alphabet) {
			walk(path + alphabet[i:i+1])
		}
	}
	walk("/")

	changes := []string{""}
	for change := range uriChanges {
		changes = append(changes, change)
	}
	accepted := 0
	for _, path := range paths {
		for _, rest := range []string{"", "?", "?x=1", "?x=a%20b;c", "?%zz", "#f", "?x=1#f"} {
			line := path + rest
			for _, change := range changes {
				req := requestFor(line, change)
				if !lineIsTarget(req, path, line) {
					continue
				}
				accepted++
				require.Equal(t, line, routedFor[struct{}](fakeCtx{req: req, path: path}, path, line), "%q %s", line, change)
				require.True(t, SegmentsAsRouted(line, AllowParameters), "%q %s", line, change)
				require.True(t, SingleSpelling(path), "%q %s", line, change)
			}
		}
	}
	require.Greater(t, accepted, 1000)
}

// queryOfScanned is queryOf as it was before it left out writing the request URI for a URI without a query: it reads
// the query of the request URI always.
func queryOfScanned(req *fasthttp.Request) []byte {
	uri := req.URI()
	sent := uri.QueryString()
	var current []byte
	requestURI := uri.RequestURI()
	if _, query, ok := bytes.Cut(requestURI, []byte{'?'}); ok {
		current = query
	}
	if bytes.Equal(current, sent) {
		return sent
	}
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)
	args.ParseBytes(sent)
	if bytes.Equal(args.QueryString(), current) {
		return sent
	}
	return current
}

// Test_queryOf pins that leaving out the request URI for a URI that has no query and no arguments added to it finds
// what writing it does, whatever was done to the URI.
func Test_queryOf(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, line, change, want string }{
		{name: "no query", line: "/a"},
		{name: "query", line: "/a?x=1&y=2", want: "x=1&y=2"},
		{name: "empty query", line: "/a?"},
		{name: "query read", line: "/a?x=a%20b;c", change: "read", want: "x=a%20b;c"},
		{name: "arguments added", line: "/a", change: "add", want: "k=v"},
		{name: "arguments added to a query", line: "/a?x=1", change: "add", want: "x=1&k=v"},
		{name: "arguments set", line: "/a?x=1", change: "set", want: "x=9"},
		{name: "arguments removed", line: "/a?x=1", change: "reset"},
		{name: "arguments emptied", line: "/a", change: "reset"},
		{name: "query string set", line: "/a", change: "set query string", want: "z=1"},
		{name: "query string cleared", line: "/a?x=1", change: "clear query string"},
		{name: "path kept", line: "/a", change: "keep the path"},
		{name: "path kept, query", line: "/a?x=1", change: "keep the path", want: "x=1"},
		{name: "path normalized, ? in it", line: "/a", change: "set a path with ?"},
		// the path is spelled as set then, "?" included, which the request URI reads as the beginning of a query
		{name: "path kept, ? in it", line: "/a", change: "keep and set a path with ?", want: "b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, string(queryOfScanned(requestFor(tc.line, tc.change))))
			require.Equal(t, tc.want, string(queryOf(requestFor(tc.line, tc.change))))
		})
	}
}
