package client

import (
	"net"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

type testServer struct {
	app *fiber.App
	ch  chan struct{}
	ln  *fasthttputil.InmemoryListener
	tb  testing.TB
}

func startTestServer(tb testing.TB, beforeStarting func(app *fiber.App)) *testServer {
	tb.Helper()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{
		CBOREncoder: cbor.Marshal,
		CBORDecoder: cbor.Unmarshal,
	})

	if beforeStarting != nil {
		beforeStarting(app)
	}

	ch := make(chan struct{})
	go func() {
		err := app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
		assert.NoError(tb, err)

		close(ch)
	}()

	return &testServer{
		app: app,
		ch:  ch,
		ln:  ln,
		tb:  tb,
	}
}

func (ts *testServer) stop() {
	ts.tb.Helper()

	if err := ts.app.Shutdown(); err != nil {
		ts.tb.Fatal(err)
	}

	select {
	case <-ts.ch:
	case <-time.After(time.Second):
		ts.tb.Fatalf("timeout when waiting for server close")
	}
}

func (ts *testServer) dial() func(addr string) (net.Conn, error) {
	ts.tb.Helper()

	return func(_ string) (net.Conn, error) {
		return ts.ln.Dial()
	}
}

// startRedirectServer redirects /start off example.com and /same-host within it.
func startRedirectServer(tb testing.TB, collect fiber.Handler) *testServer {
	tb.Helper()

	return startTestServer(tb, func(app *fiber.App) {
		app.Get("/start", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://attacker.example/collect")
		})
		app.Get("/same-host", func(c fiber.Ctx) error {
			return c.Redirect().Status(fiber.StatusFound).To("http://example.com/collect")
		})
		app.Get("/collect", collect)
	})
}

func requireNameList(t *testing.T, set func(names ...string), get func() []string) {
	t.Helper()

	require.Nil(t, get(), "none by default")

	names := []string{" X-Api-Key", "", "X-Auth-Token\t"}
	set(names...)
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get())

	names[0] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get(), "the caller's slice must not reach the list")

	got := get()
	got[0] = "X-Changed"
	require.Equal(t, []string{"X-Api-Key", "X-Auth-Token"}, get(), "the getter returns a copy")

	set("X-One")
	set("X-Two")
	require.Equal(t, []string{"X-Two"}, get(), "a second call replaces the list")

	set()
	require.Nil(t, get(), "no names clears it")

	set(" ", "")
	require.Nil(t, get(), "only blanks leave nothing")
}

func createHelperServer(tb testing.TB) (app *fiber.App, dial func(addr string) (net.Conn, error), start func()) { //nolint:nonamedreturns // gocritic unnamedResult requires explicit result identifiers for helper components
	tb.Helper()

	ln := fasthttputil.NewInmemoryListener()

	app = fiber.New()

	dial = func(_ string) (net.Conn, error) {
		return ln.Dial()
	}
	start = func() {
		require.NoError(tb, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
	}

	return app, dial, start
}

func testRequest(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted string, count ...int) {
	t.Helper()

	app, ln, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	client := New().SetDial(ln)

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		resp, err := req.Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, excepted, resp.String())
		resp.Close()
	}
}

func testRequestFail(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted error, count ...int) {
	t.Helper()

	app, ln, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	client := New().SetDial(ln)

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		_, err := req.Get("http://example.com")

		require.Equal(t, excepted.Error(), err.Error())
	}
}

func testClient(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Client), excepted string, count ...int) { //nolint:unparam // maybe needed
	t.Helper()

	app, ln, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		client := New().SetDial(ln)
		wrapAgent(client)

		resp, err := client.Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, excepted, resp.String())
		resp.Close()
	}
}
