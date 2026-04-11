package sse

import (
	"sync"
	"time"
)

// Replayer stores events for replay when a client reconnects with Last-Event-ID.
// Implement this interface to use Redis Streams, a database, or any durable store.
type Replayer interface {
	// Store persists an event for potential future replay.
	Store(event MarshaledEvent, topics []string) error

	// Replay returns all events after lastEventID that match any of the given topics.
	Replay(lastEventID string, topics []string) ([]MarshaledEvent, error)
}

// replayEntry pairs an event with its topic set for filtering.
type replayEntry struct {
	timestamp time.Time
	topics    map[string]struct{}
	event     MarshaledEvent
}

// MemoryReplayer is an in-memory Replayer backed by a fixed-size circular buffer.
// Events older than TTL or exceeding MaxEvents are evicted. Once the buffer is
// full, new events overwrite the oldest entry with zero allocations.
//
// For production deployments with high event throughput, use a persistent
// replayer backed by Redis Streams or a database.
type MemoryReplayer struct {
	entries   []replayEntry
	mu        sync.RWMutex
	ttl       time.Duration
	head      int // write position (wraps around)
	count     int // number of valid entries
	maxEvents int
}

// MemoryReplayerConfig configures the in-memory replayer.
type MemoryReplayerConfig struct {
	// MaxEvents is the maximum number of events to retain (default: 1000).
	MaxEvents int

	// TTL is how long events are kept before eviction (default: 5m).
	TTL time.Duration
}

// NewMemoryReplayer creates an in-memory replayer.
func NewMemoryReplayer(cfg ...MemoryReplayerConfig) *MemoryReplayer {
	c := MemoryReplayerConfig{
		MaxEvents: 1000,
		TTL:       5 * time.Minute,
	}
	if len(cfg) > 0 {
		if cfg[0].MaxEvents > 0 {
			c.MaxEvents = cfg[0].MaxEvents
		}
		if cfg[0].TTL > 0 {
			c.TTL = cfg[0].TTL
		}
	}
	return &MemoryReplayer{
		entries:   make([]replayEntry, c.MaxEvents),
		maxEvents: c.MaxEvents,
		ttl:       c.TTL,
	}
}

// Store adds an event to the replay buffer. Once full, overwrites the
// oldest entry (O(1), zero allocations).
func (r *MemoryReplayer) Store(event MarshaledEvent, topics []string) error { //nolint:gocritic // hugeParam: matches Replayer interface, value semantics
	topicSet := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		topicSet[t] = struct{}{}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[r.head] = replayEntry{
		event:     event,
		topics:    topicSet,
		timestamp: time.Now(),
	}
	r.head = (r.head + 1) % r.maxEvents
	if r.count < r.maxEvents {
		r.count++
	}

	return nil
}

// Replay returns events after lastEventID matching the given topics.
func (r *MemoryReplayer) Replay(lastEventID string, topics []string) ([]MarshaledEvent, error) {
	if lastEventID == "" {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cutoff := time.Now().Add(-r.ttl)

	// Walk the ring buffer in chronological order to find lastEventID.
	start := (r.head - r.count + r.maxEvents) % r.maxEvents
	foundIdx := -1
	for i := range r.count {
		idx := (start + i) % r.maxEvents
		if r.entries[idx].event.ID == lastEventID {
			foundIdx = i + 1 // start from the NEXT entry
			break
		}
	}

	if foundIdx < 0 {
		return nil, nil
	}

	var result []MarshaledEvent
	for i := foundIdx; i < r.count; i++ {
		idx := (start + i) % r.maxEvents
		entry := r.entries[idx]

		if entry.timestamp.Before(cutoff) {
			continue
		}

		if matchesAnyTopicWithWildcards(topics, entry.topics) {
			result = append(result, entry.event)
		}
	}

	return result, nil
}

// matchesAnyTopicWithWildcards returns true if any subscription pattern
// matches any of the stored event topics.
func matchesAnyTopicWithWildcards(subscriptionPatterns []string, eventTopics map[string]struct{}) bool {
	for _, pattern := range subscriptionPatterns {
		for topic := range eventTopics {
			if topicMatch(pattern, topic) {
				return true
			}
		}
	}
	return false
}
