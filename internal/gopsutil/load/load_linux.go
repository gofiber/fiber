//go:build linux

package load

import (
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

func Avg() (*AvgStat, error) {
	return AvgWithContext(context.Background())
}

func AvgWithContext(ctx context.Context) (*AvgStat, error) {
	stat, err := fileAvgWithContext(ctx)
	if err != nil {
		stat, err = sysinfoAvgWithContext(ctx)
	}
	return stat, err
}

func sysinfoAvgWithContext(ctx context.Context) (*AvgStat, error) {
	var info syscall.Sysinfo_t
	err := syscall.Sysinfo(&info)
	if err != nil {
		return nil, err
	}

	const si_load_shift = 16
	return &AvgStat{
		Load1:  float64(info.Loads[0]) / float64(1<<si_load_shift),
		Load5:  float64(info.Loads[1]) / float64(1<<si_load_shift),
		Load15: float64(info.Loads[2]) / float64(1<<si_load_shift),
	}, nil
}

func fileAvgWithContext(ctx context.Context) (*AvgStat, error) {
	values, err := readLoadAvgFromFile()
	if err != nil {
		return nil, err
	}

	load1, err := strconv.ParseFloat(values[0], 64)
	if err != nil {
		return nil, err
	}
	load5, err := strconv.ParseFloat(values[1], 64)
	if err != nil {
		return nil, err
	}
	load15, err := strconv.ParseFloat(values[2], 64)
	if err != nil {
		return nil, err
	}

	ret := &AvgStat{
		Load1:  load1,
		Load5:  load5,
		Load15: load15,
	}

	return ret, nil
}

// Misc returnes miscellaneous host-wide statistics.
// Note: the name should be changed near future.
func Misc() (*MiscStat, error) {
	return MiscWithContext(context.Background())
}

func MiscWithContext(ctx context.Context) (*MiscStat, error) {
	filename := common.HostProc("stat")
	out, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	ret := &MiscStat{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "processes":
			ret.ProcsCreated = v
		case "procs_running":
			ret.ProcsRunning = v
		case "procs_blocked":
			ret.ProcsBlocked = v
		case "ctxt":
			ret.Ctxt = v
		default:
			continue
		}

	}

	procsTotal, err := getProcsTotal()
	if err != nil {
		return ret, err
	}
	ret.ProcsTotal = procsTotal

	return ret, nil
}

func getProcsTotal() (int64, error) {
	values, err := readLoadAvgFromFile()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.Split(values[3], "/")[1], 10, 64)
}

func readLoadAvgFromFile() ([]string, error) {
	loadavgFilename := common.HostProc("loadavg")
	line, err := os.ReadFile(loadavgFilename)
	if err != nil {
		return nil, err
	}

	values := strings.Fields(string(line))
	return values, nil
}
