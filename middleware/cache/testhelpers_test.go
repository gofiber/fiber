package cache

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// cacheTestCReq returns a GET request for target with the request headers given
// as name and value pairs, leaving out any whose value is empty.
func cacheTestCReq(target string, headers ...string) *http.Request {
	var opts []apptest.Option
	for name, value := range apptest.Pairs(headers) {
		opts = append(opts, apptest.WithHeaderIf(name, value))
	}
	return apptest.Req(fiber.MethodGet, target, opts...)
}

// cacheTestCGet sends cacheTestCReq(target, headers...) through app and returns
// the answer with its body read.
func cacheTestCGet(t *testing.T, app *fiber.App, target string, headers ...string) reply {
	t.Helper()

	return doRequest(t, app, cacheTestCReq(target, headers...))
}

// cacheTestQuery returns a QUERY request for / that carries body, none when it is
// empty, with opts applied.
func cacheTestQuery(body string, opts ...apptest.Option) *http.Request {
	var reader io.Reader = http.NoBody
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(fiber.MethodQuery, "/", reader)
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// cacheTestServe runs GET target for example.com through h, the handler of an
// app, on a context of its own as a server would, with the request headers given
// as name and value pairs, leaving out any whose value is empty. It returns the
// context holding the answer. Unlike app.Test it leaves a streamed body unread.
func cacheTestServe(h fasthttp.RequestHandler, target string, headers ...string) *fasthttp.RequestCtx {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.Header.SetHost("example.com")
	fctx.Request.SetRequestURI(target)
	for name, value := range apptest.Pairs(headers) {
		if value != "" {
			fctx.Request.Header.Set(name, value)
		}
	}
	h(fctx)
	return fctx
}

// cacheTestXCache is the X-Cache status of the answer in fctx.
func cacheTestXCache(fctx *fasthttp.RequestCtx) string {
	return string(fctx.Response.Header.Peek("X-Cache"))
}

// cacheTestCRequireReply requires the X-Cache status and the body of got.
// msgAndArgs explain a mismatch of either.
func cacheTestCRequireReply(t *testing.T, got reply, cache, body string, msgAndArgs ...any) {
	t.Helper()

	require.Equal(t, cache, got.cache, msgAndArgs...)
	require.Equal(t, body, got.body, msgAndArgs...)
}

// cacheTestCRequireOneLine requires each of names to be sent on exactly one
// field line of resp; when says which answer of the test that was.
func cacheTestCRequireOneLine(t *testing.T, resp *http.Response, when string, names ...string) {
	t.Helper()

	for _, name := range names {
		require.Len(t, resp.Header.Values(name), 1,
			"%s must be sent on exactly one field line (%s): %v", name, when, resp.Header.Values(name))
	}
}

// cacheTestCQuery returns the target of a request whose query string holds n
// parameters, the ith of them spelled by format with i for %d (%[1]d where it
// appears more than once).
func cacheTestCQuery(n int, format string) string {
	params := make([]string, n)
	for i := range params {
		params[i] = fmt.Sprintf(format, i)
	}
	return "/?" + strings.Join(params, "&")
}

// cacheTestCEchoTenants answers with every X-Tenant field line of the request,
// each followed by a semicolon: "tenant:public;acme-private;".
func cacheTestCEchoTenants(c fiber.Ctx) error {
	values := c.Request().Header.PeekAll("X-Tenant")
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, string(v)+";")
	}
	return c.SendString("tenant:" + strings.Join(parts, ""))
}

// cacheTestCWire writes requests to app as raw HTTP/1.1 text, for what an
// http.Request cannot carry: a header sent on several field lines, or one kept in
// the spelling it was written in. The answers it returns have a status, an
// X-Cache status and a body, but no headers.
type cacheTestCWire struct {
	app *fiber.App
	t   *testing.T
	// exactNames reads the header names as they were written, which is what
	// DisableHeaderNormalizing is for.
	exactNames bool
}

// cacheTestCWireOf returns a wire to an app that serves GET / through the cache middleware configured by cfg,
// from route.
func cacheTestCWireOf(t *testing.T, cfg *Config, route fiber.Handler) cacheTestCWire {
	t.Helper()

	app := fiber.New()
	app.Use(New(*cfg))
	app.Get("/", route)
	return cacheTestCWire{t: t, app: app}
}

// cacheTestCExactWire is cacheTestCWireOf for an app that keeps the header names as written, caching for an hour.
func cacheTestCExactWire(t *testing.T, route fiber.Handler) cacheTestCWire {
	t.Helper()

	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(New(Config{Expiration: time.Hour}))
	app.Get("/", route)
	return cacheTestCWire{t: t, app: app, exactNames: true}
}

// get sends GET / for example.com with the header field lines, each written
// "Name: value".
func (w cacheTestCWire) get(lines ...string) reply {
	w.t.Helper()

	parts := []string{"GET / HTTP/1.1\r\nHost: example.com\r\n"}
	for _, line := range lines {
		parts = append(parts, line+"\r\n")
	}
	raw := strings.Join(append(parts, "\r\n"), "")

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	if w.exactNames {
		req.Header.DisableNormalizing()
	}
	require.NoError(w.t, req.Read(bufio.NewReader(strings.NewReader(raw))))
	return w.serve(req)
}

// serve runs req through the handler of the app, as the server would.
func (w cacheTestCWire) serve(req *fasthttp.Request) reply {
	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, nil, nil)
	w.app.Handler()(fctx)
	return reply{
		status: fctx.Response.StatusCode(),
		cache:  string(fctx.Response.Header.Peek("X-Cache")),
		body:   string(fctx.Response.Body()),
	}
}

// field is get with one field line under name for each of values.
func (w cacheTestCWire) field(name string, values ...string) reply {
	w.t.Helper()

	lines := make([]string, len(values))
	for i, value := range values {
		lines[i] = name + ": " + value
	}
	return w.get(lines...)
}
