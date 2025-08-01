# Server-Sent Events (SSE) Example for Fiber v3

This example demonstrates how to implement Server-Sent Events (SSE) in Fiber v3 using the `SendStreamWriter()` method.

## What is Server-Sent Events (SSE)?

Server-Sent Events is a web standard that allows a web server to send data to a client in real-time. Unlike WebSockets, SSE is unidirectional (server to client only) and uses standard HTTP connections.

## Features Demonstrated

- ✅ Basic SSE streaming with `SendStreamWriter()`
- ✅ Proper SSE headers configuration
- ✅ Different event types with custom names
- ✅ Event IDs for client reconnection
- ✅ Client disconnection detection
- ✅ Long-lived streaming connections
- ✅ Interactive HTML demo client

## Why Use `SendStreamWriter()` Instead of `io.Pipe()`?

In Fiber v3, `SendStreamWriter()` is the recommended approach for SSE because:

1. **Non-blocking**: No need for goroutines unlike `io.Pipe()`
2. **Better control**: Direct access to the buffered writer with `Flush()` control
3. **Efficient**: Optimized for streaming scenarios
4. **Client detection**: `w.Flush()` returns an error when clients disconnect

## Files

- `main.go` - Complete SSE server implementation
- `sse_demo.html` - Interactive HTML client for testing
- `go.mod` - Module dependencies

## Running the Example

1. Start the server:
   ```bash
   go run main.go
   ```

2. Open your browser and visit:
   ```
   http://localhost:3000/sse_demo.html
   ```

3. Click the "Connect" buttons to start receiving events

## API Endpoints

- `GET /events` - Basic SSE events (10 events, 1 per second)
- `GET /typed-events` - Typed SSE events with different event names
- `GET /infinite-events` - Infinite SSE stream (every 2 seconds)
- `GET /health` - Health check endpoint

## SSE Message Format

SSE messages follow this format:
```
data: Your message content here

event: custom-event-name
data: Message with custom event type

id: 123
event: notification
data: Message with ID for client reconnection

```

## Key Implementation Points

### 1. Required Headers

```go
c.Set("Content-Type", "text/event-stream")
c.Set("Cache-Control", "no-cache")
c.Set("Connection", "keep-alive")
c.Set("Access-Control-Allow-Origin", "*") // For CORS if needed
```

### 2. Using SendStreamWriter

```go
return c.SendStreamWriter(func(w *bufio.Writer) {
    // Send data
    fmt.Fprintf(w, "data: Your message\n\n")
    
    // Flush to send immediately
    if err := w.Flush(); err != nil {
        // Client disconnected
        return
    }
})
```

### 3. Different Event Types

```go
// Default event
fmt.Fprintf(w, "data: Hello World\n\n")

// Custom event type
fmt.Fprintf(w, "event: notification\ndata: Custom event\n\n")

// Event with ID for reconnection
fmt.Fprintf(w, "id: 123\nevent: update\ndata: Status update\n\n")
```

### 4. Client Disconnection Handling

```go
if err := w.Flush(); err != nil {
    log.Printf("Client disconnected: %v", err)
    return // Exit the stream writer function
}
```

## Browser Client Example

```javascript
const eventSource = new EventSource('/events');

eventSource.onopen = function(event) {
    console.log('Connected to SSE');
};

eventSource.onmessage = function(event) {
    console.log('Received:', event.data);
};

eventSource.addEventListener('notification', function(event) {
    console.log('Notification:', event.data);
});

eventSource.onerror = function(event) {
    console.log('SSE error:', event);
};

// Close connection
eventSource.close();
```

## Advantages Over v2 Approach

This Fiber v3 approach with `SendStreamWriter()` is superior to the v2 `io.Pipe()` pattern because:

- **Simpler code**: No need for goroutines and channels
- **Better performance**: Direct streaming without intermediate pipes
- **Automatic cleanup**: Function scope handles resource cleanup
- **Error handling**: Built-in client disconnection detection
- **Memory efficient**: Buffered writing with controlled flushing

## Use Cases

- Real-time notifications
- Live data feeds (stock prices, metrics)
- Chat applications (server-to-client messages)
- Progress updates for long-running tasks
- Live logs streaming
- Real-time collaboration features