package fiber

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

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
	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "unknown error", fmt.Sprintf("%v", err))
		}
	}()

	app.Hooks().OnName(func(_ Route) error {
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
	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "unknown error", fmt.Sprintf("%v", err))
		}
	}()

	app.Hooks().OnGroupName(func(_ Group) error {
		return errors.New("unknown error")
	})

	grp := app.Group("/x").Name("x.")
	grp.Get("/test", testSimpleHandler)
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

		hookCalled := false
		var receivedErr error
		expectedErr := errors.New("test shutdown error")

		app.Hooks().OnPostShutdown(func(err error) error {
			hookCalled = true
			receivedErr = err
			return nil
		})

		go func() {
			if err := app.Listen(":0"); err != nil {
				t.Errorf("Failed to start listener: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)

		app.hooks.executeOnPostShutdownHooks(expectedErr)

		if !hookCalled {
			t.Fatal("hook was not called")
		}

		if !errors.Is(receivedErr, expectedErr) {
			t.Fatalf("hook received wrong error: want %v, got %v", expectedErr, receivedErr)
		}
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

		if len(execution) != 2 {
			t.Fatalf("expected 2 hooks to execute, got %d", len(execution))
		}

		if execution[0] != 1 || execution[1] != 2 {
			t.Fatal("hooks executed in wrong order")
		}
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
