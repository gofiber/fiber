package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
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
	maxQueryParams               = 128  // Most parameters a query may have for them to be sorted
	maxQueryBufferSize           = 4096 // Longest query whose parameters are sorted
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
// Its key is escaped, as it may copy client text that spells a "|vary|" or "|auth=" suffix.
func appendGeneratedKey(dst []byte, c fiber.Ctx, cfg *Config) []byte {
	if cfg.usesDefaultKeyGenerator {
		return appendDefaultKey(dst, c, cfg)
	}
	return appendEscapedKeyDelimiters(dst, cfg.KeyGenerator(c))
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
	dst = appendURLKey(dst, c)
	uri := c.Req().URI()
	if !cfg.DisableQueryKeys {
		dst = append(dst, '|', 'q', '=')
		dst = appendCanonicalQueryString(dst, uri)
	}
	dst = appendTargetAsSent(dst, c.Request().Header.RequestURI(), c.Path(), uri.QueryString(), cfg)

	normalized := headerlookup.Canonical(c)
	dst = appendFieldsKey(dst, c, cfg, normalized)
	// The body: for a QUERY (RFC 10008), any request with one, and a Content-Encoding c.Body() may answer with 501.
	if body, held := bodySegment(c, normalized); held {
		dst = appendBodyKey(dst, &c.Request().Header, body, normalized)
	}
	return dst
}

// appendRecordKey keys a refusal record so that every spelling of the URL and every body but a QUERY's reach it.
func appendRecordKey(dst []byte, c fiber.Ctx, cfg *Config) []byte {
	dst = appendURLKey(dst, c)
	if !cfg.DisableQueryKeys {
		dst = append(dst, '|', 'q', '=')
		dst = appendDecodedQueryString(dst, c.Req().URI())
	}
	normalized := headerlookup.Canonical(c)
	dst = appendFieldsKey(dst, c, cfg, normalized)
	if c.Method() == fiber.MethodQuery {
		body, _ := bodySegment(c, normalized)
		dst = appendBodyKey(dst, &c.Request().Header, body, normalized)
	}
	return dst
}

func appendURLKey(dst []byte, c fiber.Ctx) []byte {
	// The target URI, not only its path: a cache key is composed of the request
	// method and the target URI at a minimum (RFC 9111 §2), and the target URI
	// carries the scheme and the authority. Keyed on the path alone, two hosts
	// served by one app shared an entry: a page routed with app.Domain answered
	// the other host, and a Location built from c.BaseURL for one host was
	// served for the other.
	//
	// Read through c.Scheme and c.Host, which honor the X-Forwarded-* fields of
	// a trusted proxy. That is the authority app.Domain routes on and c.BaseURL
	// builds from, so the key splits along the dimension the response was built
	// on, and behind such a proxy every tenant may arrive under the backend's
	// own Host. Not normalized here: fasthttp lowercases the Host it parses, and
	// a case variant arriving through X-Forwarded-Host can only fragment the
	// cache, never merge two origins. Escaped and bounded like every other
	// segment, so a crafted Host cannot inject key structure or grow the key
	// without limit.
	dst = append(dst, "scheme="...)
	dst = appendEscapedBoundKeySegment(dst, c.Scheme())
	dst = append(dst, "|host="...)
	dst = appendEscapedBoundKeySegment(dst, c.Host())
	dst = append(dst, '|')

	// Escape delimiters in path to prevent crafted paths from injecting key structure
	return appendEscapedBoundKeySegment(dst, c.Path())
}

func appendFieldsKey(dst []byte, c fiber.Ctx, cfg *Config, normalized bool) []byte { //nolint:revive // flag-parameter: normalized is a property of the header store
	if len(cfg.KeyHeaders) > 0 {
		dst = append(dst, '|', 'h', '=')
		dst = appendCanonicalHeaderSubset(dst, &c.Request().Header, cfg.KeyHeaders, cfg.credentialKeyHeaders, normalized)
	}
	if len(cfg.KeyCookies) > 0 {
		dst = append(dst, '|', 'c', '=')
		dst = appendCanonicalCookieSubset(dst, c, cfg)
	}
	return dst
}

