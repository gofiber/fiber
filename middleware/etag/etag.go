package etag

import (
	"bytes"
	"hash/crc32"
	"math"
	"slices"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/bytebufferpool"
)

var (
	weakPrefix = []byte("W/")
	crc32q     = crc32.MakeTable(0xD5828281)
)

// Generate returns a strong ETag for body.
func Generate(body []byte) []byte {
	if len(body) > math.MaxUint32 {
		return nil
	}
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)
	b := bb.B[:0]
	b = append(b, '"')
	b = appendUint(b, uint32(len(body))) // #nosec G115 -- length checked above
	b = append(b, '-')
	b = appendUint(b, crc32.Checksum(body, crc32q))
	b = append(b, '"')
	return slices.Clone(b)
}

// GenerateWeak returns a weak ETag for body.
func GenerateWeak(body []byte) []byte {
	tag := Generate(body)
	if tag == nil {
		return nil
	}
	return append(weakPrefix, tag...)
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	normalizedHeaderETag := []byte("Etag")

	// Return new handler
	return func(c fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Return err if next handler returns one
		if err := c.Next(); err != nil {
			return err
		}

		// Don't generate ETags for invalid responses
		if c.Response().StatusCode() != fiber.StatusOK {
			return nil
		}
		body := c.Response().Body()
		// Skips ETag if no response body is present
		if len(body) == 0 {
			return nil
		}
		// Skip ETag if header is already present
		if c.Response().Header.PeekBytes(normalizedHeaderETag) != nil {
			return nil
		}

		bodyLength := len(body)
		if bodyLength > math.MaxUint32 {
			return c.SendStatus(fiber.StatusRequestEntityTooLarge)
		}

		var etag []byte
		if cfg.Weak {
			etag = GenerateWeak(body)
		} else {
			etag = Generate(body)
		}

		// Get ETag header from request
		clientEtag := c.Request().Header.Peek(fiber.HeaderIfNoneMatch)

		// Check if client's ETag is weak
		if bytes.HasPrefix(clientEtag, weakPrefix) {
			// Check if server's ETag is weak
			if bytes.Equal(clientEtag[2:], etag) || bytes.Equal(clientEtag[2:], etag[2:]) {
				// W/1 == 1 || W/1 == W/1
				c.RequestCtx().ResetBody()

				return c.SendStatus(fiber.StatusNotModified)
			}
			// W/1 != W/2 || W/1 != 2
			c.Response().Header.SetCanonical(normalizedHeaderETag, etag)

			return nil
		}

		if bytes.Contains(clientEtag, etag) {
			// 1 == 1
			c.RequestCtx().ResetBody()

			return c.SendStatus(fiber.StatusNotModified)
		}
		// 1 != 2
		c.Response().Header.SetCanonical(normalizedHeaderETag, etag)

		return nil
	}
}

// appendUint appends n to dst and returns the extended dst.
func appendUint(dst []byte, n uint32) []byte {
	var b [20]byte
	buf := b[:]
	i := len(buf)
	var q uint32
	for n >= 10 {
		i--
		q = n / 10
		buf[i] = '0' + byte(n-q*10)
		n = q
	}
	i--
	buf[i] = '0' + byte(n)

	dst = append(dst, buf[i:]...)
	return dst
}
