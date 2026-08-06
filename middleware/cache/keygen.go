package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/headerlookup"
)

const (
	maxKeyDimensionSegmentLength = 192
	defaultKeyBufferCap          = 256
	maxQueryParams               = 128  // Maximum number of query parameters to parse
	maxQueryBufferSize           = 4096 // Maximum buffer size for query string canonicalization
)

// hashPrefix is the reserved namespace prefix for hashed key segments.
const hashPrefix = "sha256:"

var keyBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, defaultKeyBufferCap)
		return &buf
	},
}

// releaseKeyBuffer returns buf to the pool unless it grew too large to retain.
func releaseKeyBuffer(bufPtr *[]byte, buf []byte) {
	if cap(buf) <= defaultKeyBufferCap*4 {
		*bufPtr = buf
		keyBufferPool.Put(bufPtr)
	}
}

func defaultKeyGenerator(c fiber.Ctx, cfg *Config) string {
	v := keyBufferPool.Get()
	bufPtr, ok := v.(*[]byte)
	if !ok || bufPtr == nil {
		b := make([]byte, 0, defaultKeyBufferCap)
		bufPtr = &b
	}
	buf := (*bufPtr)[:0]

	// Escape delimiters in path to prevent crafted paths from injecting key structure
	buf = append(buf, boundKeySegment(escapeKeyDelimiters(c.Path()))...)

	if !cfg.DisableQueryKeys {
		buf = append(buf, '|', 'q', '=')
		buf = appendCanonicalQueryString(buf, c.Request().URI())
	}

	if len(cfg.KeyHeaders) > 0 {
		buf = append(buf, '|', 'h', '=')
		buf = appendCanonicalHeaderSubset(buf, &c.Request().Header, cfg.KeyHeaders, headerlookup.Canonical(c))
	}

	if len(cfg.KeyCookies) > 0 {
		buf = append(buf, '|', 'c', '=')
		buf = appendCanonicalCookieSubset(buf, c, cfg.KeyCookies)
	}

	if c.Method() == fiber.MethodQuery {
		// RFC 10008: incorporate the request body so different QUERY bodies on the
		// same URL get distinct keys.
		buf = append(buf, '|', 'b', '=')
		buf = appendQueryBodySegment(buf, c.Request().Body())
	}

	result := string(buf)
	releaseKeyBuffer(bufPtr, buf)
	return result
}

// appendCanonicalQueryString appends the canonicalized query segment to dst.
// It avoids copying the raw query and the intermediate result string the caller
// would otherwise have to re-append.
func appendCanonicalQueryString(dst []byte, uri *fasthttp.URI) []byte {
	raw := uri.QueryString()
	if len(raw) == 0 {
		return dst
	}

	// Safe: the segment is consumed synchronously (appended/hashed) before the
	// request buffer can be mutated, so no stable copy is required.
	query := utils.UnsafeString(raw)

	// Pre-scan query string to detect excessive parameters before expensive parsing.
	// This prevents DoS via url.ParseQuery allocating large maps/slices.
	if len(query) > maxQueryBufferSize {
		return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
	}

	// Fast path: single key=value pair needs no parsing or sorting
	if strings.IndexByte(query, '&') < 0 {
		return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
	}

	// Quick count of potential parameters (ampersands + 1)
	paramCount := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '&' {
			paramCount++
			if paramCount > maxQueryParams {
				// Too many parameters detected, hash without parsing
				return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
			}
		}
	}

	parsed, err := url.ParseQuery(query)
	if err != nil {
		return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
	}

	// Double-check actual parameter count after parsing
	actualCount := 0
	for _, values := range parsed {
		actualCount += len(values)
		if actualCount > maxQueryParams {
			return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
		}
	}

	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Use pooled buffer to prevent excessive memory allocation during URL escaping.
	// URL escaping can expand strings up to 3x (each byte -> %XX).
	v := keyBufferPool.Get()
	bufPtr, ok := v.(*[]byte)
	if !ok || bufPtr == nil {
		b := make([]byte, 0, defaultKeyBufferCap)
		bufPtr = &b
	}
	buf := (*bufPtr)[:0]

	for _, key := range keys {
		values := parsed[key]
		sort.Strings(values)
		for _, value := range values {
			if len(buf) > 0 {
				buf = append(buf, '&')
			}

			// Escape straight into the pooled buffer to skip the intermediate
			// strings url.QueryEscape would allocate per key and value.
			buf = utils.AppendQueryEscape(buf, key)
			buf = append(buf, '=')
			buf = utils.AppendQueryEscape(buf, value)

			// Check buffer size to prevent unbounded growth. Equivalent to the
			// pre-append check len(prev)+len(key)+len(value)+2 > max: the pair
			// plus '=' is already appended, leaving only the +1 slack.
			if len(buf)+1 > maxQueryBufferSize {
				releaseKeyBuffer(bufPtr, buf)
				return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
			}
		}
	}

	dst = appendBoundKeySegment(dst, utils.UnsafeString(buf))
	releaseKeyBuffer(bufPtr, buf)
	return dst
}

