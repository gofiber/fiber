//go:build freebsd

package load

func getForkStat() (forkstat, error) {
	return forkstat{}, nil
}
