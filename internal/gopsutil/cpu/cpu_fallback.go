//go:build !darwin && !linux && !freebsd && !openbsd && !solaris && !windows && !dragonfly
// +build !darwin,!linux,!freebsd,!openbsd,!solaris,!windows,!dragonfly

package cpu

import (
	"context"
	"runtime"

	"github.com/gofiber/fiber/v2/internal/gopsutil/common"
)

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}

func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	return []InfoStat{}, common.ErrNotImplementedError
}

func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	return runtime.NumCPU(), nil
}
