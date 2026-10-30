package sse

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type panicStringer struct{}

func (*panicStringer) String() string {
	panic("String must not be called for SSE data")
}

func Test_SSE_EventWritesFrame(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{
		ID:    " 42 ",
		Name:  "update",
		Data:  "one\r\ntwo",
		Retry: 2500 * time.Millisecond,
	}))
	require.NoError(t, w.Flush())

	require.Equal(t, "id: 42\nevent: update\nretry: 2500\ndata: one\ndata: two\n\n", buf.String())
}

func Test_SSE_EventRejectsFieldInjection(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.ErrorIs(t, writeEvent(w, Event{
		ID:   "42\nretry: 1",
		Data: "ignored",
	}), errInvalidField)
}

func Test_SSE_EventOmitsWhitespaceOnlyFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{
		ID:   "   ",
		Name: "   ",
		Data: "ok",
	}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: ok\n\n", buf.String())
}

func Test_SSE_EventJSONEncodesData(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{
		Name: "message",
		Data: map[string]string{"hello": "world"},
	}))
	require.NoError(t, w.Flush())

	require.JSONEq(t, `{"hello":"world"}`, stringsTrimData(buf.String()))
}

func Test_SSE_NewUsesAppJSONEncoder(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		JSONEncoder: func(any) ([]byte, error) {
			return []byte(`{"encoded":"custom"}`), nil
		},
	})
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			return stream.Event(Event{Data: map[string]string{"hello": "world"}})
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data: {\"encoded\":\"custom\"}\n\n", string(body))
}

func Test_SSE_EventJSONEncodesTypedNilStringer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	var value *panicStringer

	require.NoError(t, writeEvent(w, Event{Data: value}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: null\n\n", buf.String())
}

func Test_SSE_EventOmitsDataForUntypedNil(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{ID: "42"}))
	require.NoError(t, w.Flush())

	require.Equal(t, "id: 42\n\n", buf.String())
}

func Test_SSE_EventDoesNotWritePartialFrameWhenDataMarshalFails(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.Error(t, writeEvent(w, Event{
		ID:   "42",
		Name: "broken",
		Data: func() {},
	}))
	require.NoError(t, w.Flush())

	require.Empty(t, buf.String())
}

func Test_SSE_EventWritesRawJSONData(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{Data: json.RawMessage(`{"hello":"world"}`)}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: {\"hello\":\"world\"}\n\n", buf.String())
}

func Test_SSE_EventWritesByteSliceData(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{Data: []byte("hello\nworld")}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: hello\ndata: world\n\n", buf.String())
}

func Test_SSE_EventPreservesTrailingDataNewline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{Data: "hello\n"}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: hello\ndata: \n\n", buf.String())
}

func Test_SSE_EventPreservesIntentionalBlankDataLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeEvent(w, Event{Data: "hello\n\n"}))
	require.NoError(t, w.Flush())

	require.Equal(t, "data: hello\ndata: \ndata: \n\n", buf.String())
}

func Test_SSE_EventReturnsWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	w := bufio.NewWriterSize(errWriter{err: writeErr}, 1)

	require.ErrorIs(t, writeEvent(w, Event{Data: "hello"}), writeErr)
}

func Test_SSE_CommentSanitizesLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	require.NoError(t, writeComment(w, " first\r\nsecond "))
	require.NoError(t, w.Flush())

	require.Equal(t, ": first\n: second\n\n", buf.String())
}

func Test_SSE_CommentReturnsWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	w := bufio.NewWriterSize(errWriter{err: writeErr}, 1)

	require.ErrorIs(t, writeComment(w, ""), writeErr)
}

func Test_SSE_NewWritesHeadersAndEvents(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	var capturedLastEventID string
	app.Get("/events", New(Config{
		Retry:            time.Second,
		DisableHeartbeat: true,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			capturedLastEventID = stream.LastEventID()
			return stream.Event(Event{Name: "ready", Data: "ok"})
		},
	}))

	req := httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody)
	req.Header.Set("Last-Event-ID", "last-1")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, "last-1", capturedLastEventID)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, fiber.MIMETextEventStream, resp.Header.Get(fiber.HeaderContentType))
	require.Equal(t, "no-cache", resp.Header.Get(fiber.HeaderCacheControl))
	require.Equal(t, "keep-alive", resp.Header.Get(fiber.HeaderConnection))
	require.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "retry: 1000\n\nevent: ready\ndata: ok\n\n", string(body))
}

