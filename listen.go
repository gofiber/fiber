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
	"text/tabwriter"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/acme/autocert"
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
	if cfg.CertFile != "" && cfg.CertKeyFile != "" {
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
	} else if cfg.AutoCertManager != nil {
		tlsConfig = &tls.Config{ //nolint:gosec // This is a user input
			MinVersion:     cfg.TLSMinVersion,
			GetCertificate: cfg.AutoCertManager.GetCertificate,
			NextProtos:     []string{"http/1.1", "acme-tls/1"},
		}
	}

	if tlsConfig != nil && cfg.TLSConfigFunc != nil {
		cfg.TLSConfigFunc(tlsConfig)
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

	return app.server.Serve(ln)
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

	return app.server.Serve(ln)
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

	var err error

	if cfg != nil && cfg.ShutdownTimeout != 0 {
		err = app.ShutdownWithTimeout(cfg.ShutdownTimeout) //nolint:contextcheck // TODO: Implement it
	} else {
		err = app.Shutdown() //nolint:contextcheck // TODO: Implement it
	}

	if err != nil {
		app.hooks.executeOnPostShutdownHooks(err)
		return
	}

	app.hooks.executeOnPostShutdownHooks(nil)
}
