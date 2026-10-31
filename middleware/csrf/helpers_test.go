package csrf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_CSRF_Security_CompareConstantTime verifies the logical behavior of the
// constant-time comparison helpers used to validate tokens. The functions must
// only report equality for byte/string-identical inputs.
func Test_CSRF_Security_CompareConstantTime(t *testing.T) {
	t.Parallel()
	t.Run("strings", func(t *testing.T) {
		t.Parallel()
		require.True(t, compareStrings("abc123", "abc123"))
		require.False(t, compareStrings("abc123", "abc124"))
		require.False(t, compareStrings("abc", "abcd"))      // different length
		require.False(t, compareStrings("", "x"))            // empty vs non-empty
		require.True(t, compareStrings("", ""))              // both empty
		require.False(t, compareStrings("Abc123", "abc123")) // case sensitive
	})

	t.Run("tokens", func(t *testing.T) {
		t.Parallel()
		require.True(t, compareTokens([]byte("raw-token"), []byte("raw-token")))
		require.False(t, compareTokens([]byte("raw-token"), []byte("raw-tokeX")))
		require.False(t, compareTokens([]byte("short"), []byte("shorter")))
		require.False(t, compareTokens([]byte(nil), []byte("x")))
		require.True(t, compareTokens([]byte(nil), []byte(nil)))
	})
}
