package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"maps"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// clientTestBRequest returns a pooled request that is released when the test ends.
func clientTestBRequest(t *testing.T) *Request {
	t.Helper()
	return clientTestPooled(t, AcquireRequest, ReleaseRequest)
}

// clientTestBMultiValue names the methods of a client or a request, T, that read and
// write one kind of multi-valued entry, so headers, query parameters and form fields
// run the same scenarios on both. Every scenario starts from a T that fresh returns
// and chains the calls through the returned T, as the fluent API is used. Kinds with
// no struct or delete form leave setStruct and del nil and do not register the
// scenarios that need them.
type clientTestBMultiValue[T any] struct {
	fresh     func(t *testing.T) T
	add       func(r T, key, val string) T
	set       func(r T, key, val string) T
	addMany   func(r T, m map[string][]string) T
	setMany   func(r T, m map[string]string) T
	setStruct func(r T, v any) T
	del       func(r T, keys ...string) T
	get       func(r T, key string) []string
	noun      string // as in the scenario names: "header", "param" or "form field"
}

// run registers the scenarios that hold for every kind of entry.
func (e clientTestBMultiValue[T]) run(t *testing.T) {
	t.Helper()

	t.Run("add "+e.noun, func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.add(e.add(r, "foo", "bar"), "foo", "fiber")

		require.Equal(t, []string{"bar", "fiber"}, e.get(r, "foo"))
	})

	t.Run("set "+e.noun, func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.set(e.add(r, "foo", "bar"), "foo", "fiber")

		require.Equal(t, []string{"fiber"}, e.get(r, "foo"))
	})

	t.Run("add "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.addMany(e.set(r, "foo", "bar"), map[string][]string{"foo": {"fiber", "buaa"}, "bar": {"foo"}})

		require.Equal(t, []string{"bar", "fiber", "buaa"}, e.get(r, "foo"))
		require.Equal(t, []string{"foo"}, e.get(r, "bar"))
	})

	t.Run("set "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.setMany(e.set(r, "foo", "bar"), map[string]string{"foo": "fiber", "bar": "foo"})

		require.Equal(t, []string{"fiber"}, e.get(r, "foo"))
		require.Equal(t, []string{"foo"}, e.get(r, "bar"))
	})
}

// runStruct registers the scenario that sets entries from a struct.
func (e clientTestBMultiValue[T]) runStruct(t *testing.T) {
	t.Helper()

	// The struct carries the tag of every kind: each one reads only its own.
	type args struct {
		TString   string
		TSlice    []string
		TIntSlice []int `param:"int_slice" form:"int_slice"`
		TInt      int
		TFloat    float64
		TBool     bool
	}

	t.Run("set "+e.noun+"s with struct", func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.setStruct(r, &args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Empty(t, e.get(r, "unexport"))
		require.Equal(t, []string{"5"}, e.get(r, "TInt"))
		require.Equal(t, []string{"string"}, e.get(r, "TString"))
		require.Equal(t, []string{"3.1"}, e.get(r, "TFloat"))
		require.Equal(t, []string{"true"}, e.get(r, "TBool"))
		require.Equal(t, []string{"foo", "bar"}, e.get(r, "TSlice"))
		require.Equal(t, []string{"1", "2"}, e.get(r, "int_slice"))
	})
}

// runDel registers the scenario that deletes entries.
func (e clientTestBMultiValue[T]) runDel(t *testing.T) {
	t.Helper()

	t.Run("del "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.del(e.setMany(e.set(r, "foo", "bar"), map[string]string{"foo": "fiber", "bar": "foo"}), "foo", "bar")

		require.Empty(t, e.get(r, "foo"))
		require.Empty(t, e.get(r, "bar"))
	})
}

