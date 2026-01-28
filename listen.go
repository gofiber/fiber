// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/acme/autocert"

	"github.com/gofiber/fiber/v3/log"
)

// Figlet text to show Fiber ASCII art on startup message
var figletFiberText = `
    _______ __
   / ____(_) /_  ___  _____
  / /_  / / __ \/ _ \/ ___/
 / __/ / / /_/ /  __/ /
/_/   /_/_.___/\___/_/          %s`

const (
	globalIpv4Addr = "0.0.0.0"
)

// ListenConfig is a struct to customize startup of Fiber.
type ListenConfig struct {
	// GracefulContext is a field to shutdown Fiber by given context gracefully.
	//
	// Default: nil
	GracefulContext context.Context `json:"graceful_context"` //nolint:containedctx // It's needed to set context inside Listen.

	// TLSConfigFunc allows customizing tls.Config as you want.
	//
	// Default: nil
	TLSConfigFunc func(tlsConfig *tls.Config) `json:"tls_config_func"`

	// TLSConfig allows providing a tls.Config used as the base for TLS settings.
	// This enables external certificate providers via GetCertificate.
	//
	// Default: nil
	TLSConfig *tls.Config `json:"tls_config"`

	// ListenerFunc allows accessing and customizing net.Listener.
	//
	// Default: nil
	ListenerAddrFunc func(addr net.Addr) `json:"listener_addr_func"`

	// BeforeServeFunc allows customizing and accessing fiber app before serving the app.
	//
	// Default: nil
	BeforeServeFunc func(app *App) error `json:"before_serve_func"`

	// AutoCertManager manages TLS certificates automatically using the ACME protocol,
	// Enables integration with Let's Encrypt or other ACME-compatible providers.
	//
	// Default: nil
	AutoCertManager *autocert.Manager `json:"auto_cert_manager"`

	// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only), "unix" (Unix Domain Sockets)
	// WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chosen.
	//
	// Default: NetworkTCP4
	ListenerNetwork string `json:"listener_network"`

	// CertFile is a path of certificate file.
	// If you want to use TLS, you have to enter this field.
	//
	// Default : ""
	CertFile string `json:"cert_file"`

	// KeyFile is a path of certificate's private key.
	// If you want to use TLS, you have to enter this field.
	//
	// Default : ""
	CertKeyFile string `json:"cert_key_file"`

	// CertClientFile is a path of client certificate.
	// If you want to use mTLS, you have to enter this field.
	//
	// Default : ""
	CertClientFile string `json:"cert_client_file"`

	// When the graceful shutdown begins, use this field to set the timeout
	// duration. If the timeout is reached, OnPostShutdown will be called with the error.
	// Set to 0 to disable the timeout and wait indefinitely.
	//
	// Default: 10 * time.Second
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// FileMode to set for Unix Domain Socket (ListenerNetwork must be "unix")
	//
	// Default: 0770
	UnixSocketFileMode os.FileMode `json:"unix_socket_file_mode"`

	// TLSMinVersion allows to set TLS minimum version.
	//
	// Default: tls.VersionTLS12
	// WARNING: TLS1.0 and TLS1.1 versions are not supported.
	TLSMinVersion uint16 `json:"tls_min_version"`

	// When set to true, it will not print out the «Fiber» ASCII art and listening address.
	//
	// Default: false
	DisableStartupMessage bool `json:"disable_startup_message"`

	// When set to true, this will spawn multiple Go processes listening on the same port.
	//
	// Default: false
	EnablePrefork bool `json:"enable_prefork"`

	// If set to true, will print all routes with their method, path and handler.
	//
	// Default: false
	EnablePrintRoutes bool `json:"enable_print_routes"`
}

// ConnType identifies the protocol class of a tracked connection.
// The zero value (ConnTypeHTTP) means no special shutdown handling.
type ConnType int32

const (
	ConnTypeHTTP      ConnType = iota // default; force-close only
	ConnTypeWebSocket                 // send WS close frame, wait for ack
	ConnTypeSSE                       // send final SSE event, no ack
)

