// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📃 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	pathpkg "path"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var routesFixture routeJSON

// sweepTestConfig is for tests that send hundreds of requests through app.Test: each
// one gets a second by default, which a loaded runner misses once in a few thousand.
var sweepTestConfig = TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true}

func init() {
	dat, err := os.ReadFile("./.github/testdata/testRoutes.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(dat, &routesFixture); err != nil {
		panic(err)
	}
}

// routerTestRequestCtx returns a fasthttp request context holding a request for
// method and uri, ready to be handed to app.Handler() or app.AcquireCtx.
func routerTestRequestCtx(method, uri string) *fasthttp.RequestCtx {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(method)
	fctx.Request.SetRequestURI(uri)
	return fctx
}

// routerTestExpect is [apptest.Expect] for requests whose body is not under
// test. It still drains and closes the response body like a client would, so a
// failing read or close fails the test, and returns the response for header checks.
func routerTestExpect[T apptest.Target](tb testing.TB, app *App, target T, status int) *http.Response {
	tb.Helper()

	resp := apptest.Expect(tb, app, target, status)
	apptest.BodyBytes(tb, resp)
	return resp
}

// routerReq is a request and the answer it must get. An empty method is GET, an empty host is the one httptest
// gives, a zero status is 200, and an empty body is not checked. A named header must have value, which is empty for
// a header that must be absent.
type routerReq struct {
	method string
	host   string
	path   string
	body   string
	header string
	value  string
	allow  []string // methods the Allow header must list
	status int
}

// routerTestExpectAll sends each request through app, in order, and checks the answer.
func routerTestExpectAll(tb testing.TB, app *App, reqs ...routerReq) {
	tb.Helper()

	for i := range reqs {
		r := &reqs[i]
		var opts []apptest.Option
		if r.host != "" {
			opts = append(opts, apptest.WithHost(r.host))
		}
		method := cmp.Or(r.method, MethodGet)
		resp := routerTestExpect(tb, app, apptest.Req(method, r.path, opts...), cmp.Or(r.status, StatusOK))
		where := method + " " + r.host + r.path
		if r.body != "" {
			require.Equal(tb, r.body, apptest.Body(tb, resp), where)
		}
		if r.header != "" {
			require.Equal(tb, r.value, resp.Header.Get(r.header), where)
		}
		for _, method := range r.allow {
			require.Contains(tb, resp.Header.Get(HeaderAllow), method, where)
		}
	}
}

// routerTestExpectPath is [routerTestExpectAll] for one GET and a table row that names the body it expects only
// sometimes.
func routerTestExpectPath(tb testing.TB, app *App, path string, status int, want string) {
	tb.Helper()

	routerTestExpectAll(tb, app, routerReq{path: path, status: status, body: want})
}

func Test_Route_Handler_Order(t *testing.T) {
	t.Parallel()
	app := New()
	var order []int
	handler := func(n int) Handler {
		return func(c Ctx) error {
			order = append(order, n)
			return c.Next()
		}
	}
	app.Get("/test", handler(1), handler(2), handler(3), func(c Ctx) error {
		order = append(order, 4)
		return c.SendStatus(200)
	})
	apptest.Expect(t, app, "/test", 200)
	require.Equal(t, []int{1, 2, 3, 4}, order, "Handler order")
}

func Test_hasFlashCookieExactMatch(t *testing.T) {
	t.Parallel()
	buildRequestWithCookie := func(t *testing.T, cookie string) *fasthttp.Request {
		t.Helper()

		rawRequest := strings.NewReader("GET / HTTP/1.1\r\nHost: localhost\r\nCookie: " + cookie + "\r\n\r\n")
		req := new(fasthttp.Request)
		require.NoError(t, req.Read(bufio.NewReader(rawRequest)))
		return req
	}

	req := buildRequestWithCookie(t, FlashCookieName+"X=not-the-flash-cookie")
	require.False(t, hasFlashCookie(&req.Header))

	req = buildRequestWithCookie(t, FlashCookieName+"=valid")
	require.True(t, hasFlashCookie(&req.Header))

	// A programmatic request carries no raw headers, so the cookie lookup decides.
	var syntheticReq fasthttp.Request
	syntheticReq.Header.Set(HeaderCookie, FlashCookieName+"=valid")
	require.True(t, hasFlashCookie(&syntheticReq.Header))

	var syntheticOther fasthttp.Request
	syntheticOther.Header.Set(HeaderCookie, FlashCookieName+"X=not-the-flash-cookie")
	require.False(t, hasFlashCookie(&syntheticOther.Header))
}

func Test_Route_MixedFiberAndHTTPHandlers(t *testing.T) {
	t.Parallel()
	app := New()

	var order []string

	fiberBefore := func(c Ctx) error {
		order = append(order, "fiber-before")
		c.Set("X-Fiber", "1")
		return c.Next()
	}

	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "http-final")
		w.Header().Set("X-HTTP", "true")
		_, err := w.Write([]byte("http"))
		assert.NoError(t, err)
	})

	fiberAfter := func(c Ctx) error {
		order = append(order, "fiber-after")
		return c.SendString("fiber")
	}

	app.Get("/mixed", fiberBefore, httpHandler, fiberAfter)

	resp := apptest.ExpectBody(t, app, "/mixed", 200, "http")
	require.Equal(t, "true", resp.Header.Get("X-HTTP"))
	require.Equal(t, "1", resp.Header.Get("X-Fiber"))

	require.Equal(t, []string{"fiber-before", "http-final"}, order)
}

func Test_Route_Group_WithHTTPHandlers(t *testing.T) {
	t.Parallel()
	app := New()

	var order []string

	app.Use("/api", func(c Ctx) error {
		order = append(order, "app-use")
		return c.Next()
	})

	grp := app.Group("/api", func(c Ctx) error {
		order = append(order, "group-middleware")
		return c.Next()
	})

	grp.Get("/users", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "http-handler")
		_, err := w.Write([]byte("users"))
		assert.NoError(t, err)
	}))

	apptest.ExpectBody(t, app, "/api/users", 200, "users")

	require.Equal(t, []string{"app-use", "group-middleware", "http-handler"}, order)
}

func Test_RouteChain_WithHTTPHandlers(t *testing.T) {
	t.Parallel()
	app := New()

	chain := app.RouteChain("/combo")
	chain.Get(func(c Ctx) error {
		c.Set("X-Chain", "fiber")
		return c.Next()
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("combo"))
		assert.NoError(t, err)
	}))

	chain.RouteChain("/nested").Get(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Nested", "true")
		_, err := w.Write([]byte("nested"))
		assert.NoError(t, err)
	}))

	resp := apptest.ExpectBody(t, app, "/combo", 200, "combo")
	require.Equal(t, "fiber", resp.Header.Get("X-Chain"))

	nestedResp := apptest.ExpectBody(t, app, "/combo/nested", 200, "nested")
	require.Equal(t, "true", nestedResp.Header.Get("X-Nested"))
}

// Test_Route_Match_Param checks that a route whose handler answers with its one
// parameter binds the text of the path, including a path spelled exactly like
// the pattern.
func Test_Route_Match_Param(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		pattern  string
		param    string
		requests []routerReq
		use      bool
	}{
		{
			name: "same length", pattern: "/:param", param: "param",
			requests: []routerReq{{path: "/:param", body: ":param"}, {path: "/test", body: "test"}},
		},
		{
			name: "star", pattern: "/*", param: "*",
			requests: []routerReq{{path: "/*", body: "*"}, {path: "/test", body: "test"}},
		},
		{
			name: "middleware", use: true, pattern: "/foo/*", param: "*",
			requests: []routerReq{{path: "/foo/*", body: "*"}, {path: "/foo/bar/fasel", body: "bar/fasel"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()

			handler := func(c Ctx) error { return c.SendString(c.Params(tc.param)) }
			if tc.use {
				app.Use(tc.pattern, handler)
			} else {
				app.Get(tc.pattern, handler)
			}

			routerTestExpectAll(t, app, tc.requests...)
		})
	}
}

// Test_Route_Match_SlashInParams locks in the matching quirks where a
// parameter value legitimately contains '/', so the slash-count quick-reject
// must not filter these routes out end-to-end.
func Test_Route_Match_SlashInParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/api/v1/:param-:param2", func(c Ctx) error {
		return c.SendString(c.Params("param") + "|" + c.Params("param2"))
	})
	app.Get("/test:sign:param", func(c Ctx) error { return c.SendString(c.Params("sign") + "|" + c.Params("param")) })
	app.Get("/date/:day/:month?/:year?", func(c Ctx) error {
		return c.SendString(c.Params("day") + "|" + c.Params("month") + "|" + c.Params("year"))
	})
	app.Use("/mw/:version", func(c Ctx) error { return c.SendString(c.Params("version")) })

	routerTestExpectAll(t, app, []routerReq{
		// param swallows '/' via the unguarded single-byte compare part
		{path: "/api/v1/enti/ty-x", body: "enti/ty|x"},
		// adjacent params consume one byte each, possibly '/'
		{path: "/test/x", body: "/|x"},
		// optional segments drop their leading slashes
		{path: "/date/1", body: "1||"},
		{path: "/date/1/2/3", body: "1|2|3"},
		// param middleware matches deeper paths
		{path: "/mw/v1/users/list", body: "v1"},
	}...)
}

// Test_App_HasParamRoutes verifies the slash-count gate: apps whose routes
// never consult the per-request slash count (static and star only) skip the
// counting, and rebuilding the tree after adding a parametric route re-enables
// it so param routes still match.
func Test_App_HasParamRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/static/route", testEmptyHandler)
	app.Get("/*", func(c Ctx) error { return c.SendString(c.Params("*")) })
	app.startupProcess()
	require.False(t, app.hasParamRoutes)

	// static and star routing work without the per-request count
	apptest.Expect(t, app, "/static/route", StatusOK)
	apptest.ExpectBody(t, app, "/anything/else", StatusOK, "anything/else")

	// the GET star route would shadow a GET param route, so use POST
	app.Post("/users/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })
	app.RebuildTree()
	require.True(t, app.hasParamRoutes)

	apptest.ExpectBody(t, app, apptest.Req(MethodPost, "/users/42"), StatusOK, "42")
}

// Test_Route_Match_Star calls Route.match directly on a star route; the routing
// of a star pattern through an app is the "star" case of Test_Route_Match_Param.
func Test_Route_Match_Star(t *testing.T) {
	t.Parallel()
	// without parameter
	route := Route{star: true, path: "/*", routeParser: routeParser{}}
	params := [maxParams]string{}
	match := route.match("", "", &params, 0)
	require.True(t, match)
	require.Equal(t, [maxParams]string{}, params)

	// with parameter
	match = route.match("/favicon.ico", "/favicon.ico", &params, 1)
	require.True(t, match)
	require.Equal(t, [maxParams]string{"favicon.ico"}, params)

	// without parameter again
	match = route.match("", "", &params, 0)
	require.True(t, match)
	require.Equal(t, [maxParams]string{}, params)
}

func Test_Route_Match_Root(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", func(c Ctx) error { return c.SendString("root") })

	apptest.ExpectBody(t, app, "/", 200, "root")
}

func Test_Route_Match_Parser(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/foo/:ParamName", func(c Ctx) error { return c.SendString(c.Params("ParamName")) })
	app.Get("/Foobar/*", func(c Ctx) error { return c.SendString(c.Params("*")) })
	apptest.ExpectBody(t, app, "/foo/bar", 200, "bar")

	// with star
	apptest.ExpectBody(t, app, "/Foobar/test", 200, "test")
}

func TestAutoRegisterHeadRoutes(t *testing.T) {
	t.Parallel()
	// expectHead sends HEAD for path, asserts the status and that no body comes
	// back, and returns the response for header checks.
	expectHead := func(t *testing.T, app *App, path string, status int) *http.Response {
		t.Helper()

		resp := apptest.Expect(t, app, apptest.Req(MethodHead, path), status)
		require.Empty(t, apptest.BodyBytes(t, resp))
		return resp
	}

	t.Run("auto registers head for get", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/", func(c Ctx) error {
			c.Set("X-Test", "auto")
			return c.SendString("Hello")
		})

		respHead := expectHead(t, app, "/", StatusOK)
		require.Equal(t, int64(len("Hello")), respHead.ContentLength)
		require.Equal(t, "auto", respHead.Header.Get("X-Test"))

		respGet := apptest.ExpectBody(t, app, "/", StatusOK, "Hello")
		require.Equal(t, int64(len("Hello")), respGet.ContentLength)
	})

	t.Run("disable auto register config", func(t *testing.T) {
		t.Parallel()
		app := New(Config{DisableHeadAutoRegister: true})
		app.Get("/", func(c Ctx) error { return c.SendString("Hello") })

		routerTestExpect(t, app, apptest.Req(MethodHead, "/"), StatusMethodNotAllowed)
	})

	t.Run("explicit head overrides auto head route", func(t *testing.T) {
		t.Parallel()
		app := New()
		var getCalls int
		app.Get("/override", func(c Ctx) error {
			getCalls++
			return c.SendString("GET")
		})

		routerTestExpect(t, app, apptest.Req(MethodHead, "/override"), StatusOK)
		require.Equal(t, 1, getCalls)

		var headCalls int
		app.Head("/override", func(c Ctx) error {
			headCalls++
			c.Set("X-Explicit", "true")
			return c.SendStatus(StatusNoContent)
		})

		respHead := expectHead(t, app, "/override", StatusNoContent)
		require.Equal(t, "true", respHead.Header.Get("X-Explicit"))
		require.Equal(t, 1, getCalls)
		require.Equal(t, 1, headCalls)
	})

	t.Run("auto head for grouped routes", func(t *testing.T) {
		t.Parallel()
		app := New()
		group := app.Group("/api")
		group.Get("/users/:id", func(c Ctx) error {
			c.Set("X-User", c.Params("id"))
			return c.SendString("grouped")
		})

		respHead := expectHead(t, app, "/api/users/42", StatusOK)
		require.Equal(t, "42", respHead.Header.Get("X-User"))
	})

	t.Run("static handler auto head", func(t *testing.T) {
		t.Parallel()
		const file = "./.github/testdata/testRoutes.json"
		content, err := os.ReadFile(file)
		require.NoError(t, err)

		app := New()
		app.Get("/file", func(c Ctx) error { return c.SendFile(file) })

		respHead := expectHead(t, app, "/file", StatusOK)
		require.Equal(t, int64(len(content)), respHead.ContentLength)

		data := apptest.BodyBytes(t, apptest.Do(t, app, "/file"))
		require.Equal(t, content, data)
	})

	t.Run("head without matching route returns 404", func(t *testing.T) {
		t.Parallel()
		app := New()

		routerTestExpect(t, app, apptest.Req(MethodHead, "/missing"), StatusNotFound)
	})

	t.Run("late explicit get keeps explicit head", func(t *testing.T) {
		t.Parallel()
		app := New()
		var headCalls int
		app.Head("/late", func(c Ctx) error {
			headCalls++
			c.Set("X-Late", "head")
			return c.SendStatus(StatusAccepted)
		})

		var getCalls int
		app.Get("/late", func(c Ctx) error {
			getCalls++
			return c.SendString("ok")
		})

		respHead := routerTestExpect(t, app, apptest.Req(MethodHead, "/late"), StatusAccepted)
		require.Equal(t, "head", respHead.Header.Get("X-Late"))
		require.Equal(t, 1, headCalls)
		require.Equal(t, 0, getCalls)

		routerTestExpect(t, app, "/late", StatusOK)
		require.Equal(t, 1, getCalls)
	})

	t.Run("route listing includes auto head", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/list", func(c Ctx) error { return c.SendString("list") })
		app.startupProcess()
		var methods []string
		for _, route := range app.GetRoutes() {
			if route.Path == "/list" {
				methods = append(methods, route.Method)
			}
		}
		require.Contains(t, methods, MethodGet)
		require.Contains(t, methods, MethodHead)
	})

	t.Run("head mirrors status without body", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/nocontent", func(c Ctx) error { return c.SendStatus(StatusNoContent) })

		expectHead(t, app, "/nocontent", StatusNoContent)

		routerTestExpect(t, app, "/nocontent", StatusNoContent)
	})
}

func Test_Route_Match_UnescapedPath(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true})

	app.Use("/créer", func(c Ctx) error { return c.SendString("test") })

	apptest.ExpectBody(t, app, "/cr%C3%A9er", StatusOK, "test")
	// without special chars
	apptest.Expect(t, app, "/créer", StatusOK)

	app.config.UnescapePath = false
	apptest.Expect(t, app, "/cr%C3%A9er", StatusNotFound)

	// an encoded slash splits the segment only with the flag on
	app.config.UnescapePath = true
	app.Use("/cr/éer", func(c Ctx) error { return c.SendString("split") })
	apptest.ExpectBody(t, app, "/cr%2F%C3%A9er", StatusOK, "split")

	app.config.UnescapePath = false
	apptest.Expect(t, app, "/cr%2F%C3%A9er", StatusNotFound)
}

// Test_Route_Match_NormalizedPath pins the RFC 3986 normalization the router
// applies before matching, so a guard on a prefix sees the spellings of a path
// under it that name the same resource, and only those.
func Test_Route_Match_NormalizedPath(t *testing.T) {
	t.Parallel()
	newApp := func(cfg Config) *App {
		app := New(cfg)
		app.Use("/static/private", func(c Ctx) error { return c.SendStatus(StatusForbidden) })
		app.Get("/static/*", func(c Ctx) error { return c.SendString(c.Path() + "|" + c.Params("*")) })
		app.Get("/users/:id", func(c Ctx) error { return c.SendString(c.Params("id")) })
		// a name outside the unreserved set is routed in its encoded spelling
		app.Get("/enc/a%7Bb%7D", func(c Ctx) error { return c.SendString(c.Path()) })
		return app
	}

	tests := []struct {
		name       string
		target     string
		wantBody   string
		wantStatus int
		unescape   bool
	}{
		{name: "plain", target: "/static/public.txt", wantStatus: StatusOK, wantBody: "/static/public.txt|public.txt"},
		{name: "guarded", target: "/static/private/secret.txt", wantStatus: StatusForbidden},
		{name: "encoded unreserved character", target: "/static/%70rivate/secret.txt", wantStatus: StatusForbidden},
		{name: "parent segment", target: "/static/x/../private/secret.txt", wantStatus: StatusForbidden},
		{name: "current segment", target: "/static/./private/secret.txt", wantStatus: StatusForbidden},
		{name: "empty segment is kept", target: "/static//private/secret.txt", wantStatus: StatusOK, wantBody: "/static//private/secret.txt|/private/secret.txt"},
		{name: "encoded dot segment", target: "/static/x/%2E%2E/private/secret.txt", wantStatus: StatusForbidden},
		{name: "parent segment after an empty one", target: "/static/x//../private/secret.txt", wantStatus: StatusOK, wantBody: "/static/x/private/secret.txt|x/private/secret.txt"},
		{name: "parent segment above the prefix", target: "/static/../static/private/secret.txt", wantStatus: StatusForbidden},
		{name: "parent segment leaves the route", target: "/static/../other", wantStatus: StatusNotFound},
		{name: "trailing current segment", target: "/users/john/.", wantStatus: StatusOK, wantBody: "john"},
		{name: "trailing parent segment", target: "/static/private/x/..", wantStatus: StatusForbidden},
		{name: "encoded unreserved characters in params", target: "/users/%6Aohn%2Edoe%7E", wantStatus: StatusOK, wantBody: "john.doe~"},
		{name: "encoded space kept", target: "/static/a%20b.txt", wantStatus: StatusOK, wantBody: "/static/a%20b.txt|a%20b.txt"},
		{name: "encoded space decoded under UnescapePath", target: "/static/a%20b.txt", unescape: true, wantStatus: StatusOK, wantBody: "/static/a b.txt|a b.txt"},
		{name: "non-ascii kept with uppercase hex digits", target: "/users/%c3%a9", wantStatus: StatusOK, wantBody: "%C3%A9"},
		{name: "non-ascii decoded under UnescapePath", target: "/users/%c3%a9", unescape: true, wantStatus: StatusOK, wantBody: "é"},
		{name: "hex case does not split a route", target: "/enc/a%7bb%7d", wantStatus: StatusOK, wantBody: "/enc/a%7Bb%7D"},
		{name: "encoded slash stays in the segment", target: "/users/a%2fb", wantStatus: StatusOK, wantBody: "a%2Fb"},
		{name: "encoded slash splits the segment under UnescapePath", target: "/users/a%2Fb", unescape: true, wantStatus: StatusNotFound},
		{name: "reserved character kept", target: "/users/a%40b", wantStatus: StatusOK, wantBody: "a%40b"},
		{name: "reserved character decoded under UnescapePath", target: "/users/a%40b", unescape: true, wantStatus: StatusOK, wantBody: "a@b"},
		{name: "percent sign decoded once", target: "/static/%2570rivate/x", wantStatus: StatusOK, wantBody: "/static/%2570rivate/x|%2570rivate/x"},
		{name: "percent sign decoded once under UnescapePath", target: "/static/%2570rivate/x", unescape: true, wantStatus: StatusOK, wantBody: "/static/%70rivate/x|%70rivate/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newApp(Config{UnescapePath: tc.unescape})
			routerTestExpectPath(t, app, tc.target, tc.wantStatus, tc.wantBody)
		})
	}
}

// Test_Route_Match_AgreesWithNetHTTP sends the same requests to a net/http
// ServeMux and to Fiber and expects the same route, path and parameters.
// ServeMux unescapes each segment of the escaped path and redirects a path
// with dot or empty segments to its clean form, which the harness routes as
// the redirect would. Three spellings are left out because the two routers
// disagree on purpose: ServeMux keeps "a%2Fb" one segment where UnescapePath
// splits it, it reads "%2E%2E" as a literal name where Fiber decodes the
// unreserved dots first and removes the segment, and it redirects "//a" to
// "/a" where Fiber keeps the empty segment RFC 3986 leaves in place.
func Test_Route_Match_AgreesWithNetHTTP(t *testing.T) {
	t.Parallel()
	routes := []struct {
		mux, fiber string
		params     []string // wildcard names; "rest" reads Fiber's "*"
	}{
		{mux: "/private/secret.txt", fiber: "/private/secret.txt"},
		{mux: "/créer", fiber: "/créer"},
		{mux: "/tag@2x", fiber: "/tag@2x"},
		{mux: "/users/{id}", fiber: "/users/:id", params: []string{"id"}},
		{mux: "/files/{rest...}", fiber: "/files/*", params: []string{"rest"}},
	}

	newMux := func() *http.ServeMux {
		mux := http.NewServeMux()
		for i, r := range routes {
			mux.HandleFunc("GET "+r.mux, func(w http.ResponseWriter, req *http.Request) {
				values := make([]string, len(r.params))
				for j, name := range r.params {
					values[j] = req.PathValue(name)
				}
				fmt.Fprintf(w, "%d %s %q", i, req.URL.Path, values)
			})
		}
		return mux
	}
	newFiber := func(unescape bool) *App {
		app := New(Config{UnescapePath: unescape, CaseSensitive: true, StrictRouting: true})
		for i, r := range routes {
			app.Get(r.fiber, func(c Ctx) error {
				values := make([]string, len(r.params))
				for j, name := range r.params {
					if name == "rest" {
						name = "*"
					}
					values[j] = c.Params(name)
				}
				return c.SendString(fmt.Sprintf("%d %s %q", i, c.Path(), values))
			})
		}
		return app
	}

	// viaMux routes target as ServeMux does: a path that is not clean is
	// answered with a redirect to its clean form (net/http's cleanPath), so
	// the clean form is what gets routed.
	viaMux := func(mux *http.ServeMux, target string) (int, string) {
		req := httptest.NewRequest(MethodGet, target, http.NoBody)
		escaped := req.URL.EscapedPath()
		clean := pathpkg.Clean(escaped)
		if strings.HasSuffix(escaped, "/") && clean != "/" {
			clean += "/"
		}
		if clean != escaped {
			req = httptest.NewRequest(MethodGet, clean, http.NoBody)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	viaFiber := func(app *App, target string) (int, string) {
		resp := apptest.Do(t, app, target)
		body := apptest.Body(t, resp)
		return resp.StatusCode, body
	}

	requests := []struct {
		target string
		// only escapes of unreserved characters are decoded without
		// UnescapePath, so these only agree with the flag on
		needsUnescape bool
	}{
		{target: "/private/secret.txt"},
		{target: "/%70rivate/secret.txt"},
		{target: "/x/../private/secret.txt"},
		{target: "/./private/secret.txt"},
		{target: "/private/x/../secret.txt"},
		{target: "/cr%C3%A9er", needsUnescape: true},
		{target: "/cr%c3%a9er", needsUnescape: true},
		{target: "/créer", needsUnescape: true},
		{target: "/users/%C3%A9", needsUnescape: true},
		{target: "/users/a%20b", needsUnescape: true},
		{target: "/users/a%40b", needsUnescape: true},
		{target: "/tag%402x", needsUnescape: true},
		{target: "/users/100%2525", needsUnescape: true},
		{target: "/users/%2570rivate", needsUnescape: true},
		{target: "/files/a/b/c.txt"},
		{target: "/files/a/../c.txt"},
		{target: "/files/%61/b"},
		{target: "/files/../private/secret.txt"},
		{target: "/users/"},
		{target: "/nope"},
	}

	for _, unescape := range []bool{false, true} {
		app := newFiber(unescape)
		mux := newMux()
		for _, tc := range requests {
			if tc.needsUnescape && !unescape {
				continue
			}
			t.Run(fmt.Sprintf("unescape=%v/%s", unescape, tc.target), func(t *testing.T) {
				t.Parallel()
				wantStatus, wantBody := viaMux(mux, tc.target)
				gotStatus, gotBody := viaFiber(app, tc.target)
				require.Equal(t, wantStatus, gotStatus, "status for %s", tc.target)
				if wantStatus == StatusOK {
					require.Equal(t, wantBody, gotBody, "route, path and params for %s", tc.target)
				}
			})
		}
	}
}

func Test_Route_Match_WithEscapeChar(t *testing.T) {
	t.Parallel()
	app := New()
	// static route and escaped part
	app.Get("/v1/some/resource/name\\:customVerb", func(c Ctx) error { return c.SendString("static") })
	// group route
	group := app.Group("/v2/\\:firstVerb")
	group.Get("/\\:customVerb", func(c Ctx) error { return c.SendString("group") })
	// route with resource param and escaped part
	app.Get("/v3/:resource/name\\:customVerb", func(c Ctx) error { return c.SendString(c.Params("resource")) })
	apptest.ExpectBody(t, app, "/v1/some/resource/name:customVerb", StatusOK, "static")
	apptest.ExpectBody(t, app, "/v2/:firstVerb/:customVerb", StatusOK, "group")
	apptest.ExpectBody(t, app, "/v3/awesome/name:customVerb", StatusOK, "awesome")
}

func Test_Route_Match_Middleware_WildcardBeforeConst(t *testing.T) {
	t.Parallel()

	deny := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	guards := map[string]func(app *App){
		"use":       func(app *App) { app.Use("/api/*/admin", deny) },
		"use slash": func(app *App) { app.Use("/api/*/admin/", deny) },
		"plus":      func(app *App) { app.Use("/api/+/admin", deny) },
		"group":     func(app *App) { app.Group("/api/*/admin").Use(deny) },
		"mount": func(app *App) {
			sub := New(Config{StrictRouting: app.config.StrictRouting})
			sub.Use(deny)
			app.Use("/api/*/admin", sub)
		},
	}
	for name, guard := range guards {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/strict=%v", name, strict), func(t *testing.T) {
				t.Parallel()

				app := New(Config{StrictRouting: strict})
				guard(app)
				app.Get("/api/:tenant/admin/:page", func(c Ctx) error { return c.SendStatus(StatusOK) })
				app.Get("/api/*", func(c Ctx) error { return c.SendStatus(StatusOK) })

				for _, target := range []string{
					"/api/acme/admin",
					"/api/acme/admin/admins",
					"/api/acme/admin/admin-settings",
					"/api/acme/admin/users/admin.json",
					"/api/a/admin/b/admin/c",
					"/api/ACME/Admin/Admins",
				} {
					resp, err := app.Test(httptest.NewRequest(MethodGet, target, http.NoBody))
					require.NoError(t, err)
					require.Equal(t, StatusUnauthorized, resp.StatusCode, target)
				}
				for _, target := range []string{"/api/acme/admins", "/api/acme/admin.json", "/api/acme/status"} {
					resp, err := app.Test(httptest.NewRequest(MethodGet, target, http.NoBody))
					require.NoError(t, err)
					require.Equal(t, StatusOK, resp.StatusCode, target)
				}
			})
		}
	}

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()

		app := New(Config{CaseSensitive: true})
		app.Use("/api/*/Admin", deny)
		app.Get("/api/*", func(c Ctx) error { return c.SendStatus(StatusOK) })

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/acme/Admin/admins", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusUnauthorized, resp.StatusCode)

		resp, err = app.Test(httptest.NewRequest(MethodGet, "/api/acme/admin/Admins", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})
}