// runOverride registers the scenarios where set and setMany replace every value a key
// holds and leave the other keys alone. The caller passes the scenario names because
// the form field's are not built from its noun: they say "form data".
func (e clientTestBMultiValue[T]) runOverride(t *testing.T, setName, setManyName string) {
	t.Helper()

	for _, tc := range []struct {
		replace func(r T) T
		name    string
	}{
		{name: setName, replace: func(r T) T { return e.set(r, "foo", "fiber") }},
		{name: setManyName, replace: func(r T) T { return e.setMany(r, map[string]string{"foo": "fiber"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := tc.replace(e.addMany(e.fresh(t), map[string][]string{"foo": {"bar", "buaa"}, "bar": {"foo"}}))

			require.Equal(t, []string{"fiber"}, e.get(r, "foo"))
			require.Equal(t, []string{"foo"}, e.get(r, "bar"))
		})
	}
}

// runIter registers the scenarios of the iterator over all entries, which all
// returns for a T.
func (e clientTestBMultiValue[T]) runIter(t *testing.T, all func(r T) iter.Seq2[string, []string]) {
	t.Helper()

	t.Run("empty iterator", func(t *testing.T) {
		t.Parallel()
		called := false
		all(e.fresh(t))(func(_ string, _ []string) bool {
			called = true
			return true
		})

		require.False(t, called)
	})

	t.Run("populated iterator", func(t *testing.T) {
		t.Parallel()
		r := e.fresh(t)
		e.addMany(r, clientAMultiEntries)

		got := maps.Collect(all(r))

		require.Contains(t, got["foo"], "bar")
		require.Contains(t, got["foo"], "fiber")
		require.Contains(t, got["bar"], "foo")

		require.Len(t, got, 2)
	})
}

// clientTestBSingleValue is [clientTestBMultiValue] for the kinds of single-valued
// entry, cookies and path parameters.
type clientTestBSingleValue[T any] struct {
	fresh     func(t *testing.T) T
	get       func(r T, key string) string
	set       func(r T, key, val string) T
	setMany   func(r T, m map[string]string) T
	setStruct func(r T, v any) T
	del       func(r T, keys ...string) T
	noun      string // as in the scenario names: "cookie" or "path param"
}

// run registers the scenarios that hold for every kind of entry.
func (e clientTestBSingleValue[T]) run(t *testing.T) {
	t.Helper()

	// The struct carries the tag of every kind: each one reads only its own.
	type args struct {
		String string `cookie:"string" path:"string"`
		Int    int    `cookie:"int" path:"int"`
	}

	t.Run("set "+e.noun, func(t *testing.T) {
		t.Parallel()
		r := e.set(e.fresh(t), "foo", "bar")
		require.Equal(t, "bar", e.get(r, "foo"))

		e.set(r, "foo", "bar1")
		require.Equal(t, "bar1", e.get(r, "foo"))
	})

	t.Run("set "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.setMany(e.fresh(t), map[string]string{"foo": "bar", "bar": "foo"})
		require.Equal(t, "bar", e.get(r, "foo"))
		require.Equal(t, "foo", e.get(r, "bar"))

		e.setMany(r, map[string]string{"foo": "bar1"})
		require.Equal(t, "bar1", e.get(r, "foo"))
		require.Equal(t, "foo", e.get(r, "bar"))
	})

	t.Run("set "+e.noun+"s with struct", func(t *testing.T) {
		t.Parallel()
		r := e.setStruct(e.fresh(t), &args{Int: 5, String: "foo"})

		require.Equal(t, "5", e.get(r, "int"))
		require.Equal(t, "foo", e.get(r, "string"))
	})

	t.Run("del "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.setMany(e.fresh(t), map[string]string{"foo": "bar", "bar": "foo"})
		require.Equal(t, "bar", e.get(r, "foo"))
		require.Equal(t, "foo", e.get(r, "bar"))

		e.del(r, "foo")
		require.Empty(t, e.get(r, "foo"))
		require.Equal(t, "foo", e.get(r, "bar"))
	})
}

// runIter registers the scenario of the iterator over all entries, which all returns
// for a T.
func (e clientTestBSingleValue[T]) runIter(t *testing.T, all func(r T) iter.Seq2[string, string]) {
	t.Helper()

	t.Run("iterate "+e.noun+"s", func(t *testing.T) {
		t.Parallel()
		r := e.setMany(e.fresh(t), map[string]string{"foo": "bar", "bar": "foo"})

		got := maps.Collect(all(r))

		require.Equal(t, "bar", got["foo"])
		require.Equal(t, "foo", got["bar"])

		require.NotPanics(t, func() {
			for _, v := range all(r) {
				if v == "bar" {
					break
				}
			}
		})

		require.Len(t, got, 2)
	})
}

// clientAMultiEntries is the multi-valued entries the iterator tests and benchmarks add.
var clientAMultiEntries = map[string][]string{"foo": {"bar", "fiber"}, "bar": {"foo"}}

// clientABenchRequest returns a pooled request that setup has filled, for a benchmark of
// an iterator over its entries. It reports allocations, so call it right before the loop.
func clientABenchRequest(b *testing.B, setup func(r *Request)) *Request {
	b.Helper()

	req := AcquireRequest()
	setup(req)
	b.ReportAllocs()

	return req
}

