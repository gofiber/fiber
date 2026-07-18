// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gofiber/utils/v2"
	utilsbytes "github.com/gofiber/utils/v2/bytes"
	"github.com/gofiber/utils/v2/swar"

	"github.com/gofiber/fiber/v3/internal/contextvalue"
	"github.com/gofiber/fiber/v3/log"

	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

// acceptedType is a struct that holds the parsed value of an Accept header
// along with quality, specificity, parameters, and order.
// Used for sorting accept headers.
type acceptedType struct {
	params      headerParams
	spec        string
	quality     float64
	specificity int
	order       int
}

const noCacheValue = "no-cache"

// Pre-allocated byte slices for accept header parsing
var (
	semicolonQEquals = []byte(";q=")
	wildcardAll      = []byte("*/*")
	wildcardSuffix   = []byte("/*")
)

type headerParams map[string][]byte

// ValueFromContext retrieves a value stored under key from supported context types.
//
// Supported context types:
//   - Ctx (including CustomCtx implementations)
//   - *fasthttp.RequestCtx
//   - context.Context
//   - any value exposing UserValue(key any) any or Value(key any) any
func ValueFromContext[T any](ctx, key any) (T, bool) {
	return contextvalue.Value[T](ctx, key)
}

// StoreInContext stores key/value in both Fiber locals and request context.
//
// This is useful when values need to be available via both c.Locals() and
// context.Context lookups throughout middleware and handlers.
func StoreInContext(c Ctx, key, value any) {
	c.Locals(key, value)

	if c.App().config.PassLocalsToContext {
		c.SetContext(context.WithValue(c.Context(), key, value))
	}
}

// getTLSConfig returns a net listener's tls config
func getTLSConfig(ln net.Listener) *tls.Config {
	if ln == nil {
		return nil
	}

	type tlsConfigProvider interface {
		TLSConfig() *tls.Config
	}

	type configProvider interface {
		Config() *tls.Config
	}

	if provider, ok := ln.(tlsConfigProvider); ok {
		return provider.TLSConfig()
	}

	if provider, ok := ln.(configProvider); ok {
		return provider.Config()
	}

	pointer := reflect.ValueOf(ln)
	if !pointer.IsValid() {
		return nil
	}

	// Reflection fallback for listeners that do not expose a TLS config method.
	val := reflect.Indirect(pointer)
	if !val.IsValid() {
		return nil
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	field := val.FieldByName("config")
	if !field.IsValid() {
		return nil
	}

	if field.Type() != reflect.TypeFor[*tls.Config]() {
		return nil
	}

	if field.CanInterface() {
		if cfg, ok := field.Interface().(*tls.Config); ok {
			return cfg
		}
		return nil
	}

	if !field.CanAddr() {
		return nil
	}

	value := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem() //nolint:gosec // Access to unexported field is required for listeners that don't expose TLS config methods.
	if !value.IsValid() {
		return nil
	}

	cfg, ok := value.Interface().(*tls.Config)
	if !ok {
		return nil
	}

	return cfg
}

// readContent opens a named file and read content from it
func readContent(rf io.ReaderFrom, name string) (int64, error) {
	// Read file
	f, err := os.Open(filepath.Clean(name))
	if err != nil {
		return 0, fmt.Errorf("failed to open: %w", err)
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Errorf("Error closing file: %s", err)
		}
	}()
	n, readErr := rf.ReadFrom(f)
	if readErr != nil {
		return n, fmt.Errorf("failed to read: %w", readErr)
	}
	return n, nil
}

// quoteEscapeMask marks the lanes of w holding bytes quoteRawString must
// escape: '\\', '"', any C0 control (including HTAB), or DEL. Lanes >= 0x80
// are never marked; non-ASCII bytes pass through verbatim. This is
// utils.IndexNonQuotable's RFC 9110 set widened by HTAB, which the RFC
// permits as qdtext but this function has always percent-encoded.
func quoteEscapeMask(w uint64) uint64 {
	return swar.MatchByteMask(w, '\\') | swar.MatchByteMask(w, '"') |
		swar.MatchRangeMask(w, 0x00, 0x1f) | swar.MatchByteMask(w, 0x7f)
}

// indexQuoteEscape returns the index of the first byte quoteEscapeMask
// matches, or -1 if raw needs no escaping. It scans eight bytes at a time,
// finishing inputs of 8+ bytes with one overlapping word; shorter inputs
// are checked byte-wise.
func indexQuoteEscape(raw string) int {
	n := len(raw)
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		if m := quoteEscapeMask(swar.Load8(raw, i)); m != 0 {
			return i + swar.FirstLane(m)
		}
	}
	if i == n {
		return -1
	}
	if n >= swar.WordLen {
		if m := quoteEscapeMask(swar.Load8(raw, n-swar.WordLen)); m != 0 {
			return n - swar.WordLen + swar.FirstLane(m)
		}
		return -1
	}
	for ; i < n; i++ {
		if c := raw[i]; c == '\\' || c == '"' || c < 0x20 || c == 0x7f {
			return i
		}
	}
	return -1
}

