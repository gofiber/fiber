package monitor

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/gopsutil/cpu"
	"github.com/gofiber/fiber/v2/internal/gopsutil/load"
	"github.com/gofiber/fiber/v2/internal/gopsutil/mem"
	"github.com/gofiber/fiber/v2/internal/gopsutil/net"
	"github.com/gofiber/fiber/v2/internal/gopsutil/process"
)

type stats struct {
	PID statsPID `json:"pid"`
	OS  statsOS  `json:"os"`
}

type statsPID struct {
	CPU   float64 `json:"cpu"`
	RAM   uint64  `json:"ram"`
	Conns int     `json:"conns"`
}

type statsOS struct {
	CPU      float64 `json:"cpu"`
	RAM      uint64  `json:"ram"`
	TotalRAM uint64  `json:"total_ram"`
	LoadAvg  float64 `json:"load_avg"`
	Conns    int     `json:"conns"`
}

var (
	monitPIDCPU   atomic.Value
	monitPIDRAM   atomic.Value
	monitPIDConns atomic.Value

	monitOSCPU      atomic.Value
	monitOSRAM      atomic.Value
	monitOSTotalRAM atomic.Value
	monitOSLoadAvg  atomic.Value
	monitOSConns    atomic.Value
)

var (
	mutex sync.RWMutex
	once  sync.Once
	data  = &stats{}
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Start routine to update statistics
	once.Do(func() {
		p, _ := process.NewProcess(int32(os.Getpid())) //nolint:errcheck // TODO: Handle error

		updateStatistics(p)

		go func() {
			for {
				time.Sleep(cfg.Refresh)

				updateStatistics(p)
			}
		}()
	})

	// Return new handler
	//nolint:errcheck // Ignore the type-assertion errors
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return fiber.ErrMethodNotAllowed
		}
		if c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON || cfg.APIOnly {
			mutex.Lock()
			data.PID.CPU, _ = monitPIDCPU.Load().(float64)
			data.PID.RAM, _ = monitPIDRAM.Load().(uint64)
			data.PID.Conns, _ = monitPIDConns.Load().(int)

			data.OS.CPU, _ = monitOSCPU.Load().(float64)
			data.OS.RAM, _ = monitOSRAM.Load().(uint64)
			data.OS.TotalRAM, _ = monitOSTotalRAM.Load().(uint64)
			data.OS.LoadAvg, _ = monitOSLoadAvg.Load().(float64)
			data.OS.Conns, _ = monitOSConns.Load().(int)
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).SendString(cfg.index)
	}
}

func updateStatistics(p *process.Process) {
	pidCPU, err := p.CPUPercent()
	if err == nil {
		monitPIDCPU.Store(pidCPU / 10)
	}

	if osCPU, err := cpu.Percent(0, false); err == nil && len(osCPU) > 0 {
		monitOSCPU.Store(osCPU[0])
	}

	if pidRAM, err := p.MemoryInfo(); err == nil && pidRAM != nil {
		monitPIDRAM.Store(pidRAM.RSS)
	}

	if osRAM, err := mem.VirtualMemory(); err == nil && osRAM != nil {
		monitOSRAM.Store(osRAM.Used)
		monitOSTotalRAM.Store(osRAM.Total)
	}

	if loadAvg, err := load.Avg(); err == nil && loadAvg != nil {
		monitOSLoadAvg.Store(loadAvg.Load1)
	}

	pidConns, err := net.ConnectionsPid("tcp", p.Pid)
	if err == nil {
		monitPIDConns.Store(len(pidConns))
	}

	osConns, err := net.Connections("tcp")
	if err == nil {
		monitOSConns.Store(len(osConns))
	}
}
