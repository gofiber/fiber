package aigateway

import (
	"bufio"
	"errors"
	"io"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/utils/v2"
)

var (
	errResponseTooLarge  = errors.New("aigateway: upstream response exceeds MaxResponseSize")
	errStreamIdleTimeout = errors.New("aigateway: upstream stream idle timeout")
	errStreamAbandoned   = errors.New("aigateway: stream abandoned")
)

// isEventStream reports whether the upstream response is Server-Sent Events.
func isEventStream(resp *client.Response) bool {
	const eventStream = "text/event-stream"
	ct := resp.RawResponse.Header.ContentType()
	return len(ct) >= len(eventStream) && utils.EqualFold(string(ct[:len(eventStream)]), eventStream)
}

// abortUpstreamResponse releases a response whose body was not fully read.
// The body stream must be closed with a non-nil error first: a plain
// resp.Close() closes it with nil, which would hand a connection with
// unread body bytes back to the pool for reuse.
func abortUpstreamResponse(resp *client.Response) {
	if cr, ok := resp.RawResponse.BodyStream().(fasthttp.ReadCloserWithError); ok {
		cr.CloseWithError(errStreamAbandoned) //nolint:errcheck // teardown is best-effort
	}
	resp.Close()
}

// relayBuffered reads the full upstream response and sends it to the client.
// The usage hook fires synchronously before returning.
func relayBuffered(c fiber.Ctx, cfg *Config, resp *client.Response, ev *UsageEvent, start time.Time) error {
	reader := resp.BodyStream()
	if cfg.MaxResponseSize > 0 {
		reader = io.LimitReader(reader, cfg.MaxResponseSize+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		abortUpstreamResponse(resp)
		ev.Err = err
		fireUsage(cfg, ev, start)
		return fiber.ErrBadGateway
	}
	if cfg.MaxResponseSize > 0 && int64(len(body)) > cfg.MaxResponseSize {
		abortUpstreamResponse(resp)
		ev.Err = errResponseTooLarge
		fireUsage(cfg, ev, start)
		return fiber.ErrBadGateway
	}
	copyResponseHeaders(c, resp)
	c.Status(resp.StatusCode())
	ev.StatusCode = resp.StatusCode()
	// The body was consumed to EOF, so closing releases the connection for
	// reuse. Headers were copied off the pooled response above.
	resp.Close()

	ev.ResponseBytes = int64(len(body))
	ev.Usage = parseUsage(body, c.App().Config().JSONDecoder)
	fireUsage(cfg, ev, start)

	return c.Send(body)
}

// streamChunk carries one upstream read result from the reader goroutine to
// the response writer.
type streamChunk struct {
	err  error
	data []byte
}

// relayStream pipes the upstream body to the client chunk by chunk, flushing
// after every read so SSE tokens arrive as they are generated. The usage hook
// fires on the stream writer goroutine after the stream ends.
//
// Concurrency: fasthttp's streamed response body is not safe to close while
// a Read is blocked on it (closing releases pooled internals under the
// reader). A dedicated reader goroutine is therefore the sole owner of the
// upstream response: it reads, hands chunks over a channel, and tears the
// response down when the stream ends or the writer signals abandonment. On
// abandonment (client disconnect, idle timeout, size cap) a reader blocked
// in Read lingers until the upstream delivers its next byte or closes; it
// then closes the upstream connection, which stops token generation.
func relayStream(c fiber.Ctx, cfg *Config, resp *client.Response, ev *UsageEvent, start time.Time) error {
	copyResponseHeaders(c, resp)
	c.Status(resp.StatusCode())
	// Ask reverse proxies (nginx et al.) not to buffer this response.
	c.Set("X-Accel-Buffering", "no")
	ev.StatusCode = resp.StatusCode()

	// Everything the goroutines below touch must be captured here: they run
	// after the handler returns, when the fiber.Ctx may already be recycled.
	stream := resp.BodyStream()
	idle := cfg.StreamIdleTimeout
	maxSize := cfg.MaxResponseSize
	decoder := c.App().Config().JSONDecoder
	onUsage := cfg.OnUsage

	chunks := make(chan streamChunk)
	done := make(chan struct{})

	go readStream(stream, resp, chunks, done)

	return c.SendStreamWriter(func(w *bufio.Writer) {
		// Closing done releases the reader goroutine from a pending chunk
		// hand-off and tells it to tear down the upstream response.
		defer close(done)

		idleTimer := time.NewTimer(idle)
		defer idleTimer.Stop()

		tail := &usageTail{}
	loop:
		for {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(idle)

			select {
			case chunk := <-chunks:
				if chunk.err != nil {
					if !errors.Is(chunk.err, io.EOF) {
						ev.Err = chunk.err
					}
					break loop
				}
				ev.ResponseBytes += int64(len(chunk.data))
				tail.observe(chunk.data)
				if _, werr := w.Write(chunk.data); werr != nil {
					ev.Err = werr
					break loop
				}
				if werr := w.Flush(); werr != nil {
					ev.Err = werr
					break loop
				}
				if maxSize > 0 && ev.ResponseBytes >= maxSize {
					ev.Err = errResponseTooLarge
					break loop
				}
			case <-idleTimer.C:
				ev.Err = errStreamIdleTimeout
				break loop
			}
		}

		ev.Usage = tail.usage(decoder)
		ev.Latency = time.Since(start)
		if onUsage != nil {
			onUsage(ev)
		}
	})
}

// readStream is the sole owner of the streamed upstream response. It pumps
// chunks to the writer until the stream ends or done is closed, then closes
// the body stream (with an error when the body was not fully consumed, so
// the connection is dropped instead of being reused mid-body) and releases
// the pooled response.
func readStream(stream io.Reader, resp *client.Response, chunks chan<- streamChunk, done <-chan struct{}) {
	// Ping-pong buffers: while the writer processes one, the next Read
	// fills the other.
	var bufs [2][]byte
	bufs[0] = make([]byte, 4096)
	bufs[1] = make([]byte, 4096)

	abandoned := false
	var streamErr error
	for i := 0; ; i ^= 1 {
		n, rerr := stream.Read(bufs[i])
		if n > 0 {
			select {
			case chunks <- streamChunk{data: bufs[i][:n]}:
			case <-done:
				abandoned = true
			}
		}
		if rerr != nil {
			streamErr = rerr
			if !abandoned {
				select {
				case chunks <- streamChunk{err: rerr}:
				case <-done:
					abandoned = true
				}
			}
			break
		}
		if abandoned {
			break
		}
	}

	closeErr := streamErr
	if abandoned && closeErr == nil {
		closeErr = errStreamAbandoned
	}
	if errors.Is(closeErr, io.EOF) {
		// Fully consumed: close cleanly so the connection can be reused.
		closeErr = nil
	}
	if cr, ok := stream.(fasthttp.ReadCloserWithError); ok {
		cr.CloseWithError(closeErr) //nolint:errcheck // teardown is best-effort
	}
	resp.Close()
}

func fireUsage(cfg *Config, ev *UsageEvent, start time.Time) {
	ev.Latency = time.Since(start)
	if cfg.OnUsage != nil {
		cfg.OnUsage(ev)
	}
}
