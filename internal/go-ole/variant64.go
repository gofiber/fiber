//go:build amd64 || arm64 || ppc64le || s390x
// +build amd64 arm64 ppc64le s390x

package ole

type VARIANT struct {
	VT         VT      //  2
	wReserved1 uint16  //  4
	wReserved2 uint16  //  6
	wReserved3 uint16  //  8
	Val        int64   // 16
	_          [8]byte // 24
}
