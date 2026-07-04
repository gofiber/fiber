package aigateway

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// testHTTPClient avoids the forbidden http.DefaultClient and keeps streaming
// connections isolated per suite run.
var testHTTPClient = &http.Client{}

// sseUpstream serves an SSE endpoint that emits count chunks, sleeping gap
// between them, with usage in the final data chunk.
func sseUpstream(t *testing.T, count int, gap time.Duration) string {
	t.Helper()

	app := fiber.New()
	app.Post("/v1/chat/completions", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/event-stream")
		c.Set(fiber.HeaderCacheControl, "no-cache")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			for i := range count {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"tok%d\"}}],\"usage\":null}\n\n", i)
				if err := w.Flush(); err != nil {
					return
				}
				time.Sleep(gap)
			}
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":8,\"total_tokens\":12}}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			_ = w.Flush() //nolint:errcheck // client may be gone
		})
	})

	return "http://" + startServer(t, app)
}

func Test_AIGateway_StreamingRelay(t *testing.T) {
	t.Parallel()

	const chunks = 4
	const gap = 150 * time.Millisecond
	upstream := sseUpstream(t, chunks, gap)

	usageCh := make(chan *UsageEvent, 1)
	app := fiber.New()
	app.Use(New(Config{
		Upstreams: []Upstream{{Name: "sse", URL: upstream, Key: "sk"}},
		OnUsage:   func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, app)

	req, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	start := time.Now()
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.True(t, strings.HasPrefix(resp.Header.Get(fiber.HeaderContentType), "text/event-stream"))

	// Read line by line, recording when the first token line arrived.
	reader := bufio.NewReader(resp.Body)
	var firstTokenAt time.Duration
	var lines []string
	for {
		line, rerr := reader.ReadString('\n')
		if line != "" {
			if firstTokenAt == 0 && strings.Contains(line, "tok0") {
				firstTokenAt = time.Since(start)
			}
			lines = append(lines, line)
		}
		if rerr != nil {
			require.ErrorIs(t, rerr, io.EOF)
			break
		}
	}
	total := time.Since(start)

	// The full stream takes at least chunks*gap; the first token must have
	// arrived while later chunks were still pending — i.e. the gateway did
	// not buffer the stream.
	require.GreaterOrEqual(t, total, time.Duration(chunks)*gap)
	require.Positive(t, firstTokenAt)
	require.Less(t, firstTokenAt, time.Duration(chunks-1)*gap,
		"first SSE chunk should arrive before the upstream finished streaming")

	joined := strings.Join(lines, "")
	require.Contains(t, joined, "tok0")
	require.Contains(t, joined, fmt.Sprintf("tok%d", chunks-1))
	require.Contains(t, joined, "data: [DONE]")

	select {
	case ev := <-usageCh:
		require.True(t, ev.Streamed)
		require.Equal(t, fiber.StatusOK, ev.StatusCode)
		require.NoError(t, ev.Err)
		require.Positive(t, ev.ResponseBytes)
		require.NotNil(t, ev.Usage)
		require.Equal(t, 4, ev.Usage.InputTokens)
		require.Equal(t, 8, ev.Usage.OutputTokens)
		require.Equal(t, 12, ev.Usage.TotalTokens)
	case <-time.After(5 * time.Second):
		t.Fatal("usage hook did not fire after stream completion")
	}
}

func Test_AIGateway_StreamIdleTimeout(t *testing.T) {
	t.Parallel()

	// Upstream sends one chunk then stalls far beyond the idle timeout.
	app := fiber.New()
	app.Get("/v1/stall", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/event-stream")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			fmt.Fprint(w, "data: {\"first\":true}\n\n")
			if err := w.Flush(); err != nil {
				return
			}
			time.Sleep(3 * time.Second)
		})
	})
	upstream := "http://" + startServer(t, app)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams:         []Upstream{{Name: "stall", URL: upstream, Key: "sk"}},
		StreamIdleTimeout: 200 * time.Millisecond,
		OnUsage:           func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	req, err := http.NewRequest(http.MethodGet, "http://"+gwAddr+"/v1/stall", http.NoBody)
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")

	start := time.Now()
	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	body, _ := io.ReadAll(resp.Body) //nolint:errcheck // stream is expected to end abruptly
	elapsed := time.Since(start)

	// The first chunk got through, then the watchdog cut the stream well
	// before the upstream's 3s stall completed.
	require.Contains(t, string(body), "first")
	require.Less(t, elapsed, 2*time.Second)

	select {
	case ev := <-usageCh:
		require.True(t, ev.Streamed)
		require.ErrorIs(t, ev.Err, errStreamIdleTimeout)
	case <-time.After(5 * time.Second):
		t.Fatal("usage hook did not fire after idle timeout")
	}
}

func Test_AIGateway_ClientDisconnectMidStream(t *testing.T) {
	t.Parallel()

	upstream := sseUpstream(t, 20, 100*time.Millisecond)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams: []Upstream{{Name: "sse", URL: upstream, Key: "sk"}},
		OnUsage:   func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	req, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)

	// Read the first chunk, then hang up mid-stream.
	buf := make([]byte, 256)
	_, err = resp.Body.Read(buf)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// The gateway must notice the disconnect (write/flush error), tear the
	// upstream down, and still fire the usage hook.
	select {
	case ev := <-usageCh:
		require.True(t, ev.Streamed)
		require.Error(t, ev.Err)
	case <-time.After(10 * time.Second):
		t.Fatal("usage hook did not fire after client disconnect")
	}
}

func Test_AIGateway_StreamMaxResponseSize(t *testing.T) {
	t.Parallel()

	upstream := sseUpstream(t, 50, 10*time.Millisecond)

	usageCh := make(chan *UsageEvent, 1)
	gw := fiber.New()
	gw.Use(New(Config{
		Upstreams:       []Upstream{{Name: "sse", URL: upstream, Key: "sk"}},
		MaxResponseSize: 300,
		OnUsage:         func(e *UsageEvent) { usageCh <- e },
	}))
	gwAddr := startServer(t, gw)

	req, err := http.NewRequest(http.MethodPost, "http://"+gwAddr+"/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	require.NoError(t, err)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer k")
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	body, _ := io.ReadAll(resp.Body) //nolint:errcheck // stream ends abruptly at the cap
	require.LessOrEqual(t, len(body), 300+4096, "stream should stop near the cap")

	select {
	case ev := <-usageCh:
		require.ErrorIs(t, ev.Err, errResponseTooLarge)
	case <-time.After(5 * time.Second):
		t.Fatal("usage hook did not fire after cap")
	}
}
