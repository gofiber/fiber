// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
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

type headerParams map[string][]byte

// getTLSConfig returns a net listener's tls config
func getTLSConfig(ln net.Listener) *tls.Config {
	// Get listener type
	pointer := reflect.ValueOf(ln)

	// Is it a tls.listener?
	if pointer.String() != "<*tls.listener Value>" {
		return nil
	}

	// Copy value from pointer
	if val := reflect.Indirect(pointer); val.IsValid() {
		// Get private field from value
		if field := val.FieldByName("config"); field.IsValid() {
			// Copy value from pointer field (unsafe)
			newValue := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())) //nolint:gosec // Probably the only way to extract the *tls.Config from a net.Listener. TODO: Verify there really is no easier way without using unsafe.
			if !newValue.IsValid() {
				return nil
			}
			// Get element from pointer
			if elem := newValue.Elem(); elem.IsValid() {
				// Cast value to *tls.Config
				c, ok := reflect.TypeAssert[*tls.Config](elem)
				if !ok {
					panic(errTLSConfigTypeAssertion)
				}
				return c
			}
		}
	}

	return nil
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
	if n, err := rf.ReadFrom(f); err != nil {
		return n, fmt.Errorf("failed to read: %w", err)
	}
	return 0, nil
}

// quoteString escapes special characters using percent-encoding.
// Non-ASCII bytes are encoded as well so the result is always ASCII.
func (app *App) quoteString(raw string) string {
	bb := bytebufferpool.Get()
	quoted := app.toString(fasthttp.AppendQuotedArg(bb.B, app.toBytes(raw)))
	bytebufferpool.Put(bb)
	return quoted
}

// quoteRawString escapes only characters that need quoting according to
// https://www.rfc-editor.org/rfc/rfc9110#section-5.6.4 so the result may
// contain non-ASCII bytes.
func (app *App) quoteRawString(raw string) string {
	const hex = "0123456789ABCDEF"
	bb := bytebufferpool.Get()
	defer bytebufferpool.Put(bb)

	for i := 0; i < len(raw); i++ {
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
			bb.B = append(bb.B,
				'%',
				hex[c>>4],
				hex[c&0x0f],
			)
		default:
			bb.B = append(bb.B, c)
		}
	}

	return app.toString(bb.B)
}

