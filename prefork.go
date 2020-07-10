package fiber

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	utils "github.com/gofiber/utils"
)

var (
	flagChild = "-prefork-child"
	isChild   bool
)

func init() { //nolint:gochecknoinits
	// Prevent users from defining the same flag on their own.
	flag.BoolVar(&isChild, flagChild[1:], false, "Child Process")
	// Change the default usage message so the child flag isn't exposed to users of the app
	// when for example running `app -help`.
	flag.Usage = usage
}

// IsChild determines if the current process is a result of Prefork
func (app *App) IsChild() bool {
	return utils.GetArgument(flagChild)
}

// prefork manages child processes to make use of the OS REUSEPORT or REUSEADDR feature
func (app *App) prefork(addr string, tlsconfig ...*tls.Config) (err error) {
	// 👶 child process 👶
	if app.IsChild() {
		// use 1 cpu core per child process
		runtime.GOMAXPROCS(1)
		var ln net.Listener
		// SO_REUSEPORT is not supported on Windows, use SO_REUSEADDR instead
		if ln, err = reuseport("tcp4", addr); err != nil {
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

// -- string Value
// This code is copied from the stdlib.
type stringValue string

// This code is copied from the stdlib.
func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

// This code is copied from the stdlib.
func (s *stringValue) Get() interface{} { return string(*s) }

// This code is copied from the stdlib.
func (s *stringValue) String() string { return string(*s) }

// usage prints a usage message documenting all defined command-line flags,
// but skips printing our `-prefork-child` flag as it shouldn't be exposed.
// This code is based on the stdlib with the only change to skip that flag.
func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		// Skip printing usage info for our `-prefork-child` flag
		if f.Name == flagChild[1:] {
			return
		}
		s := fmt.Sprintf("  -%s", f.Name) // Two spaces before -; see next two comments.
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			s += " " + name
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if len(s) <= 4 { // space, space, '-', 'x'.
			s += "\t"
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
		}
		s += strings.Replace(usage, "\n", "\n    \t", -1)

		if !isZeroValue(f, f.DefValue) {
			if _, ok := f.Value.(*stringValue); ok {
				// put quotes on the value
				s += fmt.Sprintf(" (default %q)", f.DefValue)
			} else {
				s += fmt.Sprintf(" (default %v)", f.DefValue)
			}
		}
		fmt.Fprint(flag.CommandLine.Output(), s, "\n")
	})
}

// isZeroValue determines whether the string represents the zero
// value for a flag.
// This code is copied from the stdlib.
func isZeroValue(f *flag.Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	return value == z.Interface().(flag.Value).String()
}