// quoteRawString escapes the characters that need quoting inside an RFC 9110
// quoted-string (https://www.rfc-editor.org/rfc/rfc9110#section-5.6.4), plus
// HTAB, which the RFC permits as qdtext but this function has always
// percent-encoded. The result may contain non-ASCII bytes.
func (*App) quoteRawString(raw string) string {
	// Fast path: most values need no escaping at all; avoid the pooled
	// buffer and the string allocation entirely.
	end := indexQuoteEscape(raw)
	if end == -1 {
		return raw
	}

	const hex = "0123456789ABCDEF"
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	// Every byte before end is quotable and tab-free, so it hits the
	// verbatim case of the switch below; copy it in one append.
	bb.B = append(bb.B, raw[:end]...)
	for i := end; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\' || c == '"':
			// escape backslash and quote
			bb.B = append(bb.B, '\\', c)
		case c == '\n':
			bb.B = append(bb.B, '\\', 'n')
		case c == '\r':
			bb.B = append(bb.B, '\\', 'r')
		case c < 0x20 || c == 0x7f:
			// percent-encode control and DEL
			bb.B = append(
				bb.B,
				'%',
				hex[c>>4],
				hex[c&0x0f],
			)
		default:
			bb.B = append(bb.B, c)
		}
	}

	return string(bb.B)
}

// appendLowerASCII writes the ASCII-lowercased bytes of src into dst[:0],
// growing dst as needed, in a single pass over src (instead of a copy
// followed by an in-place case fold). Bytes outside 'A'..'Z', including
// non-ASCII, are copied unchanged. src and dst must not overlap.
func appendLowerASCII(dst, src []byte) []byte {
	n := len(src)
	// Amortized growth like append: every byte of dst[:n] is overwritten
	// below, so the grown slice's contents don't matter.
	dst = slices.Grow(dst[:0], n)[:n]
	i := 0
	for ; i+swar.WordLen <= n; i += swar.WordLen {
		swar.Store8(dst, i, swar.ToLowerWord(swar.Load8(src, i)))
	}
	if i == n {
		return dst
	}
	if n >= swar.WordLen {
		// Finish with one overlapping word; the overlapped bytes are
		// rewritten with the same values.
		swar.Store8(dst, n-swar.WordLen, swar.ToLowerWord(swar.Load8(src, n-swar.WordLen)))
		return dst
	}
	for ; i < n; i++ {
		c := src[i]
		if c-'A' <= 'Z'-'A' {
			c |= 0x20
		}
		dst[i] = c
	}
	return dst
}

// defaultString returns the value or a default value if it is set
func defaultString(value string, defaultValue []string) string {
	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return value
}

func getGroupPath(prefix, path string) string {
	if path == "" {
		return prefix
	}

	if path[0] != '/' {
		path = "/" + path
	}

	return utils.TrimRight(prefix, '/') + path
}

// Match specificity levels returned by the acceptsX helpers. A higher value
// means the range matched the offer more specifically. They only need to be
// ordered consistently within a single helper (each getOffer call uses one
// helper), so an explicit q=0 rejection can override a positive match of the
// same or lower specificity per RFC 9110 §12.5.1.
const (
	matchWildcard = 1 // "*" / trailing-"*" prefix / language "*"
	matchPrefix   = 2 // language subtag prefix, e.g. "en" matching "en-US"
	matchExact    = 3 // exact, case-insensitive match

	// Media ranges rank by their coarse class first and by the number of
	// matched media-type parameters second, so "text/html;level=1" outranks
	// "text/html", which outranks "text/*", which outranks "*/*".
	matchMediaAny         = 1 // "*/*"
	matchMediaTypeAny     = 2 // "type/*"
	matchMediaTypeSubtype = 3 // "type/subtype"
	mediaSpecificityScale = 100
)

// acceptsOffer determines if an offer matches a given specification.
// It supports a trailing '*' wildcard and performs case-insensitive exact matching.
// It returns the match specificity (0 = no match, higher = more specific): a
// wildcard/prefix match is less specific than an exact match. The specificity is
// used to let an explicit q=0 rejection override a less specific positive match
// of the same coarse class (RFC 9110 §12.5.1).
func acceptsOffer(spec, offer string, _ headerParams) int {
	if len(spec) >= 1 && spec[len(spec)-1] == '*' {
		if utils.HasPrefixFold(offer, spec[:len(spec)-1]) {
			return matchWildcard
		}
		return 0
	}

	if utils.EqualFold(spec, offer) {
		return matchExact
	}
	return 0
}

// acceptsLanguageOfferBasic determines if a language tag offer matches a range
// according to RFC 4647 Basic Filtering.
// A match occurs if the range exactly equals the tag or is a prefix of the tag
// followed by a hyphen. The comparison is case-insensitive. Only a single "*"
// as the entire range is allowed. Any "*" appearing after a hyphen renders the
// range invalid and will not match.
// It returns the match specificity (0 = no match): "*" is least specific, a
// prefix match ("en" for "en-US") is more specific, and an exact match is most
// specific — so an explicit "en-US;q=0" can override a positive "en".
func acceptsLanguageOfferBasic(spec, offer string, _ headerParams) int {
	if spec == "*" {
		return matchWildcard
	}
	if strings.IndexByte(spec, '*') >= 0 {
		return 0
	}
	if utils.EqualFold(spec, offer) {
		return matchExact
	}
	if len(offer) > len(spec) &&
		utils.HasPrefixFold(offer, spec) &&
		offer[len(spec)] == '-' {
		return matchPrefix
	}
	return 0
}

