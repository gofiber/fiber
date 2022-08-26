// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_App_Listen
func Test_App_Listen(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})

	require.False(t, app.Listen(":99999") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.Listen(":4003"))
}

// go test -run Test_App_Listen_Prefork
func Test_App_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	require.Nil(t, app.Listen(":99999"))
}

// go test -run Test_App_ListenTLS
func Test_App_ListenTLS(t *testing.T) {
	app := New()

	// invalid port
	require.False(t, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key") == nil)
	// missing perm/cert file
	require.False(t, app.ListenTLS(":0", "", "./.github/testdata/ssl.key") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_ListenTLS_Prefork
func Test_App_ListenTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	// invalid key file content
	require.False(t, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/template.tmpl") == nil)

	require.Nil(t, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_ListenMutualTLS
func Test_App_ListenMutualTLS(t *testing.T) {
	app := New()

	// invalid port
	require.False(t, app.ListenMutualTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem") == nil)
	// missing perm/cert file
	require.False(t, app.ListenMutualTLS(":0", "", "./.github/testdata/ssl.key", "") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.ListenMutualTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem"))
}

// go test -run Test_App_ListenMutualTLS_Prefork
func Test_App_ListenMutualTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	// invalid key file content
	require.False(t, app.ListenMutualTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/template.html", "") == nil)

	require.Nil(t, app.ListenMutualTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key", "./.github/testdata/ca-chain.cert.pem"))
}

// go test -run Test_App_Listener
func Test_App_Listener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		require.Nil(t, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	require.Nil(t, app.Listener(ln))
}

// go test -run Test_App_Listener_Prefork
func Test_App_Listener_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	ln := fasthttputil.NewInmemoryListener()
	require.Nil(t, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		require.NoError(t, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	require.NoError(t, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.Listener(ln))
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
		io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func Test_App_Master_Process_Show_Startup_Message(t *testing.T) {
	startupMessage := captureOutput(func() {
		New(Config{Prefork: true}).
			startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	fmt.Println(startupMessage)
	require.True(t, strings.Contains(startupMessage, "https://127.0.0.1:3000"))
	require.True(t, strings.Contains(startupMessage, "(bound on host 0.0.0.0 and port 3000)"))
	require.True(t, strings.Contains(startupMessage, "Child PIDs"))
	require.True(t, strings.Contains(startupMessage, "11111, 22222, 33333, 44444, 55555, 60000"))
	require.True(t, strings.Contains(startupMessage, "Prefork ........ Enabled"))
}

func Test_App_Master_Process_Show_Startup_MessageWithAppName(t *testing.T) {
	app := New(Config{Prefork: true, AppName: "Test App v1.0.1"})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	fmt.Println(startupMessage)
	require.Equal(t, "Test App v1.0.1", app.Config().AppName)
	require.True(t, strings.Contains(startupMessage, app.Config().AppName))
}

func Test_App_Master_Process_Show_Startup_MessageWithAppNameNonAscii(t *testing.T) {
	appName := "Serveur de vérification des données"
	app := New(Config{Prefork: true, AppName: appName})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", false, "")
	})
	fmt.Println(startupMessage)
	require.True(t, strings.Contains(startupMessage, "│        Serveur de vérification des données        │"))
}

func Test_App_print_Route(t *testing.T) {
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	fmt.Println(printRoutesMessage)
	require.True(t, strings.Contains(printRoutesMessage, "GET"))
	require.True(t, strings.Contains(printRoutesMessage, "/"))
	require.True(t, strings.Contains(printRoutesMessage, "emptyHandler"))
	require.True(t, strings.Contains(printRoutesMessage, "routeName"))
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

	require.True(t, strings.Contains(printRoutesMessage, "GET"))
	require.True(t, strings.Contains(printRoutesMessage, "/"))
	require.True(t, strings.Contains(printRoutesMessage, "emptyHandler"))
	require.True(t, strings.Contains(printRoutesMessage, "/v1/test"))
	require.True(t, strings.Contains(printRoutesMessage, "POST"))
	require.True(t, strings.Contains(printRoutesMessage, "/v1/test/fiber"))
	require.True(t, strings.Contains(printRoutesMessage, "PUT"))
	require.True(t, strings.Contains(printRoutesMessage, "/v1/test/fiber/*"))
}

func emptyHandler(c Ctx) error {
	return nil
}
