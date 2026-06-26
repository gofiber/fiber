package cache

import (
	"bytes"
	"container/heap"
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"github.com/valyala/fasthttp"
)

var errCacheWriteBudget = errors.New("write budget exhausted")

// alwaysErrWriter fails on every write.
type alwaysErrWriter struct{}

func (alwaysErrWriter) Write([]byte) (int, error) { return 0, errCacheWriteBudget }

// encodeMsgErrorSweep drives the per-write error branches of an EncodeMsg
// implementation. The msgp writer enforces an 18-byte minimum buffer, so a
// single failing writer only surfaces an error at fixed flush boundaries.
// Sweeping the buffer size moves the first flush through the encode sequence,
// exercising a different write's error branch on each iteration.
func encodeMsgErrorSweep(t *testing.T, marshaledLen int, encode func(*msgp.Writer) error) {
	t.Helper()

	sawErr := false
	for sz := 18; sz <= marshaledLen+18; sz++ {
		w := msgp.NewWriterSize(alwaysErrWriter{}, sz)
		err := encode(w)
		if err == nil {
			err = w.Flush()
		}
		if err != nil {
			sawErr = true
		}
	}
	require.True(t, sawErr)
}

func populatedItem() item {
	return item{
		headers: []cachedHeader{
			{key: []byte("X-A"), value: []byte("1")},
			{key: []byte("X-B"), value: []byte("two")},
		},
		body:            []byte("response body"),
		ctype:           []byte("text/plain"),
		cencoding:       []byte("gzip"),
		cacheControl:    []byte("max-age=60"),
		expires:         []byte("Wed, 21 Oct 2026 07:28:00 GMT"),
		etag:            []byte(`"abc123"`),
		date:            1,
		status:          200,
		age:             2,
		exp:             3,
		ttl:             4,
		forceRevalidate: true,
		revalidate:      true,
		shareable:       true,
		private:         true,
		heapidx:         5,
	}
}

func Test_item_MarshalUnmarshal_Populated(t *testing.T) {
	t.Parallel()

	v := populatedItem()
	bts, err := v.MarshalMsg(nil)
	require.NoError(t, err)

	var out item
	left, err := out.UnmarshalMsg(bts)
	require.NoError(t, err)
	require.Empty(t, left)
	require.Equal(t, v, out)
}

func Test_item_EncodeDecode_Populated(t *testing.T) {
	t.Parallel()

	v := populatedItem()
	var buf bytes.Buffer
	w := msgp.NewWriter(&buf)
	require.NoError(t, v.EncodeMsg(w))
	require.NoError(t, w.Flush())

	var out item
	require.NoError(t, out.DecodeMsg(msgp.NewReader(&buf)))
	require.Equal(t, v, out)
}

func Test_item_Decode_Truncated(t *testing.T) {
	t.Parallel()

	v := populatedItem()
	full, err := v.MarshalMsg(nil)
	require.NoError(t, err)

	for i := 0; i < len(full); i++ {
		prefix := full[:i]

		var out item
		_, uerr := out.UnmarshalMsg(prefix)
		require.Error(t, uerr, "UnmarshalMsg should fail on prefix len %d", i)

		var dec item
		require.Error(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(prefix))))
	}
}

func Test_item_EncodeMsg_WriterErrors(t *testing.T) {
	t.Parallel()

	v := populatedItem()
	full, err := v.MarshalMsg(nil)
	require.NoError(t, err)

	encodeMsgErrorSweep(t, len(full), v.EncodeMsg)
}

func Test_item_Decode_UnknownField(t *testing.T) {
	t.Parallel()

	var raw []byte
	raw = msgp.AppendMapHeader(raw, 2)
	raw = msgp.AppendString(raw, "unknownField")
	raw = msgp.AppendString(raw, "ignored")
	raw = msgp.AppendString(raw, "status")
	raw = msgp.AppendInt(raw, 204)

	var out item
	_, err := out.UnmarshalMsg(raw)
	require.NoError(t, err)
	require.Equal(t, 204, out.status)

	var dec item
	require.NoError(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(raw))))
	require.Equal(t, 204, dec.status)
}

