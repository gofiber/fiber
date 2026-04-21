package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	bridgeCancel  context.CancelFunc
	metrics       hubMetrics
	cfg           Config
	bridges       sync.WaitGroup
	mu            sync.RWMutex
	shutdownOnce  sync.Once
	draining      atomic.Bool
}

// newHub constructs a Hub from resolved Config and starts the run loop.
// Bridges (if any) are started and tracked in hub.bridges; Shutdown waits
// for all of them to finish before reporting stopped.
func newHub(cfg Config) *Hub { //nolint:gocritic // hugeParam: internal constructor, single call site, avoids pointer escape
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

	if len(cfg.Bridges) > 0 {
		// cancel is stored on the Hub and invoked in Shutdown; the linter
		// can't follow that across goroutines, so suppress G118 here.
		ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored on hub.bridgeCancel and invoked in Shutdown
		hub.bridgeCancel = cancel
		for _, bc := range cfg.Bridges {
			if bc.Subscriber == nil {
				panic("sse: BridgeConfig.Subscriber must not be nil")
			}
			hub.bridges.Add(1)
			go func(cfg BridgeConfig) {
				defer hub.bridges.Done()
				hub.runBridge(ctx, cfg)
			}(bc)
		}
	}

	return hub
}

// Publish sends an event to all connections subscribed to the event's topics.
// This method is goroutine-safe and non-blocking. If the internal event buffer
// is full, the event is dropped and eventsDropped is incremented.
func (h *Hub) Publish(event Event) { //nolint:gocritic // hugeParam: public API, value semantics preferred
	// Reject early if the hub is draining. Without this, a concurrent
	// Shutdown() can race with Publish() and enqueue an event the run
	// loop will never dispatch — inflating EventsPublished and leaving
	// the caller under the false impression the event was delivered.
	if h.draining.Load() {
		h.metrics.eventsDropped.Add(1)
		return
	}
	if event.TTL > 0 && event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	select {
	case h.events <- event:
		h.metrics.eventsPublished.Add(1)
	case <-h.shutdown:
		// Hub is shutting down, discard
		h.metrics.eventsDropped.Add(1)
	default:
		// Buffer full — drop event to avoid blocking callers.
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

// Shutdown gracefully drains all connections and stops the hub. Any
// configured bridges are canceled and awaited before the hub reports stopped.
// Safe to call multiple times — subsequent calls are no-ops.
// Pass context.Background() for an unbounded wait.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.draining.Store(true)
	h.shutdownOnce.Do(func() {
		if h.bridgeCancel != nil {
			h.bridgeCancel()
		}
		close(h.shutdown)
	})

	// Bridges must finish before we report stopped so their in-flight
	// Publish calls don't race with a re-used hub — but wait must still
	// honor the caller's deadline so a wedged bridge can't hang Shutdown.
	bridgesDone := make(chan struct{})
	go func() {
		h.bridges.Wait()
		close(bridgesDone)
	}()

	select {
	case <-bridgesDone:
	case <-ctx.Done():
		return fmt.Errorf("sse: shutdown: %w", ctx.Err())
	}

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
		// Replay is best-effort; log and continue without replayed events.
		log.Warnf("sse: replayer error, continuing without replay: %v", err)
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

// broadcastShutdown queues a server-shutdown event on every live connection.
// Called from the run loop on the shutdown signal BEFORE any Close() so that
// writeLoop has a chance to flush the event to the network. A short drain
// delay afterwards gives writers time to complete the flush before close.
func (h *Hub) broadcastShutdown() {
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, conn := range h.connections {
		if !conn.IsClosed() {
			conns = append(conns, conn)
		}
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		conn.trySend(MarshaledEvent{
			ID:    nextEventID(),
			Type:  "server-shutdown",
			Data:  "{}",
			Retry: -1,
		})
	}
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
			// Notify clients first, wait briefly for writeLoops to flush,
			// then close. Prevents a race where Close() beats the
			// server-shutdown event to the network.
			h.broadcastShutdown()
			time.Sleep(shutdownDrainDelay)
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

	log.Infof("sse: connection opened conn_id=%s topics=%v total=%d",
		conn.ID, conn.Topics, len(h.connections))
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

	// Skip replayer for group-scoped events to avoid cross-tenant leaks
	// on reconnect. Store errors are logged but non-fatal — replay is a
	// best-effort feature and one missing event shouldn't break delivery.
	if h.cfg.Replayer != nil && len(event.Group) == 0 {
		if err := h.cfg.Replayer.Store(me, event.Topics); err != nil {
			log.Warnf("sse: replayer store error, continuing: %v", err)
		}
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
// When both Topics and Group are set, only connections matching BOTH are
// included (intersection semantics) to prevent tenant/topic leaks (CRITICAL-1).
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

	// If event has a Group, filter seen to intersection with group-matching conns.
	if len(event.Group) > 0 {
		if len(event.Topics) > 0 {
			// Intersection: keep only topic-matched conns that also match group.
			for connID := range seen {
				conn := h.connections[connID]
				if conn == nil || !connMatchesGroup(conn, event.Group) {
					delete(seen, connID)
				}
			}
		} else {
			// Group-only event: match by group alone.
			h.matchGroupConns(event, seen)
		}
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
func (h *Hub) deliverToConn(conn *Connection, event *Event, me MarshaledEvent) { //nolint:gocritic // hugeParam: internal, copy is cheap
	switch event.Priority {
	case PriorityInstant:
		if !conn.trySend(me) {
			h.metrics.eventsDropped.Add(1)
		}
	case PriorityBatched:
		conn.dispatcher.AddEvent(me)
	case PriorityCoalesced:
		key := event.CoalesceKey
		if key == "" {
			key = event.Type
		}
		conn.dispatcher.AddState(key, me)
	default:
		// Unknown priority: drop to avoid misrouting.
		h.metrics.eventsDropped.Add(1)
	}
}

// flushAll drains each connection's dispatcher and sends buffered events.
func (h *Hub) flushAll() {
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, conn := range h.connections {
		if !conn.IsClosed() && !conn.paused.Load() {
			conns = append(conns, conn)
		}
	}
	h.mu.RUnlock()

	now := time.Now()
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

		events := conn.dispatcher.WriteTo()
		for _, me := range events {
			// Re-check TTL after dispatching delay: coalesced events may
			// sit in the queue past their deadline (MAJOR-6).
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
