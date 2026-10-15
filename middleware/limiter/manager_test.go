package limiter

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
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

// limiterErrWriter is an io.Writer that accepts up to n bytes total and then
// fails, used to exercise the error-handling branches of EncodeMsg.
type limiterErrWriter struct {
	n int
}

var errLimiterWriteBudget = errors.New("write budget exhausted")

func (w *limiterErrWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, errLimiterWriteBudget
	}
	if len(p) > w.n {
		k := w.n
		w.n = 0
		return k, errLimiterWriteBudget
	}
	w.n -= len(p)
	return len(p), nil
}

func Test_manager_get_UnmarshalError(t *testing.T) {
	t.Parallel()

	storage := newFakeLimiterStorage()
	storage.data["k"] = []byte{0xff, 0xff, 0xff} // not a valid msgp item

	m := newManager(storage, true)
	_, err := m.get(context.Background(), "k")
	require.Error(t, err)
	require.ErrorContains(t, err, redactedKey)
}

func Test_manager_get_RoundTripWithStorage(t *testing.T) {
	t.Parallel()

	storage := newFakeLimiterStorage()
	m := newManager(storage, false)

	it := m.acquire()
	it.currHits = 2
	it.prevHits = 1
	it.exp = 42
	require.NoError(t, m.set(context.Background(), "k", it, time.Second))

	got, err := m.get(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, 2, got.currHits)
	require.Equal(t, 1, got.prevHits)
	require.Equal(t, uint64(42), got.exp)
}

func Test_manager_get_MemoryUnexpectedType(t *testing.T) {
	t.Parallel()

	m := newManager(nil, false) // memory-backed
	m.memory.Set("k", "not-an-item", time.Minute)

	_, err := m.get(context.Background(), "k")
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected entry type")
}

// Test_item_Decode_Truncated exercises the per-read error branches of the
// generated item decoders by feeding every truncated prefix.
func Test_item_Decode_Truncated(t *testing.T) {
	t.Parallel()

	full, err := item{currHits: 3, prevHits: 5, exp: 99}.MarshalMsg(nil)
	require.NoError(t, err)

	for i := range len(full) {
		prefix := full[:i]

		var out item
		_, uerr := out.UnmarshalMsg(prefix)
		require.Error(t, uerr, "UnmarshalMsg should fail on prefix len %d", i)

		var dec item
		require.Error(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(prefix))))
	}
}

// Test_item_Decode_UnknownField covers the default/skip branch of the item
// decoders.
func Test_item_Decode_UnknownField(t *testing.T) {
	t.Parallel()

	var raw []byte
	raw = msgp.AppendMapHeader(raw, 2)
	raw = msgp.AppendString(raw, "zz")
	raw = msgp.AppendString(raw, "ignored")
	raw = msgp.AppendString(raw, "exp")
	raw = msgp.AppendUint64(raw, 7)

	var out item
	_, err := out.UnmarshalMsg(raw)
	require.NoError(t, err)
	require.Equal(t, uint64(7), out.exp)

	var dec item
	require.NoError(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(raw))))
	require.Equal(t, uint64(7), dec.exp)
}

// Test_item_EncodeMsg_WriterErrors drives the error branches of EncodeMsg by
// failing the underlying writer at every byte offset.
func Test_item_EncodeMsg_WriterErrors(t *testing.T) {
	t.Parallel()

	full, err := item{currHits: 3, prevHits: 5, exp: 99}.MarshalMsg(nil)
	require.NoError(t, err)

	// A writer that accepts fewer than the full encoding's bytes must always
	// surface an error (either mid-encode or on Flush), so assert per budget.
	for budget := range len(full) {
		w := msgp.NewWriterSize(&limiterErrWriter{n: budget}, 8)
		encErr := item{currHits: 3, prevHits: 5, exp: 99}.EncodeMsg(w)
		if encErr == nil {
			encErr = w.Flush()
		}
		require.Error(t, encErr, "expected writer failure for budget %d", budget)
	}
}
