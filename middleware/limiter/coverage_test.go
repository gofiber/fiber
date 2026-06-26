package limiter

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"github.com/valyala/fasthttp"
)

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

func Test_secondsToDuration(t *testing.T) {
	t.Parallel()

	d, ok := secondsToDuration(5)
	require.True(t, ok)
	require.Equal(t, 5*time.Second, d)

	// Overflow: seconds larger than math.MaxInt64 / time.Second.
	const maxSeconds = uint64(math.MaxInt64 / int64(time.Second))
	d, ok = secondsToDuration(maxSeconds + 1)
	require.False(t, ok)
	require.Equal(t, time.Duration(math.MaxInt64), d)
}

func Test_ttlDuration(t *testing.T) {
	t.Parallel()

	// Normal case.
	require.Equal(t, 3*time.Second, ttlDuration(1, 2))

	const maxSeconds = uint64(math.MaxInt64 / int64(time.Second))

	// resetInSec overflows.
	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(maxSeconds+1, 1))

	// expiration overflows.
	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(1, maxSeconds+1))

	// reset + expiration overflows even though each fits individually.
	require.Equal(t, time.Duration(math.MaxInt64), ttlDuration(maxSeconds, maxSeconds))
}

func Test_bucketForOriginalHit(t *testing.T) {
	t.Parallel()

	e := &item{currHits: 3, prevHits: 7}

	// ts before the recorded window expiry -> current bucket.
	require.Equal(t, &e.currHits, bucketForOriginalHit(e, 100, 50, 60))

	// ts within one expiration after the window expiry -> previous bucket.
	require.Equal(t, &e.prevHits, bucketForOriginalHit(e, 100, 130, 60))

	// ts more than one expiration past the window expiry -> nil.
	require.Nil(t, bucketForOriginalHit(e, 100, 200, 60))
}

func Test_rotateWindow(t *testing.T) {
	t.Parallel()

	// Fresh entry sets expiration.
	e := &item{}
	require.Equal(t, uint64(10), rotateWindow(e, 100, 10))
	require.Equal(t, uint64(110), e.exp)

	// Entry expired within one window -> previous hits carry over.
	e = &item{currHits: 4, exp: 100}
	rotateWindow(e, 105, 10)
	require.Equal(t, 4, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(110), e.exp)

	// Entry expired beyond a full window -> everything resets.
	e = &item{currHits: 4, prevHits: 9, exp: 100}
	rotateWindow(e, 200, 10)
	require.Equal(t, 0, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(210), e.exp)
}

func Test_getEffectiveStatusCode_FiberError(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(c) })

	c.Response().SetStatusCode(fiber.StatusOK)

	// A real *fiber.Error returns its code.
	require.Equal(t, fiber.StatusTeapot, getEffectiveStatusCode(c, fiber.NewError(fiber.StatusTeapot, "teapot")))

	// A generic error falls back to the response status code.
	c.Response().SetStatusCode(fiber.StatusBadGateway)
	require.Equal(t, fiber.StatusBadGateway, getEffectiveStatusCode(c, errors.New("generic")))
}

func Test_manager_get_UnmarshalError(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
	storage.data["k"] = []byte{0xff, 0xff, 0xff} // not a valid msgp item

	m := newManager(storage, true)
	_, err := m.get(context.Background(), "k")
	require.Error(t, err)
	require.ErrorContains(t, err, redactedKey)
}

func Test_manager_get_RoundTripWithStorage(t *testing.T) {
	t.Parallel()

	storage := newFailingLimiterStorage()
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

	sawErr := false
	for budget := range len(full) {
		w := msgp.NewWriterSize(&limiterErrWriter{n: budget}, 8)
		encErr := item{currHits: 3, prevHits: 5, exp: 99}.EncodeMsg(w)
		if encErr == nil {
			encErr = w.Flush()
		}
		if encErr != nil {
			sawErr = true
		}
	}
	require.True(t, sawErr)
}