func Test_SSE_NewWritesHeartbeat(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get("/events", New(Config{
		HeartbeatInterval: 5 * time.Millisecond,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			select {
			case <-time.After(150 * time.Millisecond):
				return nil
			case <-stream.Done():
				return stream.Err()
			}
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), ":\n\n")
}

func Test_SSE_OnCloseReceivesNilAfterNormalClose(t *testing.T) {
	t.Parallel()

	closed := make(chan error, 1)

	app := fiber.New()
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			return stream.Event(Event{Data: "ok"})
		},
		OnClose: func(_ fiber.Ctx, err error) {
			closed <- err
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("OnClose was not called")
	}
}

func Test_SSE_StreamComment(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	defer stream.closeStream()

	require.NoError(t, stream.Comment("hello"))
	require.Equal(t, ": hello\n\n", buf.String())
}

func Test_SSE_StreamCommentReturnsWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	stream := newStream(context.Background(), bufio.NewWriterSize(errWriter{err: writeErr}, 1), "")

	require.ErrorIs(t, stream.Comment("hello"), writeErr)
	require.ErrorIs(t, stream.Err(), writeErr)
}

func Test_SSE_StreamRetryIgnoresNonPositiveDuration(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	defer stream.closeStream()

	require.NoError(t, stream.Retry(0))
	require.NoError(t, stream.Retry(-time.Second))
	require.Empty(t, buf.String())
}

func Test_SSE_StreamRetryReturnsWriterError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	stream := newStream(context.Background(), bufio.NewWriterSize(errWriter{err: writeErr}, 1), "")

	require.ErrorIs(t, stream.Retry(time.Second), writeErr)
	require.ErrorIs(t, stream.Err(), writeErr)
}

func Test_SSE_StreamConcurrentWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	defer stream.closeStream()

	const writers = 16
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			errs <- stream.Event(Event{Name: "message", Data: i})
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, writers, strings.Count(buf.String(), "event: message\n"))
	require.Equal(t, writers, strings.Count(buf.String(), "data: "))
}

func Test_SSE_StreamContextCanceledOnClose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	stream.closeStream()

	select {
	case <-stream.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("stream context was not canceled")
	}
}

func Test_SSE_NewStreamUsesBackgroundContextWhenNil(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(nil, bufio.NewWriter(&buf), "") //nolint:staticcheck // Covers the nil fallback branch in newStream.
	defer stream.closeStream()

	require.NotNil(t, stream.Context())
	require.NoError(t, stream.Context().Err())
}

func Test_SSE_StreamErrAfterNormalClose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	stream.closeStream()

	require.NoError(t, stream.Err())
	require.ErrorIs(t, stream.Event(Event{Data: "late"}), errStreamClosed)
}

func Test_SSE_StreamReturnsLatchedError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	stream := newStream(context.Background(), bufio.NewWriter(errWriter{err: writeErr}), "")

	require.ErrorIs(t, stream.Event(Event{Data: "hello"}), writeErr)
	require.ErrorIs(t, stream.Event(Event{Data: "again"}), writeErr)
}

func Test_SSE_StreamWriteError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	stream := newStream(context.Background(), bufio.NewWriter(errWriter{err: writeErr}), "")

	require.ErrorIs(t, stream.Event(Event{Data: "hello"}), writeErr)
	require.ErrorIs(t, stream.Err(), writeErr)
	select {
	case <-stream.Done():
	case <-time.After(time.Second):
		t.Fatal("stream was not closed after write error")
	}
}

