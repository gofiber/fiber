package fiber

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type Request struct {
	app       *App              // Reference to the parent App.
	ctx       Ctx               // Reference to the parent Ctx.
	fasthttp  *fasthttp.Request // Reference to the underlying fasthttp.Request object.
	baseURI   string            // Memoized base HTTP URI of the current request.
	method    string            // HTTP method
	methodINT int               // HTTP method INT equivalent
}

func (r *Request) App() *App {
	return r.app
}

// Method returns the HTTP request method for the context, optionally overridden by the provided argument.
// If no override is given or if the provided override is not a valid HTTP method, it returns the current method from the context.
// Otherwise, it updates the context's method and returns the overridden method as a string.
func (r *Request) Method(override ...string) string {
	if len(override) == 0 {
		// Nothing to override, just return current method from context
		return r.method
	}

	method := utils.ToUpper(override[0])
	mINT := r.app.methodInt(method)
	if mINT == -1 {
		// Provided override does not valid HTTP method, no override, return current method
		return r.method
	}

	r.method = method
	r.methodINT = mINT
	return r.method
}

// OriginalURL contains the original request URL.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *Request) OriginalURL() string {
	return r.app.getString(r.fasthttp.Header.RequestURI())
}

// BaseURL returns (protocol + host + base path).
func (r *Request) BaseURL() string {
	// TODO: Could be improved: 53.8 ns/op  32 B/op  1 allocs/op
	// Should work like https://codeigniter.com/user_guide/helpers/url_helper.html
	if r.baseURI != "" {
		return r.baseURI
	}
	r.baseURI = r.Scheme() + "://" + r.Host()
	return r.baseURI
}

// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
func (r *Request) Protocol() string {
	return r.app.getString(r.fasthttp.Header.Protocol())
}

// Scheme contains the request protocol string: http or https for TLS requests.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (r *Request) Scheme() string {
	if string(r.fasthttp.URI().Scheme()) == "https" {
		return schemeHTTPS
	}
	if !r.ctx.IsProxyTrusted() {
		return schemeHTTP
	}

	scheme := schemeHTTP
	const lenXHeaderName = 12
	r.fasthttp.Header.VisitAll(func(key, val []byte) {
		if len(key) < lenXHeaderName {
			return // Neither "X-Forwarded-" nor "X-Url-Scheme"
		}
		switch {
		case bytes.HasPrefix(key, []byte("X-Forwarded-")):
			if string(key) == HeaderXForwardedProto ||
				string(key) == HeaderXForwardedProtocol {
				v := r.app.getString(val)
				commaPos := strings.IndexByte(v, ',')
				if commaPos != -1 {
					scheme = v[:commaPos]
				} else {
					scheme = v
				}
			} else if string(key) == HeaderXForwardedSsl && string(val) == "on" {
				scheme = schemeHTTPS
			}

		case string(key) == HeaderXUrlScheme:
			scheme = r.app.getString(val)
		}
	})
	return scheme
}

// Host contains the host derived from the X-Forwarded-Host or Host HTTP header.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (r *Request) Host() string {
	if r.ctx.IsProxyTrusted() {
		if host := r.Get(HeaderXForwardedHost); len(host) > 0 {
			commaPos := strings.Index(host, ",")
			if commaPos != -1 {
				return host[:commaPos]
			}
			return host
		}
	}
	return r.app.getString(r.fasthttp.URI().Host())
}

// Hostname contains the hostname derived from the X-Forwarded-Host or Host HTTP header using the r.Host() method.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (r *Request) Hostname() string {
	addr, _ := parseAddr(r.Host())

	return addr
}

// IP returns the remote IP address of the request.
// If ProxyHeader and IP Validation is configured, it will parse that header and return the first valid IP address.
// Please use Config.EnableTrustedProxyCheck to prevent header spoofing, in case when your app is behind the proxy.
func (r *Request) IP() string {
	if r.ctx.IsProxyTrusted() && len(r.app.config.ProxyHeader) > 0 {
		return r.extractIPFromHeader(r.app.config.ProxyHeader)
	}

	return r.ctx.Context().RemoteIP().String()
}

