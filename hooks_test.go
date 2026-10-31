package fiber

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/prefork"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/log"
)

const testMountPath = "/api"

func testSimpleHandler(c Ctx) error {
	return c.SendString("simple")
}

func Test_Hook_OnRoute(t *testing.T) {
	// Not parallel: redirects the process-wide logger output.
	app := New()

	app.Hooks().OnRoute(func(r Route) error {
		require.Empty(t, r.Name)

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
	var names string
	app.Hooks().OnName(func(r Route) error {
		names += r.Name
		return nil
	})
	app.Get("/", testSimpleHandler).Name("index")
	subApp := New()
	subApp.Get("/test", testSimpleHandler)
	subApp.Get("/test2", testSimpleHandler)
	app.Use("/sub", subApp)
	require.Equal(t, "index", names)
}

func Test_Hook_OnName_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnName(func(_ Route) error { return errors.New("unknown error") })

	require.PanicsWithError(t, "unknown error", func() { app.Get("/", testSimpleHandler).Name("index") })
}

func Test_Hook_OnGroup(t *testing.T) {
	t.Parallel()
	app := New()
	var prefixes string
	app.Hooks().OnGroup(func(g Group) error {
		prefixes += g.Prefix
		return nil
	})
	grp := app.Group("/x").Name("x.")
	grp.Group("/a")
	require.Equal(t, "/x/x/a", prefixes)
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
	v1.Get("/doe", func(c Ctx) error { return c.SendStatus(StatusOK) })
}

func Test_Hook_OnGroupName(t *testing.T) {
	t.Parallel()
	app := New()
	var groupNames, routeNames string
	app.Hooks().OnGroupName(func(g Group) error {
		groupNames += g.name
		return nil
	})
	app.Hooks().OnName(func(r Route) error {
		routeNames += r.Name
		return nil
	})
	grp := app.Group("/x").Name("x.")
	grp.Get("/test", testSimpleHandler).Name("test")
	grp.Get("/test2", testSimpleHandler)
	require.Equal(t, "x.", groupNames)
	require.Equal(t, "x.test", routeNames)
}

func Test_Hook_OnGroupName_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnGroupName(func(_ Group) error { return errors.New("unknown error") })

	require.PanicsWithError(t, "unknown error", func() { _ = app.Group("/x").Name("x.") })
}

func Test_Hook_OnPreShutdown(t *testing.T) {
	t.Parallel()
	app := New()
	var calls string
	app.Hooks().OnPreShutdown(func() error {
		calls += "pre-shutdown"
		return nil
	})
	require.NoError(t, app.Shutdown())
	require.Equal(t, "pre-shutdown", calls)
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

		require.Equal(t, expectedErr, apptest.Recv(t, hookCalled, time.Second, "hook execution timeout"))

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

		app.Hooks().OnPostShutdown(func(_ error) error { return hookErr })

		// Should not panic
		app.hooks.executeOnPostShutdownHooks(nil)
	})
}

func Test_Hook_OnListen(t *testing.T) {
	t.Parallel()
	app := New()
	var calls string
	app.Hooks().OnListen(func(_ ListenData) error {
		calls += "ready"
		return nil
	})
	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()
	require.NoError(t, app.Listen(":0"))
	require.Equal(t, "ready", calls)
}

