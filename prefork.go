package fiber

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	utils "github.com/gofiber/utils"
	"github.com/valyala/fasthttp/reuseport"
	"golang.org/x/sys/windows"
)

var (
	flagPrefork = "-prefork"
	flagChild   = "-prefork-child"
	isPrefork   bool
	isChild     bool
)

func init() { //nolint:gochecknoinits
	// Avoid panic when the user adds their own flags and runs `flag.Parse()`
	flag.BoolVar(&isPrefork, flagPrefork[1:], false, "Prefork")
	flag.BoolVar(&isChild, flagChild[1:], false, "Child Process")
}

// Windows machines do not support REUSEPORT, so we use REUSEADDR instead
var reuseaddr = net.ListenConfig{
	Control: func(network, address string, c syscall.RawConn) (err error) {
		return c.Control(func(fd uintptr) {
			err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
		})
	},
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsconfig ...*tls.Config) (err error) {
	// 👶 child process 👶
	if utils.GetArgument(flagChild) {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		var ln net.Listener
		// Use SO_REUSEADDR for windows and SO_REUSEPORT for other systems
		if runtime.GOOS == "windows" {
			ln, err = reuseaddr.Listen(context.Background(), "tcp4", addr)
		} else {
			ln, err = reuseport.Listen("tcp4", addr)
		}
		if err != nil {
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
		cmd := exec.Command(os.Args[0], append(os.Args[1:], flagChild)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			return fmt.Errorf("failed to start a child prefork process, error: %v\n", err)
		}
		// store child process
		childs[cmd.Process.Pid] = cmd
		pids = append(pids, strconv.Itoa(cmd.Process.Pid))
		// notify master if child crashes
		go func() {
			channel <- child{cmd.Process.Pid, cmd.Wait()}
		}()
	}

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
