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
	if uint64(len(body)) > uint64(math.MaxUint32) {
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
		if uint64(bodyLength) > uint64(math.MaxUint32) {
			return c.SendStatus(fiber.StatusRequestEntityTooLarge)
		}

		var etag []byte
		if cfg.Weak {
			etag = GenerateWeak(body)
		} else {
			etag = Generate(body)
		}

		// The ETag header is sent on both 200 and 304 responses (RFC 9110 §15.4.5).
		c.Response().Header.SetCanonical(normalizedHeaderETag, etag)

		// Get ETag header from request
		clientEtag := c.Request().Header.Peek(fiber.HeaderIfNoneMatch)

		if isNoneMatch(clientEtag, etag) {
			c.RequestCtx().ResetBody()

			return c.SendStatus(fiber.StatusNotModified)
		}

		return nil
	}
}

// isNoneMatch reports whether any entity tag in the If-None-Match header value
// matches the response ETag, using the weak comparison required for
// If-None-Match by RFC 9110 §8.8.3.2.
func isNoneMatch(header, etag []byte) bool {
	header = bytes.TrimSpace(header)
	if len(header) == 0 {
		return false
	}
	if bytes.Equal(header, []byte("*")) {
		return true
	}

	for len(header) > 0 {
		entry, rest, _ := bytes.Cut(header, []byte(","))
		header = rest
		if etagWeakMatch(bytes.TrimSpace(entry), etag) {
			return true
		}
	}

	return false
}

// etagWeakMatch compares two entity tags, ignoring weak indicators
// (RFC 9110 §8.8.3.2). Both tags must be quoted to match.
func etagWeakMatch(a, b []byte) bool {
	a = bytes.TrimPrefix(a, weakPrefix)
	b = bytes.TrimPrefix(b, weakPrefix)
	if len(a) < 2 || a[0] != '"' || a[len(a)-1] != '"' {
		return false
	}
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return false
	}

	return bytes.Equal(a, b)
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
		buf[i] = '0' + byte(n-q*10) //nolint:gosec // G115: integer overflow conversion uint32 -> byte
		n = q
	}
	i--
	buf[i] = '0' + byte(n)

	dst = append(dst, buf[i:]...)
	return dst
}
