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
	// written as-is; other values are JSON encoded.
	Data any

	// ID sets the SSE id field.
	ID string

	// Name sets the SSE event field.
	Name string

	// Retry sets the SSE retry field for this event.
	Retry time.Duration
}

func writeEvent(w *bufio.Writer, event Event) error {
	var frame bytes.Buffer
	fw := bufio.NewWriter(&frame)

	if event.ID != "" {
		id, err := sanitizeField(event.ID)
		if err != nil {
			return fmt.Errorf("sse: invalid id: %w", err)
		}
		if _, err := fmt.Fprintf(fw, "id: %s\n", id); err != nil {
			return fmt.Errorf("sse: write id: %w", err)
		}
	}
	if event.Name != "" {
		name, err := sanitizeField(event.Name)
		if err != nil {
			return fmt.Errorf("sse: invalid event: %w", err)
		}
		if _, err := fmt.Fprintf(fw, "event: %s\n", name); err != nil {
			return fmt.Errorf("sse: write event: %w", err)
		}
	}
	if event.Retry > 0 {
		if _, err := fmt.Fprintf(fw, "retry: %d\n", event.Retry.Milliseconds()); err != nil {
			return fmt.Errorf("sse: write retry: %w", err)
		}
	}

	data, err := eventData(event.Data)
	if err != nil {
		return err
	}
	if data.hasData {
		if err := writeData(fw, data.data); err != nil {
			return err
		}
	}
	if _, err := fw.WriteString("\n"); err != nil {
		return fmt.Errorf("sse: finish event: %w", err)
	}
	if err := fw.Flush(); err != nil {
		return fmt.Errorf("sse: flush event frame: %w", err)
	}
	if _, err := w.Write(frame.Bytes()); err != nil {
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

func eventData(data any) (eventPayload, error) {
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
		encoded, err := json.Marshal(value)
		if err != nil {
			return eventPayload{}, fmt.Errorf("sse: marshal data: %w", err)
		}
		return eventPayload{data: string(encoded), hasData: true}, nil
	}
}

func writeData(w *bufio.Writer, data string) error {
	data = normalizeNewlines(data)
	for line := range strings.SplitSeq(data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return fmt.Errorf("sse: write data: %w", err)
		}
	}
	return nil
}

func sanitizeField(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
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
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}