// The kinds of entry a request holds.
var (
	clientTestBHeaders = clientTestBMultiValue[*Request]{
		noun:    "header",
		fresh:   clientTestBRequest,
		add:     (*Request).AddHeader,
		set:     (*Request).SetHeader,
		addMany: (*Request).AddHeaders,
		setMany: (*Request).SetHeaders,
		get:     (*Request).Header,
	}

	clientTestBParams = clientTestBMultiValue[*Request]{
		noun:      "param",
		fresh:     clientTestBRequest,
		add:       (*Request).AddParam,
		set:       (*Request).SetParam,
		addMany:   (*Request).AddParams,
		setMany:   (*Request).SetParams,
		setStruct: (*Request).SetParamsWithStruct,
		del:       (*Request).DelParams,
		get:       (*Request).Param,
	}

	clientTestBFormData = clientTestBMultiValue[*Request]{
		noun:      "form field",
		fresh:     clientTestBRequest,
		add:       (*Request).AddFormData,
		set:       (*Request).SetFormData,
		addMany:   (*Request).AddFormDataWithMap,
		setMany:   (*Request).SetFormDataWithMap,
		setStruct: (*Request).SetFormDataWithStruct,
		del:       (*Request).DelFormData,
		get:       (*Request).FormData,
	}

	clientTestBCookies = clientTestBSingleValue[*Request]{
		noun:      "cookie",
		fresh:     clientTestBRequest,
		get:       (*Request).Cookie,
		set:       (*Request).SetCookie,
		setMany:   (*Request).SetCookies,
		setStruct: (*Request).SetCookiesWithStruct,
		del:       (*Request).DelCookies,
	}

	clientTestBPathParams = clientTestBSingleValue[*Request]{
		noun:      "path param",
		fresh:     clientTestBRequest,
		get:       (*Request).PathParam,
		set:       (*Request).SetPathParam,
		setMany:   (*Request).SetPathParams,
		setStruct: (*Request).SetPathParamsWithStruct,
		del:       (*Request).DelPathParams,
	}
)

// Test_Request_Setters sets each string field of a request to its values in turn:
// every call must replace the value the previous one set.
func Test_Request_Setters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		set    func(r *Request, value string) *Request
		get    func(r *Request) string
		name   string
		values []string
	}{
		{name: "method", set: (*Request).SetMethod, get: (*Request).Method, values: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "TRACE", "CUSTOM"}},
		{name: "URL", set: (*Request).SetURL, get: (*Request).URL, values: []string{"http://example.com/normal", "https://example.com/normal"}},
		{name: "user agent", set: (*Request).SetUserAgent, get: (*Request).UserAgent, values: []string{"fiber", "foo"}},
		{name: "referer", set: (*Request).SetReferer, get: (*Request).Referer, values: []string{"http://example.com", "https://example.com"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := clientTestBRequest(t)
			for _, value := range tc.values {
				req = tc.set(req, value)
				require.Equal(t, value, tc.get(req), "after setting %q", value)
			}
		})
	}
}

func Test_Request_Client(t *testing.T) {
	t.Parallel()
	client := New()
	req := AcquireRequest()

	req.SetClient(client)
	require.Equal(t, client, req.Client())
}

func Test_Request_Context(t *testing.T) {
	t.Parallel()
	req := AcquireRequest()
	ctx := req.Context()
	type ctxKey struct{}
	var key ctxKey = struct{}{}

	require.Nil(t, ctx.Value(key))

	ctx = context.WithValue(ctx, key, "string")
	req.SetContext(ctx)
	ctx = req.Context()

	v, ok := ctx.Value(key).(string)
	require.True(t, ok)
	require.Equal(t, "string", v)
}

func Test_Request_Header(t *testing.T) {
	t.Parallel()
	clientTestBHeaders.run(t)
}

func Test_Request_Headers(t *testing.T) {
	t.Parallel()
	req := clientTestBRequest(t)
	req.AddHeaders(clientAMultiEntries)

	headers := make(map[string][]string)
	for k, v := range req.Headers() {
		headers[k] = append(headers[k], v...)
	}

	require.Equal(t, []string{"bar", "fiber"}, headers["Foo"])
	require.Equal(t, []string{"foo"}, headers["Bar"])

	require.Len(t, headers, 2)
}

func Benchmark_Request_Headers(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) { r.AddHeaders(clientAMultiEntries) })

	for b.Loop() {
		for k, v := range req.Headers() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_QueryParam(t *testing.T) {
	t.Parallel()
	clientTestBParams.run(t)
	clientTestBParams.runStruct(t)
	clientTestBParams.runDel(t)
	clientTestBParams.runOverride(t, "set param overrides every value", "set params overrides every value")
}

var keysTestCases = []struct {
	name string
	del  string
	add  []string
	want []string
}{
	{name: "empty", want: []string{}},
	{name: "repeated key", add: []string{"foo", "foo", "bar", "foo"}, want: []string{"foo", "bar"}},
	{name: "interleaved keys", add: []string{"a", "b", "a", "c", "b"}, want: []string{"a", "b", "c"}},
	{name: "after Del", add: []string{"a", "b", "a", "c"}, del: "a", want: []string{"b", "c"}},
}

// clientTestBKeyed is what the Keys tests use of QueryParam and FormData, the
// types that wrap fasthttp.Args and list each of its keys once.
type clientTestBKeyed interface {
	Add(key, val string)
	Del(key string)
	Keys() []string
}

func Test_QueryParam_And_FormData_Keys(t *testing.T) {
	t.Parallel()
	for _, kind := range []struct {
		wrap func(args *fasthttp.Args) clientTestBKeyed
		name string
	}{
		{name: "QueryParam", wrap: func(args *fasthttp.Args) clientTestBKeyed { return &QueryParam{Args: args} }},
		{name: "FormData", wrap: func(args *fasthttp.Args) clientTestBKeyed { return &FormData{Args: args} }},
	} {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range keysTestCases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					args := fasthttp.AcquireArgs()
					defer fasthttp.ReleaseArgs(args)
					p := kind.wrap(args)
					for _, k := range tc.add {
						p.Add(k, "v")
					}
					if tc.del != "" {
						p.Del(tc.del)
					}
					require.Equal(t, tc.want, p.Keys())
				})
			}
		})
	}
}

