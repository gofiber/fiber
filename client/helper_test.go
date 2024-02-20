package client

import (
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

func createHelperServer(t testing.TB) (*fiber.App, func(addr string) (net.Conn, error), func()) {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()

	app := fiber.New()

	return app, func(addr string) (net.Conn, error) {
			return ln.Dial()
		}, func() {
			require.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
		}
	// TODO: add closer fn
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

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetDial(ln)
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

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetDial(ln)
		wrapAgent(req)

		_, err := req.Get("http://example.com")

		require.Equal(t, excepted.Error(), err.Error())
	}
}

func testClient(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Client), excepted string, count ...int) {
	t.Helper()

	app, ln, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		client := AcquireClient()
		wrapAgent(client)

		resp, err := client.Get("http://example.com", Config{Dial: ln})

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, excepted, resp.String())
		resp.Close()
	}
}
