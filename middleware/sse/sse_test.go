package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// startSSEServer spins up a real TCP listener serving the given handler and
// returns the base URL + cleanup. Use for end-to-end response-body checks
// since app.Test() blocks on SSE streams that never terminate.
func startSSEServer(t *testing.T, handler fiber.Handler) (string, func()) { //nolint:gocritic // unnamedResult: nonamedreturns rule forbids names; types are self-explanatory
	t.Helper()
	app := fiber.New()
	app.Get("/events", handler)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		_ = app.Listener(ln) //nolint:errcheck // best-effort test listener
	}()

	baseURL := "http://" + ln.Addr().String()
	cleanup := func() {
		// Close the listener first to force-abort in-flight SSE writers;
		// app.Shutdown would otherwise wait for the long-lived handler.
		_ = ln.Close() //nolint:errcheck // may already be closed
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_ = app.ShutdownWithContext(shutdownCtx) //nolint:errcheck // best-effort test shutdown
	}
	return baseURL, cleanup
}

// sseFrameTimeout is the deadline for reading a single SSE frame in tests.
const sseFrameTimeout = 2 * time.Second

// readSSEFrame reads one SSE frame (ending in blank line) from r.
// Fails the test if no complete frame arrives within sseFrameTimeout.
func readSSEFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	type result struct {
		err   error
		frame string
	}
	done := make(chan result, 1)
	go func() {
		var buf bytes.Buffer
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				done <- result{err: err}
				return
			}
			_, _ = buf.WriteString(line) //nolint:errcheck // bytes.Buffer.WriteString never fails
			if line == "\n" {
				done <- result{frame: buf.String()}
				return
			}
		}
	}()
	select {
	case res := <-done:
		require.NoError(t, res.err)
		return res.frame
	case <-time.After(sseFrameTimeout):
		t.Fatal("timed out waiting for SSE frame")
		return ""
	}
}

func Test_SSE_E2E_HeadersAndConnectedFrame(t *testing.T) {
	t.Parallel()

	handler, hub := NewWithHub(Config{
		MaxLifetime:       500 * time.Millisecond,
		HeartbeatInterval: 100 * time.Millisecond,
		FlushInterval:     50 * time.Millisecond,
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"updates"}
			return nil
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	base, cleanup := startSSEServer(t, handler)
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/events", http.NoBody)
	require.NoError(t, err)

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	// RFC 8895 + W3C SSE: Content-Type must be text/event-stream.
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	require.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	require.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	br := bufio.NewReader(resp.Body)

	// First: retry directive frame from writeRetry.
	retryFrame := readSSEFrame(t, br)
	require.Contains(t, retryFrame, "retry: 3000")

	// Second: connected event with connection_id and topics.
	connectedFrame := readSSEFrame(t, br)
	require.Contains(t, connectedFrame, "event: connected")
	require.Contains(t, connectedFrame, "connection_id")
	require.Contains(t, connectedFrame, `"topics":["updates"]`)
}

func Test_SSE_E2E_PublishedEventDeliveredToClient(t *testing.T) {
	t.Parallel()

	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"orders"}
			return nil
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	base, cleanup := startSSEServer(t, handler)
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/events", http.NoBody)
	require.NoError(t, err)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	br := bufio.NewReader(resp.Body)
	_ = readSSEFrame(t, br) // retry
	_ = readSSEFrame(t, br) // connected

	// Give the hub time to register the connection before publishing.
	time.Sleep(50 * time.Millisecond)

	hub.Publish(Event{
		Type:     "order-created",
		Data:     `{"id":"ord_123","total":99}`,
		Topics:   []string{"orders"},
		Priority: PriorityInstant,
	})

	frame := readSSEFrame(t, br)
	require.Contains(t, frame, "event: order-created")
	require.Contains(t, frame, `data: {"id":"ord_123","total":99}`)
	require.Contains(t, frame, "id: evt_")
}

func Test_SSE_E2E_MultilineDataProducesMultipleDataLines(t *testing.T) {
	t.Parallel()

	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"logs"}
			return nil
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	base, cleanup := startSSEServer(t, handler)
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/events", http.NoBody)
	require.NoError(t, err)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	br := bufio.NewReader(resp.Body)
	_ = readSSEFrame(t, br)
	_ = readSSEFrame(t, br)
	time.Sleep(50 * time.Millisecond)

	hub.Publish(Event{
		Type:     "log",
		Data:     "line1\nline2\nline3",
		Topics:   []string{"logs"},
		Priority: PriorityInstant,
	})

	frame := readSSEFrame(t, br)
	require.Contains(t, frame, "data: line1\n")
	require.Contains(t, frame, "data: line2\n")
	require.Contains(t, frame, "data: line3\n")
}

func Test_SSE_E2E_IDAndTypeSanitizedAgainstInjection(t *testing.T) {
	t.Parallel()

	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"t"}
			return nil
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	base, cleanup := startSSEServer(t, handler)
	defer cleanup()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/events", http.NoBody)
	require.NoError(t, err)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	br := bufio.NewReader(resp.Body)
	_ = readSSEFrame(t, br)
	_ = readSSEFrame(t, br)
	time.Sleep(50 * time.Millisecond)

	hub.Publish(Event{
		ID:       "bad\nid: injected",
		Type:     "evt\nevent: sneaky",
		Data:     "x",
		Topics:   []string{"t"},
		Priority: PriorityInstant,
	})

	frame := readSSEFrame(t, br)

	// Split frame into logical lines; id: and event: must each appear on
	// exactly one line. Injection attempts that embed `\nid: injected` or
	// `\nevent: sneaky` would create extra lines the SSE parser would
	// interpret as additional fields — we must collapse them away.
	lines := strings.Split(frame, "\n")
	var idLines, eventLines int
	for _, line := range lines {
		if strings.HasPrefix(line, "id: ") {
			idLines++
		}
		if strings.HasPrefix(line, "event: ") {
			eventLines++
		}
	}
	require.Equal(t, 1, idLines, "id: injection must be sanitized")
	require.Equal(t, 1, eventLines, "event: injection must be sanitized")
}

