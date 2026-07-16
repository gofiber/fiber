package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3"
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
		buf = appendCanonicalHeaderSubset(buf, &c.Request().Header, cfg.KeyHeaders)
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

			escapedKey := url.QueryEscape(key)
			escapedValue := url.QueryEscape(value)

			// Check buffer size before appending to prevent unbounded growth
			if len(buf)+len(escapedKey)+len(escapedValue)+2 > maxQueryBufferSize {
				releaseKeyBuffer(bufPtr, buf)
				return appendBoundKeySegment(dst, escapeKeyDelimiters(query))
			}

			buf = append(buf, escapedKey...)
			buf = append(buf, '=')
			buf = append(buf, escapedValue...)
		}
	}

	dst = appendBoundKeySegment(dst, utils.UnsafeString(buf))
	releaseKeyBuffer(bufPtr, buf)
	return dst
}

func appendCanonicalHeaderSubset(dst []byte, header *fasthttp.RequestHeader, names []string) []byte {
	for idx, name := range names {
		if idx > 0 {
			dst = append(dst, '|')
		}
		// Escape name (though names are normalized and trusted)
		dst = append(dst, escapeKeyDelimiters(name)...)
		dst = append(dst, ':')
		headerValue := header.Peek(name)
		// Escape value to prevent delimiter injection
		escapedValue := escapeKeyDelimiters(utils.UnsafeString(headerValue))
		dst = appendBoundKeySegment(dst, escapedValue)
	}

	return dst
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