func Test_Route_Match_Middleware_ParamBeforeConst(t *testing.T) {
	t.Parallel()

	deny := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	ok := func(c Ctx) error { return c.SendStatus(StatusOK) }
	guards := map[string]func(app *App, prefix string){
		"use":       func(app *App, prefix string) { app.Use(prefix, deny) },
		"use slash": func(app *App, prefix string) { app.Use(prefix+"/", deny) },
		"group":     func(app *App, prefix string) { app.Group(prefix).Use("/", deny) },
		"mount": func(app *App, prefix string) {
			sub := New(Config{StrictRouting: app.config.StrictRouting})
			sub.Use("/", deny)
			app.Use(prefix, sub)
		},
		"domain mount": func(app *App, prefix string) {
			sub := New(Config{StrictRouting: app.config.StrictRouting})
			sub.Use("/", deny)
			app.Domain("example.com").Use(prefix, sub)
		},
	}
	shapes := []struct {
		prefix, below string
		guarded, open []string
	}{
		{
			prefix:  "/files/:name.json",
			below:   "/files/:name.json/raw",
			guarded: []string{"/files/a.json", "/files/a.json/raw", "/files/a.json.json/raw", "/files/A.JSON.Json/raw"},
			open:    []string{"/files/a.jsonx/raw", "/files/a.json.bak", "/files/a/b.json"},
		},
		{
			prefix:  "/admin/:v.",
			below:   "/admin/:v./x",
			guarded: []string{"/admin/a./x", "/admin/a.b./x", "/admin/a/b./x"},
			open:    []string{"/admin/a.b/x", "/admin/./x"},
		},
	}
	for name, guard := range guards {
		for _, strict := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/strict=%v", name, strict), func(t *testing.T) {
				t.Parallel()

				for _, shape := range shapes {
					app := New(Config{StrictRouting: strict})
					guard(app, shape.prefix)
					app.Get(shape.below, ok)
					app.Get("/*", ok)

					for _, method := range []string{MethodGet, MethodHead} {
						for _, target := range shape.guarded {
							resp, err := app.Test(httptest.NewRequest(method, target, http.NoBody))
							require.NoError(t, err)
							require.Equal(t, StatusUnauthorized, resp.StatusCode, "%s %s", method, target)
						}
						for _, target := range shape.open {
							resp, err := app.Test(httptest.NewRequest(method, target, http.NoBody))
							require.NoError(t, err)
							require.Equal(t, StatusOK, resp.StatusCode, "%s %s", method, target)
						}
					}
				}
			})
		}
	}

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()

		app := New(Config{CaseSensitive: true})
		app.Use("/files/:name.JSON", deny)
		app.Get("/*", ok)

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/files/a.JSON.JSON/raw", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusUnauthorized, resp.StatusCode)

		resp, err = app.Test(httptest.NewRequest(MethodGet, "/files/a.JSON.json/raw", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, StatusOK, resp.StatusCode)
	})
}

// Test_Route_Match_Middleware_Prefix pins which paths a Use prefix covers:
// everything below it, but not a sibling that only starts with the same characters.
func Test_Route_Match_Middleware_Prefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		prefix string
		path   string
		body   string
		status int
	}{
		{name: "has prefix", prefix: "/foo", path: "/foo/bar", status: StatusOK, body: "middleware"},
		{name: "no boundary", prefix: "/foo", path: "/foobar", status: StatusNotFound},
		{name: "root", prefix: "/", path: "/everything", status: StatusOK, body: "middleware"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()

			app.Use(tc.prefix, func(c Ctx) error { return c.SendString("middleware") })

			routerTestExpectPath(t, app, tc.path, tc.status, tc.body)
		})
	}
}

func Test_Router_Register_Missing_Handler(t *testing.T) {
	t.Parallel()
	app := New()

	t.Run("No Handler", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithValue(t, "missing handler/middleware in route: /doe\n", func() {
			app.register([]string{"USE"}, "/doe", nil)
		})
	})

	t.Run("Nil Handler", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithValue(t, "nil handler in route: /doe\n", func() {
			app.register([]string{"USE"}, "/doe", nil, nil)
		})
	})
}

func Test_Ensure_Router_Interface_Implementation(t *testing.T) {
	t.Parallel()
	var app any = (*App)(nil)
	_, ok := app.(Router)
	require.True(t, ok)

	var group any = (*Group)(nil)
	_, ok = group.(Router)
	require.True(t, ok)
}

func Test_Router_Handler_Catch_Error(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.ErrorHandler = func(_ Ctx, _ error) error { return errors.New("fake error") }

	app.Get("/", func(_ Ctx) error { return ErrForbidden })

	c := &fasthttp.RequestCtx{}

	app.Handler()(c)

	require.Equal(t, StatusInternalServerError, c.Response.Header.StatusCode())
}

func Test_Router_NotFound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "unknown route", path: "/this/route/does/not/exist"},
		// the body is a fixed message, so markup in the path is never echoed back
		{name: "HTML_Inject", path: "/does/not/exist<script>alert('foo');</script>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Use(func(c Ctx) error { return c.Next() })
			appHandler := app.Handler()
			c := &fasthttp.RequestCtx{}

			c.Request.Header.SetMethod("DELETE")
			c.URI().SetPath(tc.path)

			appHandler(c)

			require.Equal(t, 404, c.Response.StatusCode())
			require.Equal(t, "Not Found", string(c.Response.Body()))
		})
	}
}

func registerTreeManipulationRoutes(app *App, middleware ...func(Ctx) error) {
	converted := make([]any, len(middleware))
	for i, h := range middleware {
		converted[i] = h
	}

	app.Get("/test", func(c Ctx) error {
		app.Get("/dynamically-defined", func(c Ctx) error { return c.SendStatus(StatusOK) })

		app.RebuildTree()

		return c.SendStatus(StatusOK)
	}, converted...)
}

func verifyRouteHandlerCounts(tb testing.TB, app *App, expectedRoutesCount int) {
	tb.Helper()

	//  this is taken from listen.go's printRoutesMessage app method
	var routes []RouteMessage
	for _, routeStack := range app.stack {
		for _, route := range routeStack {
			routeMsg := RouteMessage{name: route.Name, method: route.Method, path: route.Path}

			for _, handler := range route.Handlers {
				routeMsg.handlers += runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name() + " "
			}

			routes = append(routes, routeMsg)
		}
	}

	for _, route := range routes {
		require.Equal(tb, expectedRoutesCount, strings.Count(route.handlers, " "))
	}
}

func verifyThereAreNoRoutes(tb testing.TB, app *App) {
	tb.Helper()

	require.Equal(tb, uint32(0), app.handlersCount)
	verifyRouteHandlerCounts(tb, app, 0)
}

func Test_App_Rebuild_Tree(t *testing.T) {
	t.Parallel()
	app := New()

	registerTreeManipulationRoutes(app)

	apptest.Expect(t, app, "/dynamically-defined", StatusNotFound)
	apptest.Expect(t, app, "/test", StatusOK)
	apptest.Expect(t, app, "/dynamically-defined", StatusOK)
}

func Test_App_Remove_Route_A_B_Feature_Testing(t *testing.T) {
	t.Parallel()
	app := New()

	features := []string{"a", "b"}
	for _, feature := range features {
		app.Get("/api/feature-"+feature, func(c Ctx) error {
			app.RemoveRoute("/api/feature", MethodGet)
			app.RebuildTree()
			app.Get("/api/feature", func(c Ctx) error { return c.SendString("Testing feature-" + feature) })

			app.RebuildTree()
			return c.SendStatus(StatusOK)
		})
	}

	// Each request swaps /api/feature for its own version, so the order matters.
	for _, feature := range features {
		apptest.Expect(t, app, "/api/feature-"+feature, StatusOK)

		apptest.ExpectBody(t, app, "/api/feature", StatusOK, "Testing feature-"+feature)
	}
}

// Test_App_Remove_Route removes a route by path or by name, existing or not,
// and checks what the app still serves and holds afterwards.
func Test_App_Remove_Route(t *testing.T) {
	t.Parallel()
	ok := func(c Ctx) error { return c.SendStatus(StatusOK) }
	for _, tc := range []struct {
		register func(app *App)
		remove   func(app *App)
		name     string
		served   string // a path served before the removal
		gone     string // a path that is a 404 after the removal
		rebuild  bool   // RebuildTree after the removal
		empty    bool   // no route or handler is left
	}{
		{
			name: "Remove_Route", register: func(app *App) { app.Get("/test", ok) }, rebuild: true, gone: "/test",
			remove: func(app *App) { app.RemoveRoute("/test", MethodGet) },
		},
		{
			name: "Non_Existing_Route", rebuild: true, empty: true,
			remove: func(app *App) { app.RemoveRoute("/test", MethodGet, MethodHead) },
		},
		{
			name: "Nested", served: "/api/v1/test", empty: true,
			register: func(app *App) { app.Group("/api").Group("/v1").Get("/test", ok) },
			remove:   func(app *App) { app.RemoveRoute("/api/v1/test", MethodGet) },
		},
		{
			name: "Parameterized", served: "/test/:id", empty: true,
			register: func(app *App) { app.Get("/test/:id", ok) },
			remove:   func(app *App) { app.RemoveRoute("/test/:id", MethodGet) },
		},
		{
			name: "By_Name", rebuild: true, gone: "/test", empty: true,
			register: func(app *App) { app.Get("/api/test", ok).Name("test") },
			remove:   func(app *App) { app.RemoveRouteByName("test", MethodGet) },
		},
		{
			name: "By_Name_Non_Existing_Route", rebuild: true, empty: true,
			remove: func(app *App) { app.RemoveRouteByName("test", MethodGet) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			if tc.register != nil {
				tc.register(app)
			}
			if tc.served != "" {
				apptest.Expect(t, app, tc.served, StatusOK)
			}
			tc.remove(app)
			if tc.rebuild {
				app.RebuildTree()
			}
			if tc.gone != "" {
				apptest.Expect(t, app, tc.gone, StatusNotFound)
			}
			if tc.empty {
				verifyThereAreNoRoutes(t, app)
			}
		})
	}
}

func Test_App_Remove_Route_StrictRouting_UsePrefix(t *testing.T) {
	t.Parallel()
	app := New(Config{StrictRouting: true})

	app.Use("/test/", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})

	apptest.Expect(t, app, "/test", StatusOK)
	app.RemoveRoute("/test/")
	app.RebuildTree()

	verifyThereAreNoRoutes(t, app)
	apptest.Expect(t, app, "/test", StatusNotFound)
}

// Test_Use_StrictRoutingBoundary pins where a Use prefix stops matching, for a
// middleware on the app and on a group: the prefix itself, a trailing slash and
// anything below it match, but a sibling that only starts with the same
// characters never does, with StrictRouting on or off.
func Test_Use_StrictRoutingBoundary(t *testing.T) {
	t.Parallel()
	for _, scope := range []struct {
		register func(app *App, matched *bool)
		name     string
		prefix   string // the path the middleware covers
		sibling  string // shares the prefix's characters without being below it
	}{
		{
			name:    "app",
			prefix:  "/api",
			sibling: "/apiv1",
			register: func(app *App, matched *bool) {
				app.Use("/api", func(c Ctx) error {
					*matched = true
					return c.SendStatus(StatusOK)
				})
			},
		},
		{
			name:    "group",
			prefix:  "/api/v1",
			sibling: "/api/v1beta",
			register: func(app *App, matched *bool) {
				grp := app.Group("/api")
				grp.Use("/v1", func(c Ctx) error {
					*matched = true
					return c.Next()
				})
				grp.Get("/v1", func(c Ctx) error { return c.SendStatus(StatusOK) })
				grp.Get("/v1/*", func(c Ctx) error { return c.SendStatus(StatusOK) })
			},
		},
	} {
		cases := []struct {
			name    string
			path    string
			status  int
			matched bool
		}{
			{name: "exact match", path: scope.prefix, status: StatusOK, matched: true},
			{name: "trailing slash partial", path: scope.prefix + "/", status: StatusOK, matched: true},
			{name: "nested partial", path: scope.prefix + "/users", status: StatusOK, matched: true},
			{name: "disallows sibling prefix", path: scope.sibling, status: StatusNotFound, matched: false},
		}

		for _, strict := range []bool{true, false} {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/strict=%v/%s", scope.name, strict, tc.name), func(t *testing.T) {
					t.Parallel()
					app := New(Config{StrictRouting: strict})

					matched := false
					scope.register(app, &matched)

					apptest.Expect(t, app, tc.path, tc.status)
					require.Equal(t, tc.matched, matched)
				})
			}
		}
	}
}

func Test_App_Remove_Route_Concurrent(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c Ctx) error { return c.SendStatus(StatusOK) })

	// Concurrently remove and add routes
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			app.RemoveRoute("/test", MethodGet)
			app.Get("/test", func(c Ctx) error { return c.SendStatus(StatusOK) })
		})
	}
	wg.Wait()
	app.RebuildTree()
	apptest.Expect(t, app, "/test", StatusOK)
}

func Test_Route_Registration_Prevent_Duplicate_With_Middleware(t *testing.T) {
	t.Parallel()
	app := New()

	middleware := func(c Ctx) error { return c.Next() }

	registerTreeManipulationRoutes(app, middleware)
	registerTreeManipulationRoutes(app)

	apptest.Expect(t, app, "/dynamically-defined", StatusNotFound)
	require.Equal(t, uint32(6), app.handlersCount)

	// RebuildTree registers the automatic HEAD companion right away, hence two.
	apptest.Expect(t, app, "/test", StatusOK)
	require.Equal(t, uint32(8), app.handlersCount)

	apptest.Expect(t, app, "/dynamically-defined", StatusOK)
	require.Equal(t, uint32(8), app.handlersCount)

	// The merged handler reaches the HEAD companion as well.
	apptest.Expect(t, app, "/test", StatusOK)
	require.Equal(t, uint32(10), app.handlersCount)

	apptest.Expect(t, app, "/dynamically-defined", StatusOK)
	require.Equal(t, uint32(10), app.handlersCount)
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		path          string
		expected      string
		caseSensitive bool
		strictRouting bool
	}{
		{name: "Empty path", path: "", caseSensitive: true, strictRouting: true, expected: "/"},
		{name: "No leading slash", path: "users", caseSensitive: true, strictRouting: true, expected: "/users"},
		{
			name: "With trailing slash and strict routing", path: "/users/", caseSensitive: true, strictRouting: true, expected: "/users/",
		},
		{
			name: "With trailing slash and non-strict routing", path: "/users/", caseSensitive: true, strictRouting: false, expected: "/users",
		},
		{name: "Case sensitive", path: "/Users", caseSensitive: true, strictRouting: true, expected: "/Users"},
		{name: "Case insensitive", path: "/Users", caseSensitive: false, strictRouting: true, expected: "/users"},
		{
			name: "With escape characters", path: "/users\\/profile", caseSensitive: true, strictRouting: true, expected: "/users/profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &App{config: Config{CaseSensitive: tt.caseSensitive, StrictRouting: tt.strictRouting}}
			result := app.normalizePath(tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRemoveRoute(t *testing.T) {
	t.Parallel()
	app := New()
	var buf strings.Builder
	// record returns a handler that writes id to buf before handing over to next.
	record := func(id string, next Handler) Handler {
		return func(c Ctx) error {
			buf.WriteString(id) //nolint:errcheck // not needed
			return next(c)
		}
	}
	ok := func(c Ctx) error { return c.SendStatus(StatusOK) }
	app.Use(record("1", Ctx.Next))
	app.Post("/", record("2", ok))
	app.Use("/test", record("3", Ctx.Next))
	app.Get("/test", record("4", ok))
	app.Post("/test", record("5", ok))
	app.startupProcess()
	require.Equal(t, uint32(6), app.handlersCount)

	apptest.Expect(t, app, apptest.Req(MethodPost, "/"), 200)
	require.Equal(t, "12", buf.String())
	buf.Reset()
	apptest.Expect(t, app, apptest.Req(MethodGet, "/test"), 200)
	require.Equal(t, "134", buf.String())
	buf.Reset()
	require.Equal(t, uint32(6), app.handlersCount)

	app.RemoveRoute("/test", MethodGet)
	app.RebuildTree()
	app.RemoveRoute("/test", "TEST")
	app.RebuildTree()
	app.RemoveRouteFunc(func(_ *Route) bool { return false }, MethodGet)
	apptest.Expect(t, app, apptest.Req(MethodGet, "/test"), StatusMethodNotAllowed)
	buf.Reset()
	require.Equal(t, uint32(4), app.handlersCount)

	app.RemoveRoute("/test", MethodPost)
	app.RebuildTree()
	require.Equal(t, uint32(3), app.handlersCount)
	apptest.Expect(t, app, apptest.Req(MethodPost, "/test"), 404)
	require.Equal(t, "1", buf.String())
	buf.Reset()
	apptest.Expect(t, app, apptest.Req(MethodGet, "/test"), 404)
	require.Equal(t, "1", buf.String())
	buf.Reset()

	app.RemoveRoute("/", MethodGet, MethodPost)
	require.Equal(t, uint32(2), app.handlersCount)
	apptest.Expect(t, app, apptest.Req(MethodGet, "/"), 404)
	require.Empty(t, buf.String())
	buf.Reset()

	app.RemoveRoute("/test", MethodGet, MethodPost)
	require.Equal(t, uint32(2), app.handlersCount)
	app.RemoveRoute("/test", app.config.RequestMethods...)
	require.Equal(t, uint32(1), app.handlersCount)
	app.Patch("/test", record("6", ok))
	require.Equal(t, uint32(2), app.handlersCount)
	app.RemoveRoute("/test")
	app.RemoveRoute("/")
	app.RebuildTree()
	require.Equal(t, uint32(0), app.handlersCount)
}

func registerDummyRoutes(app *App) {
	h := func(_ Ctx) error { return nil }
	for _, r := range routesFixture.GitHubAPI {
		app.Add([]string{r.Method}, r.Path, h)
	}
}

// routerBenchCtx returns a request context for method and path, set on the parsed URI.
func routerBenchCtx(method, path string) *fasthttp.RequestCtx {
	c := &fasthttp.RequestCtx{}
	c.Request.Header.SetMethod(method)
	c.URI().SetPath(path)
	return c
}

// routerBenchHandler returns app's request handler and a routerBenchCtx for method and path.
func routerBenchHandler(app *App, method, path string) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	return app.Handler(), routerBenchCtx(method, path)
}

// routerBenchApp returns a new app, configured with cfg, that has the GitHub API routes.
func routerBenchApp(cfg ...Config) *App {
	app := New(cfg...)
	registerDummyRoutes(app)
	return app
}

// routerBenchStarted returns an app with the GitHub API routes that has been through its startup process.
func routerBenchStarted() *App {
	app := routerBenchApp()
	app.startupProcess()
	return app
}

// routerBenchKeys returns the request handler of an app configured with cfg that has the GitHub API routes, and a
// context for DELETE /user/keys/1337.
func routerBenchKeys(cfg ...Config) (fasthttp.RequestHandler, *fasthttp.RequestCtx) {
	return routerBenchHandler(routerBenchApp(cfg...), MethodDelete, "/user/keys/1337")
}

// routerBenchRootHandler returns the request handler of an app with cfg whose
// only route answers GET / with nothing.
func routerBenchRootHandler(cfg ...Config) fasthttp.RequestHandler {
	app := New(cfg...)
	app.Get("/", func(_ Ctx) error { return nil })
	return app.Handler()
}

// routerBenchRoute returns a DELETE route parsed from pattern and named path, for the Route.match benchmarks.
func routerBenchRoute(pattern, path string) *Route {
	parsed := parseRoute(pattern, regexp.MustCompile)
	return &Route{
		star: pattern == "/*", root: pattern == "/", routeParser: parsed, Params: parsed.params,
		path: path, Path: path, Method: MethodDelete, Handlers: []Handler{func(_ Ctx) error { return nil }},
	}
}

// acquireDefaultCtxForRouterBenchmark acquires a *DefaultCtx inside a benchmark
// loop. It skips b.Helper(), whose stack walk was a quarter of the samples.
//
//nolint:thelper // b.Helper() is the cost being avoided
func acquireDefaultCtxForRouterBenchmark(b *testing.B, app *App, fctx *fasthttp.RequestCtx) *DefaultCtx {
	ctx := app.AcquireCtx(fctx)
	defaultCtx, ok := ctx.(*DefaultCtx)
	if !ok {
		b.Fatal("AcquireCtx did not return *DefaultCtx")
	}
	return defaultCtx
}

func Benchmark_App_RebuildTree(b *testing.B) {
	app := routerBenchApp()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		app.hasRoutesRefreshed = true
		app.RebuildTree()
	}
}

func Benchmark_App_MethodNotAllowed(b *testing.B) {
	app := New()
	h := func(c Ctx) error { return c.SendString("Hello World!") }
	app.All("/this/is/a/", h)
	app.Get("/this/is/a/dummy/route/oke", h)
	appHandler, c := routerBenchHandler(app, MethodDelete, "/this/is/a/dummy/route/oke")
	for b.Loop() {
		appHandler(c)
	}
	require.Equal(b, 405, c.Response.StatusCode())
	require.Equal(b, MethodGet+", "+MethodHead, string(c.Response.Header.Peek("Allow")))
	require.Equal(b, utils.StatusMessage(StatusMethodNotAllowed), string(c.Response.Body()))
}

func Benchmark_Router_NotFound(b *testing.B) {
	b.ReportAllocs()
	app := New()
	app.Use(func(c Ctx) error { return c.Next() })
	registerDummyRoutes(app)
	appHandler, c := routerBenchHandler(app, MethodDelete, "/this/route/does/not/exist")
	for b.Loop() {
		appHandler(c)
	}
	require.Equal(b, 404, c.Response.StatusCode())
	require.Equal(b, "Not Found", string(c.Response.Body()))
}

func Benchmark_Router_OptionalMiss(b *testing.B) {
	for _, tc := range []struct {
		name     string
		prefix   string
		path     string
		patterns []string
	}{
		{"lang", "/:lang?", "/en/pricing", []string{"/about", "/contact", "/docs", "/blog", "/support"}},
		{"version", "/api/:version?", "/api/v1/pricing", []string{"/users", "/teams", "/projects", "/settings"}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			app := New()
			handler := func(c Ctx) error { return c.SendStatus(StatusNoContent) }
			for _, pattern := range tc.patterns {
				app.Get(tc.prefix+pattern, handler)
			}
			app.Get(tc.path, handler)
			appHandler, ctx := routerBenchHandler(app, MethodGet, tc.path)
			appHandler(ctx)
			require.Equal(b, StatusNoContent, ctx.Response.StatusCode())
			b.ReportAllocs()
			for b.Loop() {
				appHandler(ctx)
			}
			require.Equal(b, StatusNoContent, ctx.Response.StatusCode())
		})
	}
}

