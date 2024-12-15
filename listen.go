// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
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

	// OnShutdownError allows to customize error behavior when to graceful shutdown server by given signal.
	//
	// Print error with log.Fatalf() by default.
	// Default: nil
	OnShutdownError func(err error)

	// OnShutdownSuccess allows to customize success behavior when to graceful shutdown server by given signal.
	//
	// Default: nil
	OnShutdownSuccess func()

	// AutoCertManager manages TLS certificates automatically using the ACME protocol,
	// Enables integration with Let's Encrypt or other ACME-compatible providers.
	//
	// Default: nil
	AutoCertManager *autocert.Manager `json:"auto_cert_manager"`

	// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only)
	// WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chosen.
	//
	// Default: NetworkTCP4
	ListenerNetwork string `json:"listener_network"`

	// CertFile is a path of certficate file.
	// If you want to use TLS, you have to enter this field.
	//
	// Default : ""
	CertFile string `json:"cert_file"`

	// KeyFile is a path of certficate's private key.
	// If you want to use TLS, you have to enter this field.
	//
	// Default : ""
	CertKeyFile string `json:"cert_key_file"`

	// CertClientFile is a path of client certficate.
	// If you want to use mTLS, you have to enter this field.
	//
	// Default : ""
	CertClientFile string `json:"cert_client_file"`

	// When the graceful shutdown begins, use this field to set the timeout
	// duration. If the timeout is reached, OnShutdownError will be called.
	// Set to 0 to disable the timeout and wait indefinitely.
	//
	// Default: 10 * time.Second
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

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
			TLSMinVersion:   tls.VersionTLS12,
			ListenerNetwork: NetworkTCP4,
			OnShutdownError: func(err error) {
				log.Fatalf("shutdown: %v", err) //nolint:revive // It's an option
			},
			ShutdownTimeout: 10 * time.Second,
		}
	}

	cfg := config[0]
	if cfg.ListenerNetwork == "" {
		cfg.ListenerNetwork = NetworkTCP4
	}

	if cfg.OnShutdownError == nil {
		cfg.OnShutdownError = func(err error) {
			log.Fatalf("shutdown: %v", err) //nolint:revive // It's an option
		}
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

		go app.gracefulShutdown(ctx, cfg)
	}

	// Start prefork
	if cfg.EnablePrefork {
		return app.prefork(addr, tlsConfig, cfg)
	}

	// Configure Listener
	ln, err := app.createListener(addr, tlsConfig, cfg)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil, cfg))

	// Print startup message & routes
	app.printMessages(cfg, ln)

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

		go app.gracefulShutdown(ctx, cfg)
	}

	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil, cfg))

	// Print startup message & routes
	app.printMessages(cfg, ln)

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
func (*App) createListener(addr string, tlsConfig *tls.Config, cfg ListenConfig) (net.Listener, error) {
	var listener net.Listener
	var err error

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

	if cfg.ListenerAddrFunc != nil {
		cfg.ListenerAddrFunc(listener.Addr())
	}

	return listener, nil
}

func (app *App) printMessages(cfg ListenConfig, ln net.Listener) {
	// Print startup message
	if !cfg.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), getTLSConfig(ln) != nil, "", cfg)
	}

	// Print routes
	if cfg.EnablePrintRoutes {
		app.printRoutesMessage()
	}
}

// prepareListenData create an slice of ListenData
func (*App) prepareListenData(addr string, isTLS bool, cfg ListenConfig) ListenData { //revive:disable-line:flag-parameter // Accepting a bool param named isTLS if fine here
	host, port := parseAddr(addr)
	if host == "" {
		if cfg.ListenerNetwork == NetworkTCP6 {
			host = "[::1]"
		} else {
			host = globalIpv4Addr
		}
	}

	return ListenData{
		Host: host,
		Port: port,
		TLS:  isTLS,
	}
}

