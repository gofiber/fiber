//go:build s390x || ppc64 || ppc64le

package fiber

import "unsafe"

func isReadOnly(_ unsafe.Pointer) bool {
	return false
}
