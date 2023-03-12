package fiber

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/bytebufferpool"
)

func testSimpleHandler(c *Ctx) error {
	return c.SendString("simple")
}

func Test_Hook_OnRoute(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnRoute(func(r Route) error {
		utils.AssertEqual(t, "", r.Name)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Mount("/sub", subApp)
}

func Test_Hook_OnRoute_Mount(t *testing.T) {
	t.Parallel()
	app := New()
	subApp := New()
	app.Mount("/sub", subApp)

	subApp.Hooks().OnRoute(func(r Route) error {
		utils.AssertEqual(t, "/sub/test", r.Path)

		return nil
	})

	app.Hooks().OnRoute(func(r Route) error {
		utils.AssertEqual(t, "/", r.Path)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("x")
	subApp.Get("/test", testSimpleHandler)
}

func Test_Hook_OnName(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnName(func(r Route) error {
		_, err := buf.WriteString(r.Name)
		utils.AssertEqual(t, nil, err)

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

	app.Hooks().OnName(func(r Route) error {
		return errors.New("unknown error")
	})

	app.Get("/", testSimpleHandler).Name("index")
}

func Test_Hook_OnGroup(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnGroup(func(g Group) error {
		_, err := buf.WriteString(g.Prefix)
		utils.AssertEqual(t, nil, err)
		return nil
	})

	grp := app.Group("/x").Name("x.")
	grp.Group("/a")

	utils.AssertEqual(t, "/x/x/a", buf.String())
}

func Test_Hook_OnGroup_Mount(t *testing.T) {
	t.Parallel()
	app := New()
	micro := New()
	micro.Mount("/john", app)

	app.Hooks().OnGroup(func(g Group) error {
		utils.AssertEqual(t, "/john/v1", g.Prefix)
		return nil
	})

	v1 := app.Group("/v1")
	v1.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})
}

func Test_Hook_OnGroupName(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnGroupName(func(g Group) error {
		_, err := buf.WriteString(g.name)
		utils.AssertEqual(t, nil, err)

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

	app.Hooks().OnGroupName(func(g Group) error {
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

	app.Hooks().OnShutdown(func() error {
		_, err := buf.WriteString("shutdowning")
		utils.AssertEqual(t, nil, err)

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

	app.Hooks().OnListen(func() error {
		_, err := buf.WriteString("ready")
		utils.AssertEqual(t, nil, err)

		return nil
	})

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()
	utils.AssertEqual(t, nil, app.Listen(":9000"))

	utils.AssertEqual(t, "ready", buf.String())
}

func Test_Hook_OnHook(t *testing.T) {
	app := New()

	// Reset test var
	testPreforkMaster = true
	testOnPrefork = true

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	app.Hooks().OnFork(func(pid int) error {
		utils.AssertEqual(t, 1, pid)
		return nil
	})

	utils.AssertEqual(t, nil, app.prefork(NetworkTCP4, ":3000", nil))
}

func Test_Hook_OnMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	subApp.Hooks().OnMount(func(parent *App) error {
		utils.AssertEqual(t, parent.mountFields.mountPath, "")

		return nil
	})

	app.Mount("/sub", subApp)
}
