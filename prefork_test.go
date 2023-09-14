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

	"github.com/gofiber/fiber/v2/utils"
)

//nolint:paralleltest // TODO: Must be run sequentially due to global testPreforkMaster var and using t.Setenv.
func Test_App_Prefork_Child_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	t.Setenv(envPreforkChildKey, envPreforkChildVal)

	app := New()

	err := app.prefork(NetworkTCP4, "invalid", nil)
	utils.AssertEqual(t, false, err == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork(NetworkTCP6, "[::1]:", nil))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	//nolint:gosec // We're in a test so using old ciphers is fine
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork(NetworkTCP4, "127.0.0.1:", config))
}

//nolint:paralleltest // TODO: Must be run sequentially due to global testPreforkMaster var.
func Test_App_Prefork_Master_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork(NetworkTCP4, ":3000", nil))

	dummyChildCmd.Store("invalid")

	err := app.prefork(NetworkTCP4, "127.0.0.1:", nil)
	utils.AssertEqual(t, false, err == nil)

	dummyChildCmd.Store("")
}

//nolint:paralleltest // TODO: Must be run sequentially due to overwriting of os.Std globals and using t.Setenv
func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	t.Setenv(envPreforkChildKey, envPreforkChildVal)

	rescueStdout := os.Stdout
	defer func() {
		os.Stdout = rescueStdout //nolint:reassign // TODO: Don't reassign
	}()

	r, w, err := os.Pipe()
	utils.AssertEqual(t, nil, err)

	os.Stdout = w //nolint:reassign // TODO: Don't reassign

	New().startupProcess().startupMessage(":3000", false, "")

	utils.AssertEqual(t, nil, w.Close())

	out, err := io.ReadAll(r)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 0, len(out))
}
