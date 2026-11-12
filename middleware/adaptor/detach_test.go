package adaptor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type localKey struct{}

// earlierKey names a value that a middleware ahead of the handler stored with c.Locals.
type earlierKey string

// streamReading is what a streaming handler read of its request before it flushed and after the client left.
type streamReading struct {
	localAfter                                    any    // what the context HTTPHandlerWithContext stores holds
	earlierAfter                                  string // the values c.Locals stored before the handler was reached
	authBefore, authAfter, pathBefore, pathAfter  string
	hostBefore, hostAfter, methodAfter, bodyAfter string
}

// detachServer serves a handler that streams until its client leaves and then waits to be released.
type detachServer struct {
	readings chan streamReading
	gone     chan struct{}
	release  chan struct{}
	addr     string
	once     sync.Once
}

func (s *detachServer) releaseHandler() { s.once.Do(func() { close(s.release) }) }

// newDetachServer serves, on a TCP listener, the handler register mounts on /stream/*. The handler answers a
// plain body, or, for a request with X-Stream, flushes and streams until a write fails, reports that the client
// is gone, waits for releaseHandler and then reads its request again.
func newDetachServer(t *testing.T, register func(app *fiber.App, h http.Handler)) *detachServer {
	t.Helper()
	s := &detachServer{readings: make(chan streamReading, 1), gone: make(chan struct{}), release: make(chan struct{})}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Stream") == "" {
			_, err := io.WriteString(w, "plain "+r.URL.Path)
			assert.NoError(t, err)
			return
		}
		got := streamReading{authBefore: r.Header.Get("Authorization"), pathBefore: r.URL.Path, hostBefore: r.Host}
		rc := http.NewResponseController(w)
		_, err := io.WriteString(w, "first chunk\n")
		assert.NoError(t, err)
		assert.NoError(t, rc.Flush())
		for {
			time.Sleep(5 * time.Millisecond)
			if _, werr := io.WriteString(w, "tick\n"); werr != nil || rc.Flush() != nil {
				break
			}
		}
		close(s.gone)
		<-s.release
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		got.authAfter, got.pathAfter, got.hostAfter, got.methodAfter, got.bodyAfter = r.Header.Get("Authorization"), r.URL.Path, r.Host, r.Method, string(body)
		if local, ok := LocalContextFromHTTPRequest(r); ok {
			got.localAfter = local.Value(localKey{})
		}
		got.earlierAfter = fmt.Sprint(r.Context().Value(earlierKey("a")), "|", r.Context().Value(earlierKey("b")))
		s.readings <- got
	})

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), localKey{}, "mine-"+c.Path()))
		// stored before the hold is: the hold has to keep them readable
		c.Locals(earlierKey("a"), "v1")
		c.Locals(earlierKey("b"), "v2")
		return c.Next()
	})
	register(app, h)
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }() //nolint:errcheck // stopped by the cleanup
	// Shutdown waits for the connection of a handler that is held back, so a test that fails before it releases
	// the handler would hang there: release it first.
	t.Cleanup(func() {
		s.releaseHandler()
		require.NoError(t, app.Shutdown())
	})
	s.addr = ln.Addr().String()
	return s
}

