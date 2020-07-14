package fiber

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	envPreforkChildKey = "FIBER_PREFORK_CHILD"
	envPreforkChildVal = "1"
)

// IsChild determines if the current process is a result of Prefork
func (app *App) IsChild() bool {
	return os.Getenv(envPreforkChildKey) == envPreforkChildVal
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsconfig ...*tls.Config) (err error) {
	// 👶 child process 👶
	if app.IsChild() {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		var ln net.Listener
		// SO_REUSEPORT is not supported on Windows, use SO_REUSEADDR instead
		if ln, err = reuseport(app.Settings.Network, addr); err != nil {
			if !app.Settings.DisableStartupMessage {
				time.Sleep(100 * time.Millisecond) // avoid colliding with startup message
			}
			return fmt.Errorf("prefork: %v", err)
		}
		// wrap a tls config around the listener if provided
		if len(tlsconfig) > 0 {
			ln = tls.NewListener(ln, tlsconfig[0])
		}
		// listen for incoming connections
		return app.server.Serve(ln)
	}

	// 👮 master process 👮
	type child struct {
		pid int
		err error
	}
	// create variables
	var max = runtime.GOMAXPROCS(0)
	var childs = make(map[int]*exec.Cmd)
	var channel = make(chan child, max)

	// kill child procs when master exits
	defer func() {
		for _, proc := range childs {
			_ = proc.Process.Kill()
		}
	}()

	// collect child pids
	pids := []string{}

	// launch child procs
	for i := 0; i < max; i++ {
		/* #nosec G204 */
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
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
		childs[cmd.Process.Pid] = cmd
		pids = append(pids, strconv.Itoa(cmd.Process.Pid))

		// notify master if child crashes
		go func() {
			channel <- child{cmd.Process.Pid, cmd.Wait()}
		}()
	}

	// kill child procs when master exits
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	go func() {
		<-c
		for _, proc := range childs {
			_ = proc.Process.Kill()
		}
		os.Exit(1)
	}()

	// Print startup message
	if !app.Settings.DisableStartupMessage {
		app.startupMessage(addr, len(tlsconfig) > 0, ","+strings.Join(pids, ","))
	}

	// return error if child crashes
	for sig := range channel {
		return sig.err
	}

	return
}
