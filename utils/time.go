package utils

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	timestampTimer sync.Once
	Timestamp      uint32
)

func StartTimeStampUpdater() {
	timestampTimer.Do(func() {
		go func(sleep time.Duration) {
			ticker := time.NewTicker(sleep)
			defer ticker.Stop()
			for {
				select {
				case t := <-ticker.C:
					// update timestamp
					atomic.StoreUint32(&Timestamp, uint32(t.Unix()))
				}
			}
		}(1 * time.Second) // duration
	})
}
