//go:build openbsd

package cpu

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

// sys/sched.h
var (
	CPUser    = 0
	CPNice    = 1
	CPSys     = 2
	CPIntr    = 3
	CPIdle    = 4
	CPUStates = 5
)

// sys/sysctl.h
const (
	CTLKern     = 1  // "high kernel": proc, limits
	CTLHw       = 6  // CTL_HW
	SMT         = 24 // HW_SMT
	KernCptime  = 40 // KERN_CPTIME
	KernCptime2 = 71 // KERN_CPTIME2
)

var ClocksPerSec = float64(128)

func init() {
	func() {
		getconf, err := exec.LookPath("getconf")
		if err != nil {
			return
		}
		out, err := invoke.Command(getconf, "CLK_TCK")
		// ignore errors
		if err == nil {
			i, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
			if err == nil {
				ClocksPerSec = float64(i)
			}
		}
	}()
	func() {
		v, err := unix.Sysctl("kern.osrelease") // can't reuse host.PlatformInformation because of circular import
		if err != nil {
			return
		}
		v = strings.ToLower(v)
		version, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
		if version >= 6.4 {
			CPIntr = 4
			CPIdle = 5
			CPUStates = 6
		}
	}()
}

func smt() (bool, error) {
	mib := []int32{CTLHw, SMT}
	buf, _, err := common.CallSyscall(mib)
	if err != nil {
		return false, err
	}

	var ret bool
	br := bytes.NewReader(buf)
	if err := binary.Read(br, binary.LittleEndian, &ret); err != nil {
		return false, err
	}

	return ret, nil
}

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	var ret []TimesStat

	var ncpu int
	if percpu {
		ncpu, _ = Counts(true)
	} else {
		ncpu = 1
	}

	smt, err := smt()
	if err == syscall.EOPNOTSUPP {
		// if hw.smt is not applicable for this platform (e.g. i386),
		// pretend it's enabled
		smt = true
	} else if err != nil {
		return nil, err
	}

	for i := 0; i < ncpu; i++ {
		j := i
		if !smt {
			j *= 2
		}

		cpuTimes := make([]int32, CPUStates)
		var mib []int32
		if percpu {
			mib = []int32{CTLKern, KernCptime2, int32(j)}
		} else {
			mib = []int32{CTLKern, KernCptime}
		}
		buf, _, err := common.CallSyscall(mib)
		if err != nil {
			return ret, err
		}

		br := bytes.NewReader(buf)
		err = binary.Read(br, binary.LittleEndian, &cpuTimes)
		if err != nil {
			return ret, err
		}
		c := TimesStat{
			User:   float64(cpuTimes[CPUser]) / ClocksPerSec,
			Nice:   float64(cpuTimes[CPNice]) / ClocksPerSec,
			System: float64(cpuTimes[CPSys]) / ClocksPerSec,
			Idle:   float64(cpuTimes[CPIdle]) / ClocksPerSec,
			Irq:    float64(cpuTimes[CPIntr]) / ClocksPerSec,
		}
		if percpu {
			c.CPU = fmt.Sprintf("cpu%d", j)
		} else {
			c.CPU = "cpu-total"
		}
		ret = append(ret, c)
	}

	return ret, nil
}

// Returns only one (minimal) CPUInfoStat on OpenBSD
func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	var ret []InfoStat
	var err error

	c := InfoStat{}

	mhz, err := unix.SysctlUint32("hw.cpuspeed")
	if err != nil {
		return nil, err
	}
	c.Mhz = float64(mhz)

	ncpu, err := unix.SysctlUint32("hw.ncpuonline")
	if err != nil {
		return nil, err
	}
	c.Cores = int32(ncpu)

	if c.ModelName, err = unix.Sysctl("hw.model"); err != nil {
		return nil, err
	}

	return append(ret, c), nil
}

func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	return runtime.NumCPU(), nil
}
