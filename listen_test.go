package fiber

import (
	"bufio"
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
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
	"github.com/valyala/fasthttp/prefork"
	"golang.org/x/crypto/acme/autocert"
)

// shutdownProbe counts the shutdown hooks of an app and keeps the error OnPostShutdown got.
type shutdownProbe struct {
	postErr error
	posted  chan struct{} // closed when OnPostShutdown first runs
	once    sync.Once
	pre     atomic.Int32
	post    atomic.Int32
}

// gracefulFixture is an app with the probe of its shutdown hooks and a GracefulContext the test cancels.
type gracefulFixture struct {
	ln     *fasthttputil.InmemoryListener // what Listener serves
	app    *App
	probe  *shutdownProbe
	cancel context.CancelFunc
	cfg    ListenConfig
}

// gracefulEntry is a way to start a server whose GracefulContext the tests cancel.
type gracefulEntry struct {
	run  func(f *gracefulFixture) error
	name string
	// closeVisible tells whether a test can see that the entry closed its listener: only
	// Listener serves one that the test holds.
	closeVisible bool
}

var (
	listenEntry = gracefulEntry{
		name: "Listen",
		run:  func(f *gracefulFixture) error { return f.app.Listen("127.0.0.1:0", f.cfg) },
	}
	listenerEntry = gracefulEntry{
		name:         "Listener",
		run:          func(f *gracefulFixture) error { return f.app.Listener(f.ln, f.cfg) },
		closeVisible: true,
	}
	gracefulEntries = []gracefulEntry{listenEntry, listenerEntry}
)

// gracefulPoint is a place of the startup where a test acts. wire registers run there.
type gracefulPoint struct {
	wire func(f *gracefulFixture, run func())
	name string
}

var (
	onListenPoint = gracefulPoint{
		name: "OnListen",
		wire: func(f *gracefulFixture, run func()) {
			f.app.Hooks().OnListen(func(ListenData) error {
				run()
				return nil
			})
		},
	}
	beforeServePoint = gracefulPoint{
		name: "BeforeServeFunc",
		wire: func(f *gracefulFixture, run func()) {
			f.cfg.BeforeServeFunc = func(*App) error {
				run()
				return nil
			}
		},
	}
	gracefulPoints = []gracefulPoint{onListenPoint, beforeServePoint}
)

// gracefulContextStates are the two states a GracefulContext can be in when a test calls Listen: not
// done, or done already (prepare cancels it). A test that reaches a point of the startup cancels the
// context there as well, which does nothing once it is done, and keeps the startup at the point until
// the shutdown has finished: that is what puts the shutdown before Serve without relying on timing.
var gracefulContextStates = []struct {
	prepare func(f *gracefulFixture)
	name    string
}{
	{name: "not done before the call", prepare: func(*gracefulFixture) {}},
	{name: "done before the call", prepare: func(f *gracefulFixture) { f.cancel() }},
}

// closeSignalListener reports the first Close of the listener it wraps.
type closeSignalListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

// stubListener counts what a gracefulListener forwards to the listener it wraps. It never hands out
// a connection: without an acceptErr its Accept reports a closed listener, so a Serve that is given
// it by mistake returns instead of calling Accept in a loop for ever.
type stubListener struct {
	acceptErr error
	closeErr  error
	accepts   atomic.Int32
	closes    atomic.Int32
}

type benchConn struct{ net.Conn }

// benchListener accepts the same connection at once, so a benchmark sees only what its wrapper adds.
type benchListener struct{ conn net.Conn }

var benchSinkConn net.Conn

// rootMiscBShutdownAfter shuts app down once d has passed, which ends the Listen or Listener call that
// the test makes next. The goroutine fails the test if the shutdown does.
func rootMiscBShutdownAfter(t *testing.T, app *App, d time.Duration) {
	t.Helper()

	go func() {
		time.Sleep(d)
		assert.NoError(t, app.Shutdown())
	}()
}

// rootrestListenFor serves app with Listen on a free port until d has passed and requires the call to end without an
// error.
func rootrestListenFor(t *testing.T, app *App, d time.Duration, cfg ...ListenConfig) {
	t.Helper()

	rootMiscBShutdownAfter(t, app, d)
	require.NoError(t, app.Listen(":0", cfg...))
}

// rootrestRequireOverRecovery requires a prefork master that listens with cfg to give up on its children at once.
func rootrestRequireOverRecovery(t *testing.T, app *App, cfg *ListenConfig) {
	t.Helper()

	cfg.DisableStartupMessage = true
	cfg.EnablePrefork = true
	cfg.PreforkRecoverThreshold = 1
	require.ErrorIs(t, app.Listen(":0", *cfg), prefork.ErrOverRecovery)
}

// rootMiscBWaitDialable waits until ln accepts a connection, which is when the server behind it is up.
func rootMiscBWaitDialable(t *testing.T, ln *fasthttputil.InmemoryListener) {
	t.Helper()

	require.Eventually(t, func() bool {
		conn, err := ln.Dial()
		if err != nil {
			return false
		}
		_ = conn.Close() //nolint:errcheck // not needed
		return true
	}, time.Second, 20*time.Millisecond, "server failed to become ready")
}

// rootMiscBSendOnNewConn writes raw on a new connection of ln. The connection stays open until the test
// ends, which keeps a request in flight for as long as the server has not answered it.
func rootMiscBSendOnNewConn(t *testing.T, ln *fasthttputil.InmemoryListener, raw string) {
	t.Helper()

	conn, err := ln.Dial()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // not needed
	_, err = conn.Write([]byte(raw))
	require.NoError(t, err)
}

// rootMiscBFreePort returns a loopback TCP port that was free a moment ago.
func rootMiscBFreePort(t *testing.T) int {
	t.Helper()

	ln := newLocalListener(t)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.NoError(t, ln.Close())

	return addr.Port
}

// rootMiscBTestCert loads the self-signed certificate that the TLS tests serve.
func rootMiscBTestCert(t *testing.T) tls.Certificate {
	t.Helper()

	cert, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	require.NoError(t, err)

	return cert
}

// rootMiscBTLSClientConfig is how the TLS tests connect: it accepts the self-signed test certificate.
func rootMiscBTLSClientConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true, // self-signed test certificate
		MinVersion:         tls.VersionTLS12,
	}
}

// rootMiscBTLSGet sends a GET over conn, whose server closes it after answering, and returns the body.
func rootMiscBTLSGet(t *testing.T, conn *tls.Conn) string {
	t.Helper()

	_, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	require.NoError(t, resp.Read(bufio.NewReader(conn)))

	return string(resp.Body())
}

// rootMiscBRequireContains fails the test unless out contains every one of wants.
func rootMiscBRequireContains(t *testing.T, out string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		require.Contains(t, out, want)
	}
}

