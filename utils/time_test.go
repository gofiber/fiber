package utils

import (
	"sync/atomic"
	"testing"
	"time"
)

func checkTimeStamp(tb testing.TB, expectedCurrent, actualCurrent uint32) { //nolint:thelper // TODO: Verify if tb can be nil
	if tb != nil {
		tb.Helper()
	}
	// test with some buffer in front and back of the expectedCurrent time -> because of the timing on the work machine
	AssertEqual(tb, true, actualCurrent >= expectedCurrent-1 || actualCurrent <= expectedCurrent+1)
}

func Test_TimeStampUpdater(t *testing.T) {
	t.Parallel()

	StartTimeStampUpdater()

	now := uint32(time.Now().Unix())
	checkTimeStamp(t, now, atomic.LoadUint32(&Timestamp))
	// one second later
	time.Sleep(1 * time.Second)
	checkTimeStamp(t, now+1, atomic.LoadUint32(&Timestamp))
	// two seconds later
	time.Sleep(1 * time.Second)
	checkTimeStamp(t, now+2, atomic.LoadUint32(&Timestamp))
}

func Benchmark_CalculateTimestamp(b *testing.B) {
	StartTimeStampUpdater()

	var res uint32
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = atomic.LoadUint32(&Timestamp)
		}
		checkTimeStamp(b, uint32(time.Now().Unix()), res)
	})
	b.Run("default", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = uint32(time.Now().Unix())
		}
		checkTimeStamp(b, uint32(time.Now().Unix()), res)
	})
}
