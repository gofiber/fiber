package middleware

import (
	fiber "github.com/gofiber/fiber"
	fasthttp "github.com/valyala/fasthttp"
)

// CompressConfig defines the config for Compress middleware.
type CompressConfig struct {
	// Next defines a function to skip this middleware if returned true.
	Next func(ctx *fiber.Ctx) bool
	// Compression level for brotli, gzip and deflate
	Level int
}

// Compression levels determine the compression complexity
const (
	CompressLevelDisabled        = -1
	CompressLevelDefault         = 0
	CompressLevelBestSpeed       = 1
	CompressLevelBestCompression = 2
)

// CompressConfigDefault is the default config
var CompressConfigDefault = CompressConfig{
	Next:  nil,
	Level: CompressLevelDefault,
}

var compressHandlers = map[int]fasthttp.RequestHandler{
	CompressLevelDisabled:        fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliNoCompression, fasthttp.CompressNoCompression),
	CompressLevelDefault:         fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliDefaultCompression, fasthttp.CompressDefaultCompression),
	CompressLevelBestSpeed:       fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed),
	CompressLevelBestCompression: fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliBestCompression, fasthttp.CompressBestCompression),
}

/*
Compress allows the following config arguments in any order:
	- Compress()
	- Compress(next func(*fiber.Ctx) bool)
	- Compress(level int)
	- Compress(config CompressConfig)
*/
func Compress(options ...interface{}) fiber.Handler {
	// Create default config
	var config = CompressConfigDefault
	// Assert options if provided to adjust the config
	if len(options) > 0 {
		for i := range options {
			switch opt := options[i].(type) {
			case func(*fiber.Ctx) bool:
				config.Next = opt
			case int:
				config.Level = opt
			case CompressConfig:
				config = opt
			default:
				panic("Compress: the following option types are allowed: int, func(*fiber.Ctx) bool, CompressConfig")
			}
		}
	}
	// Return CompressWithConfig
	return compress(config)
}

func compress(config CompressConfig) fiber.Handler {
	// Init middleware settings
	compressHandler, ok := compressHandlers[config.Level]
	if !ok {
		// Use default level if provided level is invalid
		compressHandler = compressHandlers[CompressLevelDefault]
	}

	// Return handler
	return func(c *fiber.Ctx) {
		// Don't execute the middleware if Next returns false
		if config.Next != nil && config.Next(c) {
			c.Next()
			return
		}
		// Middleware logic...
		c.Next()
		// Compress response
		compressHandler(c.Fasthttp)
	}
}