// rootMiscBRequireNotContains fails the test if out contains any one of unwanted.
func rootMiscBRequireNotContains(t *testing.T, out string, unwanted ...string) {
	t.Helper()

	for _, s := range unwanted {
		require.NotContains(t, out, s)
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen(t *testing.T) {
	app := New()

	require.Error(t, app.Listen(":99999"))

	rootrestListenFor(t, app, 1000*time.Millisecond, ListenConfig{DisableStartupMessage: true})
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_ClosesListenerOnBeforeServeError(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", rootMiscBFreePort(t))

	app := New()
	err := app.Listen(addr, ListenConfig{
		DisableStartupMessage: true,
		BeforeServeFunc:       func(_ *App) error { return errors.New("stop before serving") },
	})
	require.Error(t, err)

	// The listener must have been closed on the error path, so the port is
	// immediately bindable again (a leaked listener would keep it bound).
	ln, err := net.Listen("tcp", addr)
	require.NoError(t, err, "listener leaked: port still bound after BeforeServeFunc error")
	require.NoError(t, ln.Close())
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_Graceful_Shutdown(t *testing.T) {
	t.Run("Basic Graceful Shutdown", func(t *testing.T) { testGracefulShutdown(t, 0) })

	t.Run("Shutdown With Timeout", func(t *testing.T) { testGracefulShutdown(t, 500*time.Millisecond) })

	t.Run("Shutdown With Timeout Error", func(t *testing.T) { testGracefulShutdown(t, 1*time.Nanosecond) })
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_ShutdownWithContext_PostShutdownHookReceivesError(t *testing.T) {
	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(10 * time.Millisecond)
		return c.SendString("ok")
	})

	hookErr := make(chan error, 1)
	app.Hooks().OnPostShutdown(func(err error) error {
		hookErr <- err
		return nil
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		_ = app.Listener(ln, ListenConfig{DisableStartupMessage: true}) //nolint:errcheck // not needed
	}()

	rootMiscBWaitDialable(t, ln)

	// Keep a request in flight so shutdown cannot drain before the 1ns deadline.
	rootMiscBSendOnNewConn(t, ln, "GET / HTTP/1.1\r\nHost: example.com\r\n")

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	shutdownErr := app.ShutdownWithContext(ctx)
	require.Error(t, shutdownErr)

	got := apptest.Recv(t, hookErr, time.Second, "OnPostShutdown hook was not called")
	// The hook must receive the actual shutdown error, not the nil value
	// captured when the defer was registered.
	require.Equal(t, shutdownErr, got)
}

func Test_GracefulShutdown_PostShutdownFiresOnce(t *testing.T) {
	t.Parallel()
	f := newGracefulFixture(t)
	f.start(listenerEntry.run)
	rootMiscBWaitDialable(t, f.ln)

	f.cancel() // trigger graceful shutdown

	// The hook must fire exactly once.
	apptest.Recv(t, f.probe.posted, 2*time.Second, "OnPostShutdown was not called")
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, int32(1), f.probe.post.Load(), "OnPostShutdown fired more than once")
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
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

	if shutdownTimeout == time.Nanosecond {
		// keep a request in flight so shutdown cannot drain to zero open
		// connections before the 1ns deadline is checked (would yield a nil error)
		rootMiscBSendOnNewConn(t, ln, "GET / HTTP/1.1\r\nHost: example.com\r\n")
	}

	client := fasthttp.HostClient{Dial: func(_ string) (net.Conn, error) { return ln.Dial() }}

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

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_Prefork(t *testing.T) {
	enableTestPreforkMaster(t)

	rootrestRequireOverRecovery(t, New(), &ListenConfig{})
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLSMinVersion(t *testing.T) {
	enableTestPreforkMaster(t)

	app := New()

	// Invalid TLSMinVersion, without and with prefork
	for _, cfg := range []ListenConfig{
		{TLSMinVersion: tls.VersionTLS10},
		{TLSMinVersion: tls.VersionTLS11},
		{DisableStartupMessage: true, EnablePrefork: true, TLSMinVersion: tls.VersionTLS10},
		{DisableStartupMessage: true, EnablePrefork: true, TLSMinVersion: tls.VersionTLS11},
	} {
		require.Panics(t, func() { _ = app.Listen(":0", cfg) }, "%+v", cfg) //nolint:errcheck // ignore error
	}

	// Valid TLSMinVersion
	rootrestListenFor(t, app, 1000*time.Millisecond, ListenConfig{TLSMinVersion: tls.VersionTLS13})

	// Valid TLSMinVersion with Prefork
	rootrestRequireOverRecovery(t, app, &ListenConfig{TLSMinVersion: tls.VersionTLS13})
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLS(t *testing.T) {
	// Sequential, as the two tests behind these cases were: each case runs a server for a second.
	tests := []struct {
		name           string
		certClientFile string
	}{
		{name: "TLS"},
		{name: "MutualTLS", certClientFile: "./.github/testdata/ca-chain.cert.pem"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := New()
			cfg := ListenConfig{
				CertFile:       "./.github/testdata/ssl.pem",
				CertKeyFile:    "./.github/testdata/ssl.key",
				CertClientFile: tc.certClientFile,
			}

			require.Error(t, app.Listen(":99999", cfg))

			rootrestListenFor(t, app, 1000*time.Millisecond, cfg)
		})
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLS_Prefork(t *testing.T) {
	// Not parallel: enableTestPreforkMaster sets a package-level flag that the other tests read.
	tests := []struct {
		name           string
		invalidKeyFile string
		certClientFile string
	}{
		{name: "TLS", invalidKeyFile: "./.github/testdata/template.tmpl"},
		{name: "MutualTLS", invalidKeyFile: "./.github/testdata/template.html", certClientFile: "./.github/testdata/ca-chain.cert.pem"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enableTestPreforkMaster(t)

			app := New()

			// invalid key file content
			require.Error(t, app.Listen(":0", ListenConfig{
				DisableStartupMessage: true,
				EnablePrefork:         true,
				CertFile:              "./.github/testdata/ssl.pem",
				CertKeyFile:           tc.invalidKeyFile,
				CertClientFile:        tc.certClientFile,
			}))

			rootrestRequireOverRecovery(t, app, &ListenConfig{
				CertFile:       "./.github/testdata/ssl.pem",
				CertKeyFile:    "./.github/testdata/ssl.key",
				CertClientFile: tc.certClientFile,
			})
		})
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listener(t *testing.T) {
	app := New()

	rootMiscBShutdownAfter(t, app, 500*time.Millisecond)

	ln := fasthttputil.NewInmemoryListener()
	require.NoError(t, app.Listener(ln))
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_App_Listener_TLS_Listener(t *testing.T) {
	config := &tls.Config{Certificates: []tls.Certificate{rootMiscBTestCert(t)}}

	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	require.NoError(t, err)

	app := New()

	rootMiscBShutdownAfter(t, app, 500*time.Millisecond)

	require.NoError(t, app.Listener(ln))
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLSConfigFunc(t *testing.T) {
	var callTLSConfig bool

	rootrestListenFor(t, New(), 1000*time.Millisecond, ListenConfig{
		DisableStartupMessage: true,
		TLSConfigFunc:         func(_ *tls.Config) { callTLSConfig = true },
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
	})

	require.True(t, callTLSConfig)
}

func Test_Listen_TLSConfig(t *testing.T) {
	t.Parallel()
	cert := rootMiscBTestCert(t)

	// run serves cfg for a second in a parallel subtest, then runs checks on what the Listen call left behind.
	run := func(name string, cfg ListenConfig, checks ...func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rootrestListenFor(t, New(), 1000*time.Millisecond, cfg)
			for _, check := range checks {
				check(t)
			}
		})
	}

	run("TLSConfig with certificates", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig:             &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
	})

	run("TLSConfig with GetCertificate", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &cert, nil },
		},
	})

	run("TLSConfig ignores other TLS fields", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig:             &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
		CertFile:              "./.github/testdata/does-not-exist.pem",
		CertKeyFile:           "./.github/testdata/does-not-exist.key",
		CertClientFile:        "./.github/testdata/does-not-exist-ca.pem",
		AutoCertManager:       &autocert.Manager{Prompt: autocert.AcceptTOS},
	})

	run("certificate files with a client CA", ListenConfig{
		DisableStartupMessage: true,
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
		CertClientFile:        "./.github/testdata/ssl.pem",
	})

	var calledTLSConfigFunc bool
	run("TLSConfig with TLSConfigFunc", ListenConfig{
		DisableStartupMessage: true,
		TLSConfig:             &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
		TLSConfigFunc:         func(_ *tls.Config) { calledTLSConfigFunc = true },
	}, func(t *testing.T) {
		t.Helper()
		require.False(t, calledTLSConfigFunc)
	})
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLSConfig_WarnsSupersededFields(t *testing.T) {
	// Not parallel: it captures the package-level log output, and Go keeps no
	// parallel test in flight while a serial one runs. Test_Listen_TLSConfig/TLSConfig_with_TLSConfigFunc
	// is parallel and warns, so it would otherwise write into this buffer.
	cert := rootMiscBTestCert(t)

	var buf bytes.Buffer
	withCapturedLogOutput(t, &buf)

	rootrestListenFor(t, New(), 500*time.Millisecond, ListenConfig{
		DisableStartupMessage: true,
		TLSConfig:             &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
		// Superseded by TLSConfig. CertClientFile is the one that matters: a
		// listener that silently drops it serves every client without asking
		// for a certificate, which reads as working mTLS until someone checks.
		CertClientFile: "./.github/testdata/ca-chain.cert.pem",
		TLSConfigFunc:  func(*tls.Config) {},
		// The supplied TLSConfig permits TLS 1.2, so this request is dropped.
		TLSMinVersion: tls.VersionTLS13,
	})

	rootMiscBRequireContains(t, buf.String(),
		"CertClientFile",
		// Not "no client certificate will be required": the supplied TLSConfig may
		// ask for one itself, and then one is. What the warning has to say is which
		// of the two decides, and that ClientAuth is what decides it.
		"required only if TLSConfig sets ClientAuth to a Require mode",
		"TLSConfigFunc",
		"TLSMinVersion",
	)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_TLSConfig_NoWarningWhenAlone(t *testing.T) {
	// Not parallel: captures the package-level log output.
	cert := rootMiscBTestCert(t)

	var buf bytes.Buffer
	withCapturedLogOutput(t, &buf)

	rootrestListenFor(t, New(), 500*time.Millisecond, ListenConfig{
		DisableStartupMessage: true,
		TLSConfig:             &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
	})

	require.NotContains(t, buf.String(), "supersedes")
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listener_WarnsIgnoredTLSFields(t *testing.T) {
	// Not parallel: captures the package-level log output.
	var buf bytes.Buffer
	withCapturedLogOutput(t, &buf)

	ln := newLocalListener(t)

	app := New()
	rootMiscBShutdownAfter(t, app, 500*time.Millisecond)

	// Listener serves ln untouched, so mTLS here is silently absent unless the
	// caller wrapped the listener themselves.
	require.NoError(t, app.Listener(ln, ListenConfig{
		DisableStartupMessage: true,
		CertClientFile:        "./.github/testdata/ca-chain.cert.pem",
	}))

	out := buf.String()
	require.Contains(t, out, "CertClientFile is ignored")
	// The supplied listener may already require a certificate, so the warning
	// names what decides rather than asserting none is asked for.
	require.Contains(t, out, "required only if the supplied listener already asks for one")
}

func Test_Listen_AutoCert_Conflicts(t *testing.T) {
	t.Parallel()
	app := New()

	require.ErrorIs(t, app.Listen(":0", ListenConfig{
		AutoCertManager: &autocert.Manager{},
		CertFile:        "./.github/testdata/ssl.pem",
		CertKeyFile:     "./.github/testdata/ssl.key",
	}), ErrAutoCertWithCertFile)
}

func Test_Listen_AutoCert_WithClientCertFile(t *testing.T) {
	t.Parallel()
	invalidClientCAPath := filepath.Join(t.TempDir(), "client-ca.pem")
	require.NoError(t, os.WriteFile(invalidClientCAPath, []byte("not a pem"), 0o600))

	testCases := []struct {
		name           string
		clientCAPath   string
		expectedErrMsg string
	}{
		{
			name:           "missing client CA file",
			clientCAPath:   "./.github/testdata/does-not-exist-ca.pem",
			expectedErrMsg: "./.github/testdata/does-not-exist-ca.pem",
		},
		{
			name:           "invalid client CA pem",
			clientCAPath:   invalidClientCAPath,
			expectedErrMsg: filepath.Base(invalidClientCAPath),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			done := make(chan struct{})
			defer close(done)

			go func() {
				select {
				case <-done:
				case <-time.After(time.Second):
					assert.NoError(t, app.Shutdown())
				}
			}()

			err := app.Listen(":0", ListenConfig{
				CertClientFile:  tc.clientCAPath,
				AutoCertManager: &autocert.Manager{Prompt: autocert.AcceptTOS},
			})
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectedErrMsg)
		})
	}
}

func Test_Listen_ClientCertErrorDoesNotSetTLSHandler(t *testing.T) {
	t.Parallel()
	invalidClientCAPath := filepath.Join(t.TempDir(), "client-ca.pem")
	require.NoError(t, os.WriteFile(invalidClientCAPath, []byte("not a pem"), 0o600))

	app := New()

	require.ErrorContains(t, app.Listen(":0", ListenConfig{
		CertFile:       "./.github/testdata/ssl.pem",
		CertKeyFile:    "./.github/testdata/ssl.key",
		CertClientFile: invalidClientCAPath,
	}), filepath.Base(invalidClientCAPath))

	c := apptest.Ctx(t, app)

	require.Nil(t, c.ClientHelloInfo())
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_ListenerAddrFunc(t *testing.T) {
	var network string

	rootrestListenFor(t, New(), 1000*time.Millisecond, ListenConfig{
		DisableStartupMessage: true,
		ListenerAddrFunc:      func(addr net.Addr) { network = addr.Network() },
		CertFile:              "./.github/testdata/ssl.pem",
		CertKeyFile:           "./.github/testdata/ssl.key",
	})

	require.Equal(t, "tcp", network)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_BeforeServeFunc(t *testing.T) {
	var handlers uint32
	app := New()

	rootMiscBShutdownAfter(t, app, 1000*time.Millisecond)

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

// Listen serves with the close hook installed.
func Test_Listen_ClosesAfterContentLengthWithTransferEncoding(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/", func(c Ctx) error { return c.SendString("ok") })

	addrCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen("127.0.0.1:0", ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       NetworkTCP4,
			ListenerAddrFunc:      func(addr net.Addr) { addrCh <- addr.String() },
		})
	}()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-errCh)
	})

	addr := apptest.Recv(t, addrCh, 5*time.Second, "the server did not start")

	responses, _ := rawExchange(t, tcpDialer(addr),
		"POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 4\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n", 1)
	require.Equal(t, StatusOK, responses[0].status)
	require.True(t, responses[0].closes, "Connection: close expected")
}

// skipIfNoIPv6 skips the test on hosts without IPv6 support (e.g. some CI containers).
func skipIfNoIPv6(t *testing.T) {
	t.Helper()

	probe, err := net.Listen(NetworkTCP6, "[::1]:0")
	if err != nil {
		t.Skipf("skipping: IPv6 is not available: %v", err)
	}
	require.NoError(t, probe.Close())
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_ListenerNetwork(t *testing.T) {
	skipIfNoIPv6(t)

	var network string
	app := New()

	for _, tc := range []struct{ network, want string }{{NetworkTCP6, "[::]:"}, {NetworkTCP4, "0.0.0.0:"}} {
		rootrestListenFor(t, app, 1000*time.Millisecond, ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       tc.network,
			ListenerAddrFunc:      func(addr net.Addr) { network = addr.String() },
		})

		require.Contains(t, network, tc.want)
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_ListenerNetwork_Unix(t *testing.T) {
	app := New()

	app.Get("/test", func(c Ctx) error { return c.SendString("all good") })

	var (
		f       os.FileInfo
		network string

		reqErr error
		resp   = &fasthttp.Response{}
	)

	tmp, err := os.MkdirTemp(os.TempDir(), "fiber-test")
	require.NoError(t, err)
	sock := filepath.Join(tmp, "fiber-test.sock")
	defer func() { assert.NoError(t, os.RemoveAll(tmp)) }()

	go func() {
		time.Sleep(1000 * time.Millisecond)

		client := &fasthttp.HostClient{
			Addr: sock,
			Dial: func(addr string) (net.Conn, error) { return net.Dial("unix", addr) },
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

	require.Equal(t, sock, network)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o666), f.Mode().Perm())

	require.NoError(t, reqErr)
	require.Equal(t, 200, resp.StatusCode())
	require.Equal(t, "all good", string(resp.Body()))
}

// Test_Listen_Master_Process_Show_Startup_Message renders the startup message of a master process for a host, an
// app name and child PIDs, and checks the lines that depend on them.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_Master_Process_Show_Startup_Message(t *testing.T) {
	// Not parallel: captureOutput redirects the process-wide stdout.
	childPIDs := slices.Repeat([]int{11111, 22222, 33333, 44444, 55555, 60000}, 10)
	colors := Colors{}
	for _, tc := range []struct {
		contains  func(port int) []string // lines besides the app name, given the port of the message
		name      string
		appName   string
		host      string
		childPIDs []int
		prefork   bool
		isTLS     bool
	}{
		{name: "prefork with child PIDs", prefork: true, isTLS: true, childPIDs: childPIDs, contains: func(port int) []string {
			return []string{
				fmt.Sprintf("https://127.0.0.1:%d", port),
				fmt.Sprintf("(bound on host 0.0.0.0 and port %d)", port),
				"Child PIDs",
				"11111, 22222, 33333, 44444, 55555, 60000",
				fmt.Sprintf("Prefork: \t\t\t%sEnabled%s", colors.Blue, colors.Reset),
			}
		}},
		{name: "app name with child PIDs", appName: "Test App v3.0.0", prefork: true, isTLS: true, childPIDs: childPIDs},
		{name: "non-ASCII app name", appName: "Serveur de vérification des données", prefork: true},
		{
			name: "disabled prefork and custom endpoint", appName: "Fiber Example Application", host: "server.com",
			isTLS: true,
			contains: func(port int) []string {
				return []string{
					fmt.Sprintf("%sINFO%s", colors.Green, colors.Reset),
					fmt.Sprintf("%s%s%s", colors.Blue, "Fiber Example Application", colors.Reset),
					fmt.Sprintf("%s%s%s", colors.Blue, fmt.Sprintf("https://server.com:%d", port), colors.Reset),
					fmt.Sprintf("Prefork: \t\t\t%sDisabled%s", colors.Red, colors.Reset),
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ListenConfig{EnablePrefork: tc.prefork}
			app := New(Config{AppName: tc.appName})
			port := rootMiscBFreePort(t)
			listenData := app.prepareListenData(fmt.Sprintf("%s:%d", tc.host, port), tc.isTLS, &cfg, tc.childPIDs)

			startupMessage := captureOutput(func() { app.startupMessage(listenData, &cfg) })
			require.Equal(t, tc.appName, app.Config().AppName)
			require.Contains(t, startupMessage, tc.appName)
			if tc.contains != nil {
				rootMiscBRequireContains(t, startupMessage, tc.contains(port)...)
			}
		})
	}
}

// Test_Listen_StartupMessage_Hooks prints the startup message with an OnPreStartupMessage hook that customizes or
// prevents it, or with the message disabled, and checks what the OnPostStartupMessage hook is told.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_StartupMessage_Hooks(t *testing.T) {
	// Not parallel: captureOutput redirects the process-wide stdout.
	for _, tc := range []struct {
		pre       func(data *PreStartupMessageData) error // the OnPreStartupMessage hook, if any
		name      string
		addr      string
		contains  []string
		absent    []string
		cfg       ListenConfig
		silent    bool // nothing is printed
		disabled  bool // PostStartupMessageData.Disabled
		prevented bool // PostStartupMessageData.Prevented
	}{
		{
			name: "Customization", addr: ":8080",
			pre: func(data *PreStartupMessageData) error {
				data.BannerHeader = "FOOBER v98\n-------"

				data.ResetEntries()
				data.AddInfo("git_hash", "Git hash", "abc123", 3)
				data.AddInfo("version", "Version", "v98", 2)

				return nil
			},
			contains: []string{"FOOBER v98", "Git hash: \tabc123", "Version: \tv98"},
			absent:   []string{"Server started on:", "Prefork:"},
		},
		{name: "DisabledPostHook", addr: ":7070", cfg: ListenConfig{DisableStartupMessage: true}, silent: true, disabled: true},
		{
			name: "PreventedByHook", addr: ":9090", silent: true, prevented: true,
			pre: func(data *PreStartupMessageData) error {
				data.PreventDefault = true

				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			app := New()
			listenData := app.prepareListenData(tc.addr, false, &cfg, nil)

			if tc.pre != nil {
				app.Hooks().OnPreStartupMessage(tc.pre)
			}
			var post PostStartupMessageData
			app.Hooks().OnPostStartupMessage(func(data *PostStartupMessageData) error {
				post = *data

				return nil
			})

			startupMessage := captureOutput(func() { app.startupMessage(listenData, &cfg) })

			if tc.silent {
				require.Empty(t, startupMessage)
			}
			rootMiscBRequireContains(t, startupMessage, tc.contains...)
			rootMiscBRequireNotContains(t, startupMessage, tc.absent...)
			require.Equal(t, tc.disabled, post.Disabled)
			require.False(t, post.IsChild)
			require.Equal(t, tc.prevented, post.Prevented)
		})
	}
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_Print_Route(t *testing.T) {
	app := New()
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() { app.printRoutesMessage() })
	rootMiscBRequireContains(t, printRoutesMessage, MethodGet, "/", "emptyHandler", "routeName")
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Listen_Print_Route_With_Group(t *testing.T) {
	app := New()
	app.Get("/", emptyHandler)

	v1 := app.Group("v1")
	v1.Get("/test", emptyHandler).Name("v1")
	v1.Post("/test/fiber", emptyHandler)
	v1.Put("/test/fiber/*", emptyHandler)

	printRoutesMessage := captureOutput(func() { app.printRoutesMessage() })

	rootMiscBRequireContains(t, printRoutesMessage,
		MethodGet, "/", "emptyHandler",
		"/v1/test",
		"POST", "/v1/test/fiber",
		"PUT", "/v1/test/fiber/*",
	)
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

// Listener serves the listener it is handed, so every TLS field in the config
// is dropped. The end-to-end tests above cover CertClientFile, which returns
// before the rest; this drives the remaining fields, the singular/plural
// wording, and the two suffixes — a plain listener is not serving TLS at all,
// which is worth saying, and a TLS one already is.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_WarnIgnoredTLSFieldsOnListener_Branches(t *testing.T) {
	// Not parallel: captures the package-level log output.
	cert := rootMiscBTestCert(t)

	plain := newLocalListener(t)
	t.Cleanup(func() { _ = plain.Close() }) //nolint:errcheck // closing a test listener

	secure := newLocalListener(t)
	tlsLn := tls.NewListener(secure, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})
	t.Cleanup(func() { _ = tlsLn.Close() }) //nolint:errcheck // closing a test listener

	tests := []struct {
		listener net.Listener
		contains []string
		absent   []string
		name     string
		cfg      ListenConfig
	}{
		{name: "nothing to report", cfg: ListenConfig{}, listener: plain, absent: []string{"ignored"}},
		{
			name:     "one field reads singular",
			cfg:      ListenConfig{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			listener: tlsLn,
			contains: []string{"TLSConfig", "is ignored."},
			absent:   []string{"not serving TLS"},
		},
		{
			// A plain listener is not serving TLS at all, which is the more
			// surprising half and is said outright.
			name:     "a plain listener says so",
			cfg:      ListenConfig{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
			listener: plain,
			contains: []string{"TLSConfig", "is ignored", "this listener is not serving TLS"},
		},
		{
			name: "several fields read plural",
			cfg: ListenConfig{
				CertFile:      "./.github/testdata/ssl.pem",
				CertKeyFile:   "./.github/testdata/ssl.key",
				TLSConfigFunc: func(*tls.Config) {},
			},
			listener: plain,
			contains: []string{"CertFile/CertKeyFile", "TLSConfigFunc", "are ignored"},
		},
		{
			name:     "the certificate manager counts too",
			cfg:      ListenConfig{AutoCertManager: &autocert.Manager{}},
			listener: plain,
			contains: []string{"AutoCertManager", "is ignored"},
		},
		{
			name:     "a raised minimum version is named",
			cfg:      ListenConfig{TLSMinVersion: tls.VersionTLS13},
			listener: plain,
			contains: []string{"TLSMinVersion", "is ignored"},
		},
		{
			// The default and the unset zero both mean nothing was displaced.
			name:     "the default minimum version is not",
			cfg:      ListenConfig{TLSMinVersion: tls.VersionTLS12},
			listener: plain,
			absent:   []string{"ignored"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			withCapturedLogOutput(t, &buf)

			cfg := tc.cfg
			warnIgnoredTLSFieldsOnListener(&cfg, tc.listener)

			rootMiscBRequireContains(t, buf.String(), tc.contains...)
			rootMiscBRequireNotContains(t, buf.String(), tc.absent...)
		})
	}
}

// TLSConfig replaces the other TLS settings rather than seeding them, so each
// one it displaces is named. The end-to-end test above covers the pair a real
// deployment hits; this drives the two that are otherwise unreached.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_WarnSupersededTLSFields_Branches(t *testing.T) {
	// Not parallel: captures the package-level log output.
	tests := []struct {
		name     string
		contains []string
		absent   []string
		cfg      ListenConfig
	}{
		{name: "nothing to report", cfg: ListenConfig{}, absent: []string{"supersedes"}},
		{
			name:     "certificate files",
			cfg:      ListenConfig{CertFile: "./.github/testdata/ssl.pem"},
			contains: []string{"supersedes CertFile/CertKeyFile"},
		},
		{
			name:     "certificate manager",
			cfg:      ListenConfig{AutoCertManager: &autocert.Manager{}},
			contains: []string{"supersedes AutoCertManager"},
		},
		{
			name:     "a raised minimum version",
			cfg:      ListenConfig{TLSMinVersion: tls.VersionTLS13},
			contains: []string{"supersedes TLSMinVersion"},
		},
		{
			// The default and the unset zero both mean nothing was displaced.
			name:   "the default minimum version is not reported",
			cfg:    ListenConfig{TLSMinVersion: tls.VersionTLS12},
			absent: []string{"supersedes"},
		},
		{
			name: "all of them at once",
			cfg: ListenConfig{
				CertKeyFile:     "./.github/testdata/ssl.key",
				AutoCertManager: &autocert.Manager{},
				TLSConfigFunc:   func(*tls.Config) {},
				TLSMinVersion:   tls.VersionTLS13,
			},
			contains: []string{"CertFile/CertKeyFile", "TLSMinVersion", "AutoCertManager", "TLSConfigFunc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			withCapturedLogOutput(t, &buf)

			cfg := tc.cfg
			warnSupersededTLSFields(&cfg)

			rootMiscBRequireContains(t, buf.String(), tc.contains...)
			rootMiscBRequireNotContains(t, buf.String(), tc.absent...)
		})
	}
}

// Test_Listen_StaleTLSMinVersionReachesTheDiagnostics covers where the
// unsupported-version check is asked.
//
// A stale tls.VersionTLS11 beside a TLSConfig, or on App.Listener, is a value
// neither path reads. Rejecting it in the defaults panicked before either
// warning could run, so the caller learned nothing about the field being
// ignored — the one thing worth telling them. Where the version is read, it is
// still rejected.
//
// conventions:ignore parallel: the warning subtests capture the package-level log output
//
//nolint:tparallel // the warning subtests capture the package-level log output
func Test_Listen_StaleTLSMinVersionReachesTheDiagnostics(t *testing.T) {
	t.Run("superseded by TLSConfig", func(t *testing.T) {
		var buf bytes.Buffer
		withCapturedLogOutput(t, &buf)

		cfg := listenConfigDefault(ListenConfig{
			TLSConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			TLSMinVersion: tls.VersionTLS11,
		})
		require.NotPanics(t, func() { warnSupersededTLSFields(&cfg) })
		require.Contains(t, buf.String(), "TLSMinVersion")
	})

	t.Run("ignored on a supplied listener", func(t *testing.T) {
		var buf bytes.Buffer
		withCapturedLogOutput(t, &buf)

		ln := newLocalListener(t)
		defer func() { require.NoError(t, ln.Close()) }()

		cfg := listenConfigDefault(ListenConfig{TLSMinVersion: tls.VersionTLS11})
		require.NotPanics(t, func() { warnIgnoredTLSFieldsOnListener(&cfg, ln) })
		require.Contains(t, buf.String(), "TLSMinVersion")
	})

	t.Run("still rejected where it is read", func(t *testing.T) {
		t.Parallel()
		cfg := listenConfigDefault(ListenConfig{TLSMinVersion: tls.VersionTLS11})
		require.PanicsWithValue(t,
			"unsupported TLS version, please use tls.VersionTLS12 or tls.VersionTLS13",
			func() { validateTLSMinVersion(&cfg) })
	})

	t.Run("the default is accepted", func(t *testing.T) {
		t.Parallel()
		cfg := listenConfigDefault()
		require.Equal(t, uint16(tls.VersionTLS12), cfg.TLSMinVersion)
		require.NotPanics(t, func() { validateTLSMinVersion(&cfg) })
	})
}

func Test_Listen_GracefulShutdown_HooksOnce(t *testing.T) {
	t.Parallel()
	f := newGracefulFixture(t)
	errs := f.start(listenerEntry.run)
	rootMiscBWaitDialable(t, f.ln)

	require.NoError(t, f.app.Shutdown())
	require.NoError(t, awaitServe(t, errs))

	// Leave the graceful-shutdown goroutine time to misfire before asserting.
	time.Sleep(300 * time.Millisecond)
	require.Equal(t, int32(1), f.probe.pre.Load(), "OnPreShutdown must fire exactly once")
	require.Equal(t, int32(1), f.probe.post.Load(), "OnPostShutdown must fire exactly once")
}

// Issue #3370: Listen must not hand control back while requests are draining.
func Test_Listen_GracefulShutdown_DrainsBeforeReturn(t *testing.T) {
	t.Parallel()
	var handlerDone atomic.Bool
	inHandler := make(chan struct{}, 1)

	app := New()
	app.Get("/", func(c Ctx) error {
		inHandler <- struct{}{}
		time.Sleep(500 * time.Millisecond)
		handlerDone.Store(true)
		return c.SendString("ok")
	})

	gctx, gcancel := context.WithCancel(context.Background())
	defer gcancel()

	ln := fasthttputil.NewInmemoryListener()
	errs := make(chan error, 1)
	go func() {
		errs <- app.Listener(ln, ListenConfig{ //nolint:contextcheck // the context is the shutdown trigger, not a request context
			DisableStartupMessage: true,
			GracefulContext:       gctx,
			ShutdownTimeout:       5 * time.Second,
		})
	}()

	rootMiscBWaitDialable(t, ln)

	client := fasthttp.HostClient{Dial: func(_ string) (net.Conn, error) { return ln.Dial() }}
	responses := make(chan string, 1)
	go func() {
		req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(resp)
		req.SetRequestURI("http://example.com/")
		if err := client.Do(req, resp); err != nil {
			responses <- err.Error()
			return
		}
		responses <- fmt.Sprintf("%d %s", resp.StatusCode(), resp.Body())
	}()

	apptest.Recv(t, inHandler, 2*time.Second, "request never reached the handler")
	gcancel()

	require.NoError(t, apptest.Recv(t, errs, 3*time.Second, "Listener did not return"))
	require.True(t, handlerDone.Load(), "Listener returned while a request was still in flight")

	// Awaited rather than checked at the return: the client reads on its own goroutine.
	got := apptest.Recv(t, responses, time.Second, "client never received the response")
	require.Equal(t, "200 ok", got, "the drained request lost its response")
}

func Test_Listen_GracefulShutdown_NoHooksOnListenError(t *testing.T) {
	t.Parallel()
	f := newGracefulFixture(t)

	require.Error(t, f.app.Listen(":99999", f.cfg))

	time.Sleep(300 * time.Millisecond)
	require.Zero(t, f.probe.pre.Load()+f.probe.post.Load(), "shutdown hooks must not run for a server that never started")
}

func Test_ListenConfigDefault_ShutdownTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  []ListenConfig
		want time.Duration
	}{
		{name: "without config", want: 10 * time.Second},
		{
			name: "user config with zero value gets the default",
			cfg:  []ListenConfig{{GracefulContext: context.Background()}},
			want: 10 * time.Second,
		},
		{name: "explicit value is kept", cfg: []ListenConfig{{ShutdownTimeout: 3 * time.Second}}, want: 3 * time.Second},
		{name: "negative value disables the timeout", cfg: []ListenConfig{{ShutdownTimeout: -1}}, want: time.Duration(-1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, listenConfigDefault(tc.cfg...).ShutdownTimeout)
		})
	}
}

func Test_GracefulShutdown_NegativeTimeoutWaitsIndefinitely(t *testing.T) {
	t.Parallel()
	inHandler := make(chan struct{}, 1)
	app := New()
	app.Get("/", func(c Ctx) error {
		inHandler <- struct{}{}
		time.Sleep(300 * time.Millisecond)
		return c.SendString("ok")
	})

	hookErr := make(chan error, 1)
	app.Hooks().OnPostShutdown(func(err error) error {
		hookErr <- err
		return nil
	})

	gctx, gcancel := context.WithCancel(context.Background())
	defer gcancel()

	ln := fasthttputil.NewInmemoryListener()
	errs := make(chan error, 1)
	go func() {
		errs <- app.Listener(ln, ListenConfig{ //nolint:contextcheck // the context is the shutdown trigger, not a request context
			DisableStartupMessage: true,
			GracefulContext:       gctx,
			ShutdownTimeout:       -1,
		})
	}()

	rootMiscBWaitDialable(t, ln)

	// Keep a request in flight while the context is canceled.
	rootMiscBSendOnNewConn(t, ln, "GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")

	apptest.Recv(t, inHandler, 2*time.Second, "request never reached the handler")

	gcancel()

	require.NoError(t, apptest.Recv(t, hookErr, 3*time.Second, "OnPostShutdown was not called"))
	require.NoError(t, <-errs)
}

// listenWait is how long these tests wait for something that has to happen. A hang is what they
// catch, so the bound only has to outlast a slow machine.
const listenWait = 5 * time.Second

func newShutdownProbe(app *App) *shutdownProbe {
	p := &shutdownProbe{posted: make(chan struct{})}
	app.Hooks().OnPreShutdown(func() error {
		p.pre.Add(1)
		return nil
	})
	app.Hooks().OnPostShutdown(func(err error) error {
		p.post.Add(1)
		p.once.Do(func() {
			p.postErr = err
			close(p.posted)
		})
		return nil
	})

	return p
}

// await returns once the shutdown has finished, or after listenWait. Called from the
// startup, it puts the whole shutdown ahead of Serve without relying on timing. It reports nothing,
// on purpose: when the shutdown does not come, Serve never returns and awaitServe fails the test
// from the test goroutine, whereas a report from here could land after the test has completed,
// which panics the whole test binary.
func (p *shutdownProbe) await() {
	select {
	case <-p.posted:
	case <-time.After(listenWait):
	}
}

// requireCleanShutdown expects one shutdown that finished without an error.
func (p *shutdownProbe) requireCleanShutdown(t *testing.T) {
	t.Helper()

	select {
	case <-p.posted:
	default:
		t.Fatal("OnPostShutdown did not run before the server returned")
	}
	require.Equal(t, int32(1), p.pre.Load(), "OnPreShutdown must fire exactly once")
	require.Equal(t, int32(1), p.post.Load(), "OnPostShutdown must fire exactly once")
	require.NoError(t, p.postErr)
}

// awaitServe returns what Listen, Listener or Serve returned. Not returning is the bug of
// issue #4763: the server keeps serving after the shutdown that should have stopped it.
func awaitServe(t *testing.T, errs <-chan error) error {
	t.Helper()

	return apptest.Recv(t, errs, listenWait, "the server kept serving: it did not return")
}

// getOnNewConn makes one request over a new connection of ln, so that it needs an Accept.
func getOnNewConn(ln *fasthttputil.InmemoryListener) (int, error) {
	client := fasthttp.HostClient{
		Dial:        func(string) (net.Conn, error) { return ln.Dial() },
		ReadTimeout: 2 * time.Second,
	}
	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI("http://example.com/")
	req.Header.SetConnectionClose()

	err := client.Do(req, resp)
	return resp.StatusCode(), err
}

// waitUntilServing returns once ln answers requests.
func waitUntilServing(t *testing.T, ln *fasthttputil.InmemoryListener) {
	t.Helper()

	require.Eventually(t, func() bool {
		code, err := getOnNewConn(ln)
		return err == nil && code == StatusOK
	}, 2*time.Second, 10*time.Millisecond, "server failed to become ready")
}

func newGracefulFixture(t *testing.T) *gracefulFixture {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("ok") })

	return &gracefulFixture{
		ln:     fasthttputil.NewInmemoryListener(),
		app:    app,
		probe:  newShutdownProbe(app),
		cancel: cancel,
		cfg:    ListenConfig{DisableStartupMessage: true, GracefulContext: ctx},
	}
}

