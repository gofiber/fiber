package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log" //nolint:depguard // TODO: Required to capture output, use internal log package instead
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
	"golang.org/x/crypto/acme/autocert"
)

// go test -run Test_Listen
func Test_Listen(t *testing.T) {
	app := New()

	require.Error(t, app.Listen(":99999"))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_Listen_Graceful_Shutdown
func Test_Listen_Graceful_Shutdown(t *testing.T) {
	t.Run("Basic Graceful Shutdown", func(t *testing.T) {
		testGracefulShutdown(t, 0)
	})

	t.Run("Shutdown With Timeout", func(t *testing.T) {
		testGracefulShutdown(t, 500*time.Millisecond)
	})

	t.Run("Shutdown With Timeout Error", func(t *testing.T) {
		testGracefulShutdown(t, 1*time.Nanosecond)
	})
}

func testGracefulShutdown(t *testing.T, shutdownTimeout time.Duration) {
	t.Helper()

	var mu sync.Mutex
	var shutdown bool
	var receivedErr error

	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(10 * time.Millisecond)
		return c.SendString(c.Hostname())
	})

	ln := fasthttputil.NewInmemoryListener()
	errs := make(chan error, 1)

	app.hooks.OnPostShutdown(func(err error) error {
		mu.Lock()
		defer mu.Unlock()
		shutdown = true
		receivedErr = err
		return nil
	})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		errs <- app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       ctx,
			ShutdownTimeout:       shutdownTimeout,
		})
	}()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err == nil {
			if err := conn.Close(); err != nil {
				t.Logf("error closing connection: %v", err)
			}
			return true
		}
		return false
	}, time.Second, 100*time.Millisecond, "Server failed to become ready")

	client := fasthttp.HostClient{
		Dial: func(_ string) (net.Conn, error) { return ln.Dial() },
	}

	type testCase struct {
		expectedErr        error
		expectedBody       string
		name               string
		waitTime           time.Duration
		expectedStatusCode int
		closeConnection    bool
	}

	testCases := []testCase{
		{
			name:               "Server running normally",
			waitTime:           500 * time.Millisecond,
			expectedBody:       "example.com",
			expectedStatusCode: StatusOK,
			expectedErr:        nil,
			closeConnection:    true,
		},
		{
			name:               "Server shutdown complete",
			waitTime:           3 * time.Second,
			expectedBody:       "",
			expectedStatusCode: StatusOK,
			expectedErr:        fasthttputil.ErrInmemoryListenerClosed,
			closeConnection:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			time.Sleep(tc.waitTime)

			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			req.SetRequestURI("http://example.com")

			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)

			err := client.Do(req, resp)

			if tc.expectedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedStatusCode, resp.StatusCode())
				require.Equal(t, tc.expectedBody, utils.UnsafeString(resp.Body()))
			} else {
				require.ErrorIs(t, err, tc.expectedErr)
			}
		})
	}

	mu.Lock()
	require.True(t, shutdown)
	if shutdownTimeout == 1*time.Nanosecond {
		require.Error(t, receivedErr)
		require.ErrorIs(t, receivedErr, context.DeadlineExceeded)
	}
	require.NoError(t, <-errs)
	mu.Unlock()
}

// go test -run Test_Listen_Prefork
func Test_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New()

	require.NoError(t, app.Listen(":0", ListenConfig{DisableStartupMessage: true, EnablePrefork: true}))
}

