package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_Listen
func Test_Listen(t *testing.T) {
	app := New()

	utils.AssertEqual(t, false, app.Listen(":99999") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":4003", ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_Listen_Graceful_Shutdown
func Test_Listen_Graceful_Shutdown(t *testing.T) {
	var mu sync.Mutex
	var shutdown bool

	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString(c.Hostname())
	})

	ln := fasthttputil.NewInmemoryListener()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()

		err := app.Listen(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       ctx,
			OnShutdownSuccess: func() {
				mu.Lock()
				shutdown = true
				mu.Unlock()
			},
		})

		utils.AssertEqual(t, nil, err)
	}()

	testCases := []struct {
		Time               time.Duration
		ExpectedBody       string
		ExpectedStatusCode int
		ExceptedErrsLen    int
	}{
		{Time: 100 * time.Millisecond, ExpectedBody: "example.com", ExpectedStatusCode: StatusOK, ExceptedErrsLen: 0},
		{Time: 500 * time.Millisecond, ExpectedBody: "", ExpectedStatusCode: 0, ExceptedErrsLen: 1},
	}

	for _, tc := range testCases {
		time.Sleep(tc.Time)

		a := Get("http://example.com")
		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		code, body, errs := a.String()

		utils.AssertEqual(t, tc.ExpectedStatusCode, code)
		utils.AssertEqual(t, tc.ExpectedBody, body)
		utils.AssertEqual(t, tc.ExceptedErrsLen, len(errs))
	}

	mu.Lock()
	utils.AssertEqual(t, true, shutdown)
	mu.Unlock()
}

// go test -run Test_Listen_Prefork
func Test_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	utils.AssertEqual(t, nil, app.Listen(":99999", ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

// go test -run Test_Listen_TLS
func Test_Listen_TLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.Listen(":99999", ListenConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))
}

// go test -run Test_Listen_TLS_Prefork
func Test_Listen_TLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	utils.AssertEqual(t, false, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.tmpl",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":99999", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
	}))
}

// go test -run Test_Listen_MutualTLS
func Test_Listen_MutualTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.Listen(":99999", ListenConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
	}))
}

// go test -run Test_Listen_MutualTLS_Prefork
func Test_Listen_MutualTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	utils.AssertEqual(t, false, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.html",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}) == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":99999", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}))
}

// go test -run Test_Listen_CustomListener
func Test_Listen_CustomListener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listen(ln))
}

// go test -run Test_Listen_CustomListener_Prefork
func Test_Listen_CustomListener_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listen(ln, ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

// go test -run Test_Listen_CustomTLSListener
func Test_Listen_CustomTLSListener(t *testing.T) {
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

	utils.AssertEqual(t, nil, app.Listen(ln))
}

// go test -run Test_Listen_TLSConfigFunc
func Test_Listen_TLSConfigFunc(t *testing.T) {
	var callTLSConfig bool
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		TLSConfigFunc: func(tlsConfig *tls.Config) {
			callTLSConfig = true
		},
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))

	utils.AssertEqual(t, true, callTLSConfig)
}

// go test -run Test_Listen_ListenerAddrFunc
func Test_Listen_ListenerAddrFunc(t *testing.T) {
	var network string
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.Network()
		},
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))

	utils.AssertEqual(t, "tcp", network)
}

// go test -run Test_Listen_BeforeServeFunc
func Test_Listen_BeforeServeFunc(t *testing.T) {
	var handlers uint32
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, errors.New("test"), app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		BeforeServeFunc: func(fiber *App) error {
			handlers = fiber.HandlersCount()

			return errors.New("test")
		},
	}))

	utils.AssertEqual(t, uint32(0), handlers)
}

// go test -run Test_Listen_ListenerNetwork
func Test_Listen_ListenerNetwork(t *testing.T) {
	var network string
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkTCP6,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.String()
		},
	}))

	utils.AssertEqual(t, true, strings.Contains(network, "[::]:"))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkTCP4,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.String()
		},
	}))

	utils.AssertEqual(t, true, strings.Contains(network, "0.0.0.0:"))
}

// go test -run Test_Listen_Master_Process_Show_Startup_Message
func Test_Listen_Master_Process_Show_Startup_Message(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
	}

	startupMessage := captureOutput(func() {
		New().
			startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10), cfg)
	})
	fmt.Println(startupMessage)
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "https://127.0.0.1:3000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "(bound on host 0.0.0.0 and port 3000)"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Child PIDs"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "11111, 22222, 33333, 44444, 55555, 60000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Prefork ........ Enabled"))
}

// go test -run Test_Listen_Master_Process_Show_Startup_MessageWithAppName
func Test_Listen_Master_Process_Show_Startup_MessageWithAppName(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
	}

	app := New(Config{AppName: "Test App v3.0.0"})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10), cfg)
	})
	fmt.Println(startupMessage)
	utils.AssertEqual(t, "Test App v3.0.0", app.Config().AppName)
	utils.AssertEqual(t, true, strings.Contains(startupMessage, app.Config().AppName))
}

// go test -run Test_Listen_Print_Route
func Test_Listen_Print_Route(t *testing.T) {
	app := New()
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

// go test -run Test_Listen_Print_Route_With_Group
func Test_Listen_Print_Route_With_Group(t *testing.T) {
	app := New()
	app.Get("/", emptyHandler)

	v1 := app.Group("v1")
	v1.Get("/test", emptyHandler).Name("v1")
	v1.Post("/test/fiber", emptyHandler)
	v1.Put("/test/fiber/*", emptyHandler)

	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})

	fmt.Println(printRoutesMessage)

	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "GET"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "POST"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "PUT"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber/*"))
}

func emptyHandler(c Ctx) error {
	return nil
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