// start runs an entry point on its own goroutine, as main would, and returns what it returns.
func (f *gracefulFixture) start(run func(f *gracefulFixture) error) <-chan error {
	errs := make(chan error, 1)
	go func() { errs <- run(f) }()

	return errs
}

// cancelAndAwait cancels the context, which does nothing to one that is done already, and returns
// once the shutdown has finished.
func (f *gracefulFixture) cancelAndAwait() {
	f.cancel()
	f.probe.await()
}

// requireStopped expects the server to have returned nil after exactly one clean shutdown.
func (f *gracefulFixture) requireStopped(t *testing.T, errs <-chan error) {
	t.Helper()

	require.NoError(t, awaitServe(t, errs))
	f.probe.requireCleanShutdown(t)
}

// requireClosed expects the listener of the entry to be closed, as a shutdown closes the one of
// a running server. A second Close reports the first one; a Dial would wait for an Accept on a
// listener that is still open, and hang the test instead of failing it.
func (e gracefulEntry) requireClosed(t *testing.T, f *gracefulFixture) {
	t.Helper()

	if !e.closeVisible {
		return
	}
	require.ErrorIs(t, f.ln.Close(), fasthttputil.ErrInmemoryListenerClosed)
}

// Issue #4763: a GracefulContext done before the server serves left Listen and Listener serving, as
// fasthttp's Shutdown closes nothing before Serve has registered the listener. Every case runs the
// whole shutdown before Serve, so none of them relies on timing.
func Test_GracefulContext_DoneBeforeServe(t *testing.T) {
	t.Parallel()
	for _, entry := range gracefulEntries {
		for _, point := range gracefulPoints {
			for _, state := range gracefulContextStates {
				t.Run(entry.name+"/"+point.name+"/"+state.name, func(t *testing.T) {
					t.Parallel()
					f := newGracefulFixture(t)
					point.wire(f, func() { f.cancelAndAwait() })
					state.prepare(f)

					f.requireStopped(t, f.start(entry.run))
					entry.requireClosed(t, f)
				})
			}
		}
	}
}

