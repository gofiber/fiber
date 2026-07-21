package proxy

import (
	"net/url"
	"sync"
	"testing"
)

func BenchmarkURLRoundrobinGet(b *testing.B) {
	pool := []*url.URL{
		{Scheme: schemeHTTP, Host: "a.example"},
		{Scheme: schemeHTTP, Host: "b.example"},
		{Scheme: schemeHTTP, Host: "c.example"},
	}
	b.Run("atomic", func(b *testing.B) {
		r := &urlRoundrobin{pool: pool}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = r.get()
			}
		})
	})
	b.Run("mutex", func(b *testing.B) {
		r := &mutexURLRoundrobin{pool: pool}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = r.get()
			}
		})
	})
}

type mutexURLRoundrobin struct {
	pool []*url.URL

	current int
	sync.Mutex
}

func (r *mutexURLRoundrobin) get() *url.URL {
	r.Lock()
	defer r.Unlock()

	if r.current >= len(r.pool) {
		r.current %= len(r.pool)
	}
	result := r.pool[r.current]
	r.current++
	return result
}