func Benchmark_QueryParam_Keys(b *testing.B) {
	p := &QueryParam{Args: fasthttp.AcquireArgs()}
	defer fasthttp.ReleaseArgs(p.Args)
	for i := range 1000 {
		p.Add("k"+strconv.Itoa(i), "v")
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = p.Keys()
	}
}

func Test_Request_Params(t *testing.T) {
	t.Parallel()
	clientTestBParams.runIter(t, (*Request).Params)
}

func Benchmark_Request_Params(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) { r.AddParams(clientAMultiEntries) })

	for b.Loop() {
		for k, v := range req.Params() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_Cookie(t *testing.T) {
	t.Parallel()
	clientTestBCookies.run(t)
	clientTestBCookies.runIter(t, (*Request).Cookies)
}

func Benchmark_Request_Cookies(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) { r.SetCookies(map[string]string{"foo": "bar", "bar": "foo"}) })

	for b.Loop() {
		for k, v := range req.Cookies() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_PathParam(t *testing.T) {
	t.Parallel()
	clientTestBPathParams.run(t)
	clientTestBPathParams.runIter(t, (*Request).PathParams)

	t.Run("clear path params", func(t *testing.T) {
		t.Parallel()
		req := clientTestBRequest(t).SetPathParams(map[string]string{"foo": "bar", "bar": "foo"})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.ResetPathParams()
		require.Empty(t, req.PathParam("foo"))
		require.Empty(t, req.PathParam("bar"))
	})
}

func Benchmark_Request_PathParams(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) { r.SetPathParams(map[string]string{"foo": "bar", "bar": "foo"}) })

	for b.Loop() {
		for k, v := range req.PathParams() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_FormData(t *testing.T) {
	t.Parallel()
	clientTestBFormData.run(t)
	clientTestBFormData.runStruct(t)
	clientTestBFormData.runDel(t)
	clientTestBFormData.runOverride(
		t, "set form data overrides every value", "set form data with map overrides every value")
}

func Test_Request_File(t *testing.T) {
	t.Parallel()
	t.Run("add file", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().AddFile("../.github/index.html").AddFiles(AcquireFile(SetFileName("tmp.txt")))

		require.Equal(t, "../.github/index.html", req.File("index.html").path)
		require.Equal(t, "../.github/index.html", req.FileByPath("../.github/index.html").path)
		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
		require.Nil(t, req.File("tmp2.txt"))
		require.Nil(t, req.FileByPath("tmp2.txt"))
	})

	t.Run("add file by reader", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().AddFileWithReader("tmp.txt", io.NopCloser(strings.NewReader("world")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)

		content, err := io.ReadAll(req.File("tmp.txt").reader)
		require.NoError(t, err)
		require.Equal(t, "world", string(content))
	})

	t.Run("add files", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().AddFiles(AcquireFile(SetFileName("tmp.txt")), AcquireFile(SetFileName("foo.txt")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
		require.Equal(t, "foo.txt", req.File("foo.txt").name)
	})
}

func Test_Request_Files(t *testing.T) {
	t.Parallel()
	req := AcquireRequest()
	req.AddFile("../.github/index.html")
	req.AddFiles(AcquireFile(SetFileName("tmp.txt")))

	files := req.Files()

	require.Equal(t, "../.github/index.html", files[0].path)
	require.Nil(t, files[0].reader)

	require.Equal(t, "tmp.txt", files[1].name)
	require.Nil(t, files[1].reader)

	require.Len(t, files, 2)
}

func Benchmark_Request_Files(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) {
		r.AddFile("../.github/index.html").AddFiles(AcquireFile(SetFileName("tmp.txt")))
	})

	for b.Loop() {
		for k, v := range req.Files() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_Timeout(t *testing.T) {
	t.Parallel()
	req := AcquireRequest().SetTimeout(5 * time.Second)

	require.Equal(t, 5*time.Second, req.Timeout())
}

func Test_Request_URL_Format(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, url string }{
		{name: "invalid URL", url: "http://example.com\r\n\r\nGET /\r\n\r\n"},
		{name: "missing scheme separator", url: "http//example"},
		{name: "unsupported protocol", url: "ftp://example.com"},
	} {
		for _, from := range []struct {
			request func() *Request
			name    string
		}{
			{name: "request", request: AcquireRequest},
			{name: "client", request: func() *Request { return New().R() }},
		} {
			t.Run(tc.name+"/"+from.name, func(t *testing.T) {
				t.Parallel()
				resp, err := from.request().Get(tc.url)

				require.Equal(t, ErrURLFormat, err)
				require.Equal(t, (*Response)(nil), resp)
			})
		}
	}
}

