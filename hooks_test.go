package fiber

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

const testMountPath = "/api"

func testSimpleHandler(c Ctx) error {
	return c.SendString("simple")
}

func Test_Hook_OnRoute(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnRoute(func(r Route) error {
		require.Equal(t, "", r.Name)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	app.Use("/sub", subApp)
}

func Test_Hook_OnRoute_Mount(t *testing.T) {
	t.Parallel()
	app := New()
	subApp := New()
	app.Use("/sub", subApp)

	subApp.Hooks().OnRoute(func(r Route) error {
		require.Equal(t, "/sub/test", r.Path)

		return nil
	})

	app.Hooks().OnRoute(func(r Route) error {
		require.Equal(t, "/", r.Path)

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
		require.NoError(t, err)

		return nil
	})

	app.Get("/", testSimpleHandler).Name("index")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)
	subApp.Get("/test2", testSimpleHandler)

	app.Use("/sub", subApp)

	require.Equal(t, "index", buf.String())
}

func Test_Hook_OnName_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnName(func(_ Route) error {
		return errors.New("unknown error")
	})

	require.PanicsWithError(t, "unknown error", func() {
		app.Get("/", testSimpleHandler).Name("index")
	})
}

func Test_Hook_OnGroup(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnGroup(func(g Group) error {
		_, err := buf.WriteString(g.Prefix)
		require.NoError(t, err)
		return nil
	})

	grp := app.Group("/x").Name("x.")
	grp.Group("/a")

	require.Equal(t, "/x/x/a", buf.String())
}

func Test_Hook_OnGroup_Mount(t *testing.T) {
	t.Parallel()
	app := New()
	micro := New()
	micro.Use("/john", app)

	app.Hooks().OnGroup(func(g Group) error {
		require.Equal(t, "/john/v1", g.Prefix)
		return nil
	})

	v1 := app.Group("/v1")
	v1.Get("/doe", func(c Ctx) error {
		return c.SendStatus(StatusOK)
	})
}

func Test_Hook_OnGroupName(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf2 := bytebufferpool.Get()
	defer bytebufferpool.Put(buf2)

	app.Hooks().OnGroupName(func(g Group) error {
		_, err := buf.WriteString(g.name)
		require.NoError(t, err)

		return nil
	})

	app.Hooks().OnName(func(r Route) error {
		_, err := buf2.WriteString(r.Name)
		require.NoError(t, err)

		return nil
	})

	grp := app.Group("/x").Name("x.")
	grp.Get("/test", testSimpleHandler).Name("test")
	grp.Get("/test2", testSimpleHandler)

	require.Equal(t, "x.", buf.String())
	require.Equal(t, "x.test", buf2.String())
}

func Test_Hook_OnGroupName_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnGroupName(func(_ Group) error {
		return errors.New("unknown error")
	})

	require.PanicsWithError(t, "unknown error", func() {
		_ = app.Group("/x").Name("x.")
	})
}

func Test_Hook_OnPrehutdown(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnPreShutdown(func() error {
		_, err := buf.WriteString("pre-shutdowning")
		require.NoError(t, err)

		return nil
	})

	require.NoError(t, app.Shutdown())
	require.Equal(t, "pre-shutdowning", buf.String())
}

func Test_Hook_OnPostShutdown(t *testing.T) {
	t.Run("should execute post shutdown hook with error", func(t *testing.T) {
		app := New()
		expectedErr := errors.New("test shutdown error")

		hookCalled := make(chan error, 1)
		defer close(hookCalled)

		app.Hooks().OnPostShutdown(func(err error) error {
			hookCalled <- err
			return nil
		})

		go func() {
			if err := app.Listen(":0"); err != nil {
				return
			}
		}()

		time.Sleep(100 * time.Millisecond)

		app.hooks.executeOnPostShutdownHooks(expectedErr)

		select {
		case err := <-hookCalled:
			require.Equal(t, expectedErr, err)
		case <-time.After(time.Second):
			t.Fatal("hook execution timeout")
		}

		require.NoError(t, app.Shutdown())
	})

	t.Run("should execute multiple hooks in order", func(t *testing.T) {
		app := New()

		execution := make([]int, 0)

		app.Hooks().OnPostShutdown(func(_ error) error {
			execution = append(execution, 1)
			return nil
		})

		app.Hooks().OnPostShutdown(func(_ error) error {
			execution = append(execution, 2)
			return nil
		})

		app.hooks.executeOnPostShutdownHooks(nil)

		require.Len(t, execution, 2, "expected 2 hooks to execute")
		require.Equal(t, []int{1, 2}, execution, "hooks executed in wrong order")
	})

	t.Run("should handle hook error", func(_ *testing.T) {
		app := New()
		hookErr := errors.New("hook error")

		app.Hooks().OnPostShutdown(func(_ error) error {
			return hookErr
		})

		// Should not panic
		app.hooks.executeOnPostShutdownHooks(nil)
	})
}

