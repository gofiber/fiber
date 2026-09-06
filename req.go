package fiber

import (
	"bytes"
	"errors"
	"io"
	"math"
	"mime/multipart"
	"net"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/idna"

	etagpkg "github.com/gofiber/fiber/v3/internal/etag"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/headerlist"
	"github.com/gofiber/fiber/v3/internal/mediatype"
)

// Pre-allocated byte slices for common header comparisons to avoid allocations
var (
	xForwardedPrefix        = []byte("X-Forwarded-")
	xForwardedProtoBytes    = []byte(HeaderXForwardedProto)
	xForwardedProtocolBytes = []byte(HeaderXForwardedProtocol)
	xForwardedSslBytes      = []byte(HeaderXForwardedSsl)
	xURLSchemeBytes         = []byte(HeaderXUrlScheme)
	onBytes                 = []byte("on")
)

// Range represents the parsed HTTP Range header extracted by DefaultReq.Range.
type Range struct {
	Type   string
	Ranges []RangeSet
}

// RangeSet represents a single content range from a request.
type RangeSet struct {
	Start int64
	End   int64
}

// DefaultReq is the default implementation of Req used by DefaultCtx.
//
//go:generate ifacemaker --file req.go --struct DefaultReq --iface Req --pkg fiber --output req_interface_gen.go --not-exported true --iface-comment "Req is an interface for request-related Ctx methods."
type DefaultReq struct {
	c *DefaultCtx
}

// Accepts checks if the specified extensions or content types are acceptable.
func (r *DefaultReq) Accepts(offers ...string) string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAccept)
	return getOffer(header, acceptsOfferType, offers...)
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (r *DefaultReq) AcceptsCharsets(offers ...string) string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptCharset)
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (r *DefaultReq) AcceptsEncodings(offers ...string) string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptEncoding)
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsLanguages checks if the specified language is acceptable using
// RFC 4647 Basic Filtering.
func (r *DefaultReq) AcceptsLanguages(offers ...string) string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptLanguage)
	return getOffer(header, acceptsLanguageOfferBasic, offers...)
}

// AcceptsLanguagesExtended checks if the specified language is acceptable using
// RFC 4647 Extended Filtering.
func (r *DefaultReq) AcceptsLanguagesExtended(offers ...string) string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptLanguage)
	return getOffer(header, acceptsLanguageOfferExtended, offers...)
}

// App returns the *App reference to the instance of the Fiber application
func (r *DefaultReq) App() *App {
	return r.c.app
}

// BaseURL returns (protocol + host + base path).
func (r *DefaultReq) BaseURL() string {
	return r.c.BaseURL()
}

// BodyRaw contains the raw body submitted in a POST request.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) BodyRaw() []byte {
	return r.getBody()
}

//nolint:nonamedreturns // gocritic unnamedResult prefers naming decoded body, decode count, and error
func (r *DefaultReq) tryDecodeBodyInOrder(
	originalBody *[]byte,
	encodings []string,
) (body []byte, decodesRealized uint8, err error) {
	request := &r.c.fasthttp.Request
	maxBodySize := r.c.app.config.BodyLimit
	for idx := range encodings {
		i := len(encodings) - 1 - idx
		encoding := encodings[i]
		decodesRealized++
		var decodeErr error
		switch encoding {
		case StrGzip, "x-gzip":
			body, decodeErr = request.BodyGunzipWithLimit(maxBodySize)
		case StrBr, StrBrotli:
			body, decodeErr = request.BodyUnbrotliWithLimit(maxBodySize)
		case StrDeflate:
			body, decodeErr = request.BodyInflateWithLimit(maxBodySize)
		case StrZstd:
			body, decodeErr = request.BodyUnzstdWithLimit(maxBodySize)
		case StrIdentity:
			body = request.Body()
		case StrCompress, "x-compress":
			return nil, decodesRealized - 1, ErrNotImplemented
		default:
			return nil, decodesRealized - 1, ErrUnsupportedMediaType
		}

		if decodeErr != nil {
			return nil, decodesRealized, decodeErr
		}

		if i > 0 && decodesRealized > 0 {
			if i == len(encodings)-1 {
				tempBody := request.Body()
				*originalBody = make([]byte, len(tempBody))
				copy(*originalBody, tempBody)
			}
			// identity leaves the body as it is; re-setting an aliasing slice would
			// release its buffer under ReduceMemoryUsage.
			if encoding != StrIdentity {
				request.SetBodyRaw(body)
			}
		}
	}

	return body, decodesRealized, nil
}

// Body contains the raw body submitted in a POST request.
// This method will decompress the body if the 'Content-Encoding' header is provided.
// It returns the original (or decompressed) body data which is valid only within the handler.
// Don't store direct references to the returned data.
// If you need to keep the body's data later, make a copy or use the Immutable option.
func (r *DefaultReq) Body() []byte {
	var (
		err                error
		body, originalBody []byte
		headerEncoding     string
		encodingOrder      = []string{"", "", ""}
	)

	request := &r.c.fasthttp.Request

	// Fast path: no Content-Encoding header at all. ContentEncoding peeks the
	// stored key byte-exactly, which is decisive only while fasthttp normalized
	// the names on the way in — under DisableHeaderNormalizing a lower-case
	// spelling would read as absent here and skip decompression entirely.
	// An empty value is still a present field line and must be joined with
	// duplicates below before RFC 9110 empty-list elements are ignored.
	if !r.c.app.config.DisableHeaderNormalizing && request.Header.ContentEncoding() == nil {
		return r.getBody()
	}

	// Multiple field lines form one list (RFC 9110 §5.2), so join before splitting.
	// The result aliases the header storage, so fold into a new string; ToLower
	// returns its input unchanged without an uppercase byte, so this stays free.
	encodedBytes := peekJoinedRequestHeader(&request.Header, HeaderContentEncoding)
	headerEncoding = utilsstrings.ToLower(utils.UnsafeString(encodedBytes))

	// Split and get the encodings list, in order to attend the
	// rule defined at: https://www.rfc-editor.org/rfc/rfc9110#section-8.4-5
	// The splitter drops empty list elements (RFC 9110 Section 5.6.1.2), and
	// headerEncoding was already lowercased wholesale above.
	encodingOrder = headerlist.Append(encodingOrder, headerEncoding)
	if len(encodingOrder) == 0 {
		return r.getBody()
	}

	var decodesRealized uint8
	body, decodesRealized, err = r.tryDecodeBodyInOrder(&originalBody, encodingOrder)

	// Ensure that the body will be the original
	if originalBody != nil && decodesRealized > 0 {
		request.SetBodyRaw(originalBody)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrUnsupportedMediaType):
			_ = r.c.DefaultRes.SendStatus(StatusUnsupportedMediaType) //nolint:errcheck,staticcheck // It is fine to ignore the error and the static check
		case errors.Is(err, ErrNotImplemented):
			_ = r.c.DefaultRes.SendStatus(StatusNotImplemented) //nolint:errcheck,staticcheck // It is fine to ignore the error and the static check
		case errors.Is(err, fasthttp.ErrBodyTooLarge):
			_ = r.c.DefaultRes.SendStatus(StatusRequestEntityTooLarge) //nolint:errcheck,staticcheck // It is fine to ignore the error and the static check
		default:
			// do nothing
		}
		return []byte(err.Error())
	}

	return r.c.app.GetBytes(body)
}

// BodyStream returns the request body as a stream, non-nil only when
// Config.StreamRequestBody is enabled and the body is not already buffered.
// Reading it consumes the body, so a later Body call will not see it.
func (r *DefaultReq) BodyStream() io.Reader {
	return r.c.fasthttp.Request.BodyStream()
}

