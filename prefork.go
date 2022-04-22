package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp/reuseport"
)

const (
	envPreforkChildKey = "FIBER_PREFORK_CHILD"
	envPreforkChildVal = "1"
)

var testPreforkMaster = false

// IsChild determines if the current process is a child of Prefork
func IsChild() bool {
	return os.Getenv(envPreforkChildKey) == envPreforkChildVal
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(network, addr string, tlsConfig *tls.Config) (err error) {
	// 👶 child process 👶
	if IsChild() {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		var ln net.Listener
		// Linux will use SO_REUSEPORT and Windows falls back to SO_REUSEADDR
		// Only tcp4 or tcp6 is supported when preforking, both are not supported
		if ln, err = reuseport.Listen(network, addr); err != nil {
			if !app.config.DisableStartupMessage {
				time.Sleep(100 * time.Millisecond) // avoid colliding with startup message
			}
			return fmt.Errorf("prefork: %v", err)
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
			_ = proc.Process.Kill()
		}
	}()

	// collect child pids
	var pids []string

	// launch child procs
	for i := 0; i < max; i++ {
		/* #nosec G204 */
		cmd := exec.Command(os.Args[0], os.Args[1:]...) // #nosec G204
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
			return fmt.Errorf("failed to start a child prefork process, error: %v", err)
		}

		// store child process
		pid := cmd.Process.Pid
		childs[pid] = cmd
		pids = append(pids, strconv.Itoa(pid))

		// notify master if child crashes
		go func() {
			channel <- child{pid, cmd.Wait()}
		}()
	}

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
			_, _ = p.Wait()
		}
		os.Exit(1)
	}
	// if it is equal to 1 (init process ID),
	// it indicates that the master process has exited
	for range time.NewTicker(time.Millisecond * 500).C {
		if os.Getppid() == 1 {
			os.Exit(1)
		}
	}
}

var dummyChildCmd = "go"

// dummyCmd is for internal prefork testing
func dummyCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", dummyChildCmd, "version")
	}
	return exec.Command(dummyChildCmd, "version")
}
