package etag

import (
	"hash/crc32"
	"math"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	internaletag "github.com/gofiber/fiber/v3/internal/etag"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
	"github.com/gofiber/utils/v2"
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

		// Never generate ETags for Server-Sent Events: hashing the body would
		// materialize the stream and break real-time delivery.
		if isEventStream(c) {
			return nil
		}

		// Don't generate ETags for invalid responses
		if c.Res().StatusCode() != fiber.StatusOK {
			return nil
		}
		body := c.Response().Body()
		// Skips ETag if no response body is present
		if len(body) == 0 {
			return nil
		}
		// Skip ETag if any field line is already present, whatever case the
		// handler spelled the name in — a second line would be a conflicting
		// validator (RFC 9110 Section 8.8.3).
		//
		// Both halves of the guard are needed: fasthttp canonicalizes ETag to
		// "Etag", so a store of canonical names still holds a spelling the
		// byte-exact PeekAll(fiber.HeaderETag) misses while normalizing is off.
		respHeader := &c.Response().Header
		if len(fieldname.Lines(respHeader, fiber.HeaderETag, headerlookup.Canonical(c) && fieldname.Canonical(respHeader))) > 0 {
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

		// 304 is defined for GET and HEAD only (RFC 9110 §13.1.2); other methods pass through.
		if method := c.Method(); method != fiber.MethodGet && method != fiber.MethodHead {
			return nil
		}

		// Get ETag header from request. If-None-Match is a list field: repeated
		// lines are one combined list (RFC 9110 Section 5.2) and the name
		// matches case-insensitively, agreeing with the core Fresh path.
		clientEtag := headerlist.Join(fieldname.Lines(&c.Request().Header, fiber.HeaderIfNoneMatch, headerlookup.Canonical(c)))

		// Both slices are only read for the duration of the comparison and
		// neither is retained, so the unsafe views cannot outlive them.
		if internaletag.AnyMatch(utils.UnsafeString(clientEtag), utils.UnsafeString(etag)) {
			c.Res().ResetBody()

			return c.SendStatus(fiber.StatusNotModified)
		}

		return nil
	}
}

// isEventStream reports whether the response is a Server-Sent Events stream.
// Such responses must be passed through untouched: buffering the body to hash
// it would break real-time delivery.
func isEventStream(c fiber.Ctx) bool {
	ct := c.GetRespHeader(fiber.HeaderContentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return utils.EqualFold(utils.TrimSpace(ct), fiber.MIMETextEventStream)
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
