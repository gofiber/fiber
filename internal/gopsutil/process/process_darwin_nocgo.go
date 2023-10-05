//go:build darwin && !cgo

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func (p *Process) ExeWithContext(ctx context.Context) (string, error) {
	lsof_bin, err := exec.LookPath("lsof")
	if err != nil {
		return "", err
	}
	out, err := invoke.CommandWithContext(ctx, lsof_bin, "-p", strconv.Itoa(int(p.Pid)), "-Fpfn")
	if err != nil {
		return "", fmt.Errorf("bad call to lsof: %s", err)
	}
	txtFound := 0
	lines := strings.Split(string(out), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "ftxt" {
			txtFound++
			if txtFound == 2 {
				return lines[i-1][1:], nil
			}
		}
	}
	return "", fmt.Errorf("missing txt data returned by lsof")
}
