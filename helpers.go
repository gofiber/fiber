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
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/gofiber/utils/v2/swar"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/internal/contextvalue"
	etagpkg "github.com/gofiber/fiber/v3/internal/etag"
	"github.com/gofiber/fiber/v3/internal/mediatype"
	"github.com/gofiber/fiber/v3/log"

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
// key in a single pass over the request headers. Unlike PeekAll it performs no
// per-call key normalization. Concrete (non-generic) so the visitor stays on
// the stack.
func peekJoinedRequestHeader(h *fasthttp.RequestHeader, key string) []byte {
	j := joinedHeaderValue{key: key}
	// VisitAll (not the replacement All) keeps this zero-alloc: All returns
	// an iterator closure that escapes to the heap on every call. The SA1019
	// deprecation is suppressed for helpers.go in .golangci.yml.
	//
	// VisitAllInOrder is deliberately not used: it reparses rawHeaders, which
	// only holds headers read off the wire, so headers set programmatically
	// (e.g. by net/http adaptors) would be missed entirely. VisitAll already
	// preserves the relative order of repeated field lines sharing a key,
	// which is all this helper needs; it only reorders across distinct keys.
	h.VisitAll(j.visit)
	return j.combined
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
	// Whether every range carries the same weight, in which case first-match selection is exact.
	uniformQuality := true
	var firstQuality float64

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
				// A list element may be followed by optional whitespace before
				// the comma (RFC 9110 §5.6.1.2), and forEachMediaRange only
				// trims the leading side. Trim the trailing side here so the
				// weight still parses; otherwise the error below is swallowed
				// and an explicit q=0 rejection silently becomes q=1.
				if q, err := fasthttp.ParseUfloat(utils.TrimSpace(accept[qIndex:])); err == nil {
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

		if order == 1 {
			firstQuality = quality
		} else if quality != firstQuality {
			uniformQuality = false
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

	// Find the best offer that matches the accepted types. Per RFC 9110 §12.5.1
	// the most specific matching range determines an offer's weight, and q=0 rejects it.
	result := ""
	switch {
	case uniformQuality && firstQuality == 0:
		// Every range rejects: nothing is acceptable.
	case uniformQuality:
		// Fast path: with equal weights the first matching range in preference order wins.
		result, _ = firstMatchingOffer(acceptedTypes, isAccepted, offers)
	default:
		// The first match stands unless a more specific range later demotes
		// that offer; only such a demotion can let another offer win.
		offer, rank := firstMatchingOffer(acceptedTypes, isAccepted, offers)
		if offer == "" {
			break
		}
		// The ranges before rank matched nothing, so only the rest can demote.
		if quality, _, ok := offerQuality(acceptedTypes[rank:], isAccepted, offer); ok && quality == acceptedTypes[rank].quality {
			result = offer
			break
		}

		// Resolve each offer from its most specific matching range and keep the heaviest.
		var bestQuality float64
		bestRank := 0
		for _, offer := range offers {
			if offer == "" {
				continue
			}
			quality, rank, ok := offerQuality(acceptedTypes, isAccepted, offer)
			if !ok {
				continue
			}
			if result == "" || quality > bestQuality || (quality == bestQuality && rank < bestRank) {
				result, bestQuality, bestRank = offer, quality, rank
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

// firstMatchingOffer returns the first offer matched by the first range that
// matches any offer, with that range's index; "" when nothing matches.
func firstMatchingOffer(types []acceptedType, isAccepted func(spec, offer string, specParams headerParams) int, offers []string) (offer string, rank int) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the pair for clarity
	for i := range types {
		for _, candidate := range offers {
			if candidate != "" && isAccepted(types[i].spec, candidate, types[i].params) > 0 {
				return candidate, i
			}
		}
	}
	return "", 0
}

// offerQuality resolves an offer against the ranges, in preference order: the
// weight of the most specific matching range (RFC 9110 §12.5.1), an explicit
// q=0 winning among equally specific ones. rank is the first matching range's
// position; ok is false when nothing matches or the weight is 0.
func offerQuality(types []acceptedType, isAccepted func(spec, offer string, specParams headerParams) int, offer string) (quality float64, rank int, ok bool) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the results for clarity
	bestSpecificity := 0
	for i := range types {
		specificity := isAccepted(types[i].spec, offer, types[i].params)
		if specificity == 0 {
			continue
		}
		if bestSpecificity == 0 {
			rank = i
		}
		if specificity > bestSpecificity || (specificity == bestSpecificity && types[i].quality == 0) {
			bestSpecificity = specificity
			quality = types[i].quality
		}
	}
	return quality, rank, bestSpecificity > 0 && quality > 0
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

// isEtagStale reports whether a response with the given ETag would be considered
// stale when presented with the raw If-None-Match header value. Comparison is
// weak as defined by RFC 9110 §8.8.3.2.
func (app *App) isEtagStale(etag string, noneMatchBytes []byte) bool {
	return !etagpkg.AnyMatch(app.toString(noneMatchBytes), etag)
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

// routesForMethod returns the routes registered for an HTTP method, or nil if
// the app does not serve it. Use this instead of indexing another app's stack
// with your own method index: RequestMethods is configurable, so two apps'
// tables can differ in both order and length.
func (app *App) routesForMethod(method string) []*Route {
	m := app.methodInt(method)
	if m < 0 || m >= len(app.stack) {
		return nil
	}

	return app.stack[m]
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

// bindMediaType returns the request's media type, lowered for comparison against
// the MIME constants. The request's own bytes are folded only for a form, the
// one case needing it in place; anything else is compared on a copy.
func bindMediaType(h *fasthttp.RequestHeader) string {
	if mediatype.IsForm(h.ContentType()) {
		raw := utils.UnsafeString(mediatype.NormalizeRequestContentType(h))
		return binder.FilterFlags(utils.ParseVendorSpecificContentType(raw))
	}

	// ToLower returns its input unchanged when there is nothing to fold, so the
	// common path costs no allocation.
	lowered := utilsstrings.ToLower(utils.UnsafeString(h.ContentType()))
	return binder.FilterFlags(utils.ParseVendorSpecificContentType(lowered))
}
