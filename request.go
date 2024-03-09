package fiber

import (
	"bytes"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

type Request struct {
	app      *App              // Reference to the parent App.
	ctx      Ctx               // Reference to the parent Ctx.
	fasthttp *fasthttp.Request // Reference to the underlying fasthttp.Request object.
	baseURI  string            // HTTP base URI for memoization.
}

func (r *Request) App() *App {
	return r.app
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
