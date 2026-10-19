package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"

	"github.com/valyala/fasthttp/prefork"
)

// Test seams for prefork testing - allows injecting dummy commands
var (
	testPreforkMaster = false
	testOnPrefork     = false
	dummyPid          = 1
	dummyChildCmd     atomic.Value
)

// IsChild determines if the current process is a child of Prefork
func IsChild() bool {
	return prefork.IsChild()
}

// prefork manages child processes to make use of the OS REUSEPORT feature.
// It delegates to fasthttp's prefork package to avoid duplicating process management logic.
func (app *App) prefork(addr string, tlsConfig *tls.Config, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	// Determine RecoverThreshold
	recoverThreshold := cfg.PreforkRecoverThreshold
	if recoverThreshold == 0 {
		recoverThreshold = max(1, runtime.GOMAXPROCS(0)/2)
	}

	// Use configured logger or default to Fiber's log package
	var logger prefork.Logger
	if cfg.PreforkLogger != nil {
		logger = cfg.PreforkLogger
	} else {
		logger = preforkLogger{}
	}

	p := &prefork.Prefork{
		Network:             cfg.ListenerNetwork,
		Reuseport:           true,
		RecoverThreshold:    recoverThreshold,
		RecoverInterval:     cfg.PreforkRecoverInterval,
		ShutdownGracePeriod: cfg.PreforkShutdownGracePeriod,
		Logger:              logger,
		OnMasterDeath:       func() { os.Exit(1) }, //nolint:revive // Exiting child process is intentional
	}

	// Use test command producer if in test mode
	if testPreforkMaster {
		p.CommandProducer = func(_ []*os.File) (*exec.Cmd, error) {
			cmd := dummyCmd()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				return cmd, fmt.Errorf("prefork: failed to start test command: %w", err)
			}
			return cmd, nil
		}
	}

	// Each process watches GracefulContext on its own. The master never serves: it watches from
	// the start, and canceling the context there only runs the master's shutdown hooks. A child
	// starts to watch in preforkServe, once it has its listener.
	if !IsChild() {
		graceful := app.watchGracefulContext(cfg)
		defer graceful.stopAndWait()
	}

	// Child process: serve function wraps TLS, starts up process, etc.
	p.ServeFunc = app.preforkServe(tlsConfig, cfg)

	// Master callback: child spawned → execute OnFork hooks
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

	// Master callback: all children spawned → startup message & OnListen hooks
	p.OnMasterReady = func(childPIDs []int) error {
		listenData := app.prepareListenData(addr, tlsConfig != nil, cfg, childPIDs)
		app.runOnListenHooks(listenData)
		app.printMessages(cfg, listenData)
		return nil
	}

	// Master callback: child recovered after crash
	p.OnChildRecover = func(oldPID, newPID int) {
		logger.Printf("prefork: child %d crashed, recovered with new PID %d", oldPID, newPID)
	}

	if err := p.ListenAndServe(addr); err != nil {
		return fmt.Errorf("prefork: %w", err)
	}

	return nil
}

// preforkServe returns what a prefork child runs on the listener it has bound: it starts to
// watch GracefulContext, wraps TLS around the listener, starts up the process and serves.
func (app *App) preforkServe(tlsConfig *tls.Config, cfg *ListenConfig) func(ln net.Listener) error {
	return func(ln net.Listener) error {
		// Watch the context only once the child has its listener, so that a bind that failed
		// never ran the shutdown hooks, and wait out the shutdown before returning.
		graceful := app.watchGracefulContext(cfg)
		defer graceful.stopAndWait()

		// wrap a tls config around the listener if provided
		if tlsConfig != nil {
			ln = tls.NewListener(ln, tlsConfig)
		}

		// prepare the server for the start
		app.startupProcess()
		app.hookHeaderReceived()

		if cfg.ListenerAddrFunc != nil {
			cfg.ListenerAddrFunc(ln.Addr())
		}

		// listen for incoming connections
		return app.serve(graceful, ln)
	}
}

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
