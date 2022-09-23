// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

// Figlet text to show Fiber ASCII art on startup message
var figletFiberText = `
    _______ __             
   / ____(_) /_  ___  _____
  / /_  / / __ \/ _ \/ ___/
 / __/ / / /_/ /  __/ /    
/_/   /_/_.___/\___/_/     %s`

// ListenConfig is a struct to customize startup of Fiber.
//
// TODO: Add timeout for graceful shutdown.
type ListenConfig struct {
	// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only)
	// WARNING: When prefork is set to true, only "tcp4" and "tcp6" can be chose.
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

	// GracefulContext is a field to shutdown Fiber by given context gracefully.
	//
	// Default: nil
	GracefulContext context.Context `json:"graceful_context"`

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

	// OnShutdownError allows to customize error behavior when to graceful shutdown server by given signal.
	//
	// Default: Print error with log.Fatalf()
	OnShutdownError func(err error)

	// OnShutdownSuccess allows to customize success behavior when to graceful shutdown server by given signal.
	//
	// Default: nil
	OnShutdownSuccess func()
}

// listenConfigDefault is a function to set default values of ListenConfig.
func listenConfigDefault(config ...ListenConfig) ListenConfig {
	if len(config) < 1 {
		return ListenConfig{
			ListenerNetwork: NetworkTCP4,
			OnShutdownError: func(err error) {
				log.Fatalf("shutdown: %v", err)
			},
		}
	}

	cfg := config[0]
	if cfg.ListenerNetwork == "" {
		cfg.ListenerNetwork = NetworkTCP4
	}

	if cfg.OnShutdownError == nil {
		cfg.OnShutdownError = func(err error) {
			log.Fatalf("shutdown: %v", err)
		}
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
	var tlsConfig *tls.Config = nil
	if cfg.CertFile != "" && cfg.CertKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.CertKeyFile)
		if err != nil {
			return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %s", cfg.CertFile, cfg.CertKeyFile, err)
		}

		tlsHandler := &TLSHandler{}
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			Certificates: []tls.Certificate{
				cert,
			},
			GetCertificate: tlsHandler.GetClientInfo,
		}

		if cfg.CertClientFile != "" {
			clientCACert, err := os.ReadFile(filepath.Clean(cfg.CertClientFile))
			if err != nil {
				return err
			}

			clientCertPool := x509.NewCertPool()
			clientCertPool.AppendCertsFromPEM(clientCACert)

			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConfig.ClientCAs = clientCertPool
		}

		// Attach the tlsHandler to the config
		app.SetTLSHandler(tlsHandler)
	}

	if cfg.TLSConfigFunc != nil {
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
		return err
	}

	// prepare the server for the start
	app.startupProcess()

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
		fmt.Println("[Warning] Prefork isn't supported for custom listeners.")
	}

	return app.server.Serve(ln)
}

// Create listener function.
func (app *App) createListener(addr string, tlsConfig *tls.Config, cfg ListenConfig) (net.Listener, error) {
	var listener net.Listener
	var err error

	if tlsConfig != nil {
		listener, err = tls.Listen(cfg.ListenerNetwork, addr, tlsConfig)
	} else {
		listener, err = net.Listen(cfg.ListenerNetwork, addr)
	}

	if cfg.ListenerAddrFunc != nil {
		cfg.ListenerAddrFunc(listener.Addr())
	}

	return listener, err
}

func (app *App) printMessages(cfg ListenConfig, ln net.Listener) {
	// Print startup message
	if !cfg.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), getTlsConfig(ln) != nil, "", cfg)
	}

	// Print routes
	if cfg.EnablePrintRoutes {
		app.printRoutesMessage()
	}
}