// extractIPFromHeader will attempt to pull the real client IP from the given header when IP validation is enabled.
// currently, it will return the first valid IP address in header.
// when IP validation is disabled, it will simply return the value of the header without any inspection.
// Implementation is almost the same as in extractIPsFromHeader, but without allocation of []string.
func (r *Request) extractIPFromHeader(header string) string {
	if r.app.config.EnableIPValidation {
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

			s := strings.TrimRight(headerValue[i:j], " ")

			if r.app.config.EnableIPValidation {
				if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
					continue iploop
				}
			}

			return s
		}

		return r.ctx.Context().RemoteIP().String()
	}

	// default behavior if IP validation is not enabled is just to return whatever value is
	// in the proxy header. Even if it is empty or invalid
	return r.Get(r.app.config.ProxyHeader)
}

// IPs returns a string slice of IP addresses specified in the X-Forwarded-For request header.
// When IP validation is enabled, only valid IPs are returned.
func (r *Request) IPs() []string {
	return r.extractIPsFromHeader(HeaderXForwardedFor)
}

// extractIPsFromHeader will return a slice of IPs it found given a header name in the order they appear.
// When IP validation is enabled, any invalid IPs will be omitted.
func (r *Request) extractIPsFromHeader(header string) []string {
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

		s := strings.TrimRight(headerValue[i:j], " ")

		if r.app.config.EnableIPValidation {
			// Skip validation if IP is clearly not IPv4/IPv6, otherwise validate without allocations
			if (!v6 && !v4) || (v6 && !utils.IsIPv6(s)) || (v4 && !utils.IsIPv4(s)) {
				continue iploop
			}
		}

		ipsFound = append(ipsFound, s)
	}

	return ipsFound
}

// BodyRaw contains the raw body submitted in a POST request.
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *Request) BodyRaw() []byte {
	if r.app.config.Immutable {
		return utils.CopyBytes(r.fasthttp.Body())
	}
	return r.fasthttp.Body()
}

// Body contains the raw body submitted in a POST request.
// This method will decompress the body if the 'Content-Encoding' header is provided.
// It returns the original (or decompressed) body data which is valid only within the handler.
// Don't store direct references to the returned data.
// If you need to keep the body's data later, make a copy or use the Immutable option.
func (r *Request) Body() []byte {
	var (
		err                error
		body, originalBody []byte
		headerEncoding     string
		encodingOrder      = []string{"", "", ""}
	)

	// faster than peek
	r.fasthttp.Header.VisitAll(func(key, value []byte) {
		if r.app.getString(key) == HeaderContentEncoding {
			headerEncoding = r.app.getString(value)
		}
	})

	// Split and get the encodings list, in order to attend the
	// rule defined at: https://www.rfc-editor.org/rfc/rfc9110#section-8.4-5
	encodingOrder = getSplicedStrList(headerEncoding, encodingOrder)
	if len(encodingOrder) == 0 {
		if r.app.config.Immutable {
			return utils.CopyBytes(r.fasthttp.Body())
		}
		return r.fasthttp.Body()
	}

	var decodesRealized uint8
	body, decodesRealized, err = r.tryDecodeBodyInOrder(&originalBody, encodingOrder)

	// Ensure that the body will be the original
	if originalBody != nil && decodesRealized > 0 {
		r.fasthttp.SetBodyRaw(originalBody)
	}
	if err != nil {
		return []byte(err.Error())
	}

	if r.app.config.Immutable {
		return utils.CopyBytes(body)
	}
	return body
}