// TrackedConn exposes metadata and lifecycle controls for a connection
// tracked by the shutdown system.  Obtain via Ctx.TrackedConn().
type TrackedConn interface {
	// SetConnType marks this connection for protocol-aware graceful close.
	SetConnType(ConnType)
	// ConnType returns the current protocol type tag.
	ConnType() ConnType
	// SetCleanupHook registers a one-shot callback invoked before the
	// connection is closed during shutdown.  If set, this hook is responsible
	// for the full close handshake (send close frame / final event and wait
	// for client acknowledgement).  The framework's default close-frame
	// writer is skipped when a hook is present.
	// Only the first call takes effect; subsequent calls are ignored.
	SetCleanupHook(func() error)
	// ID returns the internal tracking identifier for this connection.
	ID() int64
}

// ShutdownConfig holds the configuration for a graceful shutdown initiated via ShutdownWithConfig.
type ShutdownConfig struct {
	// OnShutdownStart is called once at the beginning of shutdown with the current
	// number of active connections.
	//
	// Default: nil
	OnShutdownStart func(activeConns int)

	// OnDrainProgress is called periodically while active connections are still draining.
	// remaining is the number of connections still open; elapsed is the time since shutdown began.
	//
	// Default: nil
	OnDrainProgress func(remaining int, elapsed time.Duration)

	// OnForceClose is called after the context deadline is reached and connections
	// are force-closed. forceClosed is the number of connections that were still active.
	//
	// Default: nil
	OnForceClose func(forceClosed int)

	// RequestDeadline is the maximum duration a single in-flight request is allowed
	// to complete after shutdown begins. Connections that exceed this are force-closed.
	// A zero value means no per-request deadline is enforced beyond the context deadline.
	//
	// Default: 0 (no additional per-request deadline)
	RequestDeadline time.Duration

	// DrainInterval is how often OnDrainProgress is invoked while waiting for
	// connections to drain.
	//
	// Default: 500ms
	DrainInterval time.Duration

	// RequestContext, when non-nil, replaces the application's default shutdown
	// context as the parent for every in-flight request's context.  Use this to
	// layer an additional deadline on top of the shutdown signal.  For example,
	// passing context.WithTimeout(ctx, 5*time.Second) gives every active request
	// exactly 5 s to finish before the context expires.
	//
	// When nil the application's internal shutdown context is used (cancelled
	// the moment shutdown begins, with no additional deadline).
	//
	// Default: nil
	RequestContext context.Context //nolint:containedctx // Intentional: user supplies a deadline-bearing context.

	// WebSocketCloseTimeout is how long to wait for a WebSocket client's
	// close-frame acknowledgement after the server sends its close frame.
	// Only applies when no cleanup hook is registered on the connection.
	//
	// Default: 5s
	WebSocketCloseTimeout time.Duration

	// SSECloseTimeout is how long to wait after writing the final SSE
	// shutdown event for the client to disconnect.
	//
	// Default: 2s
	SSECloseTimeout time.Duration

	// SSECloseEvent is the raw SSE payload written to each tracked SSE
	// connection.  Must follow SSE wire format (field: value\n, blank line
	// terminator).  Only used when no cleanup hook is registered.
	//
	// Default: "event: shutdown\ndata: server shutting down\n\n"
	SSECloseEvent string

	// OnWebSocketClose is called after the server completes (or times out)
	// the close handshake for each WebSocket connection.
	// connID is the internal tracking ID.
	//
	// Default: nil
	OnWebSocketClose func(connID int64, err error)

	// OnSSEClose is called after the final SSE event has been sent
	// (or the write failed) for each SSE connection.
	//
	// Default: nil
	OnSSEClose func(connID int64, err error)
}

