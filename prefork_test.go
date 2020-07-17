package fiber

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	utils "github.com/gofiber/utils"
)

func Test_App_Prefork_Child_Process(t *testing.T) {
	utils.AssertEqual(t, nil, os.Setenv(envPreforkChildKey, envPreforkChildVal))
	defer os.Setenv(envPreforkChildKey, "")

	app := New()
	app.init()

	err := app.prefork("invalid")
	utils.AssertEqual(t, false, err == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("127.0.0.1:"))

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/TEST_DATA/ssl.pem", "./.github/TEST_DATA/ssl.key")
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
	testPreforkMaster = true

	app := New()
	app.init()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("127.0.0.1:"))

	dummyChildCmd = "invalid"

	err := app.prefork("127.0.0.1:")
	utils.AssertEqual(t, false, err == nil)
}

func Test_App_Prefork_TCP6_Addr(t *testing.T) {
	app := New()
	app.Settings.Prefork = true
	app.Settings.DisableStartupMessage = true

	app.init()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	err := app.Listen("[::1]:3200")
	utils.AssertEqual(t, true, strings.Contains(err.Error(), "exit status"))
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