// startupMessage prepares the startup message with the handler number, port, address and other information
func (app *App) startupMessage(addr string, isTLS bool, pids string, cfg ListenConfig) { //nolint: revive // Accepting a bool param named isTLS if fine here
	// ignore child processes
	if IsChild() {
		return
	}

	// Alias colors
	colors := app.config.ColorScheme

	host, port := parseAddr(addr)
	if host == "" {
		if cfg.ListenerNetwork == NetworkTCP6 {
			host = "[::1]"
		} else {
			host = globalIpv4Addr
		}
	}

	scheme := schemeHTTP
	if isTLS {
		scheme = schemeHTTPS
	}

	isPrefork := "Disabled"
	if cfg.EnablePrefork {
		isPrefork = "Enabled"
	}

	procs := strconv.Itoa(runtime.GOMAXPROCS(0))
	if !cfg.EnablePrefork {
		procs = "1"
	}

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	fmt.Fprintf(out, "%s\n", fmt.Sprintf(figletFiberText, colors.Red+"v"+Version+colors.Reset)) //nolint:errcheck,revive // ignore error
	fmt.Fprintf(out, strings.Repeat("-", 50)+"\n")                                              //nolint:errcheck,revive,govet // ignore error

	if host == "0.0.0.0" {
		//nolint:errcheck,revive // ignore error
		fmt.Fprintf(out,
			"%sINFO%s Server started on: \t%s%s://127.0.0.1:%s%s (bound on host 0.0.0.0 and port %s)\n",
			colors.Green, colors.Reset, colors.Blue, scheme, port, colors.Reset, port)
	} else {
		//nolint:errcheck,revive // ignore error
		fmt.Fprintf(out,
			"%sINFO%s Server started on: \t%s%s%s\n",
			colors.Green, colors.Reset, colors.Blue, fmt.Sprintf("%s://%s:%s", scheme, host, port), colors.Reset)
	}

	if app.config.AppName != "" {
		fmt.Fprintf(out, "%sINFO%s Application name: \t\t%s%s%s\n", colors.Green, colors.Reset, colors.Blue, app.config.AppName, colors.Reset) //nolint:errcheck,revive // ignore error
	}

	//nolint:errcheck,revive // ignore error
	fmt.Fprintf(out,
		"%sINFO%s Total handlers count: \t%s%s%s\n",
		colors.Green, colors.Reset, colors.Blue, strconv.Itoa(int(app.handlersCount)), colors.Reset)

	if isPrefork == "Enabled" {
		fmt.Fprintf(out, "%sINFO%s Prefork: \t\t\t%s%s%s\n", colors.Green, colors.Reset, colors.Blue, isPrefork, colors.Reset) //nolint:errcheck,revive // ignore error
	} else {
		fmt.Fprintf(out, "%sINFO%s Prefork: \t\t\t%s%s%s\n", colors.Green, colors.Reset, colors.Red, isPrefork, colors.Reset) //nolint:errcheck,revive // ignore error
	}

	fmt.Fprintf(out, "%sINFO%s PID: \t\t\t%s%v%s\n", colors.Green, colors.Reset, colors.Blue, os.Getpid(), colors.Reset)       //nolint:errcheck,revive // ignore error
	fmt.Fprintf(out, "%sINFO%s Total process count: \t%s%s%s\n", colors.Green, colors.Reset, colors.Blue, procs, colors.Reset) //nolint:errcheck,revive // ignore error

	if cfg.EnablePrefork {
		// Turn the `pids` variable (in the form ",a,b,c,d,e,f,etc") into a slice of PIDs
		pidSlice := make([]string, 0)
		for _, v := range strings.Split(pids, ",") {
			if v != "" {
				pidSlice = append(pidSlice, v)
			}
		}

		fmt.Fprintf(out, "%sINFO%s Child PIDs: \t\t%s", colors.Green, colors.Reset, colors.Blue) //nolint:errcheck,revive // ignore error
		totalPids := len(pidSlice)
		rowTotalPidCount := 10

		for i := 0; i < totalPids; i += rowTotalPidCount {
			start := i
			end := i + rowTotalPidCount

			if end > totalPids {
				end = totalPids
			}

			for n, pid := range pidSlice[start:end] {
				fmt.Fprintf(out, "%s", pid) //nolint:errcheck,revive // ignore error
				if n+1 != len(pidSlice[start:end]) {
					fmt.Fprintf(out, ", ") //nolint:errcheck,revive // ignore error
				}
			}
			fmt.Fprintf(out, "\n%s", colors.Reset) //nolint:errcheck,revive // ignore error
		}
	}

	// add new Line as spacer
	fmt.Fprintf(out, "\n%s", colors.Reset) //nolint:errcheck,revive // ignore error
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

	fmt.Fprintf(w, "%smethod\t%s| %spath\t%s| %sname\t%s| %shandlers\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset) //nolint:errcheck,revive // ignore error
	fmt.Fprintf(w, "%s------\t%s| %s----\t%s| %s----\t%s| %s--------\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset) //nolint:errcheck,revive // ignore error

	for _, route := range routes {
		//nolint:errcheck,revive // ignore error
		fmt.Fprintf(w, "%s%s\t%s| %s%s\t%s| %s%s\t%s| %s%s%s\n", colors.Blue, route.method, colors.White, colors.Green, route.path, colors.White, colors.Cyan, route.name, colors.White, colors.Yellow, route.handlers, colors.Reset)
	}

	_ = w.Flush() //nolint:errcheck // It is fine to ignore the error here
}

// shutdown goroutine
func (app *App) gracefulShutdown(ctx context.Context, cfg ListenConfig) {
	<-ctx.Done()

	var err error

	if cfg.ShutdownTimeout != 0 {
		err = app.ShutdownWithTimeout(cfg.ShutdownTimeout) //nolint:contextcheck // TODO: Implement it
	} else {
		err = app.Shutdown() //nolint:contextcheck // TODO: Implement it
	}

	if err != nil {
		cfg.OnShutdownError(err)
		return
	}

	if success := cfg.OnShutdownSuccess; success != nil {
		success()
	}
}
