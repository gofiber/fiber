package logtemplate

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

// referenceIndexControlByte is the obvious scalar implementation the SWAR scan
// must agree with.
func referenceIndexControlByte(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			continue
		}
		if s[i] < 0x20 || s[i] == 0x7f {
			return i
		}
	}
	return -1
}

func Test_IsControlByte(t *testing.T) {
	t.Parallel()

	for b := range 256 {
		want := (b < 0x20 || b == 0x7f) && b != '\t'
		require.Equal(t, want, IsControlByte(byte(b)), "byte 0x%02x", b)
	}
}

// Test_IndexControlByte_MatchesReference walks every length across the SWAR
// boundaries (the 8-byte main loop, the overlapping tail word, and the
// byte-wise path for short inputs) with the control byte at every position.
func Test_IndexControlByte_MatchesReference(t *testing.T) {
	t.Parallel()

	const clean = "abcdefghijklmnopqrstuvwxyz0123456789"
	for n := range 25 {
		base := clean[:n]
		require.Equal(t, referenceIndexControlByte(base), IndexControlByte(base), "clean n=%d", n)

		for pos := range n {
			for _, bad := range []byte{0x00, '\n', '\r', 0x1f, 0x7f} {
				b := []byte(base)
				b[pos] = bad
				s := string(b)
				require.Equal(t, referenceIndexControlByte(s), IndexControlByte(s),
					"n=%d pos=%d byte=0x%02x", n, pos, bad)
				require.Equal(t, referenceIndexControlByte(s), IndexControlByte([]byte(s)),
					"[]byte n=%d pos=%d byte=0x%02x", n, pos, bad)
			}
		}
	}
}

// Test_IndexControlByte_HighBytesAreNotControls guards the SWAR range mask
// against matching bytes >= 0x80, which would corrupt UTF-8 in log lines: the
// C1 range 0x80-0x9f looks like a control range to a naive comparison.
func Test_IndexControlByte_HighBytesAreNotControls(t *testing.T) {
	t.Parallel()

	for n := 1; n <= 20; n++ {
		for pos := range n {
			for _, high := range []byte{0x80, 0x9f, 0xc3, 0xff} {
				b := bytes.Repeat([]byte("x"), n)
				b[pos] = high
				require.Equal(t, -1, IndexControlByte(string(b)),
					"n=%d pos=%d byte=0x%02x must not be treated as a control", n, pos, high)
			}
		}
	}

	require.Equal(t, -1, IndexControlByte("héllo wörld — ünïcode"))
}

func Test_IndexControlByte_Randomized(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	for range 20000 {
		b := make([]byte, rng.Intn(40))
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
		s := string(b)
		require.Equal(t, referenceIndexControlByte(s), IndexControlByte(s), "input %q", s)
	}
}

func Test_ScrubControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "crlf", in: "a\r\nb", want: "a  b"},
		{name: "nul", in: "a\x00b", want: "a b"},
		{name: "del", in: "a\x7fb", want: "a b"},
		{name: "tab preserved", in: "a\tb", want: "a\tb"},
		{name: "esc", in: "a\x1b[31mb", want: "a [31mb"},
		{name: "leading", in: "\nabc", want: " abc"},
		{name: "trailing", in: "abc\n", want: "abc "},
		{name: "utf8 untouched", in: "héllo\nwörld", want: "héllo wörld"},
		{name: "long multiword", in: strings.Repeat("abcdefgh", 3) + "\n" + strings.Repeat("x", 9), want: strings.Repeat("abcdefgh", 3) + " " + strings.Repeat("x", 9)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			idx := IndexControlByte(tt.in)
			require.Equal(t, referenceIndexControlByte(tt.in), idx)
			if idx == -1 {
				require.Equal(t, tt.want, tt.in)
				return
			}
			require.Equal(t, tt.want, string(ScrubControls(tt.in, idx)))
			require.Equal(t, tt.want, string(ScrubControls([]byte(tt.in), idx)))
		})
	}
}

// Test_ScrubControls_LeavesNoControlByte is the property that actually matters
// for log injection: whatever comes out must contain no control byte at all.
func Test_ScrubControls_LeavesNoControlByte(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(2))
	for range 20000 {
		b := make([]byte, rng.Intn(40))
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
		s := string(b)
		idx := IndexControlByte(s)
		if idx == -1 {
			continue
		}
		out := ScrubControls(s, idx)
		require.Len(t, out, len(s), "scrub must preserve length")
		require.Equal(t, -1, IndexControlByte(out), "control byte survived in %q", out)
	}
}

func Test_WriteSanitized(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	n, err := WriteSanitizedString(buf, "clean")
	require.NoError(t, err)
	require.Equal(t, 5, n)

	buf.Reset()
	n, err = WriteSanitizedString(buf, "a\r\nb")
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, "a  b", buf.String())

	buf.Reset()
	n, err = WriteSanitized(buf, []byte("a\nb"))
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, "a b", buf.String())

	// The []byte variant's clean fast path forwards straight to Write without
	// copying, so it needs its own case: the dirty case above exercises a
	// different branch entirely.
	buf.Reset()
	n, err = WriteSanitized(buf, []byte("clean\tbytes"))
	require.NoError(t, err)
	require.Equal(t, 11, n)
	require.Equal(t, "clean\tbytes", buf.String(), "tab must survive and nothing else may change")

	// Empty input takes the same fast path and must not panic.
	buf.Reset()
	n, err = WriteSanitized(buf, nil)
	require.NoError(t, err)
	require.Zero(t, n)
}

// Test_ScrubControls_NegativeIndex covers the clamp documented on ScrubControls:
// callers may pass IndexControlByte's -1 straight through, so a negative index
// must scan from the start rather than panicking.
func Test_ScrubControls_NegativeIndex(t *testing.T) {
	t.Parallel()

	require.Equal(t, "a b", string(ScrubControls("a\nb", -1)),
		"a negative index must be clamped to 0, not panic")
	require.Equal(t, "clean", string(ScrubControls("clean", IndexControlByte("clean"))),
		"ScrubControls(s, IndexControlByte(s)) must be safe on clean input")
}
