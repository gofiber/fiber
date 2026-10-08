package fiber

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// Tests in this file cover protocol behavior that only shows on the wire: the
// status and headers of responses to requests fasthttp rejects before routing,
// and what a connection does after a response. They talk to a real server over
// an in-memory listener and parse what comes back with net/http.

// rawResponse is one response read off a test connection.
type rawResponse struct {
	header        http.Header
	body          string
	contentLength int64 // -1 when the response did not declare one
	status        int
	closes        bool // the response announced Connection: close
	chunked       bool // the body was sent with chunked framing
}

// What a connection did once the expected responses were read.
const (
	connClosed = "closed" // the server closed it
	connOpen   = "open"   // it stayed open and sent nothing
	connData   = "data"   // more bytes arrived than were expected
)

// startRawServer serves app on an in-memory listener until the test ends.
func startRawServer(t *testing.T, app *App) *fasthttputil.InmemoryListener {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listener(ln, ListenConfig{DisableStartupMessage: true})
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	})

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err != nil {
			return false
		}
		return conn.Close() == nil
	}, time.Second, 5*time.Millisecond)

	return ln
}

// rawExchange writes raw to a new connection and reads want responses from it.
// It then reports what the connection did next, so a test can tell a server that
// closed from one that is waiting for another request.
func rawExchange(t *testing.T, ln *fasthttputil.InmemoryListener, raw string, want int) (responses []rawResponse, after string) { //nolint:nonamedreturns // the two results are easy to swap without names
	t.Helper()

	conn, err := ln.Dial()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // already closed by the server in most tests

	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	_, err = conn.Write([]byte(raw))
	require.NoError(t, err)

	// The method matters for parsing: a response to HEAD carries no body.
	method, _, _ := strings.Cut(raw, " ")
	request := &http.Request{Method: method}

	reader := bufio.NewReader(conn)
	for range want {
		resp, readErr := http.ReadResponse(reader, request)
		require.NoError(t, readErr, "reading response %d of %d", len(responses)+1, want)
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		require.NoError(t, resp.Body.Close())
		responses = append(responses, rawResponse{
			status:        resp.StatusCode,
			header:        resp.Header,
			body:          string(body),
			closes:        resp.Close,
			contentLength: resp.ContentLength,
			chunked:       len(resp.TransferEncoding) > 0 && resp.TransferEncoding[0] == "chunked",
		})
	}

	// A closed connection answers at once; an open one stays silent until the
	// deadline, which is all the wait there is.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(250*time.Millisecond)))
	if _, peekErr := reader.Peek(1); peekErr != nil {
		switch {
		case errors.Is(peekErr, io.EOF):
			after = connClosed
		case errors.Is(peekErr, fasthttputil.ErrTimeout), errors.Is(peekErr, os.ErrDeadlineExceeded):
			after = connOpen
		default:
			require.NoError(t, peekErr)
		}
	} else {
		after = connData
	}

	return responses, after
}

// A 405 has to say what is allowed (RFC 9110 Section 15.5.6). GETOnly refuses
// every method but GET and HEAD before the router runs, so the router's own
// Allow bookkeeping never sees the request.
func Test_App_GETOnly_AllowHeader(t *testing.T) {
	t.Parallel()

	app := New(Config{GETOnly: true})
	app.Get("/", func(c Ctx) error { return c.SendString("ok") })
	ln := startRawServer(t, app)

	for _, method := range []string{MethodPost, MethodPut, MethodPatch, MethodDelete, MethodOptions} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			raw := method + " / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
			responses, _ := rawExchange(t, ln, raw, 1)
			require.Equal(t, StatusMethodNotAllowed, responses[0].status)
			require.Equal(t, "GET, HEAD", responses[0].header.Get(HeaderAllow))
		})
	}

	// What the header promises is served.
	for _, method := range []string{MethodGet, MethodHead} {
		t.Run(method+" is served", func(t *testing.T) {
			t.Parallel()

			responses, _ := rawExchange(t, ln, method+" / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n", 1)
			require.Equal(t, StatusOK, responses[0].status)
			require.Empty(t, responses[0].header.Get(HeaderAllow))
		})
	}
}

