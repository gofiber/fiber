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

// acquireKeyBuffer takes a scratch buffer from the pool. The type assertion is
// guarded because a Pool may hand back whatever was Put into it.
func acquireKeyBuffer() *[]byte {
	if bufPtr, ok := keyBufferPool.Get().(*[]byte); ok && bufPtr != nil {
		return bufPtr
	}
	b := make([]byte, 0, defaultKeyBufferCap)
	return &b
}

// releaseKeyBuffer returns buf to the pool unless it grew too large to retain.
func releaseKeyBuffer(bufPtr *[]byte, buf []byte) {
	if cap(buf) <= defaultKeyBufferCap*4 {
		*bufPtr = buf
		keyBufferPool.Put(bufPtr)
	}
}

// appendGeneratedKey appends the request's generated key to dst.
//
// The configured generator hands back a string, which the caller then copies
// into the key it is assembling. Where that generator is this package's own, the
// segments go straight into dst instead and the intermediate string is never
// built — one allocation saved on every cacheable request. A generator the user
// supplied still returns a string, and is called exactly once either way.
func appendGeneratedKey(dst []byte, c fiber.Ctx, cfg *Config) []byte {
	if cfg.usesDefaultKeyGenerator {
		return appendDefaultKey(dst, c, cfg)
	}
	return append(dst, cfg.KeyGenerator(c)...)
}

func defaultKeyGenerator(c fiber.Ctx, cfg *Config) string {
	bufPtr := acquireKeyBuffer()
	buf := appendDefaultKey((*bufPtr)[:0], c, cfg)
	result := string(buf)
	releaseKeyBuffer(bufPtr, buf)
	return result
}

// appendDefaultKey appends the default generator's key to dst.
func appendDefaultKey(dst []byte, c fiber.Ctx, cfg *Config) []byte {
	// Escape delimiters in path to prevent crafted paths from injecting key structure
	dst = appendEscapedBoundKeySegment(dst, c.Path())

	if !cfg.DisableQueryKeys {
		dst = append(dst, '|', 'q', '=')
		dst = appendCanonicalQueryString(dst, c.Req().URI())
	}

	if len(cfg.KeyHeaders) > 0 {
		dst = append(dst, '|', 'h', '=')
		dst = appendCanonicalHeaderSubset(dst, &c.Request().Header, cfg.KeyHeaders, headerlookup.Canonical(c))
	}

	if len(cfg.KeyCookies) > 0 {
		dst = append(dst, '|', 'c', '=')
		dst = appendCanonicalCookieSubset(dst, c, cfg.KeyCookies)
	}

	if c.Method() == fiber.MethodQuery {
		// RFC 10008: incorporate the request body so different QUERY bodies on the
		// same URL get distinct keys.
		dst = append(dst, '|', 'b', '=')
		dst = appendQueryBodySegment(dst, c.Request().Body())
	}

	return dst
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
		return appendEscapedBoundKeySegment(dst, query)
	}

	// Fast path: single key=value pair needs no parsing or sorting
	if strings.IndexByte(query, '&') < 0 {
		return appendEscapedBoundKeySegment(dst, query)
	}

	// Quick count of potential parameters (ampersands + 1)
	paramCount := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '&' {
			paramCount++
			if paramCount > maxQueryParams {
				// Too many parameters detected, hash without parsing
				return appendEscapedBoundKeySegment(dst, query)
			}
		}
	}

	parsed, err := url.ParseQuery(query)
	if err != nil {
		return appendEscapedBoundKeySegment(dst, query)
	}

	// Double-check actual parameter count after parsing
	actualCount := 0
	for _, values := range parsed {
		actualCount += len(values)
		if actualCount > maxQueryParams {
			return appendEscapedBoundKeySegment(dst, query)
		}
	}

	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Use pooled buffer to prevent excessive memory allocation during URL escaping.
	// URL escaping can expand strings up to 3x (each byte -> %XX).
	bufPtr := acquireKeyBuffer()
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
				return appendEscapedBoundKeySegment(dst, query)
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
		dst = appendEscapedKeyDelimiters(dst, name)
		dst = append(dst, ':')

		// Every field line, not just the first: the split and comma-joined forms are
		// equivalent (RFC 9110 §5.2), so a key header sent twice keyed like one sent
		// once and was served its cached response.
		values := keyFieldLines(header, name, normalized)

		// The count keeps the framing injective: without it an absent header and
		// an empty one both emit nothing, and a list could not be told from a
		// single value containing the separator.
		dst = strconv.AppendInt(dst, int64(len(values)), 10)

		// Each value is bounded, but not how many there are, and a few hundred field
		// lines build a multi-kilobyte map key. Hash past the bound; the hash covers
		// raw lines, which the verbatim form never is, so the two cannot collide.
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
			dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(value))
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
		dst = appendEscapedKeyDelimiters(dst, name)
		dst = append(dst, ':')
		// Escape value to prevent delimiter injection
		dst = appendEscapedBoundKeySegment(dst, c.Cookies(name))
	}

	return dst
}

