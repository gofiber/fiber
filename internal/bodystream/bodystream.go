// Package bodystream buffers a streamed request body within a size limit.
package bodystream

import (
	"io"
	"math"

	"github.com/valyala/fasthttp"
)

type failedReader struct {
	err error
}

func (f failedReader) Read([]byte) (int, error) {
	return 0, f.err
}

// Buffer reads a streamed body into req, since fasthttp's Body and PostArgs drain one whatever its size.
// Past limit (<= 0 is none) it stops at limit+1 bytes and fails with fasthttp.ErrBodyTooLarge, now and later.
func Buffer(req *fasthttp.Request, limit int) error {
	stream := req.BodyStream()
	if stream == nil || limit <= 0 {
		return nil
	}
	if n := req.Header.ContentLength(); n >= 0 && n <= limit {
		// The server's stream ends at the declared length, so fasthttp may read it.
		return nil
	}

	// A declared length over limit is read too: the handler may have drained the stream already.
	readLimit := int64(limit)
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	body, err := io.ReadAll(io.LimitReader(stream, readLimit))
	if err == nil && len(body) > limit {
		err = fasthttp.ErrBodyTooLarge
	}
	if err != nil {
		// Part of the body is gone, so a later read must not see the rest as the body.
		req.SetBodyStream(failedReader{err: err}, req.Header.ContentLength())
		return err
	}

	// SwapBody keeps a multipart form already parsed off this stream; SetBodyRaw drops it.
	req.CloseBodyStream() //nolint:errcheck // already read to its end
	req.SwapBody(body)
	return nil
}
