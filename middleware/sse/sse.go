// Package sse provides Server-Sent Events middleware for Fiber.
//
// It is the only SSE implementation built natively for Fiber's
// fasthttp architecture — no net/http adapters, no broken disconnect
// detection.
//
// Features: event coalescing (last-writer-wins), three priority lanes
// (instant/batched/coalesced), NATS-style topic wildcards, adaptive
// per-connection throttling, connection groups (publish by metadata),
// built-in JWT and ticket auth helpers, Prometheus metrics, graceful
// Kubernetes-style drain, auto fan-out from Redis/NATS, and pluggable
// Last-Event-ID replay.
//
// Quick start:
//
//	handler, hub := sse.NewWithHub(sse.Config{
//	    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
//	        conn.Topics = []string{"notifications"}
//	        return nil
//	    },
//	})
//	app.Get("/events", handler)
//	hub.Publish(sse.Event{Type: "ping", Data: "hello", Topics: []string{"notifications"}})
package sse

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

// Hub is the central SSE event broker. It manages client connections,
// event routing, coalescing, and delivery. All methods are goroutine-safe.
type Hub struct {
	throttler     *adaptiveThrottler
	connections   map[string]*Connection
	topicIndex    map[string]map[string]struct{}
	wildcardConns map[string]struct{}
	register      chan *Connection
	unregister    chan *Connection
	events        chan Event
	shutdown      chan struct{}
	stopped       chan struct{}
	cfg           Config
	metrics       hubMetrics
	mu            sync.RWMutex
	shutdownOnce  sync.Once
	draining      atomic.Bool
}

// New creates a new SSE middleware handler. Use this when you don't need
// direct access to the Hub (e.g., simple streaming without Publish).
//
// For most use cases, prefer [NewWithHub] instead.
func New(config ...Config) fiber.Handler {
	handler, _ := NewWithHub(config...)
	return handler
}

// NewWithHub creates a new SSE middleware handler and returns it along
// with the Hub for publishing events. This is the primary entry point.
//
//	handler, hub := sse.NewWithHub(sse.Config{
//	    OnConnect: func(c fiber.Ctx, conn *sse.Connection) error {
//	        conn.Topics = []string{"notifications", "live"}
//	        conn.Metadata["tenant_id"] = c.Locals("tenant_id").(string)
//	        return nil
//	    },
//	})
//	app.Get("/events", handler)
//
//	// From any handler or worker:
//	hub.Publish(sse.Event{Type: "update", Data: "hello", Topics: []string{"live"}})
func NewWithHub(config ...Config) (fiber.Handler, *Hub) {
	cfg := configDefault(config...)

	hub := &Hub{
		cfg:           cfg,
		register:      make(chan *Connection, 64),
		unregister:    make(chan *Connection, 64),
		events:        make(chan Event, 1024),
		shutdown:      make(chan struct{}),
		connections:   make(map[string]*Connection),
		topicIndex:    make(map[string]map[string]struct{}),
		wildcardConns: make(map[string]struct{}),
		throttler:     newAdaptiveThrottler(cfg.FlushInterval),
		metrics:       hubMetrics{eventsByType: make(map[string]*atomic.Int64)},
		stopped:       make(chan struct{}),
	}

	go hub.run()

	handler := func(c fiber.Ctx) error {
		// Skip middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Reject during graceful drain
		if hub.draining.Load() {
			c.Set("Retry-After", "5")
			return c.Status(fiber.StatusServiceUnavailable).SendString("server draining, please reconnect")
		}

		conn := newConnection(
			generateID(),
			nil,
			cfg.SendBufferSize,
			cfg.FlushInterval,
		)

		// Let the application authenticate and configure the connection
		if cfg.OnConnect != nil {
			if err := cfg.OnConnect(c, conn); err != nil {
				return c.Status(fiber.StatusForbidden).SendString(err.Error())
			}
		}

		// Freeze metadata — defensive copy to prevent concurrent mutation
		// after the connection is registered with the hub.
		frozen := make(map[string]string, len(conn.Metadata))
		maps.Copy(frozen, conn.Metadata)
		conn.Metadata = frozen

		if len(conn.Topics) == 0 {
			return c.Status(fiber.StatusBadRequest).SendString("no topics subscribed")
		}

		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		// Capture Last-Event-ID before entering the stream writer
		lastEventID := c.Get("Last-Event-ID")
		if lastEventID == "" {
			lastEventID = c.Query("lastEventID")
		}

		return c.SendStreamWriter(func(w *bufio.Writer) {
			defer func() {
				// Use select to avoid blocking forever if hub.run() has exited (CRITICAL-3).
				select {
				case hub.unregister <- conn:
				case <-hub.shutdown:
				}
				conn.Close()
				if cfg.OnDisconnect != nil {
					cfg.OnDisconnect(conn)
				}
			}()

			if err := hub.initStream(w, conn, lastEventID); err != nil {
				return
			}

			// Register AFTER initStream to avoid duplicate events from
			// replay + live delivery race (MAJOR-7).
			select {
			case hub.register <- conn:
			case <-hub.shutdown:
				return
			}

			hub.watchLifetime(conn)
			hub.watchShutdown(conn)
			conn.writeLoop(w)
		})
	}

	return handler, hub
}

