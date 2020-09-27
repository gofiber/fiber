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
	Time int     `json:"time"`
	Reqs int     `json:"reqs"`
}

var (
	mutex sync.RWMutex
	once  sync.Once
	data  = &stats{}
)

// New creates a new middleware handler
func New() fiber.Handler {
	// Start routine to update statistics
	once.Do(func() {
		go monitor()
	})

	// Return new handler
	return func(c *fiber.Ctx) error {
		if c.Get(fiber.HeaderAccept) == fiber.MIMEApplicationJSON {
			return c.JSON(data)
		}
		c.Response().Header.SetContentType(fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(index)
	}
}

func monitor() {
	p, _ := process.NewProcess(int32(os.Getpid()))

	for {
		time.Sleep(1 * time.Second)
		// *magic*
		mutex.Lock()

		cpu, _ := p.CPUPercent()
		//fmt.Println(fmt.Sprintf("CPU:  %.1f%%", cpu/10))

		mem, _ := p.MemoryInfo()
		//fmt.Println("RAM: ", utils.ByteSize(mem.RSS))

		data = &stats{
			CPU:  cpu / 10,
			RAM:  mem.RSS,
			Load: 2.32,
			Time: 234,
			Reqs: 23,
		}
		mutex.Unlock()
		// *magic*
	}
}