func Test_SSE_InterruptedClientClosesStream(t *testing.T) {
	t.Parallel()

	const (
		testTimeout                  = time.Second
		disconnectObservationTimeout = 2 * time.Second
	)

	app := fiber.New()
	handlerDone := make(chan error, 1)
	onCloseDone := make(chan error, 1)

	app.Get("/events", New(Config{
		HeartbeatInterval: 100 * time.Millisecond,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			if err := stream.Event(Event{Data: "ready"}); err != nil {
				handlerDone <- err
				return err
			}

			<-stream.Done()
			err := stream.Err()
			handlerDone <- err
			return err
		},
		OnClose: func(_ fiber.Ctx, err error) {
			onCloseDone <- err
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody), fiber.TestConfig{
		FailOnTimeout: false,
		Timeout:       testTimeout,
	})
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.True(t, err == nil || errors.Is(err, io.ErrUnexpectedEOF))
	require.Contains(t, string(body), "data: ready\n\n")

	select {
	case err := <-handlerDone:
		require.Error(t, err)
	case <-time.After(disconnectObservationTimeout):
		t.Fatal("handler did not observe the interrupted client")
	}

	select {
	case err := <-onCloseDone:
		require.Error(t, err)
	case <-time.After(disconnectObservationTimeout):
		t.Fatal("OnClose was not called after the interrupted client")
	}
}

func Test_SSE_HandlerErrorCallsOnClose(t *testing.T) {
	t.Parallel()

	handlerErr := errors.New("boom")
	closed := make(chan error, 1)

	app := fiber.New()
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(fiber.Ctx, *Stream) error {
			return handlerErr
		},
		OnClose: func(_ fiber.Ctx, err error) {
			closed <- err
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	select {
	case err := <-closed:
		require.ErrorIs(t, err, handlerErr)
	case <-time.After(time.Second):
		t.Fatal("OnClose was not called")
	}
}

func Test_SSE_HandlerPanicCallsOnClose(t *testing.T) {
	t.Parallel()

	closed := make(chan error, 1)

	app := fiber.New()
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(fiber.Ctx, *Stream) error {
			panic("boom")
		},
		OnClose: func(_ fiber.Ctx, err error) {
			closed <- err
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	select {
	case err := <-closed:
		require.EqualError(t, err, "sse: handler panic: boom")
	case <-time.After(time.Second):
		t.Fatal("OnClose was not called")
	}
}

func Test_SSE_NewPanicsWithoutHandler(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "sse: Handler must not be nil", func() {
		New()
	})
}

func Test_SSE_StopHeartbeatIsIdempotent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	defer stream.closeStream()

	stop := stream.startHeartbeat(time.Hour)
	require.NotNil(t, stop)
	require.NotPanics(t, stop)
	require.NotPanics(t, stop)
}

func Test_SSE_StartHeartbeatReturnsNilForNonPositiveInterval(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	stream := newStream(context.Background(), bufio.NewWriter(&buf), "")
	defer stream.closeStream()

	require.Nil(t, stream.startHeartbeat(0))
	require.Nil(t, stream.startHeartbeat(-time.Second))
}

func stringsTrimData(frame string) string {
	const prefix = "event: message\ndata: "
	frame = strings.TrimPrefix(frame, prefix)
	return strings.TrimSuffix(frame, "\n\n")
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("test writer: %w", w.err)
}

// go test -v -run=^$ -bench=Benchmark_sanitizeField -benchmem -count=4
func Benchmark_sanitizeField(b *testing.B) {
	inputs := []string{
		"tick",                                 // short: scalar path
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // typical event ID
		"a longer event name for dashboards",
	}
	var s string
	var err error
	b.ReportAllocs()
	for b.Loop() {
		for _, in := range inputs {
			s, err = sanitizeField(in)
		}
	}
	_, _ = s, err
}

func Test_SSE_Retry_SubMillisecondRoundsUp(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	require.NoError(t, writeEvent(w, Event{Data: "x", Retry: 500 * time.Microsecond}))
	require.NoError(t, w.Flush())
	require.Contains(t, buf.String(), "retry: 1\n")
}

func Test_SSE_RetryMillisecondsStaysPositive(t *testing.T) {
	t.Parallel()

	// Rounding must not overflow into a negative "retry:" value.
	require.Positive(t, retryMilliseconds(time.Duration(math.MaxInt64)))
	require.Positive(t, retryMilliseconds(time.Duration(math.MaxInt64)-1))
	require.Equal(t, int64(1), retryMilliseconds(time.Nanosecond))
	require.Equal(t, int64(2), retryMilliseconds(time.Millisecond+time.Nanosecond))
}

// Middleware still uses c after c.Next, and reading the request rearranges its
// header store, so Handler must not get c before the chain has returned.
func Test_SSE_HandlerStartsAfterMiddlewareChain(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(compress.New())
	var isUnwound atomic.Bool
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		time.Sleep(5 * time.Millisecond)
		c.IPs() // walks the headers, which moves the cookies out of the header store
		isUnwound.Store(true)
		return err
	})
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(c fiber.Ctx, stream *Stream) error {
			return stream.Event(Event{Name: c.Cookies("theme"), Data: isUnwound.Load()})
		},
	}))

	for _, coding := range []string{"", "gzip"} {
		isUnwound.Store(false)
		req := httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody)
		req.Header.Set(fiber.HeaderCookie, "theme=dark")
		req.Header.Set(fiber.HeaderAcceptEncoding, coding)
		resp, err := app.Test(req)
		require.NoError(t, err)

		var body io.Reader = resp.Body
		if coding == "gzip" {
			require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
			body, err = gzip.NewReader(resp.Body)
			require.NoError(t, err)
		}
		got, err := io.ReadAll(body)
		require.NoError(t, err)
		require.Equal(t, "event: dark\ndata: true\n\n", string(got))
	}
}

