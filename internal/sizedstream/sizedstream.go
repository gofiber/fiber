// Package sizedstream buffers a response body stream that declares its length.
package sizedstream

import (
	"bytes"
	"errors"
	"sync"

	"github.com/valyala/fasthttp"
)

// errTooLong stops a stream that delivers more than it declares.
var errTooLong = errors.New("sizedstream: body stream runs past its declared length")

// maxPooled keeps one large body from pinning its buffer in the pool.
const maxPooled = 1 << 20

// buffer keeps the first limit bytes written to it and refuses the rest.
type buffer struct {
	b     []byte
	limit int
}

func (w *buffer) Write(p []byte) (int, error) { return keep(w, p) }

// WriteString spares a strings.Reader stream the copy io.WriteString makes for a plain Writer.
func (w *buffer) WriteString(s string) (int, error) { return keep(w, s) }

func keep[T []byte | string](w *buffer, p T) (int, error) {
	if room := w.limit - len(w.b); len(p) > room {
		w.b = append(w.b, p[:room]...)
		return room, errTooLong
	}
	w.b = append(w.b, p...)
	return len(p), nil
}

// Pooled because a buffer handed to the stream as an io.Writer escapes.
var buffers = sync.Pool{New: func() any { return new(buffer) }}

// Read reads the body stream of resp, which declares n bytes, into its body and reports whether
// exactly n arrived without error. Response.Body reads to the end and keeps a read error as the body.
func Read(resp *fasthttp.Response, n int) bool {
	w, ok := buffers.Get().(*buffer)
	if !ok {
		w = new(buffer)
	}
	defer func() {
		if cap(w.b) <= maxPooled {
			buffers.Put(w)
		}
	}()

	w.b, w.limit = w.b[:0], n
	// BodyWriteTo keeps the stream's own WriteTo and closes it with the error that stopped it.
	if err := resp.BodyWriteTo(w); err == nil && len(w.b) == n {
		resp.SetBody(w.b)
		return true
	}
	// Sent on as far as it arrived, so a client still sees a stream that fell short cut off.
	resp.SetBodyStream(bytes.NewReader(bytes.Clone(w.b)), n)
	return false
}
