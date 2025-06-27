// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
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

	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"

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
			newval := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())) //nolint:gosec // Probably the only way to extract the *tls.Config from a net.Listener. TODO: Verify there really is no easier way without using unsafe.
			if !newval.IsValid() {
				return nil
			}
			// Get element from pointer
			if elem := newval.Elem(); elem.IsValid() {
				// Cast value to *tls.Config
				c, ok := elem.Interface().(*tls.Config)
				if !ok {
					panic(errors.New("failed to type-assert to *tls.Config"))
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

// quoteString escape special characters in a given string
func (app *App) quoteString(raw string) string {
	bb := bytebufferpool.Get()
	quoted := app.getString(fasthttp.AppendQuotedArg(bb.B, app.getBytes(raw)))
	bytebufferpool.Put(bb)
	return quoted
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
	if len(value) == 0 && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return value
}

func getGroupPath(prefix, path string) string {
	if len(path) == 0 {
		return prefix
	}

	if path[0] != '/' {
		path = "/" + path
	}

	return utils.TrimRight(prefix, '/') + path
}

// acceptsOffer This function determines if an offer matches a given specification.
// It checks if the specification ends with a '*' or if the offer has the prefix of the specification.
// Returns true if the offer matches the specification, false otherwise.
func acceptsOffer(spec, offer string, _ headerParams) bool {
	if len(spec) >= 1 && spec[len(spec)-1] == '*' {
		return true
	} else if strings.HasPrefix(spec, offer) {
		return true
	}
	return false
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
	// Accept: <MIME_type>/*
	if strings.HasPrefix(spec, mimetype[:s]) && (spec[s:] == "/*" || mimetype[s:] == "/*") {
		return paramsMatch(specParams, offerParams)
	}

	return false
}

// paramsMatch returns whether offerParams contains all parameters present in specParams.
// Matching is case insensitive, and surrounding quotes are stripped.
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
				return nil, errors.New("invalid escape sequence")
			}
			escaping = true
			continue
		}
		res = append(res, c)
	}
	if escaping {
		return nil, errors.New("invalid escape sequence")
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

func matchEtag(s, etag string) bool {
	if s == etag || s == "W/"+etag || "W/"+s == etag {
		return true
	}

	return false
}

func (app *App) isEtagStale(etag string, noneMatchBytes []byte) bool {
	var start, end int

	// Adapted from:
	// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L110
	for i := range noneMatchBytes {
		switch noneMatchBytes[i] {
		case 0x20:
			if start == end {
				start = i + 1
				end = i + 1
			}
		case 0x2c:
			if matchEtag(app.getString(noneMatchBytes[start:end]), etag) {
				return false
			}
			start = i + 1
			end = i + 1
		default:
			end = i + 1
		}
	}

	return !matchEtag(app.getString(noneMatchBytes[start:end]), etag)
}

func parseAddr(raw string) (string, string) { //nolint:revive // Returns (host, port)
	if raw == "" {
		return "", ""
	}

	// Handle IPv6 addresses enclosed in brackets as defined by RFC 3986
	if strings.HasPrefix(raw, "[") {
		if end := strings.IndexByte(raw, ']'); end != -1 {
			host := raw[:end+1] // keep the closing ]
			if len(raw) > end+1 && raw[end+1] == ':' {
				return host, raw[end+2:]
			}
			return host, ""
		}
	}

	// Everything else with a colon
	if i := strings.LastIndexByte(raw, ':'); i != -1 {
		host, port := raw[:i], raw[i+1:]

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

const noCacheValue = "no-cache"

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

func (c *testConn) Read(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	return c.r.Read(b) //nolint:wrapcheck // This must not be wrapped
}

func (c *testConn) Write(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()

	if c.isClosed {
		return 0, errTestConnClosed
	}
	return c.w.Write(b) //nolint:wrapcheck // This must not be wrapped
}

func (c *testConn) Close() error {
	c.Lock()
	defer c.Unlock()

	c.isClosed = true
	return nil
}

func (*testConn) LocalAddr() net.Addr                { return &net.TCPAddr{Port: 0, Zone: "", IP: net.IPv4zero} }
func (*testConn) RemoteAddr() net.Addr               { return &net.TCPAddr{Port: 0, Zone: "", IP: net.IPv4zero} }
func (*testConn) SetDeadline(_ time.Time) error      { return nil }
func (*testConn) SetReadDeadline(_ time.Time) error  { return nil }
func (*testConn) SetWriteDeadline(_ time.Time) error { return nil }

func getStringImmutable(b []byte) string {
	return string(b)
}

func getBytesImmutable(s string) []byte {
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
		result, err := strconv.ParseInt(str, 10, 0)
		if err != nil {
			return v, fmt.Errorf("failed to parse int: %w", err)
		}
		return any(int(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int8:
		result, err := strconv.ParseInt(str, 10, 8)
		if err != nil {
			return v, fmt.Errorf("failed to parse int8: %w", err)
		}
		return any(int8(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int16:
		result, err := strconv.ParseInt(str, 10, 16)
		if err != nil {
			return v, fmt.Errorf("failed to parse int16: %w", err)
		}
		return any(int16(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int32:
		result, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			return v, fmt.Errorf("failed to parse int32: %w", err)
		}
		return any(int32(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case int64:
		result, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return v, fmt.Errorf("failed to parse int64: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint:
		result, err := strconv.ParseUint(str, 10, 0)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint: %w", err)
		}
		return any(uint(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint8:
		result, err := strconv.ParseUint(str, 10, 8)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint8: %w", err)
		}
		return any(uint8(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint16:
		result, err := strconv.ParseUint(str, 10, 16)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint16: %w", err)
		}
		return any(uint16(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint32:
		result, err := strconv.ParseUint(str, 10, 32)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint32: %w", err)
		}
		return any(uint32(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case uint64:
		result, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return v, fmt.Errorf("failed to parse uint64: %w", err)
		}
		return any(result).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float32:
		result, err := strconv.ParseFloat(str, 32)
		if err != nil {
			return v, fmt.Errorf("failed to parse float32: %w", err)
		}
		return any(float32(result)).(V), nil //nolint:errcheck,forcetypeassert // not needed
	case float64:
		result, err := strconv.ParseFloat(str, 64)
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

type GenericType interface {
	GenericTypeInteger | GenericTypeFloat | bool | string | []byte
}

type GenericTypeInteger interface {
	GenericTypeIntegerSigned | GenericTypeIntegerUnsigned
}

type GenericTypeIntegerSigned interface {
	int | int8 | int16 | int32 | int64
}

type GenericTypeIntegerUnsigned interface {
	uint | uint8 | uint16 | uint32 | uint64
}

type GenericTypeFloat interface {
	float32 | float64
}