// ContentLength returns the value of the Content-Length request header. A
// negative result is not a length: -1 is chunked and -2 is identity, which is
// what a bodyless GET reports. Use HasBody to test for a body at all.
func (r *DefaultReq) ContentLength() int {
	return r.c.fasthttp.Request.Header.ContentLength()
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (r *DefaultReq) RequestCtx() *fasthttp.RequestCtx {
	return r.c.fasthttp
}

// FullURL returns the full request URL (protocol + host + original URL).
func (r *DefaultReq) FullURL() string {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.WriteString(r.Scheme())
	buf.WriteString("://")
	buf.WriteString(r.Host())
	buf.WriteString(r.OriginalURL())

	return buf.String()
}

// UserAgent returns the User-Agent request header.
func (r *DefaultReq) UserAgent() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.UserAgent())
}

// Referer returns the Referer request header.
func (r *DefaultReq) Referer() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.Referer())
}

// Origin returns the Origin request header.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Origin() string {
	return r.c.app.toString(r.headerField(HeaderOrigin))
}

// headerField returns a request header's first non-empty field value, matching
// the field name case-insensitively (RFC 9110 Section 5.1). fieldname.First
// also steps over a present-but-empty first line, so a value on a later line
// is found and this agrees with GetAll on whether the field is there.
func (r *DefaultReq) headerField(name string) []byte {
	return fieldname.First(&r.c.fasthttp.Request.Header, name, !r.c.app.config.DisableHeaderNormalizing)
}

// AcceptLanguage returns the Accept-Language request header.
// Repeated field lines are combined into one comma-joined list (RFC 9110
// Section 5.2) and the field name matches case-insensitively (Section 5.1),
// matching what AcceptsLanguages negotiates on.
func (r *DefaultReq) AcceptLanguage() string {
	return r.c.app.toString(peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptLanguage))
}

// AcceptEncoding returns the Accept-Encoding request header.
// Repeated field lines are combined into one comma-joined list (RFC 9110
// Section 5.2) and the field name matches case-insensitively (Section 5.1),
// matching what AcceptsEncodings negotiates on.
func (r *DefaultReq) AcceptEncoding() string {
	return r.c.app.toString(peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderAcceptEncoding))
}

// HasHeader reports whether the request includes a header with the given key.
// The field name matches case-insensitively (RFC 9110 Section 5.1), so this
// agrees with GetAll on whether the field is there.
func (r *DefaultReq) HasHeader(key string) bool {
	return len(r.headerField(key)) > 0
}

// ContentType returns the Content-Type request header, parameters included;
// MediaType strips them and Charset returns just the charset. On Ctx the request
// wins over Res. Only valid within the handler unless Immutable is set.
func (r *DefaultReq) ContentType() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.ContentType())
}

// MediaType returns the MIME type from the Content-Type header without parameters.
func (r *DefaultReq) MediaType() string {
	contentType := utils.TrimSpace(r.c.fasthttp.Request.Header.ContentType())
	if len(contentType) == 0 {
		return ""
	}
	if idx := bytes.IndexByte(contentType, ';'); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = utils.TrimSpace(contentType)
	return r.c.app.toString(contentType)
}

// Charset returns the charset parameter from the Content-Type header.
func (r *DefaultReq) Charset() string {
	contentType := r.c.fasthttp.Request.Header.ContentType()
	_, params, ok := bytes.Cut(contentType, []byte{';'})
	if !ok {
		return ""
	}
	for len(params) > 0 {
		// Slice off the next parameter at the next ";" that sits outside a
		// quoted-string: parameter values may be quoted and contain ";"
		// (RFC 9110 Section 5.6.6). A DQUOTE only opens a quoted-string at
		// the start of a value (after an "=" plus optional whitespace), so a
		// stray quote later in an unquoted value cannot swallow the rest of
		// the header.
		param := params
		end := -1
		inQuotes := false
		escaped := false
		expectValue := false
	scan:
		for i := 0; i < len(params); i++ {
			ch := params[i]
			switch {
			case escaped:
				escaped = false
			case inQuotes:
				switch ch {
				case '\\':
					escaped = true
				case '"':
					inQuotes = false
				}
			case ch == '=':
				expectValue = true
			case ch == '"' && expectValue:
				inQuotes = true
				expectValue = false
			case ch == ';':
				end = i
				break scan
			case ch != ' ' && ch != '\t':
				expectValue = false
			}
		}
		if end >= 0 {
			param = params[:end]
			params = params[end+1:]
		} else {
			params = nil
		}

		name, value, ok := bytes.Cut(param, []byte{'='})
		if !ok || !utils.EqualFold(utils.TrimSpace(name), []byte("charset")) {
			continue
		}
		v := utils.TrimSpace(value)
		if len(v) > 0 && v[0] == '"' {
			// A quoted value must be a complete quoted-string, and its
			// quoted-pairs must be replaced with the escaped octet
			// (RFC 9110 Section 5.6.4).
			if len(v) < 2 || v[len(v)-1] != '"' {
				return ""
			}
			unescaped, err := unescapeHeaderValue(v[1 : len(v)-1])
			if err != nil {
				return ""
			}
			v = unescaped
		} else if bytes.IndexByte(v, '"') >= 0 {
			// A bare token must not contain DQUOTE; skip the invalid
			// parameter instead of surfacing garbage, so a well-formed
			// charset parameter later in the header can still be found.
			continue
		}
		return r.c.app.toString(v)
	}
	return ""
}

// IsJSON reports whether the Content-Type header is JSON.
func (r *DefaultReq) IsJSON() bool {
	return utils.EqualFold(r.MediaType(), MIMEApplicationJSON)
}

// IsForm reports whether the Content-Type header is form-encoded.
func (r *DefaultReq) IsForm() bool {
	return utils.EqualFold(r.MediaType(), MIMEApplicationForm)
}

// IsMultipart reports whether the Content-Type header is multipart form data.
func (r *DefaultReq) IsMultipart() bool {
	return utils.EqualFold(r.MediaType(), MIMEMultipartForm)
}

// AcceptsJSON reports whether the Accept header allows JSON.
func (r *DefaultReq) AcceptsJSON() bool {
	return r.Accepts(MIMEApplicationJSON) != ""
}

// AcceptsHTML reports whether the Accept header allows HTML.
func (r *DefaultReq) AcceptsHTML() bool {
	return r.Accepts(MIMETextHTML) != ""
}

// AcceptsXML reports whether the Accept header allows XML.
func (r *DefaultReq) AcceptsXML() bool {
	return r.Accepts(MIMEApplicationXML, MIMETextXML) != ""
}

