package fiber

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/valyala/fasthttp/reuseport"
)

const (
	envPreforkChildKey = "FIBER_PREFORK_CHILD"
	envPreforkChildVal = "1"
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
func (app *App) prefork(network, addr string, tlsConfig *tls.Config) error {
	// 👶 child process 👶
	if IsChild() {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		// Linux will use SO_REUSEPORT and Windows falls back to SO_REUSEADDR
		// Only tcp4 or tcp6 is supported when preforking, both are not supported
		ln, err := reuseport.Listen(network, addr)
		if err != nil {
			if !app.config.DisableStartupMessage {
				const sleepDuration = 100 * time.Millisecond
				time.Sleep(sleepDuration) // avoid colliding with startup message
			}
			return fmt.Errorf("prefork: %w", err)
		}
		// wrap a tls config around the listener if provided
		if tlsConfig != nil {
			ln = tls.NewListener(ln, tlsConfig)
		}

		// kill current child proc when master exits
		go watchMaster()

		// prepare the server for the start
		app.startupProcess()

		// listen for incoming connections
		return app.server.Serve(ln)
	}

	// 👮 master process 👮
	type child struct {
		pid int
		err error
	}
	// create variables
	max := runtime.GOMAXPROCS(0)
	childs := make(map[int]*exec.Cmd)
	channel := make(chan child, max)

	// kill child procs when master exits
	defer func() {
		for _, proc := range childs {
			if err := proc.Process.Kill(); err != nil {
				if !errors.Is(err, os.ErrProcessDone) {
					log.Errorf("prefork: failed to kill child: %v", err)
				}
			}
		}
	}()

	// collect child pids
	var pids []string

	// launch child procs
	for i := 0; i < max; i++ {
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
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start a child prefork process, error: %w", err)
		}

		// store child process
		pid := cmd.Process.Pid
		childs[pid] = cmd
		pids = append(pids, strconv.Itoa(pid))

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
			channel <- child{pid, cmd.Wait()}
		}()
	}

	// Run onListen hooks
	// Hooks have to be run here as different as non-prefork mode due to they should run as child or master
	app.runOnListenHooks(app.prepareListenData(addr, tlsConfig != nil))

	// Print startup message
	if !app.config.DisableStartupMessage {
		app.startupMessage(addr, tlsConfig != nil, ","+strings.Join(pids, ","))
	}

	// return error if child crashes
	return (<-channel).err
}

// watchMaster watches child procs
func watchMaster() {
	if runtime.GOOS == "windows" {
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
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command, "version")
	}
	return exec.Command(command, "version")
}
