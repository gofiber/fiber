package monitor

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type stats struct {
	CPU  float64 `json:"cpu"`
	RAM  float64 `json:"ram"`
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
	for {
		time.Sleep(1 * time.Second)
		// *magic*
		mutex.Lock()
		data = &stats{
			CPU:  12.2,
			RAM:  24.4,
			Load: 2.32,
			Time: 234,
			Reqs: 23,
		}
		mutex.Unlock()
		// *magic*
	}
}