// A bind that fails never started a server, so no shutdown hook runs, even for a context that is
// already done. The docs promise "not at all when Listen fails before serving".
func Test_Listen_GracefulContext_BindFailureRunsNoHooks(t *testing.T) {
	t.Parallel()
	for _, state := range gracefulContextStates {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			f := newGracefulFixture(t)
			state.prepare(f)

			require.Error(t, f.app.Listen(":99999", f.cfg))
			require.Zero(t, f.probe.pre.Load(), "OnPreShutdown must not run for a server that never started")
			require.Zero(t, f.probe.post.Load(), "OnPostShutdown must not run for a server that never started")
		})
	}
}

// The listener Listen created is closed, not left bound after Listen returned.
func Test_Listen_GracefulContext_ClosesListener(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "fiber-test")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(dir)) })
	sock := filepath.Join(dir, "fiber-test.sock")

	f := newGracefulFixture(t)
	f.cfg.ListenerNetwork = NetworkUnix
	onListenPoint.wire(f, func() { f.cancelAndAwait() })

	f.requireStopped(t, f.start(func(f *gracefulFixture) error { return f.app.Listen(sock, f.cfg) }))

	// Closing a unix listener removes its socket file, which makes the file proof of the close.
	_, err = os.Stat(sock)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func (l *closeSignalListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })

	return err //nolint:wrapcheck // a test double, passing on what the wrapped listener returned
}

