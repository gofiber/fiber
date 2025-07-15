# Fiber v3 Server-Sent Events (SSE) Implementation

This implementation addresses issue #3510 by providing proper SSE examples for Fiber v3 using the recommended `SendStreamWriter()` method.

## Problem Solved

The issue requested working SSE examples for Fiber v3, as most existing examples were for v2 and used suboptimal approaches like `io.Pipe()`.

## Solution Summary

1. **Added comprehensive SSE tests** (`sse_test.go`):
   - Basic SSE functionality
   - Typed events with custom event names
   - Client disconnection handling
   - Performance benchmarks

2. **Enhanced documentation** (`docs/api/ctx.md`):
   - Added detailed SSE examples to `SendStreamWriter` section
   - Showed proper SSE header configuration
   - Demonstrated different event types and formats
   - Explained advantages over v2 approaches

3. **Created working examples** (`examples/sse/`):
   - Complete Go server implementation
   - Interactive HTML demo client
   - Multiple SSE endpoint patterns
   - Comprehensive README with explanations

## Key Improvements Over v2 Approach

### Before (v2 with io.Pipe - suboptimal):
```go
func Events(c fiber.Ctx) error {
    pr, pw := io.Pipe()
    
    c.Set("Content-Type", "text/event-stream")
    // ... headers
    
    go func() {  // Requires goroutine
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case t := <-ticker.C:
                fmt.Fprintf(pw, "data: Time is %s\n\n", t.Format(time.RFC3339))
                // ... error handling
            }
        }
    }()
    
    return c.SendStream(pr)  // Blocking pipe approach
}
```

### After (v3 with SendStreamWriter - optimal):
```go
func Events(c fiber.Ctx) error {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Access-Control-Allow-Origin", "*")

    return c.SendStreamWriter(func(w *bufio.Writer) {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case t := <-ticker.C:
                fmt.Fprintf(w, "data: Time is %s\n\n", t.Format(time.RFC3339))
                if err := w.Flush(); err != nil {
                    // Client disconnected
                    return
                }
            }
        }
    })
}
```

## Benefits of the New Approach

1. **No goroutines needed**: Direct streaming in the handler
2. **Better resource management**: Automatic cleanup on function exit
3. **Client disconnection detection**: `w.Flush()` returns error when client disconnects
4. **More efficient**: No intermediate pipes or channels
5. **Simpler code**: Less boilerplate and complexity
6. **Production-ready**: Handles edge cases and error conditions

## Files Added/Modified

- `sse_test.go` - Comprehensive test suite for SSE functionality
- `docs/api/ctx.md` - Enhanced documentation with SSE examples
- `examples/sse/main.go` - Complete working SSE server
- `examples/sse/sse_demo.html` - Interactive HTML demo client
- `examples/sse/README.md` - Detailed explanation and usage guide
- `examples/sse/go.mod` - Module configuration for the example

## Testing

All tests pass, including:
- Existing Fiber v3 test suite
- New SSE-specific tests
- SendStreamWriter functionality tests
- Performance benchmarks

The implementation provides a complete, production-ready solution for SSE in Fiber v3.