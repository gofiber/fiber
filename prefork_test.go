// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// 💖 Maintained and modified for Fiber by @renewerner87
package fiber

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/prefork"
)

// preforkChildHelperEnv makes Test_Prefork_Child_GracefulContext_Helper run as the prefork child that
// Test_Prefork_Child_GracefulContext_WatchedOnce starts. In an ordinary test run the helper does nothing.
const preforkChildHelperEnv = "FIBER_TEST_PREFORK_CHILD_HELPER"

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_App_Prefork_Child_Process(t *testing.T) {
	skipIfNoIPv6(t)

	enableTestPreforkMaster(t)

	setupIsChild(t)

	app := New()

	cfg := listenConfigDefault()
	require.Error(t, app.prefork("invalid", nil, &cfg))

	rootMiscBShutdownAfter(t, app, 1000*time.Millisecond)

	ipv6Cfg := ListenConfig{ListenerNetwork: NetworkTCP6}
	require.NoError(t, app.prefork("[::1]:", nil, &ipv6Cfg))

	config := &tls.Config{Certificates: []tls.Certificate{rootMiscBTestCert(t)}}

	rootMiscBShutdownAfter(t, app, 1000*time.Millisecond)

	cfg = listenConfigDefault()
	require.NoError(t, app.prefork("127.0.0.1:", config, &cfg))

	// The child installs the Content-Length/Transfer-Encoding close hook before it serves.
	require.NotNil(t, app.Server().HeaderReceived)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_App_Prefork_Master_Process(t *testing.T) {
	enableTestPreforkMaster(t)

	app := New()

	// With dummy commands that exit immediately, fasthttp recovers children
	// until RecoverThreshold is exceeded, then returns ErrOverRecovery.
	// Use low threshold for fast test execution.
	cfg := listenConfigDefault()
	cfg.PreforkRecoverThreshold = 1
	cfg.PreforkRecoverInterval = 10 * time.Millisecond
	cfg.PreforkShutdownGracePeriod = 100 * time.Millisecond
	err := app.prefork(":0", nil, &cfg)
	require.ErrorIs(t, err, prefork.ErrOverRecovery)

	// With invalid command, should get a start error immediately
	// (error happens during initial spawning, before recovery loop)
	dummyChildCmd.Store("invalid")

	cfg = listenConfigDefault()
	cfg.PreforkRecoverThreshold = 1
	err = app.prefork("127.0.0.1:", nil, &cfg)
	require.Error(t, err)

	dummyChildCmd.Store("go")
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	setupIsChild(t)

	out := captureOutput(func() {
		cfg := listenConfigDefault()
		app := New()
		app.startupProcess()
		listenData := app.prepareListenData(":0", false, &cfg, nil)
		app.startupMessage(listenData, &cfg)
	})
	require.Empty(t, out)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_IsChild(t *testing.T) {
	require.False(t, IsChild())
	setupIsChild(t)
	require.True(t, IsChild())
}

func Test_Prefork_Logger(t *testing.T) {
	t.Parallel()
	l := preforkLogger{}
	// Should not panic
	l.Printf("test %s", "message")
}

// The function a prefork child serves the listener it bound with has to stop on a GracefulContext
// done before it serves, as Listen does: canceled right before it serves, or done before the child
// ran it. It starts to watch the context itself, so this covers the real arming. The fixture and
// the table come from listen_test.go.
func Test_preforkServe_GracefulContext_DoneBeforeServe(t *testing.T) {
	t.Parallel()
	for _, state := range gracefulContextStates {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			f := newGracefulFixture(t)
			// The child calls this right before it serves.
			f.cfg.ListenerAddrFunc = func(net.Addr) { f.cancelAndAwait() }
			state.prepare(f)

			f.requireStopped(t, f.start(func(f *gracefulFixture) error { return f.app.preforkServe(nil, &f.cfg)(f.ln) }))
			listenerEntry.requireClosed(t, f)
		})
	}
}

// A prefork child watches its GracefulContext once, in preforkServe, and prefork() watches only for the
// master: a child that watched in both would run its shutdown hooks twice. The child path needs the
// FASTHTTP_PREFORK_CHILD variable, which t.Setenv cannot set in a parallel test, so the child is a
// subprocess of this test binary that reports how often its shutdown hooks ran.
func Test_Prefork_Child_GracefulContext_WatchedOnce(t *testing.T) {
	t.Parallel()
	exe, err := os.Executable()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, "-test.v=true", "-test.run=^Test_Prefork_Child_GracefulContext_Helper$")
	cmd.Env = append(os.Environ(), "FASTHTTP_PREFORK_CHILD=1", preforkChildHelperEnv+"=1")

	out, err := cmd.CombinedOutput()
	output := string(out)
	require.NoError(t, err, output)
	require.Contains(t, output, "child result: err=<nil> pre=1 post=1", output)
}

// Test_Prefork_Child_GracefulContext_Helper is the prefork child of the test above: it cancels its
// context right before it serves, as the table of Test_preforkServe_GracefulContext_DoneBeforeServe does.
func Test_Prefork_Child_GracefulContext_Helper(t *testing.T) {
	t.Parallel()
	if os.Getenv(preforkChildHelperEnv) != "1" {
		return
	}

	f := newGracefulFixture(t)
	f.cfg.EnablePrefork = true
	// The child calls this right before it serves.
	f.cfg.ListenerAddrFunc = func(net.Addr) { f.cancelAndAwait() }

	err := f.app.Listen("127.0.0.1:0", f.cfg)
	t.Logf("child result: err=%v pre=%d post=%d", err, f.probe.pre.Load(), f.probe.post.Load())
}

func setupIsChild(t *testing.T) {
	t.Helper()

	// Set the environment variable that fasthttp's prefork.IsChild() checks
	t.Setenv("FASTHTTP_PREFORK_CHILD", "1")
}

func enableTestPreforkMaster(t *testing.T) {
	t.Helper()

	previous := testPreforkMaster
	testPreforkMaster = true
	t.Cleanup(func() {
		testPreforkMaster = previous
		dummyChildCmd.Store("go")
	})
}

func enableTestOnPrefork(t *testing.T) {
	t.Helper()

	previous := testOnPrefork
	testOnPrefork = true
	t.Cleanup(func() { testOnPrefork = previous })
}
