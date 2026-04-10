package sse

// Replayer stores events for replay when a client reconnects with Last-Event-ID.
// Implement this interface to plug in any storage backend (Redis Streams,
// database, in-memory ring buffer, etc.).
type Replayer interface {
	// Store persists an event for potential future replay.
	Store(event MarshaledEvent, topics []string) error

	// Replay returns all events after lastEventID that match any of the
	// given topics, in chronological order. Returns nil if lastEventID
	// is unknown (caller should treat as a fresh connection).
	Replay(lastEventID string, topics []string) ([]MarshaledEvent, error)
}