func Benchmark_Router_Handler(b *testing.B) {
	appHandler, c := routerBenchKeys()
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Strict_Case(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{StrictRouting: true, CaseSensitive: true})
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Chain(b *testing.B) {
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	app.Get("/", handler, handler, handler, handler, handler, handler)
	appHandler, c := routerBenchHandler(app, MethodGet, "/")
	for b.Loop() {
		appHandler(c)
	}
}

// Benchmark_Router_Chain_FilteredBucket runs middleware chains in front of an
// endpoint whose bucket holds enough routes to carry a filter, so each hop
// scans a filtered bucket from the middleware it left:
//   - use: eight middlewares registered between routes of other buckets,
//     which keeps them from merging into one route, so that each hop takes
//     its first candidate
//   - use_interleaved: the same with the routes in the middlewares' bucket,
//     so that each hop steps over one
//   - group: a global middleware, then two nested groups' middlewares
func Benchmark_Router_Chain_FilteredBucket(b *testing.B) {
	handler := func(Ctx) error { return nil }
	mw := func(c Ctx) error { return c.Next() }
	use := func(route string) func(app *App) {
		return func(app *App) {
			for i := range 8 {
				app.Use("/api", mw)
				app.Get(route+strconv.Itoa(i), handler)
			}
			app.Get("/api/v1/users", handler)
		}
	}
	for _, bench := range []struct {
		register func(app *App)
		name     string
	}{
		{name: "use", register: use("/web/r")},
		{name: "use_interleaved", register: use("/api/v1/r")},
		{name: "group", register: func(app *App) {
			app.Use(mw)
			v1 := app.Group("/api", mw).Group("/v1", mw)
			for i := range 8 {
				v1.Get("/r"+strconv.Itoa(i), handler)
			}
			v1.Get("/users", handler)
		}},
	} {
		b.Run(bench.name, func(b *testing.B) {
			app := New()
			bench.register(app)
			appHandler, c := routerBenchHandler(app, MethodGet, "/api/v1/users")
			appHandler(c)
			require.Equal(b, StatusOK, c.Response.StatusCode())
			for b.Loop() {
				appHandler(c)
			}
		})
	}
}

func Benchmark_Router_WithCompression(b *testing.B) {
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	for range 6 {
		app.Get("/", handler)
	}
	appHandler, c := routerBenchHandler(app, MethodGet, "/")
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Startup_Process(b *testing.B) {
	for b.Loop() {
		app := New()
		registerDummyRoutes(app)
		app.startupProcess()
	}
}

func Benchmark_Router_Next(b *testing.B) {
	app := routerBenchStarted()
	var res bool
	var err error
	c := acquireDefaultCtxForRouterBenchmark(b, app, routerBenchCtx(MethodDelete, "/user/keys/1337"))
	for b.Loop() {
		c.indexRoute = -1
		res, err = app.next(c)
	}
	require.NoError(b, err)
	require.True(b, res)
	require.Equal(b, 4, c.indexRoute)
}

// Benchmark_Router_PathOverride measures placing a prefix-mounted rewrite in the
// new bucket, registered ahead of the route table and behind it.
func Benchmark_Router_PathOverride(b *testing.B) {
	rewrite := func(c Ctx) error {
		c.Path(strings.TrimPrefix(c.Path(), "/v1"))
		return c.Next()
	}
	for _, tc := range []struct {
		name  string
		first bool
	}{{name: "rewrite first", first: true}, {name: "rewrite last"}} {
		b.Run(tc.name, func(b *testing.B) {
			app := New()
			if tc.first {
				app.Use("/v1", rewrite)
			}
			registerDummyRoutes(app)
			if !tc.first {
				app.Use("/v1", rewrite)
				app.Get("/user/keys/:id", func(c Ctx) error { return c.SendStatus(StatusOK) })
			}
			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(MethodGet)

			b.ReportAllocs()
			for b.Loop() {
				fctx.URI().SetPath("/v1/user/keys/1337")
				h(fctx)
			}
			require.Equal(b, StatusOK, fctx.Response.StatusCode())
		})
	}
}

func Benchmark_Router_Next_Default(b *testing.B) {
	h := routerBenchRootHandler()
	fctx := routerTestRequestCtx(MethodGet, "/")
	b.ReportAllocs()
	for b.Loop() {
		h(fctx)
	}
}

func Benchmark_Router_Next_Default_Parallel(b *testing.B) {
	h := routerBenchRootHandler()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		fctx := routerTestRequestCtx(MethodGet, "/")
		for pb.Next() {
			h(fctx)
		}
	})
}

func Benchmark_Router_Next_Default_Immutable(b *testing.B) {
	h := routerBenchRootHandler(Config{Immutable: true})
	fctx := routerTestRequestCtx(MethodGet, "/")
	b.ReportAllocs()
	for b.Loop() {
		h(fctx)
	}
}

func Benchmark_Router_Next_Default_Parallel_Immutable(b *testing.B) {
	h := routerBenchRootHandler(Config{Immutable: true})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		fctx := routerTestRequestCtx(MethodGet, "/")
		for pb.Next() {
			h(fctx)
		}
	})
}

func Benchmark_Route_Match(b *testing.B) {
	var match bool
	var params [maxParams]string
	route := routerBenchRoute("/user/keys/:id", "/user/keys/:id")
	for b.Loop() {
		match = route.match("/user/keys/1337", "/user/keys/1337", &params, 3)
	}
	require.True(b, match)
	require.Equal(b, []string{"1337"}, params[0:len(route.Params)])
}

func Benchmark_Route_Match_Star(b *testing.B) {
	var match bool
	var params [maxParams]string
	route := routerBenchRoute("/*", "/user/keys/bla")
	for b.Loop() {
		match = route.match("/user/keys/bla", "/user/keys/bla", &params, 3)
	}
	require.True(b, match)
	require.Equal(b, []string{"user/keys/bla"}, params[0:len(route.Params)])
}

func Benchmark_Route_Match_Root(b *testing.B) {
	var match bool
	var params [maxParams]string
	route := routerBenchRoute("/", "/")
	for b.Loop() {
		match = route.match("/", "/", &params, 1)
	}
	require.True(b, match)
	require.Equal(b, []string{}, params[0:len(route.Params)])
}

func Benchmark_Router_Handler_CaseSensitive(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{CaseSensitive: true})
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_Handler_Unescape(b *testing.B) {
	app := routerBenchApp(Config{UnescapePath: true})
	app.Delete("/créer", func(_ Ctx) error { return nil })
	appHandler, c := routerBenchHandler(app, MethodDelete, "/cr%C3%A9er")
	for b.Loop() {
		c.URI().SetPath("/cr%C3%A9er")
		appHandler(c)
	}
}

func Benchmark_Router_Handler_StrictRouting(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{CaseSensitive: true})
	for b.Loop() {
		appHandler(c)
	}
}

func Benchmark_Router_GitHub_API(b *testing.B) {
	app := routerBenchStarted()
	c := &fasthttp.RequestCtx{}
	var match bool
	var err error
	b.ResetTimer()
	for i := range routesFixture.TestRoutes {
		b.RunParallel(func(pb *testing.PB) {
			c.Request.Header.SetMethod(routesFixture.TestRoutes[i].Method)
			for pb.Next() {
				c.URI().SetPath(routesFixture.TestRoutes[i].Path)

				ctx := app.AcquireCtx(c).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

				match, err = app.next(ctx)
				app.ReleaseCtx(ctx)
			}
		})
		require.NoError(b, err)
		require.True(b, match)
	}
}

type testRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type routeJSON struct {
	TestRoutes []testRoute `json:"test_routes"`
	GitHubAPI  []testRoute `json:"github_api"`
}

// newCustomApp returns an app that serves every request with a customCtx.
func newCustomApp(cfg ...Config) *App {
	return NewWithCustomCtx(func(app *App) CustomCtx { return &customCtx{DefaultCtx: *NewDefaultCtx(app)} }, cfg...)
}

func Test_Next_SkipNonUseRoutesSkipsFallback(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })
	app.startupProcess()

	fctx := routerTestRequestCtx(MethodPost, "/foo")

	ctx := app.AcquireCtx(fctx).(*DefaultCtx) //nolint:errcheck,forcetypeassert // default app returns DefaultCtx
	ctx.setSkipNonUseRoutes(true)
	defer app.ReleaseCtx(ctx)

	matched, err := app.next(ctx)
	require.False(t, matched)
	require.NoError(t, err)
	require.Empty(t, ctx.Response().Header.Peek(HeaderAllow))
}

func Test_NextCustom_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })
	useRoute := &Route{use: true, path: "/foo", Path: "/foo", routeParser: parseRoute("/foo", regexp.MustCompile)}
	m := app.methodInt(MethodGet)
	app.stack[m] = append([]*Route{useRoute}, app.stack[m]...)
	app.hasRoutesRefreshed = true
	app.ensureAutoHeadRoutes()
	app.RebuildTree()

	ctx := apptest.CtxFor(t, app, routerTestRequestCtx(MethodPost, "/foo"))

	matched, err := app.nextCustom(ctx)
	require.False(t, matched)
	require.ErrorIs(t, err, ErrMethodNotAllowed)
	allow := string(ctx.Response().Header.Peek(HeaderAllow))
	require.Equal(t, "GET, HEAD", allow)
}

func Test_NextCustom_SkipNonUseRoutesSkipsFallback(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
	app.Get("/foo", func(c Ctx) error { return c.SendStatus(StatusOK) })
	app.startupProcess()

	ctx := apptest.CtxFor(t, app, routerTestRequestCtx(MethodPost, "/foo"))
	ctx.setSkipNonUseRoutes(true)

	matched, err := app.nextCustom(ctx)
	require.False(t, matched)
	require.NoError(t, err)
	require.Empty(t, ctx.Response().Header.Peek(HeaderAllow))
}

func Test_NextCustom_NotFound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		uri     string
		matched bool
	}{
		{name: "no route", uri: "/not-exist"},
		{name: "matched earlier", uri: "/none", matched: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newCustomApp()
			app.RebuildTree()

			ctx := apptest.CtxFor(t, app, routerTestRequestCtx(MethodGet, tc.uri))
			if tc.matched {
				ctx.setMatched(true)
			}

			matched, err := app.nextCustom(ctx)
			require.False(t, matched)
			var e *Error
			require.ErrorAs(t, err, &e)
			require.Equal(t, StatusNotFound, e.Code)
		})
	}
}

func Test_RequestHandler_CustomCtx_NotImplemented(t *testing.T) {
	t.Parallel()
	app := newCustomApp()

	h := app.Handler()
	fctx := routerTestRequestCtx("UNKNOWN", "/")

	h(fctx)
	require.Equal(t, StatusNotImplemented, fctx.Response.StatusCode())
}

func Test_NextCustom_SkipMountAndNoHandlers(t *testing.T) {
	t.Parallel()
	app := newCustomApp()
	m := app.methodInt(MethodGet)
	mountR := &Route{path: "/skip", Path: "/skip", routeParser: parseRoute("/skip", regexp.MustCompile), mount: true}
	empty := &Route{path: "/foo", Path: "/foo", routeParser: parseRoute("/foo", regexp.MustCompile)}
	app.stack[m] = []*Route{mountR, empty}
	app.hasRoutesRefreshed = true
	app.RebuildTree()

	ctx := apptest.CtxFor(t, app, routerTestRequestCtx(MethodGet, "/foo"))

	matched, err := app.nextCustom(ctx)
	require.True(t, matched)
	require.NoError(t, err)
	require.Equal(t, "/foo", ctx.Route().Path)
}

func Test_AddRoute_MergeHandlers(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/merge", testEmptyHandler)
	app.Get("/merge", testEmptyHandler)

	require.Len(t, app.stack[app.methodInt(MethodGet)], 1)
	require.Len(t, app.stack[app.methodInt(MethodGet)][0].Handlers, 2)
}

func Benchmark_App_RebuildTree_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each worker gets its own App instance to avoid data races on shared state
		localApp := routerBenchApp()
		for pb.Next() {
			localApp.hasRoutesRefreshed = true
			localApp.RebuildTree()
		}
	})
}

func Benchmark_App_MethodNotAllowed_Parallel(b *testing.B) {
	app := New()
	h := func(c Ctx) error { return c.SendString("Hello World!") }
	app.All("/this/is/a/", h)
	app.Get("/this/is/a/dummy/route/oke", h)
	appHandler := app.Handler()
	b.RunParallel(func(pb *testing.PB) {
		// Each worker gets its own RequestCtx to avoid data races
		c := routerBenchCtx(MethodDelete, "/this/is/a/dummy/route/oke")
		for pb.Next() {
			appHandler(c)
		}
	})

	// Single-threaded verification on a fresh context to preserve correctness checks
	verifyCtx := routerBenchCtx(MethodDelete, "/this/is/a/dummy/route/oke")
	appHandler(verifyCtx)
	require.Equal(b, 405, verifyCtx.Response.StatusCode())
	require.Equal(b, MethodGet+", "+MethodHead, string(verifyCtx.Response.Header.Peek("Allow")))
	require.Equal(b, utils.StatusMessage(StatusMethodNotAllowed), string(verifyCtx.Response.Body()))
}

func Benchmark_Router_NotFound_Parallel(b *testing.B) {
	b.ReportAllocs()
	app := New()
	app.Use(func(c Ctx) error { return c.Next() })
	registerDummyRoutes(app)
	appHandler, c := routerBenchHandler(app, MethodDelete, "/this/route/does/not/exist")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
	require.Equal(b, 404, c.Response.StatusCode())
	require.Equal(b, "Not Found", string(c.Response.Body()))
}

func Benchmark_Router_Handler_Parallel(b *testing.B) {
	appHandler, c := routerBenchKeys()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Router_Handler_Strict_Case_Parallel(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{StrictRouting: true, CaseSensitive: true})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Router_Chain_Parallel(b *testing.B) {
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	app.Get("/", handler, handler, handler, handler, handler, handler)
	appHandler, c := routerBenchHandler(app, MethodGet, "/")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Router_WithCompression_Parallel(b *testing.B) {
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	for range 6 {
		app.Get("/", handler)
	}
	appHandler, c := routerBenchHandler(app, MethodGet, "/")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Startup_Process_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			app := New()
			registerDummyRoutes(app)
			app.startupProcess()
		}
	})
}

func Benchmark_Router_Next_Parallel(b *testing.B) {
	app := routerBenchStarted()
	b.RunParallel(func(pb *testing.PB) {
		// Each worker gets its own request and context to avoid data races.
		c := acquireDefaultCtxForRouterBenchmark(b, app, routerBenchCtx(MethodDelete, "/user/keys/1337"))
		for pb.Next() {
			c.indexRoute = -1
			//nolint:errcheck // Benchmark hot path - error checked in verification
			_, _ = app.next(c)
		}
	})

	// Single-threaded verification on a fresh context to preserve correctness checks.
	verifyCtx := acquireDefaultCtxForRouterBenchmark(b, app, routerBenchCtx(MethodDelete, "/user/keys/1337"))
	verifyCtx.indexRoute = -1
	res, err := app.next(verifyCtx)
	require.NoError(b, err)
	require.True(b, res)
	require.Equal(b, 4, verifyCtx.indexRoute)
}

func Benchmark_Router_Next_Default_Immutable_Parallel(b *testing.B) {
	h := routerBenchRootHandler(Config{Immutable: true})
	fctx := routerTestRequestCtx(MethodGet, "/")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h(fctx)
		}
	})
}

func Benchmark_Route_Match_Parallel(b *testing.B) {
	route := routerBenchRoute("/user/keys/:id", "/user/keys/:id")
	b.RunParallel(func(pb *testing.PB) {
		// Each worker gets its own local variables to avoid data races
		var params [maxParams]string
		for pb.Next() {
			_ = route.match("/user/keys/1337", "/user/keys/1337", &params, 3)
		}
	})

	// Single-threaded verification to preserve correctness checks
	var verifyParams [maxParams]string
	match := route.match("/user/keys/1337", "/user/keys/1337", &verifyParams, 3)
	require.True(b, match)
	require.Equal(b, []string{"1337"}, verifyParams[0:len(route.Params)])
}

func Benchmark_Route_Match_Star_Parallel(b *testing.B) {
	var match bool
	var params [maxParams]string
	route := routerBenchRoute("/*", "/user/keys/bla")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			match = route.match("/user/keys/bla", "/user/keys/bla", &params, 3)
		}
	})
	require.True(b, match)
	require.Equal(b, []string{"user/keys/bla"}, params[0:len(route.Params)])
}

func Benchmark_Route_Match_Root_Parallel(b *testing.B) {
	var match bool
	var params [maxParams]string
	route := routerBenchRoute("/", "/")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			match = route.match("/", "/", &params, 1)
		}
	})
	require.True(b, match)
	require.Equal(b, []string{}, params[0:len(route.Params)])
}

func Benchmark_Router_Handler_CaseSensitive_Parallel(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{CaseSensitive: true})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Router_Handler_Unescape_Parallel(b *testing.B) {
	app := routerBenchApp(Config{UnescapePath: true})
	app.Delete("/créer", func(_ Ctx) error { return nil })
	appHandler, c := routerBenchHandler(app, MethodDelete, "/cr%C3%A9er")
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.URI().SetPath("/cr%C3%A9er")
			appHandler(c)
		}
	})
}

func Benchmark_Router_Handler_StrictRouting_Parallel(b *testing.B) {
	appHandler, c := routerBenchKeys(Config{CaseSensitive: true})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			appHandler(c)
		}
	})
}

func Benchmark_Router_GitHub_API_Parallel(b *testing.B) {
	app := routerBenchStarted()
	b.ResetTimer()
	for i := range routesFixture.TestRoutes {
		b.RunParallel(func(pb *testing.PB) {
			// Each worker gets its own RequestCtx and local variables to avoid data races
			c := &fasthttp.RequestCtx{}
			c.Request.Header.SetMethod(routesFixture.TestRoutes[i].Method)
			for pb.Next() {
				c.URI().SetPath(routesFixture.TestRoutes[i].Path)
				ctx := acquireDefaultCtxForRouterBenchmark(b, app, c)
				//nolint:errcheck // Benchmark hot path - error checked in verification
				_, _ = app.next(ctx)
				app.ReleaseCtx(ctx)
			}
		})

		// Single-threaded verification on a fresh context to preserve correctness checks
		route := routesFixture.TestRoutes[i]
		verifyCtx := acquireDefaultCtxForRouterBenchmark(b, app, routerBenchCtx(route.Method, route.Path))
		match, err := app.next(verifyCtx)
		app.ReleaseCtx(verifyCtx)
		require.NoError(b, err)
		require.True(b, match)
	}
}