// Test_HTTPHandler_FlushedHandlerKeepsItsRequest pins that a handler that flushed reads its own request until it
// returns, whatever the server did with the connection and the request context in the meantime.
func Test_HTTPHandler_FlushedHandlerKeepsItsRequest(t *testing.T) {
	t.Parallel()
	for name, register := range map[string]func(app *fiber.App, h http.Handler){
		"HTTPHandler":            func(app *fiber.App, h http.Handler) { app.All("/stream/*", HTTPHandler(h)) },
		"HTTPHandlerFunc":        func(app *fiber.App, h http.Handler) { app.All("/stream/*", HTTPHandlerFunc(h.ServeHTTP)) },
		"HTTPHandlerWithContext": func(app *fiber.App, h http.Handler) { app.All("/stream/*", HTTPHandlerWithContext(h)) },
		"HTTPMiddleware": func(app *fiber.App, h http.Handler) {
			app.All("/stream/*", HTTPMiddleware(func(http.Handler) http.Handler { return h }))
		},
		"direct": func(app *fiber.App, h http.Handler) { app.All("/stream/*", h) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newDetachServer(t, register)

			// Client A starts a streaming response and leaves.
			a, err := net.Dial("tcp", s.addr)
			require.NoError(t, err)
			require.NoError(t, a.SetDeadline(time.Now().Add(10*time.Second)))
			const payload = "payload-a"
			_, err = fmt.Fprintf(a, "POST /stream/path-a HTTP/1.1\r\nHost: host-a.example\r\nAuthorization: Bearer token-a\r\nX-Stream: 1\r\nContent-Length: %d\r\n\r\n%s", len(payload), payload)
			require.NoError(t, err)
			br := bufio.NewReader(a)
			for {
				line, err := br.ReadString('\n')
				require.NoError(t, err)
				if line == "first chunk\n" {
					break
				}
			}
			require.NoError(t, a.Close())
			apptest.Recv(t, s.gone, 10*time.Second, "the handler did not notice that the client left")
			time.Sleep(50 * time.Millisecond)

			// Client B reuses the request contexts of the server.
			for i := range 50 {
				b, err := net.Dial("tcp", s.addr)
				require.NoError(t, err)
				require.NoError(t, b.SetDeadline(time.Now().Add(10*time.Second)))
				_, err = fmt.Fprintf(b, "GET /stream/path-b-longer-%d HTTP/1.1\r\nHost: host-b.example\r\nAuthorization: Bearer token-b-longer\r\nConnection: close\r\n\r\n", i)
				require.NoError(t, err)
				answer, err := io.ReadAll(b)
				require.NoError(t, err)
				require.True(t, strings.HasSuffix(string(answer), fmt.Sprintf("plain /stream/path-b-longer-%d", i)), string(answer))
				require.NoError(t, b.Close())
			}

			s.releaseHandler()
			// what the handler read before it flushed and what it reads now are the request it was given
			local := any(nil)
			if name == "HTTPHandlerWithContext" {
				local = "mine-/stream/path-a"
			}
			require.Equal(t, streamReading{
				authBefore: "Bearer token-a", authAfter: "Bearer token-a",
				pathBefore: "/stream/path-a", pathAfter: "/stream/path-a",
				hostBefore: "host-a.example", hostAfter: "host-a.example",
				methodAfter: "POST", bodyAfter: payload, localAfter: local, earlierAfter: "v1|v2",
			}, apptest.Recv(t, s.readings, 10*time.Second))
		})
	}
}

// goneWriter is the ResponseWriter of a client that has left: nothing written to it arrives.
type goneWriter struct{ header http.Header }

func (w *goneWriter) Header() http.Header     { return w.header }
func (*goneWriter) WriteHeader(int)           {}
func (*goneWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (*goneWriter) Flush()                    {}

// Test_FiberApp_FlushedHandlerKeepsItsCtxOutOfThePool pins that FiberApp does not hand the request context of a net/http
// handler that flushed and keeps running to another request: resetting it for that request would wait for the handler.
func Test_FiberApp_FlushedHandlerKeepsItsCtxOutOfThePool(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	app := fiber.New()
	app.Get("/stream", HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := io.WriteString(w, "first chunk\n")
		assert.NoError(t, err)
		assert.NoError(t, http.NewResponseController(w).Flush())
		<-release
	})))
	app.Get("/ping", func(c fiber.Ctx) error { return c.SendString("pong") })
	serve := FiberApp(app)

	// Both in one goroutine: the pool hands back what this one put into it.
	answered := make(chan string, 1)
	go func() {
		serve(&goneWriter{header: http.Header{}}, httptest.NewRequest(fiber.MethodGet, "/stream", http.NoBody))
		rec := httptest.NewRecorder()
		serve(rec, httptest.NewRequest(fiber.MethodGet, "/ping", http.NoBody))
		answered <- rec.Body.String()
	}()
	require.Equal(t, "pong", apptest.Recv(t, answered, 5*time.Second, "the next request waits for the handler that outlived its response"))
}