// go test -run Test_Listen_TLSMinVersion
func Test_Listen_TLSMinVersion(t *testing.T) {
	testPreforkMaster = true

	app := New()

	// Invalid TLSMinVersion
	require.Panics(t, func() {
		_ = app.Listen(":0", ListenConfig{TLSMinVersion: tls.VersionTLS10}) //nolint:errcheck // ignore error
	})
	require.Panics(t, func() {
		_ = app.Listen(":0", ListenConfig{TLSMinVersion: tls.VersionTLS11}) //nolint:errcheck // ignore error
	})

	// Prefork
	require.Panics(t, func() {
		_ = app.Listen(":0", ListenConfig{DisableStartupMessage: true, EnablePrefork: true, TLSMinVersion: tls.VersionTLS10}) //nolint:errcheck // ignore error
	})
	require.Panics(t, func() {
		_ = app.Listen(":0", ListenConfig{DisableStartupMessage: true, EnablePrefork: true, TLSMinVersion: tls.VersionTLS11}) //nolint:errcheck // ignore error
	})

	// Valid TLSMinVersion
	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()
	require.NoError(t, app.Listen(":0", ListenConfig{TLSMinVersion: tls.VersionTLS13}))

	// Valid TLSMinVersion with Prefork
	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()
	require.NoError(t, app.Listen(":0", ListenConfig{DisableStartupMessage: true, EnablePrefork: true, TLSMinVersion: tls.VersionTLS13}))
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
		assert.NoError(t, app.Shutdown())
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
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
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
		assert.NoError(t, app.Shutdown())
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
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
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
		assert.NoError(t, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	require.NoError(t, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
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
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listener(ln))
}

// go test -run Test_Listen_TLSConfigFunc
func Test_Listen_TLSConfigFunc(t *testing.T) {
	var callTLSConfig bool
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
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

// go test -run Test_Listen_TLSConfig
func Test_Listen_TLSConfig(t *testing.T) {
	t.Parallel()

	cert, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	require.NoError(t, err)

	run := func(name string, cfg ListenConfig) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := New()

			go func() {
				time.Sleep(1000 * time.Millisecond)
				assert.NoError(t, app.Shutdown())
			}()

			require.NoError(t, app.Listen(":0", cfg))
		})
	}

	run("TLSConfig with certificates", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		},
	})

	run("TLSConfig with GetCertificate", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &cert, nil
			},
		},
	})

	run("TLSConfig ignores other TLS fields", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		},
		CertFile:       "./.github/testdata/does-not-exist.pem",
		CertKeyFile:    "./.github/testdata/does-not-exist.key",
		CertClientFile: "./.github/testdata/does-not-exist-ca.pem",
		AutoCertManager: &autocert.Manager{
			Prompt: autocert.AcceptTOS,
		},
	})
}

// go test -run Test_Listen_TLSConfig_WithTLSConfigFunc
func Test_Listen_TLSConfig_WithTLSConfigFunc(t *testing.T) {
	t.Parallel()

	cert, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	require.NoError(t, err)

	var calledTLSConfigFunc bool
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":0", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		},
		TLSConfigFunc: func(_ *tls.Config) {
			calledTLSConfigFunc = true
		},
	}))

	require.False(t, calledTLSConfigFunc)
}

// go test -run Test_Listen_AutoCert_Conflicts
func Test_Listen_AutoCert_Conflicts(t *testing.T) {
	t.Parallel()

	app := New()

	err := app.Listen(":0", ListenConfig{
		AutoCertManager: &autocert.Manager{},
		CertFile:        "./.github/testdata/ssl.pem",
		CertKeyFile:     "./.github/testdata/ssl.key",
	})
	require.ErrorIs(t, err, ErrAutoCertWithCertFile)
}

// go test -run Test_Listen_ListenerAddrFunc
func Test_Listen_ListenerAddrFunc(t *testing.T) {
	var network string
	app := New()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		assert.NoError(t, app.Shutdown())
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
		assert.NoError(t, app.Shutdown())
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
		assert.NoError(t, app.Shutdown())
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
		assert.NoError(t, app.Shutdown())
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

// go test -run Test_Listen_ListenerNetwork_Unix
func Test_Listen_ListenerNetwork_Unix(t *testing.T) {
	app := New()

	app.Get("/test", func(c Ctx) error {
		return c.SendString("all good")
	})

	var (
		f       os.FileInfo
		network string

		reqErr error
		resp   = &fasthttp.Response{}
	)

	// Create temporary directory for storing socket in
	tmp, err := os.MkdirTemp(os.TempDir(), "fiber-test")
	require.NoError(t, err)
	sock := filepath.Join(tmp, "fiber-test.sock")

	// Make sure temporary directory is cleaned up
	defer func() { assert.NoError(t, os.RemoveAll(tmp)) }()

	// Send request through socket
	go func() {
		time.Sleep(1000 * time.Millisecond)

		client := &fasthttp.HostClient{
			Addr: sock,
			Dial: func(addr string) (net.Conn, error) {
				return net.Dial("unix", addr)
			},
		}

		req := &fasthttp.Request{}
		req.SetRequestURI("http://host/test")

		reqErr = client.Do(req, resp)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(sock, ListenConfig{
		DisableStartupMessage: true,
		ListenerNetwork:       NetworkUnix,
		UnixSocketFileMode:    0o666,
		ListenerAddrFunc: func(addr net.Addr) {
			network = addr.String()
			f, err = os.Stat(network)
		},
	}))

	// Verify that listening and setting permissions works correctly
	require.Equal(t, sock, network)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o666), f.Mode().Perm())

	// Verify that request was successful
	require.NoError(t, reqErr)
	require.Equal(t, 200, resp.StatusCode())
	require.Equal(t, "all good", string(resp.Body()))
}

