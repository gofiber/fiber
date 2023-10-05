//go:build darwin

package cpu

import (
	"context"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// sys/resource.h
const (
	CPUser    = 0
	CPNice    = 1
	CPSys     = 2
	CPIntr    = 3
	CPIdle    = 4
	CPUStates = 5
)

// default value. from time.h
var ClocksPerSec = float64(128)

func init() {
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
}

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	if percpu {
		return perCPUTimes()
	}

	return allCPUTimes()
}

// Returns only one CPUInfoStat on FreeBSD
func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	var ret []InfoStat

	c := InfoStat{}
	c.ModelName, _ = unix.Sysctl("machdep.cpu.brand_string")
	family, _ := unix.SysctlUint32("machdep.cpu.family")
	c.Family = strconv.FormatUint(uint64(family), 10)
	model, _ := unix.SysctlUint32("machdep.cpu.model")
	c.Model = strconv.FormatUint(uint64(model), 10)
	stepping, _ := unix.SysctlUint32("machdep.cpu.stepping")
	c.Stepping = int32(stepping)
	features, err := unix.Sysctl("machdep.cpu.features")
	if err == nil {
		for _, v := range strings.Fields(features) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	leaf7Features, err := unix.Sysctl("machdep.cpu.leaf7_features")
	if err == nil {
		for _, v := range strings.Fields(leaf7Features) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	extfeatures, err := unix.Sysctl("machdep.cpu.extfeatures")
	if err == nil {
		for _, v := range strings.Fields(extfeatures) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	cores, _ := unix.SysctlUint32("machdep.cpu.core_count")
	c.Cores = int32(cores)
	cacheSize, _ := unix.SysctlUint32("machdep.cpu.cache.size")
	c.CacheSize = int32(cacheSize)
	c.VendorID, _ = unix.Sysctl("machdep.cpu.vendor")

	// Use the rated frequency of the CPU. This is a static value and does not
	// account for low power or Turbo Boost modes.
	cpuFrequency, err := unix.SysctlUint64("hw.cpufrequency")
	if err != nil {
		return ret, err
	}
	c.Mhz = float64(cpuFrequency) / 1000000.0

	return append(ret, c), nil
}

func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	var cpuArgument string
	if logical {
		cpuArgument = "hw.logicalcpu"
	} else {
		cpuArgument = "hw.physicalcpu"
	}

	count, err := unix.SysctlUint32(cpuArgument)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}
