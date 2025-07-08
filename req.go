package fiber

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/idna"
)

// Range data for c.Range
type Range struct {
	Type   string
	Ranges []RangeSet
}

// RangeSet represents a single content range from a request.
type RangeSet struct {
	Start int
	End   int
}

//go:generate ifacemaker --file req.go --struct DefaultReq --iface Req --pkg fiber --output req_interface_gen.go --not-exported true --iface-comment "Req is an interface for request-related Ctx methods."
type DefaultReq struct {
	c Ctx
}

// Accepts checks if the specified extensions or content types are acceptable.
func (r *DefaultReq) Accepts(offers ...string) string {
	header := joinHeaderValues(r.Request().Header.PeekAll(HeaderAccept))
	return getOffer(header, acceptsOfferType, offers...)
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (r *DefaultReq) AcceptsCharsets(offers ...string) string {
	header := joinHeaderValues(r.Request().Header.PeekAll(HeaderAcceptCharset))
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (r *DefaultReq) AcceptsEncodings(offers ...string) string {
	header := joinHeaderValues(r.Request().Header.PeekAll(HeaderAcceptEncoding))
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsLanguages checks if the specified language is acceptable.
func (r *DefaultReq) AcceptsLanguages(offers ...string) string {
	header := joinHeaderValues(r.Request().Header.PeekAll(HeaderAcceptLanguage))
	return getOffer(header, acceptsLanguageOffer, offers...)
}

// App returns the *App reference to the instance of the Fiber application
func (r *DefaultReq) App() *App {
	return r.c.App()
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

func (r *DefaultReq) tryDecodeBodyInOrder(
	originalBody *[]byte,
	encodings []string,
) ([]byte, uint8, error) {
	var (
		err             error
		body            []byte
		decodesRealized uint8
	)

	for idx := range encodings {
		i := len(encodings) - 1 - idx
		encoding := encodings[i]
		decodesRealized++
		switch encoding {
		case StrGzip, "x-gzip":
			body, err = r.Request().BodyGunzip()
		case StrBr, StrBrotli:
			body, err = r.Request().BodyUnbrotli()
		case StrDeflate:
			body, err = r.Request().BodyInflate()
		case StrZstd:
			body, err = r.Request().BodyUnzstd()
		case StrIdentity:
			body = r.Request().Body()
		case StrCompress, "x-compress":
			return nil, decodesRealized - 1, ErrNotImplemented
		default:
			return nil, decodesRealized - 1, ErrUnsupportedMediaType
		}

		if err != nil {
			return nil, decodesRealized, err
		}

		if i > 0 && decodesRealized > 0 {
			if i == len(encodings)-1 {
				tempBody := r.Request().Body()
				*originalBody = make([]byte, len(tempBody))
				copy(*originalBody, tempBody)
			}
			r.Request().SetBodyRaw(body)
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

	// Get Content-Encoding header
	headerEncoding = utils.ToLower(utils.UnsafeString(r.Request().Header.ContentEncoding()))

	// If no encoding is provided, return the original body
	if len(headerEncoding) == 0 {
		return r.getBody()
	}

	// Split and get the encodings list, in order to attend the
	// rule defined at: https://www.rfc-editor.org/rfc/rfc9110#section-8.4-5
	encodingOrder = getSplicedStrList(headerEncoding, encodingOrder)
	for i := range encodingOrder {
		encodingOrder[i] = utils.ToLower(encodingOrder[i])
	}
	if len(encodingOrder) == 0 {
		return r.getBody()
	}

	var decodesRealized uint8
	body, decodesRealized, err = r.tryDecodeBodyInOrder(&originalBody, encodingOrder)

	// Ensure that the body will be the original
	if originalBody != nil && decodesRealized > 0 {
		r.Request().SetBodyRaw(originalBody)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrUnsupportedMediaType):
			_ = r.c.SendStatus(StatusUnsupportedMediaType) //nolint:errcheck // It is fine to ignore the error
		case errors.Is(err, ErrNotImplemented):
			_ = r.c.SendStatus(StatusNotImplemented) //nolint:errcheck // It is fine to ignore the error
		}
		return []byte(err.Error())
	}

	if r.App().config.Immutable {
		return utils.CopyBytes(body)
	}
	return body
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (r *DefaultReq) RequestCtx() *fasthttp.RequestCtx {
	return r.c.RequestCtx()
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Cookies(key string, defaultValue ...string) string {
	return defaultString(r.App().getString(r.Request().Header.Cookie(key)), defaultValue)
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (r *DefaultReq) Request() *fasthttp.Request {
	return r.c.Request()
}

// FormFile returns the first file by key from a MultipartForm.
func (r *DefaultReq) FormFile(key string) (*multipart.FileHeader, error) {
	return r.RequestCtx().FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
// Search is performed in QueryArgs, PostArgs, MultipartForm and FormFile in this particular order.
// Defaults to the empty string "" if the form value doesn't exist.
// If a default value is given, it will return that value if the form value does not exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) FormValue(key string, defaultValue ...string) string {
	return defaultString(r.App().getString(r.RequestCtx().FormValue(key)), defaultValue)
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/master/index.js#L33
func (r *DefaultReq) Fresh() bool {
	// fields
	modifiedSince := r.Get(HeaderIfModifiedSince)
	noneMatch := r.Get(HeaderIfNoneMatch)

	// unconditional request
	if modifiedSince == "" && noneMatch == "" {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://www.rfc-editor.org/rfc/rfc9111#section-5.2.1.4
	cacheControl := r.Get(HeaderCacheControl)
	if cacheControl != "" && isNoCache(cacheControl) {
		return false
	}

	// if-none-match
	if noneMatch != "" && noneMatch != "*" {
		etag := r.App().getString(r.c.Response().Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if r.App().isEtagStale(etag, r.App().getBytes(noneMatch)) {
			return false
		}

		if modifiedSince != "" {
			lastModified := r.App().getString(r.c.Response().Header.Peek(HeaderLastModified))
			if lastModified != "" {
				lastModifiedTime, err := http.ParseTime(lastModified)
				if err != nil {
					return false
				}
				modifiedSinceTime, err := http.ParseTime(modifiedSince)
				if err != nil {
					return false
				}
				return lastModifiedTime.Compare(modifiedSinceTime) != 1
			}
		}
	}
	return true
}

// Get returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) Get(key string, defaultValue ...string) string {
	return GetReqHeader(r, key, defaultValue...)
}

// GetReqHeader returns the HTTP request header specified by filed.
// This function is generic and can handle different headers type values.
// If the generic type cannot be matched to a supported type, the function
// returns the default value (if provided) or the zero value of type V.
func GetReqHeader[V GenericType](r Req, key string, defaultValue ...V) V {
	v, err := genericParseType[V](r.App().getString(r.Request().Header.Peek(key)))
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// GetHeaders returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) GetHeaders() map[string][]string {
	headers := make(map[string][]string)
	for k, v := range r.Request().Header.All() {
		key := r.App().getString(k)
		headers[key] = append(headers[key], r.App().getString(v))
	}
	return headers
}

// Host contains the host derived from the X-Forwarded-Host or Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// In a network context, `Host` refers to the combination of a hostname and potentially a port number used for connecting,
// while `Hostname` refers specifically to the name assigned to a device on a network, excluding any port information.
// Example: URL: https://example.com:8080 -> Host: example.com:8080
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (r *DefaultReq) Host() string {
	if r.IsProxyTrusted() {
		if host := r.Get(HeaderXForwardedHost); len(host) > 0 {
			commaPos := strings.Index(host, ",")
			if commaPos != -1 {
				return host[:commaPos]
			}
			return host
		}
	}
	return r.App().getString(r.Request().URI().Host())
}

// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header using the c.Host() method.
// Returned value is only valid within the handler. Do not store any references.
// Example: URL: https://example.com:8080 -> Hostname: example.com
// Make copies or use the Immutable setting instead.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (r *DefaultReq) Hostname() string {
	addr, _ := parseAddr(r.Host())

	return addr
}

// Port returns the remote port of the request.
func (r *DefaultReq) Port() string {
	tcpaddr, ok := r.RequestCtx().RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(errors.New("failed to type-assert to *net.TCPAddr"))
	}
	return strconv.Itoa(tcpaddr.Port)
}

// IP returns the remote IP address of the request.
// If ProxyHeader and IP Validation is configured, it will parse that header and return the first valid IP address.
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (r *DefaultReq) IP() string {
	if r.IsProxyTrusted() && len(r.App().config.ProxyHeader) > 0 {
		return r.extractIPFromHeader(r.App().config.ProxyHeader)
	}

	return r.RequestCtx().RemoteIP().String()
}

// extractIPsFromHeader will return a slice of IPs it found given a header name in the order they appear.
// When IP validation is enabled, any invalid IPs will be omitted.
func (r *DefaultReq) extractIPsFromHeader(header string) []string {
	// TODO: Reuse the c.extractIPFromHeader func somehow in here

	headerValue := r.Get(header)

	// We can't know how many IPs we will return, but we will try to guess with this constant division.
	// Counting ',' makes function slower for about 50ns in general case.
	const maxEstimatedCount = 8
	estimatedCount := len(headerValue) / maxEstimatedCount
	if estimatedCount > maxEstimatedCount {
		estimatedCount = maxEstimatedCount // Avoid big allocation on big header
	}

	ipsFound := make([]string, 0, estimatedCount)

	i := 0
	j := -1

iploop:
	for {
		var v4, v6 bool

		// Manually splitting string without allocating slice, working with parts directly
		i, j = j+1, j+2

		if j > len(headerValue) {
			break
		}

		for j < len(headerValue) && headerValue[j] != ',' {
			if headerValue[j] == ':' {
				v6 = true
			} else if headerValue[j] == '.' {
				v4 = true
			}
			j++
		}

		for i < j && (headerValue[i] == ' ' || headerValue[i] == ',') {
			i++
		}

		s := utils.TrimRight(headerValue[i:j], ' ')

		if r.App().config.EnableIPValidation {
			// Skip validation if IP is clearly not IPv4/IPv6, otherwise validate without allocations
			if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
				continue iploop
			}
		}

		ipsFound = append(ipsFound, s)
	}

	return ipsFound
}

// extractIPFromHeader will attempt to pull the real client IP from the given header when IP validation is enabled.
// currently, it will return the first valid IP address in header.
// when IP validation is disabled, it will simply return the value of the header without any inspection.
// Implementation is almost the same as in extractIPsFromHeader, but without allocation of []string.
func (r *DefaultReq) extractIPFromHeader(header string) string {
	if r.App().config.EnableIPValidation {
		headerValue := r.Get(header)

		i := 0
		j := -1

	iploop:
		for {
			var v4, v6 bool

			// Manually splitting string without allocating slice, working with parts directly
			i, j = j+1, j+2

			if j > len(headerValue) {
				break
			}

			for j < len(headerValue) && headerValue[j] != ',' {
				if headerValue[j] == ':' {
					v6 = true
				} else if headerValue[j] == '.' {
					v4 = true
				}
				j++
			}

			for i < j && headerValue[i] == ' ' {
				i++
			}

			s := utils.TrimRight(headerValue[i:j], ' ')

			if r.App().config.EnableIPValidation {
				if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
					continue iploop
				}
			}

			return s
		}

		return r.RequestCtx().RemoteIP().String()
	}

	// default behavior if IP validation is not enabled is just to return whatever value is
	// in the proxy header. Even if it is empty or invalid
	return r.Get(r.App().config.ProxyHeader)
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

	ct := r.App().getString(r.Request().Header.ContentType())
	if i := strings.IndexByte(ct, ';'); i != -1 {
		ct = ct[:i]
	}
	ct = utils.Trim(ct, ' ')
	return utils.EqualFold(ct, extensionHeader)
}

// Locals makes it possible to pass any values under keys scoped to the request
// and therefore available to all following routes that match the request.
//
// All the values are removed from ctx after returning from the top
// RequestHandler. Additionally, Close method is called on each value
// implementing io.Closer before removing the value from ctx.
func (r *DefaultReq) Locals(key any, value ...any) any {
	if len(value) == 0 {
		return r.RequestCtx().UserValue(key)
	}
	r.RequestCtx().SetUserValue(key, value[0])
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
	if len(override) == 0 {
		// Nothing to override, just return current method from context
		return r.App().method(r.c.getMethodInt())
	}

	method := utils.ToUpper(override[0])
	methodInt := r.App().methodInt(method)
	if methodInt == -1 {
		// Provided override does not valid HTTP method, no override, return current method
		return r.App().method(r.c.getMethodInt())
	}
	r.c.setMethodInt(methodInt)
	return method
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key the value will be a string slice.
func (r *DefaultReq) MultipartForm() (*multipart.Form, error) {
	return r.RequestCtx().MultipartForm()
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) OriginalURL() string {
	return r.App().getString(r.Request().Header.RequestURI())
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

	route := r.Route()
	for i := range route.Params {
		if len(key) != len(r.c.Route().Params[i]) {
			continue
		}
		if route.Params[i] == key || (!r.App().config.CaseSensitive && utils.EqualFold(route.Params[i], key)) {
			// in case values are not here
			if len(r.c.getValues()) <= i || len(r.c.getValues()[i]) == 0 {
				break
			}
			return r.c.getValues()[i]
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
// Please use Config.TrustProxy to prevent header spoofing, in case when your app is behind the proxy.
func (r *DefaultReq) Scheme() string {
	if r.RequestCtx().IsTLS() {
		return schemeHTTPS
	}
	if !r.IsProxyTrusted() {
		return schemeHTTP
	}

	scheme := schemeHTTP
	const lenXHeaderName = 12
	for key, val := range r.Request().Header.All() {
		if len(key) < lenXHeaderName {
			continue // Neither "X-Forwarded-" nor "X-Url-Scheme"
		}
		switch {
		case bytes.HasPrefix(key, []byte("X-Forwarded-")):
			if bytes.Equal(key, []byte(HeaderXForwardedProto)) ||
				bytes.Equal(key, []byte(HeaderXForwardedProtocol)) {
				v := r.App().getString(val)
				commaPos := strings.Index(v, ",")
				if commaPos != -1 {
					scheme = v[:commaPos]
				} else {
					scheme = v
				}
			} else if bytes.Equal(key, []byte(HeaderXForwardedSsl)) && bytes.Equal(val, []byte("on")) {
				scheme = schemeHTTPS
			}

		case bytes.Equal(key, []byte(HeaderXUrlScheme)):
			scheme = r.App().getString(val)
		}
	}
	return scheme
}

// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
func (r *DefaultReq) Protocol() string {
	return utils.UnsafeString(r.Request().Header.Protocol())
}

// Query returns the query string parameter in the url.
// Defaults to empty string "" if the query doesn't exist.
// If a default value is given, it will return that value if the query doesn't exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Query(key string, defaultValue ...string) string {
	return Query[string](r.c, key, defaultValue...)
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
	m := make(map[string]string, r.RequestCtx().QueryArgs().Len())
	for key, value := range r.RequestCtx().QueryArgs().All() {
		m[r.App().getString(key)] = r.App().getString(value)
	}
	return m
}

// Query Retrieves the value of a query parameter from the request's URI.
// The function is generic and can handle query parameter values of different types.
// It takes the following parameters:
// - c: The context object representing the current request.
// - key: The name of the query parameter.
// - defaultValue: (Optional) The default value to return in case the query parameter is not found or cannot be parsed.
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
	q := c.App().getString(c.RequestCtx().QueryArgs().Peek(key))
	v, err := genericParseType[V](q)
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// Range returns a struct containing the type and a slice of ranges.
func (r *DefaultReq) Range(size int) (Range, error) {
	var (
		rangeData Range
		ranges    string
	)
	rangeStr := utils.Trim(r.Get(HeaderRange), ' ')

	i := strings.IndexByte(rangeStr, '=')
	if i == -1 || strings.Contains(rangeStr[i+1:], "=") {
		return rangeData, ErrRangeMalformed
	}
	rangeData.Type = utils.ToLower(utils.Trim(rangeStr[:i], ' '))
	if rangeData.Type != "bytes" {
		return rangeData, ErrRangeMalformed
	}
	ranges = utils.Trim(rangeStr[i+1:], ' ')

	var (
		singleRange string
		moreRanges  = ranges
	)
	for moreRanges != "" {
		singleRange = moreRanges
		if i := strings.IndexByte(moreRanges, ','); i >= 0 {
			singleRange = moreRanges[:i]
			moreRanges = utils.Trim(moreRanges[i+1:], ' ')
		} else {
			moreRanges = ""
		}

		singleRange = utils.Trim(singleRange, ' ')

		var (
			startStr, endStr string
			i                int
		)
		if i = strings.IndexByte(singleRange, '-'); i == -1 {
			return rangeData, ErrRangeMalformed
		}
		startStr = utils.Trim(singleRange[:i], ' ')
		endStr = utils.Trim(singleRange[i+1:], ' ')

		start, startErr := fasthttp.ParseUint(utils.UnsafeBytes(startStr))
		end, endErr := fasthttp.ParseUint(utils.UnsafeBytes(endStr))
		if startErr != nil { // -nnn
			start = size - end
			end = size - 1
		} else if endErr != nil { // nnn-
			end = size - 1
		}
		if end > size-1 { // limit last-byte-pos to current length
			end = size - 1
		}
		if start > end || start < 0 {
			continue
		}
		rangeData.Ranges = append(rangeData.Ranges, struct {
			Start int
			End   int
		}{
			Start: start,
			End:   end,
		})
	}
	if len(rangeData.Ranges) < 1 {
		r.c.Status(StatusRequestedRangeNotSatisfiable)
		r.c.Set(HeaderContentRange, "bytes */"+strconv.Itoa(size))
		return rangeData, ErrRequestedRangeNotSatisfiable
	}

	return rangeData, nil
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
	host = utils.ToLower(host)

	// Decode punycode labels only when necessary
	if strings.Contains(host, "xn--") {
		if u, err := idna.Lookup.ToUnicode(host); err == nil {
			host = utils.ToLower(u)
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

	parts := strings.Split(host, ".")

	// offset == 0, caller wants everything.
	if o == 0 {
		return parts
	}

	// If we trim away the whole slice (or more), nothing remains.
	if o >= len(parts) {
		return []string{}
	}

	return parts[:len(parts)-o]
}

// Stale returns the inverse of Fresh, indicating if the client's cached response is considered stale.
func (r *DefaultReq) Stale() bool {
	return !r.Fresh()
}

// IsProxyTrusted checks trustworthiness of remote ip.
// If Config.TrustProxy false, it returns true
// IsProxyTrusted can check remote ip by proxy ranges and ip map.
func (r *DefaultReq) IsProxyTrusted() bool {
	if !r.App().config.TrustProxy {
		return true
	}

	ip := r.RequestCtx().RemoteIP()

	if (r.App().config.TrustProxyConfig.Loopback && ip.IsLoopback()) ||
		(r.App().config.TrustProxyConfig.Private && ip.IsPrivate()) ||
		(r.App().config.TrustProxyConfig.LinkLocal && ip.IsLinkLocalUnicast()) {
		return true
	}

	if _, trusted := r.App().config.TrustProxyConfig.ips[ip.String()]; trusted {
		return true
	}

	for _, ipNet := range r.App().config.TrustProxyConfig.ranges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsFromLocal will return true if request came from local.
func (r *DefaultReq) IsFromLocal() bool {
	return r.RequestCtx().RemoteIP().IsLoopback()
}

// Release is a method to reset Req fields when to use ReleaseCtx()
func (r *DefaultReq) release() {
	r.c = nil
}

func (r *DefaultReq) getBody() []byte {
	if r.App().config.Immutable {
		return utils.CopyBytes(r.Request().Body())
	}

	return r.Request().Body()
}