// A custom ErrorHandler that rewrites the body must not lose the header.
func Test_App_GETOnly_AllowHeader_CustomErrorHandler(t *testing.T) {
	t.Parallel()

	app := New(Config{
		GETOnly: true,
		ErrorHandler: func(c Ctx, err error) error {
			return c.SendString(err.Error())
		},
	})
	app.Post("/", func(c Ctx) error { return c.SendString("posted") })

	resp, err := app.Test(newTestRequest(t, MethodPost, "/"))
	require.NoError(t, err)
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode)
	require.Equal(t, "GET, HEAD", resp.Header.Get(HeaderAllow))
}

func newTestRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, target, http.NoBody)
	require.NoError(t, err)
	return req
}

// RFC 9112 Section 6.1: a transfer coding the server does not understand is
// answered with 501. The request is refused by fasthttp while its headers are
// read, so the status comes from serverErrorHandler.
func Test_App_serverErrorHandler_TransferEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err    error
		name   string
		status int
	}{
		{
			name:   "unknown coding",
			err:    errors.New(`error when reading request headers: unsupported transfer-encoding: "foo": buffer size=70`),
			status: StatusNotImplemented,
		},
		{
			name:   "coding list",
			err:    errors.New(`error when reading request headers: unsupported transfer-encoding: "gzip, chunked": buffer size=70`),
			status: StatusNotImplemented,
		},
		{
			// The sentinel also stands for an HTTP/1.0 message that carries the
			// field and, in secure mode, for every transfer-encoding failure. Those
			// are framing faults, not an unknown coding.
			name:   "sentinel is a framing error",
			err:    fmt.Errorf("error when reading request headers: %w: buffer size=70", fasthttp.ErrUnsupportedTransferEncoding),
			status: StatusBadRequest,
		},
		{
			name:   "repeated field is a framing error",
			err:    errors.New("error when reading request headers: too many transfer-encoding headers: buffer size=70"),
			status: StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := New()
			c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
			t.Cleanup(func() { app.ReleaseCtx(c) })

			app.serverErrorHandler(c.fasthttp, tc.err)
			require.Equal(t, tc.status, c.fasthttp.Response.StatusCode())
			// The body names the status, not the request bytes fasthttp quotes.
			if tc.status == StatusNotImplemented {
				require.Equal(t, "Not Implemented", string(c.fasthttp.Response.Body()))
			}
		})
	}
}

func Test_App_TransferEncoding_Request(t *testing.T) {
	t.Parallel()

	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString("posted") })
	ln := startRawServer(t, app)

	tests := []struct {
		name   string
		raw    string
		status int
	}{
		{
			name:   "unknown coding",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: foo\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			name:   "unknown coding before chunked",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: gzip, chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			name:   "unknown coding after chunked",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked, gzip\r\n\r\n0\r\n\r\n",
			status: StatusNotImplemented,
		},
		{
			// HTTP/1.0 has no transfer codings: the message is faulty (Section 6.1).
			name:   "HTTP/1.0 message with the field",
			raw:    "POST / HTTP/1.0\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
			status: StatusBadRequest,
		},
		{
			name:   "repeated field",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n",
			status: StatusBadRequest,
		},
		{
			name:   "chunked is served",
			raw:    "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n5\r\nhello\r\n0\r\n\r\n",
			status: StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			responses, after := rawExchange(t, ln, tc.raw, 1)
			require.Equal(t, tc.status, responses[0].status)
			if tc.status != StatusOK {
				// An unusable message leaves the connection in an unknown state.
				require.True(t, responses[0].closes, "Connection: close expected")
				require.Equal(t, connClosed, after)
			}
		})
	}
}