func Test_ListenDataMetadata(t *testing.T) {
	t.Parallel()
	app := New(Config{AppName: "meta"})
	app.handlersCount = 42

	cfg := ListenConfig{EnablePrefork: true}
	childPIDs := []int{11, 22}
	listenData := app.prepareListenData(":3030", true, &cfg, childPIDs)
	// requireMeta requires data to hold what the app and listen config above set.
	requireMeta := func(data *ListenData) {
		require.Equal(t, globalIpv4Addr, data.Host)
		require.Equal(t, "3030", data.Port)
		require.True(t, data.TLS)
		require.Equal(t, Version, data.Version)
		require.Equal(t, "meta", data.AppName)
		require.Equal(t, 42, data.HandlerCount)
		require.Equal(t, runtime.GOMAXPROCS(0), data.ProcessCount)
		require.Equal(t, os.Getpid(), data.PID)
		require.True(t, data.Prefork)
		require.Equal(t, childPIDs, data.ChildPIDs)
		require.Equal(t, app.config.ColorScheme, data.ColorScheme)
	}

	app.Hooks().OnListen(func(data ListenData) error {
		requireMeta(&data)
		return nil
	})
	app.runOnListenHooks(listenData)

	app.Hooks().OnPreStartupMessage(func(data *PreStartupMessageData) error {
		requireMeta(data.ListenData)
		data.ResetEntries()

		data.AddInfo("custom", "Custom Info", "value", 3)
		data.AddInfo("other", "Other Info", "value", 2)

		return nil
	})

	pre := newPreStartupMessageData(listenData)
	require.NoError(t, app.hooks.executeOnPreStartupMessageHooks(pre))

	require.Equal(t, "value", pre.entries[0].value)
	require.Equal(t, "Custom Info", pre.entries[0].title)
	require.Equal(t, 3, pre.entries[0].priority)

	require.Equal(t, "value", pre.entries[1].value)
	require.Equal(t, "Other Info", pre.entries[1].title)
	require.Equal(t, 2, pre.entries[1].priority)
	require.False(t, pre.PreventDefault)
}

// rootMiscARunPreStartupHook registers hook as the only OnPreStartupMessage
// hook of a new app and executes the app's hooks on a blank
// PreStartupMessageData, whose entries are reset before hook runs. The
// assertions live in hook, which must not fail.
func rootMiscARunPreStartupHook(t *testing.T, hook func(data *PreStartupMessageData)) {
	t.Helper()
	app := New()
	app.Hooks().OnPreStartupMessage(func(data *PreStartupMessageData) error {
		data.ResetEntries()
		hook(data)
		return nil
	})

	pre := newPreStartupMessageData(&ListenData{})
	require.NoError(t, app.hooks.executeOnPreStartupMessageHooks(pre))
}

func Test_ListenData_Hook_HelperFunctions(t *testing.T) {
	t.Parallel()
	t.Run("EntryKeys", func(t *testing.T) {
		t.Parallel()
		rootMiscARunPreStartupHook(t, func(data *PreStartupMessageData) {
			data.AddInfo("key1", "Title 1", "Value 1", 1)
			data.AddInfo("key2", "Title 2", "Value 2", 2)
			keys := data.EntryKeys()
			require.Len(t, keys, 2)
			require.Equal(t, "key1", keys[0])
			require.Equal(t, "key2", keys[1])
		})
	})

	t.Run("ResetEntries", func(t *testing.T) {
		t.Parallel()
		rootMiscARunPreStartupHook(t, func(data *PreStartupMessageData) {
			data.AddInfo("key1", "Title 1", "Value 1", 1)
			data.AddInfo("key2", "Title 2", "Value 2", 2)
			require.Len(t, data.entries, 2)
			data.ResetEntries()
			require.Empty(t, data.entries)
		})
	})

	// AddInfo, AddWarning and AddError differ only in the level they record.
	for _, tc := range []struct {
		add   func(data *PreStartupMessageData, key, title, value string, priority ...int)
		name  string
		level StartupMessageLevel
	}{
		{name: "AddInfo", add: (*PreStartupMessageData).AddInfo, level: StartupMessageLevelInfo},
		{name: "AddWarning", add: (*PreStartupMessageData).AddWarning, level: StartupMessageLevelWarning},
		{name: "AddError", add: (*PreStartupMessageData).AddError, level: StartupMessageLevelError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootMiscARunPreStartupHook(t, func(data *PreStartupMessageData) {
				tc.add(data, "key1", "Title 1", "Value 1", 1)
				require.Len(t, data.entries, 1)
				require.Equal(t, "key1", data.entries[0].key)
				require.Equal(t, "Title 1", data.entries[0].title)
				require.Equal(t, "Value 1", data.entries[0].value)
				require.Equal(t, 1, data.entries[0].priority)
				require.Equal(t, tc.level, data.entries[0].level)
			})
		})
	}

	t.Run("AddInfo-UpdateExisting", func(t *testing.T) {
		t.Parallel()
		rootMiscARunPreStartupHook(t, func(data *PreStartupMessageData) {
			data.AddInfo("key1", "Title 1", "Value 1", 1)
			data.AddInfo("key1", "Updated Title", "Updated Value", 2)
			require.Len(t, data.entries, 1)
			require.Equal(t, "key1", data.entries[0].key)
			require.Equal(t, "Updated Title", data.entries[0].title)
			require.Equal(t, "Updated Value", data.entries[0].value)
			require.Equal(t, 2, data.entries[0].priority)
		})
	})

	t.Run("DeleteEntry", func(t *testing.T) {
		t.Parallel()
		rootMiscARunPreStartupHook(t, func(data *PreStartupMessageData) {
			data.AddInfo("key1", "Title 1", "Value 1", 1)
			data.AddInfo("key2", "Title 2", "Value 2", 2)
			require.Len(t, data.entries, 2)
			data.DeleteEntry("key1")
			require.Len(t, data.entries, 1)
			require.Equal(t, "key2", data.entries[0].key)

			data.DeleteEntry("key-not-exist") // should not panic
			require.Len(t, data.entries, 1)
		})
	})
}

