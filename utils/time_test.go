package utils

import (
	"testing"
	"time"
)

func checkTimeStamp(t testing.TB, expectedCurrent, actualCurrent uint32) {
	// test with some buffer in front and back of the expectedCurrent time -> because of the timing on the work machine
	AssertEqual(t, true, actualCurrent >= expectedCurrent-1 || actualCurrent <= expectedCurrent+1)
}

func Test_StartTimeStampUpdater(t *testing.T) {
	t.Parallel()

	StartTimeStampUpdater()

	now := uint32(time.Now().Unix())
	checkTimeStamp(t, now, Timestamp.Load())
	// one second later
	time.Sleep(1 * time.Second)
	checkTimeStamp(t, now+1, Timestamp.Load())
	// two seconds later
	time.Sleep(1 * time.Second)
	checkTimeStamp(t, now+2, Timestamp.Load())
}

func Benchmark_CalculateTimestamp(b *testing.B) {
	StartTimeStampUpdater()

	var res uint32
	b.Run("fiber", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			res = Timestamp.Load()
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