// RFC 9112 Section 6.1: a server may process a request that carries both
// Content-Length and Transfer-Encoding by its Transfer-Encoding alone, but must
// close the connection after responding. Otherwise the bytes that follow can be
// parsed as a request the other side of a proxy never saw (request smuggling).
func Test_App_ContentLengthWithTransferEncoding_ClosesConnection(t *testing.T) {
	t.Parallel()

	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString("first") })
	app.Get("/second", func(c Ctx) error { return c.SendString("second") })
	ln := startRawServer(t, app)

	const (
		second  = "GET /second HTTP/1.1\r\nHost: example.com\r\n\r\n"
		chunked = "\r\n0\r\n\r\n"
	)

	t.Run("served by Transfer-Encoding then closed", func(t *testing.T) {
		t.Parallel()

		for name, headers := range map[string]string{
			"Content-Length first":    "Content-Length: 4\r\nTransfer-Encoding: chunked\r\n",
			"Transfer-Encoding first": "Transfer-Encoding: chunked\r\nContent-Length: 4\r\n",
			"lower-case names":        "content-length: 4\r\ntransfer-encoding: chunked\r\n",
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				// The request after it is written in the same breath: it must go
				// unanswered.
				raw := "POST / HTTP/1.1\r\nHost: example.com\r\n" + headers + chunked + second
				responses, after := rawExchange(t, ln, raw, 1)

				require.Equal(t, StatusOK, responses[0].status)
				require.Equal(t, "first", responses[0].body)
				require.True(t, responses[0].closes, "Connection: close expected")
				require.Equal(t, connClosed, after, "the pipelined request must not be answered")
			})
		}
	})

	// Each header alone is ordinary traffic and keeps the connection alive.
	t.Run("one header alone keeps the connection", func(t *testing.T) {
		t.Parallel()

		for name, raw := range map[string]string{
			"Transfer-Encoding only": "POST / HTTP/1.1\r\nHost: example.com\r\nTransfer-Encoding: chunked\r\n" + chunked + second,
			"Content-Length only":    "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 0\r\n\r\n" + second,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				responses, after := rawExchange(t, ln, raw, 2)

				require.Equal(t, "first", responses[0].body)
				require.Equal(t, "second", responses[1].body)
				require.False(t, responses[1].closes, "Connection: close not expected")
				require.Equal(t, connOpen, after)
			})
		}
	})
}

// The hook that closes the connection sits in front of a HeaderReceived callback
// the application set, and leaves that callback's limits and timeouts in force.
func Test_App_ContentLengthWithTransferEncoding_KeepsUserHeaderReceived(t *testing.T) {
	t.Parallel()

	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString(strconv.Itoa(len(c.Body()))) })

	var calls atomic.Int32
	var chunkedSeen atomic.Bool
	app.Server().HeaderReceived = func(header *fasthttp.RequestHeader) fasthttp.RequestConfig {
		calls.Add(1)
		if header.ContentLength() == -1 {
			chunkedSeen.Store(true)
		}
		return fasthttp.RequestConfig{MaxRequestBodySize: 8}
	}
	ln := startRawServer(t, app)

	// Within the limit: served, and closed because of the two headers.
	responses, after := rawExchange(t, ln,
		"POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n", 1)
	require.Equal(t, StatusOK, responses[0].status)
	require.Equal(t, "5", responses[0].body)
	require.True(t, responses[0].closes)
	require.Equal(t, connClosed, after)

	// Over the callback's limit: refused, so the limit it returned is in force.
	responses, _ = rawExchange(t, ln,
		"POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 20\r\nConnection: close\r\n\r\n01234567890123456789", 1)
	require.Equal(t, StatusRequestEntityTooLarge, responses[0].status)

	require.Equal(t, int32(2), calls.Load(), "the application's callback runs for every request")
	require.True(t, chunkedSeen.Load(), "the callback still sees the chunked framing")
}

