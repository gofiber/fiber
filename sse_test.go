package fiber

import (
	"bufio"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Test_SSE_Basic tests basic Server-Sent Events functionality
func Test_SSE_Basic(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/events", func(c Ctx) error {
		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send a few events
			for i := 1; i <= 3; i++ {
				fmt.Fprintf(w, "data: Event %d\n\n", i)
				if err := w.Flush(); err != nil {
					return
				}
				// Small delay to simulate real-time events
				time.Sleep(10 * time.Millisecond)
			}
		})
	})

	req := httptest.NewRequest(MethodGet, "/events", nil)
	resp, err := app.Test(req, TestConfig{
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Check headers
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	require.Equal(t, "keep-alive", resp.Header.Get("Connection"))

	// Read response body
	defer resp.Body.Close()
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	responseText := string(body[:n])

	// Verify SSE format
	require.Contains(t, responseText, "data: Event 1\n\n")
	require.Contains(t, responseText, "data: Event 2\n\n")
	require.Contains(t, responseText, "data: Event 3\n\n")
}

// Test_SSE_WithEventTypes tests SSE with different event types
func Test_SSE_WithEventTypes(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/typed-events", func(c Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send different event types
			fmt.Fprintf(w, "event: message\ndata: Hello World\n\n")
			if err := w.Flush(); err != nil {
				return
			}

			fmt.Fprintf(w, "event: notification\ndata: {\"type\":\"info\",\"message\":\"System ready\"}\n\n")
			if err := w.Flush(); err != nil {
				return
			}

			// Event with ID for client reconnection
			fmt.Fprintf(w, "id: 123\nevent: update\ndata: Status update\n\n")
			if err := w.Flush(); err != nil {
				return
			}
		})
	})

	req := httptest.NewRequest(MethodGet, "/typed-events", nil)
	resp, err := app.Test(req, TestConfig{
		Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Read response
	defer resp.Body.Close()
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	responseText := string(body[:n])

	// Verify different event types
	require.Contains(t, responseText, "event: message\ndata: Hello World\n\n")
	require.Contains(t, responseText, "event: notification\ndata: {\"type\":\"info\",\"message\":\"System ready\"}\n\n")
	require.Contains(t, responseText, "id: 123\nevent: update\ndata: Status update\n\n")
}

// Test_SSE_ClientDisconnect tests behavior when client disconnects
func Test_SSE_ClientDisconnect(t *testing.T) {
	t.Parallel()

	app := New()
	disconnected := make(chan bool, 1)
	
	app.Get("/long-events", func(c Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			for i := 1; i <= 100; i++ {
				fmt.Fprintf(w, "data: Event %d\n\n", i)
				if err := w.Flush(); err != nil {
					// Client disconnected
					disconnected <- true
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	})

	// This test demonstrates the pattern, but app.Test doesn't simulate real disconnection
	// In a real scenario, w.Flush() would return an error when client disconnects
	req := httptest.NewRequest(MethodGet, "/long-events", nil)
	resp, err := app.Test(req, TestConfig{
		Timeout: 200 * time.Millisecond,
	})
	
	// The test passes to show the basic structure works
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// Benchmark_SSE_Performance benchmarks SSE performance
func Benchmark_SSE_Performance(b *testing.B) {
	app := New()
	app.Get("/benchmark-events", func(c Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			for i := 1; i <= 10; i++ {
				fmt.Fprintf(w, "data: Benchmark event %d\n\n", i)
				if err := w.Flush(); err != nil {
					return
				}
			}
		})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(MethodGet, "/benchmark-events", nil)
			resp, err := app.Test(req, TestConfig{
				Timeout: 1 * time.Second,
			})
			if err != nil {
				b.Error(err)
				return
			}
			resp.Body.Close()
		}
	})
}

// ExampleCtx_SendStreamWriter_sse demonstrates how to use SSE in Fiber v3
func ExampleCtx_SendStreamWriter_sse() {
	app := New()

	// Basic SSE endpoint
	app.Get("/events", func(c Ctx) error {
		// Set required SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send periodic updates
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for i := 0; i < 5; i++ {
				select {
				case t := <-ticker.C:
					fmt.Fprintf(w, "data: Current time: %s\n\n", t.Format(time.RFC3339))
					if err := w.Flush(); err != nil {
						// Client disconnected
						return
					}
				}
			}
		})
	})

	// SSE with different event types
	app.Get("/typed-events", func(c Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Welcome message
			fmt.Fprintf(w, "event: welcome\ndata: Connected to server\n\n")
			if err := w.Flush(); err != nil {
				return
			}

			// Send periodic notifications
			for i := 1; i <= 3; i++ {
				fmt.Fprintf(w, "id: %d\nevent: notification\ndata: {\"count\": %d, \"message\": \"Update %d\"}\n\n", i, i, i)
				if err := w.Flush(); err != nil {
					return
				}
				time.Sleep(500 * time.Millisecond)
			}

			// Goodbye message
			fmt.Fprintf(w, "event: goodbye\ndata: Connection closing\n\n")
			w.Flush()
		})
	})

	app.Listen(":3000")
}