// Accept can see the drain before fasthttp's Shutdown gets to close the listener. The listener is
// then closed twice, and the second close, by Shutdown, must not hand OnPostShutdown an error.
func Test_gracefulRun_Draining_ClosesListenerOnce(t *testing.T) {
	t.Parallel()
	f := newGracefulFixture(t)
	ln := &closeSignalListener{Listener: f.ln, closed: make(chan struct{})}

	f.app.startupProcess()
	graceful := f.app.newGracefulRun(&f.cfg)
	served := f.start(func(*gracefulFixture) error { return f.app.serve(graceful, ln) })
	waitUntilServing(t, f.ln)

	// Drain as shutdown does once the OnPreShutdown hooks are done, but close nothing yet. The
	// next connection makes Serve call Accept again, which sees the drain.
	graceful.beginDraining()
	code, err := getOnNewConn(f.ln)
	require.NoError(t, err)
	require.Equal(t, StatusOK, code)
	apptest.Recv(t, ln.closed, listenWait, "Accept did not see the drain")
	require.NoError(t, awaitServe(t, served))
	graceful.stopAndWait()

	// fasthttp still holds the listener Accept closed, and its Shutdown closes it once more.
	require.NoError(t, f.app.runShutdown(context.Background(), nil))
	f.probe.requireCleanShutdown(t)
}