// Test_Request_Methods_With_Server sends five requests through each method that
// sends a request (Get, Post, Head, Put, Delete, Options, Patch, Query and Send)
// to a route registered for the HTTP method it uses, and checks the status and
// body of every response.
func Test_Request_Methods_With_Server(t *testing.T) {
	t.Parallel()
	const target = "http://example.com"

	reply := func(status int, body string) fiber.Handler {
		return func(c fiber.Ctx) error { return c.Status(status).SendString(body) }
	}

	// What the requests carry and how they are sent.
	withForm := func(r *Request) { r.SetFormData("foo", "bar") }
	withRawBody := func(r *Request) { r.SetRawBody([]byte("query body")) }
	send := func(r *Request, url string) (*Response, error) {
		return r.SetURL(url).SetMethod(fiber.MethodPost).Send()
	}

	tests := []struct {
		register func(app *fiber.App, path string, handler any, handlers ...any) fiber.Router
		handler  fiber.Handler
		call     func(r *Request, url string) (*Response, error)
		setup    func(r *Request)
		name     string
		body     string
		status   int
	}{
		{name: "get", register: (*fiber.App).Get, handler: clientAHostname, call: (*Request).Get, status: fiber.StatusOK, body: "example.com"},
		{name: "post", register: (*fiber.App).Post, handler: clientACreated, setup: withForm, call: (*Request).Post, status: fiber.StatusCreated, body: "bar"},
		// A response to HEAD carries no body, whatever the handler sends.
		{name: "head", register: (*fiber.App).Head, handler: clientAHostname, call: (*Request).Head, status: fiber.StatusOK},
		{name: "put", register: (*fiber.App).Put, handler: clientAForm, setup: withForm, call: (*Request).Put, status: fiber.StatusOK, body: "bar"},
		// A 204 response carries no body, whatever the handler sends.
		{name: "delete", register: (*fiber.App).Delete, handler: reply(fiber.StatusNoContent, "deleted"), call: (*Request).Delete, status: fiber.StatusNoContent},
		{name: "options", register: (*fiber.App).Options, handler: reply(fiber.StatusOK, "options"), call: (*Request).Options, status: fiber.StatusOK, body: "options"},
		{name: "send", register: (*fiber.App).Post, handler: reply(fiber.StatusOK, "post"), call: send, status: fiber.StatusOK, body: "post"},
		{name: "patch", register: (*fiber.App).Patch, handler: clientAForm, setup: withForm, call: (*Request).Patch, status: fiber.StatusOK, body: "bar"},
		{name: "query", register: (*fiber.App).Query, handler: clientAEcho, setup: withRawBody, call: (*Request).Query, status: fiber.StatusOK, body: "query body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := clientTestAClient(t, func(app *fiber.App) { tc.register(app, "/", tc.handler) })

			for range 5 {
				req := AcquireRequest().SetClient(client)
				if tc.setup != nil {
					tc.setup(req)
				}

				resp, err := tc.call(req, target)
				require.NoError(t, err)
				require.Equal(t, tc.status, resp.StatusCode())
				require.Equal(t, tc.body, resp.String())
				resp.Close()
			}
		})
	}
}

func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
	t.Helper()

	basename := filepath.Base(filename)
	require.Equal(t, fh.Filename, basename)

	b1, err := os.ReadFile(filepath.Clean(filename))
	require.NoError(t, err)

	b2 := make([]byte, fh.Size)
	f, err := fh.Open()
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	_, err = f.Read(b2)
	require.NoError(t, err)
	require.Equal(t, b1, b2)
}