// startupMessage prepares the startup message with the handler number, port, address and other information
func (app *App) startupMessage(addr string, tls bool, pids string, cfg ListenConfig) {
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
			host = "0.0.0.0"
		}
	}

	scheme := "http"
	if tls {
		scheme = "https"
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

	_, _ = fmt.Fprintf(out, "%s\n", fmt.Sprintf(figletFiberText, colors.Red+"v"+Version+colors.Reset))
	_, _ = fmt.Fprintf(out, strings.Repeat("-", 50)+"\n")

	if host == "0.0.0.0" {
		_, _ = fmt.Fprintf(out,
			"%sINFO%s Server started on %s%s://127.0.0.1:%s%s (bound on host 0.0.0.0 and port %s)\n",
			colors.Green, colors.Reset, colors.Blue, scheme, port, colors.Reset, port)
	} else {
		_, _ = fmt.Fprintf(out,
			"%sINFO%s Server started on %s%s%s\n",
			colors.Green, colors.Reset, colors.Blue, fmt.Sprintf("%s://%s:%s", scheme, host, port), colors.Reset)
	}

	if app.config.AppName != "" {
		_, _ = fmt.Fprintf(out, "%sINFO%s Application name: %s%s%s\n", colors.Green, colors.Reset, colors.Blue, app.config.AppName, colors.Reset)
	}
	_, _ = fmt.Fprintf(out,
		"%sINFO%s Total handlers count: %s%s%s\n",
		colors.Green, colors.Reset, colors.Blue, strconv.Itoa(int(app.handlersCount)), colors.Reset)
	if isPrefork == "Enabled" {
		_, _ = fmt.Fprintf(out, "%sINFO%s Prefork: %s%s%s\n", colors.Green, colors.Reset, colors.Blue, isPrefork, colors.Reset)
	} else {
		_, _ = fmt.Fprintf(out, "%sINFO%s Prefork: %s%s%s\n", colors.Green, colors.Reset, colors.Red, isPrefork, colors.Reset)
	}
	_, _ = fmt.Fprintf(out, "%sINFO%s PID: %s%v%s\n", colors.Green, colors.Reset, colors.Blue, os.Getpid(), colors.Reset)
	_, _ = fmt.Fprintf(out, "%sINFO%s Total process count: %s%s%s\n", colors.Green, colors.Reset, colors.Blue, procs, colors.Reset)

	if cfg.EnablePrefork {
		// Turn the `pids` variable (in the form ",a,b,c,d,e,f,etc") into a slice of PIDs
		var pidSlice []string
		for _, v := range strings.Split(pids, ",") {
			if v != "" {
				pidSlice = append(pidSlice, v)
			}
		}

		_, _ = fmt.Fprintf(out, "%sINFO%s Child PIDs: %s", colors.Green, colors.Reset, colors.Blue)
		totalPids := len(pidSlice)
		rowTotalPidCount := 10
		for i := 0; i < totalPids; i += rowTotalPidCount {
			start := i
			end := i + rowTotalPidCount
			if end > totalPids {
				end = totalPids
			}
			for n, pid := range pidSlice[start:end] {
				_, _ = fmt.Fprintf(out, "%s", pid)
				if n+1 != len(pidSlice[start:end]) {
					_, _ = fmt.Fprintf(out, ", ")
				}
			}
			_, _ = fmt.Fprintf(out, "\n%s", colors.Reset)
		}
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
			var newRoute = RouteMessage{}
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

	_, _ = fmt.Fprintf(w, "%smethod\t%s| %spath\t%s| %sname\t%s| %shandlers\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow)
	_, _ = fmt.Fprintf(w, "%s------\t%s| %s----\t%s| %s----\t%s| %s--------\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow)
	for _, route := range routes {
		_, _ = fmt.Fprintf(w, "%s%s\t%s| %s%s\t%s| %s%s\t%s| %s%s\n", colors.Blue, route.method, colors.White, colors.Green, route.path, colors.White, colors.Cyan, route.name, colors.White, colors.Yellow, route.handlers)
	}

	_ = w.Flush()
}

// shutdown goroutine
func (app *App) gracefulShutdown(ctx context.Context, cfg ListenConfig) {
	<-ctx.Done()

	if err := app.Shutdown(); err != nil {
		cfg.OnShutdownError(err)
	}

	if success := cfg.OnShutdownSuccess; success != nil {
		success()
	}
}