// generateID produces a random 32-character hex string for connection IDs.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("sse: failed to generate connection ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// Publish sends an event to all connections subscribed to the event's topics.
// This method is goroutine-safe and non-blocking. If the internal event buffer
// is full, the event is dropped and eventsDropped is incremented.
func (h *Hub) Publish(event Event) { //nolint:gocritic // hugeParam: public API, value semantics preferred
	if event.TTL > 0 && event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	select {
	case h.events <- event:
		h.metrics.eventsPublished.Add(1)
	case <-h.shutdown:
		// Hub is shutting down, discard
	default:
		// Buffer full — drop event to avoid blocking callers (MAJOR-5).
		h.metrics.eventsDropped.Add(1)
	}
}

// SetPaused pauses or resumes a connection by ID. Paused connections
// skip P1/P2 events (visibility hint for hidden browser tabs).
// P0 (instant) events are always delivered regardless.
func (h *Hub) SetPaused(connID string, paused bool) { //nolint:revive // flag-parameter: public API toggle
	h.mu.RLock()
	conn, ok := h.connections[connID]
	h.mu.RUnlock()
	if ok {
		wasPaused := conn.paused.Swap(paused)
		if paused && !wasPaused && h.cfg.OnPause != nil {
			h.cfg.OnPause(conn)
		}
		if !paused && wasPaused && h.cfg.OnResume != nil {
			h.cfg.OnResume(conn)
		}
	}
}

// Shutdown gracefully drains all connections and stops the hub.
// It enters drain mode (rejects new connections), sends a server-shutdown
// event to all clients, then closes the hub.
// Safe to call multiple times — subsequent calls are no-ops.
// Pass context.Background() for an unbounded wait.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.draining.Store(true)
	h.shutdownOnce.Do(func() {
		close(h.shutdown)
	})

	select {
	case <-h.stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("sse: shutdown: %w", ctx.Err())
	}
}

// Stats returns a snapshot of the hub's current state.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	byTopic := make(map[string]int, len(h.topicIndex))
	for topic, conns := range h.topicIndex {
		byTopic[topic] = len(conns)
	}

	return HubStats{
		ActiveConnections:  len(h.connections),
		TotalTopics:        len(h.topicIndex),
		EventsPublished:    h.metrics.eventsPublished.Load(),
		EventsDropped:      h.metrics.eventsDropped.Load(),
		ConnectionsByTopic: byTopic,
		EventsByType:       h.metrics.snapshotEventsByType(),
	}
}

// initStream writes the initial SSE preamble: retry hint, replayed events,
// and the connected event.
func (h *Hub) initStream(w *bufio.Writer, conn *Connection, lastEventID string) error {
	if err := writeRetry(w, h.cfg.RetryMS); err != nil {
		return err
	}

	if err := h.replayEvents(w, conn, lastEventID); err != nil {
		return err
	}

	return sendConnectedEvent(w, conn)
}

// replayEvents replays missed events if the client sent a Last-Event-ID.
func (h *Hub) replayEvents(w *bufio.Writer, conn *Connection, lastEventID string) error {
	if lastEventID == "" || h.cfg.Replayer == nil {
		return nil
	}
	events, err := h.cfg.Replayer.Replay(lastEventID, conn.Topics)
	if err != nil {
		log.Warnf("sse: replay error for conn %s: %v", conn.ID, err)
		return nil
	}
	if len(events) == 0 {
		return nil
	}
	for _, me := range events {
		if _, werr := me.WriteTo(w); werr != nil {
			return werr
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("sse: flush replay: %w", err)
	}
	return nil
}

// sendConnectedEvent writes the connected event with the connection ID
// and subscribed topics.
func sendConnectedEvent(w *bufio.Writer, conn *Connection) error {
	topicsJSON, err := json.Marshal(conn.Topics)
	if err != nil {
		topicsJSON = []byte("[]")
	}
	connected := MarshaledEvent{
		ID:    nextEventID(),
		Type:  "connected",
		Data:  fmt.Sprintf(`{"connection_id":%q,"topics":%s}`, conn.ID, string(topicsJSON)),
		Retry: -1,
	}
	if _, err := connected.WriteTo(w); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("sse: flush connected event: %w", err)
	}
	return nil
}

