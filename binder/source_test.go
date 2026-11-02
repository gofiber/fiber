package binder

import (
	"bytes"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// acquireRequest returns a pooled request that goes back to the pool when the
// test ends. What a binder binds can alias the request's buffers, so the
// request has to outlive the assertions made on the result.
func acquireRequest(tb testing.TB) *fasthttp.Request {
	tb.Helper()

	req := fasthttp.AcquireRequest()
	tb.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	return req
}

// newQueryRequest returns a pooled request whose query string is query.
func newQueryRequest(tb testing.TB, query string) *fasthttp.Request {
	tb.Helper()

	req := acquireRequest(tb)
	req.URI().SetQueryString(query)
	return req
}

// newFormRequest returns a pooled request carrying body as a urlencoded form.
func newFormRequest(tb testing.TB, body string) *fasthttp.Request {
	tb.Helper()

	req := acquireRequest(tb)
	req.SetBodyString(body)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	return req
}

// newMultipartRequestWith returns a pooled request carrying the multipart form
// that write fills in.
func newMultipartRequestWith(tb testing.TB, write func(mw *multipart.Writer)) *fasthttp.Request {
	tb.Helper()

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	write(mw)
	require.NoError(tb, mw.Close())

	req := acquireRequest(tb)
	req.SetBody(body.Bytes())
	req.Header.SetContentType(mw.FormDataContentType())

	return req
}

// writeFormFile adds a file part to mw.
func writeFormFile(t *testing.T, mw *multipart.Writer, field, filename, content string) {
	t.Helper()

	part, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
}

// requireJohn42 requires that a benchmark's last bind succeeded and filled in
// the user the benchmarks bind: john, aged 42, with the posts post1 to post3.
func requireJohn42(tb testing.TB, err error, name string, age int, posts []string) {
	tb.Helper()

	require.NoError(tb, err)
	require.Equal(tb, "john", name)
	require.Equal(tb, 42, age)
	require.Equal(tb, []string{"post1", "post2", "post3"}, posts)
}

// pairsOf splits a urlencoded string into its name/value pairs, in order.
func pairsOf(urlEncoded string) [][2]string {
	var args fasthttp.Args
	args.Parse(urlEncoded)

	pairs := make([][2]string, 0, args.Len())
	for key, value := range args.All() {
		pairs = append(pairs, [2]string{string(key), string(value)})
	}
	return pairs
}

// source is a way to hand a binder name/value pairs, given as a urlencoded
// string that each source carries the way its own binder reads them.
type source struct {
	// bind decodes the pairs into out with a binder whose EnableSplitting is
	// splitting.
	bind func(t *testing.T, urlEncoded string, splitting bool, out any) error
	name string
	// list is whether the source reads a value as an HTTP list (RFC 9110
	// Section 5.6.1), whose elements carry no surrounding whitespace.
	list bool
}

// pairSources are the binders that read name/value pairs, so that a scenario
// written once as urlencoded input runs against every one of them.
var pairSources = []source{
	{name: "query", bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		return (&QueryBinding{EnableSplitting: splitting}).Bind(newQueryRequest(t, urlEncoded), out)
	}},
	{name: "form", bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		return (&FormBinding{EnableSplitting: splitting}).Bind(newFormRequest(t, urlEncoded), out)
	}},
	// Multipart takes its own branch through bindMultipart before it reaches
	// the shared decoder, so it has to be exercised in its own right.
	{name: "multipart", bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
			for _, pair := range pairsOf(urlEncoded) {
				require.NoError(t, mw.WriteField(pair[0], pair[1]))
			}
		})
		return (&FormBinding{EnableSplitting: splitting}).Bind(req, out)
	}},
	{name: "header", list: true, bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		req := acquireRequest(t)
		for _, pair := range pairsOf(urlEncoded) {
			req.Header.Add(pair[0], pair[1])
		}
		return (&HeaderBinding{EnableSplitting: splitting}).Bind(req, out)
	}},
	{name: "resp_header", list: true, bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		resp := fasthttp.AcquireResponse()
		t.Cleanup(func() { fasthttp.ReleaseResponse(resp) })
		for _, pair := range pairsOf(urlEncoded) {
			resp.Header.Add(pair[0], pair[1])
		}
		return (&RespHeaderBinding{EnableSplitting: splitting}).Bind(resp, out)
	}},
	{name: "cookie", bind: func(t *testing.T, urlEncoded string, splitting bool, out any) error {
		t.Helper()

		var cookies []string
		for _, pair := range pairsOf(urlEncoded) {
			cookies = append(cookies, pair[0]+"="+pair[1])
		}
		req := acquireRequest(t)
		req.Header.Set(fasthttp.HeaderCookie, strings.Join(cookies, "; "))
		return (&CookieBinding{EnableSplitting: splitting}).Bind(req, out)
	}},
}

// formSources are the first three of pairSources: the query string, a
// urlencoded body and a multipart body. They share one schema decoder, and with
// it the key notations (brackets and dots) that a header or a cookie has no use
// for.
var formSources = pairSources[:3]

// formCase is a urlencoded input and the value it must decode to.
type formCase[T any] struct {
	want  T
	name  string
	input string
}

// runFormSources decodes every case's input into a fresh T through each form
// source with splitting off, as a parallel subtest per source and case, and
// requires the result to equal the case's want.
func runFormSources[T any](t *testing.T, cases []formCase[T]) {
	t.Helper()

	for _, src := range formSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					var got T
					require.NoError(t, src.bind(t, tc.input, false, &got))
					require.Equal(t, tc.want, got)
				})
			}
		})
	}
}
