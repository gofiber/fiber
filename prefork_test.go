// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// 💖 Maintained and modified for Fiber by @renewerner87
package fiber

import (
	"crypto/tls"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/prefork"
)

func Test_App_Prefork_Child_Process(t *testing.T) {
	enableTestPreforkMaster(t)

	setupIsChild(t)

	app := New()

	cfg := listenConfigDefault()
	err := app.prefork("invalid", nil, &cfg)
	require.Error(t, err)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	ipv6Cfg := ListenConfig{ListenerNetwork: NetworkTCP6}
	require.NoError(t, app.prefork("[::1]:", nil, &ipv6Cfg))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		require.NoError(t, err)
	}
	//nolint:gosec // We're in a test so using old ciphers is fine
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	cfg = listenConfigDefault()
	require.NoError(t, app.prefork("127.0.0.1:", config, &cfg))
}

func Test_App_Prefork_Master_Process(t *testing.T) {
	enableTestPreforkMaster(t)

	app := New()

	// With dummy commands that exit immediately, fasthttp recovers children
	// until RecoverThreshold is exceeded, then returns ErrOverRecovery.
	// Use low threshold for fast test execution.
	cfg := listenConfigDefault()
	cfg.PreforkRecoverThreshold = 1
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

func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	setupIsChild(t)

	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w

	cfg := listenConfigDefault()
	app := New()
	app.startupProcess()
	listenData := app.prepareListenData(":0", false, &cfg, nil)
	app.startupMessage(listenData, &cfg)

	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, out)
}

func Test_IsChild(t *testing.T) {
	// Without env var, should be false
	require.False(t, IsChild())

	// With env var, should be true
	setupIsChild(t)
	require.True(t, IsChild())
}

func Test_Prefork_Logger(t *testing.T) {
	t.Parallel()

	l := preforkLogger{}
	// Should not panic
	l.Printf("test %s", "message")
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
	t.Cleanup(func() {
		testOnPrefork = previous
	})
}
