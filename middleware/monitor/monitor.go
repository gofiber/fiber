package monitor

import (
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/process"
)

type stats struct {
	Cpu   float64 `json:"cpu"`
	Ram   uint64  `json:"ram"`
	Rtime int64   `json:"rtime"`
	Conns uint32  `json:"conns"`
}

var (
	monitorCPU float64
	monitorRAM uint64
	mutex      sync.RWMutex
	once       sync.Once
	data       = &stats{}
)

// New creates a new middleware handler
func New() fiber.Handler {
	// Start routine to update statistics
	once.Do(func() {
		go func() {
			p, _ := process.NewProcess(int32(os.Getpid()))
			for {
				cpu, _ := p.CPUPercent()
				monitorCPU = cpu / 10

				mem, _ := p.MemoryInfo()
				monitorRAM = mem.RSS

				time.Sleep(1 * time.Second)
			}
		}()
	})

	// Return new handler
	return func(c *fiber.Ctx) error {
		if c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON {
			mutex.Lock()
			data.Cpu = monitorCPU
			data.Ram = monitorRAM
			data.Rtime = (time.Now().UnixNano() - c.Context().Time().UnixNano()) / 1000000
			data.Conns = c.App().Server().GetCurrentConcurrency()
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Response().Header.SetContentType(fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(index)
	}
}
