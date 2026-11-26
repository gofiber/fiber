// Package bodystream buffers a streamed request body within a size limit.
package bodystream

import (
	"errors"
	"math"
	"mime/multipart"

	"github.com/gofiber/fiber/v3/internal/sizedstream"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type failedReader struct {
	err error
}

func (f failedReader) Read([]byte) (int, error) {
	return 0, f.err
}

// Buffer reads a streamed body into req, since fasthttp's Body and PostArgs drain one whatever its size and keep a
// read error as the body. Past limit (<= 0 is none) it fails with fasthttp.ErrBodyTooLarge; a failure is final.
func Buffer(req *fasthttp.Request, limit int) error {
	stream := req.BodyStream()
	if failed, ok := stream.(failedReader); ok {
		return failed.err
	}
	if stream == nil {
		return nil
	}
	if limit <= 0 {
		limit = math.MaxInt
	}

	// A declared length is read here too: the connection can fail before the stream ends there, and over limit
	// the handler may have drained the stream already.
	if err := sizedstream.ReadRequest(req, limit); err != nil {
		// Part of the body is gone, so a later read must not see the rest as the body.
		req.SetBodyStream(failedReader{err: err}, req.Header.ContentLength())
		return err
	}
	return nil
}

// MultipartForm is req.MultipartFormWithLimit, except that a failed parse of a streamed body is final: fasthttp leaves
// the stream where the parse stopped, and the next read would take the rest of the body for a new form or the body.
func MultipartForm(req *fasthttp.Request, limit int) (*multipart.Form, error) {
	form, err := req.MultipartFormWithLimit(limit)
	if err == nil || !req.IsBodyStream() || !readsMultipartStream(req, err) {
		return form, err
	}
	if failed, ok := req.BodyStream().(failedReader); ok {
		return nil, failed.err
	}
	req.SetBodyStream(failedReader{err: err}, req.Header.ContentLength())
	return nil, err
}

// readsMultipartStream reports whether the parse that failed with err read from the stream. fasthttp refuses a body
// that is no multipart form, or has a Content-Encoding other than gzip, before reading any of it.
func readsMultipartStream(req *fasthttp.Request, err error) bool {
	if errors.Is(err, fasthttp.ErrNoMultipartForm) {
		return false
	}
	encoding := req.Header.Peek(fasthttp.HeaderContentEncoding)
	return len(encoding) == 0 || utils.EqualFold(utils.UnsafeString(encoding), "gzip")
}
