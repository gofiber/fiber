package favicon

import (
	"io/ioutil"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// File holds the path to an actual favicon that will be cached
	//
	// Optional. Default: ""
	File string

	// CacheControl defines how the Cache-Control header in the response should be set
	//
	// Optional. Default: "public, max-age=31536000"
	CacheControl string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	File:         "",
	CacheControl: "public, max-age=31536000",
}

const (
	hType  = "image/x-icon"
	hAllow = "GET, HEAD, OPTIONS"
	hZero  = "0"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Next == nil {
			cfg.Next = ConfigDefault.Next
		}
		if cfg.File == "" {
			cfg.File = ConfigDefault.File
		}
		if cfg.CacheControl == "" {
			cfg.CacheControl = ConfigDefault.CacheControl
		}
	}

	// Load icon if provided
	var (
		err     error
		icon    []byte
		iconLen string
	)
	if cfg.File != "" {
		if icon, err = ioutil.ReadFile(cfg.File); err != nil {
			panic(err)
		}
		iconLen = strconv.Itoa(len(icon))
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Only respond to favicon requests
		if len(c.Path()) != 12 || c.Path() != "/favicon.ico" {
			return c.Next()
		}

		// Only allow GET, HEAD and OPTIONS requests
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			if c.Method() != fiber.MethodOptions {
				c.Status(fiber.StatusMethodNotAllowed)
			} else {
				c.Status(fiber.StatusOK)
			}
			c.Set(fiber.HeaderAllow, hAllow)
			c.Set(fiber.HeaderContentLength, hZero)
			return nil
		}

		// Serve cached favicon
		if len(icon) > 0 {
			c.Set(fiber.HeaderContentLength, iconLen)
			c.Set(fiber.HeaderContentType, hType)
			c.Set(fiber.HeaderCacheControl, cfg.CacheControl)
			return c.Status(fiber.StatusOK).Send(icon)
		}

		return c.SendStatus(fiber.StatusNoContent)
	}
}
