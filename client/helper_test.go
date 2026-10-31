package client

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type testServer struct {
	app *fiber.App
	ch  chan struct{}
	ln  *fasthttputil.InmemoryListener
	tb  testing.TB
}

// startTestServer serves the routes beforeStarting registers from an app on an
// in-memory listener. The app is configured with cfg when it is given.
func startTestServer(tb testing.TB, beforeStarting func(app *fiber.App), cfg ...fiber.Config) *testServer {
	tb.Helper()

	ln := fasthttputil.NewInmemoryListener()
	config := fiber.Config{CBOREncoder: cbor.Marshal, CBORDecoder: cbor.Unmarshal}
	if len(cfg) > 0 {
		config = cfg[0]
	}
	app := fiber.New(config)

	if beforeStarting != nil {
		beforeStarting(app)
	}

	ch := make(chan struct{})
	go func() {
		assert.NoError(tb, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))

		close(ch)
	}()

	return &testServer{app: app, ch: ch, ln: ln, tb: tb}
}

func (ts *testServer) stop() {
	ts.tb.Helper()

	if err := ts.app.Shutdown(); err != nil {
		ts.tb.Fatal(err)
	}

	apptest.Recv(ts.tb, ts.ch, time.Second, "timeout when waiting for server close")
}

func (ts *testServer) dial() func(addr string) (net.Conn, error) {
	ts.tb.Helper()

	return func(_ string) (net.Conn, error) { return ts.ln.Dial() }
}

// startRedirectServer redirects /start off example.com and /same-host within it,
// to a /collect that collect answers. The server stops when the test ends, after
// its parallel subtests.
func startRedirectServer(tb testing.TB, collect fiber.Handler) *testServer {
	tb.Helper()

	server := startTestServer(tb, func(app *fiber.App) {
		app.Get("/start", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://attacker.example/collect")
		})
		app.Get("/same-host", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://example.com/collect")
		})
		app.Get("/collect", collect)
	})
	tb.Cleanup(server.stop)

	return server
}

// requireNameList requires the setter and the getter of a list of header names
// on recv to keep it as the redirect settings promise: blanks and surrounding
// whitespace are dropped, the list is copied on the way in and out, a call
// replaces it, and the setter returns recv for chaining.
func requireNameList[T any](t *testing.T, recv T, set func(T, ...string) T, get func(T) []string) {
	t.Helper()

	require.Nil(t, get(recv), "none by default")

	names := []string{" X-Api-Key", "", "X-Auth-Token\t"}
	set(recv, names...)
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get(recv))

	names[0] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get(recv), "the caller's slice must not reach the list")

	got := get(recv)
	got[0] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get(recv), "the getter returns a copy")

	set(recv, "X-One")
	set(recv, "X-Two")
	require.Equal(t, []string{"X-Two"}, get(recv), "a second call replaces the list")

	set(recv)
	require.Nil(t, get(recv), "no names clears it")

	set(recv, " ", "")
	require.Nil(t, get(recv), "only blanks leave nothing")

	require.Same(t, recv, set(recv))
}

func createHelperServer(tb testing.TB) (app *fiber.App, dial func(addr string) (net.Conn, error), start func()) { //nolint:nonamedreturns // gocritic unnamedResult requires explicit result identifiers for helper components
	tb.Helper()

	ln := fasthttputil.NewInmemoryListener()

	app = fiber.New()

	dial = func(_ string) (net.Conn, error) { return ln.Dial() }
	start = func() { require.NoError(tb, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})) }

	return app, dial, start
}

// clientTestCServeGet serves handler for GET / from a fresh in-memory server and
// returns the dial function that reaches it.
func clientTestCServeGet(tb testing.TB, handler fiber.Handler) func(addr string) (net.Conn, error) {
	tb.Helper()

	app, dial, start := createHelperServer(tb)
	app.Get("/", handler)
	go start()

	return dial
}

// acceptSignalListener closes accepting the first time it is asked to accept,
// which is when the server has registered the listener.
type acceptSignalListener struct {
	net.Listener
	accepting chan struct{}
	once      sync.Once
}

func (l *acceptSignalListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.accepting) })
	return l.Listener.Accept() //nolint:wrapcheck // a transparent wrapper
}

// clientTestCServe serves handler from a fasthttp server on an in-memory
// listener and returns the dial function that reaches it. The server is shut
// down when the test ends, once Serve has returned.
func clientTestCServe(t *testing.T, handler fasthttp.RequestHandler) fasthttp.DialFunc {
	t.Helper()

	mem := fasthttputil.NewInmemoryListener()
	ln := &acceptSignalListener{Listener: mem, accepting: make(chan struct{})}
	server := &fasthttp.Server{Handler: handler}

	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, server.Serve(ln))
	}()
	// A test that expects the request to fail never reaches the server, so
	// without this Shutdown can run before Serve has registered its listener,
	// find nothing to close, and leave Serve blocked in Accept for good.
	<-ln.accepting
	t.Cleanup(func() {
		require.NoError(t, server.Shutdown())
		<-done
	})

	return func(string) (net.Conn, error) { return mem.Dial() }
}

// clientTestPooled returns a value from acquire that release returns to its pool
// when the test ends.
func clientTestPooled[T any](t *testing.T, acquire func() T, release func(T)) T {
	t.Helper()

	v := acquire()
	t.Cleanup(func() { release(v) })
	return v
}

// clientTestCReqResp returns a pooled fasthttp request and response that are
// released when the test ends.
func clientTestCReqResp(t *testing.T) (*fasthttp.Request, *fasthttp.Response) {
	t.Helper()

	return clientTestPooled(t, fasthttp.AcquireRequest, fasthttp.ReleaseRequest),
		clientTestPooled(t, fasthttp.AcquireResponse, fasthttp.ReleaseResponse)
}

// clientTestCRepeat is how many times the testRequest helpers send their
// request: the first of count, or once.
func clientTestCRepeat(count []int) int {
	if len(count) > 0 {
		return count[0]
	}

	return 1
}

func testRequest(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted string, count ...int) {
	t.Helper()

	ln := clientTestCServeGet(t, handler)
	client := New().SetDial(ln)

	for range clientTestCRepeat(count) {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		resp, err := req.Get("http://example.com")

		clientTestARequireOK(t, resp, err, excepted)
		resp.Close()
	}
}

func testRequestFail(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted error, count ...int) {
	t.Helper()

	ln := clientTestCServeGet(t, handler)
	client := New().SetDial(ln)

	for range clientTestCRepeat(count) {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		_, err := req.Get("http://example.com")

		require.Equal(t, excepted.Error(), err.Error())
	}
}

func testClient(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Client), excepted string, count ...int) { //nolint:unparam // maybe needed
	t.Helper()

	ln := clientTestCServeGet(t, handler)

	for range clientTestCRepeat(count) {
		client := New().SetDial(ln)
		wrapAgent(client)

		resp, err := client.Get("http://example.com")

		clientTestARequireOK(t, resp, err, excepted)
		resp.Close()
	}
}
