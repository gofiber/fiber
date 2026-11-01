package clocktest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Clock(t *testing.T) {
	t.Parallel()
	start := time.Unix(1_700_000_000, 0)
	clock := NewClock(start)
	require.Equal(t, start, clock.Now())
	require.Equal(t, start, clock.Now(), "reading does not advance the clock")

	clock.Add(90 * time.Second)
	require.Equal(t, start.Add(90*time.Second), clock.Now())
	clock.Add(-time.Minute)
	require.Equal(t, start.Add(30*time.Second), clock.Now())
}

func Test_Clock_Step(t *testing.T) {
	t.Parallel()
	start := time.Unix(1_700_000_000, 0)
	clock := NewClock(start)
	clock.Step = time.Second
	require.Equal(t, start.Add(time.Second), clock.Now())
	require.Equal(t, start.Add(2*time.Second), clock.Now())
	clock.Add(time.Minute)
	require.Equal(t, start.Add(63*time.Second), clock.Now())
}

func Test_Clock_ConcurrentUse(t *testing.T) {
	t.Parallel()
	clock := NewClock(time.Unix(0, 0))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				clock.Add(time.Second)
				clock.Now()
			}
		})
	}
	wg.Wait()
	require.Equal(t, time.Unix(800, 0), clock.Now())
}
