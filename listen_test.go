// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_App_Listen
func Test_App_Listen(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	t.Parallel()
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	//nolint:gosec // We're in a test so using old ciphers is fine
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	//nolint:gosec // We're in a test so listening on all interfaces is fine
	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	utils.AssertEqual(t, nil, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listener(ln))
}

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(os.Stderr)
	}()
	os.Stdout = writer
	os.Stderr = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		_, err := io.Copy(&buf, reader)
		if err != nil {
			panic(err)
		}
		out <- buf.String()
	}()
	wg.Wait()
	f()
	err = writer.Close()
	if err != nil {
		panic(err)
	}
	return <-out
}

func Test_App_Master_Process_Show_Startup_Message(t *testing.T) {
	t.Parallel()
	startupMessage := captureOutput(func() {
		New(Config{Prefork: true}).
			startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "https://127.0.0.1:3000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "(bound on host 0.0.0.0 and port 3000)"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Child PIDs"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "11111, 22222, 33333, 44444, 55555, 60000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Prefork ........ Enabled"))
}

func Test_App_Master_Process_Show_Startup_MessageWithAppName(t *testing.T) {
	t.Parallel()
	app := New(Config{Prefork: true, AppName: "Test App v1.0.1"})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	utils.AssertEqual(t, "Test App v1.0.1", app.Config().AppName)
	utils.AssertEqual(t, true, strings.Contains(startupMessage, app.Config().AppName))
}

func Test_App_Master_Process_Show_Startup_MessageWithAppNameNonAscii(t *testing.T) {
	t.Parallel()
	appName := "Serveur de vérification des données"
	app := New(Config{Prefork: true, AppName: appName})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", false, "")
	})
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "│        Serveur de vérification des données        │"))
}

func Test_App_print_Route(t *testing.T) {
	t.Parallel()
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, MethodGet))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "routeName"))
}

func Test_App_print_Route_with_group(t *testing.T) {
	t.Parallel()
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler)

	v1 := app.Group("v1")
	v1.Get("/test", emptyHandler).Name("v1")
	v1.Post("/test/fiber", emptyHandler)
	v1.Put("/test/fiber/*", emptyHandler)

	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})

	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, MethodGet))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, MethodPost))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "PUT"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber/*"))
}

func emptyHandler(_ *Ctx) error {
	return nil
}