// watchLifetime starts a goroutine that closes the connection after
// MaxLifetime has elapsed.
func (h *Hub) watchLifetime(conn *Connection) {
	if h.cfg.MaxLifetime <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(h.cfg.MaxLifetime)
		defer timer.Stop()
		select {
		case <-timer.C:
			conn.Close()
		case <-conn.done:
		}
	}()
}

// shutdownDrainDelay is the time between sending the server-shutdown event
// and closing the connection, allowing the client to process the event.
const shutdownDrainDelay = 200 * time.Millisecond

// watchShutdown starts a goroutine that sends a server-shutdown event
// and closes the connection when the hub begins draining.
func (h *Hub) watchShutdown(conn *Connection) {
	go func() {
		select {
		case <-h.shutdown:
			if !conn.IsClosed() {
				shutdownEvt := MarshaledEvent{
					ID:    nextEventID(),
					Type:  "server-shutdown",
					Data:  "{}",
					Retry: -1,
				}
				conn.trySend(shutdownEvt)
				time.Sleep(shutdownDrainDelay)
			}
			conn.Close()
		case <-conn.done:
		}
	}()
}

// run is the hub's main event loop.
func (h *Hub) run() {
	defer close(h.stopped)

	flushTicker := time.NewTicker(h.cfg.FlushInterval)
	defer flushTicker.Stop()

	heartbeatTicker := time.NewTicker(h.cfg.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.addConnection(conn)

		case conn := <-h.unregister:
			h.removeConnection(conn)

		case event := <-h.events:
			h.routeEvent(&event)

		case <-flushTicker.C:
			h.flushAll()

		case <-heartbeatTicker.C:
			h.sendHeartbeats()

		case <-cleanupTicker.C:
			h.throttler.cleanup(time.Now().Add(-10 * time.Minute))

		case <-h.shutdown:
			h.mu.Lock()
			for _, conn := range h.connections {
				conn.Close()
			}
			h.mu.Unlock()
			return
		}
	}
}

// addConnection registers a new connection and indexes it by topic.
func (h *Hub) addConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connections[conn.ID] = conn

	hasWildcard := false
	for _, topic := range conn.Topics {
		if strings.ContainsAny(topic, "*>") {
			hasWildcard = true
		} else {
			if h.topicIndex[topic] == nil {
				h.topicIndex[topic] = make(map[string]struct{})
			}
			h.topicIndex[topic][conn.ID] = struct{}{}
		}
	}
	if hasWildcard {
		h.wildcardConns[conn.ID] = struct{}{}
	}

	log.Infof("sse: connection opened conn_id=%s topics=%v total=%d", conn.ID, conn.Topics, len(h.connections))
}

// removeConnection unregisters a connection and removes it from topic indexes.
func (h *Hub) removeConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.connections[conn.ID]; !exists {
		return
	}

	for _, topic := range conn.Topics {
		if idx, ok := h.topicIndex[topic]; ok {
			delete(idx, conn.ID)
			if len(idx) == 0 {
				delete(h.topicIndex, topic)
			}
		}
	}

	delete(h.wildcardConns, conn.ID)
	delete(h.connections, conn.ID)
	h.throttler.remove(conn.ID)

	log.Infof("sse: connection closed conn_id=%s sent=%d dropped=%d total=%d",
		conn.ID, conn.MessagesSent.Load(), conn.MessagesDropped.Load(), len(h.connections))
}

