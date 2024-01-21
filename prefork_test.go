// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📄 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io
// 💖 Maintained and modified for Fiber by @renewerner87
package fiber

import (
	"crypto/tls"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_App_Prefork_Child_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	setupIsChild(t)
	defer teardownIsChild(t)

	app := New()

	err := app.prefork("invalid", nil, listenConfigDefault())
	require.Error(t, err)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.prefork("[::1]:", nil, ListenConfig{ListenerNetwork: NetworkTCP6}))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		require.NoError(t, err)
	}
	//nolint:gosec // We're in a test so using old ciphers is fine
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.prefork("127.0.0.1:", config, listenConfigDefault()))
}

func Test_App_Prefork_Master_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.prefork(":3000", nil, listenConfigDefault()))

	dummyChildCmd.Store("invalid")

	err := app.prefork("127.0.0.1:", nil, listenConfigDefault())
	require.Error(t, err)

	dummyChildCmd.Store("go")
}

func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	setupIsChild(t)
	defer teardownIsChild(t)

	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w

	New().startupProcess().startupMessage(":3000", false, "", listenConfigDefault())

	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, out)
}

func setupIsChild(t *testing.T) {
	t.Helper()

	require.NoError(t, os.Setenv(envPreforkChildKey, envPreforkChildVal))
}

func teardownIsChild(t *testing.T) {
	t.Helper()

	require.NoError(t, os.Setenv(envPreforkChildKey, ""))
}
