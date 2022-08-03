//go:build freebsd || openbsd
// +build freebsd openbsd

package load

import (
	"context"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
	"golang.org/x/sys/unix"
)

var invoke common.Invoker = common.Invoke{}

func Avg() (*AvgStat, error) {
	return AvgWithContext(context.Background())
}

func AvgWithContext(ctx context.Context) (*AvgStat, error) {
	// This SysctlRaw method borrowed from
	// https://github.com/prometheus/node_exporter/blob/master/collector/loadavg_freebsd.go
	type loadavg struct {
		load  [3]uint32
		scale int
	}
	b, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return nil, err
	}
	load := *(*loadavg)(unsafe.Pointer((&b[0])))
	scale := float64(load.scale)
	ret := &AvgStat{
		Load1:  float64(load.load[0]) / scale,
		Load5:  float64(load.load[1]) / scale,
		Load15: float64(load.load[2]) / scale,
	}

	return ret, nil
}

type forkstat struct {
	forks    int
	vforks   int
	__tforks int
}

// Misc returns miscellaneous host-wide statistics.
// darwin use ps command to get process running/blocked count.
// Almost same as Darwin implementation, but state is different.
func Misc() (*MiscStat, error) {
	return MiscWithContext(context.Background())
}

func MiscWithContext(ctx context.Context) (*MiscStat, error) {
	bin, err := exec.LookPath("ps")
	if err != nil {
		return nil, err
	}
	out, err := invoke.CommandWithContext(ctx, bin, "axo", "state")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")

	ret := MiscStat{}
	for _, l := range lines {
		if strings.Contains(l, "R") {
			ret.ProcsRunning++
		} else if strings.Contains(l, "D") {
			ret.ProcsBlocked++
		}
	}

	f, err := getForkStat()
	if err != nil {
		return nil, err
	}
	ret.ProcsCreated = int64(f.forks)

	return &ret, nil
}
