package compress

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

func hasToken(header, token string) bool {
	for part := range strings.SplitSeq(header, ",") {
		if utils.EqualFold(utils.Trim(part, ' '), token) {
			return true
		}
	}
	return false
}

func shouldSkip(c fiber.Ctx) bool {
	status := c.Response().StatusCode()
	if status < 200 ||
		status == fiber.StatusNoContent ||
		status == fiber.StatusResetContent ||
		status == fiber.StatusNotModified ||
		status == fiber.StatusPartialContent ||
		len(c.Response().Body()) == 0 ||
		c.Get(fiber.HeaderRange) != "" ||
		hasToken(c.Get(fiber.HeaderCacheControl), "no-transform") ||
		hasToken(c.GetRespHeader(fiber.HeaderCacheControl), "no-transform") {
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
	if hasToken(vary, "*") || hasToken(vary, fiber.HeaderAcceptEncoding) {
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
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx,
			fasthttp.CompressBrotliDefaultCompression,
			fasthttp.CompressDefaultCompression,
		)
	case LevelBestSpeed:
		// LevelBestSpeed
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx,
			fasthttp.CompressBrotliBestSpeed,
			fasthttp.CompressBestSpeed,
		)
	case LevelBestCompression:
		// LevelBestCompression
		compressor = fasthttp.CompressHandlerBrotliLevel(fctx,
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

		if c.Method() == fiber.MethodHead {
			defer func() {
				clen := len(c.Response().Body())
				c.RequestCtx().ResetBody()
				c.Response().Header.SetContentLength(clen)
			}()
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