// isASCII reports whether the provided string contains only ASCII characters.
// See: https://www.rfc-editor.org/rfc/rfc0020
func (*App) isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// uniqueRouteStack drop all not unique routes from the slice
func uniqueRouteStack(stack []*Route) []*Route {
	var unique []*Route
	m := make(map[*Route]struct{})
	for _, v := range stack {
		if _, ok := m[v]; !ok {
			m[v] = struct{}{}
			unique = append(unique, v)
		}
	}

	return unique
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

// acceptsOffer determines if an offer matches a given specification.
// It supports a trailing '*' wildcard and performs case-insensitive exact matching.
// Returns true if the offer matches the specification, false otherwise.
func acceptsOffer(spec, offer string, _ headerParams) bool {
	if len(spec) >= 1 && spec[len(spec)-1] == '*' {
		return true
	}

	return utils.EqualFold(spec, offer)
}

// acceptsLanguageOfferBasic determines if a language tag offer matches a range
// according to RFC 4647 Basic Filtering.
// A match occurs if the range exactly equals the tag or is a prefix of the tag
// followed by a hyphen. The comparison is case-insensitive. Only a single "*"
// as the entire range is allowed. Any "*" appearing after a hyphen renders the
// range invalid and will not match.
func acceptsLanguageOfferBasic(spec, offer string, _ headerParams) bool {
	if spec == "*" {
		return true
	}
	if i := strings.IndexByte(spec, '*'); i != -1 {
		return false
	}
	if utils.EqualFold(spec, offer) {
		return true
	}
	return len(offer) > len(spec) &&
		utils.EqualFold(offer[:len(spec)], spec) &&
		offer[len(spec)] == '-'
}

// acceptsLanguageOfferExtended determines if a language tag offer matches a
// range according to RFC 4647 Extended Filtering (§3.3.2).
// - Case-insensitive comparisons
// - '*' matches zero or more subtags (can “slide”)
// - Unspecified subtags are treated like '*' (so trailing/extraneous tag subtags are fine)
// - Matching fails if sliding encounters a singleton (incl. 'x')
func acceptsLanguageOfferExtended(spec, offer string, _ headerParams) bool {
	if spec == "*" {
		return true
	}
	if spec == "" || offer == "" {
		return false
	}

	rs := strings.Split(spec, "-")
	ts := strings.Split(offer, "-")

	// Step 2: first subtag must match (or be '*')
	if rs[0] != "*" && !utils.EqualFold(rs[0], ts[0]) {
		return false
	}

	i, j := 1, 1 // i = range index, j = tag index
	for i < len(rs) {
		if rs[i] == "*" { // 3.A: '*' matches zero or more subtags
			i++
			continue
		}
		if j >= len(ts) { // 3.B: ran out of tag subtags
			return false
		}
		if utils.EqualFold(rs[i], ts[j]) { // 3.C: exact subtag match
			i++
			j++
			continue
		}
		// 3.D: singleton barrier (one letter or digit, incl. 'x')
		if len(ts[j]) == 1 {
			return false
		}
		// 3.E: slide forward in the tag and try again
		j++
	}
	// 4: matched all range subtags
	return true
}

// acceptsOfferType This function determines if an offer type matches a given specification.
// It checks if the specification is equal to */* (i.e., all types are accepted).
// It gets the MIME type of the offer (either from the offer itself or by its file extension).
// It checks if the offer MIME type matches the specification MIME type or if the specification is of the form <MIME_type>/* and the offer MIME type has the same MIME type.
// It checks if the offer contains every parameter present in the specification.
// Returns true if the offer type matches the specification, false otherwise.
func acceptsOfferType(spec, offerType string, specParams headerParams) bool {
	var offerMime, offerParams string

	if i := strings.IndexByte(offerType, ';'); i == -1 {
		offerMime = offerType
	} else {
		offerMime = offerType[:i]
		offerParams = offerType[i:]
	}

	// Accept: */*
	if spec == "*/*" {
		return paramsMatch(specParams, offerParams)
	}

	var mimetype string
	if strings.IndexByte(offerMime, '/') != -1 {
		mimetype = offerMime // MIME type
	} else {
		mimetype = utils.GetMIME(offerMime) // extension
	}

	if spec == mimetype {
		// Accept: <MIME_type>/<MIME_subtype>
		return paramsMatch(specParams, offerParams)
	}

	s := strings.IndexByte(mimetype, '/')
	specSlash := strings.IndexByte(spec, '/')
	// Accept: <MIME_type>/*
	if s != -1 && specSlash != -1 {
		if utils.EqualFold(spec[:specSlash], mimetype[:s]) && (spec[specSlash:] == "/*" || mimetype[s:] == "/*") {
			return paramsMatch(specParams, offerParams)
		}
	}

	return false
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
// If the given slice hasn't enough space, it will allocate more and return.
func getSplicedStrList(headerValue string, dst []string) []string {
	if headerValue == "" {
		return nil
	}

	dst = dst[:0]
	segmentStart := 0
	isLeadingSpace := true
	for i, c := range headerValue {
		switch {
		case c == ',':
			dst = append(dst, headerValue[segmentStart:i])
			segmentStart = i + 1
			isLeadingSpace = true
		case c == ' ' && isLeadingSpace:
			segmentStart = i + 1
		default:
			isLeadingSpace = false
		}
	}
	dst = append(dst, headerValue[segmentStart:])

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
		escaping := false

		if hasDQuote {
			// Complex case. We need to keep track of quotes and quoted-pairs (i.e.,  characters escaped with \ )
		loop:
			for n < len(header) {
				switch header[n] {
				case ',':
					if quotes%2 == 0 {
						break loop
					}
				case '"':
					if !escaping {
						quotes++
					}
				case '\\':
					if quotes%2 == 1 {
						escaping = !escaping
					}
				default:
					// all other characters are ignored
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
// Do not pass header using utils.UnsafeBytes - this can cause a panic due
// to the use of utils.ToLowerBytes.
func getOffer(header []byte, isAccepted func(spec, offer string, specParams headerParams) bool, offers ...string) string {
	if len(offers) == 0 {
		return ""
	}
	if len(header) == 0 {
		return offers[0]
	}

	acceptedTypes := make([]acceptedType, 0, 8)
	order := 0

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
			if bytes.HasPrefix(accept[i:], []byte(";q=")) && bytes.IndexByte(accept[qIndex:], ';') == -1 {
				if q, err := fasthttp.ParseUfloat(accept[qIndex:]); err == nil {
					quality = q
				}
			} else {
				params, _ = headerParamPool.Get().(headerParams) //nolint:errcheck // only contains headerParams
				for k := range params {
					delete(params, k)
				}
				fasthttp.VisitHeaderParams(accept[i:], func(key, value []byte) bool {
					if len(key) == 1 && key[0] == 'q' {
						if q, err := fasthttp.ParseUfloat(value); err == nil {
							quality = q
						}
						return false
					}
					lowerKey := utils.UnsafeString(utils.ToLowerBytes(key))
					val, err := unescapeHeaderValue(value)
					if err != nil {
						return true
					}
					params[lowerKey] = val
					return true
				})
			}

			// Skip this accept type if quality is 0.0
			// See: https://www.rfc-editor.org/rfc/rfc9110#quality.values
			if quality == 0.0 {
				return
			}
		}

		spec = utils.Trim(spec, ' ')

		// Determine specificity
		var specificity int

		// check for wildcard this could be a mime */* or a wildcard character *
		switch {
		case len(spec) == 1 && spec[0] == '*':
			specificity = 1
		case bytes.Equal(spec, []byte("*/*")):
			specificity = 1
		case bytes.HasSuffix(spec, []byte("/*")):
			specificity = 2
		case bytes.IndexByte(spec, '/') != -1:
			specificity = 3
		default:
			specificity = 4
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

	// Find the first offer that matches the accepted types
	for _, acceptedType := range acceptedTypes {
		for _, offer := range offers {
			if offer == "" {
				continue
			}
			if isAccepted(acceptedType.spec, offer, acceptedType.params) {
				if acceptedType.params != nil {
					headerParamPool.Put(acceptedType.params)
				}
				return offer
			}
		}
		if acceptedType.params != nil {
			headerParamPool.Put(acceptedType.params)
		}
	}

	return ""
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
	var start, end int
	header := utils.Trim(app.toString(noneMatchBytes), ' ')

	// Short-circuit the wildcard case: "*" never counts as stale.
	if header == "*" {
		return false
	}

	// Adapted from:
	// https://github.com/jshttp/fresh/blob/master/index.js#L110
	for i := range noneMatchBytes {
		switch noneMatchBytes[i] {
		case 0x20:
			if start == end {
				start = i + 1
				end = i + 1
			}
		case 0x2c:
			if matchEtag(app.toString(noneMatchBytes[start:end]), etag) {
				return false
			}
			start = i + 1
			end = i + 1
		default:
			end = i + 1
		}
	}

	return !matchEtag(app.toString(noneMatchBytes[start:end]), etag)
}

func parseAddr(raw string) (host, port string) { //nolint:nonamedreturns // gocritic unnamedResult requires naming host and port parts for clarity
	if raw == "" {
		return "", ""
	}

	raw = utils.Trim(raw, ' ')

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
		if strings.Contains(host, ":") {
			return raw, ""
		}
		return host, port
	}

	// No colon, nothing to split
	return raw, ""
}

// isNoCache checks if the cacheControl header value is a `no-cache`.
func isNoCache(cacheControl string) bool {
	n := len(cacheControl)
	ncLen := len(noCacheValue)
	for i := 0; i <= n-ncLen; i++ {
		if !utils.EqualFold(cacheControl[i:i+ncLen], noCacheValue) {
			continue
		}
		if i > 0 {
			prev := cacheControl[i-1]
			if prev != ' ' && prev != ',' {
				continue
			}
		}
		if i+ncLen == n || cacheControl[i+ncLen] == ',' {
			return true
		}
	}

	return false
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

func toBytesImmutable(s string) []byte {
	return []byte(s)
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
		default:
			return -1
		}
	}
	// For method customization
	return slices.Index(app.config.RequestMethods, s)
}

func (app *App) method(methodInt int) string {
	return app.config.RequestMethods[methodInt]
}

// IsMethodSafe reports whether the HTTP method is considered safe.
// See https://datatracker.ietf.org/doc/html/rfc9110#section-9.2.1
func IsMethodSafe(m string) bool {
	switch m {
	case MethodGet,
		MethodHead,
		MethodOptions,
		MethodTrace:
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
)

func genericParseType[V GenericType](str string) (V, error) {
	var v V
	switch any(v).(type) {
	case int:
		result, err := utils.ParseInt(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse int: %w", err)
		}
		return any(int(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int8:
		result, err := utils.ParseInt8(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse int8: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int16:
		result, err := utils.ParseInt16(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse int16: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int32:
		result, err := utils.ParseInt32(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse int32: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int64:
		result, err := utils.ParseInt(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse int64: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint:
		result, err := utils.ParseUint(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint: %w", err)
		}
		return any(uint(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint8:
		result, err := utils.ParseUint8(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint8: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint16:
		result, err := utils.ParseUint16(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint16: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint32:
		result, err := utils.ParseUint32(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint32: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint64:
		result, err := utils.ParseUint(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint64: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float32:
		result, err := utils.ParseFloat32(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse float32: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float64:
		result, err := utils.ParseFloat64(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse float64: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case bool:
		result, err := strconv.ParseBool(str)
		if err != nil {
			return v, fmt.Errorf("failed to parse bool: %w", err)
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
