package aigateway

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"sync"
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

// headerXAccelBuffering disables response buffering in reverse proxies (nginx)
// so streamed chunks reach the client immediately. Fiber has no constant for it.
const headerXAccelBuffering = "X-Accel-Buffering"

// streamingContentTypes are response media types relayed incrementally rather
// than buffered: Server-Sent Events and newline-delimited JSON.
var streamingContentTypes = []string{
	fiber.MIMETextEventStream,
	"application/x-ndjson",
}

// streamBufPool recycles the ping-pong read buffers used per streamed request.
var streamBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 4096)
		return &b
	},
}

func getStreamBuf() *[]byte {
	if b, ok := streamBufPool.Get().(*[]byte); ok {
		return b
	}
	b := make([]byte, 4096)
	return &b
}

// isStreamingResponse reports whether the upstream response should be relayed
// incrementally based on its Content-Type.
func isStreamingResponse(resp *client.Response) bool {
	ct := resp.RawResponse.Header.ContentType()
	for _, prefix := range streamingContentTypes {
		if len(ct) >= len(prefix) && utils.EqualFold(utils.UnsafeString(ct[:len(prefix)]), prefix) {
			return true
		}
	}
	return false
}

// upstreamBodyCloser returns the response body stream as a closer that, when
// closed with a non-nil error, drops the upstream connection instead of
// returning it to the pool with unread bytes.
func upstreamBodyCloser(resp *client.Response) (fasthttp.ReadCloserWithError, bool) {
	cr, ok := resp.RawResponse.BodyStream().(fasthttp.ReadCloserWithError)
	return cr, ok
}

// abortUpstreamResponse releases a response whose body was not fully read.
// The body stream must be closed with a non-nil error first: a plain
// resp.Close() closes it with nil, which would hand a connection with
// unread body bytes back to the pool for reuse.
func abortUpstreamResponse(resp *client.Response) {
	if cr, ok := upstreamBodyCloser(resp); ok {
		_ = cr.CloseWithError(errStreamAbandoned) //nolint:errcheck // teardown is best-effort
	}
	resp.Close()
}

// relayBuffered reads the full upstream response and sends it to the client.
// The usage hook fires synchronously. The read runs to the upstream's EOF;
// fasthttp's streamed body cannot be interrupted from another goroutine
// without racing the read, so a mid-body stall is bounded by the upstream and
// OS TCP timeouts rather than a gateway timer (as with middleware/proxy).
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
	ev.ResponseBytes = int64(len(body))
	if cfg.OnUsage != nil {
		ev.Usage = parseUsage(decodeForUsage(resp, body), c.App().Config().JSONDecoder)
	}
	// The body was consumed to EOF, so closing releases the connection for
	// reuse. Headers were copied off the pooled response above.
	resp.Close()

	fireUsage(cfg, ev, start)

	return c.Send(body)
}

// decodeForUsage returns a decompressed copy of body when the response is
// content-encoded, so token usage can be parsed. The client still receives the
// original (encoded) bytes. Unknown/failed encodings return body unchanged.
func decodeForUsage(resp *client.Response, body []byte) []byte {
	enc := strings.ToLower(string(resp.RawResponse.Header.Peek(fiber.HeaderContentEncoding)))
	if enc == "" || enc == "identity" {
		return body
	}
	var (
		out []byte
		err error
	)
	switch {
	case strings.Contains(enc, "gzip"):
		out, err = fasthttp.AppendGunzipBytes(nil, body)
	case strings.Contains(enc, "deflate"):
		out, err = fasthttp.AppendInflateBytes(nil, body)
	case strings.Contains(enc, "br"):
		out, err = fasthttp.AppendUnbrotliBytes(nil, body)
	default:
		return body
	}
	if err != nil {
		return body
	}
	return out
}

// streamChunk carries one upstream read result from the reader goroutine to
// the response writer.
type streamChunk struct {
	err  error
	data []byte
}

// relayStream pipes the upstream body to the client chunk by chunk, flushing
// after every read so tokens arrive as they are generated. The usage hook
// fires on the stream writer goroutine after the stream ends.
//
// Concurrency: fasthttp's streamed response body is not safe to read and close
// from different goroutines — closing runs teardown on the same pooled struct
// the reader is reading. A dedicated reader goroutine is therefore the SOLE
// owner of the resp object: it reads, hands chunks over a channel, and is the
// only goroutine that ever closes the response. The writer only signals
// abandonment by closing done; the reader then closes the upstream connection
// with an error (dropping it instead of returning a half-read connection to
// the pool). A reader blocked in Read on a fully stalled upstream lingers
// until the upstream sends a byte or closes — that is the price of never
// racing the read.
func relayStream(c fiber.Ctx, cfg *Config, resp *client.Response, ev *UsageEvent, start time.Time) error {
	copyResponseHeaders(c, resp)
	c.Status(resp.StatusCode())
	// Ask reverse proxies (nginx et al.) not to buffer this response.
	c.Set(headerXAccelBuffering, "no")
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
		// Signaling the reader to tear down is all the writer may do to the
		// upstream response; it never touches the stream itself.
		defer close(done)

		var idleTimer *time.Timer
		var idleC <-chan time.Time
		if idle > 0 {
			idleTimer = time.NewTimer(idle)
			idleC = idleTimer.C
			defer idleTimer.Stop()
		}

		tail := &usageTail{}
	loop:
		for {
			if idleTimer != nil {
				idleTimer.Reset(idle)
			}
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
				if maxSize > 0 && ev.ResponseBytes > maxSize {
					ev.Err = errResponseTooLarge
					break loop
				}
			case <-idleC:
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
// chunks to the writer until the stream ends or the writer closes done, then
// closes the response itself: cleanly on a full read (connection reusable),
// or with an error on abandonment or a mid-body failure (connection dropped,
// so a half-read connection is never returned to the pool).
func readStream(stream io.Reader, resp *client.Response, chunks chan<- streamChunk, done <-chan struct{}) {
	// Ping-pong buffers from the pool: while the writer processes one, the
	// next Read fills the other.
	b0 := getStreamBuf()
	b1 := getStreamBuf()
	defer streamBufPool.Put(b0)
	defer streamBufPool.Put(b1)
	bufs := [2][]byte{*b0, *b1}

	abandoned := false
	var readErr error
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
			readErr = rerr
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

	if abandoned || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		abortUpstreamResponse(resp)
		return
	}
	resp.Close()
}

func fireUsage(cfg *Config, ev *UsageEvent, start time.Time) {
	ev.Latency = time.Since(start)
	if cfg.OnUsage != nil {
		cfg.OnUsage(ev)
	}
}
