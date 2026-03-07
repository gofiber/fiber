package fiber

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp/reuseport"

	"github.com/gofiber/fiber/v3/log"
)

const (
	envPreforkChildKey = "FIBER_PREFORK_CHILD"
	envPreforkChildVal = "1"
	sleepDuration      = 100 * time.Millisecond
	windowsOS          = "windows"
)

var (
	testPreforkMaster = false
	testOnPrefork     = false
)

// IsChild determines if the current process is a child of Prefork
func IsChild() bool {
	return os.Getenv(envPreforkChildKey) == envPreforkChildVal
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
		// Linux will use SO_REUSEPORT and Windows falls back to SO_REUSEADDR
		// Only tcp4 or tcp6 is supported when preforking, both are not supported
		if ln, err = reuseport.Listen(cfg.ListenerNetwork, addr); err != nil {
			if !cfg.DisableStartupMessage {
				time.Sleep(sleepDuration) // avoid colliding with startup message
			}
			return fmt.Errorf("prefork: %w", err)
		}
		// wrap a tls config around the listener if provided
		if tlsConfig != nil {
			ln = tls.NewListener(ln, tlsConfig)
		}

		// kill current child proc when master exits
		masterPID := os.Getppid()
		go watchMaster(masterPID)

		// prepare the server for the start
		app.startupProcess()

		if cfg.ListenerAddrFunc != nil {
			cfg.ListenerAddrFunc(ln.Addr())
		}

		// listen for incoming connections
		return app.server.Serve(ln)
	}

	// 👮 master process 👮
	type child struct {
		err error
		pid int
	}
	// create variables
	maxProcs := runtime.GOMAXPROCS(0)
	children := make(map[int]*exec.Cmd)
	channel := make(chan child, maxProcs)

	// kill child procs when master exits
	defer func() {
		for _, proc := range children {
			if err = proc.Process.Kill(); err != nil {
				if !errors.Is(err, os.ErrProcessDone) {
					log.Errorf("prefork: failed to kill child: %v", err)
				}
			}
		}
	}()

	// collect child pids
	var childPIDs []int

	// launch child procs
	for range maxProcs {
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

		if err = cmd.Start(); err != nil {
			return fmt.Errorf("failed to start a child prefork process, error: %w", err)
		}

		// store child process
		pid := cmd.Process.Pid
		children[pid] = cmd
		childPIDs = append(childPIDs, pid)

		// execute fork hook
		if app.hooks != nil {
			if testOnPrefork {
				app.hooks.executeOnForkHooks(dummyPid)
			} else {
				app.hooks.executeOnForkHooks(pid)
			}
		}

		// notify master if child crashes
		go func() {
			channel <- child{pid: pid, err: cmd.Wait()}
		}()
	}

	// Run onListen hooks
	// Hooks have to be run here as different as non-prefork mode due to they should run as child or master
	listenData := app.prepareListenData(addr, tlsConfig != nil, cfg, childPIDs)

	app.runOnListenHooks(listenData)

	app.startupMessage(listenData, cfg)

	if cfg.EnablePrintRoutes {
		app.printRoutesMessage()
	}

	// return error if child crashes
	return (<-channel).err
}

// watchMaster watches the master process and exits if it dies.
// It detects master death by checking if the parent PID has changed,
// which happens when the master exits and the child is reparented to
// another process (often init/PID 1, but could be a subreaper).
func watchMaster(masterPID int) {
	if runtime.GOOS == windowsOS {
		// finds parent process,
		// and waits for it to exit
		p, err := os.FindProcess(masterPID)
		if err == nil {
			_, _ = p.Wait() //nolint:errcheck // It is fine to ignore the error here
		}
		os.Exit(1) //nolint:revive // Calling os.Exit is fine here in the prefork
	}
	// Watch for parent PID changes. When the master exits, the OS
	// reparents the child to another process, causing Getppid() to change.
	// Comparing against the original PID instead of hardcoding 1 ensures
	// this works correctly when the master itself is PID 1 (e.g. in
	// Docker containers).
	const watchInterval = 500 * time.Millisecond
	for range time.NewTicker(watchInterval).C {
		if os.Getppid() != masterPID {
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
