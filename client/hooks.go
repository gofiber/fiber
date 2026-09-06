package client

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

var protocolCheck = regexp.MustCompile(`(?i)^https?://.*$`)

var fileBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 1<<20) // 1MB buffer
		return &b
	},
}

const (
	headerAccept      = "Accept"
	applicationJSON   = "application/json"
	applicationCBOR   = "application/cbor"
	applicationXML    = "application/xml"
	applicationForm   = "application/x-www-form-urlencoded"
	multipartFormData = "multipart/form-data"

	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// unsafeRandString returns a random string of length n.
// An error is returned if the random source fails.
func unsafeRandString(n int) (string, error) {
	inputLength := byte(len(letterBytes))

	// Compute the largest multiple of inputLength ≤ 256 to avoid modulo bias.
	// Any byte ≥ max will be rejected and re‑read.
	maxLength := byte(256 - (256 % int(inputLength))) //nolint:gosec // G115: integer overflow conversion int -> byte

	out := make([]byte, n)
	buf := make([]byte, n)

	// Read n raw bytes in one shot
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand.Read failed: %w", err)
	}

	for i, b := range buf {
		// Reject values ≥ maxLength
		for b >= maxLength {
			if _, err := rand.Read(buf[i : i+1]); err != nil {
				return "", fmt.Errorf("rand.Read failed: %w", err)
			}
			b = buf[i]
		}
		out[i] = letterBytes[b%inputLength]
	}

	return utils.UnsafeString(out), nil
}

// parserRequestURL sets options for the hostclient and normalizes the URL.
// It merges the baseURL with the request URI if needed and applies query and path parameters.
func parserRequestURL(c *Client, req *Request) error {
	// Split URL into path and query parts using Cut (avoids allocation)
	uri, queryPart, _ := strings.Cut(req.url, "?")

	// If the URL doesn't start with http/https, prepend the baseURL.
	if !protocolCheck.MatchString(uri) {
		uri = c.baseURL + uri
		if !protocolCheck.MatchString(uri) {
			return ErrURLFormat
		}
	}

	// Set path parameters from the request and the client. Request values are
	// looked up first, so they keep overriding the client's for the same name.
	disablePathNormalizing := c.isPathNormalizingDisabled || req.DisablePathNormalizing()

	uri, err := substitutePathParams(uri, disablePathNormalizing, req.path, *c.path)
	if err != nil {
		return err
	}

	// Set the URI in the raw request.
	req.RawRequest.SetRequestURI(uri)
	req.RawRequest.URI().DisablePathNormalizing = disablePathNormalizing
	if disablePathNormalizing {
		req.RawRequest.URI().SetPathBytes(req.RawRequest.URI().PathOriginal())
	}

	// Merge query parameters (split query from fragment using Cut).
	queryOnly, hashPart, _ := strings.Cut(queryPart, "#")
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)

	args.Parse(queryOnly)

	for key, value := range c.params.All() {
		args.AddBytesKV(key, value)
	}
	for key, value := range req.params.All() {
		args.AddBytesKV(key, value)
	}

	req.RawRequest.URI().SetQueryStringBytes(utils.CopyBytes(args.QueryString()))
	req.RawRequest.URI().SetHash(hashPart)

	return nil
}

// pathParamEndChars marks the bytes that terminate a ":name" placeholder in a
// request URL. The set mirrors the route parser's parameterEndChars (path.go),
// so a client placeholder is delimited exactly like a server route parameter,
// plus '#', which ends the path client-side.
var pathParamEndChars = [256]bool{
	'/':  true,
	'-':  true,
	'.':  true,
	':':  true,
	'\\': true,
	'?':  true,
	'#':  true,
}

