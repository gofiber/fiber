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