func Test_HasContentLengthField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "empty", raw: "", want: false},
		{name: "no fields", raw: "\r\n", want: false},
		{name: "only field", raw: "Content-Length: 4\r\n\r\n", want: true},
		{name: "among others", raw: "Host: x\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n", want: true},
		{name: "last field", raw: "Host: x\r\nContent-Length: 4", want: true},
		{name: "lower case", raw: "Host: x\r\ncontent-length: 4\r\n\r\n", want: true},
		{name: "upper case", raw: "Host: x\r\nCONTENT-LENGTH: 4\r\n\r\n", want: true},
		{name: "bare LF line ends", raw: "Host: x\nContent-Length: 4\n\n", want: true},
		{name: "no space after the colon", raw: "Host: x\r\nContent-Length:4\r\n\r\n", want: true},
		{name: "empty value", raw: "Host: x\r\nContent-Length:\r\n\r\n", want: true},
		{name: "longer name", raw: "Host: x\r\nContent-Length-Extra: 4\r\n\r\n", want: false},
		{name: "longer name ending in it", raw: "Host: x\r\nX-Content-Length: 4\r\n\r\n", want: false},
		{name: "shorter name", raw: "Host: x\r\nContent-Lengt: 4\r\n\r\n", want: false},
		{name: "name inside a value", raw: "Host: x\r\nX-Note: Content-Length: 4\r\n\r\n", want: false},
		{name: "folded continuation line", raw: "Host: x\r\nX-Note: a\r\n Content-Length: 4\r\n\r\n", want: false},
		{name: "name without a colon", raw: "Host: x\r\nContent-Length\r\n\r\n", want: false},
		{name: "name only", raw: "Content-Length", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, hasContentLengthField([]byte(tc.raw)))
		})
	}
}

func Benchmark_HasContentLengthField(b *testing.B) {
	raw := []byte("Host: example.com\r\nUser-Agent: Go-http-client/1.1\r\nAccept: */*\r\nAccept-Encoding: gzip\r\nTransfer-Encoding: chunked\r\nContent-Length: 4\r\n\r\n")

	b.ReportAllocs()
	for b.Loop() {
		if !hasContentLengthField(raw) {
			b.Fatal("Content-Length field not found")
		}
	}
}

// closeRecorder is a reader that notes it was closed.
type closeRecorder struct {
	io.Reader
	closed atomic.Bool
}

func (c *closeRecorder) Close() error {
	c.closed.Store(true)
	return nil
}

// failingReader yields some bytes and then fails.
type failingReader struct {
	io.Reader
}

var errDiskFailed = errors.New("disk failed")

func (failingReader) Read([]byte) (int, error) { return 0, errDiskFailed }