func Test_Hook_OnListenPrefork(t *testing.T) {
	enableTestPreforkMaster(t)

	app := New()
	var calls string
	app.Hooks().OnListen(func(_ ListenData) error {
		calls += "ready"
		return nil
	})
	require.ErrorIs(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage:   true,
		EnablePrefork:           true,
		PreforkRecoverThreshold: 1,
	}), prefork.ErrOverRecovery)
	require.Equal(t, "ready", calls)
}

func Test_Hook_OnHook(t *testing.T) {
	app := New()

	enableTestPreforkMaster(t)
	enableTestOnPrefork(t)

	app.Hooks().OnFork(func(pid int) error {
		require.Equal(t, 1, pid)
		return nil
	})

	require.ErrorIs(t, app.prefork(":0", nil, &ListenConfig{
		DisableStartupMessage:   true,
		EnablePrefork:           true,
		PreforkRecoverThreshold: 1,
	}), prefork.ErrOverRecovery)
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

// Test_executeOnRouteHooks_ErrorWithMount pins that the route and name hooks of
// a mounted app are handed the mount path in front of the route path, and that
// the error a hook returns reaches the caller.
func Test_executeOnRouteHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		on      func(h *Hooks, handler ...OnRouteHandler)
		execute func(h *Hooks, route *Route) error
		name    string
		path    string
		err     string
	}{
		{name: "OnRoute", on: (*Hooks).OnRoute, execute: (*Hooks).executeOnRouteHooks, path: "/foo", err: "hook error"},
		{name: "OnName", on: (*Hooks).OnName, execute: (*Hooks).executeOnNameHooks, path: "/bar", err: "name error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.mountFields.mountPath = testMountPath

			var received string
			tc.on(app.Hooks(), func(r Route) error {
				received = r.Path
				return errors.New(tc.err)
			})

			err := tc.execute(app.hooks, &Route{Path: tc.path, path: tc.path})
			require.Equal(t, testMountPath+tc.path, received)
			require.EqualError(t, err, tc.err)
		})
	}
}

