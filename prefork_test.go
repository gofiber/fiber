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

	err := app.prefork(NetworkTCP4, "invalid", nil)
	require.Equal(t, false, err == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Equal(t, nil, app.Shutdown())
	}()

	require.Equal(t, nil, app.prefork(NetworkTCP6, "[::1]:", nil))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		require.Equal(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Equal(t, nil, app.Shutdown())
	}()

	require.Equal(t, nil, app.prefork(NetworkTCP4, "127.0.0.1:", config))
}

func Test_App_Prefork_Master_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Equal(t, nil, app.Shutdown())
	}()

	require.Equal(t, nil, app.prefork(NetworkTCP4, ":3000", nil))

	dummyChildCmd = "invalid"

	err := app.prefork(NetworkTCP4, "127.0.0.1:", nil)
	require.Equal(t, false, err == nil)
}

func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	setupIsChild(t)
	defer teardownIsChild(t)

	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	require.Equal(t, nil, err)

	os.Stdout = w

	New().startupProcess().startupMessage(":3000", false, "")

	require.Equal(t, nil, w.Close())

	out, err := io.ReadAll(r)
	require.Equal(t, nil, err)
	require.Equal(t, 0, len(out))
}

func setupIsChild(t *testing.T) {
	t.Helper()

	require.Equal(t, nil, os.Setenv(envPreforkChildKey, envPreforkChildVal))
}

func teardownIsChild(t *testing.T) {
	t.Helper()

	require.Equal(t, nil, os.Setenv(envPreforkChildKey, ""))
}
