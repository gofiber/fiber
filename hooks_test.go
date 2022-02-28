package fiber

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
	"github.com/gofiber/fiber/v2/utils"
)

var testSimpleHandler = func(c *Ctx) error {
	return c.SendString("simple")
}

func Test_Hook_OnRoute(t *testing.T) {
	t.Parallel()

	app := New()

	app.Hooks().OnRoute(func(c *Ctx, m Map) error {
		utils.AssertEqual(t, "", m["route"].(Route).Name)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Mount("/sub", subApp)
}

func Test_Hook_OnName(t *testing.T) {
	t.Parallel()

	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnName(func(c *Ctx, m Map) error {
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

func Test_Hook_OnName_Error(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "unknown error", fmt.Sprintf("%v", err))
		}
	}()

	app.Hooks().OnName(func(c *Ctx, m Map) error {
		return errors.New("unknown error")
	})

	app.Get("/", testSimpleHandler).Name("index")
}

func Test_Hook_OnGroupName(t *testing.T) {
	t.Parallel()

	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnGroupName(func(c *Ctx, m Map) error {
		buf.WriteString(m["group"].(Group).name)

		return nil
	})

	grp := app.Group("/x").Name("x.")
	grp.Get("/test", testSimpleHandler)
	grp.Get("/test2", testSimpleHandler)

	utils.AssertEqual(t, "x.", buf.String())
}

func Test_Hook_OnGroupName_Error(t *testing.T) {
	t.Parallel()

	app := New()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "unknown error", fmt.Sprintf("%v", err))
		}
	}()

	app.Hooks().OnGroupName(func(c *Ctx, m Map) error {
		return errors.New("unknown error")
	})

	grp := app.Group("/x").Name("x.")
	grp.Get("/test", testSimpleHandler)
}

func Test_Hook_OnShutdown(t *testing.T) {
	t.Parallel()

	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnShutdown(func(c *Ctx, m Map) error {
		buf.WriteString("shutdowning")

		return nil
	})

	utils.AssertEqual(t, nil, app.Shutdown())
	utils.AssertEqual(t, "shutdowning", buf.String())
}

func Test_Hook_OnListen(t *testing.T) {
	t.Parallel()

	app := New(Config{
		DisableStartupMessage: true,
	})

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnListen(func(c *Ctx, m Map) error {
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
