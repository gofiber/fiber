// +build darwin

package mem

import (
	"context"
	"encoding/binary"
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

func getHwMemsize() (uint64, error) {
	totalString, err := unix.Sysctl("hw.memsize")
	if err != nil {
		return 0, err
	}

	// unix.sysctl() helpfully assumes the result is a null-terminated string and
	// removes the last byte of the result if it's 0 :/
	totalString += "\x00"

	total := uint64(binary.LittleEndian.Uint64([]byte(totalString)))

	return total, nil
}

// xsw_usage in sys/sysctl.h
type swapUsage struct {
	Total     uint64
	Avail     uint64
	Used      uint64
	Pagesize  int32
	Encrypted bool
}

// SwapMemory returns swapinfo.
func SwapMemory() (*SwapMemoryStat, error) {
	return SwapMemoryWithContext(context.Background())
}

func SwapMemoryWithContext(ctx context.Context) (*SwapMemoryStat, error) {
	// https://github.com/yanllearnn/go-osstat/blob/ae8a279d26f52ec946a03698c7f50a26cfb427e3/memory/memory_darwin.go
	var ret *SwapMemoryStat

	value, err := unix.SysctlRaw("vm.swapusage")
	if err != nil {
		return ret, err
	}
	if len(value) != 32 {
		return ret, fmt.Errorf("unexpected output of sysctl vm.swapusage: %v (len: %d)", value, len(value))
	}
	swap := (*swapUsage)(unsafe.Pointer(&value[0]))

	u := float64(0)
	if swap.Total != 0 {
		u = ((float64(swap.Total) - float64(swap.Avail)) / float64(swap.Total)) * 100.0
	}

	ret = &SwapMemoryStat{
		Total:       swap.Total,
		Used:        swap.Used,
		Free:        swap.Avail,
		UsedPercent: u,
	}

	return ret, nil
}
