package fiber

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"text/tabwriter"

	utils "github.com/gofiber/utils"
	colorable "github.com/segrey/go-colorable"
	reuseport "github.com/valyala/fasthttp/reuseport"
)

var (
	flagPrefork = "-prefork"
	flagChild   = "-prefork-child"
	isPrefork   bool
	isChild     bool
)

func init() { //nolint:gochecknoinits
	// Avoid panic when the user adds their own flags and runs `flag.Parse()`
	flag.BoolVar(&isPrefork, flagChild[1:], false, "use prefork")
	flag.BoolVar(&isChild, flagPrefork[1:], false, "is child proc")
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsconfig ...*tls.Config) (err error) {
	// 👶 child process 👶
	if utils.GetArgument(flagChild) {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		var ln net.Listener
		// get an SO_REUSEPORT listener or SO_REUSEADDR for windows
		if ln, err = reuseport.Listen("tcp4", addr); err != nil {
			return err
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
	var stdout = tabwriter.NewWriter(colorable.NewColorableStdout(), 0, 8, 0, ' ', 0)

	// kill child procs when master exits
	defer func() {
		for _, proc := range childs {
			_ = proc.Process.Kill()
		}
	}()

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
		// notify stdout
		fmt.Fprintf(stdout, "%sChild PID: %s#%v%s\n", cBlack, cGreen, cmd.Process.Pid, cReset)
		_ = stdout.Flush()
		// notify master if child crashes
		go func() {
			channel <- child{cmd.Process.Pid, cmd.Wait()}
		}()
	}
	// return error if child crashes
	for sig := range channel {
		return sig.err
	}

	return
}
