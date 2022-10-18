package utils

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	timestampTimer sync.Once
	Timestamp      atomic.Uint32
)

func StartTimeStampUpdater() {
	timestampTimer.Do(func() {
		// set initial value
		Timestamp.Store(uint32(time.Now().Unix()))
		go func(sleep time.Duration) {
			ticker := time.NewTicker(sleep)
			defer ticker.Stop()
			for {
				select {
				case t := <-ticker.C:
					// update timestamp
					Timestamp.Store(uint32(t.Unix()))
				}
			}
		}(1 * time.Second) // duration
	})
}
