// +build !darwin,!linux,!freebsd,!openbsd,!windows,!solaris

package load

import (
	"context"

	"github.com/shirou/gopsutil/internal/common"
)

func Avg() (*AvgStat, error) {
	return AvgWithContext(context.Background())
}

func AvgWithContext(ctx context.Context) (*AvgStat, error) {
	return nil, common.ErrNotImplementedError
}

func Misc() (*MiscStat, error) {
	return MiscWithContext(context.Background())
}

func MiscWithContext(ctx context.Context) (*MiscStat, error) {
	return nil, common.ErrNotImplementedError
}
