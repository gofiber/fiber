package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/valyala/fasthttp/prefork"

	"github.com/gofiber/fiber/v3/log"
)

const (
	sleepDuration = 100 * time.Millisecond
)

// IsChild determines if the current process is a child of Prefork.
func IsChild() bool {
	return prefork.IsChild()
}

func (app *App) executeOnForkHooks(pid int) {
	if app.hooks == nil {
		return
	}

	hookPID := pid
	if preforkHookPIDOverride != nil {
		hookPID = preforkHookPIDOverride(pid)
	}
	app.hooks.executeOnForkHooks(hookPID)
}

func (app *App) setupPreforkChildListener(ln net.Listener, tlsConfig *tls.Config, cfg *ListenConfig) net.Listener {
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
	return ln
}

func (app *App) onPreforkMasterReady(addr string, isTLS bool, cfg *ListenConfig) func(childPIDs []int) error {
	return func(childPIDs []int) error {
		listenData := app.prepareListenData(addr, isTLS, cfg, childPIDs)
		app.runOnListenHooks(listenData)
		app.printMessages(cfg, listenData)
		return nil
	}
}

func (app *App) newPrefork(cfg *ListenConfig, onMasterReady func(childPIDs []int) error) *prefork.Prefork {
	recoverThreshold := cfg.PreforkRecoverThreshold
	if recoverThreshold == 0 {
		recoverThreshold = runtime.GOMAXPROCS(0) / 2
	}

	p := &prefork.Prefork{
		Network:          cfg.ListenerNetwork,
		Reuseport:        true,
		RecoverThreshold: recoverThreshold,
		Logger:           preforkLogger{},
		WatchMaster:      true,
		OnMasterReady:    onMasterReady,
	}

	if preforkCommandProducer != nil {
		p.CommandProducer = preforkCommandProducer
	}

	p.OnChildSpawn = func(pid int) error {
		app.executeOnForkHooks(pid)
		return nil
	}
	p.OnChildRecover = func(pid int) error {
		log.Warnf("prefork: child process crashed and has been recovered with new PID %d", pid)
		app.executeOnForkHooks(pid)
		return nil
	}

	return p
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature.
func (app *App) prefork(addr string, tlsConfig *tls.Config, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	p := app.newPrefork(cfg, app.onPreforkMasterReady(addr, tlsConfig != nil, cfg))
	p.ServeFunc = func(ln net.Listener) error {
		if prefork.IsChild() {
			ln = app.setupPreforkChildListener(ln, tlsConfig, cfg)
		}
		return app.server.Serve(ln)
	}

	if err := p.ListenAndServe(addr); err != nil {
		return fmt.Errorf("prefork listen and serve failed: %w", err)
	}
	return nil
}

// preforkListener manages child processes for prefork mode with a custom listener.
func (app *App) preforkListener(ln net.Listener, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}

	addr := ln.Addr()
	tlsConfig := getTLSConfig(ln)
	p := app.newPrefork(cfg, app.onPreforkMasterReady(addr.String(), tlsConfig != nil, cfg))
	p.ServeFunc = func(_ net.Listener) error {
		if !prefork.IsChild() {
			return nil
		}

		childLn, err := cfg.OnPreforkServe(addr)
		if err != nil {
			return fmt.Errorf("on prefork serve callback failed: %w", err)
		}
		childLn = app.setupPreforkChildListener(childLn, tlsConfig, cfg)
		return app.server.Serve(childLn)
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