// AcceptsEventStream reports whether the Accept header allows text/event-stream.
func (r *DefaultReq) AcceptsEventStream() bool {
	return r.Accepts(MIMETextEventStream) != ""
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Cookies(key string, defaultValue ...string) string {
	return defaultString(r.c.app.toString(r.c.fasthttp.Request.Header.Cookie(key)), defaultValue)
}

// CookieNames returns the request cookie names in header order, or nil when
// there are none. A repeated name appears once per occurrence, which is how the
// shadowing AllCookies collapses is detected. The strings are copies.
func (r *DefaultReq) CookieNames() []string {
	var names []string
	for key := range r.c.fasthttp.Request.Header.Cookies() {
		names = append(names, string(key))
	}
	return names
}

// AllCookies returns the request cookies as a name/value map, or nil when there
// are none. A repeated name resolves to its first occurrence, as Cookies does,
// so a shadowed cookie cannot be validated in one and consumed in the other.
func (r *DefaultReq) AllCookies() map[string]string {
	var cookies map[string]string
	for key, value := range r.c.fasthttp.Request.Header.Cookies() {
		name := string(key)
		if _, seen := cookies[name]; seen {
			continue
		}
		if cookies == nil {
			cookies = make(map[string]string)
		}
		cookies[name] = string(value)
	}
	return cookies
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (r *DefaultReq) Request() *fasthttp.Request {
	return &r.c.fasthttp.Request
}

// FormFile returns the first file by key from a MultipartForm.
// The multipart form is parsed using the application's BodyLimit to prevent
// unbounded memory usage.
func (r *DefaultReq) FormFile(key string) (*multipart.FileHeader, error) {
	if _, err := r.MultipartForm(); err != nil {
		return nil, err
	}
	return r.c.fasthttp.FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
// Search is performed in QueryArgs, PostArgs, MultipartForm and FormFile in this particular order.
// Defaults to the empty string "" if the form value doesn't exist.
// If a default value is given, it will return that value if the form value does not exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
// When the request is a multipart form, it is parsed using the application's
// BodyLimit so the configured limit is consistently enforced.
//
// On a form request this lowercases the case-insensitive parts of the request's
// own Content-Type, so a value obtained earlier from Get(HeaderContentType) —
// which aliases those bytes unless Immutable is set — can change during the
// call. Copy it first if you need it to outlive one.
func (r *DefaultReq) FormValue(key string, defaultValue ...string) string {
	// fasthttp locates the urlencoded body and multipart boundary
	// case-sensitively, so a legal "Multipart/Form-Data" yielded nothing here.
	// Guarded, because the fold rewrites the request's own bytes.
	if mediatype.IsForm(r.c.fasthttp.Request.Header.ContentType()) {
		mediatype.NormalizeRequestContentType(&r.c.fasthttp.Request.Header)
	}

	if r.c.IsMultipart() {
		// For multipart requests, parse the form using the application's BodyLimit.
		// fasthttp's FormValue would otherwise re-parse with its default 8 MiB limit,
		// effectively bypassing the configured BodyLimit.
		//
		// Preserve the original search order: QueryArgs → PostArgs → MultipartForm.
		if v := r.c.fasthttp.QueryArgs().Peek(key); len(v) > 0 {
			return r.c.app.toString(v)
		}
		if v := r.c.fasthttp.PostArgs().Peek(key); len(v) > 0 {
			return r.c.app.toString(v)
		}
		mf, err := r.MultipartForm()
		if err != nil {
			return defaultString("", defaultValue)
		}
		if vals := mf.Value[key]; len(vals) > 0 {
			return vals[0]
		}
		return defaultString("", defaultValue)
	}
	return defaultString(r.c.app.toString(r.c.fasthttp.FormValue(key)), defaultValue)
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// Freshness only applies to GET and HEAD requests; for any other method false is
// returned, as RFC 9110 defines 304 Not Modified only for those methods and
// requires If-Modified-Since to be ignored otherwise.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/master/index.js#L33
func (r *DefaultReq) Fresh() bool {
	// Freshness only applies to GET and HEAD requests: a 304 Not Modified
	// response is defined for those methods only, and RFC 9110 Section 13.1.3
	// requires If-Modified-Since to be ignored for any other method.
	// A negative methodInt means the method is not registered at all, so it
	// cannot be GET or HEAD (and must not be used to index RequestMethods).
	if r.c.methodInt < 0 {
		return false
	}
	if method := r.c.app.method(r.c.methodInt); method != MethodGet && method != MethodHead {
		return false
	}

	header := &r.c.fasthttp.Request.Header

	// fields
	// List-based fields may be split across multiple field lines, which are
	// semantically one comma-joined list (RFC 9110 Section 5.2). Field names
	// match case-insensitively, so the lower-case spellings HTTP/2 and 3 send
	// are not read as an unconditional request.
	modifiedSince := r.headerField(HeaderIfModifiedSince)
	noneMatch := peekJoinedRequestHeader(header, HeaderIfNoneMatch)

	// unconditional request
	if len(modifiedSince) == 0 && len(noneMatch) == 0 {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://www.rfc-editor.org/rfc/rfc9111#section-5.2.1.4
	cacheControl := peekJoinedRequestHeader(header, HeaderCacheControl)
	if len(cacheControl) > 0 && isNoCache(utils.UnsafeString(cacheControl)) {
		return false
	}

	// if-none-match takes precedence over if-modified-since (RFC 9110)
	if len(noneMatch) > 0 {
		if len(noneMatch) == 1 && noneMatch[0] == '*' {
			return true
		}
		app := r.c.app
		response := &r.c.fasthttp.Response
		etag := app.toString(response.Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if app.isEtagStale(etag, noneMatch) {
			return false
		}
		return true
	}

	// if-modified-since (only reached when if-none-match is absent)
	if len(modifiedSince) > 0 {
		response := &r.c.fasthttp.Response
		lastModified := response.Header.Peek(HeaderLastModified)
		if len(lastModified) == 0 {
			return false
		}
		lastModifiedTime, err := parseHTTPDate(lastModified)
		if err != nil {
			return false
		}
		// Common conditional request: the client echoes back the exact
		// Last-Modified it was given. Identical, already-validated dates are
		// equal, so skip the second parse and comparison.
		if !bytes.Equal(lastModified, modifiedSince) {
			modifiedSinceTime, err := parseHTTPDate(modifiedSince)
			if err != nil {
				return false
			}
			if lastModifiedTime.Compare(modifiedSinceTime) == 1 {
				return false
			}
		}
	}
	return true
}

// parseHTTPDate parses an HTTP-date field value. RFC 9110 Section 5.6.7
// requires recipients to accept the obsolete RFC 850 and ANSI C asctime()
// formats in addition to the preferred IMF-fixdate; utils.ParseHTTPDate
// covers all three with net/http.ParseTime semantics, taking an
// allocation-free fast path for canonical IMF-fixdate input. Unlike
// net/http.ParseTime it also tolerates surrounding ASCII whitespace —
// deliberate here, matching RFC 9110 §5.5's exclusion of leading and
// trailing OWS from field values.
func parseHTTPDate(date []byte) (time.Time, error) {
	t, err := utils.ParseHTTPDate(date)
	if err != nil {
		// Callers only nil-check the error; skip wrapping to avoid an
		// allocation for every malformed client-supplied date.
		return time.Time{}, err //nolint:wrapcheck // see above
	}
	return t, nil
}

// IfNoneMatch returns the entity tags in the If-None-Match request header,
// verbatim and unvalidated; a comma inside a quoted opaque-tag does not split
// one. Fresh already compares them. Only valid within the handler.
func (r *DefaultReq) IfNoneMatch() []string {
	header := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, HeaderIfNoneMatch)
	if len(header) == 0 {
		return nil
	}
	return etagpkg.Split(r.c.app.toString(header))
}

// IfModifiedSince returns the time carried by the If-Modified-Since request
// header, ErrHeaderNotFound when it is absent or empty, and a parse error when
// it is none of the HTTP-date formats RFC 9110 Section 5.6.7 requires.
func (r *DefaultReq) IfModifiedSince() (time.Time, error) {
	value := r.headerField(HeaderIfModifiedSince)
	if len(value) == 0 {
		return time.Time{}, ErrHeaderNotFound
	}
	return parseHTTPDate(value)
}

// Get returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) Get(key string, defaultValue ...string) string {
	return GetReqHeader(r.c, key, defaultValue...)
}

// GetReqHeader returns the HTTP request header specified by filed.
// This function is generic and can handle different headers type values.
// If the generic type cannot be matched to a supported type, the function
// returns the default value (if provided) or the zero value of type V.
func GetReqHeader[V GenericType](c Ctx, key string, defaultValue ...V) V {
	v, err := genericParseType[V](c.App().toString(c.Req().headerField(key)))
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// GetAll returns every field line stored under key, where Get returns only the
// first, or nil when the header is absent. Empty lines are skipped, so its
// presence answers as HasHeader does. Only valid within the handler.
func (r *DefaultReq) GetAll(key string) []string {
	app := r.c.app
	lines := fieldname.Lines(&r.c.fasthttp.Request.Header, key, !app.config.DisableHeaderNormalizing)

	var values []string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		values = append(values, app.toString(line))
	}
	return values
}

// Authorization splits the Authorization header into auth-scheme and credentials
// (RFC 9110 Section 11.6.2), neither decoded nor validated. They view the request
// buffer: a credential kept past the handler becomes another request's.
func (r *DefaultReq) Authorization() (scheme, credentials string) { //nolint:nonamedreturns // gocritic unnamedResult requires naming the two halves for clarity
	rawScheme, rawCredentials := r.authorizationParts()
	app := r.c.app
	return app.toString(rawScheme), app.toString(rawCredentials)
}

// authorizationParts splits the Authorization header at the first space or tab
// (RFC 9110 Section 11.4), on raw bytes so each caller materializes only the
// strings it needs. A lone token is a scheme without credentials.
func (r *DefaultReq) authorizationParts() (scheme, credentials []byte) { //nolint:nonamedreturns // the two halves need naming for clarity
	raw := utils.TrimSpace(r.headerField(HeaderAuthorization))
	if len(raw) == 0 {
		return nil, nil
	}
	i := utils.IndexAny2(raw, ' ', '\t')
	if i < 0 {
		return raw, nil
	}
	return raw[:i], utils.TrimSpace(raw[i+1:])
}

// Bearer returns the credentials of a Bearer Authorization header (RFC 6750
// Section 2.1), unverified, or "" for another scheme. It views the request
// buffer: a token kept past the handler becomes another request's.
func (r *DefaultReq) Bearer() string {
	// The scheme is compared on the raw bytes rather than through Authorization,
	// so a header naming another scheme costs no string at all and a Bearer one
	// materializes only the token.
	scheme, credentials := r.authorizationParts()
	if len(credentials) == 0 || !utils.EqualFold(utils.UnsafeString(scheme), "Bearer") {
		return ""
	}
	return r.c.app.toString(credentials)
}

// GetHeaders (a.k.a GetReqHeaders) returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) GetHeaders() map[string][]string {
	app := r.c.app
	reqHeader := &r.c.fasthttp.Request.Header
	// Pre-allocate map with known header count to avoid reallocations
	headers := make(map[string][]string, reqHeader.Len())
	for k, v := range reqHeader.All() {
		key := app.toString(k)
		headers[key] = append(headers[key], app.toString(v))
	}
	return headers
}

// Host contains the host derived from the X-Forwarded-Host or Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// In a network context, `Host` refers to the combination of a hostname and potentially a port number used for connecting,
// while `Hostname` refers specifically to the name assigned to a device on a network, excluding any port information.
// Example: URL: https://example.com:8080 -> Host: example.com:8080
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing if your app is behind a proxy.
func (r *DefaultReq) Host() string {
	if r.IsProxyTrusted() {
		if host := r.Get(HeaderXForwardedHost); host != "" {
			if before, _, found := strings.Cut(host, ","); found {
				return utils.TrimSpace(before)
			}
			return utils.TrimSpace(host)
		}
	}
	return r.c.app.toString(r.c.fasthttp.Request.URI().Host())
}

// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header using the c.Host() method.
// Returned value is only valid within the handler. Do not store any references.
// Example: URL: https://example.com:8080 -> Hostname: example.com
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing if your app is behind a proxy.
func (r *DefaultReq) Hostname() string {
	addr, _ := parseAddr(r.Host())

	return addr
}

// Port returns the remote port of the request.
func (r *DefaultReq) Port() string {
	addr := r.c.fasthttp.RemoteAddr()
	if addr == nil {
		return "0"
	}
	switch typedAddr := addr.(type) {
	case *net.TCPAddr:
		return utils.FormatInt(int64(typedAddr.Port))
	case *net.UnixAddr:
		return ""
	}

	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return ""
	}

	return port
}

// IP returns the client's IP address. When the request comes from a trusted proxy (see
// [TrustProxyConfig]), the value is extracted from the configured ProxyHeader by walking the
// X-Forwarded-For chain right-to-left and skipping all trusted proxy IPs; the first
// non-trusted IP in the chain is returned. Please use Config.TrustProxy to prevent header
// spoofing if your app is behind a proxy.
func (r *DefaultReq) IP() string {
	app := r.c.app
	if r.IsProxyTrusted() && app.config.ProxyHeader != "" {
		return r.extractIPFromHeader(app.config.ProxyHeader)
	}

	if ip := r.c.fasthttp.RemoteIP(); ip != nil {
		return ip.String()
	}
	return ""
}

// extractIPsFromHeader will return a slice of IPs it found given a header name in the order they appear.
// When IP validation is enabled, any invalid IPs will be omitted.
func (r *DefaultReq) extractIPsFromHeader(header string) []string {
	// TODO: Reuse the c.extractIPFromHeader func somehow in here

	headerValue := proxyHeaderValue(r, header)

	// We can't know how many IPs we will return, but we will try to guess with this constant division.
	// Counting ',' makes function slower for about 50ns in general case.
	const maxEstimatedCount = 8
	estimatedCount := min(len(headerValue)/maxEstimatedCount,
		// Avoid big allocation on big header
		maxEstimatedCount)

	ipsFound := make([]string, 0, estimatedCount)

	i := 0
	j := -1

	for {
		var v4, v6 bool

		// Manually splitting string without allocating slice, working with parts directly
		i, j = j+1, j+2

		if j > len(headerValue) {
			break
		}

		for j < len(headerValue) && headerValue[j] != ',' {
			switch headerValue[j] {
			case ':':
				v6 = true
			case '.':
				v4 = true
			default:
				// do nothing
			}
			j++
		}

		for i < j && (headerValue[i] == ' ' || headerValue[i] == ',') {
			i++
		}

		s := utils.TrimRight(headerValue[i:j], ' ')

		if r.c.app.config.EnableIPValidation {
			// Skip validation if IP is clearly not IPv4/IPv6; otherwise, validate without allocations.
			// A colon decides: an IPv4-mapped IPv6 address carries both separators.
			if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (!v6 && v4 && !utils.IsIPv4(s)) {
				continue
			}
		}

		ipsFound = append(ipsFound, s)
	}

	return ipsFound
}

// extractIPFromHeader returns the client IP from the given proxy header by walking the
// X-Forwarded-For chain from right to left and stripping trusted proxy IPs. When trusted
// proxies are configured, the rightmost non-trusted IP is returned; otherwise the first
// valid IP (left-to-right) is returned. When IP validation is disabled, the raw header
// value is returned as-is.
func (r *DefaultReq) extractIPFromHeader(header string) string {
	app := r.c.app

	if !app.config.EnableIPValidation {
		return proxyHeaderValue(r, header)
	}

	headerValue := proxyHeaderValue(r, header)
	hasTrustedProxyConfig := r.hasTrustedProxyConfig()
	if !hasTrustedProxyConfig {
		start := 0
		for {
			end := start
			for end < len(headerValue) && headerValue[end] != ',' {
				end++
			}

			ipStr := utils.Trim(headerValue[start:end], ' ')
			if isValidProxyIP(ipStr) {
				return ipStr
			}
			if end == len(headerValue) {
				break
			}
			start = end + 1
		}
	}

	var leftmostIP string

	for end := len(headerValue); end > 0; {
		start := end
		for start > 0 && headerValue[start-1] != ',' {
			start--
		}

		ipStr := utils.Trim(headerValue[start:end], ' ')
		if isValidProxyIP(ipStr) {
			leftmostIP = ipStr
			if !r.isTrustedProxyIP(ipStr) {
				return ipStr
			}
		}

		if start == 0 {
			break
		}
		end = start - 1
	}

	if leftmostIP != "" {
		return leftmostIP
	}
	if ip := r.c.fasthttp.RemoteIP(); ip != nil {
		return ip.String()
	}
	return ""
}

// proxyHeaderValue combines repeated proxy header field lines as required by
// RFC 9110 Section 5.2. This ensures security-sensitive IP extraction sees the
// complete chain when an adapter or proxy preserves separate field lines.
func proxyHeaderValue(r *DefaultReq, header string) string {
	value := peekJoinedRequestHeader(&r.c.fasthttp.Request.Header, header)
	return r.c.app.toString(value)
}

func isValidProxyIP(ipStr string) bool {
	if strings.IndexByte(ipStr, ':') >= 0 {
		// A colon makes it IPv6, including the IPv4-mapped form ::ffff:a.b.c.d (RFC 4291 §2.2).
		return utils.IsIPv6(ipStr)
	}
	return strings.IndexByte(ipStr, '.') >= 0 && utils.IsIPv4(ipStr)
}

// hasTrustedProxyConfig returns true if any trusted proxy configuration is set.
func (r *DefaultReq) hasTrustedProxyConfig() bool {
	cfg := r.c.app.config.TrustProxyConfig
	return len(cfg.ips) > 0 || len(cfg.ranges) > 0 || cfg.Loopback || cfg.Private || cfg.LinkLocal
}

// isTrustedProxyIP checks whether the given IP string matches any configured trusted proxy.
func (r *DefaultReq) isTrustedProxyIP(ipStr string) bool {
	cfg := r.c.app.config.TrustProxyConfig

	// utils.ParseIPv4/ParseIPv6 together accept exactly the strings
	// netip.ParseAddr does, without allocating an error for invalid input.
	ip, ok := utils.ParseIPv4(ipStr)
	if !ok {
		if ip, ok = utils.ParseIPv6(ipStr); !ok {
			return false
		}
		// An IPv4-mapped address names an IPv4 proxy: look it up as that, as net.IP does for the peer.
		ip = ip.Unmap()
	}

	if cfg.Loopback && ip.IsLoopback() {
		return true
	}
	if cfg.Private && ip.IsPrivate() {
		return true
	}
	if cfg.LinkLocal && ip.IsLinkLocalUnicast() {
		return true
	}

	var canonicalIP [net.IPv6len * 3]byte
	if _, trusted := cfg.ips[utils.UnsafeString(ip.AppendTo(canonicalIP[:0]))]; trusted {
		return true
	}
	if len(cfg.ranges) == 0 {
		return false
	}

	if ip.Is4() {
		ipv4 := ip.As4()
		for _, ipNet := range cfg.ranges {
			if ipNet.Contains(ipv4[:]) {
				return true
			}
		}
		return false
	}

	ipv6 := ip.As16()
	for _, ipNet := range cfg.ranges {
		if ipNet.Contains(ipv6[:]) {
			return true
		}
	}
	return false
}

// IPs returns a string slice of IP addresses specified in the X-Forwarded-For request header.
// When IP validation is enabled, only valid IPs are returned.
func (r *DefaultReq) IPs() []string {
	return r.extractIPsFromHeader(HeaderXForwardedFor)
}

// Is returns the matching content type,
// if the incoming request's Content-Type HTTP header field matches the MIME type specified by the type parameter
func (r *DefaultReq) Is(extension string) bool {
	extensionHeader := utils.GetMIME(extension)
	if extensionHeader == "" {
		return false
	}

	ct := r.c.app.toString(r.c.fasthttp.Request.Header.ContentType())
	if i := strings.IndexByte(ct, ';'); i != -1 {
		ct = ct[:i]
	}
	ct = utils.TrimSpace(ct)
	return utils.EqualFold(ct, extensionHeader)
}

// HasBody returns true if the request declares a body via Content-Length, Transfer-Encoding, or already buffered payload data.
func (r *DefaultReq) HasBody() bool {
	hdr := &r.c.fasthttp.Request.Header

	switch cl := hdr.ContentLength(); {
	case cl > 0:
		return true
	case cl == -1:
		// fasthttp reports -1 for Transfer-Encoding: chunked bodies.
		return true
	case cl == 0:
		if hasTransferEncodingBody(hdr) {
			return true
		}
	}

	return len(r.c.fasthttp.Request.Body()) > 0
}

func hasTransferEncodingBody(hdr *fasthttp.RequestHeader) bool {
	// Repeated field lines form one combined list (RFC 9110 Section 5.2),
	// so every Transfer-Encoding line must be inspected, not just the first.
	if lines := hdr.PeekAll(HeaderTransferEncoding); len(lines) > 0 {
		for _, line := range lines {
			if transferEncodingLineHasBody(utils.UnsafeString(line)) {
				return true
			}
		}
		return false
	}

	// Fallback scan for non-normalized header keys.
	for key, value := range hdr.All() {
		if !utils.EqualFold(utils.UnsafeString(key), HeaderTransferEncoding) {
			continue
		}
		if transferEncodingLineHasBody(utils.UnsafeString(value)) {
			return true
		}
	}

	return false
}

// transferEncodingLineHasBody reports whether a single Transfer-Encoding
// field line contains a transfer coding other than "identity".
func transferEncodingLineHasBody(te string) bool {
	for token := range headerlist.All(te) {
		// A transfer coding may carry parameters ("chunked;q=1"), which name the
		// coding no more than the bare token does (RFC 9110 Section 10.1.4).
		if idx := strings.IndexByte(token, ';'); idx >= 0 {
			token = utils.TrimSpace(token[:idx])
		}
		if token == "" || utils.EqualFold(token, "identity") {
			continue
		}
		return true
	}
	return false
}

// IsWebSocket returns true if the request includes a WebSocket upgrade handshake.
func (r *DefaultReq) IsWebSocket() bool {
	// Repeated field lines are equivalent to one combined comma-separated
	// list (RFC 9110 Section 5.2), so inspect every Connection and Upgrade
	// field line, not just the first.
	hdr := &r.c.fasthttp.Request.Header
	canonical := !r.c.app.config.DisableHeaderNormalizing
	if !headerListContainsToken(fieldname.Lines(hdr, HeaderConnection, canonical), "upgrade") {
		return false
	}
	// Upgrade is a list of protocols, each optionally carrying a "/version"
	// suffix (RFC 9110 Section 7.8), e.g. "Upgrade: websocket, h2c".
	return headerListContainsToken(fieldname.Lines(hdr, HeaderUpgrade, canonical), "websocket")
}

// headerListContainsToken reports whether any comma-separated element across
// the given field lines equals token case-insensitively. An optional
// "/version" suffix (Upgrade protocol syntax, RFC 9110 Section 7.8) is
// ignored when comparing; valid Connection members never contain "/", so
// this is safe for both headers.
func headerListContainsToken(lines [][]byte, token string) bool {
	for element := range headerlist.AllLines(lines) {
		// Connection and Upgrade carry protocol names, which may name a version
		// after a slash ("HTTP/2.0"). Match on the name alone.
		if i := strings.IndexByte(element, '/'); i >= 0 {
			element = element[:i]
		}
		if utils.EqualFold(element, token) {
			return true
		}
	}
	return false
}

// IsPreflight returns true if the request is a CORS preflight.
func (r *DefaultReq) IsPreflight() bool {
	if r.Method() != MethodOptions {
		return false
	}
	if len(r.headerField(HeaderAccessControlRequestMethod)) == 0 {
		return false
	}
	return len(r.headerField(HeaderOrigin)) > 0
}

// Secure returns whether a secure connection was established.
func (r *DefaultReq) Secure() bool {
	return r.Scheme() == schemeHTTPS
}

// xmlHTTPRequestBytes is precomputed for XHR detection
var xmlHTTPRequestBytes = []byte("xmlhttprequest")

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (r *DefaultReq) XHR() bool {
	return utils.EqualFold(r.headerField(HeaderXRequestedWith), xmlHTTPRequestBytes)
}

// Locals makes it possible to pass any values under keys scoped to the request
// and therefore available to all following routes that match the request.
//
// All the values are removed from ctx after returning from the top
// RequestHandler. Additionally, Close method is called on each value
// implementing io.Closer before removing the value from ctx.
func (r *DefaultReq) Locals(key any, value ...any) any {
	if r.c.fasthttp == nil {
		if len(value) > 0 {
			return value[0]
		}
		return nil
	}
	if len(value) == 0 {
		return r.c.fasthttp.UserValue(key)
	}
	r.c.fasthttp.SetUserValue(key, value[0])
	return value[0]
}

// Locals function utilizing Go's generics feature.
// This function allows for manipulating and retrieving local values within a
// request context with a more specific data type.
//
// All the values are removed from ctx after returning from the top
// RequestHandler. Additionally, Close method is called on each value
// implementing io.Closer before removing the value from ctx.
func Locals[V any](c Ctx, key any, value ...V) V {
	var v V
	var ok bool
	if len(value) == 0 {
		v, ok = c.Locals(key).(V)
	} else {
		v, ok = c.Locals(key, value[0]).(V)
	}
	if !ok {
		return v // return zero of type V
	}
	return v
}

// Method returns the HTTP request method for the context, optionally overridden by the provided argument.
// If no override is given or if the provided override is not a valid HTTP method, it returns the current method from the context.
// Otherwise, it updates the context's method and returns the overridden method as a string.
func (r *DefaultReq) Method(override ...string) string {
	app := r.c.app
	if len(override) == 0 {
		// Nothing to override, just return current method from context
		return currentMethod(r.c)
	}

	// Method tokens are case-sensitive (RFC 9110 Section 9.1), so try the
	// override exactly as given first — this is what keeps mixed-case custom
	// methods registered via Config.RequestMethods working.
	method := override[0]
	methodInt := app.methodInt(method)
	if methodInt == -1 {
		// Fall back to the conventional uppercase form as a convenience for
		// the standard methods (e.g. "get" -> "GET").
		method = utilsstrings.ToUpper(method)
		methodInt = app.methodInt(method)
	}
	if methodInt == -1 {
		// Provided override is not a registered HTTP method; no override,
		// return current method
		return currentMethod(r.c)
	}
	if methodInt == r.c.methodInt {
		return method
	}
	// A middleware sits at a position of its own in every method's tree, so
	// carry the index over or Next would skip the routes before it.
	r.c.indexRoute = app.routeIndexInTree(methodInt, r.c.treePathHash, r.c.route, r.c.indexRoute)
	r.c.methodInt = methodInt
	// Method changed; invalidate the lookahead index
	r.c.firstMatchIndex = -1
	return method
}

// currentMethod resolves the context's method, falling back to the raw
// request header value when the method is not registered in RequestMethods,
// so unregistered methods are reported instead of panicking.
// It is a package-level function (not a method) to stay off the generated
// Req/Ctx interfaces.
func currentMethod(c *DefaultCtx) string {
	// app.method owns the definition of "unregistered" (it bounds-checks
	// methodInt and returns "" for anything out of range).
	if m := c.app.method(c.methodInt); m != "" {
		return m
	}
	// Copy the raw header bytes: every other return path yields a stable
	// string from RequestMethods, so callers may retain the result beyond
	// the handler; an unsafe alias into the request buffer would be
	// silently corrupted on connection reuse.
	return string(c.fasthttp.Request.Header.Method())
}

// IsSafe reports whether the request method is safe, meaning it is not expected
// to change server state (RFC 9110 Section 9.2.1).
func (r *DefaultReq) IsSafe() bool {
	return IsMethodSafe(r.Method())
}

// IsIdempotent reports whether the request method is idempotent, meaning
// repeating it has the same intended effect as making it once (RFC 9110
// Section 9.2.2). Every safe method is also idempotent.
func (r *DefaultReq) IsIdempotent() bool {
	return IsMethodIdempotent(r.Method())
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key, the value will be a string slice.
//
// On a form request this lowercases the case-insensitive parts of the request's
// own Content-Type, so a value obtained earlier from Get(HeaderContentType) —
// which aliases those bytes unless Immutable is set — can change during the
// call. Copy it first if you need it to outlive one.
func (r *DefaultReq) MultipartForm() (*multipart.Form, error) {
	// fasthttp matches both "multipart/form-data" and the "boundary=" parameter
	// name case-sensitively, so fold first. FormFile and SaveFile come through
	// here too. Guarded like FormValue: the fold writes.
	if mediatype.IsForm(r.c.fasthttp.Request.Header.ContentType()) {
		mediatype.NormalizeRequestContentType(&r.c.fasthttp.Request.Header)
	}

	return r.c.fasthttp.MultipartFormWithLimit(r.c.app.config.BodyLimit)
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) OriginalURL() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.RequestURI())
}

// URI returns the parsed *fasthttp.URI of the request, giving access to all
// fasthttp URI methods. It is owned by the request and rewritten by a Path
// override, so it is only valid within the handler.
func (r *DefaultReq) URI() *fasthttp.URI {
	return r.c.fasthttp.Request.URI()
}

// Path returns the path part of the request URL.
// Optionally, you could override the path.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Path(override ...string) string {
	if len(override) != 0 && string(r.c.path) != override[0] {
		// Set new path to context
		r.c.pathOriginal = override[0]

		// Set new path to request context
		r.c.fasthttp.Request.URI().SetPath(r.c.pathOriginal)
		// Prettify path
		r.c.configDependentPaths()
		// The detection path/tree hash changed; invalidate the lookahead index.
		r.c.firstMatchIndex = -1
		// The new path may live in another bucket, so carry the index over for
		// Next to resume after this route.
		r.c.indexRoute = r.c.app.routeIndexInTree(r.c.methodInt, r.c.treePathHash, r.c.route, r.c.indexRoute)
	}
	return r.c.app.toString(r.c.path)
}