func Test_item_Decode_LimitExceeded(t *testing.T) {
	t.Parallel()

	// headers array exceeds the configured limit of 1024.
	var raw []byte
	raw = msgp.AppendMapHeader(raw, 1)
	raw = msgp.AppendString(raw, "headers")
	raw = msgp.AppendArrayHeader(raw, 1025)

	var out item
	_, err := out.UnmarshalMsg(raw)
	require.ErrorIs(t, err, msgp.ErrLimitExceeded)

	var dec item
	require.ErrorIs(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(raw))), msgp.ErrLimitExceeded)
}

func Test_cachedHeader_Roundtrip_Populated(t *testing.T) {
	t.Parallel()

	v := cachedHeader{key: []byte("Content-Type"), value: []byte("text/html")}
	bts, err := v.MarshalMsg(nil)
	require.NoError(t, err)

	var out cachedHeader
	_, err = out.UnmarshalMsg(bts)
	require.NoError(t, err)
	require.Equal(t, v, out)

	for i := 0; i < len(bts); i++ {
		var trunc cachedHeader
		_, terr := trunc.UnmarshalMsg(bts[:i])
		require.Error(t, terr)

		var dec cachedHeader
		require.Error(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(bts[:i]))))
	}
}

func Test_cachedHeader_EncodeDecode(t *testing.T) {
	t.Parallel()

	v := cachedHeader{key: []byte("Content-Type"), value: []byte("text/html")}

	var buf bytes.Buffer
	w := msgp.NewWriter(&buf)
	require.NoError(t, v.EncodeMsg(w))
	require.NoError(t, w.Flush())

	var out cachedHeader
	require.NoError(t, out.DecodeMsg(msgp.NewReader(&buf)))
	require.Equal(t, v, out)

	full, err := v.MarshalMsg(nil)
	require.NoError(t, err)
	encodeMsgErrorSweep(t, len(full), v.EncodeMsg)
}

func Test_cachedHeader_Decode_UnknownAndLimits(t *testing.T) {
	t.Parallel()

	// Unknown field is skipped.
	var raw []byte
	raw = msgp.AppendMapHeader(raw, 2)
	raw = msgp.AppendString(raw, "zz")
	raw = msgp.AppendString(raw, "ignored")
	raw = msgp.AppendString(raw, "key")
	raw = msgp.AppendBytes(raw, []byte("X-Test"))

	var out cachedHeader
	_, err := out.UnmarshalMsg(raw)
	require.NoError(t, err)
	require.Equal(t, []byte("X-Test"), out.key)

	var dec cachedHeader
	require.NoError(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(raw))))
	require.Equal(t, []byte("X-Test"), dec.key)

	// key over the 512-byte limit.
	var bigKey []byte
	bigKey = msgp.AppendMapHeader(bigKey, 1)
	bigKey = msgp.AppendString(bigKey, "key")
	bigKey = msgp.AppendBytes(bigKey, make([]byte, 513))
	var k cachedHeader
	_, err = k.UnmarshalMsg(bigKey)
	require.ErrorIs(t, err, msgp.ErrLimitExceeded)

	// value over the 16384-byte limit.
	var bigVal []byte
	bigVal = msgp.AppendMapHeader(bigVal, 1)
	bigVal = msgp.AppendString(bigVal, "value")
	bigVal = msgp.AppendBytes(bigVal, make([]byte, 16385))
	var vv cachedHeader
	_, err = vv.UnmarshalMsg(bigVal)
	require.ErrorIs(t, err, msgp.ErrLimitExceeded)
}

// Test_item_Decode_FieldLimits covers the per-field ErrLimitExceeded branches
// of the item byte fields.
func Test_item_Decode_FieldLimits(t *testing.T) {
	t.Parallel()

	cases := []struct {
		field string
		size  int
	}{
		{"ctype", 257},
		{"cencoding", 129},
		{"cacheControl", 2049},
		{"expires", 129},
		{"etag", 257},
	}
	for _, tc := range cases {
		var raw []byte
		raw = msgp.AppendMapHeader(raw, 1)
		raw = msgp.AppendString(raw, tc.field)
		raw = msgp.AppendBytes(raw, make([]byte, tc.size))

		var out item
		_, err := out.UnmarshalMsg(raw)
		require.ErrorIs(t, err, msgp.ErrLimitExceeded, "field %s", tc.field)

		var dec item
		require.ErrorIs(t, dec.DecodeMsg(msgp.NewReader(bytes.NewReader(raw))), msgp.ErrLimitExceeded, "field %s", tc.field)
	}
}

