package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
)

// Priority controls how an event is delivered to clients.
type Priority int

const (
	// PriorityInstant bypasses all buffering — the event is written to the
	// client connection immediately. Use for errors, auth revocations,
	// force-refresh commands, and chat messages.
	PriorityInstant Priority = 0

	// PriorityBatched collects events in a time window (FlushInterval) and
	// sends them all at once. Use for status changes, media updates.
	PriorityBatched Priority = 1

	// PriorityCoalesced uses last-writer-wins per CoalesceKey. Multiple
	// events with the same key within a flush window are merged — only the
	// latest is sent. Use for progress bars, live counters, typing indicators.
	PriorityCoalesced Priority = 2
)

// Event represents a single SSE event to be published through the hub.
type Event struct {
	CreatedAt   time.Time
	Data        any
	Group       map[string]string
	Type        string
	ID          string
	CoalesceKey string
	Topics      []string
	TTL         time.Duration
	Priority    Priority
}

// globalEventID is an auto-incrementing counter for event IDs.
var globalEventID atomic.Uint64

// nextEventID returns a monotonically increasing event ID string.
func nextEventID() string {
	return fmt.Sprintf("evt_%d", globalEventID.Add(1))
}

// MarshaledEvent is the wire-ready representation of an SSE event.
// External Replayer implementations receive and return this type.
type MarshaledEvent struct {
	// CreatedAt is the timestamp of the source Event (zero if unset).
	CreatedAt time.Time
	ID        string
	Type      string
	Data      string
	// TTL is the maximum age for this event. Zero means no expiry.
	TTL   time.Duration
	Retry int // -1 means omit
}

// sanitizeSSEField strips carriage returns and newlines from SSE control
// fields (id, event) to prevent SSE injection attacks. An attacker-controlled
// value containing \r or \n could break SSE framing and inject fake events.
func sanitizeSSEField(s string) string {
	return strings.NewReplacer("\r\n", "", "\r", "", "\n", "").Replace(s)
}

// marshalEvent converts an Event into wire-ready format.
func marshalEvent(e *Event) MarshaledEvent {
	me := MarshaledEvent{
		ID:        sanitizeSSEField(e.ID),
		Type:      sanitizeSSEField(e.Type),
		CreatedAt: e.CreatedAt,
		TTL:       e.TTL,
		Retry:     -1,
	}

	if me.ID == "" {
		me.ID = nextEventID()
	}

	switch v := e.Data.(type) {
	case nil:
		me.Data = ""
	case string:
		me.Data = v
	case []byte:
		me.Data = string(v)
	case json.Marshaler:
		b, err := v.MarshalJSON()
		if err != nil {
			errJSON, _ := json.Marshal(err.Error()) //nolint:errcheck,errchkjson // encoding a string never fails
			me.Data = fmt.Sprintf(`{"error":%s}`, string(errJSON))
		} else {
			me.Data = string(b)
		}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			errJSON, _ := json.Marshal(err.Error()) //nolint:errcheck,errchkjson // encoding a string never fails
			me.Data = fmt.Sprintf(`{"error":%s}`, string(errJSON))
		} else {
			me.Data = string(b)
		}
	}

	return me
}

// WriteTo writes the SSE-formatted event to w following the Server-Sent
// Events specification.
func (me *MarshaledEvent) WriteTo(w io.Writer) (int64, error) {
	var total int64

	if me.ID != "" {
		n, err := fmt.Fprintf(w, "id: %s\n", me.ID)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("sse: write id: %w", err)
		}
	}

	if me.Type != "" {
		n, err := fmt.Fprintf(w, "event: %s\n", me.Type)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("sse: write event: %w", err)
		}
	}

	if me.Retry >= 0 {
		n, err := fmt.Fprintf(w, "retry: %d\n", me.Retry)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("sse: write retry: %w", err)
		}
	}

	// strings.SplitSeq("", "\n") yields "", correctly writing "data: \n" for empty data.
	for line := range strings.SplitSeq(me.Data, "\n") {
		n, err := fmt.Fprintf(w, "data: %s\n", line)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("sse: write data: %w", err)
		}
	}

	n, err := fmt.Fprint(w, "\n")
	total += int64(n)
	if err != nil {
		return total, fmt.Errorf("sse: write terminator: %w", err)
	}
	return total, nil
}

// writeComment writes an SSE comment line.
func writeComment(w io.Writer, text string) error {
	_, err := fmt.Fprintf(w, ": %s\n\n", text)
	if err != nil {
		return fmt.Errorf("sse: write comment: %w", err)
	}
	return nil
}

// writeRetry writes the retry directive.
func writeRetry(w io.Writer, ms int) error {
	_, err := fmt.Fprintf(w, "retry: %d\n\n", ms)
	if err != nil {
		return fmt.Errorf("sse: write retry: %w", err)
	}
	return nil
}