// Params is used to get the route parameters.
// Defaults to empty string "" if the param doesn't exist.
// If a default value is given, it will return that value if the param doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Params(key string, defaultValue ...string) string {
	if key == "*" || key == "+" {
		key += "1"
	}

	app := r.c.app
	route := r.c.Route()
	values := &r.c.values
	for i := range route.Params {
		if len(key) != len(route.Params[i]) {
			continue
		}
		if route.Params[i] == key || (!app.config.CaseSensitive && utils.EqualFold(route.Params[i], key)) {
			// if there is no value for the key
			if len(values) <= i || values[i] == "" {
				break
			}
			val := values[i]
			return r.c.app.GetString(val)
		}
	}
	return defaultString("", defaultValue)
}

// Params is used to get the route parameters.
// This function is generic and can handle different route parameters type values.
// If the generic type cannot be matched to a supported type, the function
// returns the default value (if provided) or the zero value of type V.
//
// Example:
//
// http://example.com/user/:user -> http://example.com/user/john
// Params[string](c, "user") -> returns john
//
// http://example.com/id/:id -> http://example.com/user/114
// Params[int](c, "id") ->  returns 114 as integer.
//
// http://example.com/id/:number -> http://example.com/id/john
// Params[int](c, "number", 0) -> returns 0 because can't parse 'john' as integer.
func Params[V GenericType](c Ctx, key string, defaultValue ...V) V {
	v, err := genericParseType[V](c.Params(key))
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// Scheme contains the request protocol string: http or https for TLS requests.
// Please use Config.TrustProxy to prevent header spoofing if your app is behind a proxy.
func (r *DefaultReq) Scheme() string {
	ctx := r.c.fasthttp
	if ctx.IsTLS() {
		return schemeHTTPS
	}
	if !r.IsProxyTrusted() {
		return schemeHTTP
	}

	app := r.c.app
	scheme := schemeHTTP
	const lenXHeaderName = 12
	for key, val := range ctx.Request.Header.All() {
		if len(key) < lenXHeaderName {
			continue // Neither "X-Forwarded-" nor "X-Url-Scheme"
		}
		switch {
		case utils.EqualFold(key[:len(xForwardedPrefix)], xForwardedPrefix):
			if utils.EqualFold(key, xForwardedProtoBytes) ||
				utils.EqualFold(key, xForwardedProtocolBytes) {
				v := app.toString(val)
				if before, _, found := strings.Cut(v, ","); found {
					v = before
				}
				if forwarded, ok := forwardedScheme(v); ok {
					scheme = forwarded
				}
			} else if utils.EqualFold(key, xForwardedSslBytes) && utils.EqualFold(val, onBytes) {
				scheme = schemeHTTPS
			}

		case utils.EqualFold(key, xURLSchemeBytes):
			if forwarded, ok := forwardedScheme(app.toString(val)); ok {
				scheme = forwarded
			}
		default:
			continue
		}
	}
	return scheme
}

// forwardedScheme canonicalizes a scheme announced by a proxy header. Only
// "http" and "https": the value is spliced into a URL (BaseURL) and compared for
// origin equality (CSRF, Redirect.Back). Rejecting leaves the prior scheme.
func forwardedScheme(value string) (string, bool) {
	value = utils.TrimSpace(value)
	switch {
	case utils.EqualFold(value, schemeHTTPS):
		return schemeHTTPS, true
	case utils.EqualFold(value, schemeHTTP):
		return schemeHTTP, true
	default:
		return "", false
	}
}

// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
func (r *DefaultReq) Protocol() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.Protocol())
}

