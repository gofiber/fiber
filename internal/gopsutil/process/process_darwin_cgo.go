//go:build darwin && cgo

package process

// #include <stdlib.h>
// #include <libproc.h>
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

func (p *Process) ExeWithContext(ctx context.Context) (string, error) {
	var c C.char // need a var for unsafe.Sizeof need a var
	const bufsize = C.PROC_PIDPATHINFO_MAXSIZE * unsafe.Sizeof(c)
	buffer := (*C.char)(C.malloc(C.size_t(bufsize)))
	defer C.free(unsafe.Pointer(buffer))

	ret, err := C.proc_pidpath(C.int(p.Pid), unsafe.Pointer(buffer), C.uint32_t(bufsize))
	if err != nil {
		return "", err
	}
	if ret <= 0 {
		return "", fmt.Errorf("unknown error: proc_pidpath returned %d", ret)
	}

	return C.GoString(buffer), nil
}
