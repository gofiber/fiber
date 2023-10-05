//go:build ignore

/*
Input to cgo -godefs.
*/

package mem

/*
#include <sys/types.h>
#include <sys/mount.h>
#include <sys/sysctl.h>
*/
import "C"

// Machine characteristics; for internal use.

const (
	CTLVfs        = 10
	VfsGeneric    = 0
	VfsBcacheStat = 3
)

const (
	sizeOfBcachestats = C.sizeof_struct_bcachestats
)

type Bcachestats C.struct_bcachestats
