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
	"github.com/mattn/go-runewidth"

	"github.com/gofiber/fiber/v2/log"
)

const (
	globalIpv4Addr = "0.0.0.0"
)

// Listener can be used to pass a custom listener.
func (app *App) Listener(ln net.Listener) error {
	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil))

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), getTLSConfig(ln) != nil, "")
	}

	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Prefork is not supported for custom listeners
	if app.config.Prefork {
		log.Warn("Prefork isn't supported for custom listeners.")
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
		return fmt.Errorf("failed to listen: %w", err)
	}

	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), false))

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
		return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %w", certFile, keyFile, err)
	}

	return app.ListenTLSWithCertificate(addr, cert)
}

// ListenTLS serves HTTPS requests from the given addr.
// cert is a tls.Certificate
//
//	app.ListenTLSWithCertificate(":8080", cert)
func (app *App) ListenTLSWithCertificate(addr string, cert tls.Certificate) error {
	tlsHandler := &TLSHandler{}
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
		return fmt.Errorf("failed to listen: %w", err)
	}

	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil))

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}

	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Attach the tlsHandler to the config
	app.SetTLSHandler(tlsHandler)

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
		return fmt.Errorf("tls: cannot load TLS key pair from certFile=%q and keyFile=%q: %w", certFile, keyFile, err)
	}

	clientCACert, err := os.ReadFile(filepath.Clean(clientCertFile))
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	clientCertPool := x509.NewCertPool()
	clientCertPool.AppendCertsFromPEM(clientCACert)

	return app.ListenMutualTLSWithCertificate(addr, cert, clientCertPool)
}

// ListenMutualTLSWithCertificate serves HTTPS requests from the given addr.
// cert is a tls.Certificate and clientCertPool is a *x509.CertPool:
//
//	app.ListenMutualTLS(":8080", cert, clientCertPool)
func (app *App) ListenMutualTLSWithCertificate(addr string, cert tls.Certificate, clientCertPool *x509.CertPool) error {
	tlsHandler := &TLSHandler{}
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
		return fmt.Errorf("failed to listen: %w", err)
	}

	// prepare the server for the start
	app.startupProcess()

	// run hooks
	app.runOnListenHooks(app.prepareListenData(ln.Addr().String(), getTLSConfig(ln) != nil))

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(ln.Addr().String(), true, "")
	}

	// Print routes
	if app.config.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Attach the tlsHandler to the config
	app.SetTLSHandler(tlsHandler)

	// Start listening
	return app.server.Serve(ln)
}