// acceptsLanguageOfferExtended determines if a language tag offer matches a
// range according to RFC 4647 Extended Filtering (§3.3.2).
// - Case-insensitive comparisons
// - '*' matches zero or more subtags (can "slide")
// - Unspecified subtags are treated like '*' (so trailing/extraneous tag subtags are fine)
// - Matching fails if sliding encounters a singleton (incl. 'x')
// It returns the match specificity (0 = no match): a bare "*" is least specific,
// and otherwise the specificity grows with the number of concrete range subtags
// that had to match, so a deeper range (e.g. "en-US") outranks a shorter one
// ("en") and an explicit "en-US;q=0" can override a positive "en".
func acceptsLanguageOfferExtended(spec, offer string, _ headerParams) int {
	if spec == "*" {
		return matchWildcard
	}
	if spec == "" || offer == "" {
		return 0
	}

	// Use stack-allocated arrays to avoid heap allocations for typical language tags
	var rsBuf, tsBuf [8]string
	rs := rsBuf[:0]
	ts := tsBuf[:0]

	// Parse spec subtags without allocation for typical cases
	for s := range strings.SplitSeq(spec, "-") {
		rs = append(rs, s)
	}
	// Parse offer subtags without allocation for typical cases
	for s := range strings.SplitSeq(offer, "-") {
		ts = append(ts, s)
	}

	// Step 2: first subtag must match (or be '*')
	if rs[0] != "*" && !utils.EqualFold(rs[0], ts[0]) {
		return 0
	}

	i, j := 1, 1 // i = range index, j = tag index
	for i < len(rs) {
		if rs[i] == "*" { // 3.A: '*' matches zero or more subtags
			i++
			continue
		}
		if j >= len(ts) { // 3.B: ran out of tag subtags
			return 0
		}
		if utils.EqualFold(rs[i], ts[j]) { // 3.C: exact subtag match
			i++
			j++
			continue
		}
		// 3.D: singleton barrier (one letter or digit, incl. 'x')
		if len(ts[j]) == 1 {
			return 0
		}
		// 3.E: slide forward in the tag and try again
		j++
	}

	// 4: matched all range subtags. Rank by the number of concrete (non-"*")
	// range subtags so a more specific range wins the specificity comparison.
	specificity := matchWildcard
	for _, sub := range rs {
		if sub != "*" {
			specificity++
		}
	}
	return specificity
}

// acceptsOfferType This function determines if an offer type matches a given specification.
// It checks if the specification is equal to */* (i.e., all types are accepted).
// It gets the MIME type of the offer (either from the offer itself or by its file extension).
// It checks if the offer MIME type matches the specification MIME type or if the specification is of the form <MIME_type>/* and the offer MIME type has the same MIME type.
// It checks if the offer contains every parameter present in the specification.
// It returns the match specificity (0 = no match): "*/*" is least specific,
// then "type/*", then "type/subtype", and matched media-type parameters break
// ties so "text/html;level=1" outranks "text/html" (letting "text/html;level=1;q=0"
// override a positive "text/html").
func acceptsOfferType(spec, offerType string, specParams headerParams) int {
	var offerMime, offerParams string

	if i := strings.IndexByte(offerType, ';'); i == -1 {
		offerMime = offerType
	} else {
		offerMime = offerType[:i]
		offerParams = offerType[i:]
	}

	// Accept: */*
	if spec == "*/*" {
		return mediaMatchSpecificity(matchMediaAny, specParams, offerParams)
	}

	var mimetype string
	if strings.IndexByte(offerMime, '/') != -1 {
		mimetype = offerMime // MIME type
	} else {
		mimetype = utils.GetMIME(offerMime) // extension
	}

	if utils.EqualFold(spec, mimetype) {
		// Accept: <MIME_type>/<MIME_subtype>
		return mediaMatchSpecificity(matchMediaTypeSubtype, specParams, offerParams)
	}

	s := strings.IndexByte(mimetype, '/')
	specSlash := strings.IndexByte(spec, '/')
	// Accept: <MIME_type>/*
	if s != -1 && specSlash != -1 {
		if utils.EqualFold(spec[:specSlash], mimetype[:s]) && (spec[specSlash:] == "/*" || mimetype[s:] == "/*") {
			return mediaMatchSpecificity(matchMediaTypeAny, specParams, offerParams)
		}
	}

	return 0
}

// mediaMatchSpecificity returns the specificity of a media range that matched an
// offer's type/subtype, or 0 when the media-type parameters don't match. The
// coarse class dominates; the count of matched parameters breaks ties.
func mediaMatchSpecificity(base int, specParams headerParams, offerParams string) int {
	if !paramsMatch(specParams, offerParams) {
		return 0
	}
	return base*mediaSpecificityScale + len(specParams)
}

