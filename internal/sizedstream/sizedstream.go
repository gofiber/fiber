// Package sizedstream buffers a body stream within a length: the one a response declares, or a request's limit.
package sizedstream

import (
	"bytes"
	"errors"
	"io"
	"math"
	"sync"

	"github.com/valyala/fasthttp"
)

// errTooLong stops a stream that delivers more than it declares.
var errTooLong = errors.New("sizedstream: body stream runs past its declared length")

// maxPresized bounds the declared length a buffer is sized for before anything arrives.
const maxPresized = 1 << 20

// buffer keeps the first limit bytes written to it, and in tail the byte that shows a stream runs longer.
type buffer struct {
	b     []byte
	limit int
	size  int // the declared length, or -1
	past  int // bytes held in tail, 0 or 1
	tail  [1]byte
}

func (w *buffer) Write(p []byte) (int, error) { return keep(w, p) }

// WriteString spares a strings.Reader stream the copy io.WriteString makes for a plain Writer.
func (w *buffer) WriteString(s string) (int, error) { return keep(w, s) }

func keep[T []byte | string](w *buffer, p T) (int, error) {
	if room := w.limit - len(w.b); len(p) > room {
		w.grow(room)
		w.b = append(w.b, p[:room]...)
		w.tail[0], w.past = p[room], 1
		return room + 1, errTooLong
	}
	w.grow(len(p))
	w.b = append(w.b, p...)
	return len(p), nil
}

// grow makes room for need more bytes, doubling the capacity but never past the declared length. Doubling keeps a
// pool's power-of-two buffer at one, which fasthttp's body pool, calibrated by size class, then keeps.
func (w *buffer) grow(need int) {
	if cap(w.b)-len(w.b) < need {
		w.b = append(make([]byte, 0, max(len(w.b)+need, min(max(2*cap(w.b), 4096), w.limit))), w.b...)
	}
}

// ReadFrom reads straight into the buffer, where a copy loop would hand it the stream 4 KiB at a time.
func (w *buffer) ReadFrom(r io.Reader) (int64, error) {
	start := len(w.b)
	for {
		// Past limit, or with a full buffer at its declared length or with none declared, the tail only shows whether
		// the stream runs longer: growing to see the end would leave a buffer the calibrated body pool drops.
		probe := len(w.b) == w.limit || (len(w.b) == cap(w.b) && (w.size < 0 || len(w.b) == w.size))
		p := w.tail[:]
		if !probe {
			w.grow(1)
			p = w.b[len(w.b):min(cap(w.b), w.limit)]
		}
		m, err := r.Read(p)
		if probe && m > 0 {
			if len(w.b) == w.limit {
				w.past = 1
				return int64(len(w.b) - start + 1), errTooLong
			}
			w.grow(1)
			w.b, m = append(w.b, w.tail[0]), 0
		}
		w.b = w.b[:len(w.b)+m]
		if errors.Is(err, io.EOF) {
			return int64(len(w.b) - start), nil
		}
		if err != nil {
			return int64(len(w.b) - start), err
		}
	}
}

// Pooled because a buffer handed to the stream as an io.Writer escapes. A read trades its bytes for
// the response's own, so fasthttp's calibrated body pool decides which sizes stay around.
var buffers = sync.Pool{New: func() any { return new(buffer) }}

// acquire returns a pooled buffer that keeps at most limit bytes.
func acquire(limit int) *buffer {
	w, ok := buffers.Get().(*buffer)
	if !ok {
		w = new(buffer)
	}
	w.b, w.limit, w.size, w.past = w.b[:0], limit, -1, 0
	return w
}

// Read reads the body stream of resp, which declares n bytes, into its body and reports whether
// exactly n arrived without error. Response.Body reads to the end and keeps a read error as the body.
func Read(resp *fasthttp.Response, n int) bool {
	w := acquire(n)
	defer buffers.Put(w)
	w.size = n

	// Sized up front, since growing to it leaves about the body again as garbage. A larger
	// declared length is not trusted with an allocation.
	if n <= maxPresized && cap(w.b) < n {
		w.b = make([]byte, 0, n)
	}
	// BodyWriteTo keeps the stream's own WriteTo and closes it with the error that stopped it.
	if err := resp.BodyWriteTo(w); err == nil && w.past == 0 && len(w.b) == n {
		w.b = resp.SwapBody(w.b)
		return true
	}
	// Sent on as it arrived, so a stream that fell short or ran longer still does for the client and an outer reader.
	resp.SetBodyStream(bytes.NewReader(append(w.b, w.tail[:w.past]...)), n)
	w.b = nil
	return false
}

// ReadRequest reads the body stream of req into its body, failing with fasthttp.ErrBodyTooLarge past limit or with the
// stream's own error, which Request.Body would keep as the body. A client's declared length is not presized for.
func ReadRequest(req *fasthttp.Request, limit int) error {
	w := acquire(limit)
	defer buffers.Put(w)
	w.size = req.Header.ContentLength()
	var err error
	if w.size < 0 && cap(w.b) == 0 {
		// No length to grow toward and no buffer in hand: io.ReadAll's exactly sized copy costs about twice the
		// body, where doubling from nothing costs up to four times.
		w.b, err = io.ReadAll(io.LimitReader(req.BodyStream(), int64(min(limit, math.MaxInt-1))+1))
		if err == nil && len(w.b) > limit {
			err = errTooLong
		}
	} else {
		_, err = w.ReadFrom(req.BodyStream())
	}
	if err != nil {
		w.b = nil // it can hold limit bytes; as in Read, fasthttp's calibrated pool decides which sizes stay
		if errors.Is(err, errTooLong) {
			return fasthttp.ErrBodyTooLarge
		}
		return err
	}
	// SwapBody keeps a multipart form already parsed off this stream; SetBodyRaw drops it.
	req.CloseBodyStream() //nolint:errcheck // already read to its end
	w.b = req.SwapBody(w.b)
	return nil
}