// ShutdownTelemetry captures timing and connection-disposition metrics for the
// most recently completed shutdown.  Obtain via App.LastShutdownTelemetry() or
// through the JSON debug endpoint registered with App.ShutdownDebugHandler().
type ShutdownTelemetry struct {
	// StartedAt is the wall-clock moment phase 1 began.
	StartedAt time.Time
	// CompletedAt is the wall-clock moment after phase 9 finished.
	CompletedAt time.Time
	// TotalDuration is CompletedAt − StartedAt.
	TotalDuration time.Duration

	// DrainDuration is the time spent in the drain-poll loop (phase 7).
	DrainDuration time.Duration
	// PreHooksDuration is the time spent executing pre-shutdown hooks (phase 5).
	PreHooksDuration time.Duration
	// GracefulCloseDuration is the time spent in GracefulCloseTyped (phase 5b).
	GracefulCloseDuration time.Duration
	// PostHooksDuration is the time spent executing post-shutdown hooks (phase 9).
	PostHooksDuration time.Duration

	// InitialConns is the number of active connections at the start of shutdown.
	InitialConns int
	// DrainedConns is the number of connections that closed naturally before the
	// context deadline (InitialConns − ForcedConns).
	DrainedConns int
	// ForcedConns is the number of connections force-closed after the deadline.
	ForcedConns int

	// WebSocketsClosed is the number of WebSocket connections that received a
	// close frame (or had a cleanup hook executed) during phase 5b.
	WebSocketsClosed int
	// SSEsClosed is the number of SSE connections that received the shutdown
	// event (or had a cleanup hook executed) during phase 5b.
	SSEsClosed int

	// TimedOut is true when the shutdown context deadline was exceeded.
	TimedOut bool
}

// connTrackingListener wraps a net.Listener and maintains both an atomic
// active-connection counter and a registry (sync.Map) of every live
// connection.  The registry enables CloseAll to force-close every tracked
// connection at shutdown when the drain deadline is exceeded.
type connTrackingListener struct {
	net.Listener

	activeConns *int64   // shared atomic counter (lives on App)
	conns       sync.Map // map[int64]*connTrackingConn — live connections
	nextID      int64    // monotonic ID generator for registry keys
}

// Accept waits for the next connection, increments the active counter, and
// registers the connection in the internal map so it can be force-closed later.
func (l *connTrackingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	atomic.AddInt64(l.activeConns, 1)
	id := atomic.AddInt64(&l.nextID, 1)
	tc := &connTrackingConn{
		Conn:        conn,
		activeConns: l.activeConns,
		registry:    &l.conns,
		id:          id,
	}
	l.conns.Store(id, tc)
	return tc, nil
}

// CloseAll force-closes every connection currently in the registry and returns
// the number of connections that were still open (i.e. the ones actually closed
// by this call).  It is safe to call concurrently; connections that have already
// been closed by their own handler are skipped via the closed guard.
func (l *connTrackingListener) CloseAll() int {
	closed := 0
	l.conns.Range(func(_, value any) bool {
		if tc, ok := value.(*connTrackingConn); ok {
			if atomic.CompareAndSwapInt32(&tc.closed, 0, 1) {
				// We won the CAS — this connection is still alive.
				atomic.AddInt64(l.activeConns, -1)
				_ = tc.Conn.Close()
				closed++
			}
			// Remove from the registry regardless of who closed it.
			l.conns.Delete(tc.id)
		}
		return true
	})
	return closed
}

// connTrackingConn wraps a net.Conn, decrements the active counter on Close,
// and removes itself from the listener's connection registry.
type connTrackingConn struct {
	net.Conn
	activeConns *int64
	registry    *sync.Map
	id          int64
	closed      int32 // CAS guard: ensures decrement and registry removal happen once

	// Protocol-aware shutdown fields:
	connType    int32                        // atomic; ConnType enum value
	cleanupOnce sync.Once                    // one-shot guard for hook
	cleanupHook atomic.Pointer[func() error] // per-connection pre-close callback
}

func (c *connTrackingConn) SetConnType(ct ConnType) {
	atomic.StoreInt32(&c.connType, int32(ct))
}

func (c *connTrackingConn) ConnType() ConnType {
	return ConnType(atomic.LoadInt32(&c.connType))
}

func (c *connTrackingConn) SetCleanupHook(fn func() error) {
	if fn != nil {
		c.cleanupHook.Store(&fn)
	}
}

