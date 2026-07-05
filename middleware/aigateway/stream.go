package aigateway

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/client"
	"github.com/gofiber/fiber/v3/log"
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

// abortUpstreamResponse releases a response whose body was not fully read,
// dropping the connection instead of pooling one with unread body bytes.
func abortUpstreamResponse(resp *client.Response) {
	resp.CloseWithError(errStreamAbandoned)
}

// relayBuffered reads the full upstream response and sends it to the client.
// The usage hook fires synchronously. The read runs to the upstream's EOF;
// fasthttp's streamed body cannot be interrupted from another goroutine
// without racing the read, so a mid-body stall is bounded by the upstream and
// OS TCP timeouts rather than a gateway timer (as with middleware/proxy).
func relayBuffered(c fiber.Ctx, cfg *Config, resp *client.Response, ev *UsageEvent, start time.Time) error {
	// The upstream already produced a response (its headers arrived), so record
	// its status now: even the read-error and too-large paths below must report
	// the real upstream status rather than leaving StatusCode at 0, which the
	// UsageEvent contract reserves for "no upstream response at all" and which
	// the streaming path already sets before relaying.
	ev.StatusCode = resp.StatusCode()

	reader := resp.BodyStream()
	if cfg.MaxResponseSize > 0 {
		reader = io.LimitReader(reader, cfg.MaxResponseSize+1)
	}

	body, err := io.ReadAll(reader)
	// io.ReadAll returns the bytes read so far alongside an error, so this counts
	// the partial body on the error path too.
	ev.ResponseBytes = int64(len(body))
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

	// The token usage feeds both the OnUsage hook and the quota commit, so
	// parse it when either consumer is active. Parsed before OnResponse can
	// mutate the body, so usage always reflects what the upstream reported.
	if cfg.OnUsage != nil || (cfg.QuotaStore != nil && ev.quotaID != "") {
		limit := cfg.MaxResponseSize
		if limit <= 0 {
			limit = usageDecodeLimit
		}
		ev.Usage = parseUsage(decodeForUsage(resp, body, limit), c.App().Config().JSONDecoder)
	}

	status := resp.StatusCode()
	if cfg.OnResponse != nil {
		r := &RelayResponse{Body: body, Status: status}
		if herr := cfg.OnResponse(c, r); herr != nil {
			// The body was fully read, so the connection is clean to reuse.
			resp.Close()
			ev.Err = herr
			fireUsage(cfg, ev, start)
			return fiber.ErrBadGateway
		}
		body = r.Body
		status = r.Status
		ev.ResponseBytes = int64(len(body))
	}

	copyResponseHeaders(c, resp)
	c.Status(status)
	// The body was consumed to EOF, so closing releases the connection for
	// reuse. Headers were copied off the pooled response above.
	resp.Close()

	fireUsage(cfg, ev, start)

	return c.Send(body)
}

// boundedDecompress returns the decoded form of body for the given
// Content-Encoding, reading at most limit bytes so a compression bomb — a tiny
// encoded body that expands to gigabytes — cannot exhaust memory. It reports
// ok=false on an unknown/unsupported encoding, a decode error, or an overflow
// past limit. An empty or identity encoding returns body unchanged with ok.
//
// Only gzip and deflate are handled; other encodings (br, zstd) report
// ok=false rather than pulling in extra decompressors just to peek at a field.
func boundedDecompress(enc string, body []byte, limit int64) ([]byte, bool) {
	enc = strings.ToLower(strings.TrimSpace(enc))
	if enc == "" || enc == "identity" {
		return body, true
	}

	var r io.Reader
	switch {
	case strings.Contains(enc, "gzip"):
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		defer gz.Close() //nolint:errcheck // decode-only reader
		r = gz
	case strings.Contains(enc, "deflate"):
		// "deflate" is conventionally zlib-wrapped (RFC 1950); fall back to
		// raw DEFLATE (RFC 1951) for the servers that send it bare.
		if zr, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			defer zr.Close() //nolint:errcheck // decode-only reader
			r = zr
		} else {
			fr := flate.NewReader(bytes.NewReader(body))
			defer fr.Close() //nolint:errcheck // decode-only reader
			r = fr
		}
	default:
		return nil, false
	}

	out, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(out)) > limit {
		return nil, false
	}
	return out, true
}

// decodeForUsage returns a decompressed copy of body when the response is
// content-encoded, so token usage can be parsed. The client still receives the
// original (encoded) bytes. Unknown/failed/overflowing encodings return body
// unchanged (usage then parses to nil, best-effort).
func decodeForUsage(resp *client.Response, body []byte, limit int64) []byte {
	enc := string(resp.RawResponse.Header.Peek(fiber.HeaderContentEncoding))
	if out, ok := boundedDecompress(enc, body, limit); ok {
		return out
	}
	return body
}

// usageDecodeLimit bounds decompression for usage parsing when no
// MaxResponseSize is configured.
const usageDecodeLimit = 8 << 20 // 8 MiB

// sniffDecodeMax is the ceiling on decompressing a content-encoded request body
// while sniffing the model. The effective bound is min(BodyLimit, sniffDecodeMax):
// it never exceeds the body size the server already accepts uncompressed, and
// this fixed ceiling caps bomb amplification even when BodyLimit is large. It is
// generous enough to inspect a max-context gzipped request while bounding the
// per-request decompression a bomb can force.
const sniffDecodeMax = 4 << 20 // 4 MiB

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

	// ev's ctx-derived strings were already copied into owned memory in New(),
	// so the goroutines below may read them after the handler returns. cfg
	// points at the per-mount config captured by the handler closure, which
	// outlives every request; only c (the pooled ctx) is off-limits, so the
	// decoder is copied out here.
	stream := resp.BodyStream()
	idle := cfg.StreamIdleTimeout
	maxSize := cfg.MaxResponseSize
	decoder := c.App().Config().JSONDecoder

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
				// Enforce the cap before writing so the client never receives
				// bytes past MaxResponseSize: the crossing chunk is dropped whole
				// (a partial write would only split an SSE event mid-line). This
				// keeps the streamed cap as strict as the buffered path's, which
				// rejects anything over the limit.
				if maxSize > 0 && ev.ResponseBytes > maxSize {
					ev.Err = errResponseTooLarge
					break loop
				}
				tail.observe(chunk.data)
				if _, werr := w.Write(chunk.data); werr != nil {
					ev.Err = werr
					break loop
				}
				if werr := w.Flush(); werr != nil {
					ev.Err = werr
					break loop
				}
			case <-idleC:
				ev.Err = errStreamIdleTimeout
				break loop
			}
		}

		ev.Usage = tail.usage(decoder)
		fireUsage(cfg, ev, start)
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
	applyCost(cfg, ev)
	// Post-paid quota commit: record what this request actually consumed.
	// Runs on the stream writer goroutine for streamed responses, so it may
	// only touch cfg (per-mount, immortal) and owned ev fields. A failed
	// commit is logged, not surfaced: the response is already under way.
	if cfg.QuotaStore != nil && ev.quotaID != "" {
		var tokens int64
		if ev.Usage != nil {
			tokens = int64(ev.Usage.TotalTokens)
		}
		if tokens > 0 || ev.Cost > 0 {
			if _, _, err := cfg.QuotaStore.Add(ev.quotaID, cfg.QuotaWindow, tokens, ev.Cost); err != nil {
				log.Warnf("aigateway: quota commit failed: %v", err)
			}
		}
	}
	if cfg.OnUsage != nil {
		cfg.OnUsage(ev)
	}
}
