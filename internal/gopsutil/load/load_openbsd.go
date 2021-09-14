// +build openbsd

package load

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func getForkStat() (forkstat, error) {
	b, err := unix.SysctlRaw("kern.forkstat")
	if err != nil {
		return forkstat{}, err
	}
	return *(*forkstat)(unsafe.Pointer((&b[0]))), nil
}