func (c *connTrackingConn) ID() int64 {
	return c.id
}

// runCleanup executes the hook exactly once; returns its error.
func (c *connTrackingConn) runCleanup() error {
	var err error
	c.cleanupOnce.Do(func() {
		if ptr := c.cleanupHook.Load(); ptr != nil {
			err = (*ptr)()
		}
	})
	return err
}

// Close closes the underlying connection and decrements the active counter
// exactly once.  It also removes this connection from the tracking registry so
// that CloseAll does not attempt to close it again.
func (c *connTrackingConn) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		atomic.AddInt64(c.activeConns, -1)
		c.registry.Delete(c.id)
	}
	return c.Conn.Close()
}

// GracefulCloseTyped iterates all tracked connections and performs
// protocol-aware graceful close for WebSocket and SSE connections.
// Plain HTTP connections are skipped entirely.
// The provided context bounds the total time spent in this phase.
// tel is guaranteed non-nil from the caller; WS/SSE counters are incremented
// sequentially inside the Range so no additional synchronization is needed.
func (l *connTrackingListener) GracefulCloseTyped(ctx context.Context, cfg *ShutdownConfig, tel *ShutdownTelemetry) {
	l.conns.Range(func(_, value any) bool {
		tc, ok := value.(*connTrackingConn)
		if !ok {
			return true
		}

		switch tc.ConnType() {
		case ConnTypeWebSocket:
			l.closeWebSocket(ctx, tc, cfg)
			tel.WebSocketsClosed++
		case ConnTypeSSE:
			l.closeSSE(ctx, tc, cfg)
			tel.SSEsClosed++
		default:
			// Plain HTTP — run cleanup hook if registered, but no
			// protocol-specific close logic.
			_ = tc.runCleanup()
		}
		return true
	})
}

