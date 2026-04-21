package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valyala/bytebufferpool"
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
	return "evt_" + strconv.FormatUint(globalEventID.Add(1), 10)
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
	TTL time.Duration
	// Retry is the reconnection hint (milliseconds) sent to clients. Zero
	// or negative values are omitted from the wire frame — per the SSE
	// spec `retry: 0` instructs clients to reconnect immediately, which
	// could trigger reconnect storms, so only strictly positive values
	// are emitted.
	Retry int
}

// sanitizeSSEField strips carriage returns and newlines from SSE control
// fields (id, event) to prevent SSE injection attacks. An attacker-controlled
// value containing \r or \n could break SSE framing and inject fake events.
func sanitizeSSEField(s string) string {
	return strings.NewReplacer("\r\n", "", "\r", "", "\n", "").Replace(s)
}

// normalizeSSEDataTerminators is used on the data field to convert any CR or
// CRLF sequence into LF before we split on line boundaries. The HTML SSE spec
// treats all three as valid line terminators, so we must emit one "data:" per
// logical line regardless of which terminator the caller used.
// Order matters: CRLF must be replaced first so we don't double-split.
var normalizeSSEDataTerminators = strings.NewReplacer("\r\n", "\n", "\r", "\n")

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
	default:
		// All other types flow through json.Marshal. The previous explicit
		// json.Marshaler branch panicked on typed-nil pointers (e.g.
		// `var p *Foo = nil` where *Foo has a MarshalJSON that dereferences
		// the receiver) because the type switch matches the interface.
		// json.Marshal handles typed-nil safely — it checks before invoking
		// the method and emits `null` for nil pointers.
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
// Events specification. It assembles the frame in a pooled buffer so the
// hot path performs a single Write syscall with zero fmt allocations.
func (me *MarshaledEvent) WriteTo(w io.Writer) (int64, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	// Sanitize control-sequence fields at the write boundary — not just in
	// marshalEvent — so external Replayer implementations returning raw
	// MarshaledEvent values can't inject extra SSE fields via embedded
	// \r/\n in ID or Type. Defense in depth: WriteTo is the last line
	// between an event and the client.
	if id := sanitizeSSEField(me.ID); id != "" {
		buf.WriteString("id: ")
		buf.WriteString(id)
		buf.WriteByte('\n')
	}
	if evtType := sanitizeSSEField(me.Type); evtType != "" {
		buf.WriteString("event: ")
		buf.WriteString(evtType)
		buf.WriteByte('\n')
	}
	// Retry must be strictly positive to be emitted. Per the SSE spec a
	// `retry: 0` directive tells clients to reconnect immediately, which can
	// trigger reconnect storms if a Replayer implementation accidentally
	// constructs MarshaledEvent without setting Retry (its zero value is 0).
	// Treating 0 as "unset" matches the internal marshalEvent default.
	if me.Retry > 0 {
		buf.WriteString("retry: ")
		buf.WriteString(strconv.Itoa(me.Retry))
		buf.WriteByte('\n')
	}

	// Normalise CR and CRLF to LF so a caller-supplied "\r" or "\r\n"
	// produces one data line per logical line rather than a single line
	// containing raw control characters (the HTML SSE parser treats all
	// three as line terminators and would mis-frame the client).
	// strings.SplitSeq("", "\n") yields "", correctly writing "data: \n"
	// for empty data.
	data := normalizeSSEDataTerminators.Replace(me.Data)
	for line := range strings.SplitSeq(data, "\n") {
		buf.WriteString("data: ")
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')

	n, err := w.Write(buf.B)
	if err != nil {
		return int64(n), fmt.Errorf("sse: write frame: %w", err)
	}
	return int64(n), nil
}

// writeComment writes an SSE comment line.
func writeComment(w io.Writer, text string) error {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	buf.WriteString(": ")
	buf.WriteString(text)
	buf.WriteString("\n\n")
	if _, err := w.Write(buf.B); err != nil {
		return fmt.Errorf("sse: write comment: %w", err)
	}
	return nil
}

// writeRetry writes the retry directive. Non-positive ms values are
// silently skipped — per the SSE spec `retry: 0` tells clients to
// reconnect immediately, matching the MarshaledEvent.WriteTo semantics.
func writeRetry(w io.Writer, ms int) error {
	if ms <= 0 {
		return nil
	}
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)
	buf.WriteString("retry: ")
	buf.WriteString(strconv.Itoa(ms))
	buf.WriteString("\n\n")
	if _, err := w.Write(buf.B); err != nil {
		return fmt.Errorf("sse: write retry: %w", err)
	}
	return nil
}