func Test_indexedHeap_PushAndOps(t *testing.T) {
	t.Parallel()

	h := &indexedHeap{}

	// put inserts entries and returns tracking indices.
	idxA := h.put("a", 30, 1)
	idxB := h.put("b", 10, 2)
	idxC := h.put("c", 20, 3)
	require.Equal(t, 3, h.Len())

	// The lowest expiration must come out first.
	key, _ := h.removeFirst()
	require.Equal(t, "b", key)

	// Remove an arbitrary tracked entry.
	key, _ = h.remove(idxC)
	require.Equal(t, "c", key)
	_ = idxA
	_ = idxB

	// Directly exercise the heap.Interface Push method (not used by put()).
	h2 := &indexedHeap{indices: []int{0}}
	heap.Push(h2, heapEntry{key: "z", exp: 5, idx: 0})
	require.Equal(t, 1, h2.Len())
	require.Equal(t, "z", h2.entries[0].key)
}

func Test_parseUintDirective(t *testing.T) {
	t.Parallel()

	v, ok := parseUintDirective([]byte("60"))
	require.True(t, ok)
	require.Equal(t, uint64(60), v)

	_, ok = parseUintDirective(nil)
	require.False(t, ok)

	_, ok = parseUintDirective([]byte("not-a-number"))
	require.False(t, ok)

	// Invalid max-age values are ignored when parsing full directives.
	parsed := parseResponseCacheControl([]byte("max-age=abc, s-maxage=xyz"))
	require.False(t, parsed.maxAgeSet)
	require.False(t, parsed.sMaxAgeSet)
}

func Test_allowsSharedCacheDirectives(t *testing.T) {
	t.Parallel()

	require.False(t, allowsSharedCacheDirectives(responseCacheControl{hasPrivate: true}))
	require.True(t, allowsSharedCacheDirectives(responseCacheControl{hasPublic: true}))
	require.True(t, allowsSharedCacheDirectives(responseCacheControl{sMaxAgeSet: true}))
	require.True(t, allowsSharedCacheDirectives(responseCacheControl{mustRevalidate: true}))
	require.True(t, allowsSharedCacheDirectives(responseCacheControl{proxyRevalidate: true}))
	require.False(t, allowsSharedCacheDirectives(responseCacheControl{}))
}

func Test_secondsConversions_Overflow(t *testing.T) {
	t.Parallel()

	// secondsToTime clamps values beyond math.MaxInt64.
	require.Equal(t, time.Unix(math.MaxInt64, 0).UTC(), secondsToTime(math.MaxUint64))
	require.Equal(t, time.Unix(5, 0).UTC(), secondsToTime(5))

	// secondsToDuration clamps on overflow.
	require.Equal(t, time.Duration(math.MaxInt64), secondsToDuration(math.MaxUint64))
	require.Equal(t, 5*time.Second, secondsToDuration(5))
}

func Test_makeHashAuthFunc(t *testing.T) {
	t.Parallel()

	pool := &sync.Pool{}
	fn := makeHashAuthFunc(pool)
	got := fn([]byte("Bearer token"))
	require.Len(t, got, hexLen)
	// Stable for the same input, and uses the pool on the second call.
	require.Equal(t, got, fn([]byte("Bearer token")))
}

func Test_manager_get_StorageErrors(t *testing.T) {
	t.Parallel()

	// Storage GetWithContext returns an error.
	storage := newFailingCacheStorage()
	storage.errs["get|k"] = errors.New("boom")
	m := newManager(storage, true)
	_, err := m.get(context.Background(), "k")
	require.ErrorContains(t, err, redactedKey)

	// Stored bytes fail to unmarshal.
	storage2 := newFailingCacheStorage()
	storage2.data["k"] = []byte{0xff, 0xff, 0xff}
	m2 := newManager(storage2, false)
	_, err = m2.get(context.Background(), "k")
	require.ErrorContains(t, err, "unmarshal")

	// Cache miss.
	m3 := newManager(newFailingCacheStorage(), false)
	_, err = m3.get(context.Background(), "missing")
	require.ErrorIs(t, err, errCacheMiss)

	// Memory-backed unexpected type.
	m4 := newManager(nil, false)
	m4.memory.Set("k", "not-an-item", time.Minute)
	_, err = m4.get(context.Background(), "k")
	require.ErrorContains(t, err, "unexpected entry type")
}