// paramsMatch returns whether offerParams contains all parameters present in specParams.
// Matching is case-insensitive, and surrounding quotes are stripped.
// To align with the behavior of res.format from Express, the order of parameters is
// ignored, and if a parameter is specified twice in the incoming Accept, the last
// provided value is given precedence.
// In the case of quoted values, RFC 9110 says that we must treat any character escaped
// by a backslash as equivalent to the character itself (e.g., "a\aa" is equivalent to "aaa").
// For the sake of simplicity, we forgo this and compare the value as-is. Besides, it would
// be highly unusual for a client to escape something other than a double quote or backslash.
// See https://www.rfc-editor.org/rfc/rfc9110#name-parameters
func paramsMatch(specParamStr headerParams, offerParams string) bool {
	if len(specParamStr) == 0 {
		return true
	}

	allSpecParamsMatch := true
	for specParam, specVal := range specParamStr {
		foundParam := false
		fasthttp.VisitHeaderParams(utils.UnsafeBytes(offerParams), func(key, value []byte) bool {
			if utils.EqualFold(specParam, utils.UnsafeString(key)) {
				foundParam = true
				unescaped, err := unescapeHeaderValue(value)
				if err != nil {
					allSpecParamsMatch = false
					return false
				}
				allSpecParamsMatch = utils.EqualFold(specVal, unescaped)
				return false
			}
			return true
		})
		if !foundParam || !allSpecParamsMatch {
			return false
		}
	}

	return allSpecParamsMatch
}

// getSplicedStrList function takes a string and a string slice as an argument, divides the string into different
// elements divided by ',' and stores these elements in the string slice.
// It returns the populated string slice as an output.
//
// Empty list elements are parsed and ignored, as required by
// RFC 9110 Section 5.6.1.2 for all comma-separated field values.
//
// If the given slice hasn't enough space, it will allocate more and return.
func getSplicedStrList(headerValue string, dst []string) []string {
	if headerValue == "" {
		return nil
	}

	dst = dst[:0]
	segmentStart := 0
	for i := 0; i < len(headerValue); i++ {
		if headerValue[i] == ',' {
			if segment := utils.TrimSpace(headerValue[segmentStart:i]); segment != "" {
				dst = append(dst, segment)
			}
			segmentStart = i + 1
		}
	}
	if segment := utils.TrimSpace(headerValue[segmentStart:]); segment != "" {
		dst = append(dst, segment)
	}

	return dst
}

func joinHeaderValues(headers [][]byte) []byte {
	switch len(headers) {
	case 0:
		return nil
	case 1:
		return headers[0]
	default:
		return bytes.Join(headers, []byte{','})
	}
}

// joinedHeaderValue accumulates the combined value of a header's field lines
// (RFC 9110 Section 5.2). It allocates only in the rare multi-line case; the
// single-line result aliases the header storage.
type joinedHeaderValue struct {
	key      string
	combined []byte
	multi    bool
}

func (j *joinedHeaderValue) visit(k, v []byte) {
	if len(k) != len(j.key) || !utils.EqualFold(utils.UnsafeString(k), j.key) {
		return
	}
	switch {
	case j.combined == nil:
		j.combined = v
	case !j.multi:
		joined := make([]byte, 0, len(j.combined)+1+len(v))
		joined = append(joined, j.combined...)
		joined = append(joined, ',')
		joined = append(joined, v...)
		j.combined = joined
		j.multi = true
	default:
		j.combined = append(j.combined, ',')
		j.combined = append(j.combined, v...)
	}
}

// peekJoinedRequestHeader returns the combined value of every field line for
// key in a single pass over the request headers, plus whether the field
// occurred on more than one line. Unlike PeekAll it performs no per-call key
// normalization. Concrete (non-generic) so the visitor stays on the stack.
func peekJoinedRequestHeader(h *fasthttp.RequestHeader, key string) ([]byte, bool) {
	j := joinedHeaderValue{key: key}
	// VisitAll (not the replacement All) keeps this zero-alloc: All returns
	// an iterator closure that escapes to the heap on every call. The SA1019
	// deprecation is suppressed for helpers.go in .golangci.yml.
	h.VisitAll(j.visit)
	return j.combined, j.multi
}

// peekJoinedResponseHeader is peekJoinedRequestHeader for response headers.
func peekJoinedResponseHeader(h *fasthttp.ResponseHeader, key string) ([]byte, bool) {
	j := joinedHeaderValue{key: key}
	// VisitAll (not the replacement All) keeps this zero-alloc: All returns
	// an iterator closure that escapes to the heap on every call. The SA1019
	// deprecation is suppressed for helpers.go in .golangci.yml.
	h.VisitAll(j.visit)
	return j.combined, j.multi
}

func unescapeHeaderValue(v []byte) ([]byte, error) {
	if bytes.IndexByte(v, '\\') == -1 {
		return v, nil
	}
	res := make([]byte, 0, len(v))
	escaping := false
	for i, c := range v {
		if escaping {
			res = append(res, c)
			escaping = false
			continue
		}
		if c == '\\' {
			// invalid escape at end of string
			if i == len(v)-1 {
				return nil, errInvalidEscapeSequence
			}
			escaping = true
			continue
		}
		res = append(res, c)
	}
	if escaping {
		return nil, errInvalidEscapeSequence
	}
	return res, nil
}