func Test_Request_Body_With_Server(t *testing.T) {
	t.Parallel()
	// The tags of both encodings: each reads only its own.
	type args struct {
		Content string `xml:"content" cbor:"content"`
	}

	for _, tc := range []struct {
		set         func(r *Request)
		name        string
		contentType string
		want        string
	}{
		{name: "json body", contentType: "application/json", set: func(r *Request) { r.SetJSON(map[string]string{"success": "hello"}) }, want: "{\"success\":\"hello\"}"},
		{name: "xml body", contentType: "application/xml", set: func(r *Request) { r.SetXML(args{Content: "hello"}) }, want: "<args><content>hello</content></args>"},
		{name: "cbor body", contentType: "application/cbor", set: func(r *Request) { r.SetCBOR(args{Content: "hello"}) }, want: "\xa1gcontentehello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testRequest(
				t,
				func(c fiber.Ctx) error {
					require.Equal(t, tc.contentType, string(c.Request().Header.ContentType()))
					return c.SendString(string(c.Request().Body()))
				},
				tc.set,
				tc.want,
			)
		})
	}

	t.Run("formdata", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				require.Equal(t, fiber.MIMEApplicationForm, string(c.Request().Header.ContentType()))
				return c.Send([]byte("foo=" + c.FormValue("foo") + "&bar=" + c.FormValue("bar") + "&fiber=" + c.FormValue("fiber")))
			},
			func(agent *Request) {
				agent.SetFormData("foo", "bar").SetFormDataWithMap(map[string]string{"bar": "baz", "fiber": "fast"})
			},
			"foo=bar&bar=baz&fiber=fast")
	})

	t.Run("multipart form", func(t *testing.T) {
		t.Parallel()
		client := clientTestAClient(t, func(app *fiber.App) {
			app.Post("/", func(c fiber.Ctx) error {
				require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

				mf, err := c.MultipartForm()
				require.NoError(t, err)
				require.Equal(t, "bar", mf.Value["foo"][0])

				return c.Send(c.Request().Body())
			})
		})

		req := AcquireRequest().SetClient(client).SetBoundary("myBoundary").SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		require.Equal(t, "myBoundary", req.Boundary())

		resp, err := req.Post("http://example.com")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())

		form, err := multipart.NewReader(bytes.NewReader(resp.Body()), "myBoundary").ReadForm(1024 * 1024)
		require.NoError(t, err)
		require.Equal(t, "bar", form.Value["foo"][0])
		resp.Close()
	})

	t.Run("multipart form send file", func(t *testing.T) {
		t.Parallel()
		client := clientTestAClient(t, func(app *fiber.App) {
			app.Post("/", func(c fiber.Ctx) error {
				require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

				fh1, err := c.FormFile("field1")
				require.NoError(t, err)
				require.Equal(t, "name", fh1.Filename)
				buf := make([]byte, fh1.Size)
				f, err := fh1.Open()
				require.NoError(t, err)
				defer func() { require.NoError(t, f.Close()) }()
				_, err = f.Read(buf)
				require.NoError(t, err)
				require.Equal(t, "form file", string(buf))

				fh2, err := c.FormFile("file2")
				require.NoError(t, err)
				checkFormFile(t, fh2, "../.github/testdata/index.html")

				fh3, err := c.FormFile("file3")
				require.NoError(t, err)
				checkFormFile(t, fh3, "../.github/testdata/index.tmpl")

				return c.SendString("multipart form files")
			})
		})

		for range 5 {
			req := AcquireRequest().SetClient(client).
				AddFiles(
					AcquireFile(
						SetFileFieldName("field1"),
						SetFileName("name"),
						SetFileReader(io.NopCloser(bytes.NewReader([]byte("form file")))),
					),
				).
				AddFile("../.github/testdata/index.html").
				AddFile("../.github/testdata/index.tmpl").
				SetBoundary("myBoundary")

			resp, err := req.Post("http://example.com")
			require.NoError(t, err)
			require.Equal(t, "multipart form files", resp.String())

			resp.Close()
		}
	})

	t.Run("multipart random boundary", func(t *testing.T) {
		t.Parallel()
		client := clientTestAClient(t, func(app *fiber.App) {
			app.Post("/", func(c fiber.Ctx) error {
				reg := regexp.MustCompile(`multipart/form-data; boundary=[\-\w]{33}`)
				require.True(t, reg.MatchString(c.Get(fiber.HeaderContentType)))

				return c.Send(c.Request().Body())
			})
		})

		req := AcquireRequest().SetClient(client).SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		resp, err := req.Post("http://example.com")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
	})

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()
		testRequest(
			t,
			func(c fiber.Ctx) error { return c.SendString(string(c.Request().Body())) },
			func(agent *Request) { agent.SetRawBody([]byte("hello")) },
			"hello",
		)
	})
}

func Test_Request_AllFormData(t *testing.T) {
	t.Parallel()
	clientTestBFormData.runIter(t, (*Request).AllFormData)
}

func Benchmark_Request_AllFormData(b *testing.B) {
	req := clientABenchRequest(b, func(r *Request) { r.AddFormDataWithMap(clientAMultiEntries) })

	for b.Loop() {
		for k, v := range req.AllFormData() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_Error_Body_With_Server(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		set  func(r *Request)
		want error
		name string
	}{
		{name: "json error", set: func(r *Request) { r.SetJSON(complex(1, 1)) }, want: errors.New("json: unsupported type: complex128")},
		{name: "xml error", set: func(r *Request) { r.SetXML(complex(1, 1)) }, want: errors.New("xml: unsupported type: complex128")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testRequestFail(t, func(c fiber.Ctx) error { return c.SendString("") }, tc.set, tc.want)
		})
	}

	t.Run("form body with invalid boundary", func(t *testing.T) {
		t.Parallel()
		_, err := AcquireRequest().SetBoundary("*").
			AddFileWithReader("t.txt", io.NopCloser(strings.NewReader("world"))).
			Get("http://example.com")
		require.Equal(t, "set boundary error: mime: invalid boundary character", err.Error())
	})

	t.Run("open non exist file", func(t *testing.T) {
		t.Parallel()
		_, err := AcquireRequest().AddFile("non-exist-file!").Get("http://example.com")
		require.Contains(t, err.Error(), "open non-exist-file!")
	})
}

