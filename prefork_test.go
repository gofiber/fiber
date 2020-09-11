package fiber

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
)

func Test_App_Prefork_Child_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	utils.AssertEqual(t, nil, os.Setenv(envPreforkChildKey, envPreforkChildVal))
	defer os.Setenv(envPreforkChildKey, "")

	app := New()

	err := app.prefork("invalid", nil)
	utils.AssertEqual(t, false, err == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("[::]:", nil))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("127.0.0.1:", config))
}

func Test_App_Prefork_Master_Process(t *testing.T) {
	// Reset test var
	testPreforkMaster = true

	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork(":3000", nil))

	dummyChildCmd = "invalid"

	err := app.prefork("127.0.0.1:", nil)
	utils.AssertEqual(t, false, err == nil)
}

func Test_App_Prefork_Child_Process_Never_Show_Startup_Message(t *testing.T) {
	utils.AssertEqual(t, nil, os.Setenv(envPreforkChildKey, envPreforkChildVal))
	defer os.Setenv(envPreforkChildKey, "")

	rescueStdout := os.Stdout
	defer func() { os.Stdout = rescueStdout }()

	r, w, err := os.Pipe()
	utils.AssertEqual(t, nil, err)

	os.Stdout = w

	New().startupMessage(":3000", false, "")

	utils.AssertEqual(t, nil, w.Close())

	out, err := ioutil.ReadAll(r)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 0, len(out))
}
