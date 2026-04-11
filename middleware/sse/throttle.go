package sse

import (
	"sync"
	"time"
)

// adaptiveThrottler monitors per-connection buffer saturation and adjusts
// the effective flush interval. Connections with high buffer usage get
// longer flush intervals (fewer sends), reducing backpressure.
type adaptiveThrottler struct {
	lastFlush    map[string]time.Time
	mu           sync.Mutex
	baseInterval time.Duration
	minInterval  time.Duration
	maxInterval  time.Duration
}

func newAdaptiveThrottler(baseInterval time.Duration) *adaptiveThrottler {
	minInt := max(baseInterval/4, 100*time.Millisecond)
	maxInt := min(baseInterval*4, 10*time.Second)
	return &adaptiveThrottler{
		lastFlush:    make(map[string]time.Time),
		baseInterval: baseInterval,
		minInterval:  minInt,
		maxInterval:  maxInt,
	}
}

// effectiveInterval calculates the flush interval for a connection based
// on its buffer saturation (0.0 = empty, 1.0 = full).
func (at *adaptiveThrottler) effectiveInterval(saturation float64) time.Duration {
	switch {
	case saturation > 0.8:
		return at.maxInterval
	case saturation > 0.5:
		return at.baseInterval * 2
	case saturation < 0.1:
		return at.minInterval
	default:
		return at.baseInterval
	}
}

// shouldFlush returns true if enough time has passed since the last flush.
func (at *adaptiveThrottler) shouldFlush(connID string, saturation float64) bool {
	at.mu.Lock()
	defer at.mu.Unlock()

	interval := at.effectiveInterval(saturation)
	last, ok := at.lastFlush[connID]
	if !ok {
		at.lastFlush[connID] = time.Now()
		return true
	}

	if time.Since(last) >= interval {
		at.lastFlush[connID] = time.Now()
		return true
	}
	return false
}

// remove cleans up tracking for a disconnected connection.
func (at *adaptiveThrottler) remove(connID string) {
	at.mu.Lock()
	delete(at.lastFlush, connID)
	at.mu.Unlock()
}

// cleanup removes stale entries older than the given cutoff.
func (at *adaptiveThrottler) cleanup(cutoff time.Time) {
	at.mu.Lock()
	defer at.mu.Unlock()
	for k, v := range at.lastFlush {
		if v.Before(cutoff) {
			delete(at.lastFlush, k)
		}
	}
}
