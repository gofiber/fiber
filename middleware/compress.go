package middleware

import (
	"github.com/gofiber/fiber"
	"github.com/valyala/fasthttp"
)

// Middleware types
type (
	// CompressConfig defines the config for Compress middleware.
	CompressConfig struct {
		// Next defines a function to skip this middleware.
		Next func(ctx *fiber.Ctx) bool
		// Compression level for brotli, gzip and deflate
		Level int
	}
)

// Compression levels
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

// Compress is the default initiator allowing to pass the compression level
// It supports brotli, gzip and deflate compression
// The same order is used to check against the Accept-Encoding header
func Compress(level ...int) fiber.Handler {
	// Create default config
	var config = CompressConfigDefault
	// Set level if provided
	if len(level) > 0 {
		config.Level = level[0]
	}
	// Return CompressWithConfig
	return CompressWithConfig(config)
}

// CompressWithConfig allows you to pass an CompressConfig
// It supports brotli, gzip and deflate compression
// The same order is used to check against the Accept-Encoding header
func CompressWithConfig(config CompressConfig) fiber.Handler {
	// Init middleware settings
	var compress fasthttp.RequestHandler
	switch config.Level {
	case -1: // Disabled
		compress = fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliNoCompression, fasthttp.CompressNoCompression)
	case 1: // Best speed
		compress = fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed)
	case 2: // Best compression
		compress = fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliBestCompression, fasthttp.CompressBestCompression)
	default: // Default
		compress = fasthttp.CompressHandlerBrotliLevel(func(c *fasthttp.RequestCtx) {}, fasthttp.CompressBrotliDefaultCompression, fasthttp.CompressDefaultCompression)
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
		compress(c.Fasthttp)
	}
}
