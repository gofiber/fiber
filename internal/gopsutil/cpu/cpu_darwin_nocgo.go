//go:build darwin && !cgo
// +build darwin,!cgo

package cpu

import "github.com/gofiber/fiber/v3/internal/gopsutil/common"

func perCPUTimes() ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}

func allCPUTimes() ([]TimesStat, error) {
	return []TimesStat{}, common.ErrNotImplementedError
}