// Query returns the query string parameter in the url.
// Defaults to empty string "" if the query doesn't exist.
// If a default value is given, it will return that value if the query doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Query(key string, defaultValue ...string) string {
	return Query(r.c, key, defaultValue...)
}

// Queries returns a map of query parameters and their values.
//
// GET /?name=alex&wanna_cake=2&id=
// Queries()["name"] == "alex"
// Queries()["wanna_cake"] == "2"
// Queries()["id"] == ""
//
// GET /?field1=value1&field1=value2&field2=value3
// Queries()["field1"] == "value2"
// Queries()["field2"] == "value3"
//
// GET /?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3
// Queries()["list_a"] == "3"
// Queries()["list_b[]"] == "3"
// Queries()["list_c"] == "1,2,3"
//
// GET /api/search?filters.author.name=John&filters.category.name=Technology&filters[customer][name]=Alice&filters[status]=pending
// Queries()["filters.author.name"] == "John"
// Queries()["filters.category.name"] == "Technology"
// Queries()["filters[customer][name]"] == "Alice"
// Queries()["filters[status]"] == "pending"
func (r *DefaultReq) Queries() map[string]string {
	app := r.c.app
	queryArgs := r.c.fasthttp.QueryArgs()

	m := make(map[string]string, queryArgs.Len())
	for key, value := range queryArgs.All() {
		m[app.toString(key)] = app.toString(value)
	}
	return m
}

