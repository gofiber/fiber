package compress

import (
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// CompressLevel determines the compression algoritm
	//
	// Optional. Default: LevelDefault
	// LevelDisabled:         -1
	// LevelDefault:          0
	// LevelBestSpeed:        1
	// LevelBestCompression:  2
	Level int
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:  nil,
	Level: LevelDefault,
}

// Compression levels
const (
	LevelDisabled        = -1
	LevelDefault         = 0
	LevelBestSpeed       = 1
	LevelBestCompression = 2
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.Level < -1 || cfg.Level > 2 {
			cfg.Level = ConfigDefault.Level
		}
	}

	// Setup request handlers
	var (
		fctx       = func(c *fasthttp.RequestCtx) {}
		compressor fasthttp.RequestHandler
	)

	// Setup compression algorithm
	switch cfg.Level {
	case LevelDefault:
		// LevelDefault
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx, fasthttp.CompressBrotliDefaultCompression, fasthttp.CompressDefaultCompression)
	case LevelBestSpeed:
		// LevelBestSpeed
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)
	case LevelBestCompression:
		// LevelBestCompression
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx, fasthttp.CompressBrotliBestCompression, fasthttp.CompressBestCompression)
	default:
		// LevelDisabled
		return func(c *fiber.Ctx) error {
			return c.Next()
		}
	}

	// Return new handler
	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Continue stack
		if err := c.Next(); err != nil {
			return err
		}

		// Compress response
		compressor(c.Context())

		// Return from handler
		return nil
	}
}
