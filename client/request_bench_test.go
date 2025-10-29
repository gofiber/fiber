package client

import (
	"runtime"
	"runtime/metrics"
	"strconv"
	"testing"
)

// BenchmarkRequestHeapScan measures how much heap memory the GC needs to scan
// when a batch of requests is created and released.
func BenchmarkRequestHeapScan(b *testing.B) {
	samples := []metrics.Sample{
		{Name: "/gc/scan/heap:bytes"},
		{Name: "/gc/scan/total:bytes"},
	}

	b.ReportAllocs()
	b.StopTimer()
	b.ResetTimer()

	const batchSize = 512
	var totalScanHeap, totalScanAll uint64
	for i := 0; i < b.N; i++ {
		reqs := make([]*Request, batchSize)
		// revive:disable-next-line:call-to-gc // ensure consistent heap state before measuring scan metrics
		runtime.GC()
		metrics.Read(samples)
		startScanHeap := samples[0].Value.Uint64()
		startScanAll := samples[1].Value.Uint64()

		b.StartTimer()
		for j := range reqs {
			req := AcquireRequest()
			req.SetHeader("X-Benchmark", "value")
			req.SetCookie("session", strconv.Itoa(j))
			req.SetPathParam("id", strconv.Itoa(j))
			req.SetParam("page", strconv.Itoa(j))
			reqs[j] = req
		}
		b.StopTimer()

		// revive:disable-next-line:call-to-gc // force GC to capture post-benchmark scan metrics
		runtime.GC()
		metrics.Read(samples)
		totalScanHeap += samples[0].Value.Uint64() - startScanHeap
		totalScanAll += samples[1].Value.Uint64() - startScanAll

		for _, req := range reqs {
			ReleaseRequest(req)
		}
	}

	if b.N > 0 {
		b.ReportMetric(float64(totalScanHeap)/float64(b.N), "scan-bytes-heap/op")
		b.ReportMetric(float64(totalScanAll)/float64(b.N), "scan-bytes-total/op")
	}
}
