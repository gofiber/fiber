// Package sizedstream buffers a response body stream that declares its length.
package sizedstream

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"sync"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// errTooLong stops a stream that delivers more than it declares.
var errTooLong = errors.New("sizedstream: body stream runs past its declared length")

// maxPresized bounds the declared length a buffer is sized for before anything arrives.
const maxPresized = 1 << 20

// buffer keeps the first limit bytes written to it and refuses the rest.
type buffer struct {
	bb    *bytebufferpool.ByteBuffer
	limit int
}

func (w *buffer) Write(p []byte) (int, error) { return keep(w, p) }

// WriteString spares a strings.Reader stream the copy io.WriteString makes for a plain Writer.
func (w *buffer) WriteString(s string) (int, error) { return keep(w, s) }

func keep[T []byte | string](w *buffer, p T) (int, error) {
	if room := w.limit - len(w.bb.B); len(p) > room {
		w.bb.B = append(w.bb.B, p[:room]...)
		return room, errTooLong
	}
	w.bb.B = append(w.bb.B, p...)
	return len(p), nil
}

// ReadFrom reads straight into the buffer, where a copy loop would hand it the stream 4 KiB at a time.
func (w *buffer) ReadFrom(r io.Reader) (int64, error) {
	start := len(w.bb.B)
	for {
		b := w.bb.B
		if len(b) == cap(b) {
			b = slices.Grow(b, 1)
		}
		p := b[len(b):cap(b)]
		if room := w.limit - len(b); len(p) > room {
			p = p[:room+1] // the byte past the limit tells a stream that runs longer from one that ends there
		}
		m, err := r.Read(p)
		w.bb.B = b[:len(b)+m]
		if len(w.bb.B) > w.limit {
			w.bb.B = w.bb.B[:w.limit]
			return int64(w.limit - start), errTooLong
		}
		if errors.Is(err, io.EOF) {
			return int64(len(w.bb.B) - start), nil
		}
		if err != nil {
			return int64(len(w.bb.B) - start), err
		}
	}
}

// Pooled because a buffer handed to the stream as an io.Writer escapes.
var buffers = sync.Pool{New: func() any { return new(buffer) }}

// Calibrated like fasthttp's own response bodies, so a large one stays pooled only while it is common.
var bodies bytebufferpool.Pool

// Read reads the body stream of resp, which declares n bytes, into its body and reports whether
// exactly n arrived without error. Response.Body reads to the end and keeps a read error as the body.
func Read(resp *fasthttp.Response, n int) bool {
	w, ok := buffers.Get().(*buffer)
	if !ok {
		w = new(buffer)
	}
	w.bb, w.limit = bodies.Get(), n
	defer func() {
		bodies.Put(w.bb)
		w.bb = nil
		buffers.Put(w)
	}()

	// Sized up front, with the byte that shows a stream running longer: growing by append leaves
	// about 5x the body as garbage. A larger declared length is not trusted with an allocation.
	if n <= maxPresized && cap(w.bb.B) <= n {
		w.bb.B = make([]byte, 0, n+1)
	}
	// BodyWriteTo keeps the stream's own WriteTo and closes it with the error that stopped it.
	if err := resp.BodyWriteTo(w); err == nil && len(w.bb.B) == n {
		resp.SetBody(w.bb.B)
		return true
	}
	// Sent on as far as it arrived, so a client still sees a stream that fell short cut off.
	resp.SetBodyStream(bytes.NewReader(bytes.Clone(w.bb.B)), n)
	return false
}
