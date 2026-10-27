package bodystream

import (
	"bytes"
	"io"
	"math"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

const limit = 1024

func Test_Buffer_NoStream(t *testing.T) {
	t.Parallel()

	var req fasthttp.Request
	req.SetBodyString(strings.Repeat("a", 2*limit))

	require.NoError(t, Buffer(&req, limit), "the server applied the limit to a body it buffered")
	require.Len(t, req.Body(), 2*limit)
}

func Test_Buffer_WithinLimit(t *testing.T) {
	t.Parallel()

	for _, size := range []int{0, 1, limit - 1, limit} {
		for _, declared := range []int{-1, size} {
			var req fasthttp.Request
			req.SetBodyStream(strings.NewReader(strings.Repeat("a", size)), declared)

			require.NoError(t, Buffer(&req, limit))
			require.Equal(t, declared >= 0, req.IsBodyStream(), "a declared length is left to fasthttp")
			require.Equal(t, strings.Repeat("a", size), string(req.Body()))
			require.Equal(t, declared, req.Header.ContentLength())
		}
	}
}

func Test_Buffer_OverLimit(t *testing.T) {
	t.Parallel()

	for _, size := range []int{limit + 1, limit + 2, 100 * limit} {
		for _, declared := range []int{-1, size} {
			var req fasthttp.Request
			stream := strings.NewReader(strings.Repeat("a", size))
			req.SetBodyStream(stream, declared)

			require.ErrorIs(t, Buffer(&req, limit), fasthttp.ErrBodyTooLarge)
			require.Equal(t, size-limit-1, stream.Len(), "the rest of the stream is not read")
			require.Equal(t, declared, req.Header.ContentLength())

			require.ErrorIs(t, Buffer(&req, limit), fasthttp.ErrBodyTooLarge, "what follows is not taken for the body")
			_, err := io.ReadAll(req.BodyStream())
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
			require.Equal(t, size-limit-1, stream.Len())
		}
	}
}

func Test_Buffer_DeclaredOverLimitPartlyRead(t *testing.T) {
	t.Parallel()

	for _, left := range []int{0, limit} {
		var req fasthttp.Request
		stream := strings.NewReader(strings.Repeat("a", 4*limit))
		req.SetBodyStream(stream, 4*limit)
		_, err := io.CopyN(io.Discard, req.BodyStream(), int64(4*limit-left))
		require.NoError(t, err)

		require.NoError(t, Buffer(&req, limit), "what is left of the stream is within the limit")
		require.False(t, req.IsBodyStream())
		require.Len(t, req.Body(), left)
	}
}

func Test_Buffer_NoLimit(t *testing.T) {
	t.Parallel()

	var req fasthttp.Request
	req.SetBodyStream(strings.NewReader("body"), -1)

	require.NoError(t, Buffer(&req, 0))
	require.True(t, req.IsBodyStream())
}

func Test_Buffer_ReadError(t *testing.T) {
	t.Parallel()

	var req fasthttp.Request
	req.SetBodyStream(io.MultiReader(strings.NewReader("partial"), failedReader{err: io.ErrUnexpectedEOF}), -1)

	require.ErrorIs(t, Buffer(&req, limit), io.ErrUnexpectedEOF)
	require.ErrorIs(t, Buffer(&req, limit), io.ErrUnexpectedEOF)
	require.Equal(t, -1, req.Header.ContentLength())
}

func Test_Buffer_KeepsParsedMultipartForm(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	require.NoError(t, w.WriteField("name", "john"))
	require.NoError(t, w.Close())

	var req fasthttp.Request
	req.Header.SetContentType(w.FormDataContentType())
	req.SetBodyStream(&body, -1)

	form, err := req.MultipartFormWithLimit(limit)
	require.NoError(t, err)
	require.True(t, req.IsBodyStream())

	require.NoError(t, Buffer(&req, limit))

	again, err := req.MultipartFormWithLimit(limit)
	require.NoError(t, err)
	require.Same(t, form, again)
}

func Test_Buffer_MaxIntLimit(t *testing.T) {
	t.Parallel()

	var req fasthttp.Request
	req.SetBodyStream(strings.NewReader("hello"), -1)

	require.NoError(t, Buffer(&req, math.MaxInt))
	require.False(t, req.IsBodyStream())
	require.Equal(t, "hello", string(req.Body()))
}