func (r *Request) tryDecodeBodyInOrder(
	originalBody *[]byte,
	encodings []string,
) ([]byte, uint8, error) {
	var (
		err             error
		body            []byte
		decodesRealized uint8
	)

	for index, encoding := range encodings {
		decodesRealized++
		switch encoding {
		case StrGzip:
			body, err = r.fasthttp.BodyGunzip()
		case StrBr, StrBrotli:
			body, err = r.fasthttp.BodyUnbrotli()
		case StrDeflate:
			body, err = r.fasthttp.BodyInflate()
		default:
			decodesRealized--
			if len(encodings) == 1 {
				body = r.fasthttp.Body()
			}
			return body, decodesRealized, nil
		}

		if err != nil {
			return nil, decodesRealized, err
		}

		// Only execute body raw update if it has a next iteration to try to decode
		if index < len(encodings)-1 && decodesRealized > 0 {
			if index == 0 {
				tempBody := r.fasthttp.Body()
				*originalBody = make([]byte, len(tempBody))
				copy(*originalBody, tempBody)
			}
			r.fasthttp.SetBodyRaw(body)
		}
	}

	return body, decodesRealized, nil
}

// Get returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (r *Request) Get(key string, defaultValue ...string) string {
	return defaultString(r.app.getString(r.fasthttp.Header.Peek(key)), defaultValue)
}

// Cookies are used for getting a cookie value by key.
// Defaults to the empty string "" if the cookie doesn't exist.
// If a default value is given, it will return that value if the cookie doesn't exist.
// The returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting to use the value outside the Handler.
func (r *Request) Cookies(key string, defaultValue ...string) string {
	return defaultString(r.app.getString(r.fasthttp.Header.Cookie(key)), defaultValue)
}

// Fresh returns true when the response is still “fresh” in the client's cache,
// otherwise false is returned to indicate that the client cache is now stale
// and the full response should be sent.
// When a client sends the Cache-Control: no-cache request header to indicate an end-to-end
// reload request, this module will return false to make handling these requests transparent.
// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L33
func (r *Request) Fresh() bool {
	// fields
	modifiedSince := r.Get(HeaderIfModifiedSince)
	noneMatch := r.Get(HeaderIfNoneMatch)

	// unconditional request
	if modifiedSince == "" && noneMatch == "" {
		return false
	}

	// Always return stale when Cache-Control: no-cache
	// to support end-to-end reload requests
	// https://tools.ietf.org/html/rfc2616#section-14.9.4
	cacheControl := r.Get(HeaderCacheControl)
	if cacheControl != "" && isNoCache(cacheControl) {
		return false
	}

	// if-none-match
	if noneMatch != "" && noneMatch != "*" {
		etag := r.app.getString(r.ctx.Response().Header.Peek(HeaderETag))
		if etag == "" {
			return false
		}
		if r.app.isEtagStale(etag, r.app.getBytes(noneMatch)) {
			return false
		}

		if modifiedSince != "" {
			lastModified := r.app.getString(r.ctx.Response().Header.Peek(HeaderLastModified))
			if lastModified != "" {
				lastModifiedTime, err := http.ParseTime(lastModified)
				if err != nil {
					return false
				}
				modifiedSinceTime, err := http.ParseTime(modifiedSince)
				if err != nil {
					return false
				}
				return lastModifiedTime.Before(modifiedSinceTime)
			}
		}
	}
	return true
}

// Secure returns whether a secure connection was established.
func (r *Request) Secure() bool {
	return r.Protocol() == schemeHTTPS
}

// Stale is the opposite of [Request.Fresh] and returns true when the response
// to this request is no longer "fresh" in the client's cache.
func (r *Request) Stale() bool {
	return !r.Fresh()
}

// Subdomains returns a string slice of subdomains in the domain name of the request.
// The subdomain offset, which defaults to 2, is used for determining the beginning of the subdomain segments.
func (r *Request) Subdomains(offset ...int) []string {
	o := 2
	if len(offset) > 0 {
		o = offset[0]
	}
	subdomains := strings.Split(r.Host(), ".")
	l := len(subdomains) - o
	// Check index to avoid slice bounds out of range panic
	if l < 0 {
		l = len(subdomains)
	}
	subdomains = subdomains[:l]
	return subdomains
}

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (r *Request) XHR() bool {
	return utils.EqualFold(r.Get(HeaderXRequestedWith), "xmlhttprequest")
}
