package fiber

import (
	"os"
	"os/exec"
)

var dummyChildCmd = "go"

func dummyCmd() *exec.Cmd {
	return exec.Command("cmd", "/C", dummyChildCmd, "version")
}

// watchMaster finds parent process,
// and waits for it to exit
func watchMaster() {
	p, err := os.FindProcess(os.Getppid())
	if err == nil {
		_, _ = p.Wait()
	}
	os.Exit(1)
}