// A Shutdown after the early cancel succeeds, although fasthttp still holds the listener Serve refused.
func Test_GracefulContext_ShutdownAfterEarlyCancel(t *testing.T) {
	t.Parallel()
	f := newGracefulFixture(t)
	onListenPoint.wire(f, func() { f.cancelAndAwait() })

	require.NoError(t, awaitServe(t, f.start(listenerEntry.run)))
	require.NoError(t, f.app.Shutdown())
}

// OnPreShutdown runs before the server stops accepting, so the server keeps serving while a hook
// holds the shutdown, whether the cancel came while it served or while it started up.
func Test_GracefulContext_ServesDuringOnPreShutdown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		prepare func(f *gracefulFixture)
		trigger func(t *testing.T, f *gracefulFixture)
		name    string
	}{
		{
			name: "canceled while serving",
			trigger: func(t *testing.T, f *gracefulFixture) {
				t.Helper()

				waitUntilServing(t, f.ln)
				f.cancel()
			},
		},
		{
			// Serve only starts after the shutdown began, with the hook still holding it.
			name: "canceled during startup",
			prepare: func(f *gracefulFixture) {
				f.app.Hooks().OnListen(func(ListenData) error {
					f.cancel()
					return nil
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newGracefulFixture(t)
			inHook := make(chan struct{})
			release := make(chan struct{})
			f.app.Hooks().OnPreShutdown(func() error {
				close(inHook)
				<-release
				return nil
			})

			if tc.prepare != nil {
				tc.prepare(f)
			}
			errs := f.start(listenerEntry.run)
			if tc.trigger != nil {
				tc.trigger(t, f)
			}

			apptest.Recv(t, inHook, listenWait, "OnPreShutdown did not run")
			for i := range 3 {
				code, err := getOnNewConn(f.ln)
				require.NoError(t, err, "request %d while OnPreShutdown holds the shutdown", i)
				require.Equal(t, StatusOK, code)
			}

			close(release)
			f.requireStopped(t, errs)
		})
	}
}

func (s *stubListener) Accept() (net.Conn, error) {
	s.accepts.Add(1)
	if s.acceptErr == nil {
		return nil, net.ErrClosed
	}

	return nil, s.acceptErr
}

func (s *stubListener) Close() error {
	s.closes.Add(1)
	return s.closeErr
}

func (*stubListener) Addr() net.Addr { return &net.TCPAddr{} }

func Test_gracefulListener(t *testing.T) {
	t.Parallel()
	t.Run("forwards Accept until the shutdown is draining", func(t *testing.T) {
		t.Parallel()
		acceptErr := errors.New("accept failed")
		stub := &stubListener{acceptErr: acceptErr}
		ln := &gracefulListener{Listener: stub, draining: new(atomic.Bool)}

		_, err := ln.Accept()
		require.ErrorIs(t, err, acceptErr)
		require.Equal(t, int32(1), stub.accepts.Load())
		require.Zero(t, stub.closes.Load())
	})

	t.Run("refuses and closes once the shutdown is draining", func(t *testing.T) {
		t.Parallel()
		draining := new(atomic.Bool)
		draining.Store(true)
		stub := &stubListener{}
		ln := &gracefulListener{Listener: stub, draining: draining}

		for range 3 {
			conn, err := ln.Accept()
			require.Nil(t, conn)
			require.ErrorIs(t, err, net.ErrClosed)
		}
		require.Zero(t, stub.accepts.Load(), "a draining server must not reach the wrapped Accept")
		require.Equal(t, int32(1), stub.closes.Load())
	})

	t.Run("closes the wrapped listener once and repeats its result", func(t *testing.T) {
		t.Parallel()
		closeErr := errors.New("close failed")
		stub := &stubListener{closeErr: closeErr}
		ln := &gracefulListener{Listener: stub, draining: new(atomic.Bool)}

		for range 3 {
			require.ErrorIs(t, ln.Close(), closeErr)
		}
		require.Equal(t, int32(1), stub.closes.Load())
	})
}

// A failed close of a draining listener is logged once, by the Accept that made it. When Shutdown
// closed the listener first, the error is already on its way to OnPostShutdown and Accept logs nothing.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_gracefulListener_LogsAFailedCloseOnce(t *testing.T) {
	// Not parallel: redirects the process-wide logger output.
	closeErr := errors.New("close failed")
	message := []byte("failed to close the listener: close failed")

	// logged returns what fn logs. The output goes back to stderr before the buffer is read, which
	// waits for a write in progress, so a goroutine of another test cannot race the read.
	t.Cleanup(func() { fiberlog.SetOutput(os.Stderr) })
	logged := func(fn func()) []byte {
		var buf bytes.Buffer
		fiberlog.SetOutput(&buf)
		fn()
		fiberlog.SetOutput(os.Stderr)

		return buf.Bytes()
	}

	// Accept closes the draining listener first: the failure is logged once, however often Accept is
	// called, and a later Close still gets it without logging again.
	draining := new(atomic.Bool)
	draining.Store(true)
	stub := &stubListener{closeErr: closeErr}
	ln := &gracefulListener{Listener: stub, draining: draining}

	out := logged(func() {
		for range 3 {
			conn, err := ln.Accept()
			require.Nil(t, conn)
			require.ErrorIs(t, err, net.ErrClosed)
		}
	})
	require.Equal(t, 1, bytes.Count(out, message), "logged: %q", out)
	require.Equal(t, int32(1), stub.closes.Load())

	out = logged(func() { require.ErrorIs(t, ln.Close(), closeErr) })
	require.Empty(t, string(out))

	// Shutdown closes the listener first: the error is its to report, and Accept has nothing to log.
	draining = new(atomic.Bool)
	stub = &stubListener{closeErr: closeErr}
	ln = &gracefulListener{Listener: stub, draining: draining}

	out = logged(func() {
		require.ErrorIs(t, ln.Close(), closeErr)
		draining.Store(true)

		_, err := ln.Accept()
		require.ErrorIs(t, err, net.ErrClosed)
	})
	require.Empty(t, string(out))
	require.Equal(t, int32(1), stub.closes.Load())
}