// appendBodyKey appends the body and the fields that say how it is decoded, unnamed as their order is fixed.
func appendBodyKey(dst []byte, header *fasthttp.RequestHeader, body []byte, normalized bool) []byte { //nolint:revive // flag-parameter: normalized is a property of the header store
	dst = append(dst, '|', 'm', '=')
	dst = appendKeyFieldLines(dst, header, "content-encoding", nil, normalized)
	dst = append(dst, '|')
	dst = appendKeyFieldLines(dst, header, "content-type", nil, normalized)
	dst = append(dst, '|', 'b', '=')
	return appendQueryBodySegment(dst, body)
}

// bodySegment returns the body and whether the key holds it, leaving unread a stream that declares none.
func bodySegment(c fiber.Ctx, normalized bool) ([]byte, bool) { //nolint:revive // flag-parameter: normalized is a property of the header store
	var body []byte
	if req := c.Request(); !req.IsBodyStream() || req.Header.ContentLength() > 0 {
		body = req.Body()
	}
	return body, len(body) > 0 || c.Method() == fiber.MethodQuery || namesContentEncoding(&c.Request().Header, normalized)
}

// preParsedMultipart reports whether fasthttp parsed the multipart form while reading it, keeping no copy of the body.
func preParsedMultipart(c fiber.Ctx) bool {
	req := c.Request()
	return req.Header.ContentLength() > 0 && !req.IsBodyStream() && !c.App().Server().DisablePreParseMultipartForm &&
		len(req.Header.MultipartFormBoundary()) > 0 && len(req.Header.Peek(fiber.HeaderContentEncoding)) == 0
}

func namesContentEncoding(header *fasthttp.RequestHeader, normalized bool) bool { //nolint:revive // flag-parameter: normalized is a property of the header store
	if normalized && header.ContentEncoding() == nil {
		return false
	}
	return len(keyFieldLines(header, "content-encoding", normalized)) > 0
}

// appendTargetAsSent keys what the raw target, which a handler can read, holds beyond the routed path and
// query: under "|p=" a differing target, "|f=" a fragment, "|n=" a target with no "?" given a query.
func appendTargetAsSent(dst, target []byte, path string, keyed []byte, cfg *Config) []byte {
	sent, fragment, hasFragment := utils.CutByte(target, '#')
	before, query, hasQuery := utils.CutByte(sent, '?')
	if hasQuery && (cfg.DisableQueryKeys || (len(query) > 0 && bytes.Equal(query, keyed))) {
		sent = before
	}

	switch {
	case !hasQuery && len(keyed) > 0 && !cfg.DisableQueryKeys:
		dst = append(dst, '|', 'n', '=')
		dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(sent))
	case string(sent) != path:
		dst = append(dst, '|', 'p', '=')
		dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(sent))
	}
	if hasFragment {
		dst = append(dst, '|', 'f', '=')
		dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(fragment))
	}
	return dst
}

type querySegment struct {
	start, end         uint16
	nameStart, nameEnd uint16
}

// A query whose parameters are sorted has to fit the 16 bits of those offsets.
const _ uint16 = maxQueryBufferSize

// appendCanonicalQueryString appends the query stably sorted by decoded name, each parameter kept as sent.
func appendCanonicalQueryString(dst []byte, uri *fasthttp.URI) []byte {
	return appendSortedQuery(dst, uri.QueryString(), false)
}

// appendDecodedQueryString appends the query sorted by name, decoded and escaped again: the same for every spelling.
func appendDecodedQueryString(dst []byte, uri *fasthttp.URI) []byte {
	return appendSortedQuery(dst, uri.QueryString(), true)
}