func (l *connTrackingListener) closeWebSocket(ctx context.Context, tc *connTrackingConn, cfg *ShutdownConfig) {
	// If handler registered a cleanup hook, it owns the full handshake.
	if ptr := tc.cleanupHook.Load(); ptr != nil {
		err := tc.runCleanup()
		if cfg != nil && cfg.OnWebSocketClose != nil {
			cfg.OnWebSocketClose(tc.id, err)
		}
		return
	}

	// No hook — framework writes a minimal RFC 6455 close frame:
	// opcode 0x88 (close), payload length 2, status 1001 (Going Away).
	closeFrame := [4]byte{0x88, 0x02, 0x03, 0xe9}

	_ = tc.Conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	if _, err := tc.Conn.Write(closeFrame[:]); err != nil {
		if cfg != nil && cfg.OnWebSocketClose != nil {
			cfg.OnWebSocketClose(tc.id, err)
		}
		return
	}

	// Wait for client's close-frame reply.
	timeout := 5 * time.Second
	if cfg != nil && cfg.WebSocketCloseTimeout > 0 {
		timeout = cfg.WebSocketCloseTimeout
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = tc.Conn.SetReadDeadline(deadline)

	var buf [2]byte
	_, readErr := tc.Conn.Read(buf[:])

	// Reset deadlines so normal Close path isn't affected.
	_ = tc.Conn.SetReadDeadline(time.Time{})
	_ = tc.Conn.SetWriteDeadline(time.Time{})

	var reportErr error
	if readErr != nil {
		reportErr = ErrWebSocketCloseTimeout
	}
	if cfg != nil && cfg.OnWebSocketClose != nil {
		cfg.OnWebSocketClose(tc.id, reportErr)
	}
}

func (l *connTrackingListener) closeSSE(ctx context.Context, tc *connTrackingConn, cfg *ShutdownConfig) {
	// If handler registered a cleanup hook, it owns the event write.
	if ptr := tc.cleanupHook.Load(); ptr != nil {
		err := tc.runCleanup()
		if cfg != nil && cfg.OnSSEClose != nil {
			cfg.OnSSEClose(tc.id, err)
		}
		return
	}

	// No hook — framework writes the default (or configured) SSE event.
	payload := "event: shutdown\ndata: server shutting down\n\n"
	if cfg != nil && cfg.SSECloseEvent != "" {
		payload = cfg.SSECloseEvent
	}

	_ = tc.Conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	_, writeErr := tc.Conn.Write([]byte(payload))

	// Reset write deadline.
	_ = tc.Conn.SetWriteDeadline(time.Time{})

	// Wait briefly for client to acknowledge (disconnect).
	timeout := 2 * time.Second
	if cfg != nil && cfg.SSECloseTimeout > 0 {
		timeout = cfg.SSECloseTimeout
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = tc.Conn.SetReadDeadline(deadline)
	var buf [1]byte
	_, _ = tc.Conn.Read(buf[:]) // blocks until disconnect or timeout
	_ = tc.Conn.SetReadDeadline(time.Time{})

	var reportErr error
	if writeErr != nil {
		reportErr = ErrSSECloseWriteFailed
	}
	if cfg != nil && cfg.OnSSEClose != nil {
		cfg.OnSSEClose(tc.id, reportErr)
	}
}

// listenConfigDefault is a function to set default values of ListenConfig.
func listenConfigDefault(config ...ListenConfig) ListenConfig {
	if len(config) < 1 {
		return ListenConfig{
			TLSMinVersion:      tls.VersionTLS12,
			ListenerNetwork:    NetworkTCP4,
			UnixSocketFileMode: 0o770,
			ShutdownTimeout:    10 * time.Second,
		}
	}

	cfg := config[0]
	if cfg.ListenerNetwork == "" {
		cfg.ListenerNetwork = NetworkTCP4
	}

	if cfg.UnixSocketFileMode == 0 {
		cfg.UnixSocketFileMode = 0o770
	}

	if cfg.TLSMinVersion == 0 {
		cfg.TLSMinVersion = tls.VersionTLS12
	}

	if cfg.TLSMinVersion != tls.VersionTLS12 && cfg.TLSMinVersion != tls.VersionTLS13 {
		panic("unsupported TLS version, please use tls.VersionTLS12 or tls.VersionTLS13")
	}

	return cfg
}

// Listen serves HTTP requests from the given addr.
// You should enter custom ListenConfig to customize startup. (TLS, mTLS, prefork...)
//
//	app.Listen(":8080")
//	app.Listen("127.0.0.1:8080")
//	app.Listen(":8080", ListenConfig{EnablePrefork: true})
func (app *App) Listen(addr string, config ...ListenConfig) error {
	cfg := listenConfigDefault(config...)

	// Configure TLS
	var tlsConfig *tls.Config
	if cfg.TLSConfig != nil {
		tlsConfig = cfg.TLSConfig.Clone()
	} else {
		switch {
		case cfg.AutoCertManager != nil && (cfg.CertFile != "" || cfg.CertKeyFile != ""):
			return ErrAutoCertWithCertFile
		case cfg.CertFile != "" && cfg.CertKeyFile != "":
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.CertKeyFile)
			if err != nil {
				return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %w", cfg.CertFile, cfg.CertKeyFile, err)
			}

			tlsHandler := &TLSHandler{}
			tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
				MinVersion: cfg.TLSMinVersion,
				Certificates: []tls.Certificate{
					cert,
				},
				GetCertificate: tlsHandler.GetClientInfo,
			}

			if cfg.CertClientFile != "" {
				clientCACert, err := os.ReadFile(filepath.Clean(cfg.CertClientFile))
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}

				clientCertPool := x509.NewCertPool()
				clientCertPool.AppendCertsFromPEM(clientCACert)

				tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
				tlsConfig.ClientCAs = clientCertPool
			}

			// Attach the tlsHandler to the config
			app.SetTLSHandler(tlsHandler)
		case cfg.AutoCertManager != nil:
			tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
				MinVersion:     cfg.TLSMinVersion,
				GetCertificate: cfg.AutoCertManager.GetCertificate,
				NextProtos:     []string{"http/1.1", "acme-tls/1"},
			}
		default:
		}

		if tlsConfig != nil && cfg.TLSConfigFunc != nil {
			cfg.TLSConfigFunc(tlsConfig)
		}
	}

	// Graceful shutdown
	if cfg.GracefulContext != nil {
		ctx, cancel := context.WithCancel(cfg.GracefulContext)
		defer cancel()

		go app.gracefulShutdown(ctx, &cfg)
	}

	// Start prefork
	if cfg.EnablePrefork {
		return app.prefork(addr, tlsConfig, &cfg)
	}

	// Configure Listener
	ln, err := app.createListener(addr, tlsConfig, &cfg)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// prepare the server for the start
	app.startupProcess()

	listenData := app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil, &cfg, nil)

	// run hooks
	app.runOnListenHooks(listenData)

	// Print startup message & routes
	app.printMessages(&cfg, listenData)

	// Serve
	if cfg.BeforeServeFunc != nil {
		if err := cfg.BeforeServeFunc(app); err != nil {
			return err
		}
	}

	// Wrap listener with connection tracking and store the reference so
	// ShutdownWithConfig can close it and force-close tracked connections.
	app.trackingListener = &connTrackingListener{Listener: ln, activeConns: &app.activeConns}

	return app.server.Serve(app.trackingListener)
}