// RFC 9112 Section 6.1: a server must not send Transfer-Encoding to a client that
// did not send HTTP/1.1. A stream of unknown length can only be framed as chunks,
// so for an HTTP/1.0 request it is read to its end first and the response carries
// a Content-Length. HTTP/1.1 keeps streaming.
func Test_App_SendStream_HTTP10(t *testing.T) {
	t.Parallel()

	const (
		part1 = "first part of the stream, "
		part2 = "second part"
		body  = part1 + part2
	)

	app := New()
	var recorded atomic.Pointer[closeRecorder]
	app.Get("/stream", func(c Ctx) error {
		return c.SendStream(strings.NewReader(body))
	})
	app.Get("/closer", func(c Ctx) error {
		rec := &closeRecorder{Reader: strings.NewReader(body)}
		recorded.Store(rec)
		return c.SendStream(rec)
	})
	app.Get("/negative", func(c Ctx) error {
		return c.SendStream(strings.NewReader(body), -1)
	})
	app.Get("/sized", func(c Ctx) error {
		return c.SendStream(strings.NewReader(body), len(body))
	})
	app.Get("/writer", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			_, _ = w.WriteString(part1) //nolint:errcheck // a bufio.Writer over a buffer
			_ = w.Flush()               //nolint:errcheck // reported again by the Flush that follows the writer
			_, _ = w.WriteString(part2) //nolint:errcheck // a bufio.Writer over a buffer
		})
	})
	ln := startRawServer(t, app)

	tests := []struct {
		name       string
		request    string
		path       string
		chunked    bool // the framing the response is expected to use
		keepsAlive bool
	}{
		{name: "HTTP/1.1 stream is chunked", request: "HTTP/1.1\r\nHost: example.com", path: "/stream", chunked: true},
		{name: "HTTP/1.1 writer is chunked", request: "HTTP/1.1\r\nHost: example.com", path: "/writer", chunked: true},
		{name: "HTTP/1.0 stream has a length", request: "HTTP/1.0", path: "/stream"},
		{name: "HTTP/1.0 stream of size -1 has a length", request: "HTTP/1.0", path: "/negative"},
		{name: "HTTP/1.0 writer has a length", request: "HTTP/1.0", path: "/writer"},
		{name: "HTTP/1.0 keep-alive stream has a length", request: "HTTP/1.0\r\nConnection: keep-alive", path: "/stream", keepsAlive: true},
		{name: "HTTP/1.0 keep-alive writer has a length", request: "HTTP/1.0\r\nConnection: keep-alive", path: "/writer", keepsAlive: true},
		{name: "HTTP/1.0 sized stream is unchanged", request: "HTTP/1.0", path: "/sized"},
		{name: "HTTP/1.1 sized stream is unchanged", request: "HTTP/1.1\r\nHost: example.com", path: "/sized"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			responses, after := rawExchange(t, ln, "GET "+tc.path+" "+tc.request+"\r\n\r\n", 1)
			resp := responses[0]

			require.Equal(t, StatusOK, resp.status)
			require.Equal(t, body, resp.body)
			require.Equal(t, tc.chunked, resp.chunked, "chunked framing")
			if tc.chunked {
				require.Equal(t, int64(-1), resp.contentLength)
			} else {
				require.Equal(t, int64(len(body)), resp.contentLength)
				require.Empty(t, resp.header.Get(HeaderTransferEncoding))
			}

			if tc.keepsAlive {
				require.Equal(t, connOpen, after, "a keep-alive HTTP/1.0 client can reuse the connection")
			} else if !strings.HasPrefix(tc.request, "HTTP/1.1") {
				require.Equal(t, connClosed, after)
			}
		})
	}

	// fasthttp closes a stream once it has sent it; a stream read inside
	// SendStream is closed there.
	t.Run("HTTP/1.0 closes the stream it read", func(t *testing.T) {
		t.Parallel()

		responses, _ := rawExchange(t, ln, "GET /closer HTTP/1.0\r\n\r\n", 1)
		require.Equal(t, body, responses[0].body)
		rec := recorded.Load()
		require.NotNil(t, rec)
		require.True(t, rec.closed.Load(), "the stream is closed")
	})
}

