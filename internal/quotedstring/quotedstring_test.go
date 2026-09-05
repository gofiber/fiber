package quotedstring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_Escape covers each output dialect and the no-allocation fast path.
func Test_Escape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"empty", "", ""},
		{"simple", "simple", "simple"},
		{"long simple", "clean-value", "clean-value"},
		{"backslash", "A\\B", "A\\\\B"},
		{"quote", `He said "Yo"`, `He said \"Yo\"`},
		{"quote in overlapping tail", `12345678"`, `12345678\"`},
		{"newline", "Hello\n", "Hello\\n"},
		{"carriage", "Hello\r", "Hello\\r"},
		{"controls", string([]byte{0, 31, 127}), "%00%1F%7F"},
		{"tab", "a\tb", "a%09b"},
		{"mixed", "test \"A\n\r" + string([]byte{1}) + "\\", `test \"A\n\r%01\\`},
		{"non-ASCII", "café", "café"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.out, Escape(test.in))
		})
	}
}

// Test_Escape_DetachesFromPooledBuffer verifies that later pool reuse cannot
// mutate an earlier result.
func Test_Escape_DetachesFromPooledBuffer(t *testing.T) {
	t.Parallel()

	first := Escape(`A\B`)
	second := Escape(`C"D`)

	require.Equal(t, `A\\B`, first)
	require.Equal(t, `C\"D`, second)
	require.Equal(t, `A\\B`, first)
}
