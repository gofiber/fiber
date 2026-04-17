package sse

import (
	"sync"
	"time"
)

// dispatcher is a per-connection two-lane queue feeding the write loop.
//
// Lane 1 (events): FIFO buffer of batched P1 events. Each AddEvent call
// appends a distinct event; all are emitted on flush in insertion order.
//
// Lane 2 (state): last-writer-wins map keyed by CoalesceKey for P2 events.
// Duplicate keys overwrite the prior value, so only the latest state is
// delivered. First-seen order is preserved across keys for deterministic
// output.
//
// This split mirrors SSE usage patterns: events are discrete happenings
// (notifications, log lines, messages) that must all reach the client,
// while state is the current value of something (progress %, online
// users count, cursor position) where only the newest snapshot matters.
type dispatcher struct {
	// state holds P2 events keyed by CoalesceKey.
	state map[string]MarshaledEvent

	// events holds P1 events in insertion order.
	events []MarshaledEvent

	// stateOrder preserves first-seen order of coalesce keys.
	stateOrder []string

	mu sync.Mutex

	// flushInterval is the target flush cadence (informational).
	flushInterval time.Duration
}

// newDispatcher creates a dispatcher with the given flush interval hint.
func newDispatcher(flushInterval time.Duration) *dispatcher {
	return &dispatcher{
		state:         make(map[string]MarshaledEvent),
		events:        make([]MarshaledEvent, 0, 16),
		flushInterval: flushInterval,
	}
}

// AddEvent appends a P1 event to the events lane. All added events are
// sent on the next flush in insertion order.
func (d *dispatcher) AddEvent(me MarshaledEvent) { //nolint:gocritic // hugeParam: value semantics match WriteTo() return type
	d.mu.Lock()
	d.events = append(d.events, me)
	d.mu.Unlock()
}

// AddState upserts a P2 state event keyed by CoalesceKey. If the key
// already has a pending value, the previous value is overwritten
// (last-writer-wins).
func (d *dispatcher) AddState(key string, me MarshaledEvent) { //nolint:gocritic // hugeParam: value semantics match WriteTo() return type
	d.mu.Lock()
	if _, exists := d.state[key]; !exists {
		d.stateOrder = append(d.stateOrder, key)
	}
	d.state[key] = me
	d.mu.Unlock()
}

// WriteTo drains both lanes and returns the events to write, in order:
// queued events first, then state values in first-seen key order.
func (d *dispatcher) WriteTo() []MarshaledEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	eventsLen := len(d.events)
	stateLen := len(d.stateOrder)

	if eventsLen == 0 && stateLen == 0 {
		return nil
	}

	result := make([]MarshaledEvent, 0, eventsLen+stateLen)

	if eventsLen > 0 {
		result = append(result, d.events...)
		d.events = d.events[:0]
	}

	if stateLen > 0 {
		for _, key := range d.stateOrder {
			result = append(result, d.state[key])
		}
		d.state = make(map[string]MarshaledEvent, stateLen)
		d.stateOrder = d.stateOrder[:0]
	}

	return result
}

// pending returns the total number of queued events and state updates.
func (d *dispatcher) pending() int {
	d.mu.Lock()
	n := len(d.events) + len(d.stateOrder)
	d.mu.Unlock()
	return n
}
