package client

import (
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp/fasthttputil"
)

func createHelperServer(t *testing.T) (*fiber.App, func(addr string) (net.Conn, error), func()) {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	return app, func(addr string) (net.Conn, error) {
			return ln.Dial()
		}, func() {
			utils.AssertEqual(t, nil, app.Listener(ln))
		}
}

func testAgent(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted string, count ...int) {
	t.Parallel()

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

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, excepted, resp.String())
		resp.Close()
	}
}

func testAgentFail(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted error, count ...int) {
	t.Parallel()

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

		utils.AssertEqual(t, excepted.Error(), err.Error())
	}
}
