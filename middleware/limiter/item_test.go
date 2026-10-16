package limiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_item_hitBucket(t *testing.T) {
	t.Parallel()

	e := &item{currHits: 3, prevHits: 7}

	// Current window.
	require.Equal(t, &e.currHits, e.hitBucket(100, 50, 60))

	// Previous window.
	require.Equal(t, &e.prevHits, e.hitBucket(100, 130, 60))

	// Aged out.
	require.Nil(t, e.hitBucket(100, 200, 60))
}

func Test_item_rotateSliding(t *testing.T) {
	t.Parallel()

	// Fresh entry.
	e := &item{}
	require.Equal(t, uint64(10), e.rotateSliding(100, 10))
	require.Equal(t, uint64(110), e.exp)

	// Ended within a window: hits carry over.
	e = &item{currHits: 4, exp: 100}
	e.rotateSliding(105, 10)
	require.Equal(t, 4, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(110), e.exp)

	// Ended a window ago: reset.
	e = &item{currHits: 4, prevHits: 9, exp: 100}
	e.rotateSliding(200, 10)
	require.Equal(t, 0, e.prevHits)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(210), e.exp)
}

func Test_item_rotateFixed(t *testing.T) {
	t.Parallel()

	// Fresh entry.
	e := &item{}
	e.rotateFixed(100, 10)
	require.Equal(t, uint64(110), e.exp)
	require.Equal(t, 0, e.currHits)

	// Open window.
	e = &item{currHits: 4, exp: 110}
	e.rotateFixed(109, 10)
	require.Equal(t, 4, e.currHits)
	require.Equal(t, uint64(110), e.exp)

	// Ended window.
	e = &item{currHits: 4, exp: 110}
	e.rotateFixed(110, 10)
	require.Equal(t, 0, e.currHits)
	require.Equal(t, uint64(120), e.exp)
}

func Test_item_rate(t *testing.T) {
	t.Parallel()

	e := &item{prevHits: 4, currHits: 2}

	// Weights 1, 1/2, 1/10 (rounded up) and 0.
	require.Equal(t, 6, e.rate(10, 10))
	require.Equal(t, 4, e.rate(5, 10))
	require.Equal(t, 3, e.rate(1, 10))
	require.Equal(t, 2, e.rate(0, 10))

	// Capped at the whole previous window.
	require.Equal(t, 6, e.rate(45, 10))
}

func Test_item_ttl(t *testing.T) {
	t.Parallel()

	// Until the window ends.
	require.Equal(t, 59*time.Second, (&item{exp: 60}).ttl(1))

	// At least a second.
	require.Equal(t, time.Second, (&item{exp: 60}).ttl(60))
	require.Equal(t, time.Second, (&item{exp: 60}).ttl(100))
}
