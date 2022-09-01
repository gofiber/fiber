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

	app.Hooks().OnRoute(func(r Route) error {
		utils.AssertEqual(t, "", r.Name)

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
	// Reset test var
	testPreforkMaster = true
	testOnPrefork = true

	app := New()

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
