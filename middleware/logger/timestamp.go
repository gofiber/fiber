package logger

import (
	"sync"
	"sync/atomic"
	"time"
)

type timestampKey struct {
	format   string
	timeZone string
	interval time.Duration
}

var (
	tsMu    sync.RWMutex
	tsCache = map[timestampKey]*atomic.Value{}
)

func sharedTimestamp(format string, location *time.Location, interval time.Duration) *atomic.Value {
	key := timestampKey{
		format:   format,
		timeZone: location.String(),
		interval: interval,
	}

	tsMu.RLock()
	value, ok := tsCache[key]
	tsMu.RUnlock()
	if ok {
		return value
	}

	tsMu.Lock()
	defer tsMu.Unlock()

	if value, ok = tsCache[key]; ok {
		return value
	}

	value = &atomic.Value{}
	value.Store(time.Now().In(location).Format(format))
	tsCache[key] = value

	go func() {
		for {
			time.Sleep(interval)
			value.Store(time.Now().In(location).Format(format))
		}
	}()

	return value
}
