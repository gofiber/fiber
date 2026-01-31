package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/storage/memory"
)

// ---------------------------------------------------------------------------
// tagIndex benchmarks
// ---------------------------------------------------------------------------

// BenchmarkTagIndex_Has measures the has() fast path with varying index sizes.
// has() is called on every cache hit to decide whether the tag index needs
// re-population; it must stay O(1) regardless of index size.
func BenchmarkTagIndex_Has(b *testing.B) {
	for _, n := range []int{10, 100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("keys=%d", n), func(b *testing.B) {
			ti := newTagIndex()
			for i := range n {
				ti.add(fmt.Sprintf("key:%d", i), []string{fmt.Sprintf("tag:%d", i%10)})
			}
			target := fmt.Sprintf("key:%d", n/2)
			b.ResetTimer()
			for range b.N {
				ti.has(target)
			}
		})
	}
}

// BenchmarkTagIndex_Has_Parallel stress-tests has() under concurrent readers,
// matching the real request-handling workload.
func BenchmarkTagIndex_Has_Parallel(b *testing.B) {
	ti := newTagIndex()
	for i := range 10_000 {
		ti.add(fmt.Sprintf("key:%d", i), []string{fmt.Sprintf("tag:%d", i%100)})
	}
	target := "key:5000"
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ti.has(target)
		}
	})
}

// BenchmarkTagIndex_Add measures the cost of registering a key under a varying
// number of tags. Both the forward and reverse indexes are updated.
func BenchmarkTagIndex_Add(b *testing.B) {
	for _, tc := range []int{1, 5, 10} {
		b.Run(fmt.Sprintf("tags=%d", tc), func(b *testing.B) {
			ti := newTagIndex()
			tags := make([]string, tc)
			for i := range tc {
				tags[i] = fmt.Sprintf("tag:%d", i)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := range b.N {
				ti.add(fmt.Sprintf("key:%d", i), tags)
			}
		})
	}
}

// BenchmarkTagIndex_Invalidate measures invalidation of a single tag that
// covers a varying number of keys. Setup (index population) runs outside the
// timed window; only the invalidate call itself is measured.
func BenchmarkTagIndex_Invalidate(b *testing.B) {
	for _, kpt := range []int{10, 100, 1_000} {
		b.Run(fmt.Sprintf("keysPerTag=%d", kpt), func(b *testing.B) {
			// Pre-generate keys and per-key tag slices so setup allocations
			// are not charged to the timed portion.
			keys := make([]string, kpt)
			tags := make([][]string, kpt)
			for j := range kpt {
				keys[j] = fmt.Sprintf("key:%d", j)
				tags[j] = []string{"target", fmt.Sprintf("other:%d", j)}
			}
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				ti := newTagIndex()
				for j := range kpt {
					ti.add(keys[j], tags[j])
				}
				b.StartTimer()

				ti.invalidate([]string{"target"})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// distributedTagStore benchmarks
// ---------------------------------------------------------------------------

// BenchmarkDistributedTagStore_Has measures the distributed store's has() path.
// It delegates directly to the local tagIndex, so the cost should be identical
// to BenchmarkTagIndex_Has; this benchmark confirms no overhead is added by the
// distributed wrapper.
func BenchmarkDistributedTagStore_Has(b *testing.B) {
	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)
	for i := range 10_000 {
		d.add(fmt.Sprintf("key:%d", i), []string{fmt.Sprintf("tag:%d", i%10)})
	}
	target := "key:5000"
	b.ResetTimer()
	for range b.N {
		d.has(target)
	}
}

// BenchmarkDistributedTagStore_Has_Parallel confirms the distributed has() path
// remains contention-free under concurrent readers.
func BenchmarkDistributedTagStore_Has_Parallel(b *testing.B) {
	store := memory.New()
	defer store.Close()
	d := newDistributedTagStore(store, 5*time.Minute)
	for i := range 10_000 {
		d.add(fmt.Sprintf("key:%d", i), []string{fmt.Sprintf("tag:%d", i%10)})
	}
	target := "key:5000"
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			d.has(target)
		}
	})
}

// BenchmarkDistributedTagStore_Add measures the cost of add(), which writes to
// both the local index and the shared backend. Each add performs one read-modify-
// write cycle per tag on the forward index plus one on the reverse index.
func BenchmarkDistributedTagStore_Add(b *testing.B) {
	for _, tc := range []int{1, 5, 10} {
		b.Run(fmt.Sprintf("tags=%d", tc), func(b *testing.B) {
			store := memory.New()
			defer store.Close()
			d := newDistributedTagStore(store, 5*time.Minute)
			tags := make([]string, tc)
			for i := range tc {
				tags[i] = fmt.Sprintf("tag:%d", i)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := range b.N {
				d.add(fmt.Sprintf("key:%d", i), tags)
			}
		})
	}
}

// BenchmarkDistributedTagStore_Invalidate measures invalidation against the
// shared backend. Each iteration resets the shared storage and re-populates
// it so that invalidate() always operates on a freshly populated index. Only
// the invalidate call is timed.
func BenchmarkDistributedTagStore_Invalidate(b *testing.B) {
	for _, kpt := range []int{10, 100, 1_000} {
		b.Run(fmt.Sprintf("keysPerTag=%d", kpt), func(b *testing.B) {
			store := memory.New()
			defer store.Close()
			keys := make([]string, kpt)
			tags := make([][]string, kpt)
			for j := range kpt {
				keys[j] = fmt.Sprintf("key:%d", j)
				tags[j] = []string{"target", fmt.Sprintf("other:%d", j)}
			}
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				_ = store.Reset()
				d := newDistributedTagStore(store, 5*time.Minute)
				for j := range kpt {
					d.add(keys[j], tags[j])
				}
				b.StartTimer()

				d.invalidate([]string{"target"})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Encoding benchmarks
// ---------------------------------------------------------------------------

// BenchmarkEncodeStringSet measures the serialization cost for the binary
// format used by distributedTagStore to persist sets in shared storage.
// This is on the hot path of every shared-storage read and write.
func BenchmarkEncodeStringSet(b *testing.B) {
	for _, n := range []int{1, 5, 10, 50} {
		b.Run(fmt.Sprintf("strings=%d", n), func(b *testing.B) {
			ss := make([]string, n)
			for i := range n {
				ss[i] = fmt.Sprintf("tag:%d", i)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				encodeStringSet(ss)
			}
		})
	}
}

// BenchmarkDecodeStringSet measures the deserialization cost; called once per
// shared-storage read.
func BenchmarkDecodeStringSet(b *testing.B) {
	for _, n := range []int{1, 5, 10, 50} {
		b.Run(fmt.Sprintf("strings=%d", n), func(b *testing.B) {
			ss := make([]string, n)
			for i := range n {
				ss[i] = fmt.Sprintf("tag:%d", i)
			}
			data := encodeStringSet(ss)
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				decodeStringSet(data)
			}
		})
	}
}
