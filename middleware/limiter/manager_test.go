package limiter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_BoundKey_ShortKeyUnchanged(t *testing.T) {
	t.Parallel()

	require.Equal(t, "1.2.3.4", boundKey("1.2.3.4"))
	require.Empty(t, boundKey(""))
	require.Equal(t, strings.Repeat("a", maxKeyLength), boundKey(strings.Repeat("a", maxKeyLength)))
}

func Test_BoundKey_OversizedKeyIsHashed(t *testing.T) {
	t.Parallel()

	key := strings.Repeat("a", maxKeyLength+1)
	got := boundKey(key)

	require.True(t, strings.HasPrefix(got, hashPrefix), "bounded key must carry the reserved prefix, got %q", got)
	require.LessOrEqual(t, len(got), maxKeyLength, "a hashed key must itself fit the bound it exists to enforce")
}

func Test_BoundKey_Deterministic(t *testing.T) {
	t.Parallel()

	key := strings.Repeat("x", maxKeyLength*3)
	first := boundKey(key)
	second := boundKey(key)
	require.Equal(t, first, second, "the same oversized key must map to the same bucket every time, or rate limiting stops working for that client")
}

func Test_BoundKey_DistinctOversizedKeysStayDistinct(t *testing.T) {
	t.Parallel()

	a := boundKey(strings.Repeat("a", maxKeyLength+10))
	b := boundKey(strings.Repeat("b", maxKeyLength+10))
	require.NotEqual(t, a, b, "two different oversized clients must not collapse into one shared bucket")
}

// Test_BoundKey_ReservedPrefixCannotBeForged pins the collision guard: a short
// key that happens to start with hashPrefix must not pass through verbatim,
// or a client choosing that value could deliberately land in — or be
// confused for — the bucket a genuinely oversized key hashes to.
func Test_BoundKey_ReservedPrefixCannotBeForged(t *testing.T) {
	t.Parallel()

	forged := hashPrefix + "not-actually-a-hash"
	got := boundKey(forged)
	require.NotEqual(t, forged, got, "a short key claiming the reserved prefix must be re-hashed, not passed through")
	require.True(t, strings.HasPrefix(got, hashPrefix))
}
