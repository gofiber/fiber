package compress

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/valyala/fasthttp"
)

func shouldSkip(c fiber.Ctx) bool {
	if c.Method() == fiber.MethodHead {
		return true
	}

	status := c.Res().StatusCode()
	if status < 200 ||
		status == fiber.StatusNoContent ||
		status == fiber.StatusResetContent ||
		status == fiber.StatusNotModified ||
		status == fiber.StatusPartialContent ||
		c.Get(fiber.HeaderRange) != "" ||
		headerlist.ContainsFold(c.Get(fiber.HeaderCacheControl), "no-transform") ||
		headerlist.ContainsFold(c.GetRespHeader(fiber.HeaderCacheControl), "no-transform") {
		return true
	}

	// Written counts a stream without draining it, so a streaming response is
	// never mistaken for an empty one.
	if !c.Res().Written() {
		return true
	}

	return false
}

func appendVaryAcceptEncoding(c fiber.Ctx) {
	vary := c.GetRespHeader(fiber.HeaderVary)
	if vary == "" {
		c.Set(fiber.HeaderVary, fiber.HeaderAcceptEncoding)
		return
	}
	if headerlist.ContainsFold(vary, "*") || headerlist.ContainsFold(vary, fiber.HeaderAcceptEncoding) {
		return
	}
	c.Set(fiber.HeaderVary, vary+", "+fiber.HeaderAcceptEncoding)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	// Setup request handlers
	var (
		fctx       = func(_ *fasthttp.RequestCtx) {}
		compressor fasthttp.RequestHandler
	)

	// Setup compression algorithm
	switch cfg.Level {
	case LevelDefault:
		// LevelDefault
		compressor = fasthttp.CompressHandlerBrotliLevel(
			fctx,
			fasthttp.CompressBrotliDefaultCompression,
			fasthttp.CompressDefaultCompression,
		)
	case LevelBestSpeed:
		// LevelBestSpeed
		compressor = fasthttp.CompressHandlerBrotliLevel(
			fctx,
			fasthttp.CompressBrotliBestSpeed,
			fasthttp.CompressBestSpeed,
		)
	case LevelBestCompression:
		// LevelBestCompression
		compressor = fasthttp.CompressHandlerBrotliLevel(
			fctx,
			fasthttp.CompressBrotliBestCompression,
			fasthttp.CompressBestCompression,
		)
	default:
		// LevelDisabled
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Continue stack
		if err := c.Next(); err != nil {
			return err
		}

		if shouldSkip(c) {
			appendVaryAcceptEncoding(c)
			return nil
		}

		if c.GetRespHeader(fiber.HeaderContentEncoding) != "" {
			appendVaryAcceptEncoding(c)
			return nil
		}

		compressor(c.RequestCtx())

		if tag := c.GetRespHeader(fiber.HeaderETag); tag != "" && !strings.HasPrefix(tag, "W/") {
			if c.GetRespHeader(fiber.HeaderContentEncoding) != "" {
				c.Set(fiber.HeaderETag, string(etag.Generate(c.Response().Body())))
			}
		}

		appendVaryAcceptEncoding(c)

		return nil
	}
}