// forEachMediaRange parses an Accept or Content-Type header, calling functor
// on each media range.
// See: https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation-fields
func forEachMediaRange(header []byte, functor func([]byte)) {
	hasDQuote := bytes.IndexByte(header, '"') != -1

	for len(header) > 0 {
		n := 0
		header = utils.TrimLeft(header, ' ')
		quotes := 0

		if hasDQuote {
			// Complex case. We need to keep track of quotes and quoted-pairs (i.e.,  characters escaped with \ )
			// Only ',', '"' and '\\' can change state, so jump between them
			// instead of visiting every byte.
		loop:
			for n < len(header) {
				i := utils.IndexAny3(header[n:], ',', '"', '\\')
				if i == -1 {
					n = len(header)
					break
				}
				n += i
				switch header[n] {
				case ',':
					if quotes%2 == 0 {
						break loop
					}
				case '"':
					quotes++
				default: // '\\'
					if quotes%2 == 1 && n+1 < len(header) {
						// A quoted-pair escapes exactly the next byte
						// (RFC 9110 §5.6.4); consume it so an escaped
						// quote is not mistaken for a closing quote.
						n++
					}
				}
				n++
			}
		} else {
			// Simple case. Just look for the next comma.
			if n = bytes.IndexByte(header, ','); n == -1 {
				n = len(header)
			}
		}

		functor(header[:n])

		if n >= len(header) {
			return
		}
		header = header[n+1:]
	}
}

// Pool for headerParams instances. The headerParams object *must*
// be cleared before being returned to the pool.
var headerParamPool = sync.Pool{
	New: func() any {
		return make(headerParams)
	},
}

// getOffer return valid offer for header negotiation.
func getOffer(header []byte, isAccepted func(spec, offer string, specParams headerParams) int, offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	if len(header) == 0 {
		return offers[0]
	}

	acceptedTypes := make([]acceptedType, 0, 8)
	order := 0
	// Whether any range carries an explicit q=0 rejection. When none do, the
	// more-specific-rejection scan can be skipped entirely on the hot path.
	hasRejections := false

	// Parse header and get accepted types with their quality and specificity
	// See: https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation-fields
	forEachMediaRange(header, func(accept []byte) {
		order++
		spec, quality := accept, 1.0
		var params headerParams

		if i := bytes.IndexByte(accept, ';'); i != -1 {
			spec = accept[:i]

			// Optimized quality parsing
			qIndex := i + 3
			if bytes.HasPrefix(accept[i:], semicolonQEquals) && bytes.IndexByte(accept[qIndex:], ';') == -1 {
				if q, err := fasthttp.ParseUfloat(accept[qIndex:]); err == nil {
					quality = q
				}
			} else {
				params, _ = headerParamPool.Get().(headerParams) //nolint:errcheck // only contains headerParams
				for k := range params {
					delete(params, k)
				}
				fasthttp.VisitHeaderParams(accept[i:], func(key, value []byte) bool {
					// The weight parameter name "q" is case-insensitive
					// (RFC 9110 §12.4.2).
					if len(key) == 1 && (key[0] == 'q' || key[0] == 'Q') {
						if q, err := fasthttp.ParseUfloat(value); err == nil {
							quality = q
						}
						return false
					}
					lowerKey := utils.UnsafeString(utilsbytes.UnsafeToLower(key))
					val, err := unescapeHeaderValue(value)
					if err != nil {
						return true
					}
					params[lowerKey] = val
					return true
				})
			}
		}

		spec = utils.TrimSpace(spec)

		// Determine specificity
		var specificity int

		// check for wildcard this could be a mime */* or a wildcard character *
		switch {
		case len(spec) == 1 && spec[0] == '*':
			specificity = 1
		case bytes.Equal(spec, wildcardAll):
			specificity = 1
		case bytes.HasSuffix(spec, wildcardSuffix):
			specificity = 2
		case bytes.IndexByte(spec, '/') != -1:
			specificity = 3
		default:
			specificity = 4
		}

		if quality == 0 {
			hasRejections = true
		}

		// Add to accepted types
		acceptedTypes = append(acceptedTypes, acceptedType{
			spec:        utils.UnsafeString(spec),
			quality:     quality,
			specificity: specificity,
			order:       order,
			params:      params,
		})
	})

	if len(acceptedTypes) > 1 {
		// Sort accepted types by quality and specificity, preserving order of equal elements
		sortAcceptedTypes(acceptedTypes)
	}

	// Find the best offer that matches the accepted types.
	//
	// Per RFC 9110 §12.5.1 the most specific matching media range determines an
	// offer's acceptability, and a quality of 0 means the client explicitly
	// rejects that range. An offer is therefore only acceptable if its most
	// specific matching range has a quality greater than 0 — a broader range
	// with a higher quality (e.g. "*" or "text/*") must not override a more
	// specific q=0 rejection.
	// See: https://www.rfc-editor.org/rfc/rfc9110#section-12.5.1
	result := ""
	if !hasRejections {
		// Fast path: without any q=0 rejection this is the plain "first matching
		// range in preference order wins" selection, identical to the algorithm
		// before q=0 handling, so there is no need to compute or compare match
		// specificity.
	selectFast:
		for _, acceptedType := range acceptedTypes {
			for _, offer := range offers {
				if offer != "" && isAccepted(acceptedType.spec, offer, acceptedType.params) > 0 {
					result = offer
					break selectFast
				}
			}
		}
	} else {
		// Rejection-aware path: an offer is only acceptable if its matching range
		// is not overridden by a q=0 range that matches it at least as
		// specifically (RFC 9110 §12.5.1).
	selectWithRejections:
		for _, acceptedType := range acceptedTypes {
			if acceptedType.quality == 0 {
				// A q=0 range never selects an offer; it can only reject one.
				continue
			}
			for _, offer := range offers {
				if offer == "" {
					continue
				}
				matchSpecificity := isAccepted(acceptedType.spec, offer, acceptedType.params)
				if matchSpecificity > 0 &&
					!rejectedByMoreSpecificRange(acceptedTypes, isAccepted, offer, matchSpecificity) {
					result = offer
					break selectWithRejections
				}
			}
		}
	}

	for i := range acceptedTypes {
		if acceptedTypes[i].params != nil {
			headerParamPool.Put(acceptedTypes[i].params)
		}
	}

	return result
}