// substitutePathParams replaces every ":name" placeholder in uri with the value
// found in sources, searched in order. A placeholder ends at a path-segment
// boundary, so ":id" no longer also matches the head of ":idx"; the scan is a
// single left-to-right pass, so a substituted value is never rescanned for
// placeholders; and each value is percent-encoded with path-segment rules, so
// a "?" or "#" inside a value cannot restructure the request target.
// A placeholder with no matching value is left untouched.
//
// With path normalizing left on, escaping alone cannot hold a value inside its
// segment — normalizing decodes the path before it collapses "." and ".." — so
// a value that would break out is rejected with ErrPathParamInPath. A
// placeholder in the authority is filled verbatim and checked against
// isHostSafe, returning ErrPathParamInHost.
//
//nolint:revive // flag-parameter: disablePathNormalizing is the request setting, as in composeRedirectURL
func substitutePathParams(uri string, disablePathNormalizing bool, sources ...PathParam) (string, error) {
	if !strings.ContainsRune(uri, ':') {
		return uri, nil
	}

	authEnd := authorityEnd(uri)

	var (
		buf  []byte
		last int
	)

	for i := 0; i < len(uri); i++ {
		if uri[i] != ':' {
			continue
		}

		// The colons inside an IPv6 literal belong to the address, so
		// "http://[2001:db8::1]/x" must not be rewritten by a "db8" parameter.
		if i < authEnd && inIPv6Literal(uri[:i]) {
			continue
		}

		ph, ok := lookupPlaceholder(uri, i, i < authEnd, sources)
		if !ok {
			continue
		}

		if buf == nil {
			// One growth for the common case: the substituted values are
			// usually shorter than the placeholders plus a little slack.
			buf = make([]byte, 0, len(uri)+16)
		}

		buf = append(buf, uri[last:i]...)
		if i < authEnd {
			if !isHostSafe(ph.value) {
				return "", fmt.Errorf("%w: %q", ErrPathParamInHost, ph.name)
			}
			buf = append(buf, ph.value...)
		} else {
			if !disablePathNormalizing && !isSingleSegment(ph.value) {
				return "", fmt.Errorf("%w: %q", ErrPathParamInPath, ph.name)
			}
			buf = utils.AppendPathEscape(buf, ph.value)
		}
		last = ph.end
		i = ph.end - 1
	}

	if buf == nil {
		return uri, nil
	}

	return string(append(buf, uri[last:]...)), nil
}