func Test_Hook_OnListen(t *testing.T) {
	t.Parallel()

	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnListen(func(_ ListenData) error {
		_, err := buf.WriteString("ready")
		require.NoError(t, err)

		return nil
	})

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()
	require.NoError(t, app.Listen(":9000"))

	require.Equal(t, "ready", buf.String())
}

func Test_Hook_OnListenPrefork(t *testing.T) {
	t.Parallel()
	app := New()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	app.Hooks().OnListen(func(_ ListenData) error {
		_, err := buf.WriteString("ready")
		require.NoError(t, err)

		return nil
	})

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":9000", ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
	require.Equal(t, "ready", buf.String())
}

func Test_Hook_OnHook(t *testing.T) {
	app := New()

	// Reset test var
	testPreforkMaster = true
	testOnPrefork = true

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	app.Hooks().OnFork(func(pid int) error {
		require.Equal(t, 1, pid)
		return nil
	})

	require.NoError(t, app.prefork(":3000", nil, ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

func Test_Hook_OnMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", testSimpleHandler).Name("x")

	subApp := New()
	subApp.Get("/test", testSimpleHandler)

	subApp.Hooks().OnMount(func(parent *App) error {
		require.Empty(t, parent.mountFields.mountPath)

		return nil
	})

	app.Use("/sub", subApp)
}

func Test_executeOnRouteHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.mountFields.mountPath = testMountPath

	var received string
	app.Hooks().OnRoute(func(r Route) error {
		received = r.Path
		return errors.New("hook error")
	})

	err := app.hooks.executeOnRouteHooks(Route{Path: "/foo", path: "/foo"})
	require.Equal(t, testMountPath+"/foo", received)
	require.EqualError(t, err, "hook error")
}

func Test_executeOnNameHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.mountFields.mountPath = testMountPath

	var received string
	app.Hooks().OnName(func(r Route) error {
		received = r.Path
		return errors.New("name error")
	})

	err := app.hooks.executeOnNameHooks(Route{Path: "/bar", path: "/bar"})
	require.Equal(t, testMountPath+"/bar", received)
	require.EqualError(t, err, "name error")
}

func Test_executeOnGroupHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.mountFields.mountPath = testMountPath

	var prefix string
	app.Hooks().OnGroup(func(g Group) error {
		prefix = g.Prefix
		return errors.New("group error")
	})

	err := app.hooks.executeOnGroupHooks(Group{Prefix: "/grp"})
	require.Equal(t, testMountPath+"/grp", prefix)
	require.EqualError(t, err, "group error")
}

func Test_executeOnGroupNameHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	app := New()
	app.mountFields.mountPath = testMountPath

	var prefix string
	app.Hooks().OnGroupName(func(g Group) error {
		prefix = g.Prefix
		return errors.New("group name error")
	})

	err := app.hooks.executeOnGroupNameHooks(Group{Prefix: "/grp"})
	require.Equal(t, testMountPath+"/grp", prefix)
	require.EqualError(t, err, "group name error")
}

func Test_executeOnListenHooks_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnListen(func(_ ListenData) error {
		return errors.New("listen error")
	})

	err := app.hooks.executeOnListenHooks(ListenData{Host: "127.0.0.1", Port: "80"})
	require.EqualError(t, err, "listen error")
}

func Test_executeOnPreShutdownHooks_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnPreShutdown(func() error {
		return errors.New("pre error")
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	app.hooks.executeOnPreShutdownHooks()
	require.NotZero(t, buf.Len())
}

func Test_executeOnForkHooks_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnFork(func(pid int) error {
		require.Equal(t, 1, pid)
		return errors.New("fork error")
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	app.hooks.executeOnForkHooks(1)
	require.NotZero(t, buf.Len())
}

func Test_executeOnMountHooks_Error(t *testing.T) {
	t.Parallel()
	app := New()
	parent := New()

	app.Hooks().OnMount(func(a *App) error {
		require.Equal(t, parent, a)
		return errors.New("mount error")
	})

	err := app.hooks.executeOnMountHooks(parent)
	require.EqualError(t, err, "mount error")
}
