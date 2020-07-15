package fiber

import (
	"os"
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
}

func Test_App_Prefork_Main_Process(t *testing.T) {
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
	utils.AssertEqual(t, "listen: tcp6 is not supported when prefork is enabled", app.Listen("[::1]:3000").Error())

	app.Settings.Network = "tcp6"
	app.init()
	utils.AssertEqual(t, "listen: tcp6 is not supported when prefork is enabled", app.Listen(":3000").Error())
}
