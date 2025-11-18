package fiber

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"net"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/idna"
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
	header := joinHeaderValues(r.c.fasthttp.Request.Header.PeekAll(HeaderAccept))
	return getOffer(header, acceptsOfferType, offers...)
}

// AcceptsCharsets checks if the specified charset is acceptable.
func (r *DefaultReq) AcceptsCharsets(offers ...string) string {
	header := joinHeaderValues(r.c.fasthttp.Request.Header.PeekAll(HeaderAcceptCharset))
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsEncodings checks if the specified encoding is acceptable.
func (r *DefaultReq) AcceptsEncodings(offers ...string) string {
	header := joinHeaderValues(r.c.fasthttp.Request.Header.PeekAll(HeaderAcceptEncoding))
	return getOffer(header, acceptsOffer, offers...)
}

// AcceptsLanguages checks if the specified language is acceptable using
// RFC 4647 Basic Filtering.
func (r *DefaultReq) AcceptsLanguages(offers ...string) string {
	header := joinHeaderValues(r.c.fasthttp.Request.Header.PeekAll(HeaderAcceptLanguage))
	return getOffer(header, acceptsLanguageOfferBasic, offers...)
}

// AcceptsLanguagesExtended checks if the specified language is acceptable using
// RFC 4647 Extended Filtering.
func (r *DefaultReq) AcceptsLanguagesExtended(offers ...string) string {
	header := joinHeaderValues(r.c.fasthttp.Request.Header.PeekAll(HeaderAcceptLanguage))
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
	for idx := range encodings {
		i := len(encodings) - 1 - idx
		encoding := encodings[i]
		decodesRealized++
		var decodeErr error
		switch encoding {
		case StrGzip, "x-gzip":
			body, decodeErr = request.BodyGunzip()
		case StrBr, StrBrotli:
			body, decodeErr = request.BodyUnbrotli()
		case StrDeflate:
			body, decodeErr = request.BodyInflate()
		case StrZstd:
			body, decodeErr = request.BodyUnzstd()
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
			request.SetBodyRaw(body)
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

	// Get Content-Encoding header
	headerEncoding = utils.ToLower(utils.UnsafeString(request.Header.ContentEncoding()))

	// If no encoding is provided, return the original body
	if headerEncoding == "" {
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
		request.SetBodyRaw(originalBody)
	}
	if err != nil {
		switch {
		case errors.Is(err, ErrUnsupportedMediaType):
			_ = r.c.DefaultRes.SendStatus(StatusUnsupportedMediaType) //nolint:errcheck,staticcheck // It is fine to ignore the error and the static check
		case errors.Is(err, ErrNotImplemented):
			_ = r.c.DefaultRes.SendStatus(StatusNotImplemented) //nolint:errcheck,staticcheck // It is fine to ignore the error and the static check
		default:
			// do nothing
		}
		return []byte(err.Error())
	}

	return r.c.app.GetBytes(body)
}

// RequestCtx returns *fasthttp.RequestCtx that carries a deadline
// a cancellation signal, and other values across API boundaries.
func (r *DefaultReq) RequestCtx() *fasthttp.RequestCtx {
	return r.c.fasthttp
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) Cookies(key string, defaultValue ...string) string {
	return defaultString(r.c.app.toString(r.c.fasthttp.Request.Header.Cookie(key)), defaultValue)
}

// Request return the *fasthttp.Request object
// This allows you to use all fasthttp request methods
// https://godoc.org/github.com/valyala/fasthttp#Request
func (r *DefaultReq) Request() *fasthttp.Request {
	return &r.c.fasthttp.Request
}

// FormFile returns the first file by key from a MultipartForm.
func (r *DefaultReq) FormFile(key string) (*multipart.FileHeader, error) {
	return r.c.fasthttp.FormFile(key)
}

// FormValue returns the first value by key from a MultipartForm.
// Search is performed in QueryArgs, PostArgs, MultipartForm and FormFile in this particular order.
// Defaults to the empty string "" if the form value doesn't exist.
// If a default value is given, it will return that value if the form value does not exist.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) FormValue(key string, defaultValue ...string) string {
	return defaultString(r.c.app.toString(r.c.fasthttp.FormValue(key)), defaultValue)
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/master/index.js#L33
func (r *DefaultReq) Fresh() bool {
	header := &r.c.fasthttp.Request.Header

	// fields
	modifiedSince := header.Peek(HeaderIfModifiedSince)
	noneMatch := header.Peek(HeaderIfNoneMatch)

	// unconditional request
	if len(modifiedSince) == 0 && len(noneMatch) == 0 {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://www.rfc-editor.org/rfc/rfc9111#section-5.2.1.4
	cacheControl := header.Peek(HeaderCacheControl)
	if len(cacheControl) > 0 && isNoCache(utils.UnsafeString(cacheControl)) {
		return false
	}

	// if-none-match
	if len(noneMatch) > 0 && (len(noneMatch) != 1 || noneMatch[0] != '*') {
		app := r.c.app
		response := &r.c.fasthttp.Response
		etag := app.toString(response.Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if app.isEtagStale(etag, noneMatch) {
			return false
		}

		if len(modifiedSince) > 0 {
			lastModified := response.Header.Peek(HeaderLastModified)
			if len(lastModified) > 0 {
				lastModifiedTime, err := fasthttp.ParseHTTPDate(lastModified)
				if err != nil {
					return false
				}
				modifiedSinceTime, err := fasthttp.ParseHTTPDate(modifiedSince)
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
	return GetReqHeader(r.c, key, defaultValue...)
}

// GetReqHeader returns the HTTP request header specified by filed.
// This function is generic and can handle different headers type values.
// If the generic type cannot be matched to a supported type, the function
// returns the default value (if provided) or the zero value of type V.
func GetReqHeader[V GenericType](c Ctx, key string, defaultValue ...V) V {
	v, err := genericParseType[V](c.App().toString(c.Request().Header.Peek(key)))
	if err != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return v
}

// GetHeaders (a.k.a GetReqHeaders) returns the HTTP request headers.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *DefaultReq) GetHeaders() map[string][]string {
	app := r.c.app
	headers := make(map[string][]string)
	for k, v := range r.c.fasthttp.Request.Header.All() {
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
			commaPos := strings.Index(host, ",")
			if commaPos != -1 {
				return host[:commaPos]
			}
			return host
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
	tcpaddr, ok := r.c.fasthttp.RemoteAddr().(*net.TCPAddr)
	if !ok {
		panic(errTCPAddrTypeAssertion)
	}
	return strconv.Itoa(tcpaddr.Port)
}

// IP returns the remote IP address of the request.
// If ProxyHeader and IP Validation is configured, it will parse that header and return the first valid IP address.
// Please use Config.TrustProxy to prevent header spoofing if your app is behind a proxy.
func (r *DefaultReq) IP() string {
	app := r.c.app
	if r.IsProxyTrusted() && app.config.ProxyHeader != "" {
		return r.extractIPFromHeader(app.config.ProxyHeader)
	}

	return r.c.fasthttp.RemoteIP().String()
}

// extractIPsFromHeader will return a slice of IPs it found given a header name in the order they appear.
// When IP validation is enabled, any invalid IPs will be omitted.
func (r *DefaultReq) extractIPsFromHeader(header string) []string {
	// TODO: Reuse the c.extractIPFromHeader func somehow in here

	headerValue := r.Get(header)

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
			// Skip validation if IP is clearly not IPv4/IPv6; otherwise, validate without allocations
			if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
				continue
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
	app := r.c.app
	if app.config.EnableIPValidation {
		headerValue := r.Get(header)

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

			for i < j && headerValue[i] == ' ' {
				i++
			}

			s := utils.TrimRight(headerValue[i:j], ' ')

			if app.config.EnableIPValidation {
				if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
					continue
				}
			}

			return s
		}

		return r.c.fasthttp.RemoteIP().String()
	}

	// default behavior if IP validation is not enabled is just to return whatever value is
	// in the proxy header. Even if it is empty or invalid
	return r.Get(app.config.ProxyHeader)
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
		return app.method(r.c.methodInt)
	}

	method := utils.ToUpper(override[0])
	methodInt := app.methodInt(method)
	if methodInt == -1 {
		// Provided override does not valid HTTP method, no override, return current method
		return app.method(r.c.methodInt)
	}
	r.c.methodInt = methodInt
	return method
}

// MultipartForm parse form entries from binary.
// This returns a map[string][]string, so given a key, the value will be a string slice.
func (r *DefaultReq) MultipartForm() (*multipart.Form, error) {
	return r.c.fasthttp.MultipartForm()
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *DefaultReq) OriginalURL() string {
	return r.c.app.toString(r.c.fasthttp.Request.Header.RequestURI())
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
		case bytes.HasPrefix(key, []byte("X-Forwarded-")):
			if bytes.Equal(key, []byte(HeaderXForwardedProto)) ||
				bytes.Equal(key, []byte(HeaderXForwardedProtocol)) {
				v := app.toString(val)
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
			scheme = app.toString(val)
		default:
			continue
		}
	}
	return scheme
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
	rangeStr := utils.Trim(r.Get(HeaderRange), ' ')

	parseBound := func(value string) (int64, error) {
		parsed, err := utils.ParseUint(value)
		if err != nil {
			return 0, fmt.Errorf("parse range bound %q: %w", value, err)
		}
		if parsed > (math.MaxUint64 >> 1) {
			return 0, ErrRangeMalformed
		}
		return int64(parsed), nil
	}

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

		start, startErr := parseBound(startStr)
		end, endErr := parseBound(endStr)
		if errors.Is(startErr, ErrRangeMalformed) || errors.Is(endErr, ErrRangeMalformed) {
			return rangeData, ErrRangeMalformed
		}
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
		rangeData.Ranges = append(rangeData.Ranges, RangeSet{
			Start: start,
			End:   end,
		})
	}
	if len(rangeData.Ranges) < 1 {
		r.c.DefaultRes.Status(StatusRequestedRangeNotSatisfiable)
		r.c.DefaultRes.Set(HeaderContentRange, "bytes */"+strconv.FormatInt(size, 10)) //nolint:staticcheck // It is fine to ignore the static check
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
	config := r.c.app.config
	if !config.TrustProxy {
		return true
	}

	ip := r.c.fasthttp.RemoteIP()

	if (config.TrustProxyConfig.Loopback && ip.IsLoopback()) ||
		(config.TrustProxyConfig.Private && ip.IsPrivate()) ||
		(config.TrustProxyConfig.LinkLocal && ip.IsLinkLocalUnicast()) {
		return true
	}

	if _, trusted := config.TrustProxyConfig.ips[ip.String()]; trusted {
		return true
	}

	for _, ipNet := range config.TrustProxyConfig.ranges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// IsFromLocal will return true if request came from local.
func (r *DefaultReq) IsFromLocal() bool {
	return r.c.fasthttp.RemoteIP().IsLoopback()
}

// Release is a method to reset Req fields when to use ReleaseCtx()
func (r *DefaultReq) release() {
	r.c = nil
}

func (r *DefaultReq) getBody() []byte {
	return r.c.app.GetBytes(r.c.fasthttp.Request.Body())
}