// routeEvent delivers an event to all connections subscribed to its topics.
func (h *Hub) routeEvent(event *Event) {
	if event.TTL > 0 && !event.CreatedAt.IsZero() {
		if time.Since(event.CreatedAt) > event.TTL {
			h.metrics.eventsDropped.Add(1)
			return
		}
	}

	me := marshalEvent(event)
	h.metrics.trackEventType(event.Type)

	// Skip replay storage for group-scoped events — replaying them without
	// tenant context would leak data across tenants (CRITICAL-2).
	if h.cfg.Replayer != nil && len(event.Group) == 0 {
		_ = h.cfg.Replayer.Store(me, event.Topics) //nolint:errcheck // best-effort replay storage
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := h.matchConnections(event)

	for connID := range seen {
		conn, ok := h.connections[connID]
		if !ok || conn.IsClosed() {
			continue
		}
		if conn.paused.Load() && event.Priority != PriorityInstant {
			continue
		}
		h.deliverToConn(conn, event, me)
	}
}

// matchConnections collects all connection IDs that should receive the event.
// When an event has BOTH Topics AND Group set, only connections matching BOTH
// are included (intersection semantics for tenant isolation). When only one
// dimension is set, the existing OR behavior applies.
func (h *Hub) matchConnections(event *Event) map[string]struct{} {
	seen := make(map[string]struct{})

	for _, topic := range event.Topics {
		if idx, ok := h.topicIndex[topic]; ok {
			for connID := range idx {
				seen[connID] = struct{}{}
			}
		}
	}

	h.matchWildcardConns(event, seen)

	// When both Topics and Group are present, filter topic-matched connections
	// down to those also matching the group (AND semantics).
	if len(event.Group) > 0 && len(event.Topics) > 0 {
		for connID := range seen {
			conn, ok := h.connections[connID]
			if !ok || !connMatchesGroup(conn, event.Group) {
				delete(seen, connID)
			}
		}
	} else {
		h.matchGroupConns(event, seen)
	}

	return seen
}

// matchWildcardConns adds wildcard-subscribed connections that match the event topics.
func (h *Hub) matchWildcardConns(event *Event, seen map[string]struct{}) {
	for connID := range h.wildcardConns {
		if _, already := seen[connID]; already {
			continue
		}
		conn, ok := h.connections[connID]
		if !ok {
			continue
		}
		for _, eventTopic := range event.Topics {
			if connMatchesTopic(conn, eventTopic) {
				seen[connID] = struct{}{}
				break
			}
		}
	}
}

// matchGroupConns adds connections that match the event's group metadata.
func (h *Hub) matchGroupConns(event *Event, seen map[string]struct{}) {
	if len(event.Group) == 0 {
		return
	}
	for connID, conn := range h.connections {
		if _, already := seen[connID]; already {
			continue
		}
		if connMatchesGroup(conn, event.Group) {
			seen[connID] = struct{}{}
		}
	}
}

// deliverToConn routes an event to a connection based on priority.
func (h *Hub) deliverToConn(conn *Connection, event *Event, me MarshaledEvent) { //nolint:gocritic // hugeParam: value semantics preferred for event routing
	switch event.Priority {
	case PriorityInstant:
		if !conn.trySend(me) {
			h.metrics.eventsDropped.Add(1)
		}
	case PriorityBatched:
		conn.coalescer.addBatched(me)
	default: // PriorityCoalesced
		key := event.CoalesceKey
		if key == "" {
			key = event.Type
		}
		conn.coalescer.addCoalesced(key, me)
	}
}

// flushAll drains each connection's coalescer and sends buffered events.
func (h *Hub) flushAll() {
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, conn := range h.connections {
		if !conn.IsClosed() && !conn.paused.Load() {
			conns = append(conns, conn)
		}
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		if conn.IsClosed() {
			continue
		}

		bufCap := cap(conn.send)
		saturation := float64(0)
		if bufCap > 0 {
			saturation = float64(len(conn.send)) / float64(bufCap)
		}

		if !h.throttler.shouldFlush(conn.ID, saturation) {
			continue
		}

		events := conn.coalescer.flush()
		now := time.Now()
		for _, me := range events {
			// Drop coalesced events that have expired while buffered (MAJOR-6).
			if me.TTL > 0 && !me.CreatedAt.IsZero() && now.Sub(me.CreatedAt) > me.TTL {
				h.metrics.eventsDropped.Add(1)
				continue
			}
			if !conn.trySend(me) {
				h.metrics.eventsDropped.Add(1)
			}
		}
	}
}

// sendHeartbeats sends a comment to connections that haven't received
// real data recently.
func (h *Hub) sendHeartbeats() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	for _, conn := range h.connections {
		if conn.IsClosed() {
			continue
		}
		lastWrite, _ := conn.lastWrite.Load().(time.Time) //nolint:errcheck // type assertion on atomic.Value
		if now.Sub(lastWrite) >= h.cfg.HeartbeatInterval {
			conn.sendHeartbeat()
		}
	}
}