// go test -run Test_Listen_Master_Process_Show_Startup_Message
func Test_Listen_Master_Process_Show_Startup_Message(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
	}

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port
	require.NoError(t, ln.Close())

	childTemplate := []int{11111, 22222, 33333, 44444, 55555, 60000}
	childPIDs := make([]int, 0, len(childTemplate)*10)
	for range 10 {
		childPIDs = append(childPIDs, childTemplate...)
	}

	app := New()
	listenData := app.prepareListenData(fmt.Sprintf(":%d", port), true, &cfg, childPIDs)

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
	})
	colors := Colors{}
	require.Contains(t, startupMessage, fmt.Sprintf("https://127.0.0.1:%d", port))
	require.Contains(t, startupMessage, fmt.Sprintf("(bound on host 0.0.0.0 and port %d)", port))
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
	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port
	require.NoError(t, ln.Close())

	childTemplate := []int{11111, 22222, 33333, 44444, 55555, 60000}
	childPIDs := make([]int, 0, len(childTemplate)*10)
	for range 10 {
		childPIDs = append(childPIDs, childTemplate...)
	}

	listenData := app.prepareListenData(fmt.Sprintf(":%d", port), true, &cfg, childPIDs)

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
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

	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port
	require.NoError(t, ln.Close())

	listenData := app.prepareListenData(fmt.Sprintf(":%d", port), false, &cfg, nil)

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
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
	ln, err := net.Listen(NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := addr.Port
	require.NoError(t, ln.Close())

	listenData := app.prepareListenData(fmt.Sprintf("server.com:%d", port), true, &cfg, nil)

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
	})
	colors := Colors{}
	require.Contains(t, startupMessage, fmt.Sprintf("%sINFO%s", colors.Green, colors.Reset))
	require.Contains(t, startupMessage, fmt.Sprintf("%s%s%s", colors.Blue, appName, colors.Reset))
	expectedURL := fmt.Sprintf("https://server.com:%d", port)
	require.Contains(t, startupMessage, fmt.Sprintf("%s%s%s", colors.Blue, expectedURL, colors.Reset))
	require.Contains(t, startupMessage, fmt.Sprintf("Prefork: \t\t\t%sDisabled%s", colors.Red, colors.Reset))
}

func Test_StartupMessageCustomization(t *testing.T) {
	cfg := ListenConfig{}
	app := New()
	listenData := app.prepareListenData(":8080", false, &cfg, nil)

	app.Hooks().OnPreStartupMessage(func(data *PreStartupMessageData) error {
		data.BannerHeader = "FOOBER v98\n-------"

		data.ResetEntries()
		data.AddInfo("git_hash", "Git hash", "abc123", 3)
		data.AddInfo("version", "Version", "v98", 2)

		return nil
	})

	var post PostStartupMessageData
	app.Hooks().OnPostStartupMessage(func(data *PostStartupMessageData) error {
		post = *data

		return nil
	})

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
	})

	require.Contains(t, startupMessage, "FOOBER v98")
	require.Contains(t, startupMessage, "Git hash: \tabc123")
	require.Contains(t, startupMessage, "Version: \tv98")
	require.NotContains(t, startupMessage, "Server started on:")
	require.NotContains(t, startupMessage, "Prefork:")

	require.False(t, post.Disabled)
	require.False(t, post.IsChild)
	require.False(t, post.Prevented)
}

func Test_StartupMessageDisabledPostHook(t *testing.T) {
	cfg := ListenConfig{DisableStartupMessage: true}
	app := New()
	listenData := app.prepareListenData(":7070", false, &cfg, nil)

	var post PostStartupMessageData
	app.Hooks().OnPostStartupMessage(func(data *PostStartupMessageData) error {
		post = *data

		return nil
	})

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
	})

	require.Empty(t, startupMessage)
	require.True(t, post.Disabled)
	require.False(t, post.IsChild)
	require.False(t, post.Prevented)
}

func Test_StartupMessagePreventedByHook(t *testing.T) {
	cfg := ListenConfig{}
	app := New()
	listenData := app.prepareListenData(":9090", false, &cfg, nil)

	app.Hooks().OnPreStartupMessage(func(data *PreStartupMessageData) error {
		data.PreventDefault = true

		return nil
	})

	var post PostStartupMessageData
	app.Hooks().OnPostStartupMessage(func(data *PostStartupMessageData) error {
		post = *data

		return nil
	})

	startupMessage := captureOutput(func() {
		app.startupMessage(listenData, &cfg)
	})

	require.Empty(t, startupMessage)
	require.False(t, post.Disabled)
	require.False(t, post.IsChild)
	require.True(t, post.Prevented)
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
	go func() {
		var buf bytes.Buffer
		_, copyErr := io.Copy(&buf, reader)
		if copyErr != nil {
			panic(copyErr)
		}
		out <- buf.String() // this out channel helps in synchronization
	}()
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
