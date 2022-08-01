// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"crypto/tls"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_App_Listen
func Test_App_Listen(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})

	utils.AssertEqual(t, false, app.Listen(":99999") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":4003"))
}

// go test -run Test_App_Listen_Prefork
func Test_App_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	utils.AssertEqual(t, nil, app.Listen(":99999"))
}

// go test -run Test_App_ListenTLS
func Test_App_ListenTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key") == nil)
	// missing perm/cert file
	utils.AssertEqual(t, false, app.ListenTLS(":0", "", "./.github/testdata/ssl.key") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_ListenTLS_Prefork
func Test_App_ListenTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	// invalid key file content
	utils.AssertEqual(t, false, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/template.tmpl") == nil)

	utils.AssertEqual(t, nil, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_ListenMutualTLS
func Test_App_ListenMutualTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.ListenMutualTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem") == nil)
	// missing perm/cert file
	utils.AssertEqual(t, false, app.ListenMutualTLS(":0", "", "./.github/testdata/ssl.key", "") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.ListenMutualTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem"))
}

// go test -run Test_App_ListenMutualTLS_Prefork
func Test_App_ListenMutualTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	// invalid key file content
	utils.AssertEqual(t, false, app.ListenMutualTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/template.html", "") == nil)

	utils.AssertEqual(t, nil, app.ListenMutualTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem"))
}

// go test -run Test_App_Listener
func Test_App_Listener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

// go test -run Test_App_Listener_Prefork
func Test_App_Listener_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	utils.AssertEqual(t, nil, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listener(ln))
}

func Test_App_print_Route(t *testing.T) {
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	fmt.Println(printRoutesMessage)
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "GET"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "routeName"))
}

func Test_App_print_Route_with_group(t *testing.T) {
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler)

	v1 := app.Group("v1")
	v1.Get("/test", emptyHandler).Name("v1")
	v1.Post("/test/fiber", emptyHandler)
	v1.Put("/test/fiber/*", emptyHandler)

	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})

	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "GET"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "POST"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "PUT"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber/*"))
}

func emptyHandler(c *Ctx) error {
	return nil
}
