package monitor

import (
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/process"
)

type stats struct {
	CPU  float64 `json:"cpu"`
	RAM  uint64  `json:"ram"`
	Load float64 `json:"load"`
	Time int64   `json:"time"`
	Reqs uint32  `json:"reqs"`
}

var (
	monitorCPU float64
	monitorRAM uint64
)

var (
	mutex sync.RWMutex
	once  sync.Once
	data  = &stats{}
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
			data.CPU = monitorCPU
			data.RAM = monitorRAM
			data.Time = (time.Now().UnixNano() - c.Context().Time().UnixNano()) / 1000000
			data.Reqs = c.App().Server().GetCurrentConcurrency()
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Response().Header.SetContentType(fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(index)
	}
}
