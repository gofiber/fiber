package fieldname

import (
	"bufio"
	"bytes"
	"testing"

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
// whatever it holds, so the value beside it was invisible to every caller —
// including the CSRF origin check, which treats an unreadable Origin as an
// absent one and skips itself on a plaintext request.
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
