//go:build freebsd
// +build freebsd

package load

func getForkStat() (forkstat, error) {
	return forkstat{}, nil
}