func appendSortedQuery(dst, raw []byte, decode bool) []byte { //nolint:revive // flag-parameter: the two forms share the sort
	if len(raw) == 0 {
		return dst
	}

	// Safe: the segment is consumed synchronously (appended/hashed) before the
	// request buffer can be mutated, so no stable copy is required.
	query := utils.UnsafeString(raw)

	// Past either bound the query is keyed as sent: they cap the work and keep the offsets in 16 bits.
	if len(query) > maxQueryBufferSize {
		return appendEscapedBoundKeySegment(dst, query)
	}
	count := strings.Count(query, "&") + 1
	if (count == 1 && !decode) || count > maxQueryParams {
		return appendEscapedBoundKeySegment(dst, query)
	}

	bufPtr := acquireKeyBuffer()
	buf := (*bufPtr)[:0]

	var array [maxQueryParams]querySegment
	segments := array[:count]
	start := 0
	for i := range segments {
		end := len(raw)
		if next := bytes.IndexByte(raw[start:], '&'); next >= 0 {
			end = start + next
		}
		name := raw[start:end]
		if eq := bytes.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		nameStart := len(buf)
		buf = fasthttp.AppendUnquotedArg(buf, name)
		segments[i] = querySegment{ //nolint:gosec // G115 - offsets into a query of at most maxQueryBufferSize bytes
			start: uint16(start), end: uint16(end), nameStart: uint16(nameStart), nameEnd: uint16(len(buf)),
		}
		start = end + 1
	}

	names := buf
	slices.SortStableFunc(segments, func(a, b querySegment) int {
		return bytes.Compare(names[a.nameStart:a.nameEnd], names[b.nameStart:b.nameEnd])
	})

	sorted := len(buf)
	for i, segment := range segments {
		if i > 0 {
			buf = append(buf, '&')
		}
		if decode {
			buf = appendDecodedParam(buf, names[segment.nameStart:segment.nameEnd], raw[segment.start:segment.end])
		} else {
			buf = append(buf, raw[segment.start:segment.end]...)
		}
	}

	// Escaped, as a parameter sent raw may hold a delimiter.
	dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(buf[sorted:]))
	releaseKeyBuffer(bufPtr, buf)
	return dst
}

// appendDecodedParam appends param as name=value, both escaped again: q=%41 and q=A, q and q= append the same.
func appendDecodedParam(dst, name, param []byte) []byte {
	var value []byte
	if _, raw, found := utils.CutByte(param, '='); found {
		var decoded [64]byte
		value = fasthttp.AppendUnquotedArg(decoded[:0], raw)
	}
	dst = utils.AppendQueryEscape(dst, utils.UnsafeString(name))
	dst = append(dst, '=')
	return utils.AppendQueryEscape(dst, utils.UnsafeString(value))
}

// appendCanonicalHeaderSubset appends the named request headers, hashing the values of credentials.
func appendCanonicalHeaderSubset(dst []byte, header *fasthttp.RequestHeader, names, credentials []string, normalized bool) []byte {
	for idx, name := range names {
		if idx > 0 {
			dst = append(dst, '|')
		}
		// Escape name (though names are normalized and trusted)
		dst = appendEscapedKeyDelimiters(dst, name)
		dst = append(dst, ':')
		dst = appendKeyFieldLines(dst, header, name, credentials, normalized)
	}

	return dst
}

// appendKeyFieldLines appends the field lines of name, counted, hashed when name is a credential.
func appendKeyFieldLines(dst []byte, header *fasthttp.RequestHeader, name string, credentials []string, normalized bool) []byte { //nolint:revive // flag-parameter: normalized is a property of the header store
	// Every field line, not just the first: the split and comma-joined forms are
	// equivalent (RFC 9110 §5.2), so a key header sent twice keyed like one sent
	// once and was served its cached response.
	values := keyFieldLines(header, name, normalized)

	// The count keeps the framing injective: without it an absent header and
	// an empty one both emit nothing, and a list could not be told from a
	// single value containing the separator.
	dst = utils.AppendInt(dst, int64(len(values)))

	// Each value is bounded, but not how many there are, and a few hundred field
	// lines build a multi-kilobyte map key. Hash past the bound; the hash covers
	// raw lines, which the verbatim form never is, so the two cannot collide. A
	// credential is hashed whatever its size.
	total := 0
	for _, value := range values {
		total += len(value) + 1
	}
	if total > maxKeyDimensionSegmentLength || (len(values) > 0 && slices.Contains(credentials, name)) {
		dst = append(dst, hashPrefix...)
		return appendAuthHash(dst, values)
	}

	for _, value := range values {
		dst = append(dst, '|')
		// Escape value to prevent delimiter injection
		dst = appendEscapedBoundKeySegment(dst, utils.UnsafeString(value))
	}
	return dst
}

// appendCanonicalCookieSubset appends the KeyCookies cookies, their values hashed like a credential
// header's; a missing or empty one leaves just its name.
func appendCanonicalCookieSubset(dst []byte, c fiber.Ctx, cfg *Config) []byte {
	for idx, name := range cfg.KeyCookies {
		if idx > 0 {
			dst = append(dst, '|')
		}
		// Escape name (though names are normalized and trusted)
		dst = appendEscapedKeyDelimiters(dst, name)
		dst = append(dst, ':')

		if value := c.Cookies(name); value != "" {
			dst = append(dst, hashPrefix...)
			dst = appendAuthHash(dst, [][]byte{utils.UnsafeBytes(value)})
		}
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