// countKeyDelimiters returns how many bytes escaping s would add, which is one
// per delimiter since each is replaced by a two-byte sequence. Asking this first
// lets the append forms below bound a segment without building the escaped
// string a strings.Replacer used to allocate for every request whose path,
// query, or body carries one of these bytes — a colon in a JSON body is
// enough.
func countKeyDelimiters(s string) int {
	n := 0
	for i := utils.IndexAny3(s, '|', ':', '\\'); i >= 0; i = utils.IndexAny3(s, '|', ':', '\\') {
		n++
		s = s[i+1:]
	}
	return n
}

// appendEscapedKeyDelimiters appends s to dst with the cache-key delimiters
// escaped: \ as \\, | as \p, : as \c. Escaping them stops a crafted path,
// query, header, or cookie value from injecting key structure of its own.
func appendEscapedKeyDelimiters(dst []byte, s string) []byte {
	for {
		i := utils.IndexAny3(s, '|', ':', '\\')
		if i < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:i]...)
		switch s[i] {
		case '\\':
			dst = append(dst, '\\', '\\')
		case '|':
			dst = append(dst, '\\', 'p')
		default: // ':'
			dst = append(dst, '\\', 'c')
		}
		s = s[i+1:]
	}
}

// appendEscapedBoundKeySegment escapes segment and appends it under the same
// bound appendBoundKeySegment applies.
//
// The reserved-prefix check appendBoundKeySegment makes is kept only on the
// path where nothing needed escaping. A segment that did cannot need it: the
// escaping emits no ':' at all, so its result never starts with hashPrefix.
func appendEscapedBoundKeySegment(dst []byte, segment string) []byte {
	extra := countKeyDelimiters(segment)
	if extra == 0 {
		// Escaping would leave the segment as it is, so this is
		// appendBoundKeySegment's own path, reserved-prefix check included.
		return appendBoundKeySegment(dst, segment)
	}

	if len(segment)+extra <= maxKeyDimensionSegmentLength {
		return appendEscapedKeyDelimiters(dst, segment)
	}

	// Oversized, so the digest is what lands in the key, and it covers the
	// escaped bytes — the same preimage as when the caller escaped first and
	// handed the result to appendBoundKeySegment. Escaped in a pooled buffer so
	// that stays true without a fresh string.
	bufPtr := acquireKeyBuffer()
	buf := appendEscapedKeyDelimiters((*bufPtr)[:0], segment)
	dst = appendHashedKeySegment(dst, buf)
	releaseKeyBuffer(bufPtr, buf)
	return dst
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
		s := utils.UnsafeString(body)
		if len(s)+countKeyDelimiters(s) <= maxKeyDimensionSegmentLength {
			return appendEscapedKeyDelimiters(dst, s)
		}
	}
	return appendHashedKeySegment(dst, body)
}