// Listener serves HTTP requests from the given listener.
// You should enter custom ListenConfig to customize startup. (prefork, startup message, graceful shutdown...)
func (app *App) Listener(ln net.Listener, config ...ListenConfig) error {
	cfg := listenConfigDefault(config...)

	// Graceful shutdown
	if cfg.GracefulContext != nil {
		ctx, cancel := context.WithCancel(cfg.GracefulContext)
		defer cancel()

		go app.gracefulShutdown(ctx, &cfg)
	}

	// prepare the server for the start
	app.startupProcess()

	listenData := app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil, &cfg, nil)

	// run hooks
	app.runOnListenHooks(listenData)

	// Print startup message & routes
	app.printMessages(&cfg, listenData)

	// Serve
	if cfg.BeforeServeFunc != nil {
		if err := cfg.BeforeServeFunc(app); err != nil {
			return err
		}
	}

	// Prefork is not supported for custom listeners
	if cfg.EnablePrefork {
		log.Warn("Prefork isn't supported for custom listeners.")
	}

	// Wrap listener with connection tracking and store the reference so
	// ShutdownWithConfig can close it and force-close tracked connections.
	app.trackingListener = &connTrackingListener{Listener: ln, activeConns: &app.activeConns}

	return app.server.Serve(app.trackingListener)
}

// Create listener function.
func (*App) createListener(addr string, tlsConfig *tls.Config, cfg *ListenConfig) (net.Listener, error) {
	if cfg == nil {
		cfg = &ListenConfig{}
	}
	var listener net.Listener
	var err error

	// Remove previously created socket, to make sure it's possible to listen
	if cfg.ListenerNetwork == NetworkUnix {
		if err = os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("unexpected error when trying to remove unix socket file %q: %w", addr, err)
		}
	}

	if tlsConfig != nil {
		listener, err = tls.Listen(cfg.ListenerNetwork, addr, tlsConfig)
	} else {
		listener, err = net.Listen(cfg.ListenerNetwork, addr)
	}

	// Check for error before using the listener
	if err != nil {
		// Wrap the error from tls.Listen/net.Listen
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	if cfg.ListenerNetwork == NetworkUnix {
		if err = os.Chmod(addr, cfg.UnixSocketFileMode); err != nil {
			return nil, fmt.Errorf("cannot chmod %#o for %q: %w", cfg.UnixSocketFileMode, addr, err)
		}
	}

	if cfg.ListenerAddrFunc != nil {
		cfg.ListenerAddrFunc(listener.Addr())
	}

	return listener, nil
}

func (app *App) printMessages(cfg *ListenConfig, listenData *ListenData) {
	app.startupMessage(listenData, cfg)

	if cfg.EnablePrintRoutes {
		app.printRoutesMessage()
	}
}