func Test_gracefulRun(t *testing.T) {
	t.Parallel()
	t.Run("without a GracefulContext the watch is inert and hands the listener back as it is", func(t *testing.T) {
		t.Parallel()
		watch := New().watchGracefulContext(&ListenConfig{})
		plain := &stubListener{}

		require.IsType(t, inertWatch{}, watch)
		require.Same(t, plain, watch.wrap(plain))
		watch.stopAndWait()
	})

	t.Run("with a GracefulContext the watch is a run, which wraps the listener and follows its own flag", func(t *testing.T) {
		t.Parallel()
		watch := New().watchGracefulContext(&ListenConfig{GracefulContext: t.Context()})
		run, ok := watch.(*gracefulRun)
		require.True(t, ok, "a context needs a run that watches it")
		inner := &stubListener{}

		wrapped := watch.wrap(inner)
		require.IsType(t, &gracefulListener{}, wrapped)

		run.beginDraining()
		_, err := wrapped.Accept()
		require.ErrorIs(t, err, net.ErrClosed)
		require.Zero(t, inner.accepts.Load(), "a draining run must not reach the wrapped Accept")
		watch.stopAndWait()
	})

	t.Run("runs on one app do not share their drain", func(t *testing.T) {
		t.Parallel()
		app := New()
		first := app.newGracefulRun(&ListenConfig{GracefulContext: t.Context()})
		second := app.newGracefulRun(&ListenConfig{GracefulContext: t.Context()})

		first.beginDraining()
		require.True(t, first.draining.Load())
		require.False(t, second.draining.Load())
		first.stopAndWait()
		second.stopAndWait()
	})
}

