package filestream

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// closeCounter is a stream that counts how often it was closed.
type closeCounter struct {
	io.Reader
	closed int
}

func (c *closeCounter) Close() error {
	c.closed++
	return nil
}

func Test_Marked(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		build  func(ctx *fasthttp.RequestCtx)
		name   string
		marked bool
	}{
		{name: "no stream", build: Mark},
		{name: "body, no stream", build: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetBodyString("body")
			Mark(ctx)
		}},
		{name: "stream not marked", build: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetBodyStream(strings.NewReader("body"), -1)
		}},
		{name: "marked stream", marked: true, build: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetBodyStream(strings.NewReader("file"), -1)
			Mark(ctx)
		}},
		{name: "stream that replaced a marked one", build: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetBodyStream(strings.NewReader("file"), -1)
			Mark(ctx)
			ctx.Response.SetBodyStreamWriter(func(w *bufio.Writer) { _, _ = w.WriteString("body") }) //nolint:errcheck // a test
		}},
		{name: "marked stream replaced by a body", build: func(ctx *fasthttp.RequestCtx) {
			ctx.Response.SetBodyStream(strings.NewReader("file"), -1)
			Mark(ctx)
			ctx.Response.SetBodyString("body")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ctx fasthttp.RequestCtx
			tc.build(&ctx)
			require.Equal(t, tc.marked, Marked(&ctx))
		})
	}
}

// The mark stays in the user values until the request is reset, which closes the io.Closer values: the
// stream, which the response closes, must not be closed a second time there.
func Test_Mark_ResetDoesNotCloseTheStream(t *testing.T) {
	t.Parallel()
	var ctx fasthttp.RequestCtx
	stream := &closeCounter{Reader: strings.NewReader("file")}
	ctx.Response.SetBodyStream(stream, -1)
	Mark(&ctx)

	ctx.ResetUserValues()
	require.Zero(t, stream.closed)
	require.NoError(t, ctx.Response.CloseBodyStream())
	require.Equal(t, 1, stream.closed)
}