// rejectedByMoreSpecificRange reports whether a q=0 range matches the offer at
// least as specifically as the positive match at baseSpecificity, i.e. the
// client explicitly rejected the offer per RFC 9110 §12.5.1. Comparing the
// effective match specificity (rather than the coarse parsed bucket) lets a
// same-class rejection win — e.g. "en-US;q=0" over "en", "utf-8;q=0" over an
// earlier "utf-8", or "text/html;level=1;q=0" over "text/html" — while a less
// specific rejection such as "text/*;q=0" still does not override "text/html".
func rejectedByMoreSpecificRange(types []acceptedType, isAccepted func(spec, offer string, specParams headerParams) int, offer string, baseSpecificity int) bool {
	for i := range types {
		if types[i].quality == 0 &&
			isAccepted(types[i].spec, offer, types[i].params) >= baseSpecificity {
			return true
		}
	}
	return false
}

// sortAcceptedTypes sorts accepted types by quality and specificity, preserving order of equal elements
// A type with parameters has higher priority than an equivalent one without parameters.
// e.g., text/html;a=1;b=2 comes before text/html;a=1
// See: https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation-fields
func sortAcceptedTypes(at []acceptedType) {
	for i := 1; i < len(at); i++ {
		lo, hi := 0, i-1
		for lo <= hi {
			mid := (lo + hi) / 2
			if at[i].quality < at[mid].quality ||
				(at[i].quality == at[mid].quality && at[i].specificity < at[mid].specificity) ||
				(at[i].quality == at[mid].quality && at[i].specificity == at[mid].specificity && len(at[i].params) < len(at[mid].params)) ||
				(at[i].quality == at[mid].quality && at[i].specificity == at[mid].specificity && len(at[i].params) == len(at[mid].params) && at[i].order > at[mid].order) {
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		for j := i; j > lo; j-- {
			at[j-1], at[j] = at[j], at[j-1]
		}
	}
}

// normalizeEtag validates an entity tag and returns the
// value without quotes. weak is true if the tag has the "W/" prefix.
func normalizeEtag(t string) (value string, weak, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the parsed ETag components
	weak = strings.HasPrefix(t, "W/")
	if weak {
		t = t[2:]
	}

	if len(t) < 2 || t[0] != '"' || t[len(t)-1] != '"' {
		return "", weak, false
	}
	return t[1 : len(t)-1], weak, true
}

// matchEtag performs a weak comparison of entity tags according to
// RFC 9110 §8.8.3.2. The weak indicator ("W/") is ignored, but both tags must
// be properly quoted. Invalid tags result in a mismatch.
func matchEtag(s, etag string) bool {
	n1, _, ok1 := normalizeEtag(s)
	n2, _, ok2 := normalizeEtag(etag)
	if !ok1 || !ok2 {
		return false
	}

	return n1 == n2
}

// matchEtagStrong performs a strong entity-tag comparison following
// RFC 9110 §8.8.3.1. A weak tag never matches a strong one, even if the quoted
// values are identical.
func matchEtagStrong(s, etag string) bool {
	n1, w1, ok1 := normalizeEtag(s)
	n2, w2, ok2 := normalizeEtag(etag)
	if !ok1 || !ok2 || w1 || w2 {
		return false
	}

	return n1 == n2
}

// isEtagStale reports whether a response with the given ETag would be considered
// stale when presented with the raw If-None-Match header value. Comparison is
// weak as defined by RFC 9110 §8.8.3.2.
func (app *App) isEtagStale(etag string, noneMatchBytes []byte) bool {
	header := utils.TrimSpace(app.toString(noneMatchBytes))

	// Short-circuit the wildcard case: "*" never counts as stale.
	if header == "*" {
		return false
	}

	// Split the header on commas that sit outside DQUOTE-delimited opaque-tags:
	// etagc permits "," inside the quoted tag (RFC 9110 §8.8.3), so `"v1,v2"`
	// is a single entity tag, not two list elements. Only '"' and ','
	// affect the split, so jump between them instead of visiting every byte.
	start := 0
	pos := 0
	inQuotes := false
	for {
		i := utils.IndexAny2(header[pos:], '"', ',')
		if i == -1 {
			break
		}
		i += pos
		pos = i + 1
		if header[i] == '"' {
			inQuotes = !inQuotes
		} else if !inQuotes {
			if matchEtag(utils.TrimSpace(header[start:i]), etag) {
				return false
			}
			start = i + 1
		}
	}

	return !matchEtag(utils.TrimSpace(header[start:]), etag)
}

func parseAddr(raw string) (host, port string) { //nolint:nonamedreturns // gocritic unnamedResult requires naming host and port parts for clarity
	if raw == "" {
		return "", ""
	}

	raw = utils.TrimSpace(raw)

	// Handle IPv6 addresses enclosed in brackets as defined by RFC 3986
	if strings.HasPrefix(raw, "[") {
		if end := strings.IndexByte(raw, ']'); end != -1 {
			host = raw[:end+1] // keep the closing ]
			if len(raw) > end+1 && raw[end+1] == ':' {
				return host, raw[end+2:]
			}
			return host, ""
		}
	}

	// Everything else with a colon
	if i := strings.LastIndexByte(raw, ':'); i != -1 {
		host, port = raw[:i], raw[i+1:]

		// If “host” still contains ':', we must have hit an un-bracketed IPv6
		// literal. In that form a port is impossible, so treat the whole thing
		// as host.
		if strings.IndexByte(host, ':') >= 0 {
			return raw, ""
		}
		return host, port
	}

	// No colon, nothing to split
	return raw, ""
}

// isNoCache checks if the cacheControl header value contains a `no-cache` directive.
// Per RFC 9111 §5.2.2.4, no-cache can appear as either:
// - "no-cache" (applies to entire response)
// - "no-cache=field-name" (applies to specific header field)
// Both forms indicate the response should not be served from cache without revalidation.
func isNoCache(cacheControl string) bool {
	n := len(cacheControl)
	if n < len(noCacheValue) {
		return false
	}

	const noCacheLen = len(noCacheValue)
	const asciiCaseFold = byte(0x20)
	for i := 0; i <= n-noCacheLen; i++ {
		if (cacheControl[i] | asciiCaseFold) != 'n' {
			continue
		}
		if !matchNoCacheToken(cacheControl, i) {
			continue
		}
		if i > 0 && !isNoCacheDelimiter(cacheControl[i-1]) {
			continue
		}

		// Handle: "no-cache", "no-cache, ...", "no-cache=...", "no-cache ,"
		if i+noCacheLen == n {
			return true
		}
		if isNoCacheDelimiter(cacheControl[i+noCacheLen]) || cacheControl[i+noCacheLen] == '=' {
			return true
		}
	}

	return false
}

func isNoCacheDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == ','
}

func matchNoCacheToken(s string, i int) bool {
	// ASCII-only case-insensitive compare for "no-cache".
	const asciiCaseFold = byte(0x20)
	b := s[i:]

	return (b[0]|asciiCaseFold) == 'n' &&
		(b[1]|asciiCaseFold) == 'o' &&
		b[2] == '-' &&
		(b[3]|asciiCaseFold) == 'c' &&
		(b[4]|asciiCaseFold) == 'a' &&
		(b[5]|asciiCaseFold) == 'c' &&
		(b[6]|asciiCaseFold) == 'h' &&
		(b[7]|asciiCaseFold) == 'e'
}

var errTestConnClosed = errors.New("testConn is closed")

type testConn struct {
	r        bytes.Buffer
	w        bytes.Buffer
	isClosed bool
	sync.Mutex
}

// Read implements net.Conn by reading from the buffered input.
func (c *testConn) Read(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	return c.r.Read(b) //nolint:wrapcheck // This must not be wrapped
}

// Write implements net.Conn by appending to the buffered output.
func (c *testConn) Write(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	if c.isClosed {
		return 0, errTestConnClosed
	}
	return c.w.Write(b) //nolint:wrapcheck // This must not be wrapped
}

// Close marks the connection as closed and prevents further writes.
func (c *testConn) Close() error {
	c.Lock()
	defer c.Unlock()

	c.isClosed = true
	return nil
}

// LocalAddr implements net.Conn and returns a placeholder address.
func (*testConn) LocalAddr() net.Addr { return &net.TCPAddr{Port: 0, Zone: "", IP: net.IPv4zero} }

// RemoteAddr implements net.Conn and returns a placeholder address.
func (*testConn) RemoteAddr() net.Addr { return &net.TCPAddr{Port: 0, Zone: "", IP: net.IPv4zero} }

// SetDeadline implements net.Conn but is a no-op for the in-memory connection.
func (*testConn) SetDeadline(_ time.Time) error { return nil }

// SetReadDeadline implements net.Conn but is a no-op for the in-memory connection.
func (*testConn) SetReadDeadline(_ time.Time) error { return nil }

// SetWriteDeadline implements net.Conn but is a no-op for the in-memory connection.
func (*testConn) SetWriteDeadline(_ time.Time) error { return nil }

func toStringImmutable(b []byte) string {
	return string(b)
}

// HTTP methods and their unique INTs
func (app *App) methodInt(s string) int {
	// For better performance
	if len(app.configured.RequestMethods) == 0 {
		switch s {
		case MethodGet:
			return methodGet
		case MethodHead:
			return methodHead
		case MethodPost:
			return methodPost
		case MethodPut:
			return methodPut
		case MethodDelete:
			return methodDelete
		case MethodConnect:
			return methodConnect
		case MethodOptions:
			return methodOptions
		case MethodTrace:
			return methodTrace
		case MethodPatch:
			return methodPatch
		case MethodQuery:
			return methodQuery
		default:
			return -1
		}
	}
	// For method customization
	return slices.Index(app.config.RequestMethods, s)
}

func (app *App) method(methodInt int) string {
	// methodInt is -1 for methods not registered in RequestMethods (the
	// router responds 501 before dispatch, but contexts acquired directly
	// via AcquireCtx can still carry one); never index with it.
	if methodInt < 0 || methodInt >= len(app.config.RequestMethods) {
		return ""
	}
	return app.config.RequestMethods[methodInt]
}

// IsMethodSafe reports whether the HTTP method is considered safe.
// See https://datatracker.ietf.org/doc/html/rfc9110#section-9.2.1
func IsMethodSafe(m string) bool {
	switch m {
	case MethodGet,
		MethodHead,
		MethodOptions,
		MethodTrace,
		MethodQuery:
		return true
	default:
		return false
	}
}

// IsMethodIdempotent reports whether the HTTP method is considered idempotent.
// See https://datatracker.ietf.org/doc/html/rfc9110#section-9.2.2
func IsMethodIdempotent(m string) bool {
	if IsMethodSafe(m) {
		return true
	}

	switch m {
	case MethodPut, MethodDelete:
		return true
	default:
		return false
	}
}

// Convert a string value to a specified type, handling errors and optional default values.
func Convert[T any](value string, converter func(string) (T, error), defaultValue ...T) (T, error) {
	converted, err := converter(value)
	if err != nil {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}

		return converted, fmt.Errorf("failed to convert: %w", err)
	}

	return converted, nil
}

