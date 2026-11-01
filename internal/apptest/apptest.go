// Package apptest provides the request/response plumbing that nearly every
// Fiber test repeats: sending a request through an app, checking the transport
// error, the status and the body, borrowing a request context from an app, and
// waiting on a channel with a deadline.
//
// The helpers are generic over the app type instead of naming *fiber.App, so
// this package does not import fiber. That lets the in-package tests of the
// fiber package itself use them without an import cycle, alongside the tests of
// every middleware, extractor and client package. Type arguments are always
// inferred from the app's methods, so call sites never spell them out:
//
//	apptest.ExpectBody(t, app, "/hello", fiber.StatusOK, "world")
//
// Every helper marks itself with [testing.TB.Helper], so a failure is reported
// at the line of the call in the test. The helpers stop the test with
// FailNow and must therefore be called from the goroutine running the test.
package apptest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tester is implemented by *fiber.App. C is the app's config type
// (fiber.TestConfig) and is inferred from the app, never written out.
type Tester[C any] interface {
	Test(req *http.Request, config ...C) (*http.Response, error)
}

// Target is what a helper sends: a string is the target of a body-less GET
// request, a *http.Request is sent as it is.
type Target interface {
	string | *http.Request
}

// Option modifies a request built by [Req].
type Option func(*http.Request)

// WithHeader sets a request header, replacing any value already present.
func WithHeader(key, value string) Option {
	return func(req *http.Request) { req.Header.Set(key, value) }
}

// WithHeaderIf is [WithHeader] that leaves the request alone when value is empty.
func WithHeaderIf(key, value string) Option {
	return func(req *http.Request) {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
}

// WithHost sets the request's Host, which is how Fiber matches domain routes.
func WithHost(host string) Option {
	return func(req *http.Request) { req.Host = host }
}

// Req returns a body-less request for method and target with opts applied.
// Requests with a body or anything the options do not cover are built with
// [httptest.NewRequest] and passed to the helpers directly.
func Req(method, target string, opts ...Option) *http.Request {
	req := httptest.NewRequest(method, target, http.NoBody)
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// request resolves a Target to the request to send.
func request[T Target](target T) *http.Request {
	switch v := any(target).(type) {
	case string:
		return Req(http.MethodGet, v)
	case *http.Request:
		return v
	default:
		panic("apptest: unreachable: Target admits only string and *http.Request")
	}
}

// Do sends target through app and returns the response, failing the test if
// the app returns an error. It asserts nothing about the response itself. Pass
// the app's test config (fiber.TestConfig) as cfg to change its timeout.
func Do[C any, T Target](tb testing.TB, app Tester[C], target T, cfg ...C) *http.Response {
	tb.Helper()

	req := request(target)
	resp, err := app.Test(req, cfg...)
	require.NoError(tb, err, "%s %s", req.Method, req.URL)
	return resp
}

// Expect sends target through app and fails the test unless the response has
// the given status. A mismatch reports the response body, which usually says
// why. The response is returned for further assertions on its headers.
func Expect[C any, T Target](tb testing.TB, app Tester[C], target T, status int) *http.Response {
	tb.Helper()

	req := request(target)
	resp := Do(tb, app, req)
	if resp.StatusCode != status {
		tb.Fatalf("%s %s: status = %d, want %d; body: %q", req.Method, req.URL, resp.StatusCode, status, BodyBytes(tb, resp))
	}
	return resp
}

// ExpectBody is [Expect] that also fails the test unless the response body is
// exactly body. The body stays readable from the returned response.
func ExpectBody[C any, T Target](tb testing.TB, app Tester[C], target T, status int, body string) *http.Response {
	tb.Helper()

	req := request(target)
	resp := Expect(tb, app, req, status)
	require.Equal(tb, body, Body(tb, resp), "%s %s: body", req.Method, req.URL)
	return resp
}

// BodyBytes reads and closes the response body, failing the test on a read
// error; the body is closed even then. The body is replaced by an in-memory
// copy, so calling BodyBytes or [Body] again, or reading resp.Body afterwards,
// yields the same bytes.
func BodyBytes(tb testing.TB, resp *http.Response) []byte {
	tb.Helper()

	b, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	require.NoError(tb, err, "read response body")
	require.NoError(tb, closeErr, "close response body")
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return b
}

// Body is [BodyBytes] as a string.
func Body(tb testing.TB, resp *http.Response) string {
	tb.Helper()

	return string(BodyBytes(tb, resp))
}