// isSingleSegment reports whether val still occupies exactly one path segment
// after fasthttp has normalized the path. Normalizing percent-decodes the path
// before it collapses "//", "/./" and "/x/../", so an escaped "/" turns back
// into a real separator and a value of "../../admin" walks out of the path the
// template set, while an empty value leaves a "//" that collapses and drops
// the segment entirely. Only the raw bytes matter: a "%2F" typed by the caller
// is written as "%252F" and decodes to a literal "%2F", not to a separator.
func isSingleSegment(val string) bool {
	return val != "" && val != "." && val != ".." && !strings.ContainsAny(val, `/\`)
}

// isHostSafe reports whether val may be substituted into a URI authority
// verbatim. Percent-encoding is not an option there: fasthttp rejects a "%XX"
// below 0x80 inside a host, and it then abandons the rest of the URI, so a
// value that would need escaping has to fail the request instead. The set is
// RFC 3986's unreserved characters plus the sub-delims a host parse keeps, and
// the bytes of a valid UTF-8 sequence, which fasthttp passes through so an IDN
// label can be templated. ":" and "@" are excluded deliberately: they would add
// a port or a userinfo section, and a value of "x@evil.com" in
// "http://:host/api" would send the request to evil.com. Malformed UTF-8 is
// excluded for the same reason one step removed: the bytes reach the Host
// header verbatim, and an overlong sequence such as "\xc0\xaf" is a "/" to a
// lenient decoder somewhere downstream.
func isHostSafe(val string) bool {
	for i := range len(val) {
		switch c := val[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '.', c == '_', c == '~':
		case c == '$', c == '&', c == '+', c == '=':
		case c >= utf8.RuneSelf:
		default:
			return false
		}
	}

	// An empty value would build "http:///api", which fails later at dial with
	// a message that says nothing about the parameter.
	return val != "" && utf8.ValidString(val)
}

// inIPv6Literal reports whether the authority prefix ends inside a "[...]"
// IPv6 literal.
func inIPv6Literal(prefix string) bool {
	return strings.LastIndexByte(prefix, '[') > strings.LastIndexByte(prefix, ']')
}

// authorityEnd returns the index at which uri's authority ends, or 0 when uri
// has none. Everything from "://" up to the first "/", "?" or "#" is authority.
func authorityEnd(uri string) int {
	start := strings.Index(uri, "://")
	if start < 0 {
		return 0
	}
	start += len("://")

	for i := start; i < len(uri); i++ {
		switch uri[i] {
		case '/', '?', '#':
			return i
		}
	}

	return len(uri)
}

// isAllDigits reports whether s is a non-empty run of ASCII digits.
func isAllDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return s != ""
}

// pathParamSegmentEndChars marks the bytes that end the segment a ":name" lives
// in, which is as far as a placeholder name can reach.
var pathParamSegmentEndChars = [256]bool{
	'/':  true,
	'\\': true,
	'?':  true,
	'#':  true,
}

// lookupPlaceholder resolves the ":name" starting at i, returning the value and
// the index just past the name. Candidates run from the end of the segment down
// to each terminator inside it, longest first, so a name holding a "-", "." or
// ":" still resolves. ":idx" never offers "id", because "x" does not terminate a
// name and the shorter run is therefore not a candidate at all.
// placeholder is a resolved ":name" in a request URL: the parameter name, the
// value found for it, and the index just past the name in the URL.
type placeholder struct {
	name  string
	value string
	end   int
}

//nolint:revive // flag-parameter: inAuthority selects the port rule, as in substitutePathParams
func lookupPlaceholder(uri string, i int, inAuthority bool, sources []PathParam) (placeholder, bool) {
	segEnd := i + 1
	for segEnd < len(uri) && !pathParamSegmentEndChars[uri[segEnd]] {
		segEnd++
	}

	for end := segEnd; end > i+1; end-- {
		if end != segEnd && !pathParamEndChars[uri[end]] {
			continue
		}

		name := uri[i+1 : end]

		// A host may be templated ("http://:tenant.example.com"), so the
		// authority is scanned too, but a digits-only name there is the port,
		// and substituting it would eat the ":" that separates it from the host.
		if inAuthority && isAllDigits(name) {
			continue
		}

		if val, ok := lookupPathParam(name, sources); ok {
			return placeholder{name: name, value: val, end: end}, true
		}
	}

	return placeholder{}, false
}

// lookupPathParam returns the first value stored under name, searching sources
// in order.
func lookupPathParam(name string, sources []PathParam) (string, bool) {
	for _, source := range sources {
		if val, ok := source[name]; ok {
			return val, true
		}
	}

	return "", false
}

// parserRequestHeader merges client and request headers, and sets headers automatically based on the request data.
// It also sets the User-Agent and Referer headers, and applies any cookies from the cookie jar.
func parserRequestHeader(c *Client, req *Request) error {
	// Set HTTP method.
	req.RawRequest.Header.SetMethod(req.Method())

	// Merge headers from the client, clearing each key first so a resend does not accumulate values.
	for key := range c.header.All() {
		req.RawRequest.Header.DelBytes(key)
	}
	for key, value := range c.header.All() {
		req.RawRequest.Header.AddBytesKV(key, value)
	}

	// Merge headers from the request; they override the client's for the same key.
	for key := range req.header.All() {
		req.RawRequest.Header.DelBytes(key)
	}
	for key, value := range req.header.All() {
		req.RawRequest.Header.AddBytesKV(key, value)
	}

	// Set Content-Type and Accept headers based on the request body type.
	switch req.bodyType {
	case jsonBody:
		req.RawRequest.Header.SetContentType(applicationJSON)
		req.RawRequest.Header.Set(headerAccept, applicationJSON)
	case xmlBody:
		req.RawRequest.Header.SetContentType(applicationXML)
	case cborBody:
		req.RawRequest.Header.SetContentType(applicationCBOR)
	case formBody:
		req.RawRequest.Header.SetContentType(applicationForm)
	case filesBody:
		req.RawRequest.Header.SetContentType(multipartFormData)
		// If boundary is default, append a random string to it.
		if req.boundary == boundary {
			randStr, err := unsafeRandString(16)
			if err != nil {
				return fmt.Errorf("boundary generation: %w", err)
			}
			req.boundary += randStr
		}
		req.RawRequest.Header.SetMultipartFormBoundary(req.boundary)
	default:
		// noBody or rawBody do not require special handling here.
	}

	// Set User-Agent header.
	req.RawRequest.Header.SetUserAgent(defaultUserAgent)
	if c.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(c.userAgent)
	}
	if req.userAgent != "" {
		req.RawRequest.Header.SetUserAgent(req.userAgent)
	}

	// Set Referer header.
	req.RawRequest.Header.SetReferer(c.referer)
	if req.referer != "" {
		req.RawRequest.Header.SetReferer(req.referer)
	}

	// Set cookies from the cookie jar if available.
	if c.cookieJar != nil {
		c.cookieJar.dumpCookiesToReq(req.RawRequest)
	}

	// Set cookies from the client.
	for key, val := range c.cookies.All() {
		req.RawRequest.Header.SetCookie(key, val)
	}

	// Set cookies from the request.
	for key, val := range req.cookies.All() {
		req.RawRequest.Header.SetCookie(key, val)
	}

	return nil
}

// parserRequestBody serializes the request body based on its type and sets it into the RawRequest.
func parserRequestBody(c *Client, req *Request) error {
	switch req.bodyType {
	case jsonBody:
		body, err := c.jsonMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case xmlBody:
		body, err := c.xmlMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case cborBody:
		body, err := c.cborMarshal(req.body)
		if err != nil {
			return err
		}
		req.RawRequest.SetBody(body)
	case formBody:
		req.RawRequest.SetBody(req.formData.QueryString())
	case filesBody:
		return parserRequestBodyFile(req)
	case rawBody:
		if body, ok := req.body.([]byte); ok { //nolint:revive // ignore simplicity
			req.RawRequest.SetBody(body)
		} else {
			return ErrBodyType
		}
	case noBody:
		// No body to set.
		return nil
	default:
		return ErrBodyTypeNotSupported
	}
	return nil
}

// parserRequestBodyFile handles the case where the request contains files to be uploaded.
func parserRequestBodyFile(req *Request) error {
	mw := multipart.NewWriter(req.RawRequest.BodyWriter())
	if err := mw.SetBoundary(req.boundary); err != nil {
		return fmt.Errorf("set boundary error: %w", err)
	}

	if err := writeMultipartBody(mw, req); err != nil {
		mw.Close() //nolint:errcheck // the body write already failed; surface that error instead
		return err
	}

	// Close writes the trailing boundary; if it fails the multipart body is
	// incomplete, so surface the error instead of sending a malformed request.
	if err := mw.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return nil
}

// writeMultipartBody writes the form fields and files of req to mw.
func writeMultipartBody(mw *multipart.Writer, req *Request) error {
	// Add form data.
	var err error
	for key, value := range req.formData.All() {
		err = mw.WriteField(utils.UnsafeString(key), utils.UnsafeString(value))
		if err != nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("write formdata error: %w", err)
	}

	// Add files.
	fileBuf, ok := fileBufPool.Get().(*[]byte)
	if !ok {
		return errSyncPoolBuffer
	}

	defer fileBufPool.Put(fileBuf)

	for i, f := range req.files {
		if f.name == "" && f.path == "" {
			return ErrFileNoName
		}

		// Set the file name if not provided.
		if f.name == "" && f.path != "" {
			f.path = filepath.Clean(f.path)
			f.name = filepath.Base(f.path)
		}

		// Set the field name if not provided.
		if f.fieldName == "" {
			f.fieldName = "file" + strconv.Itoa(i+1)
		}

		if err := addFormFile(mw, f, fileBuf); err != nil {
			return err
		}
	}

	return nil
}

func addFormFile(mw *multipart.Writer, f *File, fileBuf *[]byte) error {
	// If reader is not set, open the file.
	if f.reader == nil {
		var err error
		f.reader, err = os.Open(f.path)
		if err != nil {
			return fmt.Errorf("open file error: %w", err)
		}
	}

	// Ensure the file reader is always closed after copying.
	defer f.reader.Close() //nolint:errcheck // not needed

	// Create form file and copy the content.
	w, err := mw.CreateFormFile(f.fieldName, f.name)
	if err != nil {
		return fmt.Errorf("create file error: %w", err)
	}

	if _, err := io.CopyBuffer(w, f.reader, *fileBuf); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	return nil
}

// parserResponseCookie parses the Set-Cookie headers from the response and stores them.
func parserResponseCookie(c *Client, resp *Response, req *Request) error {
	for key, value := range resp.RawResponse.Header.Cookies() {
		cookie := fasthttp.AcquireCookie()
		if err := cookie.ParseBytes(value); err != nil {
			if err := parseCookieIgnoringBadAttrs(cookie, value); err != nil {
				fasthttp.ReleaseCookie(cookie)
				continue
			}
		}
		cookie.SetKeyBytes(key)
		resp.cookie = append(resp.cookie, cookie)
	}

	// Store cookies in the jar if available, keyed by the responding URI rather
	// than the requested one — otherwise a redirect's last hop could plant cookies
	// for an unrelated origin. Only the final response reaches this hook.
	if c.cookieJar != nil {
		host, path := resp.respondedOrigin(req.RawRequest.URI())
		c.cookieJar.parseCookiesFromResp(host, path, resp.RawResponse)
	}

	return nil
}

// parseCookieIgnoringBadAttrs parses a Set-Cookie value, dropping only the
// attributes fasthttp cannot parse. RFC 6265 §5.2 has an unparsable attribute
// ignored, while fasthttp abandons the cookie at the first one — losing the
// name, the value, and every attribute it had already accepted. Each attribute
// is offered against the ones kept so far, so those on either side of a bad one
// survive. Only a name/value pair that will not parse fails the cookie.
func parseCookieIgnoringBadAttrs(cookie *fasthttp.Cookie, value []byte) error {
	pair, rest, _ := bytes.Cut(value, []byte{';'})
	kept := append([]byte(nil), pair...)

	// ParseBytes resets the cookie, so a rejected attempt leaves nothing behind.
	trial := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(trial)
	if err := trial.ParseBytes(kept); err != nil {
		return err
	}

	for len(rest) > 0 {
		var attr []byte
		attr, rest, _ = bytes.Cut(rest, []byte{';'})
		candidate := append(append(append([]byte(nil), kept...), ';'), attr...)
		if err := trial.ParseBytes(candidate); err == nil {
			kept = candidate
		}
	}

	return cookie.ParseBytes(kept)
}

// logger is a response hook that logs request and response data if debug mode is enabled.
func logger(c *Client, resp *Response, req *Request) error {
	if !c.isDebug || c.logger == nil {
		return nil
	}

	// A streamed body is consumed by its first reader, so only the headers are logged.
	if req.RawRequest.IsBodyStream() {
		c.logger.Debugf("%s\n", req.RawRequest.Header.String())
	} else {
		c.logger.Debugf("%s\n", req.RawRequest.String())
	}
	if resp.RawResponse.IsBodyStream() {
		c.logger.Debugf("%s\n", resp.RawResponse.Header.String())
	} else {
		c.logger.Debugf("%s\n", resp.RawResponse.String())
	}

	return nil
}
