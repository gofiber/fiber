//go:build linux

package mem

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

type VirtualMemoryExStat struct {
	ActiveFile   uint64 `json:"activefile"`
	InactiveFile uint64 `json:"inactivefile"`
	ActiveAnon   uint64 `json:"activeanon"`
	InactiveAnon uint64 `json:"inactiveanon"`
	Unevictable  uint64 `json:"unevictable"`
}

func (v VirtualMemoryExStat) String() string {
	s, _ := json.Marshal(v)
	return string(s)
}

func VirtualMemory() (*VirtualMemoryStat, error) {
	return VirtualMemoryWithContext(context.Background())
}

func VirtualMemoryWithContext(ctx context.Context) (*VirtualMemoryStat, error) {
	vm, _, err := fillFromMeminfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

func VirtualMemoryEx() (*VirtualMemoryExStat, error) {
	return VirtualMemoryExWithContext(context.Background())
}

func VirtualMemoryExWithContext(ctx context.Context) (*VirtualMemoryExStat, error) {
	_, vmEx, err := fillFromMeminfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return vmEx, nil
}

func fillFromMeminfoWithContext(ctx context.Context) (*VirtualMemoryStat, *VirtualMemoryExStat, error) {
	filename := common.HostProc("meminfo")
	lines, _ := common.ReadLines(filename)

	// flag if MemAvailable is in /proc/meminfo (kernel 3.14+)
	memavail := false
	activeFile := false   // "Active(file)" not available: 2.6.28 / Dec 2008
	inactiveFile := false // "Inactive(file)" not available: 2.6.28 / Dec 2008
	sReclaimable := false // "SReclaimable:" not available: 2.6.19 / Nov 2006

	ret := &VirtualMemoryStat{}
	retEx := &VirtualMemoryExStat{}

	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		value = strings.Replace(value, " kB", "", -1)

		t, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return ret, retEx, err
		}
		switch key {
		case "MemTotal":
			ret.Total = t * 1024
		case "MemFree":
			ret.Free = t * 1024
		case "MemAvailable":
			memavail = true
			ret.Available = t * 1024
		case "Buffers":
			ret.Buffers = t * 1024
		case "Cached":
			ret.Cached = t * 1024
		case "Active":
			ret.Active = t * 1024
		case "Inactive":
			ret.Inactive = t * 1024
		case "Active(anon)":
			retEx.ActiveAnon = t * 1024
		case "Inactive(anon)":
			retEx.InactiveAnon = t * 1024
		case "Active(file)":
			activeFile = true
			retEx.ActiveFile = t * 1024
		case "Inactive(file)":
			inactiveFile = true
			retEx.InactiveFile = t * 1024
		case "Unevictable":
			retEx.Unevictable = t * 1024
		case "Writeback":
			ret.Writeback = t * 1024
		case "WritebackTmp":
			ret.WritebackTmp = t * 1024
		case "Dirty":
			ret.Dirty = t * 1024
		case "Shmem":
			ret.Shared = t * 1024
		case "Slab":
			ret.Slab = t * 1024
		case "SReclaimable":
			sReclaimable = true
			ret.SReclaimable = t * 1024
		case "SUnreclaim":
			ret.SUnreclaim = t * 1024
		case "PageTables":
			ret.PageTables = t * 1024
		case "SwapCached":
			ret.SwapCached = t * 1024
		case "CommitLimit":
			ret.CommitLimit = t * 1024
		case "Committed_AS":
			ret.CommittedAS = t * 1024
		case "HighTotal":
			ret.HighTotal = t * 1024
		case "HighFree":
			ret.HighFree = t * 1024
		case "LowTotal":
			ret.LowTotal = t * 1024
		case "LowFree":
			ret.LowFree = t * 1024
		case "SwapTotal":
			ret.SwapTotal = t * 1024
		case "SwapFree":
			ret.SwapFree = t * 1024
		case "Mapped":
			ret.Mapped = t * 1024
		case "VmallocTotal":
			ret.VMallocTotal = t * 1024
		case "VmallocUsed":
			ret.VMallocUsed = t * 1024
		case "VmallocChunk":
			ret.VMallocChunk = t * 1024
		case "HugePages_Total":
			ret.HugePagesTotal = t
		case "HugePages_Free":
			ret.HugePagesFree = t
		case "Hugepagesize":
			ret.HugePageSize = t * 1024
		}
	}

	ret.Cached += ret.SReclaimable

	if !memavail {
		if activeFile && inactiveFile && sReclaimable {
			ret.Available = calcuateAvailVmem(ret, retEx)
		} else {
			ret.Available = ret.Cached + ret.Free
		}
	}

	ret.Used = ret.Total - ret.Free - ret.Buffers - ret.Cached
	ret.UsedPercent = float64(ret.Used) / float64(ret.Total) * 100.0

	return ret, retEx, nil
}

func SwapMemory() (*SwapMemoryStat, error) {
	return SwapMemoryWithContext(context.Background())
}

func SwapMemoryWithContext(ctx context.Context) (*SwapMemoryStat, error) {
	sysinfo := &unix.Sysinfo_t{}

	if err := unix.Sysinfo(sysinfo); err != nil {
		return nil, err
	}
	ret := &SwapMemoryStat{
		Total: uint64(sysinfo.Totalswap) * uint64(sysinfo.Unit),
		Free:  uint64(sysinfo.Freeswap) * uint64(sysinfo.Unit),
	}
	ret.Used = ret.Total - ret.Free
	// check Infinity
	if ret.Total != 0 {
		ret.UsedPercent = float64(ret.Total-ret.Free) / float64(ret.Total) * 100.0
	} else {
		ret.UsedPercent = 0
	}
	filename := common.HostProc("vmstat")
	lines, _ := common.ReadLines(filename)
	for _, l := range lines {
		fields := strings.Fields(l)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "pswpin":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.Sin = value * 4 * 1024
		case "pswpout":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.Sout = value * 4 * 1024
		case "pgpgin":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.PgIn = value * 4 * 1024
		case "pgpgout":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.PgOut = value * 4 * 1024
		case "pgfault":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.PgFault = value * 4 * 1024
		case "pgmajfault":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			ret.PgMajFault = value * 4 * 1024
		}
	}
	return ret, nil
}

// calcuateAvailVmem is a fallback under kernel 3.14 where /proc/meminfo does not provide
// "MemAvailable:" column. It reimplements an algorithm from the link below
// https://github.com/giampaolo/psutil/pull/890
func calcuateAvailVmem(ret *VirtualMemoryStat, retEx *VirtualMemoryExStat) uint64 {
	var watermarkLow uint64

	fn := common.HostProc("zoneinfo")
	lines, err := common.ReadLines(fn)
	if err != nil {
		return ret.Free + ret.Cached // fallback under kernel 2.6.13
	}

	pagesize := uint64(os.Getpagesize())
	watermarkLow = 0

	for _, line := range lines {
		fields := strings.Fields(line)

		if strings.HasPrefix(fields[0], "low") {
			lowValue, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				lowValue = 0
			}
			watermarkLow += lowValue
		}
	}

	watermarkLow *= pagesize

	availMemory := ret.Free - watermarkLow
	pageCache := retEx.ActiveFile + retEx.InactiveFile
	pageCache -= uint64(math.Min(float64(pageCache/2), float64(watermarkLow)))
	availMemory += pageCache
	availMemory += ret.SReclaimable - uint64(math.Min(float64(ret.SReclaimable/2.0), float64(watermarkLow)))

	return availMemory
}
