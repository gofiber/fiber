package monitor

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
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
	monitPidCpu   atomic.Value
	monitPidRam   atomic.Value
	monitPidConns atomic.Value

	monitOsCpu      atomic.Value
	monitOsRam      atomic.Value
	monitOsTotalRam atomic.Value
	monitOsLoadAvg  atomic.Value
	monitOsConns    atomic.Value
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
		p, _ := process.NewProcess(int32(os.Getpid()))

		updateStatistics(p)

		go func() {
			for {
				time.Sleep(cfg.Refresh)

				updateStatistics(p)
			}
		}()
	})

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if c.Method() != fiber.MethodGet {
			return fiber.ErrMethodNotAllowed
		}
		if c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON || cfg.APIOnly {
			mutex.Lock()
			data.PID.CPU = monitPidCpu.Load().(float64)
			data.PID.RAM = monitPidRam.Load().(uint64)
			data.PID.Conns = monitPidConns.Load().(int)

			data.OS.CPU = monitOsCpu.Load().(float64)
			data.OS.RAM = monitOsRam.Load().(uint64)
			data.OS.TotalRAM = monitOsTotalRam.Load().(uint64)
			data.OS.LoadAvg = monitOsLoadAvg.Load().(float64)
			data.OS.Conns = monitOsConns.Load().(int)
			mutex.Unlock()
			return c.Status(fiber.StatusOK).JSON(data)
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Status(fiber.StatusOK).SendString(cfg.index)
	}
}

func updateStatistics(p *process.Process) {
	pidCpu, _ := p.CPUPercent()
	monitPidCpu.Store(pidCpu / 10)

	if osCpu, _ := cpu.Percent(0, false); len(osCpu) > 0 {
		monitOsCpu.Store(osCpu[0])
	}

	if pidMem, _ := p.MemoryInfo(); pidMem != nil {
		monitPidRam.Store(pidMem.RSS)
	}

	if osMem, _ := mem.VirtualMemory(); osMem != nil {
		monitOsRam.Store(osMem.Used)
		monitOsTotalRam.Store(osMem.Total)
	}

	if loadAvg, _ := load.Avg(); loadAvg != nil {
		monitOsLoadAvg.Store(loadAvg.Load1)
	}

	pidConns, _ := net.ConnectionsPid("tcp", p.Pid)
	monitPidConns.Store(len(pidConns))

	osConns, _ := net.Connections("tcp")
	monitOsConns.Store(len(osConns))
}