// A stream is buffered up to Config.BodyLimit for an HTTP/1.0 client, so a request
// cannot make the server hold more than it accepts in one. Past it the response is
// a 505 that says why, and the writer of a stream that never ends sees Flush fail,
// which is how it learns to stop.
func Test_App_SendStream_HTTP10_Limit(t *testing.T) {
	t.Parallel()

	const limit = 1024
	exact := strings.Repeat("x", limit)
	over := strings.Repeat("x", limit+1)

	app := New(Config{BodyLimit: limit})
	var recorded atomic.Pointer[closeRecorder]
	var neverEndingReturned atomic.Bool
	app.Get("/exact", func(c Ctx) error { return c.SendStream(strings.NewReader(exact)) })
	app.Get("/exact-writer", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) { _, _ = w.WriteString(exact) }) //nolint:errcheck // a bufio.Writer over a buffer
	})
	app.Get("/over", func(c Ctx) error { return c.SendStream(strings.NewReader(over)) })
	app.Get("/over-closer", func(c Ctx) error {
		rec := &closeRecorder{Reader: strings.NewReader(over)}
		recorded.Store(rec)
		return c.SendStream(rec)
	})
	app.Get("/over-writer", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) { _, _ = w.WriteString(over) }) //nolint:errcheck // a bufio.Writer over a buffer
	})
	app.Get("/never-ending", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer neverEndingReturned.Store(true)
			for {
				_, _ = w.WriteString("data: tick\n\n") //nolint:errcheck // the Flush below reports it
				if err := w.Flush(); err != nil {
					return
				}
			}
		})
	})
	ln := startRawServer(t, app)

	t.Run("exactly the limit is sent", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{"/exact", "/exact-writer"} {
			responses, _ := rawExchange(t, ln, "GET "+path+" HTTP/1.0\r\n\r\n", 1)
			require.Equal(t, StatusOK, responses[0].status, path)
			require.Equal(t, exact, responses[0].body, path)
			require.Equal(t, int64(limit), responses[0].contentLength, path)
		}
	})

	t.Run("one byte more is refused", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{"/over", "/over-closer", "/over-writer", "/never-ending"} {
			responses, _ := rawExchange(t, ln, "GET "+path+" HTTP/1.0\r\n\r\n", 1)
			require.Equal(t, StatusHTTPVersionNotSupported, responses[0].status, path)
			require.Contains(t, responses[0].body, "HTTP/1.1 is needed", path)
		}
		rec := recorded.Load()
		require.NotNil(t, rec)
		require.True(t, rec.closed.Load(), "the stream that was refused is closed")
		require.Eventually(t, neverEndingReturned.Load, 5*time.Second, 5*time.Millisecond, "the writer of a stream that never ends must be told to stop")
	})

	t.Run("HTTP/1.1 streams without the limit", func(t *testing.T) {
		t.Parallel()

		responses, _ := rawExchange(t, ln, "GET /over HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n", 1)
		require.Equal(t, StatusOK, responses[0].status)
		require.Equal(t, over, responses[0].body)
		require.True(t, responses[0].chunked)
	})
}

// A stream that fails is an error of the handler, as for any other body, and a
// writer that panics is contained as before instead of taking the server down.
func Test_App_SendStream_HTTP10_Failures(t *testing.T) {
	t.Parallel()

	app := New()
	var recorded atomic.Pointer[closeRecorder]
	app.Get("/fails", func(c Ctx) error {
		rec := &closeRecorder{Reader: failingReader{}}
		recorded.Store(rec)
		return c.SendStream(rec)
	})
	app.Get("/panics", func(c Ctx) error {
		return c.SendStreamWriter(func(w *bufio.Writer) {
			_, _ = w.WriteString("partial") //nolint:errcheck // a bufio.Writer over a buffer
			panic("writer broke")
		})
	})
	app.Get("/ok", func(c Ctx) error { return c.SendString("still serving") })
	ln := startRawServer(t, app)

	responses, _ := rawExchange(t, ln, "GET /fails HTTP/1.0\r\n\r\n", 1)
	require.Equal(t, StatusInternalServerError, responses[0].status)
	require.Equal(t, errDiskFailed.Error(), responses[0].body)
	rec := recorded.Load()
	require.NotNil(t, rec)
	require.True(t, rec.closed.Load(), "a stream that failed is closed")

	responses, _ = rawExchange(t, ln, "GET /panics HTTP/1.0\r\n\r\n", 1)
	require.Equal(t, StatusInternalServerError, responses[0].status)
	require.Equal(t, "Internal Server Error", responses[0].body, "the panic value is not sent to the client")

	responses, _ = rawExchange(t, ln, "GET /ok HTTP/1.0\r\n\r\n", 1)
	require.Equal(t, "still serving", responses[0].body)
}
