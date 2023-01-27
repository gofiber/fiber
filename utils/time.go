package utils

import (
	"sync"
	"sync/atomic"
	"time"
)

//nolint:gochecknoglobals // TODO: Do not use global vars here
var (
	timestampTimer sync.Once
	// Timestamp please start the timer function before you use this value
	// please load the value with atomic `atomic.LoadUint32(&utils.Timestamp)`
	Timestamp uint32
)

// StartTimeStampUpdater starts a concurrent function which stores the timestamp to an atomic value per second,
// which is much better for performance than determining it at runtime each time
func StartTimeStampUpdater() {
	timestampTimer.Do(func() {
		// set initial value
		atomic.StoreUint32(&Timestamp, uint32(time.Now().Unix()))
		go func(sleep time.Duration) {
			ticker := time.NewTicker(sleep)
			defer ticker.Stop()

			for t := range ticker.C {
				// update timestamp
				atomic.StoreUint32(&Timestamp, uint32(t.Unix()))
			}
		}(1 * time.Second) // duration
	})
}