// prepareListenData creates a ListenData instance populated with the application metadata.
func (app *App) prepareListenData(addr string, isTLS bool, cfg *ListenConfig, childPIDs []int) *ListenData { //revive:disable-line:flag-parameter // Accepting a bool param named isTLS is fine here
	host, port := parseAddr(addr)
	if host == "" {
		if cfg.ListenerNetwork == NetworkTCP6 {
			host = "[::1]"
		} else {
			host = globalIpv4Addr
		}
	}

	processCount := 1
	if cfg.EnablePrefork {
		processCount = runtime.GOMAXPROCS(0)
	}

	var clonedPIDs []int
	if len(childPIDs) > 0 {
		clonedPIDs = slices.Clone(childPIDs)
	}

	return &ListenData{
		Host:         host,
		Port:         port,
		Version:      Version,
		AppName:      app.config.AppName,
		ColorScheme:  app.config.ColorScheme,
		ChildPIDs:    clonedPIDs,
		HandlerCount: int(app.handlersCount),
		ProcessCount: processCount,
		PID:          os.Getpid(),
		TLS:          isTLS,
		Prefork:      cfg.EnablePrefork,
	}
}

// startupMessage renders the startup banner using the provided listener metadata and configuration.
func (app *App) startupMessage(listenData *ListenData, cfg *ListenConfig) {
	preData := newPreStartupMessageData(listenData)
	colors := listenData.ColorScheme

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	// Add default entries
	scheme := schemeHTTP
	if listenData.TLS {
		scheme = schemeHTTPS
	}

	if listenData.Host == globalIpv4Addr {
		preData.AddInfo("server_address", "Server started on", fmt.Sprintf("%s%s://127.0.0.1:%s%s (bound on host 0.0.0.0 and port %s)",
			colors.Blue, scheme, listenData.Port, colors.Reset, listenData.Port), 10)
	} else {
		preData.AddInfo("server_address", "Server started on", fmt.Sprintf("%s%s://%s:%s%s",
			colors.Blue, scheme, listenData.Host, listenData.Port, colors.Reset), 10)
	}

	if listenData.AppName != "" {
		preData.AddInfo("app_name", "Application name", fmt.Sprintf("\t%s%s%s", colors.Blue, listenData.AppName, colors.Reset), 9)
	}

	preData.AddInfo("total_handlers", "Total handlers", fmt.Sprintf("\t%s%d%s", colors.Blue, listenData.HandlerCount, colors.Reset), 8)

	if listenData.Prefork {
		preData.AddInfo("prefork", "Prefork", fmt.Sprintf("\t\t%sEnabled%s", colors.Blue, colors.Reset), 7)
	} else {
		preData.AddInfo("prefork", "Prefork", fmt.Sprintf("\t\t%sDisabled%s", colors.Red, colors.Reset), 6)
	}

	preData.AddInfo("pid", "PID", fmt.Sprintf("\t\t%s%d%s", colors.Blue, listenData.PID, colors.Reset), 5)

	preData.AddInfo("process_count", "Total process count", fmt.Sprintf("%s%d%s", colors.Blue, listenData.ProcessCount, colors.Reset), 4)

	if err := app.hooks.executeOnPreStartupMessageHooks(preData); err != nil {
		log.Errorf("failed to call pre startup message hook: %v", err)
	}

	disabled := cfg.DisableStartupMessage
	isChild := IsChild()
	prevented := preData != nil && preData.PreventDefault

	defer func() {
		postData := newPostStartupMessageData(listenData, disabled, isChild, prevented)
		if err := app.hooks.executeOnPostStartupMessageHooks(postData); err != nil {
			log.Errorf("failed to call post startup message hook: %v", err)
		}
	}()

	if preData == nil || disabled || isChild || prevented {
		return
	}

	if preData.BannerHeader != "" {
		header := preData.BannerHeader
		fmt.Fprint(out, header)
		if !strings.HasSuffix(header, "\n") {
			fmt.Fprintln(out)
		}
	} else {
		fmt.Fprintf(out, "%s\n", fmt.Sprintf(figletFiberText, colors.Red+"v"+listenData.Version+colors.Reset))
		fmt.Fprintln(out, strings.Repeat("-", 50))
	}

	printStartupEntries(out, &colors, preData.entries)

	app.logServices(app.servicesStartupCtx(), out, &colors)

	if listenData.Prefork && len(listenData.ChildPIDs) > 0 {
		fmt.Fprintf(out, "%sINFO%s Child PIDs: \t\t%s", colors.Green, colors.Reset, colors.Blue)

		totalPIDs := len(listenData.ChildPIDs)
		rowTotalPidCount := 10

		for i := 0; i < totalPIDs; i += rowTotalPidCount {
			start := i
			end := min(i+rowTotalPidCount, totalPIDs)

			for idx, pid := range listenData.ChildPIDs[start:end] {
				fmt.Fprintf(out, "%d", pid)
				if idx+1 != len(listenData.ChildPIDs[start:end]) {
					fmt.Fprint(out, ", ")
				}
			}
			fmt.Fprintf(out, "\n%s", colors.Reset)
		}
	}

	fmt.Fprintf(out, "\n%s", colors.Reset)
}

