package fiber

import (
	"crypto/tls"
	"fmt"
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

func (app *App) executeOnForkHooks(pid int) {
	if app.hooks == nil {
		return
	}

	if testOnPrefork {
		app.hooks.executeOnForkHooks(dummyPid)
		return
	}

	app.hooks.executeOnForkHooks(pid)
}

func (app *App) newPrefork(cfg *ListenConfig, onMasterReady func(childPIDs []int) error) *prefork.Prefork {
	recoverThreshold := cfg.PreforkRecoverThreshold
	if recoverThreshold == 0 {
		recoverThreshold = runtime.GOMAXPROCS(0) / 2
	}

	p := &prefork.Prefork{
		Network:          cfg.ListenerNetwork,
		Reuseport:        true, // Fiber uses reuseport by default.
		RecoverThreshold: recoverThreshold,
		Logger:           preforkLogger{},
		WatchMaster:      true,
	}

	if testPreforkMaster {
		p.CommandProducer = func(files []*os.File) (*exec.Cmd, error) {
			cmd := dummyCmd()
			cmd.ExtraFiles = files
			if err := cmd.Start(); err != nil {
				return cmd, fmt.Errorf("failed to start dummy prefork command: %w", err)
			}
			return cmd, nil
		}
	}

	p.OnChildSpawn = func(pid int) error {
		app.executeOnForkHooks(pid)
		return nil
	}

	p.OnMasterReady = onMasterReady

	p.OnChildRecover = func(pid int) error {
		log.Warnf("prefork: child process crashed and has been recovered with new PID %d", pid)
		app.executeOnForkHooks(pid)
		return nil
	}

	return p
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsConfig *tls.Config, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	p := app.newPrefork(cfg, func(childPIDs []int) error {
		listenData := app.prepareListenData(addr, tlsConfig != nil, cfg, childPIDs)
		app.runOnListenHooks(listenData)
		app.startupMessage(listenData, cfg)
		if cfg.EnablePrintRoutes {
			app.printRoutesMessage()
		}
		return nil
	})

	p.ServeFunc = func(ln net.Listener) error {
		if prefork.IsChild() {
			if tlsConfig != nil {
				ln = tls.NewListener(ln, tlsConfig)
			}

			if !cfg.DisableStartupMessage {
				time.Sleep(sleepDuration)
			}

			app.startupProcess()

			if cfg.ListenerAddrFunc != nil {
				cfg.ListenerAddrFunc(ln.Addr())
			}
		}

		return app.server.Serve(ln)
	}

	if err := p.ListenAndServe(addr); err != nil {
		return fmt.Errorf("prefork listen and serve failed: %w", err)
	}
	return nil
}

// preforkListener manages child processes for prefork mode with a custom listener.
// This allows using prefork with app.Listener() when the user provides an OnPreforkServe callback.
func (app *App) preforkListener(ln net.Listener, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	addr := ln.Addr()
	tlsConfig := getTLSConfig(ln)
	p := app.newPrefork(cfg, func(childPIDs []int) error {
		listenData := app.prepareListenData(addr.String(), tlsConfig != nil, cfg, childPIDs)
		app.runOnListenHooks(listenData)
		app.startupMessage(listenData, cfg)
		if cfg.EnablePrintRoutes {
			app.printRoutesMessage()
		}
		return nil
	})

	p.ServeFunc = func(_ net.Listener) error {
		if prefork.IsChild() {
			childLn, err := cfg.OnPreforkServe(addr)
			if err != nil {
				return fmt.Errorf("on prefork serve callback failed: %w", err)
			}

			if tlsConfig != nil {
				childLn = tls.NewListener(childLn, tlsConfig)
			}

			if !cfg.DisableStartupMessage {
				time.Sleep(sleepDuration)
			}

			app.startupProcess()

			if cfg.ListenerAddrFunc != nil {
				cfg.ListenerAddrFunc(childLn.Addr())
			}

			return app.server.Serve(childLn)
		}

		return nil
	}

	if !prefork.IsChild() {
		if err := ln.Close(); err != nil {
			log.Warnf("prefork: failed to close original listener: %v", err)
		}
	}

	if err := p.ListenAndServe(addr.String()); err != nil {
		return fmt.Errorf("prefork listener serve failed: %w", err)
	}
	return nil
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
