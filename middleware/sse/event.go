package sse

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/gofiber/utils/v2/simd"
)

var errInvalidField = errors.New("field must not contain CR or LF")

// Event defines a single Server-Sent Event frame.
type Event struct {
	// Data is written as one or more data fields. Strings and byte slices are
	// written as-is; other values are JSON encoded. Do not modify byte slices
	// or raw JSON until Event returns.
	Data any

	// ID sets the SSE id field.
	ID string

	// Name sets the SSE event field.
	Name string

	// Retry sets the SSE retry field for this event.
	Retry time.Duration
}

// writeEvent encodes the payload and validates the ID and name before writing any fields.
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

	if id != "" {
		writeEventField(w, "id: ", id)
	}
	if name != "" {
		writeEventField(w, "event: ", name)
	}
	if event.Retry > 0 {
		writeRetryField(w, event.Retry)
	}
	if data.hasData {
		writeEventData(w, data.text)
	}
	// The buffered writer returns the first write error on all subsequent writes.
	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf("sse: write event: %w", err)
	}
	return nil
}

// writeEventData emits CRLF, bare CR, and LF as line breaks without copying the payload.
func writeEventData(w *bufio.Writer, data string) {
	// AVX2 finds either delimiter in one scan. On other CPUs, the standard
	// library's single-byte search is faster for longer CR-free payloads.
	mixed := len(data) < simd.MinLen || simd.Accelerated() || strings.IndexByte(data, '\r') >= 0
	for {
		var i int
		if mixed {
			i = utils.IndexAny2(data, '\r', '\n')
		} else {
			i = strings.IndexByte(data, '\n')
		}
		if i < 0 {
			writeEventField(w, "data: ", data)
			return
		}
		writeEventField(w, "data: ", data[:i])
		cr := data[i] == '\r'
		data = data[i+1:]
		if cr && data != "" && data[0] == '\n' {
			data = data[1:]
		}
	}
}

// writeEventField writes one field and its line terminator.
//
//nolint:errcheck // The write of the final blank line reports any sticky bufio error.
func writeEventField(w *bufio.Writer, prefix, value string) {
	w.WriteString(prefix)
	w.WriteString(value)
	w.WriteByte('\n')
}

// writeRetryField writes the delay in whole milliseconds.
//
//nolint:errcheck // The write of the final blank line reports any sticky bufio error.
func writeRetryField(w *bufio.Writer, retry time.Duration) {
	w.WriteString("retry: ")
	const maxDigits = 20 // An int64 takes at most 20 characters.
	delay := retryMilliseconds(retry)
	if w.Available() >= maxDigits {
		w.Write(utils.AppendInt(w.AvailableBuffer(), delay))
	} else {
		var scratch [maxDigits]byte
		// Write individual bytes so the scratch stays on the stack if the writer flushes.
		for _, digit := range utils.AppendInt(scratch[:0], delay) {
			w.WriteByte(digit)
		}
	}
	w.WriteByte('\n')
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
	text    string
	hasData bool
}

// eventData borrows byte-backed payloads for the duration of the synchronous write.
func eventData(data any, jsonMarshal utils.JSONMarshal) (eventPayload, error) {
	switch value := data.(type) {
	case nil:
		return eventPayload{}, nil
	case string:
		return eventPayload{text: value, hasData: true}, nil
	case []byte:
		return eventPayload{text: utils.UnsafeString(value), hasData: true}, nil
	case json.RawMessage:
		return eventPayload{text: utils.UnsafeString(value), hasData: true}, nil
	default:
		encoded, err := jsonMarshal(value)
		if err != nil {
			return eventPayload{}, fmt.Errorf("sse: marshal data: %w", err)
		}
		return eventPayload{text: utils.UnsafeString(encoded), hasData: true}, nil
	}
}

func jsonMarshalOrDefault(jsonMarshal []utils.JSONMarshal) utils.JSONMarshal {
	if len(jsonMarshal) > 0 && jsonMarshal[0] != nil {
		return jsonMarshal[0]
	}
	return json.Marshal
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
