package fiber

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp/reuseport"

	"github.com/gofiber/fiber/v3/log"
)

const (
	envPreforkChildKey   = "FIBER_PREFORK_CHILD"
	envPreforkChildVal   = "1"
	envPreforkFDKey      = "FIBER_PREFORK_USE_FD"
	envPreforkFDVal      = "1"
	sleepDuration        = 100 * time.Millisecond
	windowsOS            = "windows"
	inheritedListenerFD  = 3 // First FD in ExtraFiles becomes FD 3
)

// childInfo tracks information about a child process
type childInfo struct {
	cmd           *exec.Cmd
	pid           int
	recoveryCount int
}

var (
	testPreforkMaster = false
	testOnPrefork     = false
)

// IsChild determines if the current process is a child of Prefork
func IsChild() bool {
	return os.Getenv(envPreforkChildKey) == envPreforkChildVal
}

// isReusePortError checks if the error is related to SO_REUSEPORT not being supported
func isReusePortError(err error) bool {
	if err == nil {
		return false
	}
	// Check for the specific ErrNoReusePort type from fasthttp
	var errNoReusePort *reuseport.ErrNoReusePort
	return errors.As(err, &errNoReusePort)
}

// testReuseportSupport checks if SO_REUSEPORT is supported on this system
func testReuseportSupport(network, addr string) error {
	ln, err := reuseport.Listen(network, addr)
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}

