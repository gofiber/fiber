// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
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

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

const figletFiberText = `
    _______ __             
   / ____(_) /_  ___  _____
  / /_  / / __ \/ _ \/ ___/
 / __/ / / /_/ /  __/ /    
/_/   /_/_.___/\___/_/     `

var (
	resetColor = "\033[0m"
	red        = "\033[31m"
	green      = "\033[32m"
	blue       = "\033[34m"
)

func init() {
	if runtime.GOOS == "windows" {
		resetColor = ""
		red = ""
		green = ""
		blue = ""
	}
}

// Listener can be used to pass a custom listener.
func (app *App) Listener(ln net.Listener) error {
	// Prefork is supported for custom listeners
	if app.config.Prefork {
		addr, tlsConfig := lnMetadata(app.config.Network, ln)
		return app.prefork(app.config.Network, addr, tlsConfig)
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), getTlsConfig(ln) != nil, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}
	// Start listening
	return app.server.Serve(ln)
}

// Listen serves HTTP requests from the given addr.
//
//	app.Listen(":8080")
//	app.Listen("127.0.0.1:8080")
func (app *App) Listen(addr string) error {
	// Start prefork
	if app.config.Prefork {
		return app.prefork(app.config.Network, addr, nil)
	}
	// Setup listener
	ln, err := net.Listen(app.config.Network, addr)
	if err != nil {
		return err
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), false, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}
	// Start listening
	return app.server.Serve(ln)
}

// ListenTLS serves HTTPS requests from the given addr.
// certFile and keyFile are the paths to TLS certificate and key file:
//
//	app.ListenTLS(":8080", "./cert.pem", "./cert.key")
func (app *App) ListenTLS(addr, certFile, keyFile string) error {
	// Check for valid cert/key path
	if len(certFile) == 0 || len(keyFile) == 0 {
		return errors.New("tls: provide a valid cert or key path")
	}
	// Set TLS config with handler
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %s", certFile, keyFile, err)
	}
	tlsHandler := &tlsHandler{}
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{
			cert,
		},
		GetCertificate: tlsHandler.GetClientInfo,
	}
	// Prefork is supported
	if app.config.Prefork {
		return app.prefork(app.config.Network, addr, config)
	}

	// Setup listener
	ln, err := net.Listen(app.config.Network, addr)
	ln = tls.NewListener(ln, config)

	if err != nil {
		return err
	}
	// prepare the server for the start
	app.startupProcess()
	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}
	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Attach the tlsHandler to the config
	app.tlsHandler = tlsHandler

	// Start listening
	return app.server.Serve(ln)
}

// ListenMutualTLS serves HTTPS requests from the given addr.
// certFile, keyFile and clientCertFile are the paths to TLS certificate and key file:
//
//	app.ListenMutualTLS(":8080", "./cert.pem", "./cert.key", "./client.pem")
func (app *App) ListenMutualTLS(addr, certFile, keyFile, clientCertFile string) error {
	// Check for valid cert/key path
	if len(certFile) == 0 || len(keyFile) == 0 {
		return errors.New("tls: provide a valid cert or key path")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %s", certFile, keyFile, err)
	}

	clientCACert, err := os.ReadFile(filepath.Clean(clientCertFile))
	if err != nil {
		return err
	}
	clientCertPool := x509.NewCertPool()
	clientCertPool.AppendCertsFromPEM(clientCACert)

	tlsHandler := &tlsHandler{}
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
		Certificates: []tls.Certificate{
			cert,
		},
		GetCertificate: tlsHandler.GetClientInfo,
	}

	// Prefork is supported
	if app.config.Prefork {
		return app.prefork(app.config.Network, addr, config)
	}

	// Setup listener
	ln, err := tls.Listen(app.config.Network, addr, config)
	if err != nil {
		return err
	}

	// prepare the server for the start
	app.startupProcess()

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}

	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Attach the tlsHandler to the config
	app.tlsHandler = tlsHandler

	// Start listening
	return app.server.Serve(ln)
}

// startupMessage prepares the startup message with the handler number, port, address and other information
func (app *App) startupMessage(addr string, tls bool, pids string) {
	// ignore child processes
	if IsChild() {
		return
	}

	host, port := parseAddr(addr)
	if host == "" {
		if app.config.Network == NetworkTCP6 {
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
	if app.config.Prefork {
		isPrefork = "Enabled"
	}

	procs := strconv.Itoa(runtime.GOMAXPROCS(0))
	if !app.config.Prefork {
		procs = "1"
	}

	out := os.Stdout
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		//out = colorable.NewNonColorable(os.Stdout)
	}
	_, _ = fmt.Fprintf(out, "%s\n\n", figletFiberText)
	if app.config.AppName != "" {
		_, _ = fmt.Fprintf(out, "%sINFO%s Application name: %s%s%s\n", green, resetColor, blue, app.config.AppName, resetColor)
	}
	_, _ = fmt.Fprintf(out, "%sINFO%s Fiber version: %sv+%s%s\n", green, resetColor, blue, Version, resetColor)

	if host == "0.0.0.0" {
		_, _ = fmt.Fprintf(out,
			"%sINFO%s Server started on %s%s://127.0.0.1:%s%s (bound on host 0.0.0.0 and port %s)\n",
			green, resetColor, blue, scheme, port, resetColor, port)
	} else {
		_, _ = fmt.Fprintf(out,
			"%sINFO%s Server started on %s%s%s\n",
			green, resetColor, blue, fmt.Sprintf("%s://%s:%s", scheme, host, port), resetColor)
	}

	_, _ = fmt.Fprintf(out,
		"%sINFO%s Total handlers count: %s%s%s\n",
		green, resetColor, blue, strconv.Itoa(int(app.handlersCount)), resetColor)
	if isPrefork == "Enabled" {
		_, _ = fmt.Fprintf(out, "%sINFO%s Prefork: %s%s%s\n", green, resetColor, blue, isPrefork, resetColor)
	} else {
		_, _ = fmt.Fprintf(out, "%sINFO%s Prefork: %s%s%s\n", green, resetColor, red, isPrefork, resetColor)
	}
	_, _ = fmt.Fprintf(out, "%sINFO%s PID: %s%v%s\n", green, resetColor, blue, os.Getpid(), resetColor)
	_, _ = fmt.Fprintf(out, "%sINFO%s Total process count: %s%s%s\n", green, resetColor, blue, procs, resetColor)

	if app.config.Prefork {
		// Turn the `pids` variable (in the form ",a,b,c,d,e,f,etc") into a slice of PIDs
		var pidSlice []string
		for _, v := range strings.Split(pids, ",") {
			if v != "" {
				pidSlice = append(pidSlice, v)
			}
		}

		_, _ = fmt.Fprintf(out, "%sINFO%s Child PIDs: ", green, resetColor)
		totalPids := len(pidSlice)
		rowTotalPidCount := 10
		for i := 0; i < totalPids; i += rowTotalPidCount {
			start := i
			end := i + rowTotalPidCount
			if end > totalPids {
				end = totalPids
			}
			for _, pid := range pidSlice[start:end] {
				_, _ = fmt.Fprintf(out, "%s, ", pid)
			}
			_, _ = fmt.Fprintf(out, "\n")
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
