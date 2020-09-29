package monitor

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
	"github.com/gofiber/fiber/v2/internal/gopsutil/mem"
	"github.com/gofiber/fiber/v2/internal/gopsutil/process"
)

type stats struct {
	PID   statsPID `json:"pid"`
	OS    statsOS  `json:"os"`
	Conns uint32   `json:"conns"`
}

type statsPID struct {
	CPU float64 `json:"cpu"`
	RAM uint64  `json:"ram"`
}
type statsOS struct {
	CPU float64 `json:"cpu"`
	RAM uint64  `json:"ram"`
}

var (
	monitPidCpu atomic.Value
	monitPidRam atomic.Value

	monitOsCpu atomic.Value
	monitOsRam atomic.Value
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
		p, _ := process.NewProcess(int32(os.Getpid()))
		updateStatistics(p)

		go func() {
			for {
				updateStatistics(p)

				time.Sleep(1 * time.Second)
			}
		}()
	})

	// Return new handler
	return func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
			return fiber.ErrMethodNotAllowed
		}
		if c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON {
			mutex.Lock()
			data.PID.CPU = monitPidCpu.Load().(float64)
			data.PID.RAM = monitPidRam.Load().(uint64)
			data.OS.CPU = monitOsCpu.Load().(float64)
			data.OS.RAM = monitOsRam.Load().(uint64)
			data.Conns = c.App().Server().GetCurrentConcurrency()
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Response().Header.SetContentType(fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(index)
	}
}

func updateStatistics(p *process.Process) {
	pidCpu, _ := p.CPUPercent()
	monitPidCpu.Store(pidCpu / 10)

	osCpu, _ := cpu.Percent(0, false)
	monitOsCpu.Store(osCpu[0])

	pidMem, _ := p.MemoryInfo()
	monitPidRam.Store(pidMem.RSS)

	osMem, _ := mem.VirtualMemory()
	monitOsRam.Store(osMem.Used)
}
