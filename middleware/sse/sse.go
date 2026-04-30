// Package sse provides small Server-Sent Events middleware for Fiber.
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
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

var errStreamClosed = errors.New("sse: stream closed")

// New creates a new middleware handler.
func New(config ...Config) fiber.Handler {
	cfg := configDefault(config...)
	if cfg.Handler == nil {
		panic("sse: Handler must not be nil")
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		c.Set(fiber.HeaderContentType, mimeTextEventStream)
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderConnection, "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		streamContext := c.Context()
		lastEventID := c.Get(fiber.HeaderLastEventID)

		c.Abandon()

		return c.SendStreamWriter(func(w *bufio.Writer) {
			stream := newStream(streamContext, w, lastEventID, c.App().Config().JSONEncoder)
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
		})
	}
}

// Stream is an active SSE response stream.
type Stream struct {
	ctx         context.Context //nolint:containedctx // Stream exposes a per-stream context canceled with the stream lifecycle.
	cancel      context.CancelFunc
	err         error
	w           *bufio.Writer
	done        chan struct{}
	jsonMarshal utils.JSONMarshal
	lastEventID string
	closed      bool
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

// Done returns a channel closed when a write fails or the handler returns.
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
		_, err := fmt.Fprintf(w, "retry: %d\n\n", retry.Milliseconds())
		if err != nil {
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
	if s.closed {
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
	s.closed = true
	s.once.Do(func() {
		s.cancel()
		close(s.done)
	})
	return err
}

func (s *Stream) closeStream() {
	s.mu.Lock()
	s.closed = true
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