var (
	errParsedEmptyString = errors.New("parsed result is empty string")
	errParsedEmptyBytes  = errors.New("parsed result is empty bytes")
	errParsedType        = errors.New("unsupported generic type")
	// errParseValue flags a failed numeric/bool parse; callers only test err != nil.
	errParseValue = errors.New("failed to parse value")
)

// genericParseType parses str into V. Parse failures return the static errParseValue
// sentinel: the error is never surfaced (callers only test err != nil), so a flat
// sentinel is enough and avoids a per-call fmt.Errorf alloc on the hot default path.
func genericParseType[V GenericType](str string) (V, error) {
	var v V
	switch any(v).(type) {
	case int:
		result, err := utils.ParseInt(str)
		if err != nil {
			return v, errParseValue
		}
		return any(int(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int8:
		result, err := utils.ParseInt8(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int16:
		result, err := utils.ParseInt16(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int32:
		result, err := utils.ParseInt32(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int64:
		result, err := utils.ParseInt(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint:
		result, err := utils.ParseUint(str)
		if err != nil {
			return v, errParseValue
		}
		return any(uint(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint8:
		result, err := utils.ParseUint8(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint16:
		result, err := utils.ParseUint16(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint32:
		result, err := utils.ParseUint32(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint64:
		result, err := utils.ParseUint(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float32:
		result, err := utils.ParseFloat32(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float64:
		result, err := utils.ParseFloat64(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case bool:
		result, err := strconv.ParseBool(str)
		if err != nil {
			return v, errParseValue
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case string:
		if str == "" {
			return v, errParsedEmptyString
		}
		return any(str).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case []byte:
		if str == "" {
			return v, errParsedEmptyBytes
		}
		return any([]byte(str)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	default:
		return v, errParsedType
	}
}

// GenericType enumerates the values that can be parsed from strings by the
// generic helper functions.
type GenericType interface {
	GenericTypeInteger | GenericTypeFloat | bool | string | []byte
}

// GenericTypeInteger is the union of all supported integer types.
type GenericTypeInteger interface {
	GenericTypeIntegerSigned | GenericTypeIntegerUnsigned
}

// GenericTypeIntegerSigned is the union of supported signed integer types.
type GenericTypeIntegerSigned interface {
	int | int8 | int16 | int32 | int64
}

// GenericTypeIntegerUnsigned is the union of supported unsigned integer types.
type GenericTypeIntegerUnsigned interface {
	uint | uint8 | uint16 | uint32 | uint64
}

// GenericTypeFloat is the union of supported floating-point types.
type GenericTypeFloat interface {
	float32 | float64
}
