package fieldname

import (
	"bufio"
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// readRequestHeader parses raw as a request, which is the only way to build the
// two-line shapes below: they are what a peer or an intermediary puts on the
// wire, not something the setters express directly.
//
//nolint:revive // flag-parameter: canonical is a property of the header store
func readRequestHeader(tb testing.TB, raw string, canonical bool) *fasthttp.RequestHeader {
	tb.Helper()

	h := &fasthttp.RequestHeader{}
	if !canonical {
		h.DisableNormalizing()
	}
	require.NoError(tb, h.Read(bufio.NewReader(bytes.NewBufferString(raw))))
	return h
}

// Test_First_EmptyLineIsNotNil pins the fasthttp behavior First's fast path
// rests on: Peek answers nil for a field that is absent and a non-nil empty
// slice for one that is present and empty. First skips its second walk on the
// nil answer, so were the two to become indistinguishable it would read only
// the first line again — silently, and only for the messages this guards
// against. This test is what makes that loud instead.
func Test_First_EmptyLineIsNotNil(t *testing.T) {
	t.Parallel()

	h := readRequestHeader(t, "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\n\r\n", true)

	require.Nil(t, h.Peek("Absent"), "Peek must answer nil for a field that is not there")

	present := h.Peek("Origin")
	require.Empty(t, present)
	require.NotNil(t, present, "Peek must answer a non-nil empty slice for a field line that is present and empty")
}

// Test_First_EmptyLineDoesNotHideTheValue covers the read First exists for: a
// field name carried twice, the first line empty. Peek reports that first line
// whatever it holds, so the value beside it was invisible to every caller — a
// response's "Cache-Control: private" behind an empty line of the same name
// read as a response that said nothing about caching.
//
// Origin is used here only because it is a name fasthttp keeps no slot for, the
// same as the response fields this now serves; the request-side reads that must
// refuse a repeated field rather than resolve it go through headerlookup.Value.
func Test_First_EmptyLineDoesNotHideTheValue(t *testing.T) {
	t.Parallel()

	const origin = "http://evil.example"

	tests := []struct {
		name      string
		raw       string
		want      string
		canonical bool
	}{
		{
			name:      "empty line ahead of the value",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\nOrigin: " + origin + "\r\n\r\n",
			canonical: true,
			want:      origin,
		},
		{
			name:      "empty line ahead of another spelling",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\norigin: " + origin + "\r\n\r\n",
			canonical: false,
			want:      origin,
		},
		{
			name:      "another spelling alone",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\norigin: " + origin + "\r\n\r\n",
			canonical: false,
			want:      origin,
		},
		{
			name:      "the ordinary single line",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin: " + origin + "\r\n\r\n",
			canonical: true,
			want:      origin,
		},
		{
			// Present and empty is still nothing a caller can act on, so it
			// reads as absent rather than as a value of "".
			name:      "only an empty line",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\nOrigin:\r\n\r\n",
			canonical: true,
			want:      "",
		},
		{
			name:      "absent",
			raw:       "GET / HTTP/1.1\r\nHost: app.example\r\n\r\n",
			canonical: true,
			want:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := readRequestHeader(t, tc.raw, tc.canonical)
			require.Equal(t, tc.want, string(First(h, fasthttp.HeaderOrigin, tc.canonical)))
		})
	}
}

func Test_Del_TerminatesOnSynthesizedContentType(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h := &fasthttp.ResponseHeader{}
		Del(h, fasthttp.HeaderContentType, false)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Del did not return: ResponseHeader.All reports a default Content-Type once the slot is empty")
	}
}

func Test_Del_TerminatesOnNonCanonicalNameInNormalizingStore(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h := &fasthttp.ResponseHeader{}
		h.SetNoDefaultContentType(true)
		h.SetCanonical([]byte("x-session-id"), []byte("v"))
		Del(h, "X-Session-Id", false)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Del did not return: the store normalizes the key it is given, so it never matches the stored spelling")
	}
}

func Test_Del_RemovesEverySpelling(t *testing.T) {
	t.Parallel()

	h := &fasthttp.ResponseHeader{}
	h.SetNoDefaultContentType(true)
	h.DisableNormalizing()
	h.Add("x-trace", "a")
	h.Add("X-Trace", "b")
	h.Add("X-TRACE", "c")
	h.Add("X-Keep", "k")

	Del(h, "X-Trace", false)

	require.Empty(t, Lines(h, "X-Trace", false))
	require.Equal(t, "k", string(First(h, "X-Keep", false)))
}
