package etag

import (
	"bytes"
	"hash/crc32"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/bytebufferpool"
)

// Config defines the config for middleware.
type Config struct {
	// Weak indicates that a weak validator is used. Weak etags are easy
	// to generate, but are far less useful for comparisons. Strong
	// validators are ideal for comparisons but can be very difficult
	// to generate efficiently. Weak ETag values of two representations
	// of the same resources might be semantically equivalent, but not
	// byte-for-byte identical. This means weak etags prevent caching
	// when byte range requests are used, but strong etags mean range
	// requests can still be cached.
	Weak bool

	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	Weak: false,
	Next: nil,
}

var normalizedHeaderETag = []byte("Etag")
var weakPrefix = []byte("W/")

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]
	}

	var crc32q = crc32.MakeTable(0xD5828281)

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Return err if next handler returns one
		if err = c.Next(); err != nil {
			return
		}

		// Don't generate ETags for invalid responses
		if c.Response().StatusCode() != fiber.StatusOK {
			return
		}
		body := c.Response().Body()
		// Skips ETag if no response body is present
		if len(body) <= 0 {
			return
		}

		// Generate ETag for response
		bb := bytebufferpool.Get()
		defer bytebufferpool.Put(bb)

		// Enable weak tag
		if cfg.Weak {
			_, _ = bb.Write(weakPrefix)
		}

		_ = bb.WriteByte('"')
		bb.B = appendUint(bb.Bytes(), uint32(len(body)))
		_ = bb.WriteByte('-')
		bb.B = appendUint(bb.Bytes(), crc32.Checksum(body, crc32q))
		_ = bb.WriteByte('"')

		etag := bb.Bytes()

		// Get ETag header from request
		clientEtag := c.Request().Header.Peek(fiber.HeaderIfNoneMatch)

		// Check if client's ETag is weak
		if bytes.HasPrefix(clientEtag, weakPrefix) {
			// Check if server's ETag is weak
			if bytes.Equal(clientEtag[2:], etag) || bytes.Equal(clientEtag[2:], etag[2:]) {
				// W/1 == 1 || W/1 == W/1
				c.Context().ResetBody()

				return c.SendStatus(fiber.StatusNotModified)
			}
			// W/1 != W/2 || W/1 != 2
			c.Response().Header.SetCanonical(normalizedHeaderETag, etag)

			return
		}

		if bytes.Contains(clientEtag, etag) {
			// 1 == 1
			c.Context().ResetBody()

			return c.SendStatus(fiber.StatusNotModified)
		}
		// 1 != 2
		c.Response().Header.SetCanonical(normalizedHeaderETag, etag)

		return
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
