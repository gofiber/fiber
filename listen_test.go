//nolint:wrapcheck // We must not wrap errors in tests
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

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_Listen
func Test_Listen(t *testing.T) {
	app := New()

	require.Error(t, app.Listen(":99999"))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":4003", ListenConfig{DisableStartupMessage: true}))
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

		err := app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       ctx,
			OnShutdownSuccess: func() {
				mu.Lock()
				shutdown = true
				mu.Unlock()
			},
		})

		require.NoError(t, err)
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
		a.HostClient.Dial = func(_ string) (net.Conn, error) { return ln.Dial() }
		code, body, errs := a.String()

		require.Equal(t, tc.ExpectedStatusCode, code)
		require.Equal(t, tc.ExpectedBody, body)
		require.Len(t, errs, tc.ExceptedErrsLen)
	}

	mu.Lock()
	require.True(t, shutdown)
	mu.Unlock()
}

// go test -run Test_Listen_Prefork
func Test_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	require.NoError(t, app.Listen(":99999", ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

// go test -run Test_Listen_TLS
func Test_Listen_TLS(t *testing.T) {
	app := New()

	// invalid port
	require.Error(t, app.Listen(":99999", ListenConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))
}

// go test -run Test_Listen_TLS_Prefork
func Test_Listen_TLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// invalid key file content
	require.Error(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.tmpl",
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":99999", ListenConfig{
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
	require.Error(t, app.Listen(":99999", ListenConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
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
	require.Error(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/template.html",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":99999", ListenConfig{
		DisableStartupMessage: true,
		EnablePrefork:         true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}))
}

// go test -run Test_Listener
func Test_Listener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	require.NoError(t, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	t.Parallel()
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		require.NoError(t, err)
	}
	//nolint:gosec // We're in a test so using old ciphers is fine
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	//nolint:gosec // We're in a test so listening on all interfaces is fine
	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	require.NoError(t, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listener(ln))
}

// go test -run Test_Listen_TLSConfigFunc
func Test_Listen_TLSConfigFunc(t *testing.T) {
	var callTLSConfig bool
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		TLSConfigFunc: func(_ *tls.Config) {
			callTLSConfig = true
		},
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))

	require.True(t, callTLSConfig)
}

// go test -run Test_Listen_ListenerAddrFunc
func Test_Listen_ListenerAddrFunc(t *testing.T) {
	var network string
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.Network()
		},
		CertFile:    "./.github/testdata/ssl.pem",
		CertKeyFile: "./.github/testdata/ssl.key",
	}))

	require.Equal(t, "tcp", network)
}

// go test -run Test_Listen_BeforeServeFunc
func Test_Listen_BeforeServeFunc(t *testing.T) {
	var handlers uint32
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	wantErr := errors.New("test")
	require.ErrorIs(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		BeforeServeFunc: func(fiber *App) error {
			handlers = fiber.HandlersCount()

			return wantErr
		},
	}), wantErr)

	require.Zero(t, handlers)
}

// go test -run Test_Listen_ListenerNetwork
func Test_Listen_ListenerNetwork(t *testing.T) {
	var network string
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkTCP6,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.String()
		},
	}))

	require.Contains(t, network, "[::]:")

	go func() {
		time.Sleep(1000 * time.Millisecond)
		require.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkTCP4,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.String()
		},
	}))

	require.Contains(t, network, "0.0.0.0:")
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
	colors := Colors{}
	require.Contains(t, startupMessage, "https://127.0.0.1:3000")
	require.Contains(t, startupMessage, "(bound on host 0.0.0.0 and port 3000)")
	require.Contains(t, startupMessage, "Child PIDs")
	require.Contains(t, startupMessage, "11111, 22222, 33333, 44444, 55555, 60000")
	require.Contains(t, startupMessage, fmt.Sprintf("Prefork: \t\t\t%sEnabled%s", colors.Blue, colors.Reset))
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
	require.Equal(t, "Test App v3.0.0", app.Config().AppName)
	require.Contains(t, startupMessage, app.Config().AppName)
}

// go test -run Test_Listen_Master_Process_Show_Startup_MessageWithAppNameNonAscii
func Test_Listen_Master_Process_Show_Startup_MessageWithAppNameNonAscii(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
	}

	appName := "Serveur de vérification des données"
	app := New(Config{AppName: appName})

	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", false, "", cfg)
	})
	require.Contains(t, startupMessage, "Serveur de vérification des données")
}

// go test -run Test_Listen_Master_Process_Show_Startup_MessageWithDisabledPreforkAndCustomEndpoint
func Test_Listen_Master_Process_Show_Startup_MessageWithDisabledPreforkAndCustomEndpoint(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: false,
	}

	appName := "Fiber Example Application"
	app := New(Config{AppName: appName})
	startupMessage := captureOutput(func() {
		app.startupMessage("server.com:8081", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 5), cfg)
	})
	colors := Colors{}
	require.Contains(t, startupMessage, fmt.Sprintf("%sINFO%s", colors.Green, colors.Reset))
	require.Contains(t, startupMessage, fmt.Sprintf("%s%s%s", colors.Blue, appName, colors.Reset))
	require.Contains(t, startupMessage, fmt.Sprintf("%s%s%s", colors.Blue, "https://server.com:8081", colors.Reset))
	require.Contains(t, startupMessage, fmt.Sprintf("Prefork: \t\t\t%sDisabled%s", colors.Red, colors.Reset))
}

// go test -run Test_Listen_Print_Route
func Test_Listen_Print_Route(t *testing.T) {
	app := New()
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	require.Contains(t, printRoutesMessage, MethodGet)
	require.Contains(t, printRoutesMessage, "/")
	require.Contains(t, printRoutesMessage, "emptyHandler")
	require.Contains(t, printRoutesMessage, "routeName")
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

	require.Contains(t, printRoutesMessage, MethodGet)
	require.Contains(t, printRoutesMessage, "/")
	require.Contains(t, printRoutesMessage, "emptyHandler")
	require.Contains(t, printRoutesMessage, "/v1/test")
	require.Contains(t, printRoutesMessage, "POST")
	require.Contains(t, printRoutesMessage, "/v1/test/fiber")
	require.Contains(t, printRoutesMessage, "PUT")
	require.Contains(t, printRoutesMessage, "/v1/test/fiber/*")
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

func emptyHandler(_ Ctx) error {
	return nil
}