func (l benchListener) Accept() (net.Conn, error) { return l.conn, nil }
func (benchListener) Close() error                { return nil }
func (benchListener) Addr() net.Addr              { return nil }

// Benchmark_gracefulListener_Accept is what the wrapper adds per accepted connection when a
// GracefulContext is set: Serve calls Accept once per connection, and nothing else of the wrapper
// is on the hot path.
func Benchmark_gracefulListener_Accept(b *testing.B) {
	raw := net.Listener(benchListener{conn: benchConn{}})
	graceful := net.Listener(&gracefulListener{Listener: raw, draining: new(atomic.Bool)})

	for _, bm := range []struct {
		ln   net.Listener
		name string
	}{{raw, "raw"}, {graceful, "graceful"}} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				conn, err := bm.ln.Accept()
				if err != nil {
					b.Fatal(err)
				}
				benchSinkConn = conn
			}
		})
	}
}

func Test_Listen_TLS_PartialCertConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  ListenConfig
	}{
		{name: "CertFile only", cfg: ListenConfig{CertFile: "./.github/testdata/ssl.pem"}},
		{name: "CertKeyFile only", cfg: ListenConfig{CertKeyFile: "./.github/testdata/ssl.key"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			cfg := tc.cfg
			cfg.DisableStartupMessage = true
			// Fail instead of serving plaintext.
			cfg.BeforeServeFunc = func(_ *App) error { return errors.New("server would have served plaintext HTTP") }

			require.ErrorIs(t, app.Listen("127.0.0.1:0", cfg), ErrCertFileAndKeyRequired)
			require.Nil(t, app.tlsHandler)
		})
	}
}

// rootrestServeTLSHandler serves app on a TLS listener over base, whose certificate lookup records the
// ClientHelloInfo of each connection in handler. The app is shut down, which has to end cleanly, when the test ends.
func rootrestServeTLSHandler(t *testing.T, app *App, handler *TLSHandler, base net.Listener) {
	t.Helper()

	app.SetTLSHandler(handler)
	tlsLn := tls.NewListener(base, &tls.Config{
		MinVersion:     tls.VersionTLS12,
		Certificates:   []tls.Certificate{rootMiscBTestCert(t)},
		GetCertificate: handler.GetClientInfo,
	})

	errs := make(chan error, 1)
	go func() { errs <- app.Listener(tlsLn, ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-errs)
	})
}

func Test_Listener_TLS_ClientHelloInfo_PerConnection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		// maxConnsPerIP makes fasthttp wrap every accepted connection for its accounting, and the wrapper is
		// what the request side sees.
		maxConnsPerIP int
		inMemory      bool
	}{
		{name: "in-memory listener", inMemory: true},
		{name: "MaxConnsPerIP", maxConnsPerIP: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Server().MaxConnsPerIP = tc.maxConnsPerIP
			app.Get("/", func(c Ctx) error {
				if info := c.ClientHelloInfo(); info != nil {
					return c.SendString(info.ServerName)
				}
				return c.SendString("<nil>")
			})

			var dialRaw func() (net.Conn, error)
			if tc.inMemory {
				ln := fasthttputil.NewInmemoryListener()
				rootrestServeTLSHandler(t, app, &TLSHandler{}, ln)
				rootMiscBWaitDialable(t, ln)
				dialRaw = ln.Dial
			} else {
				ln := newLocalListener(t)
				rootrestServeTLSHandler(t, app, &TLSHandler{}, ln)
				dialRaw = func() (net.Conn, error) { return net.Dial("tcp", ln.Addr().String()) }
			}

			dial := func(serverName string) *tls.Conn {
				t.Helper()

				raw, dialErr := dialRaw()
				require.NoError(t, dialErr)
				conn := tls.Client(raw, rootMiscBTLSClientConfig(serverName))
				require.NoError(t, conn.Handshake())
				t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // not needed
				return conn
			}

			first := dial("first.example")
			second := dial("second.example")

			// Both handshakes complete before either request, so a shared value would report second.example twice.
			require.Equal(t, "first.example", rootMiscBTLSGet(t, first))
			require.Equal(t, "second.example", rootMiscBTLSGet(t, second))
		})
	}
}

func Test_Listener_ConnState_UserCallbackPreserved(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error { return c.SendString("ok") })

	closed := make(chan struct{}, 1)
	app.Server().ConnState = func(_ net.Conn, state fasthttp.ConnState) {
		if state == fasthttp.StateClosed {
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	}
	app.SetTLSHandler(&TLSHandler{})

	ln := fasthttputil.NewInmemoryListener()
	errs := make(chan error, 1)
	go func() { errs <- app.Listener(ln, ListenConfig{DisableStartupMessage: true}) }()

	conn, err := ln.Dial()
	require.NoError(t, err)
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"))
	require.NoError(t, err)
	_, err = io.ReadAll(conn)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	apptest.Recv(t, closed, 5*time.Second, "the user ConnState callback was not called")

	require.NoError(t, app.Shutdown())
	require.NoError(t, <-errs)
}

func Test_Listener_TLS_ClientHelloInfo_MaxConnsPerIP_NoLeak(t *testing.T) {
	t.Parallel()
	tlsHandler := &TLSHandler{}
	app := New()
	app.Server().MaxConnsPerIP = 8
	app.Get("/", func(c Ctx) error { return c.SendString("ok") })

	raw := newLocalListener(t)
	rootrestServeTLSHandler(t, app, tlsHandler, raw)

	// Sequential handshakes: the per-IP cap bounds concurrency, not the total.
	for range 12 {
		conn, dialErr := tls.Dial("tcp", raw.Addr().String(), rootMiscBTLSClientConfig("example.com"))
		require.NoError(t, dialErr)
		rootMiscBTLSGet(t, conn)
		require.NoError(t, conn.Close())
	}

	count := func(m *sync.Map) int {
		n := 0
		m.Range(func(_, _ any) bool { n++; return true })
		return n
	}
	require.Eventually(t, func() bool {
		return count(&tlsHandler.clientHelloInfos) == 0 && count(&tlsHandler.serverConns) == 0
	}, 5*time.Second, 20*time.Millisecond, "closed connections left %d ClientHelloInfo and %d wrapper entries behind",
		count(&tlsHandler.clientHelloInfos), count(&tlsHandler.serverConns))
}
