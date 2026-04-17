package sse

import (
	"bufio"
	"sync"
	"sync/atomic"
	"time"
)

// Connection represents a single SSE client connection managed by the hub.
type Connection struct {
	CreatedAt   time.Time
	LastEventID atomic.Value
	lastWrite   atomic.Value
	send        chan MarshaledEvent
	heartbeat   chan struct{}
	done        chan struct{}
	dispatcher  *dispatcher
	// Metadata holds connection metadata set during OnConnect.
	// It is frozen (defensive-copied) after OnConnect returns -- do not
	// mutate it from other goroutines after the connection is registered.
	Metadata        map[string]string
	ID              string
	Topics          []string
	MessagesSent    atomic.Int64
	MessagesDropped atomic.Int64
	once            sync.Once
	paused          atomic.Bool
}

// newConnection creates a Connection with the given buffer size.
func newConnection(id string, topics []string, bufferSize int, flushInterval time.Duration) *Connection {
	c := &Connection{
		ID:        id,
		Topics:    topics,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		send:      make(chan MarshaledEvent, bufferSize),
		heartbeat: make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	c.lastWrite.Store(time.Now())
	c.LastEventID.Store("")
	c.dispatcher = newDispatcher(flushInterval)
	return c
}

// Close terminates the connection. Safe to call multiple times.
func (c *Connection) Close() {
	c.once.Do(func() {
		close(c.done)
	})
}

// IsClosed returns true if the connection has been terminated.
func (c *Connection) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// trySend attempts to deliver an event to the connection's send channel.
// Returns false if the buffer is full (backpressure).
func (c *Connection) trySend(me MarshaledEvent) bool { //nolint:gocritic // hugeParam: value semantics for channel send
	select {
	case c.send <- me:
		return true
	default:
		c.MessagesDropped.Add(1)
		return false
	}
}

// sendHeartbeat sends a heartbeat signal to the connection.
// Non-blocking — if a heartbeat is already pending it is silently dropped.
func (c *Connection) sendHeartbeat() {
	select {
	case c.heartbeat <- struct{}{}:
	default:
	}
}

// writeLoop runs inside Fiber's SendStreamWriter. It reads from the send
// and heartbeat channels, writing SSE-formatted events to the bufio.Writer.
func (c *Connection) writeLoop(w *bufio.Writer) {
	for {
		select {
		case <-c.done:
			return
		case <-c.heartbeat:
			if err := writeComment(w, "heartbeat"); err != nil {
				c.Close()
				return
			}
			if err := w.Flush(); err != nil {
				c.Close()
				return
			}
		case me, ok := <-c.send:
			if !ok {
				return
			}
			if _, err := me.WriteTo(w); err != nil {
				c.Close()
				return
			}
			if err := w.Flush(); err != nil {
				c.Close()
				return
			}
			c.MessagesSent.Add(1)
			c.lastWrite.Store(time.Now())
			if me.ID != "" {
				c.LastEventID.Store(me.ID)
			}
		}
	}
}

// connMatchesGroup returns true if ALL key-value pairs in the group
// match the connection's metadata.
func connMatchesGroup(conn *Connection, group map[string]string) bool {
	for k, v := range group {
		if conn.Metadata[k] != v {
			return false
		}
	}
	return true
}
