package sizedstream

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Read(t *testing.T) {
	t.Parallel()

	reset := errors.New("connection reset")
	for name, tc := range map[string]struct {
		stream   func() io.Reader
		body     string
		declared int
		complete bool
		cut      bool
	}{
		"exact":  {stream: func() io.Reader { return strings.NewReader("hello") }, declared: 5, complete: true, body: "hello"},
		"empty":  {stream: func() io.Reader { return strings.NewReader("") }, complete: true},
		"short":  {stream: func() io.Reader { return strings.NewReader("hel") }, declared: 5, cut: true},
		"longer": {stream: func() io.Reader { return strings.NewReader("hello, world") }, declared: 5},
		"failed": {
			stream:   func() io.Reader { return io.MultiReader(strings.NewReader("hel"), iotest.ErrReader(reset)) },
			declared: 5, cut: true,
		},
		"failed at the end": {
			stream:   func() io.Reader { return io.MultiReader(strings.NewReader("hello"), iotest.ErrReader(reset)) },
			declared: 5,
		},
	} {
		for kind, stream := range map[string]func() io.Reader{
			"written":  tc.stream,
			"read":     func() io.Reader { return struct{ io.Reader }{tc.stream()} },
			"one byte": func() io.Reader { return iotest.OneByteReader(tc.stream()) },
		} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				t.Parallel()

				var resp fasthttp.Response
				resp.SetBodyStream(stream(), tc.declared)
				require.Equal(t, tc.complete, Read(&resp, tc.declared))
				if tc.complete {
					require.False(t, resp.IsBodyStream())
					require.Equal(t, tc.body, string(resp.Body()))
					return
				}

				// Still a stream of the declared length, never the read error as a whole body.
				require.True(t, resp.IsBodyStream())
				require.Equal(t, tc.declared, resp.Header.ContentLength())
				var wire strings.Builder
				err := resp.Write(bufio.NewWriter(&wire))
				require.Equal(t, tc.cut, err != nil)
				require.NotContains(t, wire.String(), reset.Error())
			})
		}
	}
}

func Test_Read_DoesNotDrainALongerStream(t *testing.T) {
	t.Parallel()

	// No WriteTo, so the stream is pulled through a copy buffer rather than written whole.
	stream := strings.NewReader(strings.Repeat("x", 1<<20))
	var resp fasthttp.Response
	resp.SetBodyStream(struct{ io.Reader }{stream}, 10)
	require.False(t, Read(&resp, 10))
	require.Greater(t, stream.Len(), 1<<20-64<<10)
}

// Sequential: testing.AllocsPerRun panics during a parallel test. The fewest of many single runs,
// since under -race sync.Pool drops a quarter of what it is handed.
func Test_Read_DoesNotAllocate(t *testing.T) {
	stream := strings.NewReader("")
	var resp fasthttp.Response
	fewest := math.MaxFloat64
	for range 200 {
		fewest = min(fewest, testing.AllocsPerRun(1, func() {
			stream.Reset("hello")
			resp.SetBodyStream(stream, 5)
			Read(&resp, 5)
		}))
	}
	require.Zero(t, fewest)
	require.Equal(t, "hello", string(resp.Body()))
}

// Sequential, as above. Growing by append left about 6x the body as garbage on every read of a
// chunked stream, and a buffer past 1 MiB was never pooled.
func Test_Read_ChunkedStreamDoesNotAllocate(t *testing.T) {
	for _, size := range []int{900 << 10, 4 << 20} {
		body := bytes.Repeat([]byte("x"), size)
		src := bytes.NewReader(nil)
		var stream io.Reader = struct{ io.Reader }{src} // no WriteTo, so it arrives in chunks
		var resp fasthttp.Response
		fewest := math.MaxFloat64
		for range 50 {
			fewest = min(fewest, testing.AllocsPerRun(1, func() {
				src.Reset(body)
				resp.SetBodyStream(stream, size)
				Read(&resp, size)
			}))
		}
		require.Zero(t, fewest, size)
		require.Equal(t, body, resp.Body())
	}
}
