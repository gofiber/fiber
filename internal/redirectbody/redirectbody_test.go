package redirectbody

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// pooledRequest returns a pooled request that goes back to the pool when t ends.
func pooledRequest(t *testing.T) *fasthttp.Request {
	t.Helper()

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	return req
}

// Test_Drop_RemovesExpect covers the 100-continue expectation, which
// may not be generated without content (RFC 9110 Section 10.1.1). Left on the
// bodyless follow-up, a strict target answers 417 instead of serving it.
func Test_Drop_RemovesExpect(t *testing.T) {
	t.Parallel()
	req := pooledRequest(t)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.Set(fasthttp.HeaderExpect, "100-continue")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")

	require.True(t, Drop(req))

	require.Empty(t, req.Header.Peek(fasthttp.HeaderExpect))
	require.Empty(t, req.Body())
}

// Test_Drop_ClearsTheParsedForm pins why the PostArgs reset is there:
// Request.Write falls back to the parsed form when the body is empty, so a
// caller that had read PostArgs would otherwise have sent the dropped body on
// the redirected GET.
func Test_Drop_ClearsTheParsedForm(t *testing.T) {
	t.Parallel()
	req := pooledRequest(t)
	req.SetRequestURI("http://example.com/y")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")
	require.Equal(t, "a=1", req.PostArgs().String(), "the caller reads the form first")

	req.Header.SetMethod(fasthttp.MethodGet)
	require.True(t, Drop(req))

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, req.Write(w))
	require.NoError(t, w.Flush())
	require.NotContains(t, buf.String(), "a=1", "the dropped form must not come back as the body")
}

// Test_Drop_ReusedRequestSendsTheNewBody covers the request a caller
// gets back and then reuses without resetting it.
//
// fasthttp clears its parsed-form flag beside the values here, which this cannot
// reach: the field is unexported and only Request.Reset clears it, which would
// take the URI and headers the redirect loop is still using. What survives that
// gap has to be the part that matters — the body actually sent, and the earlier
// form not reappearing in it or in a later read. An untouched fasthttp request
// is no better here: SetBody does not invalidate the parsed form either, so it
// answers a reused PostArgs with the previous body's values.
func Test_Drop_ReusedRequestSendsTheNewBody(t *testing.T) {
	t.Parallel()
	req := pooledRequest(t)
	req.SetRequestURI("http://example.com/y")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("a=1")
	require.Equal(t, "a=1", req.PostArgs().String(), "the caller reads the form first")

	require.True(t, Drop(req))

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
