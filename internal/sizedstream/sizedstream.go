// Package sizedstream buffers a response body stream that declares its length.
package sizedstream

import (
	"bytes"
	"errors"
	"io"
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

// grow makes room for need more bytes, doubling like append but never past the declared length.
func (w *buffer) grow(need int) {
	if cap(w.b)-len(w.b) < need {
		w.b = append(make([]byte, 0, len(w.b)+max(need, min(max(cap(w.b), 4096), w.limit-len(w.b)))), w.b...)
	}
}

// ReadFrom reads straight into the buffer, where a copy loop would hand it the stream 4 KiB at a time.
func (w *buffer) ReadFrom(r io.Reader) (int64, error) {
	start := len(w.b)
	for {
		p := w.tail[:] // once the buffer is full, only to see whether the stream runs longer
		if room := w.limit - len(w.b); room > 0 {
			w.grow(1)
			p = w.b[len(w.b):min(cap(w.b), w.limit)]
		}
		m, err := r.Read(p)
		if m > 0 && len(w.b) == w.limit {
			w.past = 1
			return int64(len(w.b) - start + 1), errTooLong
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

// Read reads the body stream of resp, which declares n bytes, into its body and reports whether
// exactly n arrived without error. Response.Body reads to the end and keeps a read error as the body.
func Read(resp *fasthttp.Response, n int) bool {
	w, ok := buffers.Get().(*buffer)
	if !ok {
		w = new(buffer)
	}
	defer buffers.Put(w)
	w.b, w.limit, w.past = w.b[:0], n, 0

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
