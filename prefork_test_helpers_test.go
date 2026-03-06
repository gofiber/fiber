package fiber

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func resetPreforkSeams() {
	preforkHookPIDOverride = nil
	preforkCommandProducer = nil
}

func setPreforkDummyCommand(command string) {
	preforkCommandProducer = func(files []*os.File) (*exec.Cmd, error) {
		cmd := exec.Command(command, "version")
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", command, "version")
		}
		cmd.ExtraFiles = files
		if err := cmd.Start(); err != nil {
			return cmd, fmt.Errorf("failed to start dummy prefork command: %w", err)
		}
		return cmd, nil
	}
}

func usePreforkDummyCommand(t *testing.T, command string) {
	t.Helper()
	resetPreforkSeams()
	setPreforkDummyCommand(command)
	t.Cleanup(resetPreforkSeams)
}

func usePreforkHookPIDOverride(t *testing.T, pid int) {
	t.Helper()
	preforkHookPIDOverride = func(int) int { return pid }
	t.Cleanup(func() {
		preforkHookPIDOverride = nil
	})
}