// fasthttp closes a HEAD response's body unread, and the connection is free
// for the next request at once.
func Test_SSE_UnreadBodyRunsNoHandler(t *testing.T) {
	t.Parallel()

	var handlers, closes atomic.Int32
	app := fiber.New()
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(_ fiber.Ctx, stream *Stream) error {
			handlers.Add(1)
			return stream.Comment("open")
		},
		OnClose: func(fiber.Ctx, error) {
			closes.Add(1)
		},
	}))

	for range 20 {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodHead, "/events", http.NoBody))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
		require.Equal(t, fiber.MIMETextEventStream, resp.Header.Get(fiber.HeaderContentType))
	}

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/events", http.NoBody))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, ": open\n\n", string(body))
	require.Equal(t, int32(1), handlers.Load())
	require.Equal(t, int32(1), closes.Load(), "the body ends after OnClose")
}

// fasthttp reuses the RequestCtx once it has closed the response body, which it
// does when the client is gone.
func Test_SSE_CloseWaitsForCallbacks(t *testing.T) {
	t.Parallel()

	type seen struct {
		user          any
		authorization string
	}
	started := make(chan *Stream, 1)
	handlerSaw := make(chan seen, 1)
	onCloseSaw := make(chan seen, 1)
	onCloseErr := make(chan error, 1)

	app := fiber.New()
	app.Get("/events", New(Config{
		DisableHeartbeat: true,
		Handler: func(c fiber.Ctx, stream *Stream) error {
			started <- stream
			if err := stream.Comment("open"); err != nil {
				return err
			}
			<-stream.Done()
			handlerSaw <- seen{user: c.Locals("user"), authorization: utils.CopyString(c.Get(fiber.HeaderAuthorization))}
			return stream.Err()
		},
		OnClose: func(c fiber.Ctx, err error) {
			onCloseSaw <- seen{user: c.Locals("user"), authorization: utils.CopyString(c.Get(fiber.HeaderAuthorization))}
			onCloseErr <- err
		},
	}))

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/events")
	fctx.Request.Header.Set(fiber.HeaderLastEventID, "first-request")
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "Bearer first")
	fctx.SetUserValue("user", "first")
	app.Handler()(fctx)

	// The server sends part of the stream, fails to write to the gone client,
	// closes the body and parses the next request into the same RequestCtx.
	n, err := fctx.Response.BodyStream().Read(make([]byte, 64))
	require.NoError(t, err)
	require.Positive(t, n)
	require.NoError(t, fctx.Response.CloseBodyStream())
	fctx.Request.Reset()
	fctx.Request.Header.Set(fiber.HeaderAuthorization, "Bearer other")
	fctx.Request.Header.Set(fiber.HeaderLastEventID, "other-request")
	fctx.SetUserValue("user", "other")

	first := seen{user: "first", authorization: "Bearer first"}
	select {
	case got := <-handlerSaw:
		require.Equal(t, first, got)
	default:
		t.Fatal("Handler was still running after the body was closed")
	}
	select {
	case got := <-onCloseSaw:
		require.Equal(t, first, got)
		require.ErrorIs(t, <-onCloseErr, errStreamClosed)
	default:
		t.Fatal("OnClose had not run when the body was closed")
	}
	require.Equal(t, "first-request", (<-started).LastEventID())
}
