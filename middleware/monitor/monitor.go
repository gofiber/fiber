package monitor

import (
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
	"github.com/gofiber/fiber/v2/internal/gopsutil/mem"
	"github.com/gofiber/fiber/v2/internal/gopsutil/process"
)

type stats struct {
	PID   statsPID `json:"pid"`
	OS    statsOS  `json:"os"`
	Rtime int64    `json:"rtime"`
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
	monitPidCpu float64
	monitPidRam uint64

	monitOsCpu float64
	monitOsRam uint64
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
				pidCpu, _ := p.CPUPercent()
				monitPidCpu = pidCpu / 10

				osCpu, _ := cpu.Percent(0, false)
				monitOsCpu = osCpu[0]

				pidMem, _ := p.MemoryInfo()
				monitPidRam = pidMem.RSS

				osMem, _ := mem.VirtualMemory()
				monitOsRam = osMem.Used

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
			data.PID.CPU = monitPidCpu
			data.PID.RAM = monitPidRam
			data.OS.CPU = monitOsCpu
			data.OS.RAM = monitOsRam
			data.Rtime = (time.Now().UnixNano() - c.Context().Time().UnixNano()) / 1000000
			data.Conns = c.App().Server().GetCurrentConcurrency()
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Response().Header.SetContentType(fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).Send(index)
	}
}
