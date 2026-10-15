package limiter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_item_hitBucket(t *testing.T) {
	t.Parallel()

	e := &item{currHits: 3, prevHits: 7}

	// ts before the recorded window expiry -> current bucket.
	require.Equal(t, &e.currHits, e.hitBucket(100, 50, 60))

	// ts within one expiration after the window expiry -> previous bucket.
	require.Equal(t, &e.prevHits, e.hitBucket(100, 130, 60))

	// ts more than one expiration past the window expiry -> nil.
	require.Nil(t, e.hitBucket(100, 200, 60))
}

func Test_item_rotateSliding(t *testing.T) {
	t.Parallel()

	// Fresh entry sets expiration.
	e := &item{}
	require.Equal(t, uint64(10), e.rotateSliding(100, 10))
	require.Equal(t, uint64(110), e.exp)

	// Entry expired within one window -> previous hits carry over.
	e = &item{currHits: 4, exp: 100}
	e.rotateSliding(105, 10)
	require.Equal(t, 4, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(110), e.exp)

	// Entry expired beyond a full window -> everything resets.
	e = &item{currHits: 4, prevHits: 9, exp: 100}
	e.rotateSliding(200, 10)
	require.Equal(t, 0, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(210), e.exp)
}

func Test_item_rotateFixed(t *testing.T) {
	t.Parallel()

	// Fresh entry opens a window.
	e := &item{}
	e.rotateFixed(100, 10)
	require.Equal(t, uint64(110), e.exp)
	require.Equal(t, 0, e.currHits)

	// Inside the window nothing moves.
	e = &item{currHits: 4, exp: 110}
	e.rotateFixed(109, 10)
	require.Equal(t, 4, e.currHits)
	require.Equal(t, uint64(110), e.exp)

	// After the window a fresh one starts.
	e = &item{currHits: 4, exp: 110}
	e.rotateFixed(110, 10)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(120), e.exp)
}

func Test_item_rate(t *testing.T) {
	t.Parallel()

	e := &item{prevHits: 4, currHits: 2}

	// The 4 previous hits weigh in full, by half, by a tenth (rounded up) and not at all.
	require.Equal(t, 6, e.rate(10, 10))
	require.Equal(t, 4, e.rate(5, 10))
	require.Equal(t, 3, e.rate(1, 10))
	require.Equal(t, 2, e.rate(0, 10))

	// More time left than the window cannot weigh more than the whole previous window.
	require.Equal(t, 6, e.rate(45, 10))
}
