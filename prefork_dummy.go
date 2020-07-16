// +build !windows

package fiber

import (
	"os/exec"
)

var dummyChildCmd = "go"

func dummyCmd() *exec.Cmd {
	return exec.Command(dummyChildCmd, "version")
}
