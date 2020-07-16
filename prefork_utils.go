// +build !windows

package fiber

import (
	"os"
	"os/exec"
	"time"
)

var (
	dummyChildCmd = "go"
)

func dummyCmd() *exec.Cmd {
	return exec.Command(dummyChildCmd, "version")
}

// watchMaster gets ppid regularly,
// if it is equal to 1 (init process ID),
// it indicates that the master process has exited
func watchMaster() {
	for range time.NewTicker(time.Millisecond * 500).C {
		if os.Getppid() == 1 {
			os.Exit(1)
		}
	}
}