func Test_Route_URL(t *testing.T) {
	t.Parallel()
	// requireURL asserts the URL composed from params for the route called name.
	requireURL := func(t *testing.T, app *App, name string, params Map, want string) {
		t.Helper()

		route := app.GetRoute(name)
		url, err := route.URL(params)
		require.NoError(t, err)
		require.Equal(t, want, url)
	}

	for _, tc := range []struct {
		params  Map
		name    string
		pattern string
		want    string
	}{
		{name: "simple parameter", pattern: "/user/:name", params: Map{"name": "fiber"}, want: "/user/fiber"},
		{name: "multiple parameters", pattern: "/user/:name/:id", params: Map{"name": "john", "id": "123"}, want: "/user/john/123"},
		{
			name: "wildcard parameters", pattern: "/:phone/*/send/*",
			params: Map{"phone": "23456789", "*1": "sms", "*2": "test-msg"}, want: "/23456789/sms/send/test-msg",
		},
		{
			name: "single wildcard parameter", pattern: "/:phone/*/send", params: Map{"phone": "23456789", "*": "sms"},
			want: "/23456789/sms/send",
		},
		{name: "plus parameters prefer plus fallback", pattern: "/user/+", params: Map{"*": "wildcard", "+": "plus"}, want: "/user/plus"},
		{name: "no parameters", pattern: "/static/path", params: Map{}, want: "/static/path"},
		// a parameter without a value leaves an empty placeholder
		{name: "missing parameter value", pattern: "/user/:name/:id", params: Map{"name": "john"}, want: "/user/john/"},
		{name: "extra parameters ignored", pattern: "/user/:name", params: Map{"name": "fiber", "extra": "ignored"}, want: "/user/fiber"},
		{name: "numeric parameter values", pattern: "/item/:id", params: Map{"id": 42}, want: "/item/42"},
		{
			name: "complex path with mixed content", pattern: "/api/v1/users/:userId/posts/:postId/comments",
			params: Map{"userId": "user123", "postId": "post456"}, want: "/api/v1/users/user123/posts/post456/comments",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get(tc.pattern, emptyHandler).Name("route")

			requireURL(t, app, "route", tc.params, tc.want)
		})
	}

	t.Run("wildcard parameters preserve slashes", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/files/*", func(c Ctx) error { return c.SendString(c.Params("*")) }).Name("Files")

		requireURL(t, app, "Files", Map{"*": "docs/index.html"}, "/files/docs/index.html")

		apptest.ExpectBody(t, app, "/files/docs/index.html", StatusOK, "docs/index.html")
	})

	t.Run("preferred greedy parameters default fallback", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, preferredPlusGreedyParameters, preferredGreedyParameters("+1"))
		require.Equal(t, []string{"*", "+"}, preferredGreedyParameters("*1"))
		require.Equal(t, defaultGreedyParameterKeys, preferredGreedyParameters(""))
		require.Equal(t, defaultGreedyParameterKeys, preferredGreedyParameters("name"))
	})

	t.Run("case insensitive default", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/user/:name", emptyHandler).Name("User")

		// Case-insensitive by default (matching Fiber's default behavior)
		requireURL(t, app, "User", Map{"Name": "fiber"}, "/user/fiber")

		// When multiple keys case-fold to the same param name and no exact key
		// exists, the lexicographically-smallest key wins deterministically.
		for range 50 {
			requireURL(t, app, "User", Map{"nAme": "second", "Name": "first"}, "/user/first")
		}

		// When multiple keys case-fold to the same param name, prefer the exact match.
		requireURL(t, app, "User", Map{"name": "exact", "Name": "fallback"}, "/user/exact")
	})

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()
		app := New(Config{CaseSensitive: true})
		app.Get("/user/:name", emptyHandler).Name("User")

		// Exact case match succeeds
		requireURL(t, app, "User", Map{"name": "fiber"}, "/user/fiber")
		// Different case does not match when CaseSensitive is true
		requireURL(t, app, "User", Map{"Name": "fiber"}, "/user/")
	})

	t.Run("empty route", func(t *testing.T) {
		t.Parallel()
		route := Route{}
		url, err := route.URL(Map{"name": "fiber"})
		require.Error(t, err)
		require.Equal(t, ErrNotFound, err)
		require.Empty(t, url)
	})

	t.Run("route without parsed segments returns path", func(t *testing.T) {
		t.Parallel()
		// This covers fallback/manual Route values whose Path is set but whose
		// routeParser was never populated, such as synthetic routes outside the
		// normal registration pipeline.
		route := Route{Path: "/error"}
		url, err := route.URL(Map{"name": "fiber"})
		require.NoError(t, err)
		require.Equal(t, "/error", url)
	})

	t.Run("GetRoute direct call", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/user/:name", emptyHandler).Name("User")
		requireURL(t, app, "User", Map{"name": "fiber"}, "/user/fiber")
	})

	t.Run("parameter values are path escaped", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Get("/value/:value", emptyHandler).Name("Value")

		const value = "a/b?c#d% e"
		location, err := app.GetRoute("Value").URL(Map{"value": value})
		require.NoError(t, err)
		require.Equal(t, "/value/a%2Fb%3Fc%23d%25%20e", location)

		decoded, err := neturl.PathUnescape(strings.TrimPrefix(location, "/value/"))
		require.NoError(t, err)
		require.Equal(t, value, decoded)
	})
}

// newSkipApp builds an app with SkipUnmatchedRoutes enabled plus a benign
// middleware, so requests actually exercise the lookahead fast path (which is
// gated on the presence of middleware).
func newSkipApp(cfg ...Config) *App {
	c := Config{SkipUnmatchedRoutes: true}
	if len(cfg) > 0 {
		c = cfg[0]
		c.SkipUnmatchedRoutes = true
	}
	app := New(c)
	app.Use(func(c Ctx) error { return c.Next() })
	return app
}

func Test_App_SkipUnmatchedRoutes_CORSPreflight(t *testing.T) {
	t.Parallel()
	app := New(Config{SkipUnmatchedRoutes: true})
	// Minimal CORS-style middleware: answer preflight, pass everything else through.
	app.Use(func(c Ctx) error {
		if c.IsPreflight() {
			c.Set(HeaderAccessControlAllowOrigin, "*")
			return c.SendStatus(StatusNoContent)
		}
		return c.Next()
	})
	app.Get("/users", func(c Ctx) error { return c.SendString("users") })

	t.Run("preflight_runs_middleware", func(t *testing.T) {
		t.Parallel()
		req := apptest.Req(MethodOptions, "/users",
			apptest.WithHeader(HeaderOrigin, "https://example.com"),
			apptest.WithHeader(HeaderAccessControlRequestMethod, MethodGet))
		resp := routerTestExpect(t, app, req, StatusNoContent)
		require.Equal(t, "*", resp.Header.Get(HeaderAccessControlAllowOrigin))
	})

	t.Run("non_preflight_options_still_405", func(t *testing.T) {
		t.Parallel()
		// Bare OPTIONS (no CORS headers) is not a preflight, so the fast path still answers 405.
		routerTestExpect(t, app, apptest.Req(MethodOptions, "/users"), StatusMethodNotAllowed)
	})
}

// Test_App_SkipUnmatchedRoutes_MiddlewareSkipped verifies that middleware does
// NOT run for unmatched requests but DOES run for matched ones.
func Test_App_SkipUnmatchedRoutes_MiddlewareSkipped(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		path    string
		message string
		status  int
		skip    bool
		called  bool
	}{
		{
			name: "skipped_on_404", skip: true, path: "/nope", status: StatusNotFound,
			called: false, message: "middleware must NOT run for an unmatched route",
		},
		{
			name: "runs_on_match", skip: true, path: "/ok", status: StatusOK,
			called: true, message: "middleware must run for a matched route",
		},
		{
			// SkipUnmatchedRoutes is false by default
			name: "disabled_runs_middleware", skip: false, path: "/nope", status: StatusNotFound,
			called: true, message: "middleware should still run when the feature is disabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{SkipUnmatchedRoutes: tc.skip})
			called := false
			app.Use(func(c Ctx) error { called = true; return c.Next() })
			app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

			routerTestExpect(t, app, tc.path, tc.status)
			require.Equal(t, tc.called, called, tc.message)
		})
	}
}

// Test_App_SkipUnmatchedRoutes_Requests sends requests through apps with
// SkipUnmatchedRoutes on: each row is a route shape or config the lookahead has
// to resolve exactly as the router does. Rows run on newSkipApp's pass-through
// middleware, which is what turns the lookahead on, unless they are bare.
func Test_App_SkipUnmatchedRoutes_Requests(t *testing.T) {
	t.Parallel()
	ok := func(c Ctx) error { return c.SendString("ok") }
	param := func(name string) Handler { return func(c Ctx) error { return c.SendString(c.Params(name)) } }
	mountProfile := func(app *App) {
		sub := New()
		sub.Get("/profile", func(c Ctx) error { return c.SendString("profile") })
		app.Use("/account", sub)
	}
	// The same three requests go to a static and to a parametric endpoint: a match,
	// a miss (404) and a method the matched path has no endpoint for (405 listing
	// the methods it does have).
	static := func(app *App) {
		app.Get("/users", func(c Ctx) error { return c.SendString("users") })
		app.Post("/users", func(c Ctx) error { return c.SendString("created") })
	}
	parametric := func(app *App) {
		app.Get("/user/keys/:id", param("id"))
		app.Post("/user/keys/:id", func(c Ctx) error { return c.SendString("post") })
	}
	manyMethods := append([]string{}, DefaultMethods...)
	for i := 0; len(manyMethods) <= 64; i++ {
		manyMethods = append(manyMethods, "X"+string(rune('A'+i%26))+string(rune('A'+i/26)))
	}
	for _, tc := range []struct {
		register func(app *App)
		cfg      Config // SkipUnmatchedRoutes is always set
		name     string
		requests []routerReq
		custom   bool // served with a custom context
		bare     bool // without the pass-through middleware
	}{
		{name: "static_match", register: static, requests: []routerReq{{path: "/users", status: StatusOK, body: "users"}}},
		{name: "static_not_found", register: static, requests: []routerReq{{path: "/does/not/exist", status: StatusNotFound}}},
		{
			name: "static_method_not_allowed", register: static,
			requests: []routerReq{{method: MethodDelete, path: "/users", status: StatusMethodNotAllowed, allow: []string{MethodGet, MethodPost}}},
		},
		{
			name: "parametric_match", register: parametric,
			requests: []routerReq{{path: "/user/keys/1337", status: StatusOK, body: "1337"}},
		},
		{
			name: "parametric_not_found", register: parametric,
			requests: []routerReq{{path: "/user/secrets/1337", status: StatusNotFound}},
		},
		{
			name: "parametric_method_not_allowed", register: parametric,
			requests: []routerReq{
				{method: MethodDelete, path: "/user/keys/1337", status: StatusMethodNotAllowed, allow: []string{MethodGet, MethodPost}},
			},
		},
		{
			// enabled without middleware, so the lookahead is gated off
			name: "NoMiddlewareGate", bare: true,
			register: func(app *App) { app.Get("/ping", func(c Ctx) error { return c.SendString("pong") }) },
			requests: []routerReq{
				{path: "/ping", status: StatusOK},
				{path: "/missing", status: StatusNotFound},
				{method: MethodPost, path: "/ping", status: StatusMethodNotAllowed},
			},
		},
		{
			// the emitSkip path where the configured error handler itself returns an error (-> 500)
			name: "CustomErrorHandler", cfg: Config{ErrorHandler: func(_ Ctx, _ error) error { return errors.New("boom") }},
			register: func(app *App) { app.Get("/ok", ok) },
			requests: []routerReq{{path: "/nope", status: StatusInternalServerError}},
		},
		{
			// the 500 fallback in emitSkipCustom
			name: "CustomCtxFailingErrorHandler", custom: true,
			cfg:      Config{ErrorHandler: func(Ctx, error) error { return errors.New("fail") }},
			register: func(app *App) { app.Get("/users", func(c Ctx) error { return c.SendString("users") }) },
			requests: []routerReq{{path: "/nope", status: StatusInternalServerError}},
		},
		{
			// the 405 path where the path matches a static endpoint of one method and a
			// parametric endpoint of another, so both Allow entries must be reported
			name: "CrossMethodMixed",
			register: func(app *App) {
				app.Get("/thing/:id", func(c Ctx) error { return c.SendString("get") })  // parametric
				app.Post("/thing/fixed", func(c Ctx) error { return c.SendString("p") }) // static
			},
			requests: []routerReq{
				{method: MethodDelete, path: "/thing/fixed", status: StatusMethodNotAllowed, allow: []string{MethodGet, MethodPost}},
			},
		},
		{
			// the tier-2 fallback to the global (bucket 0) candidate list when the request's prefix bucket is absent
			name: "Bucket0Fallback", register: func(app *App) { app.Get("/:id", param("id")) },
			requests: []routerReq{{path: "/abc", status: StatusOK, body: "abc"}},
		},
		{
			// next()'s skip-before guard: a static endpoint sits before the parametric endpoint
			// in the same bucket, so firstMatchIndex > 0 and the endpoints before it are skipped
			name: "SkipBefore",
			register: func(app *App) {
				app.Get("/key/aaa", func(c Ctx) error { return c.SendString("static") })
				app.Get("/key/:id", param("id"))
			},
			requests: []routerReq{{path: "/key/123", status: StatusOK, body: "123"}},
		},
		{
			name:     "OptionalParam",
			register: func(app *App) { app.Get("/opt/:v?", func(c Ctx) error { return c.SendString("opt:" + c.Params("v")) }) },
			requests: []routerReq{{path: "/opt", status: StatusOK}, {path: "/opt/x", status: StatusOK}, {path: "/other", status: StatusNotFound}},
		},
		{
			name: "Constraint", register: func(app *App) { app.Get("/num/:id<int>", param("id")) },
			requests: []routerReq{{path: "/num/5", status: StatusOK}, {path: "/num/abc", status: StatusNotFound}},
		},
		{
			name:     "Root",
			register: func(app *App) { app.Get("/", func(c Ctx) error { return c.SendString("root") }) },
			requests: []routerReq{{path: "/", status: StatusOK}},
		},
		{
			name:     "Star",
			register: func(app *App) { app.Get("/*", func(c Ctx) error { return c.SendString("star") }) },
			requests: []routerReq{{path: "/anything/here", status: StatusOK, body: "star"}},
		},
		{
			name:     "AutoHead_static",
			register: func(app *App) { app.Get("/page", func(c Ctx) error { return c.SendString("page") }) },
			requests: []routerReq{{method: MethodHead, path: "/page", status: StatusOK}},
		},
		{
			name: "AutoHead_param", register: func(app *App) { app.Get("/h/:id", param("id")) },
			requests: []routerReq{{method: MethodHead, path: "/h/1", status: StatusOK}},
		},
		{
			name: "Mount_match", register: mountProfile,
			requests: []routerReq{{path: "/account/profile", status: StatusOK, body: "profile"}},
		},
		{name: "Mount_miss", register: mountProfile, requests: []routerReq{{path: "/account/missing", status: StatusNotFound}}},
		{
			name: "case_sensitive", cfg: Config{CaseSensitive: true}, register: func(app *App) { app.Get("/Foo", ok) },
			requests: []routerReq{{path: "/Foo", status: StatusOK}, {path: "/foo", status: StatusNotFound}},
		},
		{
			name: "strict_routing", cfg: Config{StrictRouting: true}, register: func(app *App) { app.Get("/bar", ok) },
			requests: []routerReq{{path: "/bar", status: StatusOK}, {path: "/bar/", status: StatusNotFound}},
		},
		{
			name: "non_strict_trailing_slash", register: func(app *App) { app.Get("/baz", ok) },
			requests: []routerReq{{path: "/baz/", status: StatusOK}},
		},
		{
			// a handler that rewrites the path and restarts routing re-resolves correctly (no stale lookahead index)
			name: "RestartRouting",
			register: func(app *App) {
				app.Get("/old", func(c Ctx) error {
					c.Path("/new")
					return c.RestartRouting()
				})
				app.Get("/new", func(c Ctx) error { return c.SendString("new") })
			},
			requests: []routerReq{{path: "/old", status: StatusOK, body: "new"}},
		},
		{
			// Path(override) invalidates the lookahead index even without RestartRouting
			name: "PathOverrideMidChain",
			register: func(app *App) {
				app.Use(func(c Ctx) error {
					if c.Path() == "/aaa/zzz" {
						c.Path("/bbb")
					}
					return c.Next()
				})
				app.Get("/bbb", func(c Ctx) error { return c.SendString("bbb") })
				app.Get("/aaa/static", func(c Ctx) error { return c.SendString("static") })
				app.Get("/aaa/:id", param("id"))
			},
			requests: []routerReq{{path: "/aaa/zzz", status: StatusOK, body: "bbb"}},
		},
		{
			// a skipped 404 under a mount prefix still resolves the sub-app's custom error handler
			name: "MountedErrorHandler",
			register: func(app *App) {
				sub := New(Config{ErrorHandler: func(c Ctx, _ error) error {
					return c.Status(StatusNotFound).SendString("sub-404")
				}})
				sub.Get("/ok", ok)
				app.Use("/sub", sub)
			},
			requests: []routerReq{{path: "/sub/missing", status: StatusNotFound, body: "sub-404"}},
		},
		{
			// the route.star half of hasParamUse: wildcard middleware writes c.values, so the endpoint re-matches
			name: "StarMiddlewareNoClobber",
			register: func(app *App) {
				app.Use("/*", func(c Ctx) error { return c.Next() })
				app.Get("/user/:id", param("id"))
			},
			requests: []routerReq{{path: "/user/42", status: StatusOK, body: "42"}},
		},
		{
			// the static index lookup by the decoded detection path
			name: "UnescapePath", cfg: Config{UnescapePath: true}, register: func(app *App) { app.Get("/some/path", ok) },
			requests: []routerReq{{path: "/some/p%61th", status: StatusOK}},
		},
		{
			// Method(...) clears the lookahead index: a middleware switches GET->POST mid-chain,
			// and the POST endpoint sits at a lower bucket index than the GET match, so a stale
			// firstMatchIndex would wrongly skip it
			name: "MethodOverride", bare: true,
			register: func(app *App) {
				app.Use(func(c Ctx) error {
					if c.Method() == MethodGet {
						c.Method(MethodPost)
					}
					return c.Next()
				})
				app.Get("/zzz/aaa", func(c Ctx) error { return c.SendString("get-static") })
				app.Get("/zzz/:id", func(c Ctx) error { return c.SendString("get-param") })
				app.Post("/zzz/:id", func(c Ctx) error { return c.SendString("post-param") })
			},
			requests: []routerReq{{path: "/zzz/123", status: StatusOK, body: "post-param"}},
		},
		{
			// a matched parametric endpoint returns correct multi-param values when next() reuses
			// the lookahead's params (no parametric middleware, so the fast path is taken)
			name: "ParamReuse",
			register: func(app *App) {
				app.Use("/api", func(c Ctx) error { return c.Next() }) // static-prefix middleware
				app.Get("/api/users/:uid/books/:bid", func(c Ctx) error {
					return c.SendString(c.Params("uid") + "," + c.Params("bid"))
				})
			},
			requests: []routerReq{{path: "/api/users/u7/books/b9", status: StatusOK, body: "u7,b9"}},
		},
		{
			// guards the param-reuse fast path: a parametric middleware runs before the endpoint
			// and writes into the shared param slot, so the fast path must be disabled and the
			// endpoint re-matched; if reuse fired incorrectly the endpoint would observe the
			// middleware's value
			name: "ParamMiddlewareNoClobber", bare: true,
			register: func(app *App) {
				// parametric middleware whose match writes param slot 0 as the first segment
				app.Use("/:seg", func(c Ctx) error { return c.Next() })
				app.Get("/foo/:name", param("name"))
			},
			requests: []routerReq{{path: "/foo/bar", status: StatusOK, body: "bar"}},
		},
		{
			// with more than 64 RequestMethods the fast path is disabled and routing still
			// returns correct results via the normal router
			name: "ManyMethods", cfg: Config{RequestMethods: manyMethods}, register: func(app *App) { app.Get("/ok", ok) },
			requests: []routerReq{{path: "/ok", status: StatusOK}, {path: "/nope", status: StatusNotFound}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			cfg.SkipUnmatchedRoutes = true
			newApp := New
			if tc.custom {
				newApp = newCustomApp
			}
			app := newApp(cfg)
			if !tc.bare {
				app.Use(func(c Ctx) error { return c.Next() })
			}
			tc.register(app)
			routerTestExpectAll(t, app, tc.requests...)
		})
	}
}

// Test_App_SkipUnmatchedRoutes_GroupMiddleware checks that group middleware is
// skipped for unmatched routes and runs for matched ones. The two checks share
// the called flag and must run in order, so they are not split into subtests.
func Test_App_SkipUnmatchedRoutes_GroupMiddleware(t *testing.T) {
	t.Parallel()
	app := New(Config{SkipUnmatchedRoutes: true})
	called := false
	g := app.Group("/g")
	g.Use(func(c Ctx) error { called = true; return c.Next() })
	g.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

	routerTestExpect(t, app, "/g/nope", StatusNotFound)
	require.False(t, called, "group middleware must not run for an unmatched route")

	routerTestExpect(t, app, "/g/ok", StatusOK)
	require.True(t, called, "group middleware must run for a matched route")
}

func Test_App_SkipUnmatchedRoutes_RebuildTree(t *testing.T) {
	t.Parallel()
	app := newSkipApp()
	app.Get("/first", func(c Ctx) error { return c.SendString("first") })

	routerTestExpect(t, app, "/second", StatusNotFound)

	app.Get("/second", func(c Ctx) error { return c.SendString("second") })
	app.RebuildTree()

	routerTestExpect(t, app, "/second", StatusOK)
}

// Test_App_SkipUnmatchedRoutes_Parity asserts that enabling SkipUnmatchedRoutes
// produces the same status codes and Allow headers as the default behavior.
func Test_App_SkipUnmatchedRoutes_Parity(t *testing.T) {
	t.Parallel()
	build := func(skip bool) *App {
		app := New(Config{SkipUnmatchedRoutes: skip})
		app.Use(func(c Ctx) error { return c.Next() })
		registerDummyRoutes(app)
		return app
	}
	off := build(false)
	on := build(true)

	cases := []struct {
		method string
		path   string
	}{
		{MethodGet, "/user/keys/1337"},
		{MethodGet, "/does/not/exist"},
		{MethodGet, "/repos/gofiber/fiber/stargazers"},
		{MethodPost, "/user/keys/1337"},
		{MethodDelete, "/user/keys/1337"},
		{MethodGet, "/"},
		{MethodGet, "/applications/client/tokens"},
		{MethodGet, "/user/repos"},
		{MethodHead, "/user/keys/1337"},
		{MethodOptions, "/user/keys/1337"},
		{MethodPost, "/"},
	}

	for _, tc := range cases {
		respOff := apptest.Do(t, off, apptest.Req(tc.method, tc.path))
		respOn := apptest.Do(t, on, apptest.Req(tc.method, tc.path))
		require.Equal(t, respOff.StatusCode, respOn.StatusCode, "%s %s", tc.method, tc.path)
		require.Equal(t, respOff.Header.Get(HeaderAllow), respOn.Header.Get(HeaderAllow), "%s %s allow", tc.method, tc.path)
		require.Equal(t, respOff.Header.Get(HeaderContentType), respOn.Header.Get(HeaderContentType), "%s %s content-type", tc.method, tc.path)
		bodyOff := apptest.Body(t, respOff)
		bodyOn := apptest.Body(t, respOn)
		require.Equal(t, bodyOff, bodyOn, "%s %s body", tc.method, tc.path)
	}
}

// Test_App_SkipUnmatchedRoutes_CustomCtx exercises the customRequestHandler path.
func Test_App_SkipUnmatchedRoutes_CustomCtx(t *testing.T) {
	t.Parallel()
	app := newCustomApp(Config{SkipUnmatchedRoutes: true})
	app.Use(func(c Ctx) error {
		if c.IsPreflight() {
			return c.SendStatus(StatusNoContent)
		}
		return c.Next()
	})
	app.Get("/users", func(c Ctx) error { return c.SendString("users") })
	app.Get("/user/keys/:id", func(c Ctx) error { return c.SendString("key") })

	for _, tc := range []struct {
		req    *http.Request
		name   string
		status int
	}{
		{name: "static_match", req: apptest.Req(MethodGet, "/users"), status: StatusOK},
		{name: "param_match", req: apptest.Req(MethodGet, "/user/keys/1"), status: StatusOK},
		{name: "not_found", req: apptest.Req(MethodGet, "/nope"), status: StatusNotFound},
		{name: "method_not_allowed", req: apptest.Req(MethodDelete, "/users"), status: StatusMethodNotAllowed},
		{
			// Preflight is exempt from the skip, so the middleware answers it instead of a 405.
			name: "cors_preflight",
			req: apptest.Req(MethodOptions, "/users",
				apptest.WithHeader(HeaderOrigin, "https://example.com"),
				apptest.WithHeader(HeaderAccessControlRequestMethod, MethodGet)),
			status: StatusNoContent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			routerTestExpect(t, app, tc.req, tc.status)
		})
	}
}

// Test_App_SkipUnmatchedRoutes_FlashCookieCleared pins the flash check running
// before the short-circuit: a skipped 404 must still clear the flash cookie.
func Test_App_SkipUnmatchedRoutes_FlashCookieCleared(t *testing.T) {
	t.Parallel()
	app := newSkipApp()
	app.Get("/set", func(c Ctx) error { return c.Redirect().With("k", "v").To("/target") })

	resp := apptest.Do(t, app, "/set")
	flash := resp.Header.Get(HeaderSetCookie)
	require.Contains(t, flash, FlashCookieName+"=")
	require.NoError(t, resp.Body.Close())

	req := apptest.Req(MethodGet, "/missing", apptest.WithHeader(HeaderCookie, strings.Split(flash, ";")[0]))
	resp = routerTestExpect(t, app, req, StatusNotFound)
	cleared := resp.Header.Get(HeaderSetCookie)
	require.Contains(t, cleared, FlashCookieName+"=")
	require.Contains(t, strings.ToLower(cleared), "max-age=0")
}

// Test_App_SkipUnmatchedRoutes_ErrorSentinels pins the errors handed to the
// error handler on the fast path (users match on them with errors.Is).
func Test_App_SkipUnmatchedRoutes_ErrorSentinels(t *testing.T) {
	t.Parallel()
	var got error
	app := New(Config{
		SkipUnmatchedRoutes: true,
		ErrorHandler: func(c Ctx, err error) error {
			got = err
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use(func(c Ctx) error { return c.Next() })
	app.Get("/ok", func(c Ctx) error { return c.SendString("ok") })

	routerTestExpect(t, app, "/nope", StatusNotFound)
	require.ErrorIs(t, got, ErrNotFound)

	routerTestExpect(t, app, apptest.Req(MethodPost, "/ok"), StatusMethodNotAllowed)
	require.ErrorIs(t, got, ErrMethodNotAllowed)
}

func Benchmark_SkipUnmatchedRoutes(b *testing.B) {
	scenarios := []struct {
		name        string
		method      string
		path        string
		middlewares int
	}{
		{"Unmatched", MethodGet, "/this/route/does/not/exist", 3},
		{"Matched", MethodGet, "/user/keys/1337", 1},
		{"405_Middleware", MethodPost, "/user/keys/1337", 3},
		{"405_NoMiddleware", MethodPost, "/user/keys/1337", 0},
	}
	for _, s := range scenarios {
		for _, skip := range []bool{false, true} {
			variant := map[bool]string{false: "without_skip", true: "with_skip"}[skip]
			b.Run(s.name+"/"+variant, func(b *testing.B) {
				app := New(Config{SkipUnmatchedRoutes: skip})
				for range s.middlewares {
					app.Use(func(c Ctx) error { return c.Next() })
				}
				registerDummyRoutes(app)
				appHandler, c := routerBenchHandler(app, s.method, s.path)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					appHandler(c)
				}
			})
		}
	}
}

func Benchmark_SkipUnmatchedRoutes_Deep(b *testing.B) {
	deepScenarios := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{"Matched_health_0param", MethodGet, "/health", StatusOK},
		{"Unmatched_Top_level", MethodGet, "/.env", StatusNotFound},
		{"Unmatched_Top_level_SBucket", MethodGet, "/heap", StatusNotFound},
		{"Unmatched_Top_level_SBucket_Diff_Method", MethodDelete, "/heap", StatusNotFound},
		{"Unmatched_Top_level_SBucket_Diff_Method2", MethodPost, "/heap", StatusNotFound},
		{"Unmatched_Top_level_SBucket_WrongMethod", MethodPost, "/health", StatusMethodNotAllowed},
		{"Matched_1param", MethodGet, "/api/v1/matters/m1", StatusOK},
		{"Matched_3param", MethodGet, "/api/v1/matters/m1/staff/s1/docs/d1", StatusOK},
		{"Matched_4param", MethodGet, "/api/v1/matters/m1/staff/s1/docs/d1/versions/v1", StatusOK},
		{"Matched_5param", MethodGet, "/api/v1/matters/m1/staff/s1/docs/d1/versions/v1/notes/n1", StatusOK},
		{"MethodNotAllowed_deep", MethodPost, "/api/v1/matters/m1/staff/s1/primary", StatusMethodNotAllowed},
	}
	for _, s := range deepScenarios {
		for _, skip := range []bool{false, true} {
			variant := map[bool]string{false: "without_skip", true: "with_skip"}[skip]
			b.Run(s.name+"/"+variant, func(b *testing.B) {
				app := New(Config{SkipUnmatchedRoutes: skip})
				h := func(_ Ctx) error { return nil }
				mw := func(c Ctx) error { return c.Next() }
				app.Get("/health", h)
				api := app.Group("/api/v1", mw)
				matters := api.Group("/matters", mw)
				matters.Get("/:id", h)
				matters.Put("/:matterId/staff/:staffId/primary", h)
				api.Put("/emails/threads/:threadID/pin/:matterID", h)
				api.Post("/conflicts/:entityType/:entityID/refresh", h)
				matters.Get("/:matterId/staff/:staffId/docs/:docId", h)
				matters.Get("/:matterId/staff/:staffId/docs/:docId/versions/:versionId", h)
				matters.Get("/:matterId/staff/:staffId/docs/:docId/versions/:versionId/notes/:noteId", h)
				appHandler, c := routerBenchHandler(app, s.method, s.path)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					appHandler(c)
				}
			})
		}
	}
}

// Benchmark_Router_HandlerCustom exercises the custom-context request path
// (nextCustom) over a deep bucket, where the per-iteration accessor hoisting
// matters most.
func Benchmark_Router_HandlerCustom(b *testing.B) {
	app := newCustomApp()
	registerDummyRoutes(app)
	appHandler, c := routerBenchHandler(app, MethodGet, "/user/keys/1337")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		appHandler(c)
	}
}

// Benchmark_Router_HandlerCustom_NotFound exercises the custom-context 404/405
// cross-method fallback scan (nextCustom), where accessor hoisting matters.
func Benchmark_Router_HandlerCustom_NotFound(b *testing.B) {
	app := newCustomApp()
	registerDummyRoutes(app)
	appHandler, c := routerBenchHandler(app, MethodGet, "/this/route/does/not/exist")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		appHandler(c)
	}
}

// Test_Route_PrefixFilter_Differential generatively proves the filters App.next
// applies before Route.match are transparent: for every generated pattern and
// path, a route they reject must be a route Route.match, with its own filters
// stood down, would have rejected anyway. The filters only ever get to skip
// work, never to change an outcome, so a false reject here is a routing bug.
func Test_Route_PrefixFilter_Differential(t *testing.T) {
	t.Parallel()
	segments := []string{
		"/api", "/foo/", "/:a", "/:b?", "/*", "/+", "/:a-:b", "/:f.:e?",
		":tail", "/::c", "/:x:y", "/name\\:verb", "/:p/fixed", "/",
		"/verylongconstantsegment",
		// Escaped forms matter: register derives star/root from the
		// escape-stripped path but Params from the escaped one, so these are
		// the shapes where the two disagree.
		"/\\*", "/\\+", "/\\:a", "/a\\*b",
	}
	patterns := make([]string, 0, len(segments)*(len(segments)+1))
	for _, s1 := range segments {
		patterns = append(patterns, s1)
		for _, s2 := range segments {
			patterns = append(patterns, s1+s2)
		}
	}

	pieces := []string{
		"", "/a", "/a/b", "/a-b", "/a.b", "/x/y-z", "/api", "/api/", "/foo",
		"/foo/", "/fixed", "/name:verb", "/verylongconstantsegment", "/",
		"/*", "/+", "/a*b",
	}
	paths := make([]string, 0, len(pieces)*len(pieces))
	for _, p1 := range pieces {
		for _, p2 := range pieces {
			paths = append(paths, p1+p2)
		}
	}

	for _, use := range []bool{false, true} {
		for _, route := range registerFilterRoutes(t, patterns, use) {
			for _, path := range paths {
				var params [maxParams]string
				if !route.match(path, path, &params, 0) {
					continue
				}
				if routeFilterRejects(route, path) {
					t.Fatalf("scan filter rejected a matching route: pattern %q, path %q, use %v",
						route.Path, path, use)
				}
			}
		}
	}
}

// newScanHead returns the scanHead a filtered bucket holds for r.
func newScanHead(r *Route) scanHead {
	var h scanHead
	h.init(r)
	return h
}

// routeFilterRejects mirrors what the scan loops apply before calling
// Route.match: the leading-byte filter App.next, App.nextCustom and resolveSkip
// test on a Route, and the scanHead routeScan.skip tests for a filtered bucket
// -- two words of the prefix, the slash count and the probe. The tests below
// assert that neither rejects a route Route.match accepts.
func routeFilterRejects(route *Route, path string) bool {
	head := pathHeadWord(path)
	if route.prefixRejects(head) {
		return true
	}
	var scan routeScan
	scan.init(path, head, strings.Count(path, "/"))
	h := newScanHead(route)
	return h.rejects(scan.head, scan.head2, scan.slash) ||
		h.probeLen != 0 && scan.probes.headRejects(&h, path)
}

// registerFilterRoutes registers patterns on a real App and returns the routes
// the router will actually serve, tree built and prefix filters computed.
//
// The differential tests must not hand-build Route literals: register derives
// path, Params, star and root from three different strings (raw, prettified and
// escape-stripped), and a filter bug that only shows up when those disagree —
// as the escaped-star case did — is invisible to a stand-in built from one.
//
//nolint:revive // flag-parameter: the two registration modes are the point of the helper
func registerFilterRoutes(t *testing.T, patterns []string, use bool) []*Route {
	t.Helper()

	app := New()
	handler := func(c Ctx) error { return c.Next() }
	for _, pattern := range patterns {
		func() {
			// A few generated shapes are not registrable; they are not the
			// subject of this test, so skip them rather than fail.
			defer func() {
				//nolint:errcheck // the panic value is irrelevant; skipping the pattern is the point
				recover()
			}()
			if use {
				app.Use(pattern, handler)
			} else {
				app.Get(pattern, handler)
			}
		}()
	}
	_ = app.RebuildTree()

	method := app.methodInt(MethodGet)
	require.NotEqual(t, -1, method)
	routes := app.stack[method]
	require.NotEmpty(t, routes, "no routes registered")
	return routes
}

// Test_Route_PrefixFilter_Fixture proves the leading-byte filter agrees with
// Route.match across the exhaustive path-matching fixture, which covers far
// more pattern shapes than the generated set above.
func Test_Route_PrefixFilter_Fixture(t *testing.T) {
	t.Parallel()
	patterns := make([]string, 0, len(routeTestCases))
	for _, testCollection := range routeTestCases {
		patterns = append(patterns, testCollection.pattern)
	}

	for _, use := range []bool{false, true} {
		byPath := make(map[string]*Route)
		for _, route := range registerFilterRoutes(t, patterns, use) {
			byPath[route.Path] = route
		}

		for _, testCollection := range routeTestCases {
			route, ok := byPath[testCollection.pattern]
			if !ok {
				continue
			}
			for _, c := range testCollection.testCases {
				if c.partialCheck != use {
					continue
				}
				// Route.match is the oracle, not the fixture's expectation:
				// the fixture records what getMatch returns, while match wraps
				// it in the root/star/exact branches the filters stay behind.
				// A count of 0 stands match's own filters down.
				var params [maxParams]string
				if !route.match(c.url, c.url, &params, 0) {
					continue
				}
				require.False(t, routeFilterRejects(route, c.url),
					"scan filter rejected a matching route: '%s', url: '%s'", testCollection.pattern, c.url)
			}
		}
	}
}

// Test_Route_PrefixFilter_Rejects proves the filter is actually doing work:
// routes whose leading constant bytes differ from the request must be skipped
// without Route.match ever being consulted.
func Test_Route_PrefixFilter_Rejects(t *testing.T) {
	t.Parallel()
	route := func(pattern string) *Route {
		routes := registerFilterRoutes(t, []string{pattern}, false)
		require.Len(t, routes, 1)
		return routes[0]
	}
	for _, tc := range []struct {
		pattern, path string
		rejects       bool // routeFilterRejects, which also holds the scan head's two words
		prefixRejects bool // the prefix filter alone, which an unfiltered bucket relies on
	}{
		{pattern: "/user/subscriptions/:owner", path: "/user/keys/1337", rejects: true, prefixRejects: true},
		{pattern: "/user/keys/:id", path: "/user/emails", rejects: true, prefixRejects: true},
		{pattern: "/repos/:owner", path: "/user/keys", rejects: true, prefixRejects: true},
		{pattern: "/user/keys/:id", path: "/user/keys/1337"},
		// Past the first word only the scan head's second word tells these apart.
		{pattern: "/api/v1/users/:id", path: "/api/v1/teams/7", rejects: true},
		// wildcards and leading parameters constrain nothing, so they must not filter
		{pattern: "/*", path: "/anything/at/all"},
		{pattern: "/:name", path: "/anything"},
	} {
		r, msg := route(tc.pattern), tc.pattern+" vs "+tc.path
		require.Equal(t, tc.rejects, routeFilterRejects(r, tc.path), msg)
		// on its own, which routeFilterRejects would hide behind the scan head's first word
		require.Equal(t, tc.prefixRejects, r.prefixRejects(pathHeadWord(tc.path)), msg+" (prefix)")
	}
}

func Test_Route_PrefixFilter_EscapedStar(t *testing.T) {
	t.Parallel()
	require.True(t, RoutePatternMatch("/*", `/\*`))
	require.False(t, RoutePatternMatch("/anything", `/\*`))
	require.True(t, RoutePatternMatch("/anything", "/*"))

	// An escaped star is a literal "*", for a plain route and a Use prefix alike.
	for _, caseSensitive := range []bool{false, true} {
		for _, use := range []bool{false, true} {
			t.Run(fmt.Sprintf("caseSensitive=%v/use=%v", caseSensitive, use), func(t *testing.T) {
				t.Parallel()
				app := New(Config{CaseSensitive: caseSensitive})
				handler := func(c Ctx) error { return c.SendString("ok") }
				if use {
					app.Use(`/\*`, handler)
				} else {
					app.Get(`/\*`, handler)
				}
				apptest.Expect(t, app, "/*", StatusOK)
				for _, path := range []string{"/foo", "/bar/baz", "/", "/anything", "/a/b/c"} {
					apptest.Expect(t, app, path, StatusNotFound)
				}
			})
		}
	}
}

// Benchmark_Router_GitHub_API_Serial routes the whole fixture corpus once per
// iteration on a single goroutine.
//
// The RunParallel variants above are the right shape for measuring contention
// but are dominated by scheduler noise on small machines (±40% run to run
// observed on 4 cores), which buries the few-percent deltas that route-scan
// changes produce. This one is deterministic, so it is the benchmark to use
// when comparing scan-side changes across builds, and it exercises the case
// those changes target: a large route set where most candidates are rejected.
func Benchmark_Router_GitHub_API_Serial(b *testing.B) {
	app := routerBenchStarted()

	// Fail loudly if the corpus stops matching, rather than silently
	// benchmarking a router that answers 404 for everything.
	c := &fasthttp.RequestCtx{}
	for _, route := range routesFixture.TestRoutes {
		c.Request.Header.SetMethod(route.Method)
		c.URI().SetPath(route.Path)
		ctx := acquireDefaultCtxForRouterBenchmark(b, app, c)
		match, err := app.next(ctx)
		app.ReleaseCtx(ctx)
		require.NoError(b, err, "route: %s %s", route.Method, route.Path)
		require.True(b, match, "route: %s %s", route.Method, route.Path)
	}

	b.ReportAllocs()
	for b.Loop() {
		for i := range routesFixture.TestRoutes {
			c.Request.Header.SetMethod(routesFixture.TestRoutes[i].Method)
			c.URI().SetPath(routesFixture.TestRoutes[i].Path)
			ctx := acquireDefaultCtxForRouterBenchmark(b, app, c)
			//nolint:errcheck // verified above; the loop measures the scan
			_, _ = app.next(ctx)
			app.ReleaseCtx(ctx)
		}
	}
}

// referenceEndpoint resolves a request the slow, obvious way: walk the method's
// route stack in registration order and return the first endpoint whose
// Route.match accepts, with every optimization the router layers on top of that
// deliberately switched off.
//
//   - the leading-byte filter is bypassed by not applying it
//   - the slash-count quick-reject is bypassed by passing an unknown count (0)
//   - the "/const/:param" specialization is bypassed on a parser copy
//
// Those three may only ever skip work, so the router has to land on the same
// route with the same parameters as this does.
//
// Bucket selection is bypassed too -- this walks app.stack rather than a tree
// bucket -- but it does not belong on that list, because it is not purely
// work-skipping: a route whose first constant segment is at least
// maxDetectionPaths bytes is filed under a hashed bucket only, while a request
// shorter than that hashes to 0 and scans globals, so the route is invisible to
// the router but visible here. `/a/*` against `/a` is such a pair. That is
// long-standing router behavior, not something this file's subject introduced,
// so the corpus stays clear of the shape rather than asserting against it.
//
//nolint:gocritic // unnamedResult: the pair is documented above and named results trip nonamedreturns
func referenceEndpoint(app *App, method int, detectionPath, path string) (*Route, [maxParams]string) {
	for _, route := range app.stack[method] {
		if route.use || route.mount {
			continue
		}

		// Route is a value copy; routeParser is a value field and segs is only
		// read, so clearing the specialization here cannot affect the real one.
		ref := *route
		ref.routeParser.constParam = false

		var got [maxParams]string
		if ref.match(detectionPath, path, &got, 0) {
			return route, got
		}
	}
	return nil, [maxParams]string{}
}

// Test_Router_ScanMatchesReference is the end-to-end guard for the router's
// candidate scan. Across every routing-relevant config combination it drives
// real requests through App.next and requires the resulting route and captured
// parameters to equal what an unoptimized linear scan of the route stack picks.
//
// This is the property every scan-side optimization has to preserve, and it
// covers them together rather than one at a time: bucket selection, the
// leading-byte filter, the slash-count bounds and the "/const/:param"
// specialization all have to agree with the obvious implementation. It replaces
// a golden-file comparison, so it needs no regeneration when routes are added.
func Test_Router_ScanMatchesReference(t *testing.T) {
	t.Parallel()
	patterns := []string{
		"/", "/health", "/health/live", "/api", "/api/v1/status",
		"/user/keys/:id", "/user/emails", "/user/following/:user",
		"/repos/:owner/:repo", "/repos/:owner/:repo/issues",
		"/repos/:owner/:repo/issues/:number", "/repos/:owner/:repo/stargazers",
		"/api/v1/:resource", "/api/v1/:resource/:id", "/api/v1/:resource/:id/edit",
		"/files/*", "/assets/+", "/mixed/:a-:b", "/dotted/:a.:b",
		"/opt/:a?", "/opt/:a/:b?", "/num/:id<int>", "/word/:name<alpha>",
		// Optional param whose constant does not end in '/', so the constant
		// carries no optional slash: the one shape where the specialization
		// gate's own optional-param check is what keeps it out.
		"/optx:a?",
		`/esc/\:literal`, `/esc/\*`, "/deep/:a/:b/:c/:d/:e",
		"/CaseSensitive/Path", "/trailing/slash/", "/x/:y/fixedEnd",
		// Registered last so they act as catch-alls: a filter that wrongly
		// constrains a star route shows up here as an unexpected miss.
		`/\*`, "/*",
	}

	// No empty path: httptest.NewRequest rejects it, and fasthttp never yields
	// one anyway. Route.match's empty-detection-path behavior is covered by
	// Test_Route_PrefixFilter_Differential, which calls match directly.
	paths := []string{
		"/", "/health", "/health/", "/health/live", "/HEALTH",
		"/api", "/api/v1/status", "/api/v1/things", "/api/v1/things/7",
		"/api/v1/things/7/edit", "/user/keys/1337", "/user/emails",
		"/user/following/octocat", "/repos/a/b", "/repos/a/b/issues",
		"/repos/a/b/issues/42", "/repos/a/b/stargazers", "/repos/a",
		"/files/any/deep/path", "/assets/x/y", "/mixed/p-q", "/dotted/p.q",
		"/opt", "/opt/", "/opt/one", "/opt/one/two", "/num/123", "/num/abc",
		"/optx", "/optxy", "/optx/z",
		"/word/abc", "/word/123", "/esc/:literal", "/esc/*", "/esc/anything",
		"/deep/1/2/3/4/5", "/CaseSensitive/Path", "/casesensitive/path",
		"/trailing/slash", "/trailing/slash/", "/x/y/fixedEnd", "/x/y/z/fixedEnd",
		"/nope", "/a/b/c/d/e/f/g",
	}

	for _, caseSensitive := range []bool{false, true} {
		for _, strictRouting := range []bool{false, true} {
			for _, skipUnmatched := range []bool{false, true} {
				for _, withMiddleware := range []bool{false, true} {
					for _, filtered := range []bool{false, true} {
						cfg := Config{
							CaseSensitive:       caseSensitive,
							StrictRouting:       strictRouting,
							SkipUnmatchedRoutes: skipUnmatched,
						}
						name := fmt.Sprintf("cs=%v/sr=%v/skip=%v/mw=%v/filtered=%v",
							caseSensitive, strictRouting, skipUnmatched, withMiddleware, filtered)

						t.Run(name, func(t *testing.T) {
							t.Parallel()
							assertScanMatchesReference(t, &cfg, withMiddleware, filtered, patterns, paths)
						})
					}
				}
			}
		}
	}
}

//nolint:revive // flag-parameter: withMiddleware and filtered select the app shape under test
func assertScanMatchesReference(t *testing.T, cfg *Config, withMiddleware, filtered bool, patterns, paths []string) {
	t.Helper()

	app := New(*cfg)
	if withMiddleware {
		// Middleware changes which scan path App.next takes, and it is what
		// enables the SkipUnmatchedRoutes lookahead, so both shapes matter.
		app.Use(func(c Ctx) error { return c.Next() })
	}
	if filtered {
		// Global routes land in every bucket, so filterMinBucket of them
		// carry each bucket past the threshold, and the scan then walks it
		// through routeScan.skip instead of loading every route. Registered
		// first, every request is scanned past them, and they match no path
		// of the corpus.
		for i := range filterMinBucket {
			app.Get("/:filler/zzfill"+strconv.Itoa(i), func(c Ctx) error { return c.Next() })
		}
	}

	// Requests run through the real handler rather than App.next directly, so
	// the lookahead is live: resolveSkip is reached only from requestHandler,
	// and firstMatchIndex is -1 straight out of Reset. Driving next() by hand
	// left the SkipUnmatchedRoutes axis inert -- eight of the sixteen configs
	// re-ran the other eight, and the prefix-filter rejects in resolveSkip and
	// the firstMatchIndex fast path in next() went untouched, which are exactly
	// the paths where a disagreement picks a wrong route rather than a slow one.
	var (
		hitRoute       *Route
		hitParams      [maxParams]string
		hitLookahead   int
		sawLookahead   bool
		registeredHits int
	)
	//nolint:unparam // must satisfy fiber.Handler; the recording is the point
	handler := func(c Ctx) error {
		defaultCtx, ok := c.(*DefaultCtx)
		require.True(t, ok)
		hitRoute = c.Route()
		hitParams = defaultCtx.values
		hitLookahead = defaultCtx.firstMatchIndex
		if hitLookahead >= 0 {
			sawLookahead = true
		}
		return nil
	}
	for _, pattern := range patterns {
		func() {
			//nolint:errcheck // an unregistrable pattern is not this test's subject
			defer func() { recover() }()
			app.Get(pattern, handler)
			registeredHits++
		}()
	}
	require.Equal(t, len(patterns), registeredHits,
		"every pattern in the corpus must register; a silently dropped one shrinks the test")
	app.startupProcess()

	method := app.methodInt(MethodGet)
	require.NotEqual(t, -1, method)
	if _, filter := app.treeIndex[method].lookup(0); filtered {
		require.NotNil(t, filter, "the fillers must carry the buckets past filterMinBucket")
	} else {
		require.Nil(t, filter, "the corpus alone must stay below filterMinBucket")
	}

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)

	for _, path := range paths {
		// The reference needs the detection path the router will derive.
		// Normalization is shared machinery, not something the scan optimizes,
		// so reading it from a real ctx is not begging the question.
		fctx.URI().SetPath(path)
		ctx, ok := app.AcquireCtx(fctx).(*DefaultCtx)
		require.True(t, ok)
		detectionPath := app.toString(ctx.detectionPath)
		routePath := app.toString(ctx.path)
		app.ReleaseCtx(ctx)

		wantRoute, wantParams := referenceEndpoint(app, method, detectionPath, routePath)

		hitRoute, hitParams, hitLookahead = nil, [maxParams]string{}, -1
		resp := apptest.Do(t, app, path)

		if wantRoute == nil {
			require.Nil(t, hitRoute,
				"router matched %q via %q but the reference scan found nothing",
				path, routeLabel(hitRoute))
			require.NotEqual(t, StatusOK, resp.StatusCode, "unmatched path %q should not answer 200", path)
			continue
		}

		require.NotNil(t, hitRoute,
			"reference scan matched %q via %q but the router ran no handler",
			path, wantRoute.Path)
		require.Same(t, wantRoute, hitRoute,
			"path %q: router chose %q, reference scan chose %q",
			path, routeLabel(hitRoute), wantRoute.Path)
		require.Equal(t, wantParams[:len(wantRoute.Params)], hitParams[:len(wantRoute.Params)],
			"path %q via %q: captured parameters diverged", path, wantRoute.Path)
	}

	// Pin the axis as live. Without this the test can quietly stop exercising
	// the lookahead again and still pass every assertion above.
	if withMiddleware && cfg.SkipUnmatchedRoutes {
		require.True(t, sawLookahead,
			"SkipUnmatchedRoutes lookahead never resolved an endpoint; the axis is inert again")
	}
}

// routeLabel names a route for a failure message, tolerating nil.
func routeLabel(r *Route) string {
	if r == nil {
		return "<none>"
	}
	return r.Path
}

// Test_ResolveSkip_StaticSlashGate covers the '/'-count gate that lets
// resolveSkip skip hashing the detection path for the static endpoint index.
// The gate is a pure filter over an exact-match lookup, so a wrong bit turns a
// matching static route into a 404 -- silently, and only when
// SkipUnmatchedRoutes and middleware are both on and some route has a
// parameter, which is what makes the '/' census run in the first place.
//
// The routes here deliberately include no wildcard or root fallback: with one
// present, tier 2 would rescue a botched tier 1 and hide the bug.
func Test_ResolveSkip_StaticSlashGate(t *testing.T) {
	t.Parallel()
	statics := []string{"/health", "/api/status", "/a/b/c/d", "/one/two/three/four/five/six"}

	app := New(Config{SkipUnmatchedRoutes: true})
	app.Use(func(c Ctx) error { return c.Next() })
	for _, path := range statics {
		app.Get(path, func(c Ctx) error { return c.SendString("ok") })
	}
	// A parametric route at a depth none of the statics share, so the slash
	// census actually runs and the static bitmask is what decides tier 1.
	app.Get("/params/:a/:b/:c/:d/:e/:f/:g", func(c Ctx) error { return c.SendString("param") })

	for _, path := range statics {
		apptest.Expect(t, app, path, StatusOK)
	}

	apptest.Expect(t, app, "/params/1/2/3/4/5/6/7", StatusOK)

	// Paths that share a '/' count with a registered static but nothing else,
	// so a gate that is too permissive is not mistaken for a correct one.
	for _, path := range []string{"/missing", "/api/missing", "/a/b/c/x", "/nope/nope"} {
		apptest.Expect(t, app, path, StatusNotFound)
	}
}

// Test_Route_PrefixFilter_NotStale proves every path that can produce or alter
// a Route leaves its leading-byte filter consistent with the route's current
// path, params and parser.
//
// The filter is computed where routes are built rather than in buildTree,
// because buildTree writes to routes that in-flight requests are reading. That
// trades a torn-read hazard for a staleness hazard, and this is what closes the
// second one: it recomputes the filter for every registered route and requires
// it to equal what is stored. A future in-place rewrite of a route's path that
// forgets to refresh the filter fails here.
func Test_Route_PrefixFilter_NotStale(t *testing.T) {
	t.Parallel()
	assertFresh := func(t *testing.T, app *App, stage string) {
		t.Helper()
		checked := 0
		for method, routes := range app.stack {
			for _, route := range routes {
				wantWord, wantMask := computePrefixFilter(route)
				require.Equal(t, wantWord, route.prefix,
					"%s: stale prefix on %s %q", stage, app.config.RequestMethods[method], route.Path)
				require.Equal(t, wantMask, route.prefixMask,
					"%s: stale prefixMask on %s %q", stage, app.config.RequestMethods[method], route.Path)
				checked++
			}
		}
		require.NotZero(t, checked, "%s: no routes to check", stage)
	}

	handler := func(c Ctx) error { return c.Next() }

	// register, groups, and the auto-HEAD copyRoute path
	app := New()
	app.Get("/plain", handler)
	app.Get("/plain/:id", handler)
	app.Use("/mw", handler)
	app.Get("/*", handler)
	app.Get(`/\*`, handler)
	group := app.Group("/grp", handler)
	group.Get("/:x", handler)
	group.Get("/nested/:y/end", handler)

	// mounted sub-app: addPrefixToRoute rewrites path, parser, star and root
	// in place, which is the only route mutation outside construction
	sub := New()
	sub.Get("/inner/:y", handler)
	sub.Get("/*", handler)
	sub.Use("/subuse", handler)
	app.Use("/mounted", sub)

	app.startupProcess()
	assertFresh(t, app, "after startup")

	// routes added and removed on a live app, the flow RebuildTree exists for
	app.Get("/late/:id", handler)
	app.RemoveRoute("/plain")
	_ = app.RebuildTree()
	assertFresh(t, app, "after runtime registration and RebuildTree")

	// a second app with the opposite config, since the filter is derived from
	// the case-folded and slash-trimmed forms
	strict := New(Config{CaseSensitive: true, StrictRouting: true})
	strict.Get("/Mixed/Case/:Id", handler)
	strict.Get("/trailing/", handler)
	strict.Use("/Pre", handler)
	strict.startupProcess()
	assertFresh(t, strict, "case-sensitive strict app")
}

// Test_RouteTree_SlotSpreadsAcrossLargeTables pins that the probe index is
// taken from the high bits of the multiply, so it can address the whole table.
//
// Fibonacci hashing only works if the index comes off the top: the multiply
// moves the entropy of a tree hash's low bytes upward, and a fixed shift bounds
// the result by whatever it exposes regardless of how large the table is. With
// a fixed >>16 every probe started below slot 65536, so a table sized past that
// had an unreachable upper half and the lower half degraded into a linear scan
// -- in exactly the large-route-set case the index exists to speed up.
func Test_RouteTree_SlotSpreadsAcrossLargeTables(t *testing.T) {
	t.Parallel()
	// Detection paths always begin with '/', so the first hash byte is fixed
	// and the other two vary: 65536 distinct prefixes, sizing the table to
	// 131072 slots -- just past where a 16-bit index runs out.
	buckets := make(map[int][]*Route, 1<<16)
	for b1 := range 256 {
		for b2 := range 256 {
			buckets[int('/')<<16|b1<<8|b2] = make([]*Route, 1)
		}
	}

	tree := buildRouteTree(buckets)
	require.Len(t, tree.hashes, 1<<17)

	var maxSlot uint32
	for hash := range buckets {
		if s := tree.slot(hash); s > maxSlot {
			maxSlot = s
		}
		require.Less(t, tree.slot(hash), uint32(len(tree.hashes)), "slot out of range for hash %#x", hash)
	}
	require.Greater(t, maxSlot, uint32(len(tree.hashes)/2), "probe starts never reach the upper half of the table")

	// The table still resolves every key to its own bucket.
	for hash, want := range buckets {
		got, _ := tree.lookup(hash)
		if len(got) != 1 || &got[0] != &want[0] {
			t.Fatalf("lookup returned the wrong bucket for hash %#x", hash)
		}
	}
}

// Test_RebuildTree_PreservesPublishedBuckets ensures rebuilding cannot rewrite
// the backing arrays that an in-flight request may still be scanning.
func Test_RebuildTree_PreservesPublishedBuckets(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/aa/first", testEmptyHandler)
	app.Get("/aa/removed", testEmptyHandler)
	app.Get("/aa/last", testEmptyHandler)
	app.startupProcess()

	published, _ := routerTestBucket(app, MethodGet, "/aa")
	require.Equal(t, []string{"/aa/first", "/aa/removed", "/aa/last"}, routeTreePaths(published))
	want := append([]*Route(nil), published...)

	app.RemoveRoute("/aa/removed", MethodGet)
	app.RebuildTree()

	// The bucket the previous build published is what an in-flight request may
	// still be scanning, so the rebuild must not have written through it.
	require.Equal(t, want, published)

	// The rebuild must still take effect: the new bucket drops the route.
	rebuilt, _ := routerTestBucket(app, MethodGet, "/aa")
	require.Equal(t, []string{"/aa/first", "/aa/last"}, routeTreePaths(rebuilt))
}

// Test_BuildTree_BucketsAreExactlyFull guards the invariant the shared bucket
// arena rests on: every bucket's cap is exactly what buildTree appends to it.
// Slack would leave a bucket able to grow into the neighboring window, which
// is the cross-bucket write the fresh-arena rule exists to prevent.
func Test_BuildTree_BucketsAreExactlyFull(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/", testEmptyHandler)
	app.Get("/aa/first", testEmptyHandler)
	app.Get("/aa/second", testEmptyHandler)
	app.Get("/bb/only", testEmptyHandler)
	app.Post("/aa/first", testEmptyHandler)
	app.Get("/x", testEmptyHandler)
	app.startupProcess()

	for method, buckets := range app.treeStack {
		for treeHash, bucket := range buckets {
			require.Equal(t, len(bucket), cap(bucket),
				"method %d bucket %d: cap must equal len so appends cannot cross into the next bucket",
				method, treeHash)
		}
	}
}

// routeTreePaths returns the normalized paths of a tree bucket's routes, so
// assertions report path names instead of route pointers.
func routeTreePaths(routes []*Route) []string {
	paths := make([]string, len(routes))
	for i, route := range routes {
		paths[i] = route.path
	}
	return paths
}

// routerTestBucket returns the routes and the filter the method's route tree
// holds for the bucket keyed by the first three bytes of prefix, e.g. "/ro".
func routerTestBucket(app *App, method, prefix string) ([]*Route, *bucketFilter) {
	treeHash := int(prefix[0])<<16 | int(prefix[1])<<8 | int(prefix[2])
	return app.treeIndex[app.methodInt(method)].lookup(treeHash)
}

// Test_Route_PrefixFilter_UnconstrainedShapes covers the guards in
// computePrefixFilter that disable the filter when a route's first segment
// constrains nothing.
//
// Neither shape is reachable through the public API -- register always forces a
// leading '/', so a parsed route's first segment is a constant and its parser
// is never empty -- but the guards are what make that an assumption rather than
// a dependency, and a filter that stayed enabled for either would reject paths
// the route matches.
func Test_Route_PrefixFilter_UnconstrainedShapes(t *testing.T) {
	t.Parallel()
	// A pattern whose very first segment is a parameter, which only parses
	// this way without the leading slash register would add.
	parser := parseRoute(":name", regexp.MustCompile)
	require.NotEmpty(t, parser.segs)
	require.True(t, parser.segs[0].IsParam, "expected a leading parameter segment")

	leadingParam := &Route{routeParser: parser, Params: parser.params, path: ":name", Path: ":name"}
	word, mask := computePrefixFilter(leadingParam)
	require.Zero(t, mask, "a leading parameter must disable the filter")
	require.Zero(t, word)

	// A parametric route with no parsed segments at all.
	empty := &Route{Params: []string{"x"}}
	word, mask = computePrefixFilter(empty)
	require.Zero(t, mask, "an empty parser must disable the filter")
	require.Zero(t, word)

	// A disabled filter must never reject, whatever the path.
	for _, path := range []string{"", "/", "/anything", "/a/b/c"} {
		head := pathHeadWord(path)
		require.False(t, leadingParam.prefixRejects(head), "path %q", path)
		require.False(t, empty.prefixRejects(head), "path %q", path)
	}
	// Nor may either prefix word of the scanHead a filtered bucket holds.
	for _, route := range []*Route{leadingParam, empty} {
		h := newScanHead(route)
		require.Zero(t, h.prefixMask)
		require.Zero(t, h.prefixMask2)
	}
}

// Test_Route_OptionalSlash_SingleCharSegment guards the tree-bucket key against
// the optional trailing slash. A leading constant of exactly maxDetectionPaths
// bytes ("/a/") can match a detection path one byte shorter ("/a"), and that
// shorter path always hashes to bucket 0 — so bucketing the route under the
// hash of the full constant made it unreachable, while the otherwise identical
// "/ab/..." routes matched.
func Test_Route_OptionalSlash_SingleCharSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		path    string
	}{
		{"/a/:id?", "/a"},
		{"/a/:id?", "/a/"},
		{"/a/:id?", "/a/1"},
		{"/a/*", "/a"},
		{"/a/*", "/a/"},
		{"/a/*", "/a/b"},
		{"/ab/:id?", "/ab"},
		{"/ab/*", "/ab"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get(tt.pattern, func(c Ctx) error { return c.SendString("ok") })

			apptest.Expect(t, app, tt.path, StatusOK)
		})
	}
}

// Test_Route_URL_RefusesUnrepresentableRoute covers a route whose own path
// starts more than one slash. No relative URL names it: two leading slashes open
// an authority, so "//internal" resolves to the host "internal" rather than a
// path here, and collapsing to one reaches a different route. Both silently
// composed the wrong thing, one of them an open redirect, so it errors instead.
func Test_Route_URL_RefusesUnrepresentableRoute(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("//internal", func(c Ctx) error { return c.SendString("internal") }).Name("dbl")
	app.Get("//*", func(c Ctx) error { return c.SendString("wild") }).Name("dblwild")
	// The parser deletes the tab, so this path is read as "//internal" too. The
	// byte is not there to be seen by the time anything acts on the composition.
	app.Get("/\t/internal", func(c Ctx) error { return c.SendString("tabbed") }).Name("tab")
	// A trailing space goes the same way: the parser strips a run of controls or
	// spaces from either end, so this route is read as "/internal", which is a
	// different one. Reachable as "/internal%20" under UnescapePath, but no URL
	// composed here can name it.
	app.Get("/internal ", func(c Ctx) error { return c.SendString("spaced") }).Name("space")
	app.Get("/*", func(c Ctx) error { return c.SendString("ok") }).Name("wild")

	for _, name := range []string{"dbl", "dblwild", "tab", "space"} {
		url, err := app.GetRoute(name).URL(Map{"*": "evil.com"})
		require.ErrorIs(t, err, ErrRouteNotRepresentable, name)
		require.Empty(t, url, name)
	}

	// A wildcard route still composes, and a value cannot open an authority.
	url, err := app.GetRoute("wild").URL(Map{"*": "/evil.com"})
	require.NoError(t, err)
	require.Equal(t, "/evil.com", url)

	// What that composed reaches is this origin, not evil.com.
	ref, err := neturl.Parse(url)
	require.NoError(t, err)
	require.Empty(t, ref.Host)
}

// Constraints must not be lowercased along with the pattern.
func Test_Router_Constraints_CaseInsensitiveRouting(t *testing.T) {
	t.Parallel()
	for _, caseSensitive := range []bool{false, true} {
		t.Run(fmt.Sprintf("CaseSensitive=%v", caseSensitive), func(t *testing.T) {
			t.Parallel()
			app := New(Config{CaseSensitive: caseSensitive})
			app.RegisterCustomConstraint(&evenConstraint{})
			sub := New(Config{CaseSensitive: caseSensitive})
			sub.Get("/:code<regex([A-Z]{2})>", func(c Ctx) error { return c.SendString(c.Params("code")) })
			app.Use("/api", sub)

			app.Get("/code/:code<regex([A-Z]{2})>", func(c Ctx) error { return c.SendString(c.Params("code")) })
			app.Get(`/word/:w<regex(^\D+$)>`, func(c Ctx) error { return c.SendString(c.Params("w")) })
			app.Get("/when/:d<datetime(2006-01-02T15\\:04\\:05Z07\\:00)>", func(c Ctx) error { return c.SendString(c.Params("d")) })
			app.Get("/even/:n<isEven>", func(c Ctx) error { return c.SendString(c.Params("n")) })

			routerTestExpectAll(t, app, []routerReq{
				{path: "/code/US", status: StatusOK},
				{path: "/code/us", status: StatusNotFound},
				{path: "/api/US", status: StatusOK},
				{path: "/api/us", status: StatusNotFound},
				{path: "/word/abc", status: StatusOK},
				{path: "/word/123", status: StatusNotFound},
				{path: "/when/2024-01-02T15:04:05Z", status: StatusOK},
				{path: "/when/not-a-date", status: StatusNotFound},
				{path: "/even/4", status: StatusOK},
				{path: "/even/3", status: StatusNotFound},
				{path: "/even/abc", status: StatusNotFound},
			}...)

			cfg := Config{CaseSensitive: caseSensitive}
			require.True(t, RoutePatternMatch("/US", "/:code<regex([A-Z]{2})>", cfg))
			require.False(t, RoutePatternMatch("/us", "/:code<regex([A-Z]{2})>", cfg))
		})
	}
}

// evenConstraint accepts even integers; its mixed-case name is the point.
type evenConstraint struct{}

func (*evenConstraint) Name() string {
	return "isEven"
}

func (*evenConstraint) Execute(param string, _ ...string) bool {
	n, err := strconv.Atoi(param)
	return err == nil && n%2 == 0
}

func Test_Router_MethodOverride_ContinuesInNewMethodTree(t *testing.T) {
	t.Parallel()
	build := func(app *App) {
		app.Get("/a", func(c Ctx) error { return c.SendString("a") })
		app.Use(func(c Ctx) error {
			c.Method(MethodPost)
			return c.Next()
		})
		app.Post("/x", func(c Ctx) error { return c.SendString("post x") })
	}

	testCases := []struct {
		app  *App
		name string
	}{
		{name: "default ctx", app: New()},
		{name: "custom ctx", app: newCustomApp()},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			build(tc.app)
			apptest.ExpectBody(t, tc.app, "/x", StatusOK, "post x")
		})
	}

	// SkipUnmatchedRoutes answers 405 before middleware runs, so no override happens.
	t.Run("skip unmatched routes", func(t *testing.T) {
		t.Parallel()
		app := New(Config{SkipUnmatchedRoutes: true})
		build(app)
		apptest.Expect(t, app, "/x", StatusMethodNotAllowed)
	})
}

// Test_Router_MethodOverride_UseAfterMethodRoute pins that a single-method route
// between two Use registrations keeps them apart in every method's stack.
func Test_Router_MethodOverride_UseAfterMethodRoute(t *testing.T) {
	t.Parallel()

	override := func(c Ctx) error {
		if m := c.Get("X-HTTP-Method-Override"); m != "" {
			c.Method(m)
		}
		return c.Next()
	}
	guard := func(c Ctx) error {
		if c.Get(HeaderAuthorization) != "Bearer letmein" {
			return c.SendStatus(StatusUnauthorized)
		}
		return c.Next()
	}
	secret := func(c Ctx) error { return c.SendString("secret") }
	login := func(c Ctx) error { return c.SendString("login") }

	layouts := []struct {
		build func(Router)
		name  string
	}{
		{name: "guard after override", build: func(r Router) {
			r.Use(override)
			r.Post("/login", login)
			r.Use(guard)
			r.Get("/admin/secret", secret)
		}},
		{name: "override after use", build: func(r Router) {
			r.Use(func(c Ctx) error { return c.Next() })
			r.Post("/login", login)
			r.Use(override)
			r.Use("/admin", guard)
			r.Get("/admin/secret", secret)
		}},
	}
	apps := []struct {
		newApp func(build func(Router)) (*App, string)
		name   string
	}{
		{name: "default ctx", newApp: func(build func(Router)) (*App, string) {
			app := New()
			build(app)
			return app, ""
		}},
		{name: "custom ctx", newApp: func(build func(Router)) (*App, string) {
			app := NewWithCustomCtx(func(app *App) CustomCtx {
				return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
			})
			build(app)
			return app, ""
		}},
		{name: "group", newApp: func(build func(Router)) (*App, string) {
			app := New()
			build(app.Group("/v1"))
			return app, "/v1"
		}},
		{name: "mounted sub-app", newApp: func(build func(Router)) (*App, string) {
			sub := New()
			build(sub)
			app := New()
			app.Use("/api", sub)
			return app, "/api"
		}},
	}

	for _, layout := range layouts {
		for _, tc := range apps {
			t.Run(layout.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				app, prefix := tc.newApp(layout.build)
				target := prefix + "/admin/secret"

				resp, err := app.Test(httptest.NewRequest(MethodGet, target, http.NoBody))
				require.NoError(t, err)
				require.Equal(t, StatusUnauthorized, resp.StatusCode, "GET")

				req := httptest.NewRequest(MethodPost, target, http.NoBody)
				req.Header.Set("X-HTTP-Method-Override", MethodGet)
				resp, err = app.Test(req)
				require.NoError(t, err)
				require.Equal(t, StatusUnauthorized, resp.StatusCode, "POST overridden to GET")

				req = httptest.NewRequest(MethodPost, target, http.NoBody)
				req.Header.Set("X-HTTP-Method-Override", MethodGet)
				req.Header.Set(HeaderAuthorization, "Bearer letmein")
				resp, err = app.Test(req)
				require.NoError(t, err)
				require.Equal(t, StatusOK, resp.StatusCode, "POST overridden to GET with credentials")
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				require.Equal(t, "secret", string(body))
			})
		}
	}
}

func Test_Router_StrictRouting_ParamTrailingSlash(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		path   string
		status int
		strict bool
	}{
		{name: "strict with slash", strict: true, path: "/a/x/", status: StatusOK},
		{name: "strict without slash", strict: true, path: "/a/x", status: StatusNotFound},
		{name: "strict static with slash", strict: true, path: "/b/c/", status: StatusOK},
		{name: "strict static without slash", strict: true, path: "/b/c", status: StatusNotFound},
		{name: "lenient with slash", strict: false, path: "/a/x/", status: StatusOK},
		{name: "lenient without slash", strict: false, path: "/a/x", status: StatusOK},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New(Config{StrictRouting: tc.strict})
			app.Get("/a/:id/", func(c Ctx) error { return c.SendString(c.Params("id")) })
			app.Get("/b/c/", func(c Ctx) error { return c.SendString("c") })

			apptest.Expect(t, app, tc.path, tc.status)

			cfg := Config{StrictRouting: tc.strict}
			pattern := "/a/:id/"
			if strings.HasPrefix(tc.path, "/b") {
				pattern = "/b/c/"
			}
			require.Equal(t, tc.status == StatusOK, RoutePatternMatch(tc.path, pattern, cfg))
		})
	}
}

// Test_Router_StrictRouting_PrefixTrailingSlash pins that a Use route's trailing
// slash stays optional under StrictRouting, as "/admin/:id?" matches "/admin".
func Test_Router_StrictRouting_PrefixTrailingSlash(t *testing.T) {
	t.Parallel()

	guard := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	handler := func(c Ctx) error { return c.SendString("endpoint") }

	testCases := []struct {
		register func(app *App)
		name     string
		endpoint string
		guarded  []string
		missed   []string
	}{
		{
			name:     "app",
			register: func(app *App) { app.Use("/admin/", guard) },
			endpoint: "/admin/*",
			guarded:  []string{"/admin", "/admin/", "/admin/x"},
			missed:   []string{"/adminx", "/adm"},
		},
		{
			name:     "group",
			register: func(app *App) { app.Group("/admin").Use("/", guard) },
			endpoint: "/admin/:id?",
			guarded:  []string{"/admin", "/admin/", "/admin/x"},
			missed:   []string{"/adminx", "/adm"},
		},
		{
			name:     "group handler",
			register: func(app *App) { app.Group("/admin/", guard) },
			endpoint: "/admin/*",
			guarded:  []string{"/admin", "/admin/", "/admin/x"},
			missed:   []string{"/adminx", "/adm"},
		},
		{
			// "/a/" is as long as a tree hash, which "/a" is too short to carry
			name:     "short",
			register: func(app *App) { app.Use("/a/", guard) },
			endpoint: "/a/*",
			guarded:  []string{"/a", "/a/", "/a/x"},
			missed:   []string{"/ax"},
		},
		{
			name:     "parametric",
			register: func(app *App) { app.Use("/a/:id/", guard) },
			endpoint: "/a/:id/:sub?",
			guarded:  []string{"/a/x", "/a/x/", "/a/x/y"},
			missed:   []string{"/a", "/a/"},
		},
		{
			name:     "greedy tail",
			register: func(app *App) { app.Group("/api/*/admin").Use("/", guard) },
			endpoint: "/api/:tenant/admin/:page?",
			guarded:  []string{"/api/acme/admin", "/api/acme/admin/", "/api/acme/admin/admins"},
			missed:   []string{"/api/acme/admins"},
		},
		{
			// other greedy shapes keep the '/', which steers the guess as in v3.6.1
			name:     "greedy guess",
			register: func(app *App) { app.Group("/**/+/admin").Use("/", guard) },
			endpoint: "/**/+/admin/*",
			guarded:  []string{"/a/b/admin/x", "/x/admin/admin/x", "/a/b/c/admin/users"},
		},
	}
	for _, tc := range testCases {
		// The second round fills the buckets until their scan is filtered.
		for _, fill := range []int{0, 64} {
			t.Run(fmt.Sprintf("%s/fill=%d", tc.name, fill), func(t *testing.T) {
				t.Parallel()

				app := New(Config{StrictRouting: true})
				for i := range fill {
					app.Get(fmt.Sprintf("/admin/fill/%d", i), handler)
					app.Get(fmt.Sprintf("/a/fill/fill/%d", i), handler)
					app.Get(fmt.Sprintf("/%d", i), handler)
				}
				tc.register(app)
				app.Get(tc.endpoint, handler)

				for _, path := range tc.guarded {
					resp, err := app.Test(httptest.NewRequest(MethodGet, path, http.NoBody))
					require.NoError(t, err)
					require.Equal(t, StatusUnauthorized, resp.StatusCode, path)
				}
				for _, path := range tc.missed {
					resp, err := app.Test(httptest.NewRequest(MethodGet, path, http.NoBody))
					require.NoError(t, err)
					require.Equal(t, StatusNotFound, resp.StatusCode, path)
				}
			})
		}
	}
}

