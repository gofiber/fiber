// Package sse provides a small Server-Sent Events handler for Fiber.
//
// The package focuses on the SSE transport: response headers, wire formatting,
// flushing, heartbeat comments, and disconnect detection via flush errors.
// Application-specific concerns such as topics, replay storage, authentication,
// and pub/sub fan-out intentionally stay outside the core middleware.
package sse

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

var errStreamClosed = errors.New("sse: stream closed")

// New creates a new SSE handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	if cfg.Handler == nil {
		panic("sse: Handler must not be nil")
	}

	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextEventStream)
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderConnection, "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		// A Stream kept past the request must not alias its header buffer.
		stream := newStream(c.Context(), nil, utils.CopyString(c.Get(fiber.HeaderLastEventID)), c.App().Config().JSONEncoder)

		c.Abandon()

		return c.SendStream(&streamBody{stream: stream, done: make(chan struct{}), run: func() {
			var streamErr error
			defer func() {
				if cfg.OnClose != nil {
					finalErr := streamErr
					if finalErr == nil {
						finalErr = stream.Err()
					}
					cfg.OnClose(c, finalErr)
				}
			}()
			defer func() {
				if recovered := recover(); recovered != nil {
					streamErr = fmt.Errorf("sse: handler panic: %v", recovered)
				}
			}()
			defer stream.closeStream()

			if cfg.Retry > 0 {
				streamErr = stream.Retry(cfg.Retry)
				if streamErr != nil {
					return
				}
			}

			if !cfg.DisableHeartbeat {
				stopHeartbeat := stream.startHeartbeat(cfg.HeartbeatInterval)
				if stopHeartbeat != nil {
					defer stopHeartbeat()
				}
			}

			streamErr = cfg.Handler(c, stream)
			if streamErr == nil {
				streamErr = stream.Err()
			}
		}})
	}
}

// streamBody starts Handler on the first Read, once the middleware chain has returned. Close waits
// for Handler and OnClose, as fasthttp reuses the RequestCtx next; an unread body runs neither.
type streamBody struct {
	reader    io.ReadCloser
	stream    *Stream
	run       func()
	done      chan struct{}
	closeErr  error
	startOnce sync.Once
	closeOnce sync.Once
}

func (b *streamBody) start() {
	b.reader = fasthttp.NewStreamReader(func(w *bufio.Writer) {
		defer close(b.done)
		b.stream.w = w
		b.run()
	})
}

func (b *streamBody) Read(p []byte) (int, error) {
	b.startOnce.Do(b.start)
	if b.reader == nil {
		return 0, io.ErrClosedPipe
	}
	return b.reader.Read(p) //nolint:wrapcheck // fasthttp expects the reader's own io.EOF
}

func (b *streamBody) Close() error {
	b.closeOnce.Do(func() {
		started := true
		b.startOnce.Do(func() { started = false })
		if !started {
			b.stream.closeStream()
			return
		}
		// Closing the pipe first frees a write that holds the stream lock.
		b.closeErr = b.reader.Close()
		b.stream.abort()
		<-b.done
	})
	return b.closeErr
}

// Stream is an active SSE response stream.
// Stream is safe for concurrent use.
type Stream struct {
	ctx         context.Context //nolint:containedctx // Stream exposes a per-stream context canceled with the stream lifecycle.
	cancel      context.CancelFunc
	err         error
	w           *bufio.Writer
	done        chan struct{}
	jsonMarshal utils.JSONMarshal
	lastEventID string
	isClosed    bool
	once        sync.Once
	mu          sync.Mutex
}

func newStream(ctx context.Context, w *bufio.Writer, lastEventID string, jsonMarshal ...utils.JSONMarshal) *Stream { //nolint:contextcheck // ctx is the parent for the derived stream lifecycle context.
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Stream{
		ctx:         ctx,
		cancel:      cancel,
		w:           w,
		done:        make(chan struct{}),
		jsonMarshal: jsonMarshalOrDefault(jsonMarshal),
		lastEventID: lastEventID,
	}
}

// Context returns a context canceled when the stream ends or a write fails.
func (s *Stream) Context() context.Context {
	return s.ctx
}

// Done returns a channel closed when a write fails, the server closes the
// response body, or the handler returns.
func (s *Stream) Done() <-chan struct{} {
	return s.done
}

// LastEventID returns the Last-Event-ID header value sent by the client.
func (s *Stream) LastEventID() string {
	return s.lastEventID
}

// Err returns the first stream write error.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Event writes one SSE event and flushes it to the client.
func (s *Stream) Event(event Event) error {
	return s.write(func(w *bufio.Writer) error {
		return writeEvent(w, event, s.jsonMarshal)
	})
}

// Comment writes one SSE comment and flushes it to the client.
func (s *Stream) Comment(comment string) error {
	return s.write(func(w *bufio.Writer) error {
		return writeComment(w, comment)
	})
}

// Retry writes an SSE retry field and flushes it to the client.
func (s *Stream) Retry(retry time.Duration) error {
	if retry <= 0 {
		return nil
	}
	return s.write(func(w *bufio.Writer) error {
		// Assemble the whole field in a fixed-size scratch and issue one
		// write: no fmt boxing, and a single error branch.
		var scratch [32]byte
		frame := append(scratch[:0], "retry: "...)
		frame = utils.AppendInt(frame, retryMilliseconds(retry))
		frame = append(frame, '\n', '\n')
		if _, err := w.Write(frame); err != nil {
			return fmt.Errorf("sse: write retry: %w", err)
		}
		return nil
	})
}

func (s *Stream) write(fn func(w *bufio.Writer) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}
	if s.isClosed {
		return errStreamClosed
	}
	if err := fn(s.w); err != nil {
		return s.failLocked(err)
	}
	if err := s.w.Flush(); err != nil {
		return s.failLocked(err)
	}
	return nil
}

func (s *Stream) failLocked(err error) error {
	s.err = err
	s.isClosed = true
	s.once.Do(func() {
		s.cancel()
		close(s.done)
	})
	return err
}

// abort ends a stream the handler has not finished, so Err reports it.
func (s *Stream) abort() {
	s.mu.Lock()
	if !s.isClosed {
		s.err = errStreamClosed
	}
	s.mu.Unlock()
	s.closeStream()
}

func (s *Stream) closeStream() {
	s.mu.Lock()
	s.isClosed = true
	s.mu.Unlock()
	s.once.Do(func() {
		s.cancel()
		close(s.done)
	})
}

func (s *Stream) startHeartbeat(interval time.Duration) func() {
	if interval <= 0 {
		return nil
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.Comment(""); err != nil {
					return
				}
			case <-stop:
				return
			case <-s.Done():
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(stop)
		})
		<-done
	}
}