func Test_SSE_New(t *testing.T) {
	t.Parallel()

	handler, hub := NewWithHub()
	require.NotNil(t, handler)
	require.NotNil(t, hub)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_New_DefaultConfig(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	require.Equal(t, 2*time.Second, hub.cfg.FlushInterval)
	require.Equal(t, 256, hub.cfg.SendBufferSize)
	require.Equal(t, 30*time.Second, hub.cfg.HeartbeatInterval)
	require.Equal(t, 30*time.Minute, hub.cfg.MaxLifetime)
	require.Equal(t, 3000, hub.cfg.RetryMS)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_New_CustomConfig(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{
		FlushInterval:     5 * time.Second,
		SendBufferSize:    128,
		HeartbeatInterval: 10 * time.Second,
		MaxLifetime:       time.Hour,
		RetryMS:           5000,
	})
	require.Equal(t, 5*time.Second, hub.cfg.FlushInterval)
	require.Equal(t, 128, hub.cfg.SendBufferSize)
	require.Equal(t, 10*time.Second, hub.cfg.HeartbeatInterval)
	require.Equal(t, time.Hour, hub.cfg.MaxLifetime)
	require.Equal(t, 5000, hub.cfg.RetryMS)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_NoTopics(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, _ *Connection) error {
			// Don't set any topics
			return nil
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	app.Get("/events", handler)

	req, err := http.NewRequest(fiber.MethodGet, "/events", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func Test_SSE_OnConnectReject(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, _ *Connection) error {
			return errors.New("unauthorized")
		},
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	app.Get("/events", handler)

	req, err := http.NewRequest(fiber.MethodGet, "/events", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func Test_SSE_GenerateID(t *testing.T) {
	t.Parallel()

	ids := make(map[string]struct{})
	for range 1000 {
		id := generateID()
		require.Len(t, id, 32)
		_, exists := ids[id]
		require.False(t, exists, "duplicate ID generated")
		ids[id] = struct{}{}
	}
}

func Test_SSE_TopicMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern string
		topic   string
		want    bool
	}{
		{"events", "events", true},
		{"events", "events.sub", false},
		{"notifications.*", "notifications.orders", true},
		{"notifications.*", "notifications.orders.new", false},
		{"analytics.>", "analytics.live", true},
		{"analytics.>", "analytics.live.visitors", true},
		{"analytics.>", "analytics", false},
		{"*", "anything", true},
		{">", "anything", true},
		{">", "a.b.c", true},
		// > must be last token — invalid patterns should not match
		{"a.>.c", "a.b.c", false},
		{">.b", "a.b", false},
	}

	for _, tt := range tests {
		got := topicMatch(tt.pattern, tt.topic)
		require.Equal(t, tt.want, got, "topicMatch(%q, %q)", tt.pattern, tt.topic)
	}
}

func Test_SSE_MarshalEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"bytes", []byte("world"), "world"},
		{"struct", map[string]string{"key": "val"}, `{"key":"val"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			me := marshalEvent(&Event{Data: tt.data})
			require.Equal(t, tt.want, me.Data)
			require.NotEmpty(t, me.ID) // auto-generated
		})
	}
}

func Test_SSE_MarshaledEvent_WriteTo(t *testing.T) {
	t.Parallel()

	me := MarshaledEvent{
		ID:   "evt_1",
		Type: "test",
		Data: "hello world",
	}

	var buf bytes.Buffer
	n, err := me.WriteTo(&buf)
	require.NoError(t, err)
	require.Positive(t, n)

	output := buf.String()
	require.Contains(t, output, "id: evt_1\n")
	require.Contains(t, output, "event: test\n")
	require.Contains(t, output, "data: hello world\n")
	// A zero-value Retry field must not produce `retry: 0` (which would
	// tell clients to reconnect immediately per the SSE spec).
	require.NotContains(t, output, "retry:")
	require.True(t, strings.HasSuffix(output, "\n\n"))
}

func Test_SSE_MarshaledEvent_WriteTo_Multiline(t *testing.T) {
	t.Parallel()

	me := MarshaledEvent{
		ID:   "evt_2",
		Type: "test",
		Data: "line1\nline2\nline3",
	}

	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "data: line1\n")
	require.Contains(t, output, "data: line2\n")
	require.Contains(t, output, "data: line3\n")
}

func Test_SSE_MarshaledEvent_WriteTo_RetryZeroOmitted(t *testing.T) {
	t.Parallel()

	// Retry: 0 (the zero value) must NOT emit `retry: 0\n`. Per the SSE
	// spec that directive tells clients to reconnect immediately —
	// emitting it for an unset field could cascade into a reconnect storm.
	me := MarshaledEvent{ID: "evt_zero", Type: "test", Data: "x", Retry: 0}
	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)
	require.NotContains(t, buf.String(), "retry:")
}

func Test_SSE_MarshaledEvent_WriteTo_SanitizesInjectionAtBoundary(t *testing.T) {
	t.Parallel()

	// An external Replayer can construct MarshaledEvent directly, bypassing
	// marshalEvent's sanitization. WriteTo is the last line of defense —
	// control sequences in ID or Type must be stripped so an attacker can't
	// inject additional SSE fields onto the wire by embedding \n.
	me := MarshaledEvent{
		ID:   "evt_1\nevent: injected\nid: fake",
		Type: "custom\ndata: also_injected",
		Data: "payload",
	}
	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)

	output := buf.String()

	// Invariant: exactly one event frame, exactly one id header, exactly
	// one event header. The SSE parser only recognizes `id:` / `event:`
	// at the start of a line — so count lines starting with them, not raw
	// substring occurrences (the attacker's text survives INSIDE the
	// field value but cannot start a new line).
	var idLines, eventLines int
	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "id: "):
			idLines++
		case strings.HasPrefix(line, "event: "):
			eventLines++
		default:
		}
	}
	require.Equal(t, 1, idLines, "exactly one id line")
	require.Equal(t, 1, eventLines, "exactly one event line")

	// Frame terminator: ends with exactly one blank line separator.
	require.True(t, strings.HasSuffix(output, "\n\n"))
	// No partial frames — only one `\n\n` separator total.
	require.Equal(t, 1, strings.Count(output, "\n\n"))
}

func Test_SSE_MarshaledEvent_WriteTo_TypedNilJSONMarshaler(t *testing.T) {
	t.Parallel()

	// A typed-nil pointer whose type implements json.Marshaler used to panic
	// in the explicit `case json.Marshaler:` branch because the receiver
	// was dereferenced without a nil check. All values now flow through
	// json.Marshal in the default branch which is nil-safe (emits "null").
	var nilMarshaler *panicOnMarshal
	evt := Event{ID: "evt_tn", Type: "test", Data: nilMarshaler, Topics: []string{"t"}}
	me := marshalEvent(&evt)

	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "data: null\n")
}

// panicOnMarshal dereferences its receiver in MarshalJSON — a typed-nil
// pointer to this type would panic if invoked directly. Used to prove
// marshalEvent routes through json.Marshal's nil-safe path.
type panicOnMarshal struct{ Name string }

func (p *panicOnMarshal) MarshalJSON() ([]byte, error) {
	// Intentionally dereferences — would panic if called on a typed-nil.
	return []byte(`"` + p.Name + `"`), nil
}

func Test_SSE_MarshaledEvent_WriteTo_Retry(t *testing.T) {
	t.Parallel()

	me := MarshaledEvent{
		ID:    "evt_3",
		Type:  "test",
		Data:  "x",
		Retry: 3000,
	}

	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "retry: 3000\n")
}

func Test_SSE_Dispatcher(t *testing.T) {
	t.Parallel()

	c := newDispatcher(time.Second)

	// Add batched events
	c.AddEvent(MarshaledEvent{ID: "1", Data: "a"})
	c.AddEvent(MarshaledEvent{ID: "2", Data: "b"})

	// Add coalesced events (last wins)
	c.AddState("key1", MarshaledEvent{ID: "3", Data: "old"})
	c.AddState("key1", MarshaledEvent{ID: "4", Data: "new"})
	c.AddState("key2", MarshaledEvent{ID: "5", Data: "other"})

	require.Equal(t, 4, c.pending())

	events := c.WriteTo()
	require.Len(t, events, 4)

	// Batched first
	require.Equal(t, "a", events[0].Data)
	require.Equal(t, "b", events[1].Data)

	// Coalesced: key1 = "new" (last wins), key2 = "other"
	require.Equal(t, "new", events[2].Data)
	require.Equal(t, "other", events[3].Data)

	// Should be empty now
	require.Nil(t, c.WriteTo())
}

func Test_SSE_AdaptiveThrottler(t *testing.T) {
	t.Parallel()

	at := newAdaptiveThrottler(2 * time.Second)

	// First flush always passes
	require.True(t, at.shouldFlush("conn1", 0.0))

	// Second flush immediately — should fail (too soon)
	require.False(t, at.shouldFlush("conn1", 0.0))

	// Clean up
	at.remove("conn1")
}

func Test_SSE_Publish_Stats(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	hub.Publish(Event{Type: "test", Topics: []string{"t"}, Data: "hello"})
	time.Sleep(50 * time.Millisecond) // let run loop process

	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsPublished)
}

func Test_SSE_Shutdown(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := hub.Shutdown(ctx)
	require.NoError(t, err)
	require.True(t, hub.draining.Load())
}

func Test_SSE_Shutdown_Idempotent(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// First call shuts down
	require.NoError(t, hub.Shutdown(ctx))

	// Second call must not panic (sync.Once guards close)
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_Shutdown_Background_Context(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()

	err := hub.Shutdown(context.Background())
	require.NoError(t, err)
}

func Test_SSE_Draining_RejectsConnection(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	handler, hub := NewWithHub(Config{
		OnConnect: func(_ fiber.Ctx, conn *Connection) error {
			conn.Topics = []string{"test"}
			return nil
		},
	})

	app.Get("/events", handler)

	// Start draining
	hub.draining.Store(true)
	defer func() {
		close(hub.shutdown)
		<-hub.stopped
	}()

	req, err := http.NewRequest(fiber.MethodGet, "/events", http.NoBody)
	require.NoError(t, err)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
}

func Test_SSE_Connection_Lifecycle(t *testing.T) {
	t.Parallel()

	conn := newConnection("test-id", []string{"t"}, 10, time.Second)
	require.Equal(t, "test-id", conn.ID)
	require.False(t, conn.IsClosed())

	conn.Close()
	require.True(t, conn.IsClosed())

	// Double close should not panic
	conn.Close()
}

func Test_SSE_Connection_TrySend_Backpressure(t *testing.T) {
	t.Parallel()

	conn := newConnection("test", nil, 2, time.Second)

	require.True(t, conn.trySend(MarshaledEvent{Data: "1"}))
	require.True(t, conn.trySend(MarshaledEvent{Data: "2"}))

	// Buffer full
	require.False(t, conn.trySend(MarshaledEvent{Data: "3"}))
	require.Equal(t, int64(1), conn.MessagesDropped.Load())
}

func Test_SSE_WriteComment(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeComment(&buf, "heartbeat")
	require.NoError(t, err)
	require.Equal(t, ": heartbeat\n\n", buf.String())
}

func Test_SSE_WriteRetry(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeRetry(&buf, 3000)
	require.NoError(t, err)
	require.Equal(t, "retry: 3000\n\n", buf.String())
}

func Test_SSE_MaxLifetime_Unlimited(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{
		MaxLifetime: -1, // unlimited
	})
	require.Equal(t, time.Duration(-1), hub.cfg.MaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_SetPaused(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// SetPaused on non-existent connection should not panic
	hub.SetPaused("nonexistent", true)

	// Add a connection manually
	conn := newConnection("test-conn", []string{"t"}, 10, time.Second)
	hub.mu.Lock()
	hub.connections["test-conn"] = conn
	hub.mu.Unlock()

	hub.SetPaused("test-conn", true)
	require.True(t, conn.paused.Load())

	hub.SetPaused("test-conn", false)
	require.False(t, conn.paused.Load())
}

// ---------------------------------------------------------------------------
// Coverage-boost tests
// ---------------------------------------------------------------------------

func Test_SSE_New_Wrapper(t *testing.T) {
	t.Parallel()
	handler := New()
	require.NotNil(t, handler)
}

func Test_SSE_SanitizeSSEField(t *testing.T) {
	t.Parallel()

	require.Equal(t, "clean", sanitizeSSEField("clean"))
	require.Equal(t, "ab", sanitizeSSEField("a\nb"))
	require.Equal(t, "ab", sanitizeSSEField("a\rb"))
	require.Equal(t, "ab", sanitizeSSEField("a\r\nb"))
	require.Equal(t, "abc", sanitizeSSEField("a\r\nb\nc"))
}

func Test_SSE_MarshalEvent_SanitizesIDAndType(t *testing.T) {
	t.Parallel()

	me := marshalEvent(&Event{
		ID:   "id\r\ninjected",
		Type: "type\ninjected",
		Data: "safe",
	})
	require.Equal(t, "idinjected", me.ID)
	require.Equal(t, "typeinjected", me.Type)
}

func Test_SSE_MarshalEvent_JsonMarshalerError(t *testing.T) {
	t.Parallel()

	me := marshalEvent(&Event{Data: badMarshaler{}})
	require.Contains(t, me.Data, "error")
}

func Test_SSE_MarshalEvent_DefaultMarshalError(t *testing.T) {
	t.Parallel()

	// A channel cannot be JSON-marshaled
	me := marshalEvent(&Event{Data: make(chan int)})
	require.Contains(t, me.Data, "error")
}

// badMarshaler implements json.Marshaler and always returns an error.
type badMarshaler struct{}

func (badMarshaler) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failed")
}

func Test_SSE_WriteTo_EmptyFields(t *testing.T) {
	t.Parallel()

	me := MarshaledEvent{
		Data:  "x",
		Retry: -1,
	}
	var buf bytes.Buffer
	_, err := me.WriteTo(&buf)
	require.NoError(t, err)
	output := buf.String()
	// No id: or event: lines
	require.NotContains(t, output, "id:")
	require.NotContains(t, output, "event:")
	require.Contains(t, output, "data: x\n")
}

func Test_SSE_ConnMatchesGroup(t *testing.T) {
	t.Parallel()

	conn := newConnection("c1", []string{"t"}, 10, time.Second)
	conn.Metadata["tenant_id"] = "t_1"
	conn.Metadata["role"] = "admin"

	require.True(t, connMatchesGroup(conn, map[string]string{"tenant_id": "t_1"}))
	require.True(t, connMatchesGroup(conn, map[string]string{"tenant_id": "t_1", "role": "admin"}))
	require.False(t, connMatchesGroup(conn, map[string]string{"tenant_id": "t_2"}))
	require.False(t, connMatchesGroup(conn, map[string]string{"missing": "key"}))
	require.True(t, connMatchesGroup(conn, map[string]string{})) // empty group matches all
}

func Test_SSE_SendHeartbeat(t *testing.T) {
	t.Parallel()

	conn := newConnection("hb", []string{"t"}, 10, time.Second)

	// First heartbeat should succeed
	conn.sendHeartbeat()
	// Second should be silently dropped (buffer 1)
	conn.sendHeartbeat()

	// Drain the heartbeat channel
	select {
	case <-conn.heartbeat:
	default:
		t.Fatal("expected heartbeat in channel")
	}
}

func Test_SSE_WriteLoop_Events(t *testing.T) {
	t.Parallel()

	conn := newConnection("wl", []string{"t"}, 10, time.Second)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	// Send an event + heartbeat, then close
	conn.trySend(MarshaledEvent{ID: "e1", Type: "test", Data: "hello", Retry: -1})
	conn.sendHeartbeat()

	go func() {
		time.Sleep(50 * time.Millisecond)
		conn.Close()
	}()

	conn.writeLoop(w)

	output := buf.String()
	require.Contains(t, output, "id: e1\n")
	require.Contains(t, output, "event: test\n")
	require.Contains(t, output, "data: hello\n")
	require.Contains(t, output, ": heartbeat\n")
	require.Equal(t, int64(1), conn.MessagesSent.Load())
}

func Test_SSE_WriteLoop_ChannelClose(t *testing.T) {
	t.Parallel()

	conn := newConnection("wlc", []string{"t"}, 10, time.Second)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	// Close the send channel directly to test the !ok path
	close(conn.send)
	conn.writeLoop(w)
	// Should return without panic
}

func Test_SSE_TopicMatchesAny(t *testing.T) {
	t.Parallel()

	require.True(t, topicMatchesAny([]string{"orders", "products"}, "orders"))
	require.True(t, topicMatchesAny([]string{"orders.*"}, "orders.created"))
	require.False(t, topicMatchesAny([]string{"orders", "products"}, "users"))
	require.False(t, topicMatchesAny(nil, "anything"))
}

func Test_SSE_ConnMatchesTopic(t *testing.T) {
	t.Parallel()

	conn := newConnection("ct", []string{"orders.*", "products"}, 10, time.Second)
	require.True(t, connMatchesTopic(conn, "orders.created"))
	require.True(t, connMatchesTopic(conn, "products"))
	require.False(t, connMatchesTopic(conn, "users"))
}

func Test_SSE_EffectiveInterval_AllBranches(t *testing.T) {
	t.Parallel()

	at := newAdaptiveThrottler(2 * time.Second)

	// saturation > 0.8 → maxInterval
	require.Equal(t, at.maxInterval, at.effectiveInterval(0.9))
	// saturation > 0.5 → baseInterval * 2
	require.Equal(t, at.baseInterval*2, at.effectiveInterval(0.6))
	// saturation < 0.1 → minInterval
	require.Equal(t, at.minInterval, at.effectiveInterval(0.05))
	// default → baseInterval
	require.Equal(t, at.baseInterval, at.effectiveInterval(0.3))
}

func Test_SSE_Throttler_Cleanup(t *testing.T) {
	t.Parallel()

	at := newAdaptiveThrottler(time.Second)
	at.shouldFlush("old-conn", 0.0)
	at.shouldFlush("new-conn", 0.0)

	// Make "old-conn" stale
	at.mu.Lock()
	at.lastFlush["old-conn"] = time.Now().Add(-20 * time.Minute)
	at.mu.Unlock()

	at.cleanup(time.Now().Add(-10 * time.Minute))

	at.mu.Lock()
	_, oldExists := at.lastFlush["old-conn"]
	_, newExists := at.lastFlush["new-conn"]
	at.mu.Unlock()

	require.False(t, oldExists, "old conn should be cleaned up")
	require.True(t, newExists, "new conn should remain")
}

func Test_SSE_SetPaused_Callbacks(t *testing.T) {
	t.Parallel()

	var paused, resumed bool
	_, hub := NewWithHub(Config{
		OnPause:  func(_ *Connection) { paused = true },
		OnResume: func(_ *Connection) { resumed = true },
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("cb-conn", []string{"t"}, 10, time.Second)
	hub.mu.Lock()
	hub.connections["cb-conn"] = conn
	hub.mu.Unlock()

	hub.SetPaused("cb-conn", true)
	require.True(t, paused)

	hub.SetPaused("cb-conn", false)
	require.True(t, resumed)
}

func Test_SSE_RouteEvent_WithGroup(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Add two connections with different tenants
	conn1 := newConnection("c1", []string{"orders"}, 10, time.Second)
	conn1.Metadata["tenant_id"] = "t_1"
	conn2 := newConnection("c2", []string{"orders"}, 10, time.Second)
	conn2.Metadata["tenant_id"] = "t_2"

	hub.mu.Lock()
	hub.connections["c1"] = conn1
	hub.connections["c2"] = conn2
	hub.topicIndex["orders"] = map[string]struct{}{"c1": {}, "c2": {}}
	hub.mu.Unlock()

	// Publish with group targeting t_1 only
	hub.Publish(Event{
		Type:     "test",
		Topics:   []string{"orders"},
		Data:     "for-t1",
		Group:    map[string]string{"tenant_id": "t_1"},
		Priority: PriorityInstant,
	})

	time.Sleep(100 * time.Millisecond)

	// conn1 should have received the event, conn2 should not
	require.Equal(t, int64(0), conn1.MessagesDropped.Load())
	// Check send channel
	select {
	case me := <-conn1.send:
		require.Contains(t, me.Data, "for-t1")
	default:
		t.Fatal("expected event in conn1 send channel")
	}

	select {
	case <-conn2.send:
		t.Fatal("conn2 should NOT have received the event")
	default:
		// correct
	}
}

func Test_SSE_RouteEvent_GroupOnly(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Connection with metadata but no topic match — group-only delivery
	conn := newConnection("g1", []string{"unrelated"}, 10, time.Second)
	conn.Metadata["role"] = "admin"

	hub.mu.Lock()
	hub.connections["g1"] = conn
	hub.topicIndex["unrelated"] = map[string]struct{}{"g1": {}}
	hub.mu.Unlock()

	// Publish with group only (no topic overlap)
	hub.Publish(Event{
		Type:     "admin-alert",
		Data:     "alert",
		Group:    map[string]string{"role": "admin"},
		Priority: PriorityInstant,
	})

	time.Sleep(100 * time.Millisecond)

	select {
	case me := <-conn.send:
		require.Contains(t, me.Data, "alert")
	default:
		t.Fatal("expected event via group match")
	}
}

func Test_SSE_RouteEvent_WildcardConn(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("wc1", []string{"orders.*"}, 10, time.Second)

	hub.mu.Lock()
	hub.connections["wc1"] = conn
	hub.wildcardConns["wc1"] = struct{}{}
	hub.mu.Unlock()

	hub.Publish(Event{
		Type:     "test",
		Topics:   []string{"orders.created"},
		Data:     "wildcard-match",
		Priority: PriorityInstant,
	})

	time.Sleep(100 * time.Millisecond)

	select {
	case me := <-conn.send:
		require.Contains(t, me.Data, "wildcard-match")
	default:
		t.Fatal("wildcard connection should have received the event")
	}
}

func Test_SSE_RouteEvent_PausedSkipsNonInstant(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("p1", []string{"t"}, 10, time.Second)
	conn.paused.Store(true)

	hub.mu.Lock()
	hub.connections["p1"] = conn
	hub.topicIndex["t"] = map[string]struct{}{"p1": {}}
	hub.mu.Unlock()

	// P1 event should be skipped for paused connection
	hub.Publish(Event{
		Type:     "batch",
		Topics:   []string{"t"},
		Data:     "batched",
		Priority: PriorityBatched,
	})

	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 0, conn.dispatcher.pending())

	// P0 (instant) should still deliver
	hub.Publish(Event{
		Type:     "urgent",
		Topics:   []string{"t"},
		Data:     "instant",
		Priority: PriorityInstant,
	})

	time.Sleep(100 * time.Millisecond)

	select {
	case me := <-conn.send:
		require.Contains(t, me.Data, "instant")
	default:
		t.Fatal("P0 event should deliver to paused connection")
	}
}

func Test_SSE_RouteEvent_TTLExpired(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Publish an expired event
	hub.Publish(Event{
		Type:      "old",
		Topics:    []string{"t"},
		Data:      "expired",
		Priority:  PriorityInstant,
		TTL:       time.Millisecond,
		CreatedAt: time.Now().Add(-time.Second),
	})

	time.Sleep(100 * time.Millisecond)

	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsDropped)
}

func Test_SSE_DeliverToConn_AllPriorities(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("dc", []string{"t"}, 10, time.Second)

	me := MarshaledEvent{ID: "e1", Data: "test"}

	// Test instant delivery
	hub.deliverToConn(conn, &Event{Priority: PriorityInstant}, me)
	select {
	case <-conn.send:
	default:
		t.Fatal("instant event should be in send channel")
	}

	// Test batched delivery
	hub.deliverToConn(conn, &Event{Priority: PriorityBatched}, me)
	require.Equal(t, 1, conn.dispatcher.pending())
	conn.dispatcher.WriteTo()

	// Test coalesced delivery
	hub.deliverToConn(conn, &Event{Priority: PriorityCoalesced, Type: "progress", CoalesceKey: "k1"}, me)
	require.Equal(t, 1, conn.dispatcher.pending())
	conn.dispatcher.WriteTo()

	// Test coalesced without explicit key — uses Type
	hub.deliverToConn(conn, &Event{Priority: PriorityCoalesced, Type: "counter"}, me)
	require.Equal(t, 1, conn.dispatcher.pending())
}

func Test_SSE_FlushAll(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{FlushInterval: 50 * time.Millisecond})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("fl", []string{"t"}, 10, 50*time.Millisecond)

	hub.mu.Lock()
	hub.connections["fl"] = conn
	hub.topicIndex["t"] = map[string]struct{}{"fl": {}}
	hub.mu.Unlock()

	// Add batched events to the coalescer
	conn.dispatcher.AddEvent(MarshaledEvent{ID: "b1", Data: "batch1"})
	conn.dispatcher.AddEvent(MarshaledEvent{ID: "b2", Data: "batch2"})

	// Wait for throttler to allow flush, then flush
	time.Sleep(100 * time.Millisecond)
	hub.flushAll()

	// Events should now be in the send channel
	require.Len(t, conn.send, 2)
}

func Test_SSE_FlushAll_TTLExpiry(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{FlushInterval: 50 * time.Millisecond})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("fle", []string{"t"}, 10, 50*time.Millisecond)

	hub.mu.Lock()
	hub.connections["fle"] = conn
	hub.topicIndex["t"] = map[string]struct{}{"fle": {}}
	hub.mu.Unlock()

	// Add an expired event to the coalescer
	conn.dispatcher.AddEvent(MarshaledEvent{
		ID:        "exp",
		Data:      "expired",
		TTL:       time.Millisecond,
		CreatedAt: time.Now().Add(-time.Second),
	})

	time.Sleep(100 * time.Millisecond)
	hub.flushAll()

	// Event should be dropped, not delivered
	require.Empty(t, conn.send)
	require.Equal(t, int64(1), hub.metrics.eventsDropped.Load())
}

func Test_SSE_SendHeartbeats(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{HeartbeatInterval: 50 * time.Millisecond})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("hb", []string{"t"}, 10, time.Second)
	// Set lastWrite to long ago
	conn.lastWrite.Store(time.Now().Add(-time.Minute))

	hub.mu.Lock()
	hub.connections["hb"] = conn
	hub.mu.Unlock()

	hub.sendHeartbeats()

	// Should have a heartbeat pending
	select {
	case <-conn.heartbeat:
	default:
		t.Fatal("expected heartbeat")
	}
}

func Test_SSE_SendHeartbeats_SkipsClosed(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub(Config{HeartbeatInterval: 50 * time.Millisecond})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("closed-hb", []string{"t"}, 10, time.Second)
	conn.lastWrite.Store(time.Now().Add(-time.Minute))
	conn.Close()

	hub.mu.Lock()
	hub.connections["closed-hb"] = conn
	hub.mu.Unlock()

	// Should not panic
	hub.sendHeartbeats()
}

func Test_SSE_RemoveConnection(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("rm", []string{"orders", "products"}, 10, time.Second)

	hub.addConnection(conn)

	stats := hub.Stats()
	require.Equal(t, 1, stats.ActiveConnections)
	require.Equal(t, 2, stats.TotalTopics)

	hub.removeConnection(conn)

	stats = hub.Stats()
	require.Equal(t, 0, stats.ActiveConnections)
	require.Equal(t, 0, stats.TotalTopics)

	// Remove again should be no-op
	hub.removeConnection(conn)
}

func Test_SSE_RemoveConnection_Wildcard(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("rmw", []string{"orders.*"}, 10, time.Second)

	hub.addConnection(conn)

	hub.mu.RLock()
	_, hasWildcard := hub.wildcardConns["rmw"]
	hub.mu.RUnlock()
	require.True(t, hasWildcard)

	hub.removeConnection(conn)

	hub.mu.RLock()
	_, hasWildcard = hub.wildcardConns["rmw"]
	hub.mu.RUnlock()
	require.False(t, hasWildcard)
}

func Test_SSE_Publish_BufferFull(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Fill the event buffer (size 1024)
	for range 2000 {
		hub.Publish(Event{Type: "flood", Topics: []string{"t"}, Data: "x"})
	}

	time.Sleep(100 * time.Millisecond)
	stats := hub.Stats()
	// At least some events must have landed in the pipeline AND some must
	// have been dropped by the non-blocking `default:` branch in Publish.
	// Asserting only EventsPublished > 0 would let a regression that makes
	// Publish blocking pass silently — the drop counter is the actual
	// invariant this test exists to pin.
	require.Positive(t, stats.EventsPublished)
	require.Positive(t, stats.EventsDropped)
}

// testReplayer is a minimal in-memory Replayer implementation used by tests
// that exercise hub.replayEvents and hub.initStream. The production
// MemoryReplayer has been removed from the library surface.
type testReplayer struct {
	entries []testReplayEntry
	mu      sync.RWMutex
}

type testReplayEntry struct {
	topics []string
	event  MarshaledEvent
}

//nolint:gocritic // hugeParam: signature must match Replayer interface
func (r *testReplayer) Store(event MarshaledEvent, topics []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, testReplayEntry{event: event, topics: topics})
	return nil
}

func (r *testReplayer) Replay(lastEventID string, topics []string) ([]MarshaledEvent, error) {
	if lastEventID == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Find the index of lastEventID
	idx := -1
	for i, entry := range r.entries {
		if entry.event.ID == lastEventID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, nil
	}
	var out []MarshaledEvent
	for _, entry := range r.entries[idx+1:] {
		if topicsOverlap(entry.topics, topics) {
			out = append(out, entry.event)
		}
	}
	return out, nil
}

func (r *testReplayer) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func topicsOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if topicMatch(y, x) || topicMatch(x, y) || x == y {
				return true
			}
		}
	}
	return false
}

func Test_SSE_ReplayEvents(t *testing.T) {
	t.Parallel()

	replayer := &testReplayer{}
	require.NoError(t, replayer.Store(MarshaledEvent{ID: "r1", Data: "d1", Retry: -1}, []string{"t"}))
	require.NoError(t, replayer.Store(MarshaledEvent{ID: "r2", Data: "d2", Retry: -1}, []string{"t"}))

	_, hub := NewWithHub(Config{Replayer: replayer})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("replay-conn", []string{"t"}, 10, time.Second)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	err := hub.replayEvents(w, conn, "r1")
	require.NoError(t, err)
	require.Contains(t, buf.String(), "id: r2")
}

func Test_SSE_ReplayEvents_NoReplayer(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("no-replay", []string{"t"}, 10, time.Second)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	err := hub.replayEvents(w, conn, "some-id")
	require.NoError(t, err)
	require.Empty(t, buf.String())
}

func Test_SSE_InitStream(t *testing.T) {
	t.Parallel()

	replayer := &testReplayer{}
	require.NoError(t, replayer.Store(MarshaledEvent{ID: "i1", Data: "d1", Retry: -1}, []string{"t"}))
	require.NoError(t, replayer.Store(MarshaledEvent{ID: "i2", Data: "d2", Retry: -1}, []string{"t"}))

	_, hub := NewWithHub(Config{Replayer: replayer})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("init-conn", []string{"t"}, 10, time.Second)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	err := hub.initStream(w, conn, "i1")
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "retry: 3000")
	require.Contains(t, output, "id: i2")
	require.Contains(t, output, `event: connected`)
}

func Test_SSE_RouteEvent_ReplayerStore(t *testing.T) {
	t.Parallel()

	replayer := &testReplayer{}
	_, hub := NewWithHub(Config{Replayer: replayer})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Publish a non-group event — should be stored in replayer
	hub.Publish(Event{Type: "test", Topics: []string{"t"}, Data: "stored"})
	time.Sleep(100 * time.Millisecond)

	// Publish a group event — should NOT be stored in replayer
	hub.Publish(Event{
		Type:   "test",
		Topics: []string{"t"},
		Data:   "not-stored",
		Group:  map[string]string{"tenant_id": "t_1"},
	})
	time.Sleep(100 * time.Millisecond)

	// The replayer should only have 1 event (the non-group one)
	require.Equal(t, 1, replayer.count())
}

func Test_SSE_Shutdown_Timeout(t *testing.T) {
	// Not parallel. Previously this test used t.Parallel() and returned
	// without waiting for hub.run() to drain — run() is still alive inside
	// <-h.shutdown executing broadcastShutdown + time.Sleep(drainDelay)
	// (~200ms) and would outlive the test, mutating hub.connections under
	// mu.Lock concurrently with other parallel tests. Running serial and
	// awaiting hub.stopped eliminates that cross-test goroutine leak.

	_, hub := NewWithHub()

	// Pre-cancel context — Shutdown should surface ctx.Err().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := hub.Shutdown(ctx)
	require.Error(t, err, "Shutdown with canceled ctx must return an error")
	require.ErrorIs(t, err, context.Canceled)

	// Wait for the run loop to actually exit so we don't leak the
	// goroutine into subsequent tests. Bounded wait so a regression that
	// never closes `stopped` fails loudly.
	select {
	case <-hub.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("hub.run() did not exit after Shutdown with canceled ctx")
	}
}

func Benchmark_SSE_Publish(b *testing.B) {
	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(b, hub.Shutdown(ctx))
	}()

	event := Event{
		Type:   "test",
		Topics: []string{"benchmark"},
		Data:   "hello",
	}

	b.ResetTimer()
	for b.Loop() {
		hub.Publish(event)
	}
}

func Benchmark_SSE_TopicMatch(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		topicMatch("notifications.*", "notifications.orders")
	}
}

func Benchmark_SSE_TopicMatch_Exact(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		topicMatch("notifications.orders", "notifications.orders")
	}
}

func Benchmark_SSE_MarshalEvent(b *testing.B) {
	event := &Event{
		Type: "test",
		Data: map[string]string{"key": "value", "foo": "bar"},
	}

	b.ResetTimer()
	for b.Loop() {
		marshalEvent(event)
	}
}

func Benchmark_SSE_WriteTo(b *testing.B) {
	me := MarshaledEvent{
		ID:   "evt_1",
		Type: "test",
		Data: `{"key":"value"}`,
	}

	w := bufio.NewWriter(io.Discard)

	b.ResetTimer()
	for b.Loop() {
		me.WriteTo(w) //nolint:errcheck // benchmark: error irrelevant for perf measurement
	}
}

func Benchmark_SSE_Coalescer(b *testing.B) {
	c := newDispatcher(time.Second)
	me := MarshaledEvent{ID: "1", Data: "test"}

	b.ResetTimer()
	for b.Loop() {
		c.AddState("key", me)
		c.WriteTo()
	}
}

func Benchmark_SSE_GenerateID(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		generateID()
	}
}

// mockBridge implements SubscriberBridge for testing.
type mockBridge struct {
	onSubscribe func(ctx context.Context, channel string, onMessage func(string)) error
}

func (m *mockBridge) Subscribe(ctx context.Context, channel string, onMessage func(string)) error {
	return m.onSubscribe(ctx, channel, onMessage)
}

func Test_SSE_Bridge_Publishes(t *testing.T) {
	t.Parallel()

	delivered := make(chan string, 1)
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, _ string, onMessage func(string)) error {
			onMessage("test-payload")
			delivered <- "ok"
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{{
			Subscriber: bridge,
			Channel:    "test-channel",
			EventType:  "notification",
		}},
	})

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not deliver message in time")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))

	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsPublished)
}

func Test_SSE_Bridge_CancelsOnShutdown(t *testing.T) {
	t.Parallel()

	subscribed := make(chan struct{}, 1)
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, _ string, _ func(string)) error {
			subscribed <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{{
			Subscriber: bridge,
			Channel:    "ch",
			EventType:  "evt",
		}},
	})

	select {
	case <-subscribed:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not subscribe in time")
	}
	// Shutdown should cancel the bridge context and wait for the goroutine.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_Bridge_Multiple(t *testing.T) {
	t.Parallel()

	delivered := make(chan struct{}, 2)
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, channel string, onMessage func(string)) error {
			onMessage("msg-from-" + channel)
			delivered <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{
			{Subscriber: bridge, Channel: "ch1", EventType: "e1"},
			{Subscriber: bridge, Channel: "ch2", EventType: "e2"},
		},
	})

	for range 2 {
		select {
		case <-delivered:
		case <-time.After(2 * time.Second):
			t.Fatal("bridge did not deliver both messages in time")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))

	stats := hub.Stats()
	require.Equal(t, int64(2), stats.EventsPublished)
}

func Test_SSE_Bridge_Transform(t *testing.T) {
	t.Parallel()

	done := make(chan struct{}, 1)
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, _ string, onMessage func(string)) error {
			onMessage("raw-data")
			done <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{{
			Subscriber: bridge,
			Channel:    "ch",
			EventType:  "default",
			Transform: func(payload string) *Event {
				return &Event{
					Type:   "transformed",
					Data:   "transformed:" + payload,
					Topics: []string{"custom-topic"},
				}
			},
		}},
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not deliver in time")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))

	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsPublished)
}

func Test_SSE_Bridge_TransformNilSkipsMessage(t *testing.T) {
	t.Parallel()

	done := make(chan struct{}, 1)
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, _ string, onMessage func(string)) error {
			onMessage("skip-this")
			done <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{{
			Subscriber: bridge,
			Channel:    "ch",
			EventType:  "evt",
			Transform: func(_ string) *Event {
				return nil
			},
		}},
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not deliver in time")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))

	stats := hub.Stats()
	require.Equal(t, int64(0), stats.EventsPublished)
}

func Test_SSE_Bridge_RetriesOnError(t *testing.T) {
	// Cannot run in parallel — we mutate the package-level bridgeRetryDelay
	// so the retry loop is deterministic within the test's time budget.
	original := bridgeRetryDelay
	bridgeRetryDelay = 20 * time.Millisecond
	t.Cleanup(func() { bridgeRetryDelay = original })

	var attempts atomic.Int32
	secondAttemptBlocked := make(chan struct{})
	bridge := &mockBridge{
		onSubscribe: func(ctx context.Context, _ string, _ func(string)) error {
			n := attempts.Add(1)
			if n < 2 {
				return errors.New("transient error")
			}
			// Second attempt reached — signal the test and block until
			// Shutdown cancels the ctx. This proves the loop retried
			// past the error rather than exiting.
			close(secondAttemptBlocked)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	_, hub := NewWithHub(Config{
		Bridges: []BridgeConfig{{
			Subscriber: bridge,
			Channel:    "ch",
			EventType:  "e",
		}},
	})

	// Wait for the retry to actually happen. Before reaching here,
	// attempts must be exactly 2: one error + one in-progress call.
	select {
	case <-secondAttemptBlocked:
	case <-time.After(2 * time.Second):
		t.Fatalf("bridge did not retry after error (attempts=%d)", attempts.Load())
	}
	require.Equal(t, int32(2), attempts.Load(), "expected exactly 2 Subscribe calls")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_Bridge_BuildEvent_Defaults(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// Non-transform: event built entirely from config defaults.
	cfg := &BridgeConfig{
		Channel:     "ch",
		EventType:   "my-event",
		CoalesceKey: "k",
		TTL:         5 * time.Second,
		Priority:    PriorityCoalesced,
	}
	event := hub.buildBridgeEvent(cfg, "my-topic", "payload")
	require.NotNil(t, event)
	require.Equal(t, "my-event", event.Type)
	require.Equal(t, "payload", event.Data)
	require.Equal(t, []string{"my-topic"}, event.Topics)
	require.Equal(t, PriorityCoalesced, event.Priority)
	require.Equal(t, "k", event.CoalesceKey)
	require.Equal(t, 5*time.Second, event.TTL)

	// Transform path: only missing Topics/Type filled from defaults.
	cfgT := &BridgeConfig{
		EventType: "fallback-type",
		Transform: func(_ string) *Event {
			return &Event{Priority: PriorityInstant, Data: "x"}
		},
	}
	event = hub.buildBridgeEvent(cfgT, "fallback-topic", "raw")
	require.NotNil(t, event)
	require.Equal(t, "fallback-type", event.Type)
	require.Equal(t, []string{"fallback-topic"}, event.Topics)
	require.Equal(t, PriorityInstant, event.Priority)

	// Transform nil filters message.
	cfgT2 := &BridgeConfig{
		EventType: "x",
		Transform: func(_ string) *Event { return nil },
	}
	event = hub.buildBridgeEvent(cfgT2, "default-topic", "x")
	require.Nil(t, event)
}

func Test_SSE_Bridge_PanicsWithoutSubscriber(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		_, _ = NewWithHub(Config{
			Bridges: []BridgeConfig{{
				Channel:   "ch",
				EventType: "e",
			}},
		})
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Coverage boosters — targeted tests for previously-uncovered branches.
// ──────────────────────────────────────────────────────────────────────────────

func Test_SSE_Publish_DropsDuringDrain(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()

	// Flip the drain flag and Publish — event must be counted as dropped,
	// not published. Exercises the early-return branch in Publish.
	hub.draining.Store(true)
	hub.Publish(Event{Type: "x", Topics: []string{"t"}, Data: "d"})

	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsDropped)
	require.Equal(t, int64(0), stats.EventsPublished)

	hub.draining.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, hub.Shutdown(ctx))
}

func Test_SSE_Publish_StampsCreatedAtWhenTTLSet(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// TTL > 0 with zero CreatedAt — Publish must stamp CreatedAt so
	// routeEvent can compute age correctly.
	before := time.Now()
	hub.Publish(Event{
		Type:   "x",
		Topics: []string{"t"},
		Data:   "d",
		TTL:    time.Second,
	})
	time.Sleep(30 * time.Millisecond)

	// No direct getter for the enqueued event; just assert the
	// corresponding counter to ensure we hit the enqueue branch.
	stats := hub.Stats()
	require.Equal(t, int64(1), stats.EventsPublished)
	require.Less(t, time.Since(before), time.Second)
}

func Test_SSE_WriteRetry_SkipsNonPositive(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeRetry(&buf, 0))
	require.NoError(t, writeRetry(&buf, -42))
	require.Empty(t, buf.String(), "non-positive ms must not emit a retry: directive")

	require.NoError(t, writeRetry(&buf, 1500))
	require.Contains(t, buf.String(), "retry: 1500\n")
}

func Test_SSE_TrackEventType_EmptyDefaultsToMessage(t *testing.T) {
	t.Parallel()

	m := &hubMetrics{eventsByType: make(map[string]*atomic.Int64)}
	m.trackEventType("")
	m.trackEventType("")
	m.trackEventType("custom")

	snap := m.snapshotEventsByType()
	require.Equal(t, int64(2), snap["message"], "empty event type falls back to \"message\"")
	require.Equal(t, int64(1), snap["custom"])
}

func Test_SSE_MatchGroupConns_EmptyGroupIsNoOp(t *testing.T) {
	t.Parallel()

	_, hub := NewWithHub()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	// With no Group set on the event, matchGroupConns must short-circuit
	// without scanning connections — the early-return branch.
	seen := make(map[string]struct{})
	hub.mu.RLock()
	hub.matchGroupConns(&Event{Type: "x", Topics: []string{"t"}}, seen)
	hub.mu.RUnlock()
	require.Empty(t, seen)
}

func Test_SSE_WatchLifetime_NoOpWhenDisabled(t *testing.T) {
	t.Parallel()

	// MaxLifetime <= 0 must leave watchLifetime as a no-op (no goroutine
	// spawned, no eventual Close on the connection).
	_, hub := NewWithHub(Config{MaxLifetime: -1})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, hub.Shutdown(ctx))
	}()

	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	hub.watchLifetime(conn)

	// If watchLifetime spawned a goroutine it would close `conn.done`
	// eventually (with MaxLifetime<=0 it must NOT). Allow plenty of
	// scheduler time then assert conn is still alive.
	time.Sleep(50 * time.Millisecond)
	require.False(t, conn.IsClosed(), "watchLifetime must not close the conn when MaxLifetime<=0")
}

func Test_SSE_ReplayEvents_NoReplayerOrEmptyLastEventID(t *testing.T) {
	t.Parallel()

	// nil Replayer OR empty Last-Event-ID — both branches return nil
	// without touching the writer.
	hub := &Hub{cfg: Config{Replayer: nil}}
	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	var buf bytes.Buffer
	require.NoError(t, hub.replayEvents(bufio.NewWriter(&buf), conn, "some-id"))
	require.Empty(t, buf.String())

	hub2 := &Hub{cfg: Config{Replayer: &testReplayer{}}}
	require.NoError(t, hub2.replayEvents(bufio.NewWriter(&buf), conn, ""))
	require.Empty(t, buf.String())
}

// failingWriter writes `limit` bytes successfully then returns errWrite on
// subsequent writes. Used to hit the error branches in initStream, replayEvents,
// sendConnectedEvent, and writeLoop without spinning up a real TCP listener.
type failingWriter struct {
	err     error
	written int
	limit   int
}

func (fw *failingWriter) Write(p []byte) (int, error) {
	if fw.err != nil && fw.written >= fw.limit {
		return 0, fw.err
	}
	fw.written += len(p)
	return len(p), nil
}

func Test_SSE_InitStream_PropagatesWriteErrors(t *testing.T) {
	t.Parallel()

	// Fail on the very first write so writeRetry returns an error —
	// exercises initStream's first `if err != nil { return err }` branch.
	hub := &Hub{cfg: Config{RetryMS: 3000}}
	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	fw := &failingWriter{limit: 0, err: errors.New("forced write error")}
	err := hub.initStream(bufio.NewWriter(fw), conn, "")
	require.Error(t, err)
}

func Test_SSE_ReplayEvents_HandlesReplayerError(t *testing.T) {
	t.Parallel()

	// Replayer returning an error must be treated as best-effort — caller
	// gets nil and no events are written.
	hub := &Hub{cfg: Config{Replayer: &errorReplayer{err: errors.New("store down")}}}
	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	var buf bytes.Buffer
	require.NoError(t, hub.replayEvents(bufio.NewWriter(&buf), conn, "last-id"))
	require.Empty(t, buf.String())
}

type errorReplayer struct{ err error }

func (*errorReplayer) Store(MarshaledEvent, []string) error { return nil }
func (e *errorReplayer) Replay(string, []string) ([]MarshaledEvent, error) {
	return nil, e.err
}

func Test_SSE_ReplayEvents_WritesEventsAndFlushes(t *testing.T) {
	t.Parallel()

	// Replayer returning events must produce written frames terminated
	// with a flush — exercises the write-and-flush branch.
	r := &testReplayer{}
	// testReplayer.Replay returns entries AFTER the lastEventID marker
	// entry — store the marker first, then the two we want replayed.
	require.NoError(t, r.Store(MarshaledEvent{ID: "last", Type: "test", Data: "anchor"}, []string{"t"}))
	require.NoError(t, r.Store(MarshaledEvent{ID: "e1", Type: "test", Data: "one"}, []string{"t"}))
	require.NoError(t, r.Store(MarshaledEvent{ID: "e2", Type: "test", Data: "two"}, []string{"t"}))

	hub := &Hub{cfg: Config{Replayer: r}}
	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	require.NoError(t, hub.replayEvents(bw, conn, "last"))
	require.NoError(t, bw.Flush())
	out := buf.String()
	require.Contains(t, out, "id: e1\n")
	require.Contains(t, out, "id: e2\n")
	require.Contains(t, out, "data: one\n")
	require.Contains(t, out, "data: two\n")
}

func Test_SSE_SendConnectedEvent_PropagatesWriteError(t *testing.T) {
	t.Parallel()

	conn := newConnection("c1", []string{"t"}, 8, 100*time.Millisecond)
	fw := &failingWriter{limit: 0, err: errors.New("no space")}
	err := sendConnectedEvent(bufio.NewWriter(fw), conn)
	require.Error(t, err)
}

func Test_SSE_WriteLoop_HeartbeatFlush(t *testing.T) {
	t.Parallel()

	// Drive writeLoop: fire a heartbeat then a real event, then close,
	// so we hit the heartbeat branch, the normal event branch, and the
	// done-exit branch — previously uncovered paths in writeLoop.
	conn := newConnection("c1", []string{"t"}, 8, 50*time.Millisecond)
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	done := make(chan struct{})
	go func() {
		conn.writeLoop(bw)
		close(done)
	}()

	conn.sendHeartbeat()
	// Heartbeat fires, wait briefly for flush.
	time.Sleep(30 * time.Millisecond)

	conn.trySend(MarshaledEvent{ID: "evt_x", Type: "test", Data: "hi"})
	time.Sleep(30 * time.Millisecond)

	conn.Close()
	<-done

	require.NoError(t, bw.Flush())
	out := buf.String()
	require.Contains(t, out, ": heartbeat\n", "heartbeat comment present")
	require.Contains(t, out, "data: hi\n", "real event data present")
	require.Equal(t, int64(1), conn.MessagesSent.Load(), "real event counted once")
	require.Equal(t, "evt_x", conn.LastEventID.Load())
}