func Test_manager_getRaw_Paths(t *testing.T) {
	t.Parallel()

	// Storage error.
	storage := newFailingCacheStorage()
	storage.errs["get|k"] = errors.New("boom")
	m := newManager(storage, false)
	_, err := m.getRaw(context.Background(), "k")
	require.ErrorContains(t, err, "boom")

	// Storage hit.
	storage2 := newFailingCacheStorage()
	storage2.data["k"] = []byte("raw-value")
	m2 := newManager(storage2, false)
	raw, err := m2.getRaw(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, []byte("raw-value"), raw)

	// Memory hit.
	m3 := newManager(nil, false)
	require.NoError(t, m3.setRaw(context.Background(), "k", []byte("mem"), time.Minute))
	raw, err = m3.getRaw(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, []byte("mem"), raw)

	// Memory unexpected raw type.
	m4 := newManager(nil, false)
	m4.memory.Set("k", 12345, time.Minute)
	_, err = m4.getRaw(context.Background(), "k")
	require.ErrorContains(t, err, "unexpected raw entry type")

	// Miss.
	m5 := newManager(newFailingCacheStorage(), false)
	_, err = m5.getRaw(context.Background(), "missing")
	require.ErrorIs(t, err, errCacheMiss)
}

func Test_manager_set_StorageError(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	storage.errs["set|k"] = errors.New("boom")
	m := newManager(storage, false)

	it := m.acquire()
	it.status = 200
	err := m.set(context.Background(), "k", it, time.Minute)
	require.ErrorContains(t, err, "boom")

	// setRaw storage error.
	require.ErrorContains(t, m.setRaw(context.Background(), "k", []byte("v"), time.Minute), "boom")
}

func Test_manager_get_StorageRoundTrip(t *testing.T) {
	t.Parallel()

	storage := newFailingCacheStorage()
	m := newManager(storage, false)

	it := m.acquire()
	it.status = 201
	it.body = []byte("hello")
	require.NoError(t, m.set(context.Background(), "k", it, time.Minute))

	got, err := m.get(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, 201, got.status)
	require.Equal(t, []byte("hello"), got.body)
}

func Test_varyManifest_StoreLoad(t *testing.T) {
	t.Parallel()

	m := newManager(newFailingCacheStorage(), false)
	ctx := context.Background()

	// Empty names is a no-op store.
	require.NoError(t, storeVaryManifest(ctx, m, "mk", nil, time.Minute))

	// Missing manifest -> no names, no error.
	names, ok, err := loadVaryManifest(ctx, m, "mk")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, names)

	// Store and load a real manifest.
	require.NoError(t, storeVaryManifest(ctx, m, "mk", []string{"accept", "accept-encoding"}, time.Minute))
	names, ok, err = loadVaryManifest(ctx, m, "mk")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"accept", "accept-encoding"}, names)

	// A wildcard manifest is treated as uncacheable.
	require.NoError(t, m.setRaw(ctx, "star", []byte("*"), time.Minute))
	names, ok, err = loadVaryManifest(ctx, m, "star")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, names)

	// Storage error propagates.
	storage := newFailingCacheStorage()
	storage.errs["get|mk"] = errors.New("boom")
	mErr := newManager(storage, false)
	_, _, err = loadVaryManifest(ctx, mErr, "mk")
	require.ErrorContains(t, err, "boom")
}

func Test_makeBuildVaryKeyFunc(t *testing.T) {
	t.Parallel()

	fn := makeBuildVaryKeyFunc(&sync.Pool{})

	var hdr fasthttp.RequestHeader
	hdr.Set("Accept", "application/json")
	hdr.Set("Accept-Encoding", "gzip")

	key := fn([]string{"accept", "accept-encoding"}, &hdr)
	require.Contains(t, key, "|vary|")
	// Deterministic for the same inputs (also exercises the pooled buffer path).
	require.Equal(t, key, fn([]string{"accept", "accept-encoding"}, &hdr))
}