// startChildProcess starts a new child process for prefork
func startChildProcess(app *App, inheritedLn net.Listener) (*childInfo, error) {
	cmd := exec.Command(os.Args[0], os.Args[1:]...) //nolint:gosec // It's fine to launch the same process again
	if testPreforkMaster {
		// When test prefork master,
		// just start the child process with a dummy cmd,
		// which will exit soon
		cmd = dummyCmd()
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// add fiber prefork child flag into child proc env
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%s", envPreforkChildKey, envPreforkChildVal),
	)

	// If using file descriptor sharing (fallback mode), pass the listener FD
	if inheritedLn != nil {
		// Extract the file descriptor from the listener
		tcpLn, ok := inheritedLn.(*net.TCPListener)
		if !ok {
			return nil, fmt.Errorf("prefork: inherited listener is not a TCP listener")
		}

		file, err := tcpLn.File()
		if err != nil {
			return nil, fmt.Errorf("prefork: failed to get file descriptor from listener: %w", err)
		}

		// Pass the FD to the child process via ExtraFiles
		// ExtraFiles[0] will become FD 3 in the child
		cmd.ExtraFiles = []*os.File{file}

		// Tell the child to use the inherited FD
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("%s=%s", envPreforkFDKey, envPreforkFDVal),
		)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start a child prefork process: %w", err)
	}

	pid := cmd.Process.Pid

	// execute fork hook
	if app.hooks != nil {
		if testOnPrefork {
			app.hooks.executeOnForkHooks(dummyPid)
		} else {
			app.hooks.executeOnForkHooks(pid)
		}
	}

	return &childInfo{
		cmd:           cmd,
		pid:           pid,
		recoveryCount: 0,
	}, nil
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsConfig *tls.Config, cfg *ListenConfig) error {
	if cfg == nil {
		cfg = &ListenConfig{}
	}
	var ln net.Listener
	var err error

	// 👶 child process 👶
	if IsChild() {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)

		// Check if we should use inherited file descriptor (fallback mode)
		if os.Getenv(envPreforkFDKey) == envPreforkFDVal {
			// Recreate listener from inherited file descriptor
			// ExtraFiles[0] becomes FD 3 in the child process
			f := os.NewFile(inheritedListenerFD, "listener")
			if f == nil {
				if !cfg.DisableStartupMessage {
					time.Sleep(sleepDuration)
				}
				return fmt.Errorf("prefork: failed to recreate listener from file descriptor")
			}

			ln, err = net.FileListener(f)
			if err != nil {
				if !cfg.DisableStartupMessage {
					time.Sleep(sleepDuration)
				}
				return fmt.Errorf("prefork: failed to create listener from file: %w", err)
			}

			// Close the file as we don't need it anymore (listener is created)
			_ = f.Close()
		} else {
			// Use SO_REUSEPORT mode (default)
			// Linux will use SO_REUSEPORT and Windows falls back to SO_REUSEADDR
			// Only tcp4 or tcp6 is supported when preforking, both are not supported
			if ln, err = reuseport.Listen(cfg.ListenerNetwork, addr); err != nil {
				if !cfg.DisableStartupMessage {
					time.Sleep(sleepDuration) // avoid colliding with startup message
				}
				return fmt.Errorf("prefork: %w", err)
			}
		}

		// wrap a tls config around the listener if provided
		if tlsConfig != nil {
			ln = tls.NewListener(ln, tlsConfig)
		}

		// kill current child proc when master exits
		go watchMaster()

		// prepare the server for the start
		app.startupProcess()

		if cfg.ListenerAddrFunc != nil {
			cfg.ListenerAddrFunc(ln.Addr())
		}

		// listen for incoming connections
		return app.server.Serve(ln)
	}

	// 👮 master process 👮

	// In test mode with testPreforkMaster, disable child recovery automatically
	// to avoid endless loops with dummy children that exit immediately
	if testPreforkMaster && !cfg.DisableChildRecovery {
		cfg.DisableChildRecovery = true
	}

	// Test if SO_REUSEPORT is supported before spawning children
	var inheritedLn net.Listener
	if err = testReuseportSupport(cfg.ListenerNetwork, addr); err != nil {
		if isReusePortError(err) && !cfg.DisableReuseportFallback {
			log.Warn("[prefork] SO_REUSEPORT is not supported on this system, using file descriptor sharing fallback")
			// Create a single shared listener that will be passed to all children
			inheritedLn, err = net.Listen(cfg.ListenerNetwork, addr)
			if err != nil {
				return fmt.Errorf("prefork: failed to create shared listener for FD fallback: %w", err)
			}
			// Close the listener in the master process after all children have inherited it
			defer func() {
				if inheritedLn != nil {
					_ = inheritedLn.Close()
				}
			}()
			log.Info("[prefork] File descriptor sharing fallback enabled, all children will share the same socket")
		} else if isReusePortError(err) {
			// DisableReuseportFallback is true, fail
			return fmt.Errorf("prefork: SO_REUSEPORT not supported and fallback is disabled: %w", err)
		} else {
			return fmt.Errorf("prefork: failed to test SO_REUSEPORT support: %w", err)
		}
	}

	type childEvent struct {
		pid int
		err error
	}

	// create variables
	maxProcs := runtime.GOMAXPROCS(0)
	children := make(map[int]*childInfo)
	childEvents := make(chan childEvent, maxProcs)
	shutdownCh := make(chan struct{})
	var shutdownOnce sync.Once

	// Setup graceful shutdown handler if context provided
	if cfg.GracefulContext != nil {
		go func() {
			<-cfg.GracefulContext.Done()
			shutdownOnce.Do(func() { close(shutdownCh) })
		}()
	}

	// kill child procs when master exits
	defer func() {
		shutdownOnce.Do(func() { close(shutdownCh) })
		for _, child := range children {
			if child.cmd != nil && child.cmd.Process != nil {
				if err = child.cmd.Process.Kill(); err != nil {
					if !errors.Is(err, os.ErrProcessDone) {
						log.Errorf("[prefork] failed to kill child %d: %v", child.pid, err)
					}
				}
			}
		}
	}()

	// launch initial child procs
	var childPIDs []int
	for i := 0; i < maxProcs; i++ {
		child, err := startChildProcess(app, inheritedLn)
		if err != nil {
			return err
		}

		children[child.pid] = child
		childPIDs = append(childPIDs, child.pid)

		// monitor child process
		go func(c *childInfo) {
			childEvents <- childEvent{pid: c.pid, err: c.cmd.Wait()}
		}(child)
	}

	// Run onListen hooks
	// Hooks have to be run here as different as non-prefork mode due to they should run as child or master
	listenData := app.prepareListenData(addr, tlsConfig != nil, cfg, childPIDs)

	app.runOnListenHooks(listenData)

	app.startupMessage(listenData, cfg)

	if cfg.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// Monitor child processes and handle crashes
	for {
		select {
		case event := <-childEvents:
			child, exists := children[event.pid]
			if !exists {
				// Child was already cleaned up or doesn't exist
				continue
			}

			// Log child exit
			if event.err != nil {
				log.Errorf("[prefork] child process %d exited with error: %v", event.pid, event.err)
			} else {
				log.Warnf("[prefork] child process %d exited unexpectedly", event.pid)
			}

			// Check if recovery is disabled
			if cfg.DisableChildRecovery {
				log.Errorf("[prefork] child recovery is disabled, shutting down")
				return fmt.Errorf("child process %d crashed and recovery is disabled: %w", event.pid, event.err)
			}

			// Check if we've exceeded max recoveries for this child
			if cfg.MaxChildRecoveries > 0 && child.recoveryCount >= cfg.MaxChildRecoveries {
				log.Errorf("[prefork] child process %d exceeded max recovery attempts (%d), shutting down",
					event.pid, cfg.MaxChildRecoveries)
				return fmt.Errorf("child process %d exceeded max recovery attempts: %w", event.pid, event.err)
			}

			// Remove old child from map
			delete(children, event.pid)

			// Start new child process
			log.Infof("[prefork] recovering child process (previous PID: %d, recovery attempt: %d)",
				event.pid, child.recoveryCount+1)

			newChild, err := startChildProcess(app, inheritedLn)
			if err != nil {
				log.Errorf("[prefork] failed to recover child process: %v", err)
				return fmt.Errorf("failed to recover child process: %w", err)
			}

			// Inherit recovery count from crashed child
			newChild.recoveryCount = child.recoveryCount + 1
			children[newChild.pid] = newChild

			// Monitor new child process
			go func(c *childInfo) {
				childEvents <- childEvent{pid: c.pid, err: c.cmd.Wait()}
			}(newChild)

			log.Infof("[prefork] child process recovered (new PID: %d)", newChild.pid)

		case <-shutdownCh:
			// Graceful shutdown initiated
			return nil
		}
	}
}

// watchMaster watches child procs
func watchMaster() {
	if runtime.GOOS == windowsOS {
		// finds parent process,
		// and waits for it to exit
		p, err := os.FindProcess(os.Getppid())
		if err == nil {
			_, _ = p.Wait() //nolint:errcheck // It is fine to ignore the error here
		}
		os.Exit(1) //nolint:revive // Calling os.Exit is fine here in the prefork
	}
	// if it is equal to 1 (init process ID),
	// it indicates that the master process has exited
	const watchInterval = 500 * time.Millisecond
	for range time.NewTicker(watchInterval).C {
		if os.Getppid() == 1 {
			os.Exit(1) //nolint:revive // Calling os.Exit is fine here in the prefork
		}
	}
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
