package fiber

import (
	"os"
	"os/exec"
)

const windowsOS = "windows"

// Internal seams used by tests and integration scenarios.
// They default to nil and do not affect normal runtime behavior.
var (
	preforkHookPIDOverride func(pid int) int
	preforkCommandProducer func(files []*os.File) (*exec.Cmd, error)
)