// routerTestExpectRawBody serves fctx through handler and asserts a 200 response
// whose body is want. It is for request targets httptest rejects, which have to
// reach the router as raw fasthttp requests; msgAndArgs name the case in a failure.
func routerTestExpectRawBody(tb testing.TB, handler fasthttp.RequestHandler, fctx *fasthttp.RequestCtx, want string, msgAndArgs ...any) {
	tb.Helper()

	handler(fctx)
	require.Equal(tb, StatusOK, fctx.Response.StatusCode(), msgAndArgs...)
	require.Equal(tb, want, string(fctx.Response.Body()), msgAndArgs...)
}

func Test_Router_UnescapePath_PlusIsLiteral(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true})
	app.Get("/u/:name", func(c Ctx) error { return c.SendString(c.Params("name") + "|" + c.Path()) })

	testCases := []struct {
		path string
		body string
	}{
		{path: "/u/john+doe", body: "john+doe|/u/john+doe"},
		{path: "/u/john%2Bdoe", body: "john+doe|/u/john+doe"},
		{path: "/u/john%20doe", body: "john doe|/u/john doe"},
		{path: "/u/a%zzb", body: "a%zzb|/u/a%zzb"},
		{path: "/u/end%2", body: "end%2|/u/end%2"},
	}
	// Through the raw handler: net/http rejects malformed escapes.
	handler := app.Handler()
	for _, tc := range testCases {
		routerTestExpectRawBody(t, handler, routerTestRequestCtx(MethodGet, tc.path), tc.body, "GET %s", tc.path)
	}

	require.True(t, RoutePatternMatch("/u/john+doe", "/u/john+doe", Config{UnescapePath: true}))
	require.False(t, RoutePatternMatch("/u/john+doe", "/u/john doe", Config{UnescapePath: true}))
}

