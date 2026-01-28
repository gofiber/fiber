package fiber

import (
	"crypto/tls"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp/prefork"

	"github.com/gofiber/fiber/v3/log"
)

const (
	sleepDuration = 100 * time.Millisecond
	windowsOS     = "windows"
)

var (
	testPreforkMaster = false
	testOnPrefork     = false
)

// IsChild determines if the current process is a child of Prefork
func IsChild() bool {
	return prefork.IsChild()
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsConfig *tls.Config, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	// Determine RecoverThreshold
	recoverThreshold := cfg.PreforkRecoverThreshold
	if recoverThreshold == 0 {
		recoverThreshold = runtime.GOMAXPROCS(0) / 2
	}

	// Create FastHTTP Prefork instance
	p := &prefork.Prefork{
		Network:          cfg.ListenerNetwork,
		Reuseport:        true, // Fiber uses reuseport by default
		RecoverThreshold: recoverThreshold,
		Logger:           preforkLogger{},
		WatchMaster:      true, // Enable master process watching
	}

	// Use custom CommandProducer for testing
	if testPreforkMaster {
		p.CommandProducer = func(files []*os.File) (*exec.Cmd, error) {
			cmd := dummyCmd()
			cmd.ExtraFiles = files
			err := cmd.Start()
			return cmd, err
		}
	}

	// Configure ServeFunc for child processes
	p.ServeFunc = func(ln net.Listener) error {
		// Child process setup
		if prefork.IsChild() {
			// Wrap listener with TLS if configured
			if tlsConfig != nil {
				ln = tls.NewListener(ln, tlsConfig)
			}

			// Avoid startup message collision
			if !cfg.DisableStartupMessage {
				time.Sleep(sleepDuration)
			}

			// Prepare the server for the start
			app.startupProcess()

			// Call ListenerAddrFunc if provided
			if cfg.ListenerAddrFunc != nil {
				cfg.ListenerAddrFunc(ln.Addr())
			}
		}

		// Serve requests
		return app.server.Serve(ln)
	}

	// Configure OnChildSpawn callback
	p.OnChildSpawn = func(pid int) error {
		if app.hooks != nil {
			if testOnPrefork {
				app.hooks.executeOnForkHooks(dummyPid)
			} else {
				app.hooks.executeOnForkHooks(pid)
			}
		}
		return nil
	}

	// Configure OnMasterReady callback
	p.OnMasterReady = func(childPIDs []int) error {
		// Prepare listen data with child PIDs
		listenData := app.prepareListenData(addr, tlsConfig != nil, cfg, childPIDs)

		// Run OnListen hooks
		app.runOnListenHooks(listenData)

		// Display startup message
		app.startupMessage(listenData, cfg)

		// Print routes if enabled
		if cfg.EnablePrintRoutes {
			app.printRoutesMessage()
		}

		return nil
	}

	// Configure OnChildRecover callback for monitoring
	p.OnChildRecover = func(pid int) error {
		log.Warnf("prefork: child process crashed and has been recovered with new PID %d", pid)

		// Execute OnFork hook for recovered process if hooks are available
		if app.hooks != nil {
			app.hooks.executeOnForkHooks(pid)
		}

		return nil
	}

	// Start the prefork server
	return p.ListenAndServe(addr)
}

// preforkListener manages child processes for prefork mode with a custom listener.
// This allows using prefork with app.Listener() when the user provides an OnPreforkServe callback.
func (app *App) preforkListener(ln net.Listener, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	// Get the address from the provided listener
	addr := ln.Addr()

	// Determine RecoverThreshold
	recoverThreshold := cfg.PreforkRecoverThreshold
	if recoverThreshold == 0 {
		recoverThreshold = runtime.GOMAXPROCS(0) / 2
	}

	// Extract TLS config from listener if present
	tlsConfig := getTLSConfig(ln)

	// Create FastHTTP Prefork instance
	p := &prefork.Prefork{
		Network:          cfg.ListenerNetwork,
		Reuseport:        true, // Custom listener is expected to use reuseport
		RecoverThreshold: recoverThreshold,
		Logger:           preforkLogger{},
		WatchMaster:      true, // Enable master process watching
	}

	// Configure ServeFunc for child processes
	p.ServeFunc = func(_ net.Listener) error {
		// Child process: create new listener using user's callback
		if prefork.IsChild() {
			childLn, err := cfg.OnPreforkServe(addr)
			if err != nil {
				return err
			}

			// Wrap with TLS if original listener had TLS
			if tlsConfig != nil {
				childLn = tls.NewListener(childLn, tlsConfig)
			}

			// Avoid startup message collision
			if !cfg.DisableStartupMessage {
				time.Sleep(sleepDuration)
			}

			// Prepare the server for the start
			app.startupProcess()

			// Call ListenerAddrFunc if provided
			if cfg.ListenerAddrFunc != nil {
				cfg.ListenerAddrFunc(childLn.Addr())
			}

			// Serve requests using the child's listener
			return app.server.Serve(childLn)
		}

		// Master process should not reach here in normal operation
		return nil
	}

	// Configure OnChildSpawn callback
	p.OnChildSpawn = func(pid int) error {
		if app.hooks != nil {
			if testOnPrefork {
				app.hooks.executeOnForkHooks(dummyPid)
			} else {
				app.hooks.executeOnForkHooks(pid)
			}
		}
		return nil
	}

	// Configure OnMasterReady callback
	p.OnMasterReady = func(childPIDs []int) error {
		// Prepare listen data with child PIDs
		listenData := app.prepareListenData(addr.String(), tlsConfig != nil, cfg, childPIDs)

		// Run OnListen hooks
		app.runOnListenHooks(listenData)

		// Display startup message
		app.startupMessage(listenData, cfg)

		// Print routes if enabled
		if cfg.EnablePrintRoutes {
			app.printRoutesMessage()
		}

		return nil
	}

	// Configure OnChildRecover callback for monitoring
	p.OnChildRecover = func(pid int) error {
		log.Warnf("prefork: child process crashed and has been recovered with new PID %d", pid)

		// Execute OnFork hook for recovered process if hooks are available
		if app.hooks != nil {
			app.hooks.executeOnForkHooks(pid)
		}

		return nil
	}

	// Close the original listener in master process since children will create their own
	if !prefork.IsChild() {
		if err := ln.Close(); err != nil {
			log.Warnf("prefork: failed to close original listener: %v", err)
		}
	}

	// Start the prefork server using the address from the original listener
	return p.ListenAndServe(addr.String())
}

var (
	dummyPid      = 1
	dummyChildCmd atomic.Value
)

// dummyCmd is for internal prefork testing
func dummyCmd() *exec.Cmd {
	command := "go"
	if storeCommand := dummyChildCmd.Load(); storeCommand != nil && storeCommand != "" {
		command = storeCommand.(string) //nolint:forcetypeassert,errcheck // We always store a string in here
	}
	if runtime.GOOS == windowsOS {
		return exec.Command("cmd", "/C", command, "version")
	}
	return exec.Command(command, "version")
}