func Test_Request_Timeout_With_Server(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			time.Sleep(time.Millisecond * 200)
			return c.SendString("timeout")
		})
	})

	_, err := AcquireRequest().SetClient(client).SetTimeout(50 * time.Millisecond).Get("http://example.com")

	require.Equal(t, ErrTimeoutOrCancel, err)
}

func Test_Request_MaxRedirects(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			if c.Request().URI().QueryArgs().Has("foo") {
				return c.Redirect().To("/foo")
			}
			return c.Redirect().To("/")
		})
		app.Get("/foo", func(c fiber.Ctx) error { return c.SendString("redirect") })
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		resp, err := AcquireRequest().SetClient(client).SetMaxRedirects(1).Get("http://example.com?foo")
		body := resp.String()
		code := resp.StatusCode()

		require.Equal(t, 200, code)
		require.Equal(t, "redirect", body)
		require.NoError(t, err)

		resp.Close()
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		resp, err := AcquireRequest().SetClient(client).SetMaxRedirects(1).Get("http://example.com")

		require.Nil(t, resp)
		require.ErrorIs(t, err, fasthttp.ErrTooManyRedirects)
	})

	t.Run("MaxRedirects", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().SetClient(client).SetMaxRedirects(3)

		require.Equal(t, 3, req.MaxRedirects())
	})
}

func Test_Request_RedirectSensitiveHeaders(t *testing.T) {
	t.Parallel()
	t.Run("the list", func(t *testing.T) {
		t.Parallel()
		req := clientTestBRequest(t)
		requireNameList(t, req, (*Request).SetRedirectSensitiveHeaders, (*Request).RedirectSensitiveHeaders)
	})

	t.Run("Reset clears the list", func(t *testing.T) {
		t.Parallel()
		req := clientTestBRequest(t).SetRedirectSensitiveHeaders("X-Api-Key")

		req.Reset()
		require.Nil(t, req.RedirectSensitiveHeaders())
	})
}

func Test_Request_RedirectSensitiveHeaders_EndToEnd(t *testing.T) {
	t.Parallel()
	server := startRedirectServer(t, func(c fiber.Ctx) error {
		return c.SendString(c.Get("X-Api-Key") + "|" + c.Get("X-Auth-Token") + "|" + c.Get("X-Trace-Id"))
	})

	headers := map[string]string{"X-Api-Key": "key", "X-Auth-Token": "tok", "X-Trace-Id": "trace"}

	for _, tc := range []struct {
		prepare func(client *Client)
		send    func(client *Client, url string) (*Response, error)
		name    string
		want    string
	}{
		{
			name: "nothing listed forwards every header",
			send: func(client *Client, url string) (*Response, error) {
				return client.Get(url, Config{Header: headers, MaxRedirects: 5})
			},
			want: "key|tok|trace",
		},
		{
			name: "Config",
			send: func(client *Client, url string) (*Response, error) {
				return client.Get(url, Config{Header: headers, MaxRedirects: 5, RedirectSensitiveHeaders: []string{"X-Api-Key"}})
			},
			want: "|tok|trace",
		},
		{
			name: "Request",
			send: func(client *Client, url string) (*Response, error) {
				return client.R().SetHeaders(headers).SetMaxRedirects(5).SetRedirectSensitiveHeaders("x-api-key").Get(url)
			},
			want: "|tok|trace",
		},
		{
			name:    "Client",
			prepare: func(client *Client) { client.SetHeaders(headers).SetRedirectSensitiveHeaders("X-Api-Key") },
			send: func(client *Client, url string) (*Response, error) {
				return client.Get(url, Config{MaxRedirects: 5})
			},
			want: "|tok|trace",
		},
		{
			name:    "the client's names and the request's are added together",
			prepare: func(client *Client) { client.SetHeaders(headers).SetRedirectSensitiveHeaders("X-Api-Key") },
			send: func(client *Client, url string) (*Response, error) {
				return client.R().SetMaxRedirects(5).SetRedirectSensitiveHeaders("X-Auth-Token").Get(url)
			},
			want: "||trace",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := New().SetDial(server.dial())
			if tc.prepare != nil {
				tc.prepare(client)
			}

			same, err := tc.send(client, "http://example.com/same-host")
			require.NoError(t, err)
			defer same.Close()
			require.Equal(t, "key|tok|trace", same.String())

			resp, err := tc.send(client, "http://example.com/start")
			require.NoError(t, err)
			defer resp.Close()
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, tc.want, resp.String())

			require.Equal(t, "key", string(resp.request.RawRequest.Header.Peek("X-Api-Key")))
		})
	}
}

