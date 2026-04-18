package sse

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// MetricsSnapshot is a detailed point-in-time view of the hub for monitoring.
type MetricsSnapshot struct {
	ConnectionsByTopic  map[string]int   `json:"connections_by_topic"`
	EventsByType        map[string]int64 `json:"events_by_type"`
	Timestamp           string           `json:"timestamp"`
	Connections         []ConnectionInfo `json:"connections,omitempty"`
	EventsPublished     int64            `json:"events_published"`
	EventsDropped       int64            `json:"events_dropped"`
	AvgBufferSaturation float64          `json:"avg_buffer_saturation"`
	MaxBufferSaturation float64          `json:"max_buffer_saturation"`
	ActiveConnections   int              `json:"active_connections"`
	PausedConnections   int              `json:"paused_connections"`
	TotalPendingEvents  int              `json:"total_pending_events"`
}

// ConnectionInfo is per-connection detail for the metrics snapshot.
type ConnectionInfo struct {
	Metadata        map[string]string `json:"metadata"`
	ID              string            `json:"id"`
	CreatedAt       string            `json:"created_at"`
	Uptime          string            `json:"uptime"`
	LastEventID     string            `json:"last_event_id"`
	Topics          []string          `json:"topics"`
	MessagesSent    int64             `json:"messages_sent"`
	MessagesDropped int64             `json:"messages_dropped"`
	BufferUsage     int               `json:"buffer_usage"`
	BufferCapacity  int               `json:"buffer_capacity"`
	Paused          bool              `json:"paused"`
}

// Metrics returns a detailed snapshot of the hub for monitoring dashboards.
func (h *Hub) Metrics(includeConnections bool) MetricsSnapshot { //nolint:revive // flag-parameter: public API toggle
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	snap := MetricsSnapshot{
		Timestamp:          now.Format(time.RFC3339),
		ActiveConnections:  len(h.connections),
		ConnectionsByTopic: make(map[string]int, len(h.topicIndex)),
		EventsPublished:    h.metrics.eventsPublished.Load(),
		EventsDropped:      h.metrics.eventsDropped.Load(),
	}

	for topic, conns := range h.topicIndex {
		snap.ConnectionsByTopic[topic] = len(conns)
	}

	snap.EventsByType = h.metrics.snapshotEventsByType()

	var totalSat float64
	var maxSat float64
	for _, conn := range h.connections {
		if conn.paused.Load() {
			snap.PausedConnections++
		}

		pending := conn.coalescer.pending()
		snap.TotalPendingEvents += pending

		bufCap := cap(conn.send)
		sat := float64(0)
		if bufCap > 0 {
			sat = float64(len(conn.send)) / float64(bufCap)
		}
		totalSat += sat
		if sat > maxSat {
			maxSat = sat
		}

		if includeConnections {
			lastID, _ := conn.LastEventID.Load().(string) //nolint:errcheck // type assertion on atomic.Value
			snap.Connections = append(snap.Connections, ConnectionInfo{
				ID:              conn.ID,
				Topics:          conn.Topics,
				Metadata:        conn.Metadata,
				CreatedAt:       conn.CreatedAt.Format(time.RFC3339),
				Uptime:          now.Sub(conn.CreatedAt).Round(time.Second).String(),
				MessagesSent:    conn.MessagesSent.Load(),
				MessagesDropped: conn.MessagesDropped.Load(),
				LastEventID:     lastID,
				BufferUsage:     len(conn.send),
				BufferCapacity:  cap(conn.send),
				Paused:          conn.paused.Load(),
			})
		}
	}

	if len(h.connections) > 0 {
		snap.AvgBufferSaturation = totalSat / float64(len(h.connections))
	}
	snap.MaxBufferSaturation = maxSat

	return snap
}

// MetricsHandler returns a Fiber handler that serves the metrics snapshot
// as JSON. Mount it on an admin route:
//
//	app.Get("/admin/sse/metrics", hub.MetricsHandler())
func (h *Hub) MetricsHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		includeConns := c.Query("connections") == "true"
		snap := h.Metrics(includeConns)
		return c.JSON(snap)
	}
}

// PrometheusHandler returns a Fiber handler that serves Prometheus-formatted
// metrics. Mount on your /metrics endpoint:
//
//	app.Get("/metrics/sse", hub.PrometheusHandler())
func (h *Hub) PrometheusHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		snap := h.Metrics(false)
		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		lines := []byte("")
		lines = appendProm(lines, "sse_connections_active", "", float64(snap.ActiveConnections))
		lines = appendProm(lines, "sse_connections_paused", "", float64(snap.PausedConnections))
		lines = appendProm(lines, "sse_events_published_total", "", float64(snap.EventsPublished))
		lines = appendProm(lines, "sse_events_dropped_total", "", float64(snap.EventsDropped))
		lines = appendProm(lines, "sse_pending_events", "", float64(snap.TotalPendingEvents))
		lines = appendProm(lines, "sse_buffer_saturation_avg", "", snap.AvgBufferSaturation)
		lines = appendProm(lines, "sse_buffer_saturation_max", "", snap.MaxBufferSaturation)

		for topic, count := range snap.ConnectionsByTopic {
			lines = appendProm(lines, "sse_connections_by_topic", `topic="`+escapePromLabelValue(topic)+`"`, float64(count))
		}

		for eventType, count := range snap.EventsByType {
			lines = appendProm(lines, "sse_events_by_type_total", `type="`+escapePromLabelValue(eventType)+`"`, float64(count))
		}

		return c.Send(lines)
	}
}

func appendProm(buf []byte, name, labels string, value float64) []byte {
	if labels != "" {
		return append(buf, []byte(name+"{"+labels+"} "+formatFloat(value)+"\n")...)
	}
	return append(buf, []byte(name+" "+formatFloat(value)+"\n")...)
}

// escapePromLabelValue escapes backslashes, double quotes, and newlines in
// Prometheus label values per the exposition format spec.
func escapePromLabelValue(s string) string {
	var needsEscape bool
	for _, c := range s {
		if c == '\\' || c == '"' || c == '\n' {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, c := range s {
		switch c {
		case '\\':
			b.WriteString(`\\`) //nolint:errcheck // strings.Builder.WriteString never fails
		case '"':
			b.WriteString(`\"`) //nolint:errcheck // strings.Builder.WriteString never fails
		case '\n':
			b.WriteString(`\n`) //nolint:errcheck // strings.Builder.WriteString never fails
		default:
			b.WriteRune(c) //nolint:errcheck // strings.Builder.WriteRune never fails
		}
	}
	return b.String()
}

func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 6, 64)
}