// prepareListenData create an slice of ListenData
func (app *App) prepareListenData(addr string, isTLS bool) ListenData { //revive:disable-line:flag-parameter // Accepting a bool param named isTLS if fine here
	host, port := parseAddr(addr)
	if host == "" {
		if app.config.Network == NetworkTCP6 {
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
func (app *App) startupMessage(addr string, isTLS bool, pids string) { //nolint: revive // Accepting a bool param named isTLS if fine here
	// ignore child processes
	if IsChild() {
		return
	}

	// Alias colors
	colors := app.config.ColorScheme

	value := func(s string, width int) string {
		pad := width - len(s)
		str := ""
		for i := 0; i < pad; i++ {
			str += "."
		}
		if s == "Disabled" {
			str += " " + s
		} else {
			str += fmt.Sprintf(" %s%s%s", colors.Cyan, s, colors.Black)
		}
		return str
	}

	center := func(s string, width int) string {
		const padDiv = 2
		pad := strconv.Itoa((width - len(s)) / padDiv)
		str := fmt.Sprintf("%"+pad+"s", " ")
		str += s
		str += fmt.Sprintf("%"+pad+"s", " ")
		if len(str) < width {
			str += " "
		}
		return str
	}

	centerValue := func(s string, width int) string {
		const padDiv = 2
		pad := strconv.Itoa((width - runewidth.StringWidth(s)) / padDiv)
		str := fmt.Sprintf("%"+pad+"s", " ")
		str += fmt.Sprintf("%s%s%s", colors.Cyan, s, colors.Black)
		str += fmt.Sprintf("%"+pad+"s", " ")
		if runewidth.StringWidth(s)-10 < width && runewidth.StringWidth(s)%2 == 0 {
			// add an ending space if the length of str is even and str is not too long
			str += " "
		}
		return str
	}

	pad := func(s string, width int) string {
		toAdd := width - len(s)
		str := s
		for i := 0; i < toAdd; i++ {
			str += " "
		}
		return str
	}

	host, port := parseAddr(addr)
	if host == "" {
		if app.config.Network == NetworkTCP6 {
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
	if app.config.Prefork {
		isPrefork = "Enabled"
	}

	procs := strconv.Itoa(runtime.GOMAXPROCS(0))
	if !app.config.Prefork {
		procs = "1"
	}

	const lineLen = 49
	mainLogo := colors.Black + " ┌───────────────────────────────────────────────────┐\n"
	if app.config.AppName != "" {
		mainLogo += " │ " + centerValue(app.config.AppName, lineLen) + " │\n"
	}
	mainLogo += " │ " + centerValue("Fiber v"+Version, lineLen) + " │\n"

	if host == globalIpv4Addr {
		mainLogo += " │ " + center(fmt.Sprintf("%s://127.0.0.1:%s", scheme, port), lineLen) + " │\n" +
			" │ " + center(fmt.Sprintf("(bound on host 0.0.0.0 and port %s)", port), lineLen) + " │\n"
	} else {
		mainLogo += " │ " + center(fmt.Sprintf("%s://%s:%s", scheme, host, port), lineLen) + " │\n"
	}

	mainLogo += fmt.Sprintf(
		" │                                                   │\n"+
			" │ Handlers %s  Processes %s │\n"+
			" │ Prefork .%s  PID ....%s │\n"+
			" └───────────────────────────────────────────────────┘"+
			colors.Reset,
		value(strconv.Itoa(int(app.handlersCount)), 14), value(procs, 12),
		value(isPrefork, 14), value(strconv.Itoa(os.Getpid()), 14),
	)

	var childPidsLogo string
	if app.config.Prefork {
		var childPidsTemplate string
		childPidsTemplate += "%s"
		childPidsTemplate += " ┌───────────────────────────────────────────────────┐\n%s"
		childPidsTemplate += " └───────────────────────────────────────────────────┘"
		childPidsTemplate += "%s"

		newLine := " │ %s%s%s │"

		// Turn the `pids` variable (in the form ",a,b,c,d,e,f,etc") into a slice of PIDs
		var pidSlice []string
		for _, v := range strings.Split(pids, ",") {
			if v != "" {
				pidSlice = append(pidSlice, v)
			}
		}

		var lines []string
		thisLine := "Child PIDs ... "
		var itemsOnThisLine []string

		const maxLineLen = 49

		addLine := func() {
			lines = append(lines,
				fmt.Sprintf(
					newLine,
					colors.Black,
					thisLine+colors.Cyan+pad(strings.Join(itemsOnThisLine, ", "), maxLineLen-len(thisLine)),
					colors.Black,
				),
			)
		}

		for _, pid := range pidSlice {
			if len(thisLine+strings.Join(append(itemsOnThisLine, pid), ", ")) > maxLineLen {
				addLine()
				thisLine = ""
				itemsOnThisLine = []string{pid}
			} else {
				itemsOnThisLine = append(itemsOnThisLine, pid)
			}
		}

		// Add left over items to their own line
		if len(itemsOnThisLine) != 0 {
			addLine()
		}

		// Form logo
		childPidsLogo = fmt.Sprintf(childPidsTemplate,
			colors.Black,
			strings.Join(lines, "\n")+"\n",
			colors.Reset,
		)
	}

	// Combine both the child PID logo and the main Fiber logo

	// Pad the shorter logo to the length of the longer one
	splitMainLogo := strings.Split(mainLogo, "\n")
	splitChildPidsLogo := strings.Split(childPidsLogo, "\n")

	mainLen := len(splitMainLogo)
	childLen := len(splitChildPidsLogo)

	if mainLen > childLen {
		diff := mainLen - childLen
		for i := 0; i < diff; i++ {
			splitChildPidsLogo = append(splitChildPidsLogo, "")
		}
	} else {
		diff := childLen - mainLen
		for i := 0; i < diff; i++ {
			splitMainLogo = append(splitMainLogo, "")
		}
	}

	// Combine the two logos, line by line
	output := "\n"
	for i := range splitMainLogo {
		output += colors.Black + splitMainLogo[i] + " " + splitChildPidsLogo[i] + "\n"
	}

	out := colorable.NewColorableStdout()
	if os.Getenv("TERM") == "dumb" || os.Getenv("NO_COLOR") == "1" || (!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd())) {
		out = colorable.NewNonColorable(os.Stdout)
	}

	_, _ = fmt.Fprintln(out, output)
}

// printRoutesMessage print all routes with method, path, name and handlers
// in a format of table, like this:
// method | path | name      | handlers
// GET    | /    | routeName | github.com/gofiber/fiber/v2.emptyHandler
// HEAD   | /    |           | github.com/gofiber/fiber/v2.emptyHandler
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

	_, _ = fmt.Fprintf(w, "%smethod\t%s| %spath\t%s| %sname\t%s| %shandlers\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset)
	_, _ = fmt.Fprintf(w, "%s------\t%s| %s----\t%s| %s----\t%s| %s--------\t%s\n", colors.Blue, colors.White, colors.Green, colors.White, colors.Cyan, colors.White, colors.Yellow, colors.Reset)
	for _, route := range routes {
		_, _ = fmt.Fprintf(w, "%s%s\t%s| %s%s\t%s| %s%s\t%s| %s%s%s\n", colors.Blue, route.method, colors.White, colors.Green, route.path, colors.White, colors.Cyan, route.name, colors.White, colors.Yellow, route.handlers, colors.Reset)
	}

	_ = w.Flush() //nolint:errcheck // It is fine to ignore the error here
}
