package compress

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/utils/v2"
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

		// Negotiate with weights, wildcards and lists honored (RFC 9110 §12.5.3).
		encoding := negotiateEncoding(c.Get(fiber.HeaderAcceptEncoding))
		if encoding == "" {
			appendVaryAcceptEncoding(c)
			return nil
		}

		compressWith(c, compressor, encoding)

		if tag := c.GetRespHeader(fiber.HeaderETag); tag != "" && !strings.HasPrefix(tag, "W/") {
			if c.GetRespHeader(fiber.HeaderContentEncoding) != "" {
				if c.Response().IsBodyStream() {
					// The encoded bytes are not in memory to hash; a weak validator still names the representation.
					c.Set(fiber.HeaderETag, "W/"+tag)
				} else {
					c.Set(fiber.HeaderETag, string(etag.Generate(c.Response().Body())))
				}
			}
		}

		appendVaryAcceptEncoding(c)

		return nil
	}
}

// supportedEncodings lists the content codings the middleware can produce, in preference order.
var supportedEncodings = [...]string{"br", "zstd", "gzip", "deflate"}

// negotiateEncoding picks the supported coding the client weighs highest in
// Accept-Encoding (RFC 9110 §12.5.3); "" when none is acceptable or the header is absent.
func negotiateEncoding(accept string) string {
	best := ""
	bestQuality := 0.0
	for _, offer := range supportedEncodings {
		if quality := encodingQuality(accept, offer); quality > bestQuality {
			best, bestQuality = offer, quality
		}
	}
	return best
}

// encodingQuality returns the weight Accept-Encoding gives a coding: its own entry, else the wildcard's, else 0.
func encodingQuality(accept, offer string) float64 {
	wildcard := 0.0
	for element := range headerlist.All(accept) {
		token, params := element, ""
		if i := strings.IndexByte(element, ';'); i >= 0 {
			// VisitHeaderParams expects the parameters with their leading ';'.
			token, params = element[:i], element[i:]
		}
		token = utils.TrimSpace(token)
		if token != "*" && !utils.EqualFold(token, offer) {
			continue
		}
		// An element carries other parameters than the weight, and an absent or
		// malformed q leaves the default weight of 1 (RFC 9110 §12.4.2).
		quality := 1.0
		if params != "" {
			fasthttp.VisitHeaderParams(utils.UnsafeBytes(params), func(key, value []byte) bool {
				if len(key) == 1 && (key[0] == 'q' || key[0] == 'Q') {
					if parsed, err := fasthttp.ParseUfloat(value); err == nil {
						quality = parsed
					}
					return false
				}
				return true
			})
		}
		if token == "*" {
			wildcard = quality
			continue
		}
		return quality
	}
	return wildcard
}

// compressWith runs fasthttp's compressor for the negotiated encoding, handing
// it the bare token, and restores the client's Accept-Encoding afterwards.
func compressWith(c fiber.Ctx, compressor fasthttp.RequestHandler, encoding string) {
	header := &c.Request().Header
	lines := header.PeekAll(fiber.HeaderAcceptEncoding)
	saved := make([][]byte, len(lines))
	for i, line := range lines {
		saved[i] = utils.CopyBytes(line)
	}

	header.Set(fiber.HeaderAcceptEncoding, encoding)
	compressor(c.RequestCtx())

	header.Del(fiber.HeaderAcceptEncoding)
	for _, line := range saved {
		header.AddBytesV(fiber.HeaderAcceptEncoding, line)
	}
}