// Test_Router_StrayPercent pins what a "%" that begins no escape becomes in
// c.Path(). fasthttp copies it as sent, so a path holding nothing else to
// decode or resolve keeps its parsed length, passes pathNeedsNormalization and
// is matched as sent. A path normalized for another reason keeps it as sent
// too, unless an escape decoded after it would complete a new escape with it;
// then it is written as "%25". Either way c.Path() never holds an escape of an
// unreserved character, and the same name comes out alike whatever else the
// path holds. httptest rejects these spellings, so the requests go through the
// raw handler.
func Test_Router_StrayPercent(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		path string
		body string
	}{
		// nothing to decode or resolve: matched as sent
		{path: "/a%zzb", body: "/a%zzb"},
		{path: "/trailing%2", body: "/trailing%2"},
		{path: "/%", body: "/%"},
		// normalized for something else: the stray "%" is kept
		{path: "/%2g%41", body: "/%2gA"},
		{path: "/x%zz/%41", body: "/x%zz/A"},
		{path: "/a%zzb/./c", body: "/a%zzb/c"},
		{path: "/a//%zzb", body: "/a//%zzb"},
		{path: "/files/a%2Fb/100%", body: "/files/a%2Fb/100%"},
		// a decoded escape would complete an escape with it: "%25"
		{path: "/%%370rivate", body: "/%2570rivate"},
		{path: "/x/%%32%65git", body: "/x/%252egit"},
	}
	for _, caseSensitive := range []bool{false, true} {
		app := New(Config{CaseSensitive: caseSensitive})
		app.Use(func(c Ctx) error { return c.SendString(c.Path()) })
		handler := app.Handler()
		for _, tc := range testCases {
			routerTestExpectRawBody(t, handler, routerTestRequestCtx(MethodGet, tc.path), tc.body, "CaseSensitive=%v GET %s", caseSensitive, tc.path)
		}
	}
}

// Test_Router_StrayPercentParamIsStable pins that a route parameter holding a
// stray "%" does not depend on the rest of the path: "/files/100%" is matched
// as sent, "/files/./100%" and "/files/x/../100%" are normalized for their dot
// segments, and under DisablePathNormalizing every "%" sends the path through
// normalization. All of them give the parameter "100%".
func Test_Router_StrayPercentParamIsStable(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/files/:name", func(c Ctx) error { return c.SendString(c.Params("name")) })
	handler := app.Handler()
	for _, disable := range []bool{false, true} {
		for _, target := range []string{"/files/100%", "/files/./100%", "/files/x/../100%", "/files/%2e/100%"} {
			fctx := routerTestRequestCtx(MethodGet, target)
			fctx.Request.URI().DisablePathNormalizing = disable
			routerTestExpectRawBody(t, handler, fctx, "100%", "DisablePathNormalizing=%v GET %s", disable, target)
		}
	}
}

// Test_Router_UnrootedTargetIsRooted pins that a request target without a
// leading slash, the absolute form with an empty path or the asterisk form,
// is routed as the rooted path fasthttp reports for it. Unrooted, it slipped
// past every root-level Use middleware while a "/*" route still matched it.
// httptest rejects these targets, so the requests go through the raw handler.
func Test_Router_UnrootedTargetIsRooted(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		method, target, path string
	}{
		{MethodGet, "http://example.com?x=1", "/"},
		{MethodPost, "http://example.com#f", "/"},
		{MethodOptions, "*", "/*"},
		{MethodGet, "http://example.com/a/b", "/a/b"},
		// the authority form fasthttp admits for CONNECT; with one doubled
		// slash the parsed copy keeps the original's length, since the slash
		// fasthttp adds in front is the one it takes out of the pair, so the
		// length gate alone would not notice the missing slash
		{MethodConnect, "admin//secret", "/admin//secret"},
		{MethodConnect, "secret//", "/secret//"},
		{MethodConnect, "example.com:443", "/example.com:443"},
	}
	// with DisablePathNormalizing the scan, not fasthttp's parsed copy, has
	// to notice the missing slash
	for _, disable := range []bool{false, true} {
		for _, tc := range testCases {
			app := New()
			guardRan, seen := false, ""
			app.Use(func(c Ctx) error {
				guardRan = true
				return c.Next()
			})
			app.All("/*", func(c Ctx) error {
				seen = c.Path()
				return c.SendString("catch-all")
			})

			var req fasthttp.Request
			req.Header.SetMethod(tc.method)
			req.SetRequestURI(tc.target)
			req.Header.SetHost("example.com")
			var fctx fasthttp.RequestCtx
			fctx.Init(&req, nil, nil)
			if disable {
				fctx.Request.URI().DisablePathNormalizing = true
			}
			app.Handler()(&fctx)

			require.Equal(t, StatusOK, fctx.Response.StatusCode(), "disable=%v %s %s", disable, tc.method, tc.target)
			require.True(t, guardRan, "disable=%v %s %s: root middleware skipped", disable, tc.method, tc.target)
			require.Equal(t, tc.path, seen, "disable=%v %s %s", disable, tc.method, tc.target)
		}
	}

	// an override is rooted the same way
	app := New()
	app.Use(func(c Ctx) error {
		c.Path("rewritten")
		return c.Next()
	})
	app.Get("/rewritten", func(c Ctx) error { return c.SendString(c.Path()) })
	apptest.ExpectBody(t, app, "/original", StatusOK, "/rewritten")
}

