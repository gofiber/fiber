package fiber

import (
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/utils"
)

func Test_Hook_OnRoute(t *testing.T) {
	app := New()

	app.Hooks.OnRoute(func(c *Ctx, m Map) error {
		utils.AssertEqual(t, "", m["route"].(Route).Name)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Mount("/sub", subApp)
}

func Test_Hook_OnName(t *testing.T) {
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks.OnName(func(c *Ctx, m Map) error {
		buf.WriteString(m["route"].(Route).Name)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("index")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)
	subApp.Get("/test2", testSimpleHandler)

	app.Mount("/sub", subApp)

	utils.AssertEqual(t, "index", buf.String())
}

func Test_Hook_OnShutdown(t *testing.T) {
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks.OnShutdown(func(c *Ctx, m Map) error {
		buf.WriteString("shutdowning")

		return nil
	})

	utils.AssertEqual(t, nil, app.Shutdown())
	utils.AssertEqual(t, "shutdowning", buf.String())
}

func Test_Hook_OnRequest(t *testing.T) {
	app := New()

	app.Hooks.OnRequest(func(c *Ctx, m Map) error {
		return c.SendString("-")
	})

	app.Get("/", testSimpleHandler)

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Mount("/sub", subApp)

	resp, err := app.Test(httptest.NewRequest("GET", "/sub/test", nil))
	utils.AssertEqual(t, nil, err)

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "-", string(body))
}

func Test_Hook_OnResponse(t *testing.T) {
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks.OnResponse(func(c *Ctx, m Map) error {
		buf.WriteString(c.Path() + "-")

		return nil
	})

	app.Get("/", testSimpleHandler)

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Mount("/sub", subApp)

	_, err := app.Test(httptest.NewRequest("GET", "/sub/test", nil))
	utils.AssertEqual(t, nil, err)

	_, err = app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, "/sub/test-/-", buf.String())
}

func Test_Hook_OnListen(t *testing.T) {
	app := New(Config{
		DisableStartupMessage: true,
	})

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks.OnListen(func(c *Ctx, m Map) error {
		buf.WriteString("ready")

		return nil
	})

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()
	utils.AssertEqual(t, nil, app.Listen(":9000"))

	utils.AssertEqual(t, "ready", buf.String())
}
