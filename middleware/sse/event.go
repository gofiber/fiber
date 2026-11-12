package sse

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
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

	if err := writeEventFrame(w, data, id, name, event.Retry); err != nil {
		return fmt.Errorf("sse: write event: %w", err)
	}
	return nil
}

// writeEventFrame reuses the writer buffer and writes fields directly as it fills.
func writeEventFrame(w *bufio.Writer, data eventPayload, id, name string, retry time.Duration) error {
	frame := w.AvailableBuffer()
	var err error
	if id != "" {
		frame, err = appendEventField(w, frame, "id: ", id)
		if err != nil {
			return err
		}
	}
	if name != "" {
		frame, err = appendEventField(w, frame, "event: ", name)
		if err != nil {
			return err
		}
	}
	if retry > 0 {
		frame, err = appendEventRetry(w, frame, retry)
		if err != nil {
			return err
		}
	}
	if data.hasData {
		if data.raw != nil {
			frame, err = appendEventData(w, frame, data.raw)
		} else {
			frame, err = appendEventData(w, frame, data.text)
		}
		if err != nil {
			return err
		}
	}
	return finishEventFrame(w, frame)
}

// appendEventData emits CRLF, bare CR, and LF as line breaks without copying the payload.
func appendEventData[T string | []byte](w *bufio.Writer, frame []byte, data T) ([]byte, error) {
	// For longer CR-free data, the standard library's single-byte search is faster.
	mixed := len(data) <= 32 || eventIndexByte(data, '\r') >= 0
	for {
		var i int
		if mixed {
			i = utils.IndexAny2(data, '\r', '\n')
		} else {
			i = eventIndexByte(data, '\n')
		}
		if i < 0 {
			return appendEventField(w, frame, "data: ", data)
		}
		var err error
		frame, err = appendEventField(w, frame, "data: ", data[:i])
		if err != nil {
			return nil, err
		}
		cr := data[i] == '\r'
		data = data[i+1:]
		if cr && len(data) > 0 && data[0] == '\n' {
			data = data[1:]
		}
	}
}

// eventIndexByte searches either payload representation without converting it.
func eventIndexByte[T string | []byte](data T, c byte) int {
	switch v := any(data).(type) {
	case string:
		return strings.IndexByte(v, c)
	case []byte:
		return bytes.IndexByte(v, c)
	}
	return -1
}

// appendEventField appends into the writer's available buffer or sends the field through the writer.
func appendEventField[T string | []byte](w *bufio.Writer, frame []byte, prefix string, value T) ([]byte, error) {
	if len(value) <= cap(frame)-len(frame)-len(prefix)-1 {
		frame = append(frame, prefix...)
		switch v := any(value).(type) {
		case string:
			frame = append(frame, v...)
		case []byte:
			frame = append(frame, v...)
		}
		return append(frame, '\n'), nil
	}
	return writeEventField(w, frame, prefix, value)
}

// writeEventField commits pending bytes before sending a field that does not fit.
//
//nolint:wrapcheck // writeEvent adds the frame context to writer errors.
func writeEventField[T string | []byte](w *bufio.Writer, frame []byte, prefix string, value T) ([]byte, error) {
	if _, err := w.Write(frame); err != nil {
		return nil, err
	}
	if _, err := w.WriteString(prefix); err != nil {
		return nil, err
	}
	var err error
	switch v := any(value).(type) {
	case string:
		_, err = w.WriteString(v)
	case []byte:
		_, err = w.Write(v)
	}
	if err != nil {
		return nil, err
	}
	if err := w.WriteByte('\n'); err != nil {
		return nil, err
	}
	return w.AvailableBuffer(), nil
}

// appendEventRetry uses the writer's buffer when it has room for a complete retry field.
//
//nolint:wrapcheck // writeEvent and Stream.Retry add the frame context to writer errors.
func appendEventRetry(w *bufio.Writer, frame []byte, retry time.Duration) ([]byte, error) {
	const maxRetryFieldSize = len("retry: \n") + 20 // An int64 takes at most 20 characters.
	if cap(frame)-len(frame) >= maxRetryFieldSize {
		return appendRetry(frame, retry), nil
	}

	var scratch [maxRetryFieldSize]byte
	line := appendRetry(scratch[:0], retry)
	if len(line) <= cap(frame)-len(frame) {
		return append(frame, line...), nil
	}
	if _, err := w.Write(frame); err != nil {
		return nil, err
	}
	for _, c := range line {
		if err := w.WriteByte(c); err != nil {
			return nil, err
		}
	}
	return w.AvailableBuffer(), nil
}

// appendRetry appends the retry field, including its line terminator.
func appendRetry(dst []byte, retry time.Duration) []byte {
	dst = append(dst, "retry: "...)
	dst = utils.AppendInt(dst, retryMilliseconds(retry))
	return append(dst, '\n')
}

// finishEventFrame writes any pending fields and the blank line ending the frame.
//
//nolint:wrapcheck // writeEvent and Stream.Retry add the frame context to writer errors.
func finishEventFrame(w *bufio.Writer, frame []byte) error {
	if len(frame) < cap(frame) {
		_, err := w.Write(append(frame, '\n'))
		return err
	}
	if _, err := w.Write(frame); err != nil {
		return err
	}
	return w.WriteByte('\n')
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
	raw     []byte
	hasData bool
}

func eventData(data any, jsonMarshal utils.JSONMarshal) (eventPayload, error) {
	switch value := data.(type) {
	case nil:
		return eventPayload{}, nil
	case string:
		return eventPayload{text: value, hasData: true}, nil
	case []byte:
		return eventPayload{raw: value, hasData: true}, nil
	case json.RawMessage:
		return eventPayload{raw: value, hasData: true}, nil
	default:
		encoded, err := jsonMarshal(value)
		if err != nil {
			return eventPayload{}, fmt.Errorf("sse: marshal data: %w", err)
		}
		return eventPayload{raw: encoded, hasData: true}, nil
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