// Query Retrieves the value of a query parameter from the request's URI.
// The function is generic and can handle query parameter values of different types.
// It takes the following parameters:
// - c: The context object representing the current request.
// - key: The name of the query parameter.
// - defaultValue: (Optional) The default value to return if the query parameter is not found or cannot be parsed.
// The function performs the following steps:
//  1. Type-asserts the context object to *DefaultCtx.
//  2. Retrieves the raw query parameter value from the request's URI.
//  3. Parses the raw value into the appropriate type based on the generic type parameter V.
//     If parsing fails, the function checks if a default value is provided. If so, it returns the default value.
//  4. Returns the parsed value.
//
// If the generic type cannot be matched to a supported type, the function returns the default value (if provided) or the zero value of type V.
//
// Example usage:
//
//	GET /?search=john&age=8
//	name := Query[string](c, "search") // Returns "john"
//	age := Query[int](c, "age") // Returns 8
//	unknown := Query[string](c, "unknown", "default") // Returns "default" since the query parameter "unknown" is not found
func Query[V GenericType](c Ctx, key string, defaultValue ...V) V {
	q := c.App().toString(c.RequestCtx().QueryArgs().Peek(key))
	v, err := genericParseType[V](q)
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// Range returns a struct containing the type and a slice of ranges.
func (r *DefaultReq) Range(size int64) (Range, error) {
	var (
		rangeData Range
		ranges    string
	)
	rangeStr := utils.TrimSpace(r.Get(HeaderRange))
	maxRanges := r.c.app.config.MaxRanges
	const maxRangePrealloc = 8
	prealloc := min(maxRanges, maxRangePrealloc)
	if prealloc > 0 {
		rangeData.Ranges = make([]RangeSet, 0, prealloc)
	}

	// parseBound parses a present (non-empty) range bound. A bound that is
	// not a valid integer makes the range-spec, and therefore the whole
	// ranges-specifier, invalid (RFC 9110 Section 14.1.1).
	parseBound := func(value string) (int64, error) {
		parsed, err := utils.ParseUint(value)
		if err != nil {
			return 0, ErrRangeMalformed
		}
		if parsed > (math.MaxUint64 >> 1) {
			return 0, ErrRangeMalformed
		}
		return int64(parsed), nil
	}

	before, after, found := strings.Cut(rangeStr, "=")
	if !found {
		return Range{}, ErrRangeMalformed
	}
	if !utils.EqualFold(utils.TrimSpace(before), "bytes") {
		// A range unit the server does not understand is not malformed: it
		// must be ignored (RFC 9110 Section 14.2), which only the caller can
		// do, so signal it distinctly. This check runs before any syntax
		// checks on the range-set, since the other-range grammar permits
		// characters (such as "=") that byte ranges do not.
		return Range{}, ErrRangeUnsupported
	}
	rangeData.Type = "bytes"
	if strings.IndexByte(after, '=') >= 0 {
		return Range{}, ErrRangeMalformed
	}
	ranges = utils.TrimSpace(after)

	var (
		singleRange  string
		moreRanges   = ranges
		elementCount int  // every list element, including empty ones (MaxRanges bound)
		sawRangeSpec bool // at least one non-empty range-spec was present
	)
	for moreRanges != "" {
		// Empty elements count toward MaxRanges too, so the cap bounds the
		// total parsing work per header, not just the accepted range-specs.
		elementCount++
		if elementCount > maxRanges {
			r.c.DefaultRes.Status(StatusRequestedRangeNotSatisfiable)
			r.c.DefaultRes.Set(HeaderContentRange, "bytes */"+utils.FormatInt(size)) //nolint:staticcheck // It is fine to ignore the static check
			return Range{}, ErrRangeTooLarge
		}

		singleRange = moreRanges
		if i := strings.IndexByte(moreRanges, ','); i >= 0 {
			singleRange = moreRanges[:i]
			moreRanges = utils.TrimSpace(moreRanges[i+1:])
		} else {
			moreRanges = ""
		}

		singleRange = utils.TrimSpace(singleRange)

		// RFC 9110 Section 5.6.1.2: recipients must parse and ignore a
		// reasonable number of empty list elements, e.g. "bytes=,0-5" is
		// equivalent to "bytes=0-5".
		if singleRange == "" {
			continue
		}
		sawRangeSpec = true

		var (
			startStr, endStr string
			i                int
		)
		if i = strings.IndexByte(singleRange, '-'); i == -1 {
			return Range{}, ErrRangeMalformed
		}
		startStr = utils.TrimSpace(singleRange[:i])
		endStr = utils.TrimSpace(singleRange[i+1:])

		var (
			start, end int64
			err        error
		)
		if startStr != "" {
			if start, err = parseBound(startStr); err != nil {
				return Range{}, err
			}
		}
		if endStr != "" {
			if end, err = parseBound(endStr); err != nil {
				return Range{}, err
			}
		}
		switch {
		case startStr == "" && endStr == "":
			// "-" carries neither a first-byte-pos nor a suffix-length and is
			// not a valid range-spec (RFC 9110 Section 14.1.1).
			return Range{}, ErrRangeMalformed
		case startStr == "": // -nnn (suffix range)
			start = max(size-end, 0)
			end = size - 1
		case endStr == "": // nnn- (open-ended range)
			end = size - 1
		default: // nnn-mmm
			// An int-range with a last-byte-pos less than its first-byte-pos
			// invalidates the whole ranges-specifier (RFC 9110 Section 14.1.1).
			if end < start {
				return Range{}, ErrRangeMalformed
			}
			if end > size-1 { // limit last-byte-pos to current length
				end = size - 1
			}
		}
		if start > end {
			// Syntactically valid but does not overlap the representation
			// (e.g. first-byte-pos beyond EOF or a zero-length suffix); skip
			// it and let the satisfiability check below decide.
			continue
		}
		rangeData.Ranges = append(rangeData.Ranges, RangeSet{
			Start: start,
			End:   end,
		})
	}
	if len(rangeData.Ranges) < 1 {
		if !sawRangeSpec {
			// Only empty list elements: there was no range-spec at all, so
			// the ranges-specifier is invalid rather than unsatisfiable.
			return Range{}, ErrRangeMalformed
		}
		r.c.DefaultRes.Status(StatusRequestedRangeNotSatisfiable)
		r.c.DefaultRes.Set(HeaderContentRange, "bytes */"+utils.FormatInt(size)) //nolint:staticcheck // It is fine to ignore the static check
		return rangeData, ErrRequestedRangeNotSatisfiable
	}

	return rangeData, nil
}

// Endpoint returns the route that will handle this request. See Ctx.Endpoint.
func (r *DefaultReq) Endpoint() *Route {
	return r.c.Endpoint()
}

// Route returns the matched Route struct.
func (r *DefaultReq) Route() *Route {
	return r.c.Route()
}

// Subdomains returns a slice of subdomains from the host, excluding the last `offset` components.
// If the offset is negative or exceeds the number of subdomains, an empty slice is returned.
// If the offset is zero every label (no trimming) is returned.
func (r *DefaultReq) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}

	// Negative offset, return nothing.
	if o < 0 {
		return []string{}
	}

	// Normalize host according to RFC 3986
	host := r.Hostname()
	// Trim the trailing dot of a fully-qualified domain
	if strings.HasSuffix(host, ".") {
		host = utils.TrimRight(host, '.')
	}
	host = utilsstrings.ToLower(host)

	// Decode punycode labels only when necessary
	if strings.Contains(host, "xn--") {
		if u, err := idna.Lookup.ToUnicode(host); err == nil {
			host = utilsstrings.ToLower(u)
		}
	}

	// Return nothing for IP addresses
	ip := host
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		ip = ip[1 : len(ip)-1]
	}
	if utils.IsIPv4(ip) || utils.IsIPv6(ip) {
		return []string{}
	}

	// Use stack-allocated array for typical domain names (up to 8 labels)
	// This avoids heap allocation for most common cases
	var partsBuf [8]string
	parts := partsBuf[:0]

	for part := range strings.SplitSeq(host, ".") {
		parts = append(parts, part)
	}

	// offset == 0, caller wants everything.
	if o == 0 {
		// Need to return a copy since partsBuf is on the stack
		result := make([]string, len(parts))
		copy(result, parts)
		return result
	}

	// If we trim away the whole slice (or more), nothing remains.
	if o >= len(parts) {
		return []string{}
	}

	// Return a heap-allocated copy of the relevant portion
	result := make([]string, len(parts)-o)
	copy(result, parts[:len(parts)-o])
	return result
}