// Test_Router_SlashesOnlyTargetIsRoot pins that a target of slashes alone is
// routed as the root without StrictRouting, as "/a//" is routed as "/a".
func Test_Router_SlashesOnlyTargetIsRoot(t *testing.T) {
	t.Parallel()

	serve := func(app *App, target string) int {
		var req fasthttp.Request
		req.Header.SetMethod(MethodGet)
		req.SetRequestURI(target)
		req.Header.SetHost("example.com")
		var fctx fasthttp.RequestCtx
		fctx.Init(&req, nil, nil)
		app.Handler()(&fctx)
		return fctx.Response.StatusCode()
	}

	testCases := []struct {
		target, path string
		unescape     bool
	}{
		{target: "//", path: "//"},
		{target: "///", path: "///"},
		{target: "//?x=1", path: "//"},
		{target: "http://example.com//", path: "//"},
		{target: "/a/..//", path: "//"},
		{target: "/%2F", path: "//", unescape: true},
	}
	for _, pattern := range []string{"/*", "/:name?"} {
		for _, tc := range testCases {
			app := New(Config{UnescapePath: tc.unescape})
			guardRan, seen := false, ""
			app.Use(func(c Ctx) error {
				guardRan = true
				return c.Next()
			})
			app.Get(pattern, func(c Ctx) error {
				seen = c.Path()
				return c.SendString("catch-all")
			})

			require.Equal(t, StatusOK, serve(app, tc.target), "%s %s", pattern, tc.target)
			require.True(t, guardRan, "%s %s: root middleware skipped", pattern, tc.target)
			require.Equal(t, tc.path, seen, "%s %s", pattern, tc.target)
		}
	}

	// StrictRouting keeps "//" apart from "/"
	for _, strict := range []bool{false, true} {
		app := New(Config{StrictRouting: strict})
		app.Get("/", func(c Ctx) error { return c.SendString("root") })

		want := StatusOK
		if strict {
			want = StatusNotFound
		}
		require.Equal(t, want, serve(app, "//"), "strict=%v", strict)
	}

	// a pattern of slashes alone is the root too
	app := New()
	app.Use("//", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
	app.Get("//", func(c Ctx) error { return c.SendString("root") })
	for _, target := range []string{"/", "//", "///"} {
		require.Equal(t, StatusUnauthorized, serve(app, target), target)
	}

	require.True(t, RoutePatternMatch("//", "/"))
	require.True(t, RoutePatternMatch("///", "/"))
	require.True(t, RoutePatternMatch("/", "//"))
	require.False(t, RoutePatternMatch("//", "/", Config{StrictRouting: true}))
}

// routerTestRouteNames returns "METHOD:name" for every route registered on app
// under path, sorted.
func routerTestRouteNames(app *App, path string) []string {
	var names []string
	routes := app.GetRoutes()
	for i := range routes {
		if routes[i].Path == path {
			names = append(names, routes[i].Method+":"+routes[i].Name)
		}
	}
	slices.Sort(names)
	return names
}

// Test_App_Name_MultipleMethods checks the names the routes of a registration
// for several methods end up with, alone and next to earlier registrations of
// the same path that it merges into or leaves alone.
func Test_App_Name_MultipleMethods(t *testing.T) {
	t.Parallel()
	h := func(c Ctx) error { return c.SendString("ok") }
	for _, tc := range []struct {
		register func(app *App)
		paths    map[string]string // route name -> the path GetRoute finds for it
		name     string
		path     string
		want     []string // "METHOD:name" of every route under path
	}{
		{
			// GET /x and POST /x must carry the name
			name: "Add_MultipleMethods_Name", path: "/x", want: []string{"GET:x", "POST:x"},
			register: func(app *App) {
				app.Add([]string{MethodGet, MethodPost}, "/x", func(c Ctx) error { return c.SendString("x") }).Name("x")
			},
		},
		{
			name: "Add_MultipleMethods_NameMergedRoutes", path: "/x",
			want: []string{"GET:first", "GET:second", "POST:first", "POST:second"},
			register: func(app *App) {
				app.Get("/x", h).Name("first")
				app.Post("/x", h).Name("first")
				app.Add([]string{MethodGet, MethodPost}, "/x", h).Name("second")
			},
		},
		{
			// Two groups share a path, so the second registration merges into the first
			// group's route, and the name it is given belongs to the group it was
			// registered through.
			name: "MergedRouteUsesLatestGroup", path: "/api/x", want: []string{"GET:two.second", "POST:two.second"},
			paths: map[string]string{"two.second": "/api/x", "one.second": ""},
			register: func(app *App) {
				one := app.Group("/api")
				one.Name("one.")
				two := app.Group("/api")
				two.Name("two.")
				one.Add([]string{MethodGet, MethodPost}, "/x", h).Name("first")
				two.Add([]string{MethodGet, MethodPost}, "/x", h).Name("second")
			},
		},
		{
			// Registering another path moves /x off the tail of the GET stack, so the
			// routes above survive rather than being merged into the ones below: they
			// keep their own name, and only the routes the latest Add created are renamed.
			name: "OnlyLatestRegistration", path: "/x",
			want: []string{"GET:first", "GET:second", "POST:first", "POST:second"},
			register: func(app *App) {
				app.Get("/x", h).Name("first")
				app.Post("/x", h).Name("first")
				app.Get("/y", h)
				app.Add([]string{MethodGet, MethodPost}, "/x", h).Name("second")
			},
		},
		{
			// The row above with the methods of the last Add swapped: which method it
			// finishes on is not allowed to decide whether the older GET route is
			// renamed too, so looking either name up lands on the registration that
			// carries it.
			name: "OnlyLatestRegistration_ReverseMethodOrder", path: "/x",
			want: []string{"GET:first", "GET:second", "POST:second"}, paths: map[string]string{"first": "/x"},
			register: func(app *App) {
				app.Get("/x", h).Name("first")
				app.Get("/y", h)
				app.Add([]string{MethodPost, MethodGet}, "/x", h).Name("second")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			tc.register(app)
			require.Equal(t, tc.want, routerTestRouteNames(app, tc.path))
			for name, path := range tc.paths {
				require.Equal(t, path, app.GetRoute(name).Path, name)
			}
		})
	}
}

// Test_App_Add_MergedRoute_KeepsRegistrationID covers what a merge must not
// cost the route it merges into: its id pairs it with the copies the same
// registration filed under the other methods, which is how routeIndexInTree
// resumes the scan when a handler switches method mid-request.
func Test_App_Add_MergedRoute_KeepsRegistrationID(t *testing.T) {
	t.Parallel()
	app := New()
	// Sits ahead of /x in the GET bucket only, so the index the GET match
	// leaves behind is past the one POST /x holds in its own bucket: carrying
	// it over unchanged would step over the wildcard below.
	app.Get("/a", func(c Ctx) error { return c.SendString("a") })
	app.Add([]string{MethodGet, MethodPost}, "/x", func(c Ctx) error { return c.Next() })
	// Merges into both routes of the registration above, whose POST copy is
	// the one the switch has to find.
	app.Add([]string{MethodGet, MethodPost}, "/x", func(c Ctx) error {
		c.Method(MethodPost)
		return c.Next()
	})
	app.Post("/*", func(c Ctx) error { return c.SendString("wildcard") })

	apptest.ExpectBody(t, app, "/x", StatusOK, "wildcard")
}

// Test_App_Use_MultiplePrefixes_MountsEachPrefix mounts a sub-app under every
// prefix of a list, through the app, a group and a domain.
func Test_App_Use_MultiplePrefixes_MountsEachPrefix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mount func(app, sub *App)
		name  string
		route string // the sub-app's route
		paths []string
	}{
		{name: "app", route: "/x", paths: []string{"/a/x", "/b/x"}, mount: func(app, sub *App) { app.Use([]string{"/a", "/b"}, sub) }},
		{
			name: "group", route: "/ping", paths: []string{"/g/one/ping", "/g/two/ping"},
			mount: func(app, sub *App) { app.Group("/g").Use([]string{"/one", "/two"}, sub) },
		},
		{
			// httptest requests carry the host example.com
			name: "domain", route: "/ping", paths: []string{"/one/ping", "/two/ping"},
			mount: func(app, sub *App) { app.Domain("example.com").Use([]string{"/one", "/two"}, sub) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sub := New()
			sub.Get(tc.route, func(c Ctx) error { return c.SendString("ok") })
			app := New()
			tc.mount(app, sub)
			for _, path := range tc.paths {
				apptest.Expect(t, app, path, StatusOK)
			}
		})
	}
}

func Test_App_Mount_Self_Panics(t *testing.T) {
	t.Parallel()
	app := New()
	require.Panics(t, func() { app.Use("/x", app) })
}

func Test_App_RebuildTree_RegistersAutoHead(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/early", func(c Ctx) error { return c.SendString("early") })
	handler := app.Handler()

	app.Get("/late", func(c Ctx) error { return c.SendString("late") })
	app.RebuildTree()

	for _, path := range []string{"/early", "/late"} {
		fctx := routerTestRequestCtx(MethodHead, path)
		handler(fctx)
		require.Equal(t, StatusOK, fctx.Response.StatusCode(), "HEAD %s", path)
	}
}

func Test_Mount_Nested_AutoHead_SingleStartup(t *testing.T) {
	t.Parallel()
	nested := New()
	nested.Get("/x", func(c Ctx) error { return c.SendString("x") })
	sub := New()
	app := New()
	// Mount the parent first: a sub-app nested afterwards is only discovered at startup.
	app.Use("/a", sub)
	sub.Use("/b", nested)

	// One startup, then the very first request.
	handler := app.Handler()
	for _, method := range []string{MethodHead, MethodGet} {
		fctx := routerTestRequestCtx(method, "/a/b/x")
		handler(fctx)
		require.Equal(t, StatusOK, fctx.Response.StatusCode(), "%s /a/b/x", method)
	}
}