func Test_Request_Query_Redirect(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		// 308 preserves method and body, so QUERY must follow it and keep its body.
		app.Query("/start", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusPermanentRedirect).To("/end")
		})
		app.Query("/end", func(c fiber.Ctx) error { return c.Send(c.Body()) })
	})

	resp, err := AcquireRequest().SetClient(client).SetMaxRedirects(1).SetRawBody([]byte("query body")).
		Query("http://example.com/start")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "query body", resp.String())

	resp.Close()
}

// clientTestBStructArgs is what the SetValWithStruct test and benchmark pass, as a
// value and through a pointer.
type clientTestBStructArgs struct {
	TString   string
	TSlice    []string
	TIntSlice []int `param:"int_slice"`
	unexport  int
	TInt      int
	TUint     uint
	TFloat    float64
	TComplex  complex128
	TBool     bool
}

// clientTestBStructParams is what SetValWithStruct must have written: the value a
// key holds, and values that the list under a key must contain.
type clientTestBStructParams struct {
	single map[string]string
	multi  map[string][]string
}

// require fails tb unless p holds what w describes.
func (w clientTestBStructParams) require(tb testing.TB, p *QueryParam) {
	tb.Helper()

	for key, want := range w.single {
		require.Equal(tb, want, string(p.Peek(key)), key)
	}
	for key, wants := range w.multi {
		for _, want := range wants {
			require.Contains(tb, p.PeekMulti(key), []byte(want), key)
		}
	}
}

// What SetValWithStruct writes for the value and for the pointer the test and the
// benchmark pass.
var (
	clientTestBValueParams = clientTestBStructParams{
		single: map[string]string{
			"unexport": "", "TInt": "5", "TUint": "5", "TString": "string",
			"TFloat": "3.1", "TComplex": "(3+4i)", "TBool": "false",
		},
		multi: map[string][]string{"TSlice": {"foo", "bar"}, "int_slice": {"0", "1", "2"}},
	}

	clientTestBPointerParams = clientTestBStructParams{
		single: map[string]string{"TInt": "5", "TString": "string", "TFloat": "3.1", "TBool": "true"},
		multi:  map[string][]string{"TSlice": {"foo", "bar"}, "int_slice": {"1", "2"}},
	}
)

func Test_SetValWithStruct(t *testing.T) {
	t.Parallel()
	type args = clientTestBStructArgs

	for _, tc := range []struct {
		v    any
		want clientTestBStructParams
		name string
	}{
		{
			name: "the struct should be applied",
			v: args{
				unexport:  5,
				TInt:      5,
				TUint:     5,
				TString:   "string",
				TFloat:    3.1,
				TComplex:  3 + 4i,
				TBool:     false,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{0, 1, 2},
			},
			want: clientTestBValueParams,
		},
		{
			name: "the pointer of a struct should be applied",
			v: &args{
				TInt:      5,
				TString:   "string",
				TFloat:    3.1,
				TBool:     true,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{1, 2},
			},
			want: clientTestBPointerParams,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &QueryParam{Args: fasthttp.AcquireArgs()}

			SetValWithStruct(p, "param", tc.v)

			tc.want.require(t, p)
		})
	}

	t.Run("error type should ignore", func(t *testing.T) {
		t.Parallel()
		p := &QueryParam{Args: fasthttp.AcquireArgs()}
		SetValWithStruct(p, "param", 5)
		require.Equal(t, 0, p.Len())
	})
}

func Benchmark_SetValWithStruct(b *testing.B) {
	type args = clientTestBStructArgs

	b.Run("the struct should be applied", func(b *testing.B) {
		p := &QueryParam{Args: fasthttp.AcquireArgs()}

		b.ReportAllocs()

		for b.Loop() {
			SetValWithStruct(p, "param", args{
				unexport:  5,
				TInt:      5,
				TUint:     5,
				TString:   "string",
				TFloat:    3.1,
				TComplex:  3 + 4i,
				TBool:     false,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{0, 1, 2},
			})
		}

		clientTestBValueParams.require(b, p)
	})

	b.Run("the pointer of a struct should be applied", func(b *testing.B) {
		p := &QueryParam{Args: fasthttp.AcquireArgs()}

		b.ReportAllocs()

		for b.Loop() {
			SetValWithStruct(p, "param", &args{
				TInt:      5,
				TString:   "string",
				TFloat:    3.1,
				TBool:     true,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{1, 2},
			})
		}

		clientTestBPointerParams.require(b, p)
	})

	b.Run("error type should ignore", func(b *testing.B) {
		p := &QueryParam{Args: fasthttp.AcquireArgs()}

		b.ReportAllocs()

		for b.Loop() {
			SetValWithStruct(p, "param", 5)
		}

		require.Equal(b, 0, p.Len())
	})
}