// Stale returns the inverse of Fresh, indicating if the client's cached response is considered stale.
func (r *DefaultReq) Stale() bool {
	return !r.Fresh()
}

// IsProxyTrusted checks trustworthiness of remote ip.
// If Config.TrustProxy false, it returns false.
// IsProxyTrusted can check remote ip by proxy ranges and ip map.
func (r *DefaultReq) IsProxyTrusted() bool {
	config := r.c.app.config
	if !config.TrustProxy {
		return false
	}

	remoteAddr := r.c.fasthttp.RemoteAddr()
	switch remoteAddr.(type) {
	case *net.UnixAddr:
		return config.TrustProxyConfig.UnixSocket
	case *net.TCPAddr, *net.UDPAddr:
		// Keep existing RemoteIP/IP-map/CIDR checks for TCP/UDP paths as-is.
	default:
		// Unknown address type: do not trust by default.
		return false
	}

	ip := r.c.fasthttp.RemoteIP()
	if ip == nil {
		return false
	}

	if (config.TrustProxyConfig.Loopback && ip.IsLoopback()) ||
		(config.TrustProxyConfig.Private && ip.IsPrivate()) ||
		(config.TrustProxyConfig.LinkLocal && ip.IsLinkLocalUnicast()) {
		return true
	}

	// Only stringify the IP when there is an exact-match map to look it up in;
	// ip.String() heap-allocates and is wasted work for CIDR-only configs.
	if len(config.TrustProxyConfig.ips) > 0 {
		if _, trusted := config.TrustProxyConfig.ips[ip.String()]; trusted {
			return true
		}
	}

	for _, ipNet := range config.TrustProxyConfig.ranges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsFromLocal will return true if request came from a loopback IP.
func (r *DefaultReq) IsFromLocal() bool {
	if ip := r.c.fasthttp.RemoteIP(); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// IsFromUnixSocket returns true if the request arrived over a Unix domain socket.
func (r *DefaultReq) IsFromUnixSocket() bool {
	_, ok := r.c.fasthttp.RemoteAddr().(*net.UnixAddr)
	return ok
}

func (r *DefaultReq) getBody() []byte {
	return r.c.app.GetBytes(r.c.fasthttp.Request.Body())
}
