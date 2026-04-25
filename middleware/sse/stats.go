package sse

import (
	"sync"
	"sync/atomic"
)

// HubStats provides a snapshot of the hub's current state.
type HubStats struct {
	// ConnectionsByTopic maps each topic to its subscriber count.
	ConnectionsByTopic map[string]int `json:"connections_by_topic"`

	// EventsByType maps each SSE event type to its lifetime count.
	EventsByType map[string]int64 `json:"events_by_type"`

	// EventsPublished is the lifetime count of events published to the hub.
	EventsPublished int64 `json:"events_published"`

	// EventsDropped is the lifetime count of events dropped before delivery, for any reason.
	EventsDropped int64 `json:"events_dropped"`

	// ActiveConnections is the total number of open SSE connections.
	ActiveConnections int `json:"active_connections"`

	// TotalTopics is the number of unique topics with at least one subscriber.
	TotalTopics int `json:"total_topics"`
}

// hubMetrics tracks lifetime counters for the hub.
type hubMetrics struct {
	eventsByType    map[string]*atomic.Int64
	eventsByTypeMu  sync.RWMutex
	eventsPublished atomic.Int64
	eventsDropped   atomic.Int64
}

// trackEventType increments the counter for a specific event type.
func (m *hubMetrics) trackEventType(eventType string) {
	if eventType == "" {
		eventType = "message"
	}

	m.eventsByTypeMu.RLock()
	counter, ok := m.eventsByType[eventType]
	m.eventsByTypeMu.RUnlock()

	if ok {
		counter.Add(1)
		return
	}

	m.eventsByTypeMu.Lock()
	if counter, ok = m.eventsByType[eventType]; ok {
		m.eventsByTypeMu.Unlock()
		counter.Add(1)
		return
	}
	counter = &atomic.Int64{}
	counter.Add(1)
	m.eventsByType[eventType] = counter
	m.eventsByTypeMu.Unlock()
}

// snapshotEventsByType returns a copy of the per-event-type counters.
func (m *hubMetrics) snapshotEventsByType() map[string]int64 {
	m.eventsByTypeMu.RLock()
	defer m.eventsByTypeMu.RUnlock()

	result := make(map[string]int64, len(m.eventsByType))
	for k, v := range m.eventsByType {
		result[k] = v.Load()
	}
	return result
}
