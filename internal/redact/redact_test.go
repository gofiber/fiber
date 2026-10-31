package redact

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Prefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "shorter than min", in: "abc", want: Mask},
		{name: "exactly below min", in: "abcdefg", want: Mask},
		{name: "exactly min", in: "abcdefgh", want: "abcd" + Mask},
		{name: "long", in: "session-value", want: "sess" + Mask},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, Prefix(tt.in))
		})
	}
}

func Test_Value(t *testing.T) {
	t.Parallel()
	require.Equal(t, "[redacted]", Placeholder)
	require.Equal(t, Placeholder, Value("secret", true))
	require.Equal(t, Placeholder, Value("", true))
	require.Equal(t, "secret", Value("secret", false))
	require.Empty(t, Value("", false))
}

func Test_Masker(t *testing.T) {
	t.Parallel()
	require.Equal(t, Placeholder, Masker(true)("https://example.com"))
	require.Equal(t, "https://example.com", Masker(false)("https://example.com"))
}
