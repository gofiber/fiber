package sse

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

var errInvalidField = errors.New("field must not contain CR or LF")

// Event defines a single Server-Sent Event frame.
type Event struct {
	// Data is written as one or more data fields. Strings and byte slices are
	// written as-is; other values are JSON encoded.
	Data any

	// ID sets the SSE id field.
	ID string

	// Name sets the SSE event field.
	Name string

	// Retry sets the SSE retry field for this event.
	Retry time.Duration
}

func writeEvent(w *bufio.Writer, event Event, jsonMarshal ...utils.JSONMarshal) error {
	data, err := eventData(event.Data, jsonMarshalOrDefault(jsonMarshal))
	if err != nil {
		return err
	}

	id, name := event.ID, event.Name
	if id != "" {
		id, err = sanitizeField(id)
		if err != nil {
			return fmt.Errorf("sse: invalid id: %w", err)
		}
	}
	if name != "" {
		name, err = sanitizeField(name)
		if err != nil {
			return fmt.Errorf("sse: invalid event: %w", err)
		}
	}

	// Reuse the writer's space for frames that fit. Append grows the slice
	// for larger frames, and Write commits the complete frame either way.
	frame := w.AvailableBuffer()
	if id != "" {
		frame = appendField(frame, "id: ", id)
	}
	if name != "" {
		frame = appendField(frame, "event: ", name)
	}
	if event.Retry > 0 {
		frame = append(frame, "retry: "...)
		frame = utils.AppendInt(frame, retryMilliseconds(event.Retry))
		frame = append(frame, '\n')
	}
	if data.hasData {
		for line := range strings.SplitSeq(normalizeNewlines(data.data), "\n") {
			frame = appendField(frame, "data: ", line)
		}
	}
	frame = append(frame, '\n')
	if _, err := w.Write(frame); err != nil {
		return fmt.Errorf("sse: write event: %w", err)
	}
	return nil
}

func writeComment(w *bufio.Writer, comment string) error {
	comment = sanitizeComment(comment)
	if comment == "" {
		if _, err := w.WriteString(":\n\n"); err != nil {
			return fmt.Errorf("sse: write heartbeat: %w", err)
		}
		return nil
	}
	for line := range strings.SplitSeq(comment, "\n") {
		if _, err := fmt.Fprintf(w, ": %s\n", line); err != nil {
			return fmt.Errorf("sse: write comment: %w", err)
		}
	}
	if _, err := w.WriteString("\n"); err != nil {
		return fmt.Errorf("sse: finish comment: %w", err)
	}
	return nil
}

type eventPayload struct {
	data    string
	hasData bool
}

func eventData(data any, jsonMarshal utils.JSONMarshal) (eventPayload, error) {
	switch value := data.(type) {
	case nil:
		return eventPayload{}, nil
	case string:
		return eventPayload{data: value, hasData: true}, nil
	case []byte:
		return eventPayload{data: string(value), hasData: true}, nil
	case json.RawMessage:
		return eventPayload{data: string(value), hasData: true}, nil
	default:
		encoded, err := jsonMarshal(value)
		if err != nil {
			return eventPayload{}, fmt.Errorf("sse: marshal data: %w", err)
		}
		return eventPayload{data: string(encoded), hasData: true}, nil
	}
}

func jsonMarshalOrDefault(jsonMarshal []utils.JSONMarshal) utils.JSONMarshal {
	if len(jsonMarshal) > 0 && jsonMarshal[0] != nil {
		return jsonMarshal[0]
	}
	return json.Marshal
}

func appendField(dst []byte, prefix, value string) []byte {
	dst = append(dst, prefix...)
	if len(value) > cap(dst)-len(dst) {
		// Size the growth from the bytes already appended, not the full writer buffer.
		dst = slices.Clip(dst)
	}
	dst = append(dst, value...)
	return append(dst, '\n')
}

func sanitizeField(value string) (string, error) {
	if utils.IndexAny2(value, '\r', '\n') != -1 {
		return "", errInvalidField
	}
	return utils.Trim(value, ' '), nil
}

func sanitizeComment(value string) string {
	value = normalizeNewlines(value)
	lines := make([]string, 0, strings.Count(value, "\n")+1)
	for line := range strings.SplitSeq(value, "\n") {
		lines = append(lines, utils.Trim(line, ' '))
	}
	return strings.Join(lines, "\n")
}

func normalizeNewlines(value string) string {
	// Fast path: nothing to normalize without a carriage return.
	if !strings.ContainsRune(value, '\r') {
		return value
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

// retryMilliseconds converts a delay to whole milliseconds, rounding up so it never becomes "retry: 0".
func retryMilliseconds(d time.Duration) int64 {
	// Divide before rounding up: adding a millisecond first overflows for a
	// duration near the top of the range and wraps to a negative count.
	ms := d / time.Millisecond
	if d%time.Millisecond != 0 {
		ms++
	}
	return int64(ms)
}