func appendCanonicalHeaderSubset(dst []byte, header *fasthttp.RequestHeader, names []string, normalized bool) []byte {
	for idx, name := range names {
		if idx > 0 {
			dst = append(dst, '|')
		}
		// Escape name (though names are normalized and trusted)
		dst = append(dst, escapeKeyDelimiters(name)...)
		dst = append(dst, ':')

		// Every field line, not just the first. A name may arrive on more than
		// one line, and the split form is equivalent to the comma-joined one on
		// the wire (RFC 9110 Section 5.2) — but Peek returns only the first, so
		// a request sending a key header twice keyed identically to one sending
		// just that first value and was served its cached response. PeekAll
		// reuses the header's own scratch slice for ordinary names, so those
		// cost no allocation; the values are consumed before the next call to
		// it. Cookie and Trailer are the exception — fasthttp re-serializes
		// those from its own stores into a fresh buffer per call — so naming
		// either in KeyHeaders allocates once per request.
		values := keyFieldLines(header, name, normalized)

		// The count keeps the framing injective. Without it an absent header
		// and one present with an empty value both emit nothing, and a list of
		// values could not be told from a single value that happened to contain
		// the separator. Escaping guarantees no value holds a raw '|', so the
		// separator below is unambiguous.
		dst = strconv.AppendInt(dst, int64(len(values)), 10)

		// appendBoundKeySegment bounds each value, but nothing bounds how many
		// there are, and a client may repeat a header as often as the read
		// buffer allows. Left alone, a few hundred field lines would build a
		// multi-kilobyte key that is then concatenated into the manifest and
		// body keys and used as a map key in the store — so hold the whole
		// dimension to the same per-dimension bound the rest of this file keeps,
		// hashing it once past that point. The hash covers the raw lines, which
		// the verbatim form never is, so the two cannot collide.
		total := 0
		for _, value := range values {
			total += len(value) + 1
		}
		if total > maxKeyDimensionSegmentLength {
			dst = appendHashedKeySegment(dst, joinKeyHeaderValues(values))
			continue
		}

		for _, value := range values {
			dst = append(dst, '|')
			// Escape value to prevent delimiter injection
			dst = appendBoundKeySegment(dst, escapeKeyDelimiters(utils.UnsafeString(value)))
		}
	}

	return dst
}

// joinKeyHeaderValues concatenates field lines with a length prefix each, so
// the digest taken over the result distinguishes value lists that a plain
// concatenation would not (["ab"] from ["a","b"]).
func joinKeyHeaderValues(values [][]byte) []byte {
	n := 0
	for _, v := range values {
		n += len(v) + binary.MaxVarintLen64
	}
	joined := make([]byte, 0, n)
	for _, v := range values {
		joined = binary.AppendUvarint(joined, uint64(len(v)))
		joined = append(joined, v...)
	}
	return joined
}

func appendCanonicalCookieSubset(dst []byte, c fiber.Ctx, names []string) []byte {
	for idx, name := range names {
		if idx > 0 {
			dst = append(dst, '|')
		}
		// Escape name (though names are normalized and trusted)
		dst = append(dst, escapeKeyDelimiters(name)...)
		dst = append(dst, ':')
		cookieValue := c.Cookies(name)
		// Escape value to prevent delimiter injection
		escapedValue := escapeKeyDelimiters(cookieValue)
		dst = appendBoundKeySegment(dst, escapedValue)
	}

	return dst
}

// keyDelimiterEscaper escapes the delimiters in one pass: \ as \\, | as \p, : as \c.
var keyDelimiterEscaper = strings.NewReplacer(`\`, `\\`, `|`, `\p`, `:`, `\c`)

// escapeKeyDelimiters escapes pipe, colon, and backslash characters used as delimiters in cache keys
// to prevent injection attacks where crafted values could collide with different inputs
func escapeKeyDelimiters(s string) string {
	// Fast path: no characters to escape
	if utils.IndexAny3(s, '|', ':', '\\') == -1 {
		return s
	}
	return keyDelimiterEscaper.Replace(s)
}

func boundKeySegment(segment string) string {
	// Hash oversized segments, and also any segment that already starts with the
	// reserved hashPrefix, so a literal "sha256:..." value cannot collide with a
	// genuinely-hashed long segment (defense-in-depth alongside escapeKeyDelimiters).
	if len(segment) <= maxKeyDimensionSegmentLength && !strings.HasPrefix(segment, hashPrefix) {
		return segment
	}
	hash := sha256.Sum256(utils.UnsafeBytes(segment))
	return hashPrefix + hex.EncodeToString(hash[:])
}

// appendBoundKeySegment appends segment to dst, hashing it first when it exceeds
// the per-dimension length bound or already starts with the reserved hashPrefix
// (same policy as boundKeySegment).
func appendBoundKeySegment(dst []byte, segment string) []byte {
	if len(segment) <= maxKeyDimensionSegmentLength && !strings.HasPrefix(segment, hashPrefix) {
		return append(dst, segment...)
	}
	hash := sha256.Sum256(utils.UnsafeBytes(segment))
	dst = append(dst, hashPrefix...)
	return hex.AppendEncode(dst, hash[:])
}

func appendHashedKeySegment(dst, segment []byte) []byte {
	hash := sha256.Sum256(segment)
	dst = append(dst, hashPrefix...)
	return hex.AppendEncode(dst, hash[:])
}

// appendQueryBodySegment appends a QUERY request body as a key segment. A body
// that fits the per-dimension bound both raw and after escaping is escaped and
// appended verbatim; otherwise the raw body is hashed. The hash is always taken
// over the raw bytes, so the verbatim and hashed forms can never share a
// preimage and collide, and an oversized body is never escaped (avoids 2x
// memory amplification on delimiter-heavy input). Escaping the verbatim form
// still stops a body containing |/:/\ from injecting key-suffix structure.
func appendQueryBodySegment(dst, body []byte) []byte {
	if len(body) <= maxKeyDimensionSegmentLength {
		if escaped := escapeKeyDelimiters(utils.UnsafeString(body)); len(escaped) <= maxKeyDimensionSegmentLength {
			return append(dst, escaped...)
		}
	}
	return appendHashedKeySegment(dst, body)
}
