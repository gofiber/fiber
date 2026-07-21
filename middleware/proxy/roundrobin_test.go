package proxy

import (
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_RoundRobin_WrapsAround(t *testing.T) {
	t.Parallel()
	a := &url.URL{Scheme: "http", Host: "a"}
	b := &url.URL{Scheme: "http", Host: "b"}
	c := &url.URL{Scheme: "http", Host: "c"}
	r := &urlRoundrobin{pool: []*url.URL{a, b, c}}
	r.next.Store(^uint64(0) - 1)
	require.Same(t, c, r.get())
	require.Same(t, a, r.get())
	require.Same(t, b, r.get())
}

func Test_RoundRobin_ConcurrentSelection(t *testing.T) {
	t.Parallel()
	pool := []*url.URL{
		{Scheme: "http", Host: "a"},
		{Scheme: "http", Host: "b"},
		{Scheme: "http", Host: "c"},
	}
	r := &urlRoundrobin{pool: pool}

	var counts [3]atomic.Int32
	var wg sync.WaitGroup
	for range 300 {
		wg.Go(func() {
			selected := r.get()
			for i, candidate := range pool {
				if selected == candidate {
					counts[i].Add(1)
					return
				}
			}
		})
	}
	wg.Wait()

	for i := range counts {
		require.Equal(t, int32(100), counts[i].Load())
	}
}
