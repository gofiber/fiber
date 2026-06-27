package keylock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockerSameKeyBlocksUntilRelease(t *testing.T) {
	t.Parallel()

	locker := New(128)
	release := locker.Lock("alpha")
	defer release()

	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		releaseNext := locker.Lock("alpha")
		releaseNext()
	}()

	select {
	case <-acquired:
		t.Fatal("same key lock should block until release")
	case <-time.After(50 * time.Millisecond):
	}

	release()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for same key lock after release")
	}
}

func TestNextPow2(t *testing.T) {
	t.Parallel()

	cases := map[int]int{
		-1:   1,
		0:    1,
		1:    1,
		2:    2,
		3:    4,
		5:    8,
		8:    8,
		127:  128,
		128:  128,
		129:  256,
		1000: 1024,
	}
	for in, want := range cases {
		require.Equalf(t, want, nextPow2(in), "nextPow2(%d)", in)
	}
}

func TestShardCountFloorAndPow2(t *testing.T) {
	t.Parallel()

	// Result is always a power of two and never below the requested floor.
	for _, minShards := range []int{1, 16, 128, 1000} {
		got := shardCount(minShards)
		require.GreaterOrEqual(t, got, minShards)
		require.Equalf(t, 0, got&(got-1), "shardCount(%d)=%d is not a power of two", minShards, got)
	}

	// A non-positive floor still yields a valid power-of-two count.
	require.Equal(t, 0, shardCount(0)&(shardCount(0)-1))
}

func TestLockerDifferentKeysDoNotBlock(t *testing.T) {
	t.Parallel()

	locker := New(128)
	release := locker.Lock("alpha")
	defer release()

	acquired := make(chan struct{})
	go func() {
		releaseNext := locker.Lock("bravo")
		releaseNext()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("different keys should not block each other")
	}

	require.NotPanics(t, release)
}