func printStartupEntries(out io.Writer, colors *Colors, entries []startupMessageEntry) {
	// Sort entries by priority (higher priority first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].priority > entries[j].priority
	})

	for _, entry := range entries {
		var label string
		var color string
		switch entry.level {
		case StartupMessageLevelWarning:
			label, color = "WARN", colors.Yellow
		case StartupMessageLevelError:
			label, color = errString, colors.Red
		default:
			label, color = "INFO", colors.Green
		}

		fmt.Fprintf(out, "%s%s%s %s: \t%s%s%s\n", color, label, colors.Reset, entry.title, colors.Blue, entry.value, colors.Reset)
	}
}

// printRoutesMessage print all routes with method, path, name and handlers
// in a format of table, like this:
// method | path | name      | handlers
// GET    | /    | routeName | github.com/gofiber/fiber/v3.emptyHandler
// HEAD   | /    |           | github.com/gofiber/fiber/v3.emptyHandler
func (app *App) printRoutesMessage() {
	// ignore child processes
	if IsChild() {
		return
	}

	// Alias colors
	colors := app.config.ColorScheme

	var routes []RouteMessage
	for _, routeStack := range app.stack {
		for _, route := range routeStack {
			var newRoute RouteMessage
			newRoute.name = route.Name
			newRoute.method = route.Method
			newRoute.path = route.Path
			for _, handler := range route.Handlers {
				newRoute.handlers += runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name() + " "
			}
			routes = append(routes, newRoute)
		}
	}

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	w := tabwriter.NewWriter(out, 1, 1, 1, ' ', 0)
	// Sort routes by path
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].path < routes[j].path
	})

	fmt.Fprintf(w, "%smethod\t%s| %spath\t%s| %sname\t%s| %shandlers\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset)
	fmt.Fprintf(w, "%s------\t%s| %s----\t%s| %s----\t%s| %s--------\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset)

	for _, route := range routes {
		fmt.Fprintf(w, "%s%s\t%s| %s%s\t%s| %s%s\t%s| %s%s%s\n", colors.Blue, route.method, colors.White, colors.Green, route.path, colors.White, colors.Cyan, route.name, colors.White, colors.Yellow, route.handlers, colors.Reset)
	}

	_ = w.Flush() //nolint:errcheck // It is fine to ignore the error here
}

// shutdown goroutine
func (app *App) gracefulShutdown(ctx context.Context, cfg *ListenConfig) {
	<-ctx.Done()

	// Derive the shutdown context from the parent context so that any deadline
	// already set on ctx is respected. If ShutdownTimeout is also configured,
	// apply it as an additional bound — the effective deadline is the earlier of
	// the two.
	shutdownCtx := ctx
	if cfg != nil && cfg.ShutdownTimeout != 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(ctx, cfg.ShutdownTimeout)
		defer cancel()
	}

	// ShutdownWithContext already executes pre- and post-shutdown hooks via defer,
	// so we must not call executeOnPostShutdownHooks again here.
	_ = app.ShutdownWithContext(shutdownCtx)
}