// Test_App_AutoHead_KeepsRegistrationOrder pins that an automatic HEAD route
// runs where its GET route does, so a Get guard covers HEAD too.
func Test_App_AutoHead_KeepsRegistrationOrder(t *testing.T) {
	t.Parallel()

	guard := func(c Ctx) error { return c.SendStatus(StatusUnauthorized) }
	next := func(c Ctx) error { return c.Next() }
	endpoint := func(c Ctx) error {
		c.Set("X-Secret", "1")
		return c.SendString("secret")
	}

	for _, tc := range []struct {
		register func(app *App)
		name     string
	}{
		{name: "mounted app", register: func(app *App) {
			app.Get("/admin/*", guard)
			sub := New()
			sub.Get("/secret", endpoint)
			app.Use("/admin", sub)
		}},
		{name: "Use", register: func(app *App) {
			app.Get("/admin/*", guard)
			app.Use("/admin", endpoint)
		}},
		{name: "Add", register: func(app *App) {
			app.Get("/admin/*", guard)
			app.Add([]string{MethodGet, MethodHead}, "/admin/secret", endpoint)
		}},
		{name: "explicit Head", register: func(app *App) {
			app.Get("/admin/*", guard)
			app.Get("/admin/secret", endpoint)
			app.Head("/admin/secret", endpoint)
		}},
		{name: "Use before and after", register: func(app *App) {
			app.Use(next)
			app.Get("/admin/*", guard)
			app.Use(endpoint)
		}},
		{name: "late explicit Head on another path", register: func(app *App) {
			app.Get("/x", endpoint)
			app.Get("/admin/*", guard)
			sub := New()
			sub.Get("/secret", endpoint)
			app.Use("/admin", sub)
			app.Head("/x", endpoint)
		}},
		{name: "repeated Get path", register: func(app *App) {
			app.Get("/admin/*", next)
			app.Use(next)
			app.Get("/admin/*", guard)
			app.Get("/admin/secret", endpoint)
		}},
		{name: "Domain mount", register: func(app *App) {
			sub := New()
			sub.Get("/admin/*", guard)
			sub.Use("/admin", endpoint)
			app.Domain("example.com").Use("/", sub)
		}},
		{name: "Domain mount with an older Head route", register: func(app *App) {
			inner := New()
			inner.Head("/secret", endpoint)
			inner.Get("/secret", endpoint)
			sub := New()
			sub.Get("/admin/*", guard)
			sub.Use("/admin", inner)
			app.Domain("example.com").Use("/", sub)
		}},
		{name: "All after Get", register: func(app *App) {
			app.Get("/admin/*", guard)
			app.All("/admin/*", endpoint)
		}},
		{name: "Add of both methods after Get", register: func(app *App) {
			app.Get("/admin/*", guard)
			app.Add([]string{MethodGet, MethodHead}, "/admin/*", endpoint)
		}},
		{name: "Get after All", register: func(app *App) {
			app.All("/admin/*", next)
			app.Get("/admin/*", guard)
			app.Get("/admin/secret", endpoint)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			tc.register(app)

			// Twice: the second startup sees any mount already expanded.
			for range 2 {
				for _, method := range []string{MethodGet, MethodHead} {
					resp, err := app.Test(httptest.NewRequest(method, "/admin/secret", http.NoBody))
					require.NoError(t, err)
					require.NoError(t, resp.Body.Close())
					require.Equal(t, StatusUnauthorized, resp.StatusCode, method)
					require.Empty(t, resp.Header.Get("X-Secret"), method)
				}
			}
		})
	}

	t.Run("catch-all after Get", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/hello", endpoint)
		app.Use(func(c Ctx) error { return c.SendStatus(StatusNotFound) })

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/hello", http.NoBody))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, StatusOK, resp.StatusCode)
	})

	t.Run("earlier Head on another path stays first", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Head("/:name", func(c Ctx) error { return c.SendStatus(StatusTeapot) })
		app.Get("/hello", endpoint)

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/hello", http.NoBody))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, StatusTeapot, resp.StatusCode)
	})

	t.Run("late Head replaces every copy", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/a", endpoint)
		app.Use(next)
		app.Get("/a", endpoint)
		app.Handler()

		app.Head("/a", func(c Ctx) error { return c.SendStatus(StatusTeapot) })
		app.RebuildTree()

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/a", http.NoBody))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, StatusTeapot, resp.StatusCode)
	})

	t.Run("Get merged after startup", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/admin/*", next)
		app.Handler()

		app.Get("/admin/*", guard)
		app.Get("/admin/secret", endpoint)
		app.RebuildTree()

		resp, err := app.Test(httptest.NewRequest(MethodHead, "/admin/secret", http.NoBody))
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, StatusUnauthorized, resp.StatusCode)
		require.Empty(t, resp.Header.Get("X-Secret"))
	})
}

// Test_App_Mount_SamePrefixKeepsAutoHead pins that a second mount on one prefix
// does not cost the first one its automatic HEAD routes.
func Test_App_Mount_SamePrefixKeepsAutoHead(t *testing.T) {
	t.Parallel()

	guards := New()
	guards.Get("/admin/*", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
	api := New()
	api.Get("/admin/flush", func(c Ctx) error {
		c.Set("X-Secret", "1")
		return c.SendString("flushed")
	})

	app := New()
	app.Use("/api", guards)
	app.Use("/api", api)

	// One startup: a second one would fill the gap in the expanded stack.
	handler := app.Handler()
	for _, method := range []string{MethodGet, MethodHead} {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(method)
		fctx.Request.SetRequestURI("/api/admin/flush")
		handler(fctx)
		require.Equal(t, StatusUnauthorized, fctx.Response.StatusCode(), method)
		require.Empty(t, fctx.Response.Header.Peek("X-Secret"), method)
	}
}

// Test_App_Mount_NestedRootMounts pins that an app displaced from the app list
// by a nested mount on the same prefix still has its own mounts expanded.
func Test_App_Mount_NestedRootMounts(t *testing.T) {
	t.Parallel()

	// Which of the two apps the app list keeps depends on map iteration order.
	for range 20 {
		inner := New()
		inner.Get("/x", func(c Ctx) error { return c.SendString("x") })
		site := New()
		site.Use(inner)
		app := New()
		app.Use(site)

		for _, method := range []string{MethodGet, MethodHead} {
			resp, err := app.Test(httptest.NewRequest(method, "/x", http.NoBody))
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, StatusOK, resp.StatusCode, method)
		}
	}
}

// Test_App_Mount_DisplacedAppKeepsHeadGuard pins that a child's HEAD route on
// the guard's path does not replace the guard's own HEAD route in a displaced app.
func Test_App_Mount_DisplacedAppKeepsHeadGuard(t *testing.T) {
	t.Parallel()

	// Which app the app list keeps at "/admin" depends on map iteration order.
	for range 100 {
		logger := New()
		logger.Head("/*", func(c Ctx) error { return c.Next() })
		admin := New()
		admin.Use("/", logger)
		admin.Get("/*", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
		admin.Get("/flush", func(c Ctx) error { return c.SendString("flushed") })
		wrap := New()
		wrap.Use("/", admin)
		other, pages := New(), New()
		other.Use("/", New())
		pages.Use("/admin", other)
		app := New()
		app.Use("/admin", wrap)
		app.Use("/", pages)

		for _, method := range []string{MethodGet, MethodHead} {
			resp, err := app.Test(httptest.NewRequest(method, "/admin/flush", http.NoBody))
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, StatusUnauthorized, resp.StatusCode, method)
		}
	}
}

func Test_Router_PathOverride_ContinuesInNewBucket(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/zzz/other", func(c Ctx) error { return c.SendString("other") })
	app.Use(func(c Ctx) error {
		if c.Path() == "/zzz/y" {
			c.Path("/aaa/x")
		}
		return c.Next()
	})
	app.Get("/aaa/x", func(c Ctx) error { return c.SendString("aaa x") })

	apptest.ExpectBody(t, app, "/zzz/y", StatusOK, "aaa x")
}

// Test_Router_PathOverride_FromPrefixedRoute pins that a path change made by a
// route filed under its own prefix bucket resumes with the routes registered after it.
func Test_Router_PathOverride_FromPrefixedRoute(t *testing.T) {
	t.Parallel()

	for _, ahead := range []int{0, 1, 3, 6, 13} {
		for _, global := range []bool{false, true} {
			t.Run(fmt.Sprintf("ahead=%d/global=%v", ahead, global), func(t *testing.T) {
				t.Parallel()

				app := New()
				var ran []string
				note := func(name string) Handler {
					return func(c Ctx) error {
						ran = append(ran, name)
						return c.Next()
					}
				}
				deny := func(c Ctx) error { return c.SendStatus(StatusForbidden) }

				app.Use("/admin", note("early"))
				for range ahead {
					app.Use(note("global"))
					app.Use("/v1", note("v1"))
				}
				app.Use("/v1", func(c Ctx) error {
					c.Path(strings.TrimPrefix(c.Path(), "/v1"))
					return c.Next()
				})
				if global {
					app.Use(deny)
				} else {
					app.Use("/admin", deny)
				}
				app.Get("/admin/report", func(c Ctx) error { return c.SendString("report") })
				app.Get("/public/page", func(c Ctx) error { return c.SendString("page") })

				resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/admin/report", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, StatusForbidden, resp.StatusCode)
				require.NoError(t, resp.Body.Close())
				require.NotContains(t, ran, "early")
				require.Equal(t, ahead, countOf(ran, "global"))

				// The handler heading its bucket is not skipped either.
				resp, err = app.Test(httptest.NewRequest(MethodGet, "/v1/public/page", http.NoBody))
				require.NoError(t, err)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				if global {
					require.Equal(t, StatusForbidden, resp.StatusCode)
					return
				}
				require.Equal(t, StatusOK, resp.StatusCode)
				require.Equal(t, "page", string(body))
			})
		}
	}
}

// Test_Router_PathOverride_AppMountedTwice pins that the running route is placed
// by itself, not by its copy under another mount prefix.
func Test_Router_PathOverride_AppMountedTwice(t *testing.T) {
	t.Parallel()

	// No HEAD copies, so a miss is a 404 rather than a 405.
	sub := New(Config{DisableHeadAutoRegister: true})
	sub.Use(func(c Ctx) error {
		for _, prefix := range []string{"/aaa", "/ccc"} {
			if rest, ok := strings.CutPrefix(c.Path(), prefix); ok {
				c.Path("/bbb" + rest)
			}
		}
		return c.Next()
	})
	sub.Get("/x", func(c Ctx) error { return c.SendString("x") })

	app := New()
	app.Use("/aaa", sub)
	app.Use("/bbb", func(c Ctx) error { return c.SendStatus(StatusForbidden) })
	app.Use("/bbb", sub)
	app.Use("/ccc", sub)

	// Registered after the /aaa mount, so the guard runs.
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/aaa/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())

	// Registered before the /ccc mount, so neither the guard nor /bbb runs.
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/ccc/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusNotFound, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// Test_Router_MethodOverride_FromRouteOfOneMethod covers a method change made by
// a route the new method holds no copy of.
func Test_Router_MethodOverride_GetAfterRouteOfSeveralMethods(t *testing.T) {
	t.Parallel()

	override := func(c Ctx) error {
		c.Method(c.Get("X-HTTP-Method-Override"))
		return c.Next()
	}
	for name, register := range map[string]func(app *App){
		"All": func(app *App) { app.All("/admin/*", override) },
		"Add": func(app *App) { app.Add([]string{MethodPost, MethodGet}, "/admin/*", override) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := New()
			register(app)
			app.Get("/admin/*", func(c Ctx) error { return c.SendStatus(StatusUnauthorized) })
			app.Get("/admin/flush", func(c Ctx) error { return c.SendString("flushed") })

			req := httptest.NewRequest(MethodPost, "/admin/flush", http.NoBody)
			req.Header.Set("X-HTTP-Method-Override", MethodGet)
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, StatusUnauthorized, resp.StatusCode)
		})
	}
}

func Test_Router_MethodOverride_FromRouteOfOneMethod(t *testing.T) {
	t.Parallel()

	build := func(guarded bool, override Handler) (*App, *[]string) {
		app := New()
		ran := new([]string)
		note := func(name string) Handler {
			return func(c Ctx) error {
				*ran = append(*ran, name)
				return c.Next()
			}
		}

		app.Use(note("global"))
		app.All("/admin/*", note("all"))
		app.Get("/other", note("other"))
		app.Post("/*", override)
		if guarded {
			app.Use("/admin", func(c Ctx) error { return c.SendStatus(StatusForbidden) })
		}
		app.Delete("/admin/x", func(c Ctx) error { return c.SendString("deleted") })
		return app, ran
	}

	testCases := []struct {
		override Handler
		name     string
		target   string
	}{
		{name: "method", target: "/admin/x", override: func(c Ctx) error {
			c.Method(MethodDelete)
			return c.Next()
		}},
		{name: "method then path", target: "/v1/x", override: func(c Ctx) error {
			c.Method(MethodDelete)
			c.Path("/admin/x")
			return c.Next()
		}},
		{name: "path then method", target: "/v1/x", override: func(c Ctx) error {
			c.Path("/admin/x")
			c.Method(MethodDelete)
			return c.Next()
		}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, _ := build(true, tc.override)
			resp, err := app.Test(httptest.NewRequest(MethodPost, tc.target, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, StatusForbidden, resp.StatusCode)
			require.NoError(t, resp.Body.Close())

			app, ran := build(false, tc.override)
			resp, err = app.Test(httptest.NewRequest(MethodPost, tc.target, http.NoBody))
			require.NoError(t, err)
			require.Equal(t, StatusOK, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
			// Shared routes ahead of the override do not run twice.
			require.Equal(t, 1, countOf(*ran, "global"))
			require.LessOrEqual(t, countOf(*ran, "all"), 1)
		})
	}
}

// Test_Router_MethodOverride_IgnoresAutoHeadRoute pins that the automatic HEAD
// copy of a GET route does not mark where that route stands.
func Test_Router_MethodOverride_IgnoresAutoHeadRoute(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/admin/*", func(c Ctx) error {
		c.Method(MethodHead)
		return c.Next()
	})
	app.Use("/admin", func(c Ctx) error { return c.SendStatus(StatusForbidden) })
	app.Get("/admin/x", func(c Ctx) error { return c.SendString("x") })

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/admin/x", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, StatusForbidden, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func countOf(values []string, value string) int {
	n := 0
	for _, v := range values {
		if v == value {
			n++
		}
	}
	return n
}

func Test_Router_Constraints_MixedCaseNames(t *testing.T) {
	t.Parallel()
	// Built-in constraint names are matched case-insensitively, whatever the
	// pattern spells them, and through every entry point that parses one.
	//nolint:usestdlibvars // route constraint names, not go/constant kinds
	names := []string{"INT", "Int", "iNt", "int", "BOOL", "Guid", "ALPHA", "Float", "minLen(3)", "Range(5,10)"}
	//nolint:usestdlibvars // route constraint names, not go/constant kinds
	reject := map[string]string{
		"INT": "abc", "Int": "abc", "iNt": "abc", "int": "abc",
		"BOOL": "abc", "Guid": "abc", "ALPHA": "123", "Float": "abc",
		"minLen(3)": "ab", "Range(5,10)": "99",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New()
			pattern := "/:id<" + name + ">"
			app.Get(pattern, func(c Ctx) error { return c.SendString(c.Params("id")) })

			sub := New()
			sub.Get(pattern, func(c Ctx) error { return c.SendString(c.Params("id")) })
			app.Use("/api", sub)

			bad := "/" + reject[name]
			apptest.Expect(t, app, bad, StatusNotFound)
			apptest.Expect(t, app, "/api"+bad, StatusNotFound)

			require.False(t, RoutePatternMatch(bad, pattern), "RoutePatternMatch accepted %q for %s", bad, pattern)
		})
	}
}

func Test_App_AutoHead_PanickingHookLeavesScanIncomplete(t *testing.T) {
	t.Parallel()
	app := New()
	fail := true
	// Only the synthesized companion fails, so registration itself succeeds.
	app.Hooks().OnRoute(func(r Route) error {
		if fail && r.Method == MethodHead {
			return errors.New("boom")
		}
		return nil
	})
	app.Get("/ping", func(c Ctx) error { return c.SendString("pong") })

	require.Panics(t, func() { app.RebuildTree() })

	// The aborted scan must not be recorded as complete, or HEAD stays 405.
	fail = false
	app.RebuildTree()

	apptest.Expect(t, app, apptest.Req(MethodHead, "/ping"), StatusOK)
}

// Test_Route_Fingerprint_MatchesMatch is the safety contract of the static-path
// filter in next() and nextCustom(): the filter is only ever allowed to reject
// what Route.match would reject anyway. A route the filter speaks for (a
// non-zero fingerprint) must never match a detection path whose fingerprint
// differs, because the scan drops it without calling match at all.
//
// staticFingerprint's exclusions mirror match's branches, so this walks every
// route shape against every path and fails if the filter is ever stricter than
// the matcher it stands in for.
func Test_Route_Fingerprint_MatchesMatch(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/mw", func(c Ctx) error { return c.Next() })
	app.Use(func(c Ctx) error { return c.Next() })
	for _, pattern := range []string{
		"/", "/users", "/users/list", "/users/:id", "/users/:id/posts", "/files/*",
		"/opt/:name?", "/long/static/path/segment", "/a", "/ab", "/abc",
	} {
		app.Get(pattern, testEmptyHandler)
	}

	paths := []string{
		"/", "/users", "/users/", "/users/list", "/users/42", "/users/42/posts",
		"/files/a/b", "/opt", "/opt/x", "/long/static/path/segment", "/mw",
		"/mw/deep", "/a", "/ab", "/abc", "/abcd", "/nope", "",
	}

	for _, route := range app.stack[app.methodInt(MethodGet)] {
		fp := staticFingerprint(route)
		if fp == 0 {
			continue // the filter does not speak for this route
		}
		for _, path := range paths {
			if fp == pathFingerprint(path) {
				continue // not rejected; match decides as usual
			}
			var params [maxParams]string
			slashes := strings.Count(path, "/")
			require.False(t, route.match(path, path, &params, slashes),
				"filter drops %q for %q but match accepts it", route.path, path)
		}
	}
}

// Test_PathFingerprint_Contract pins what the filter relies on: equal inputs
// hash equal whatever their type, and 0 is reserved for "no fingerprint".
func Test_PathFingerprint_Contract(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"", "/", "/a", "/ab", "/abcdefg", "/abcdefgh", "/abcdefghi",
		"/users/list", "/long/static/path/segment/that/exceeds/one/word",
	} {
		require.NotZero(t, pathFingerprint(path), "fingerprint of %q must not be 0", path)
		require.Equal(t, pathFingerprint(path), pathFingerprint([]byte(path)),
			"string and []byte fingerprints must agree for %q", path)
	}

	// Routes are separated by length alone, without reading a byte.
	require.NotEqual(t, pathFingerprint("/routes/1"), pathFingerprint("/routes/11"))
	// ...and by their tail, which is where a bucket of sibling routes differs.
	require.NotEqual(t, pathFingerprint("/routes/998"), pathFingerprint("/routes/999"))
}

// routerTestStatics registers n static GET routes, prefix+"0" through
// prefix+"<n-1>", answered by handler: enough of them put one tree bucket past
// the thresholds at which it carries a filter.
func routerTestStatics(app *App, prefix string, n int, handler func(Ctx) error) {
	for i := range n {
		app.Get(prefix+strconv.Itoa(i), handler)
	}
}

// routerTestPathHandler answers with the path it was asked for.
func routerTestPathHandler(c Ctx) error { return c.SendString(c.Path()) }

// Test_RouteTree_Fingerprints_LineUp pins that a bucket's filter is the one
// built from that same bucket. next() indexes both with the same counter, so a
// slice that drifted by one entry would reject live routes; maxLen must come
// from the static routes alone, as it stands the hash down for longer paths.
// The heads are held to the same line-up.
func Test_RouteTree_Fingerprints_LineUp(t *testing.T) {
	t.Parallel()
	app := New()
	routerTestStatics(app, "/routes/", fingerprintMinBucket*2, testEmptyHandler)
	app.Use("/routes", func(c Ctx) error { return c.Next() })
	app.Get("/routes/:id/edit", testEmptyHandler)
	app.RebuildTree()

	routes, filter := routerTestBucket(app, MethodGet, "/ro")
	require.NotNil(t, filter, "a bucket this size must carry a filter")
	require.Len(t, filter.prints, len(routes))
	require.Len(t, filter.heads, len(routes))
	for i, route := range routes {
		require.Equal(t, staticFingerprint(route), filter.prints[i], "fingerprint %d is not this route's", i)
		require.Equal(t, newScanHead(route), filter.heads[i], "head %d is not this route's", i)
	}
	require.Equal(t, len("/routes/"+strconv.Itoa(fingerprintMinBucket*2-1)), filter.maxLen,
		"maxLen must be the longest static path, not the longer param route")
}

// Test_RouteTree_Fingerprints_SmallBucketNil pins the other half: a bucket below
// the threshold carries no filter, so a small app neither allocates the slices
// nor hashes its detection paths, and scans its routes directly.
func Test_RouteTree_Fingerprints_SmallBucketNil(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/hello", testEmptyHandler)
	app.RebuildTree()

	routes, filter := routerTestBucket(app, MethodGet, "/he")
	require.Equal(t, []string{"/hello"}, routeTreePaths(routes))
	require.Nil(t, filter, "a bucket below fingerprintMinBucket must not carry a filter")
}

// Test_RouteTree_Fingerprints_CountsStaticRoutes pins that the fingerprints'
// threshold counts static routes, not bucket size: a bucket full of param
// routes has nothing for them to reject, so it must not make every request
// hash its path. Its filter still carries heads, which do reject param routes.
func Test_RouteTree_Fingerprints_CountsStaticRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	for i := range fingerprintMinBucket * 2 {
		app.Get("/routes/:id/v"+strconv.Itoa(i), testEmptyHandler)
	}
	routerTestStatics(app, "/routes/", fingerprintMinBucket-1, testEmptyHandler)
	app.RebuildTree()

	routes, filter := routerTestBucket(app, MethodGet, "/ro")
	require.Len(t, routes, fingerprintMinBucket*3-1)
	require.NotNil(t, filter, "a bucket of filterMinBucket routes must carry a filter")
	require.Len(t, filter.heads, len(routes))
	require.Nil(t, filter.prints, "fewer static routes than fingerprintMinBucket must not carry fingerprints")

	app.Get("/routes/"+strconv.Itoa(fingerprintMinBucket-1), testEmptyHandler)
	app.RebuildTree()
	_, filter = routerTestBucket(app, MethodGet, "/ro")
	require.NotNil(t, filter, "a bucket of filterMinBucket routes must carry a filter")
	require.NotNil(t, filter.prints, "fingerprintMinBucket static routes must carry fingerprints")
}

// Test_Router_Fingerprint_HashedOnDemand pins when the scan hashes the path:
// never for a hit on the first candidate, never for a path longer than every
// static route in the bucket, and once for anything that scans past a route.
func Test_Router_Fingerprint_HashedOnDemand(t *testing.T) {
	t.Parallel()
	app := New()
	routerTestStatics(app, "/routes/", fingerprintMinBucket*2, testEmptyHandler)
	app.startupProcess()

	last := "/routes/" + strconv.Itoa(fingerprintMinBucket*2-1)
	miss := "/routes/" + strconv.Itoa(fingerprintMinBucket*2)
	long := "/routes/" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		path      string
		wantPrint uint64
		matched   bool
	}{
		{path: "/routes/0", wantPrint: 0, matched: true},
		{path: last, wantPrint: pathFingerprint(last), matched: true},
		{path: miss, wantPrint: pathFingerprint(miss), matched: false},
		{path: long, wantPrint: 0, matched: false},
	} {
		fctx := routerTestRequestCtx(MethodGet, tc.path)
		ctx := app.AcquireCtx(fctx).(*DefaultCtx) //nolint:errcheck,forcetypeassert // default app returns DefaultCtx

		matched, err := app.next(ctx)
		require.Equal(t, tc.matched, matched, tc.path)
		if tc.matched {
			require.NoError(t, err, tc.path)
		} else {
			require.ErrorIs(t, err, ErrNotFound, tc.path)
		}
		require.Equal(t, tc.wantPrint, ctx.pathPrint, "hashed state after routing %s", tc.path)
		app.ReleaseCtx(ctx)
	}
}

// Test_Router_LargeBucket_RoutesCorrectly drives the filter through the real
// request path: in every row the routes share one oversized bucket, so the
// fingerprint scan is the thing deciding what runs. Rows with mw put a /routes
// middleware in front that must still run for every request.
func Test_Router_LargeBucket_RoutesCorrectly(t *testing.T) {
	t.Parallel()
	n := fingerprintMinBucket * 4
	last, past := "/routes/"+strconv.Itoa(n-1), "/routes/"+strconv.Itoa(n)
	longest, name, long := "/routes/"+strings.Repeat("s", 40), strings.Repeat("n", 64), strings.Repeat("e", 64)
	edit := func(c Ctx) error { return c.SendString("edit:" + c.Params("id")) }
	newCustom := func() *App {
		return NewWithCustomCtx(func(app *App) CustomCtx {
			return &localsRecordingCtx{DefaultCtx: *NewDefaultCtx(app)}
		})
	}
	for _, tc := range []struct {
		register func(app *App)
		newApp   func() *App // New when nil
		name     string
		requests []routerReq
		mw       bool
	}{
		{
			// every route shape in the one bucket
			name: "AllShapes", mw: true,
			register: func(app *App) {
				routerTestStatics(app, "/routes/", n, routerTestPathHandler)
				app.Get("/routes/:id/edit", edit)
				app.Get("/routes/files/*", func(c Ctx) error { return c.SendString("star:" + c.Params("*")) })
			},
			requests: []routerReq{
				{path: "/routes/0", body: "/routes/0", status: StatusOK},
				{path: last, body: last, status: StatusOK},
				{path: "/routes/7/edit", body: "edit:7", status: StatusOK},
				{path: "/routes/files/a/b", body: "star:a/b", status: StatusOK},
				{path: "/routes/missing", status: StatusNotFound},
				{path: past, status: StatusNotFound},
			},
		},
		{
			// the default case-insensitive matching: the detection path is folded before it is
			// hashed, and routes are registered folded, so the two fingerprints have to agree
			name: "CaseInsensitive",
			register: func(app *App) {
				routerTestStatics(app, "/Routes/Item", fingerprintMinBucket*2, func(c Ctx) error { return c.SendString("hit") })
			},
			requests: []routerReq{{path: "/Routes/Item3", status: StatusOK}, {path: "/routes/item3", status: StatusOK}, {path: "/ROUTES/ITEM3", status: StatusOK}},
		},
		{
			// requests longer than every static route in the bucket, which the scan rejects
			// without hashing: they must still reach the param route behind the statics, a
			// static route as long as the bound must still match, and a miss is a 404
			name: "LongPath",
			register: func(app *App) {
				routerTestStatics(app, "/routes/", fingerprintMinBucket*2, routerTestPathHandler)
				app.Get(longest, routerTestPathHandler)
				app.Get("/routes/:name", func(c Ctx) error { return c.SendString("name:" + c.Params("name")) })
			},
			requests: []routerReq{
				{path: longest, body: longest, status: StatusOK},
				{path: "/routes/" + name, body: "name:" + name, status: StatusOK},
				{path: "/routes/" + name + "/deeper", status: StatusNotFound},
			},
		},
		{
			// the scan running off the end of a bucket made entirely of static routes: the
			// fingerprint loop rejects every entry, the scan stops without loading a Route,
			// and the request is a 404 rather than a match against the last route in the bucket
			name:     "StaticOnlyMiss",
			register: func(app *App) { routerTestStatics(app, "/routes/", n, routerTestPathHandler) },
			requests: []routerReq{{path: "/routes/missing", status: StatusNotFound}, {path: last, status: StatusOK}},
		},
		{
			// nextCustom, the scan a custom context takes, where the fingerprint is hashed per
			// call rather than kept on the context: same bucket shapes and outcomes as the
			// default context, including the long paths the scan rejects without hashing
			name: "CustomCtx", newApp: newCustom, mw: true,
			register: func(app *App) {
				routerTestStatics(app, "/routes/", n, routerTestPathHandler)
				app.Get("/routes/:id/edit", edit)
			},
			requests: []routerReq{
				{path: "/routes/0", body: "/routes/0", status: StatusOK},
				{path: last, body: last, status: StatusOK},
				{path: "/routes/9/edit", body: "edit:9", status: StatusOK},
				{path: "/routes/" + long + "/edit", body: "edit:" + long, status: StatusOK},
				{path: "/routes/missing", status: StatusNotFound},
				{path: "/routes/" + strings.Repeat("m", 64), status: StatusNotFound},
			},
		},
		{
			// the custom-context miss with no non-static route after the statics runs the scan off the end
			name: "CustomCtx_StaticOnlyMiss", newApp: newCustom,
			register: func(app *App) {
				routerTestStatics(app, "/routes/", fingerprintMinBucket*2, func(c Ctx) error { return c.SendString("ok") })
			},
			requests: []routerReq{{path: "/routes/missing", status: StatusNotFound}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			if tc.newApp != nil {
				app = tc.newApp()
			}
			var seen []string
			if tc.mw {
				app.Use("/routes", func(c Ctx) error {
					seen = append(seen, "mw")
					return c.Next()
				})
			}
			tc.register(app)
			for _, r := range tc.requests {
				seen = nil
				routerTestExpectAll(t, app, r)
				if tc.mw {
					require.Equal(t, []string{"mw"}, seen, "middleware must still run for %s", r.path)
				}
			}
		})
	}
}

// Test_Router_LargeBucket_SkipUnmatchedRoutes covers the filter meeting the
// SkipUnmatchedRoutes lookahead. The filter runs before next() consults
// firstMatchIndex, so a fingerprint that wrongly rejected the pre-resolved
// endpoint would turn a match into a 404 only when that config is on.
func Test_Router_LargeBucket_SkipUnmatchedRoutes(t *testing.T) {
	t.Parallel()
	app := New(Config{SkipUnmatchedRoutes: true})
	ran := 0
	app.Use("/routes", func(c Ctx) error {
		ran++
		return c.Next()
	})
	routerTestStatics(app, "/routes/", fingerprintMinBucket*4, routerTestPathHandler)
	app.Get("/routes/:id/edit", func(c Ctx) error { return c.SendString("edit:" + c.Params("id")) })

	last := "/routes/" + strconv.Itoa(fingerprintMinBucket*4-1)
	routerTestExpectAll(t, app, []routerReq{
		{path: "/routes/0", body: "/routes/0"},
		{path: last, body: last},
		{path: "/routes/5/edit", body: "edit:5"},
		{path: "/routes/missing", status: StatusNotFound},
	}...)
	require.Positive(t, ran, "the middleware must still have run")
}

// Test_ScanHead_SlashMask pins the slash counts newScanHead lets through for
// each route shape: the count of its path for a route without parameters, any
// count from it up for a prefix (use) one, the parser's bounds for a
// parametric route, only the lower one when it is a prefix route, and every
// count for star and root routes. Bit 0, which stands for a count the scan did
// not compute, is always set, and bit 31 stands for 31 or more.
func Test_ScanHead_SlashMask(t *testing.T) {
	t.Parallel()
	// bits returns a mask with bits lo through hi set, plus bit 0.
	bits := func(lo, hi int) uint32 {
		var mask uint32 = 1
		for n := lo; n <= hi; n++ {
			mask |= uint32(1) << n
		}
		return mask
	}
	deep := strings.Repeat("/d", 70)

	app := New()
	handler := func(c Ctx) error { return c.Next() }
	app.Get("/a/b", handler)
	app.Use("/mw", handler)
	app.Get("/p/:id", handler)
	app.Get("/o/:id?", handler)
	app.Use("/u/:id", handler)
	app.Get("/w/*", handler)
	app.Get("/*", handler)
	app.Get("/", handler)
	app.Get(deep, handler)
	app.Get(deep+"/:id", handler)
	app.startupProcess()

	want := map[string]uint32{
		"/a/b":        bits(2, 2),
		"/mw":         bits(1, 31),
		"/p/:id":      bits(2, 2),
		"/o/:id?":     bits(1, 2),
		"/u/:id":      bits(2, 31),
		"/w/*":        bits(1, 31),
		"/*":          ^uint32(0),
		"/":           ^uint32(0),
		deep:          bits(31, 31),
		deep + "/:id": bits(31, 31),
	}
	seen := 0
	for _, route := range app.stack[app.methodInt(MethodGet)] {
		mask, ok := want[route.Path]
		if !ok {
			continue
		}
		seen++
		require.Equal(t, mask, newScanHead(route).slashMask, "slash mask of %q (use %v)", route.Path, route.use)
	}
	require.Len(t, want, seen, "every shape must have been registered")

	require.Equal(t, uint32(1), slashBit(0))
	require.Equal(t, uint32(1)<<5, slashBit(5))
	require.Equal(t, uint32(1)<<31, slashBit(31))
	require.Equal(t, uint32(1)<<31, slashBit(200))
}

// Test_ScanHead_Probe pins how a head holds a route's constant probe in its
// narrow fields: a probe that fits comes back as it went in, and one they
// cannot hold, an offset past them or a mask that does not cover leading
// lanes, leaves the head without a probe, for match to check.
func Test_ScanHead_Probe(t *testing.T) {
	t.Parallel()
	word, mask := packConst("/issues")
	fits := constProbe{word: word, mask: mask, from: 13, skip: 2}
	// A filtered bucket holds a head per route: what a rebuild allocates
	// grows with this.
	require.Equal(t, uintptr(48), unsafe.Sizeof(scanHead{}))
	var h scanHead
	h.setProbe(fits)
	require.Equal(t, uint8(7), h.probeLen)
	require.Equal(t, fits, h.probe())

	full, fullMask := packConst("/comments")
	h = scanHead{}
	h.setProbe(constProbe{word: full, mask: fullMask, from: 1})
	require.Equal(t, uint8(8), h.probeLen)
	require.Equal(t, constProbe{word: full, mask: fullMask, from: 1}, h.probe())

	for _, p := range []constProbe{
		{},
		{word: word, mask: mask, from: math.MaxUint16 + 1},
		{word: word, mask: mask, from: 13, skip: math.MaxUint8 + 1},
		{word: word, mask: mask, from: -1},
		{word: word, mask: mask, from: 0},
		{word: word, mask: mask &^ 0xff00, from: 13},
	} {
		h = scanHead{}
		h.setProbe(p)
		require.Zero(t, h.probeLen, "probe %+v", p)
	}

	// Routes: the head's probe is the parser's.
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	app.Get("/repos/:owner/:repo/issues", handler)
	app.startupProcess()
	route := app.stack[app.methodInt(MethodGet)][0]
	require.NotZero(t, route.routeParser.probe.mask)
	head := newScanHead(route)
	require.Equal(t, route.routeParser.probe, head.probe())

	// headRejects agrees with the parser's probe, found or not.
	for _, path := range []string{"/repos/gofiber/fiber/issues", "/repos/gofiber/fiber/pulls", "/repos/gofiber"} {
		var byHead probeMemo
		probe := route.routeParser.probe
		require.Equal(t, probe.rejects(path), byHead.headRejects(&head, path), path)
	}
}

// Test_ScanHead_PrefixWords pins the two leading-byte words a head compares:
// the first is the route's own leading-byte filter, and the second continues
// knownPrefix past it, which is what tells apart the routes of an API that all
// begin "/api/v1/".
func Test_ScanHead_PrefixWords(t *testing.T) {
	t.Parallel()
	app := New()
	handler := func(c Ctx) error { return c.Next() }
	patterns := []string{
		"/api/v1/users/:id", "/api/v1/users", "/api/v1/posts/:id/comments",
		"/short/:x", "/:x", "/opt/:x?", "/api/v1/",
	}
	for _, pattern := range patterns {
		app.Get(pattern, handler)
	}
	app.startupProcess()

	byPath := make(map[string]*Route)
	for _, route := range app.stack[app.methodInt(MethodGet)] {
		byPath[route.Path] = route
	}
	for _, pattern := range patterns {
		route := byPath[pattern]
		require.NotNil(t, route, pattern)
		h := newScanHead(route)
		require.Equal(t, route.prefix, h.prefix, "first word of %q", pattern)
		require.Equal(t, route.prefixMask, h.prefixMask, "first mask of %q", pattern)

		prefix := knownPrefix(route)
		wantWord, wantMask := uint64(0), uint64(0)
		if len(prefix) > 8 {
			wantWord, wantMask = packConst(prefix[8:])
		}
		require.Equal(t, wantWord, h.prefix2, "second word of %q", pattern)
		require.Equal(t, wantMask, h.prefixMask2, "second mask of %q", pattern)
	}

	// The second word rejects what the first cannot tell apart.
	users := newScanHead(byPath["/api/v1/users/:id"])
	for path, rejected := range map[string]bool{
		"/api/v1/users/1":       false,
		"/api/v1/posts/1":       true,
		"/api/v1/usersx/1":      true,
		"/api/v1/":              true,
		"/api/v2/users/1":       true,
		"/api/v1/users/1/extra": true, // too many slashes for the endpoint
	} {
		var scan routeScan
		scan.init(path, pathHeadWord(path), strings.Count(path, "/"))
		require.Equal(t, rejected, users.rejects(scan.head, scan.head2, scan.slash), "path %q", path)
	}
}

// Test_Router_FilteredBucket_SharedProbeFrom routes through a filtered bucket
// whose probes share their from but read different slashes: "/foo/" follows
// the first parameter's slash, "/bar" the second's. A scan that reused the
// word one of them located for the other would reject the route it tests.
func Test_Router_FilteredBucket_SharedProbeFrom(t *testing.T) {
	t.Parallel()
	apps := map[string]*App{"default": New(), "custom": newCustomApp()}
	for name, app := range apps {
		handler := func(c Ctx) error { return c.SendString(c.Route().Path) }
		routerTestStatics(app, "/api/static", filterMinBucket, handler)
		app.Get("/api/:x/foo/*", handler)
		app.Get("/api/:x/:y/bar", handler)
		app.startupProcess()

		routes, filter := routerTestBucket(app, MethodGet, "/ap")
		require.NotNil(t, filter, "%s: the bucket must be filtered for this test to mean anything", name)
		foo, bar := filter.heads[len(routes)-2], filter.heads[len(routes)-1]
		require.NotZero(t, foo.probeLen, name)
		require.NotZero(t, bar.probeLen, name)
		require.Equal(t, foo.probeFrom, bar.probeFrom, "%s: the probes must share their from", name)
		require.NotEqual(t, foo.probeSkip, bar.probeSkip, name)

		for path, want := range map[string]string{
			"/api/1/2/bar":   "/api/:x/:y/bar",
			"/api/1/foo/z":   "/api/:x/foo/*",
			"/api/1/foo/bar": "/api/:x/foo/*",
			"/api/static3":   "/api/static3",
		} {
			resp := apptest.Do(t, app, path)
			body := apptest.Body(t, resp)
			require.Equal(t, StatusOK, resp.StatusCode, "%s: %s", name, path)
			require.Equal(t, want, body, "%s: %s", name, path)
		}
		resp := apptest.Do(t, app, "/api/1/2/baz")
		require.Equal(t, StatusNotFound, resp.StatusCode, name)
	}
}

// Test_Router_FilteredBucket_MethodNotAllowed drives the 404/405 decision
// through filtered buckets, which routeScan.endpoint walks for the methods the
// request did not use. The Allow header must name exactly the methods with a
// matching endpoint, through a default and a custom context alike.
func Test_Router_FilteredBucket_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	register := func(app *App) {
		handler := func(c Ctx) error { return c.SendString(c.Route().Path) }
		for i := range filterMinBucket * 2 {
			app.Get("/api/v1/res"+strconv.Itoa(i)+"/:id", handler)
			app.Post("/api/v1/res"+strconv.Itoa(i)+"/:id/items", handler)
		}
		app.Put("/api/v1/res3/:id", handler)
		app.Delete("/api/v1/static/path", handler)
	}
	apps := map[string]*App{"default": New(), "custom": newCustomApp()}

	for name, app := range apps {
		register(app)
		app.startupProcess()

		for _, method := range []string{MethodGet, MethodPost} {
			_, filter := routerTestBucket(app, method, "/ap")
			require.NotNil(t, filter, "%s: the %s bucket must be filtered for this test to mean anything", name, method)
		}

		for _, tc := range []struct {
			method, path, allow string
			status              int
		}{
			{method: MethodGet, path: "/api/v1/res5/7", status: StatusOK},
			{method: MethodPost, path: "/api/v1/res5/7/items", status: StatusOK},
			{method: MethodPatch, path: "/api/v1/res3/7", status: StatusMethodNotAllowed, allow: "GET, HEAD, PUT"},
			{method: MethodPatch, path: "/api/v1/res5/7/items", status: StatusMethodNotAllowed, allow: "POST"},
			{method: MethodGet, path: "/api/v1/res5/7/items", status: StatusMethodNotAllowed, allow: "POST"},
			{method: MethodPost, path: "/api/v1/res5/7", status: StatusMethodNotAllowed, allow: "GET, HEAD"},
			{method: MethodGet, path: "/api/v1/static/path", status: StatusMethodNotAllowed, allow: "DELETE"},
			{method: MethodGet, path: "/api/v1/nope/7", status: StatusNotFound},
			{method: MethodGet, path: "/api/v1/res5", status: StatusNotFound},
		} {
			resp := apptest.Do(t, app, apptest.Req(tc.method, tc.path))
			require.Equal(t, tc.status, resp.StatusCode, "%s: %s %s", name, tc.method, tc.path)
			require.Equal(t, tc.allow, resp.Header.Get(HeaderAllow), "%s: %s %s", name, tc.method, tc.path)
		}
	}
}

// Test_Router_FilteredBucket_CustomCtx runs the GitHub API corpus, whose
// larger buckets are filtered, through nextCustom, the scan a custom context
// takes, and requires the route and parameters App.next picks for each request.
func Test_Router_FilteredBucket_CustomCtx(t *testing.T) {
	t.Parallel()
	defaultApp := New()
	registerDummyRoutes(defaultApp)
	defaultApp.startupProcess()
	customApp := newCustomApp()
	registerDummyRoutes(customApp)
	customApp.startupProcess()

	filtered := 0
	for m := range defaultApp.config.RequestMethods {
		tree := defaultApp.treeIndex[m]
		for _, filter := range append([]*bucketFilter{tree.globalFilter}, tree.filters...) {
			if filter != nil {
				filtered++
			}
		}
	}
	require.NotZero(t, filtered, "the corpus must have filtered buckets for this test to mean anything")

	for _, r := range routesFixture.TestRoutes {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(r.Method)
		fctx.URI().SetPath(r.Path)

		want, ok := defaultApp.AcquireCtx(fctx).(*DefaultCtx)
		require.True(t, ok)
		wantMatched, wantErr := defaultApp.next(want)

		got := customApp.AcquireCtx(fctx)
		gotMatched, gotErr := customApp.nextCustom(got)

		require.Equal(t, wantMatched, gotMatched, "%s %s", r.Method, r.Path)
		require.Equal(t, wantErr, gotErr, "%s %s", r.Method, r.Path)
		require.Equal(t, want.Route().Path, got.Route().Path, "%s %s", r.Method, r.Path)
		n := len(want.Route().Params)
		require.Equal(t, want.values[:n], got.getValues()[:n], "%s %s", r.Method, r.Path)

		defaultApp.ReleaseCtx(want)
		customApp.ReleaseCtx(got)
	}
}

// Test_RouteTree_Heads_NotStale proves every filtered bucket's heads are the
// ones its routes derive, after each way an app gains, loses or rewrites
// routes. The heads are derived when the tree is built, from routes that are
// final by then, so this guards that no path reaches a published tree without
// a build.
func Test_RouteTree_Heads_NotStale(t *testing.T) {
	t.Parallel()
	assertFresh := func(t *testing.T, app *App, stage string) {
		t.Helper()
		checked := 0
		for m, tree := range app.treeIndex {
			check := func(routes []*Route, filter *bucketFilter) {
				if filter == nil {
					require.Less(t, len(routes), filterMinBucket, "%s: a bucket this size must be filtered", stage)
					return
				}
				require.Len(t, filter.heads, len(routes), stage)
				for i, route := range routes {
					require.Equal(t, newScanHead(route), filter.heads[i], "%s: stale head for %s %q",
						stage, app.config.RequestMethods[m], route.Path)
					checked++
				}
			}
			check(tree.globals, tree.globalFilter)
			for i, routes := range tree.buckets {
				check(routes, tree.filters[i])
			}
		}
		require.NotZero(t, checked, "%s: no filtered bucket to check", stage)
	}

	handler := func(c Ctx) error { return c.Next() }
	app := New()
	for i := range filterMinBucket {
		app.Get("/api/v1/r"+strconv.Itoa(i)+"/:id", handler)
		app.Get("/api/v1/s"+strconv.Itoa(i), handler)
	}
	app.Use("/api", handler)
	group := app.Group("/api/v1/grp", handler)
	group.Get("/:x/y", handler)

	sub := New()
	for i := range filterMinBucket {
		sub.Get("/m"+strconv.Itoa(i)+"/:id", handler)
	}
	app.Use("/api/v1/mounted", sub)

	app.startupProcess()
	assertFresh(t, app, "after startup")

	app.Get("/api/v1/late/:id", handler)
	app.RemoveRoute("/api/v1/s0")
	_ = app.RebuildTree()
	assertFresh(t, app, "after runtime registration and RebuildTree")
}

func Test_OptionalParam_ConstraintFailureDoesNotBypassAsEmpty(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use("/api/:id<int>?", func(c Ctx) error { return c.SendString("matched") })
	for path, status := range map[string]int{"/api/abc": StatusNotFound, "/api/123": StatusOK, "/api": StatusOK} {
		apptest.Expect(t, app, path, status)
	}
}

func Test_RouteParser_BacktrackMaxRetriesBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		pattern string
		status  int
	}{
		{"within_limit", "/pre:a?:b?:c?:d?:e?-end", StatusOK},
		{"exceeds_limit", "/pre:a?:b?:c?:d?:e?:f?-end", StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Get(tc.pattern, func(c Ctx) error {
				for _, name := range []string{"a", "b", "c", "d"} {
					require.Empty(t, c.Params(name))
				}
				return c.SendStatus(StatusOK)
			})

			routerTestExpect(t, app, "/pre-end", tc.status)
		})
	}
}

func Test_RouteParser_GreedyWildcardDoesNotYield(t *testing.T) {
	t.Parallel()
	app := New()
	// Greedy wildcard * must not yield to plus param +
	app.Get("/*+:x?", func(c Ctx) error { return c.SendStatus(StatusOK) })

	apptest.Expect(t, app, "/-", StatusNotFound)
}
