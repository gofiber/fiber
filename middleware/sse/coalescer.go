package sse

import (
	"sync"
	"time"
)

// coalescer buffers P1 (batched) and P2 (coalesced) events per connection.
// The hub's flush ticker drains these buffers periodically.
type coalescer struct {
	// coalesced holds P2 events keyed by CoalesceKey — only the latest per key survives.
	coalesced map[string]MarshaledEvent

	// batched holds P1 events in insertion order — all are sent on flush.
	batched []MarshaledEvent

	// coalescedOrder preserves first-seen order of coalesce keys for deterministic output.
	coalescedOrder []string

	mu sync.Mutex

	// flushInterval is the target flush cadence (informational).
	flushInterval time.Duration
}

// newCoalescer creates a coalescer with the given flush interval hint.
func newCoalescer(flushInterval time.Duration) *coalescer {
	return &coalescer{
		coalesced:     make(map[string]MarshaledEvent),
		batched:       make([]MarshaledEvent, 0, 16),
		flushInterval: flushInterval,
	}
}

// addBatched appends a P1 event to the batch buffer.
func (c *coalescer) addBatched(me MarshaledEvent) { //nolint:gocritic // hugeParam: value semantics match flush() return type
	c.mu.Lock()
	c.batched = append(c.batched, me)
	c.mu.Unlock()
}

// addCoalesced upserts a P2 event by its coalesce key. If the key already
// exists, the previous event is overwritten (last-writer-wins).
func (c *coalescer) addCoalesced(key string, me MarshaledEvent) { //nolint:gocritic // hugeParam: value semantics match flush() return type
	c.mu.Lock()
	if _, exists := c.coalesced[key]; !exists {
		c.coalescedOrder = append(c.coalescedOrder, key)
	}
	c.coalesced[key] = me
	c.mu.Unlock()
}

// flush drains both buffers and returns the events to send.
func (c *coalescer) flush() []MarshaledEvent {
	c.mu.Lock()
	defer c.mu.Unlock()

	batchLen := len(c.batched)
	coalLen := len(c.coalescedOrder)

	if batchLen == 0 && coalLen == 0 {
		return nil
	}

	result := make([]MarshaledEvent, 0, batchLen+coalLen)

	if batchLen > 0 {
		result = append(result, c.batched...)
		c.batched = c.batched[:0]
	}

	if coalLen > 0 {
		for _, key := range c.coalescedOrder {
			result = append(result, c.coalesced[key])
		}
		c.coalesced = make(map[string]MarshaledEvent, coalLen)
		c.coalescedOrder = c.coalescedOrder[:0]
	}

	return result
}

// pending returns the total number of buffered events.
func (c *coalescer) pending() int {
	c.mu.Lock()
	n := len(c.batched) + len(c.coalescedOrder)
	c.mu.Unlock()
	return n
}
