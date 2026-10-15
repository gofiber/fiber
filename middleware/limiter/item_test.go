package limiter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_item_hitBucket(t *testing.T) {
	t.Parallel()

	e := &item{currHits: 3, prevHits: 7}

	// ts before the end of the hit's window -> current bucket.
	require.Equal(t, &e.currHits, e.hitBucket(100, 50, 60))

	// ts within one window after the hit's window ended -> previous bucket.
	require.Equal(t, &e.prevHits, e.hitBucket(100, 130, 60))

	// ts more than one window past the end of the hit's window -> nil.
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

	// Once the window ended a fresh one starts and the hits are forgotten.
	e = &item{currHits: 4, exp: 110}
	e.rotateFixed(110, 10)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(120), e.exp)
}

func Test_item_rate(t *testing.T) {
	t.Parallel()

	e := &item{prevHits: 4, currHits: 2}

	// The previous window counts in full at the start of a window...
	require.Equal(t, 6, e.rate(10, 10))
	// ...fades out as the window runs, rounded up...
	require.Equal(t, 4, e.rate(5, 10))
	require.Equal(t, 3, e.rate(1, 10))
	// ...and no longer counts once the window is over.
	require.Equal(t, 2, e.rate(0, 10))
}
