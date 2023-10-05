//go:build windows

package process

import (
	"syscall"
	"unsafe"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

type PROCESS_MEMORY_COUNTERS struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uint64
	WorkingSetSize             uint64
	QuotaPeakPagedPoolUsage    uint64
	QuotaPagedPoolUsage        uint64
	QuotaPeakNonPagedPoolUsage uint64
	QuotaNonPagedPoolUsage     uint64
	PagefileUsage              uint64
	PeakPagefileUsage          uint64
}

func queryPebAddress(procHandle syscall.Handle, is32BitProcess bool) uint64 {
	if is32BitProcess {
		// we are on a 64-bit process reading an external 32-bit process
		var wow64 uint

		ret, _, _ := common.ProcNtQueryInformationProcess.Call(
			uintptr(procHandle),
			uintptr(common.ProcessWow64Information),
			uintptr(unsafe.Pointer(&wow64)),
			uintptr(unsafe.Sizeof(wow64)),
			uintptr(0),
		)
		if int(ret) >= 0 {
			return uint64(wow64)
		}
	} else {
		// we are on a 64-bit process reading an external 64-bit process
		var info processBasicInformation64

		ret, _, _ := common.ProcNtQueryInformationProcess.Call(
			uintptr(procHandle),
			uintptr(common.ProcessBasicInformation),
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Sizeof(info)),
			uintptr(0),
		)
		if int(ret) >= 0 {
			return info.PebBaseAddress
		}
	}

	// return 0 on error
	return 0
}

func readProcessMemory(procHandle syscall.Handle, _ bool, address uint64, size uint) []byte {
	var read uint

	buffer := make([]byte, size)

	ret, _, _ := common.ProcNtReadVirtualMemory.Call(
		uintptr(procHandle),
		uintptr(address),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&read)),
	)
	if int(ret) >= 0 && read > 0 {
		return buffer[:read]
	}
	return nil
}