// Test_executeOnGroupHooks_ErrorWithMount is the group counterpart of
// Test_executeOnRouteHooks_ErrorWithMount: the group and group name hooks are
// handed the mount path in front of the group prefix.
func Test_executeOnGroupHooks_ErrorWithMount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		on      func(h *Hooks, handler ...OnGroupHandler)
		execute func(h *Hooks, group Group) error
		name    string
		prefix  string
		err     string
	}{
		{name: "OnGroup", on: (*Hooks).OnGroup, execute: (*Hooks).executeOnGroupHooks, prefix: "/grp", err: "group error"},
		{name: "OnGroupName", on: (*Hooks).OnGroupName, execute: (*Hooks).executeOnGroupNameHooks, prefix: "/grp", err: "group name error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.mountFields.mountPath = testMountPath

			var prefix string
			tc.on(app.Hooks(), func(g Group) error {
				prefix = g.Prefix
				return errors.New(tc.err)
			})

			err := tc.execute(app.hooks, Group{Prefix: tc.prefix})
			require.Equal(t, testMountPath+tc.prefix, prefix)
			require.EqualError(t, err, tc.err)
		})
	}
}

// Test_executeOnHooks_Error pins that the error a hook returns reaches the
// caller of the function that executes the hooks.
func Test_executeOnHooks_Error(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		run  func(t *testing.T, app *App) error // registers a failing hook and executes the hooks
		name string
		err  string
	}{
		{name: "OnListen", err: "listen error", run: func(_ *testing.T, app *App) error {
			app.Hooks().OnListen(func(_ ListenData) error { return errors.New("listen error") })
			return app.hooks.executeOnListenHooks(&ListenData{Host: "127.0.0.1", Port: "0"})
		}},
		{name: "OnPreStartupMessage", err: "pre startup message error", run: func(_ *testing.T, app *App) error {
			app.Hooks().OnPreStartupMessage(func(_ *PreStartupMessageData) error { return errors.New("pre startup message error") })
			return app.hooks.executeOnPreStartupMessageHooks(newPreStartupMessageData(&ListenData{}))
		}},
		{name: "OnPostStartupMessage", err: "post startup message error", run: func(_ *testing.T, app *App) error {
			app.Hooks().OnPostStartupMessage(func(_ *PostStartupMessageData) error { return errors.New("post startup message error") })
			return app.hooks.executeOnPostStartupMessageHooks(newPostStartupMessageData(&ListenData{}, false, false, false))
		}},
		{name: "OnMount", err: "mount error", run: func(t *testing.T, app *App) error {
			t.Helper()
			parent := New()
			app.Hooks().OnMount(func(a *App) error {
				require.Equal(t, parent, a)
				return errors.New("mount error")
			})
			return app.hooks.executeOnMountHooks(parent)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.EqualError(t, tc.run(t, New()), tc.err)
		})
	}
}

func Test_executeOnPreShutdownHooks_Error(t *testing.T) {
	t.Parallel()
	app := New()

	app.Hooks().OnPreShutdown(func() error { return errors.New("pre error") })

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	app.hooks.executeOnPreShutdownHooks()
	require.NotZero(t, buf.Len())
}

func Test_executeOnForkHooks_Error(t *testing.T) {
	// Not parallel: redirects the process-wide logger output.
	app := New()

	app.Hooks().OnFork(func(pid int) error {
		require.Equal(t, 1, pid)
		return errors.New("fork error")
	})

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	app.hooks.executeOnForkHooks(1)
	require.NotZero(t, buf.Len())
}

func Test_Hooks_ShutdownSnapshot_WithoutApp(t *testing.T) {
	t.Parallel()
	var pre, post int
	h := &Hooks{
		onPreShutdown:  []OnPreShutdownHandler{func() error { pre++; return nil }},
		onPostShutdown: []OnPostShutdownHandler{func(error) error { post++; return nil }},
	}

	require.Len(t, h.snapshotOnPreShutdown(), 1)
	require.Len(t, h.snapshotOnPostShutdown(), 1)

	h.executeOnPreShutdownHooks()
	h.executeOnPostShutdownHooks(nil)
	require.Equal(t, 1, pre)
	require.Equal(t, 1, post)
}
