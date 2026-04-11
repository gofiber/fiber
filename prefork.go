package fiber

import (
	"crypto/tls"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"

	"github.com/valyala/fasthttp/prefork"

	"github.com/gofiber/fiber/v3/log"
)

const windowsOS = "windows"

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
		recoverThreshold = runtime.GOMAXPROCS(0) / 2
	}

	p := &prefork.Prefork{
		Network:          cfg.ListenerNetwork,
		Reuseport:        true,
		RecoverThreshold: recoverThreshold,
		Logger:           preforkLogger{},
		OnMasterDeath:    func() { os.Exit(1) }, //nolint:revive // Exiting child process is intentional
	}

	// Use test command producer if in test mode
	if testPreforkMaster {
		p.CommandProducer = func(files []*os.File) (*exec.Cmd, error) {
			cmd := dummyCmd()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Start()
			return cmd, err
		}
	}

	// Child process: serve function wraps TLS, starts up process, etc.
	p.ServeFunc = func(ln net.Listener) error {
		// wrap a tls config around the listener if provided
		if tlsConfig != nil {
			ln = tls.NewListener(ln, tlsConfig)
		}

		// prepare the server for the start
		app.startupProcess()

		if cfg.ListenerAddrFunc != nil {
			cfg.ListenerAddrFunc(ln.Addr())
		}

		// listen for incoming connections
		return app.server.Serve(ln)
	}

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
	p.OnChildRecover = func(pid int) error {
		log.Warnf("prefork: child process crashed, recovered with new PID %d", pid)
		if app.hooks != nil {
			if testOnPrefork {
				app.hooks.executeOnForkHooks(dummyPid)
			} else {
				app.hooks.executeOnForkHooks(pid)
			}
		}
		return nil
	}

	return p.ListenAndServe(addr)
}

// dummyCmd is for internal prefork testing
func dummyCmd() *exec.Cmd {
	command := "go"
	if storeCommand := dummyChildCmd.Load(); storeCommand != nil && storeCommand != "" {
		command = storeCommand.(string) //nolint:forcetypeassert,errcheck // We always store a string in here
	}
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command, "version")
	}
	return exec.Command(command, "version")
}
