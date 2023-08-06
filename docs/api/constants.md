---
id: constants
title: 📋 Constants
description: Some constants for Fiber.
sidebar_position: 4
---

HTTP methods were copied from net/http.

```go
const (
    MethodGet     = "GET"     // RFC 7231, 4.3.1
    MethodHead    = "HEAD"    // RFC 7231, 4.3.2
    MethodPost    = "POST"    // RFC 7231, 4.3.3
    MethodPut     = "PUT"     // RFC 7231, 4.3.4
    MethodPatch   = "PATCH"   // RFC 5789
    MethodDelete  = "DELETE"  // RFC 7231, 4.3.5
    MethodConnect = "CONNECT" // RFC 7231, 4.3.6
    MethodOptions = "OPTIONS" // RFC 7231, 4.3.7
    MethodTrace   = "TRACE"   // RFC 7231, 4.3.8
    methodUse     = "USE"
)
```

MIME types that are commonly used

```go
const (
	MIMETextXML               = "text/xml"
	MIMETextHTML              = "text/html"
	MIMETextPlain             = "text/plain"
	MIMEApplicationXML        = "application/xml"
	MIMEApplicationJSON       = "application/json"
	MIMEApplicationJavaScript = "application/javascript"
	MIMEApplicationForm       = "application/x-www-form-urlencoded"
	MIMEOctetStream           = "application/octet-stream"
	MIMEMultipartForm         = "multipart/form-data"

	MIMETextXMLCharsetUTF8               = "text/xml; charset=utf-8"
	MIMETextHTMLCharsetUTF8              = "text/html; charset=utf-8"
	MIMETextPlainCharsetUTF8             = "text/plain; charset=utf-8"
	MIMEApplicationXMLCharsetUTF8        = "application/xml; charset=utf-8"
	MIMEApplicationJSONCharsetUTF8       = "application/json; charset=utf-8"
	MIMEApplicationJavaScriptCharsetUTF8 = "application/javascript; charset=utf-8"
)
```

HTTP status codes were copied from net/http.

```go
const (
	StatusContinue                      = 100 // RFC 7231, 6.2.1
	StatusSwitchingProtocols            = 101 // RFC 7231, 6.2.2
	StatusProcessing                    = 102 // RFC 2518, 10.1
	StatusEarlyHints                    = 103 // RFC 8297
	StatusOK                            = 200 // RFC 7231, 6.3.1
	StatusCreated                       = 201 // RFC 7231, 6.3.2
	StatusAccepted                      = 202 // RFC 7231, 6.3.3
	StatusNonAuthoritativeInformation   = 203 // RFC 7231, 6.3.4
	StatusNoContent                     = 204 // RFC 7231, 6.3.5
	StatusResetContent                  = 205 // RFC 7231, 6.3.6
	StatusPartialContent                = 206 // RFC 7233, 4.1
	StatusMultiStatus                   = 207 // RFC 4918, 11.1
	StatusAlreadyReported               = 208 // RFC 5842, 7.1
	StatusIMUsed                        = 226 // RFC 3229, 10.4.1
	StatusMultipleChoices               = 300 // RFC 7231, 6.4.1
	StatusMovedPermanently              = 301 // RFC 7231, 6.4.2
	StatusFound                         = 302 // RFC 7231, 6.4.3
	StatusSeeOther                      = 303 // RFC 7231, 6.4.4
	StatusNotModified                   = 304 // RFC 7232, 4.1
	StatusUseProxy                      = 305 // RFC 7231, 6.4.5
	StatusTemporaryRedirect             = 307 // RFC 7231, 6.4.7
	StatusPermanentRedirect             = 308 // RFC 7538, 3
	StatusBadRequest                    = 400 // RFC 7231, 6.5.1
	StatusUnauthorized                  = 401 // RFC 7235, 3.1
	StatusPaymentRequired               = 402 // RFC 7231, 6.5.2
	StatusForbidden                     = 403 // RFC 7231, 6.5.3
	StatusNotFound                      = 404 // RFC 7231, 6.5.4
	StatusMethodNotAllowed              = 405 // RFC 7231, 6.5.5
	StatusNotAcceptable                 = 406 // RFC 7231, 6.5.6
	StatusProxyAuthRequired             = 407 // RFC 7235, 3.2
	StatusRequestTimeout                = 408 // RFC 7231, 6.5.7
	StatusConflict                      = 409 // RFC 7231, 6.5.8
	StatusGone                          = 410 // RFC 7231, 6.5.9
	StatusLengthRequired                = 411 // RFC 7231, 6.5.10
	StatusPreconditionFailed            = 412 // RFC 7232, 4.2
	StatusRequestEntityTooLarge         = 413 // RFC 7231, 6.5.11
	StatusRequestURITooLong             = 414 // RFC 7231, 6.5.12
	StatusUnsupportedMediaType          = 415 // RFC 7231, 6.5.13
	StatusRequestedRangeNotSatisfiable  = 416 // RFC 7233, 4.4
	StatusExpectationFailed             = 417 // RFC 7231, 6.5.14
	StatusTeapot                        = 418 // RFC 7168, 2.3.3
	StatusMisdirectedRequest            = 421 // RFC 7540, 9.1.2
	StatusUnprocessableEntity           = 422 // RFC 4918, 11.2
	StatusLocked                        = 423 // RFC 4918, 11.3
	StatusFailedDependency              = 424 // RFC 4918, 11.4
	StatusTooEarly                      = 425 // RFC 8470, 5.2.
	StatusUpgradeRequired               = 426 // RFC 7231, 6.5.15
	StatusPreconditionRequired          = 428 // RFC 6585, 3
	StatusTooManyRequests               = 429 // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge   = 431 // RFC 6585, 5
	StatusUnavailableForLegalReasons    = 451 // RFC 7725, 3
	StatusInternalServerError           = 500 // RFC 7231, 6.6.1
	StatusNotImplemented                = 501 // RFC 7231, 6.6.2
	StatusBadGateway                    = 502 // RFC 7231, 6.6.3
	StatusServiceUnavailable            = 503 // RFC 7231, 6.6.4
	StatusGatewayTimeout                = 504 // RFC 7231, 6.6.5
	StatusHTTPVersionNotSupported       = 505 // RFC 7231, 6.6.6
	StatusVariantAlsoNegotiates         = 506 // RFC 2295, 8.1
	StatusInsufficientStorage           = 507 // RFC 4918, 11.5
	StatusLoopDetected                  = 508 // RFC 5842, 7.2
	StatusNotExtended                   = 510 // RFC 2774, 7
	StatusNetworkAuthenticationRequired = 511 // RFC 6585, 6
)
```

Errors

```go
var (
	ErrBadRequest                    = NewError(StatusBadRequest)                    // RFC 7231, 6.5.1
	ErrUnauthorized                  = NewError(StatusUnauthorized)                  // RFC 7235, 3.1
	ErrPaymentRequired               = NewError(StatusPaymentRequired)               // RFC 7231, 6.5.2
	ErrForbidden                     = NewError(StatusForbidden)                     // RFC 7231, 6.5.3
	ErrNotFound                      = NewError(StatusNotFound)                      // RFC 7231, 6.5.4
	ErrMethodNotAllowed              = NewError(StatusMethodNotAllowed)              // RFC 7231, 6.5.5
	ErrNotAcceptable                 = NewError(StatusNotAcceptable)                 // RFC 7231, 6.5.6
	ErrProxyAuthRequired             = NewError(StatusProxyAuthRequired)             // RFC 7235, 3.2
	ErrRequestTimeout                = NewError(StatusRequestTimeout)                // RFC 7231, 6.5.7
	ErrConflict                      = NewError(StatusConflict)                      // RFC 7231, 6.5.8
	ErrGone                          = NewError(StatusGone)                          // RFC 7231, 6.5.9
	ErrLengthRequired                = NewError(StatusLengthRequired)                // RFC 7231, 6.5.10
	ErrPreconditionFailed            = NewError(StatusPreconditionFailed)            // RFC 7232, 4.2
	ErrRequestEntityTooLarge         = NewError(StatusRequestEntityTooLarge)         // RFC 7231, 6.5.11
	ErrRequestURITooLong             = NewError(StatusRequestURITooLong)             // RFC 7231, 6.5.12
	ErrUnsupportedMediaType          = NewError(StatusUnsupportedMediaType)          // RFC 7231, 6.5.13
	ErrRequestedRangeNotSatisfiable  = NewError(StatusRequestedRangeNotSatisfiable)  // RFC 7233, 4.4
	ErrExpectationFailed             = NewError(StatusExpectationFailed)             // RFC 7231, 6.5.14
	ErrTeapot                        = NewError(StatusTeapot)                        // RFC 7168, 2.3.3
	ErrMisdirectedRequest            = NewError(StatusMisdirectedRequest)            // RFC 7540, 9.1.2
	ErrUnprocessableEntity           = NewError(StatusUnprocessableEntity)           // RFC 4918, 11.2
	ErrLocked                        = NewError(StatusLocked)                        // RFC 4918, 11.3
	ErrFailedDependency              = NewError(StatusFailedDependency)              // RFC 4918, 11.4
	ErrTooEarly                      = NewError(StatusTooEarly)                      // RFC 8470, 5.2.
	ErrUpgradeRequired               = NewError(StatusUpgradeRequired)               // RFC 7231, 6.5.15
	ErrPreconditionRequired          = NewError(StatusPreconditionRequired)          // RFC 6585, 3
	ErrTooManyRequests               = NewError(StatusTooManyRequests)               // RFC 6585, 4
	ErrRequestHeaderFieldsTooLarge   = NewError(StatusRequestHeaderFieldsTooLarge)   // RFC 6585, 5
	ErrUnavailableForLegalReasons    = NewError(StatusUnavailableForLegalReasons)    // RFC 7725, 3
	ErrInternalServerError           = NewError(StatusInternalServerError)           // RFC 7231, 6.6.1
	ErrNotImplemented                = NewError(StatusNotImplemented)                // RFC 7231, 6.6.2
	ErrBadGateway                    = NewError(StatusBadGateway)                    // RFC 7231, 6.6.3
	ErrServiceUnavailable            = NewError(StatusServiceUnavailable)            // RFC 7231, 6.6.4
	ErrGatewayTimeout                = NewError(StatusGatewayTimeout)                // RFC 7231, 6.6.5
	ErrHTTPVersionNotSupported       = NewError(StatusHTTPVersionNotSupported)       // RFC 7231, 6.6.6
	ErrVariantAlsoNegotiates         = NewError(StatusVariantAlsoNegotiates)         // RFC 2295, 8.1
	ErrInsufficientStorage           = NewError(StatusInsufficientStorage)           // RFC 4918, 11.5
	ErrLoopDetected                  = NewError(StatusLoopDetected)                  // RFC 5842, 7.2
	ErrNotExtended                   = NewError(StatusNotExtended)                   // RFC 2774, 7
	ErrNetworkAuthenticationRequired = NewError(StatusNetworkAuthenticationRequired) // RFC 6585, 6
)
```

HTTP Headers were copied from net/http.

```go
const (
	HeaderAuthorization                   = "Authorization"
	HeaderProxyAuthenticate               = "Proxy-Authenticate"
	HeaderProxyAuthorization              = "Proxy-Authorization"
	HeaderWWWAuthenticate                 = "WWW-Authenticate"
	HeaderAge                             = "Age"
	HeaderCacheControl                    = "Cache-Control"
	HeaderClearSiteData                   = "Clear-Site-Data"
	HeaderExpires                         = "Expires"
	HeaderPragma                          = "Pragma"
	HeaderWarning                         = "Warning"
	HeaderAcceptCH                        = "Accept-CH"
	HeaderAcceptCHLifetime                = "Accept-CH-Lifetime"
	HeaderContentDPR                      = "Content-DPR"
	HeaderDPR                             = "DPR"
	HeaderEarlyData                       = "Early-Data"
	HeaderSaveData                        = "Save-Data"
	HeaderViewportWidth                   = "Viewport-Width"
	HeaderWidth                           = "Width"
	HeaderETag                            = "ETag"
	HeaderIfMatch                         = "If-Match"
	HeaderIfModifiedSince                 = "If-Modified-Since"
	HeaderIfNoneMatch                     = "If-None-Match"
	HeaderIfUnmodifiedSince               = "If-Unmodified-Since"
	HeaderLastModified                    = "Last-Modified"
	HeaderVary                            = "Vary"
	HeaderConnection                      = "Connection"
	HeaderKeepAlive                       = "Keep-Alive"
	HeaderAccept                          = "Accept"
	HeaderAcceptCharset                   = "Accept-Charset"
	HeaderAcceptEncoding                  = "Accept-Encoding"
	HeaderAcceptLanguage                  = "Accept-Language"
	HeaderCookie                          = "Cookie"
	HeaderExpect                          = "Expect"
	HeaderMaxForwards                     = "Max-Forwards"
	HeaderSetCookie                       = "Set-Cookie"
	HeaderAccessControlAllowCredentials   = "Access-Control-Allow-Credentials"
	HeaderAccessControlAllowHeaders       = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowMethods       = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowOrigin        = "Access-Control-Allow-Origin"
	HeaderAccessControlExposeHeaders      = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge             = "Access-Control-Max-Age"
	HeaderAccessControlRequestHeaders     = "Access-Control-Request-Headers"
	HeaderAccessControlRequestMethod      = "Access-Control-Request-Method"
	HeaderOrigin                          = "Origin"
	HeaderTimingAllowOrigin               = "Timing-Allow-Origin"
	HeaderXPermittedCrossDomainPolicies   = "X-Permitted-Cross-Domain-Policies"
	HeaderDNT                             = "DNT"
	HeaderTk                              = "Tk"
	HeaderContentDisposition              = "Content-Disposition"
	HeaderContentEncoding                 = "Content-Encoding"
	HeaderContentLanguage                 = "Content-Language"
	HeaderContentLength                   = "Content-Length"
	HeaderContentLocation                 = "Content-Location"
	HeaderContentType                     = "Content-Type"
	HeaderForwarded                       = "Forwarded"
	HeaderVia                             = "Via"
	HeaderXForwardedFor                   = "X-Forwarded-For"
	HeaderXForwardedHost                  = "X-Forwarded-Host"
	HeaderXForwardedProto                 = "X-Forwarded-Proto"
	HeaderXForwardedProtocol              = "X-Forwarded-Protocol"
	HeaderXForwardedSsl                   = "X-Forwarded-Ssl"
	HeaderXUrlScheme                      = "X-Url-Scheme"
	HeaderLocation                        = "Location"
	HeaderFrom                            = "From"
	HeaderHost                            = "Host"
	HeaderReferer                         = "Referer"
	HeaderReferrerPolicy                  = "Referrer-Policy"
	HeaderUserAgent                       = "User-Agent"
	HeaderAllow                           = "Allow"
	HeaderServer                          = "Server"
	HeaderAcceptRanges                    = "Accept-Ranges"
	HeaderContentRange                    = "Content-Range"
	HeaderIfRange                         = "If-Range"
	HeaderRange                           = "Range"
	HeaderContentSecurityPolicy           = "Content-Security-Policy"
	HeaderContentSecurityPolicyReportOnly = "Content-Security-Policy-Report-Only"
	HeaderCrossOriginResourcePolicy       = "Cross-Origin-Resource-Policy"
	HeaderExpectCT                        = "Expect-CT"
	HeaderFeaturePolicy                   = "Feature-Policy"
	HeaderPublicKeyPins                   = "Public-Key-Pins"
	HeaderPublicKeyPinsReportOnly         = "Public-Key-Pins-Report-Only"
	HeaderStrictTransportSecurity         = "Strict-Transport-Security"
	HeaderUpgradeInsecureRequests         = "Upgrade-Insecure-Requests"
	HeaderXContentTypeOptions             = "X-Content-Type-Options"
	HeaderXDownloadOptions                = "X-Download-Options"
	HeaderXFrameOptions                   = "X-Frame-Options"
	HeaderXPoweredBy                      = "X-Powered-By"
	HeaderXXSSProtection                  = "X-XSS-Protection"
	HeaderLastEventID                     = "Last-Event-ID"
	HeaderNEL                             = "NEL"
	HeaderPingFrom                        = "Ping-From"
	HeaderPingTo                          = "Ping-To"
	HeaderReportTo                        = "Report-To"
	HeaderTE                              = "TE"
	HeaderTrailer                         = "Trailer"
	HeaderTransferEncoding                = "Transfer-Encoding"
	HeaderSecWebSocketAccept              = "Sec-WebSocket-Accept"
	HeaderSecWebSocketExtensions          = "Sec-WebSocket-Extensions"
	HeaderSecWebSocketKey                 = "Sec-WebSocket-Key"
	HeaderSecWebSocketProtocol            = "Sec-WebSocket-Protocol"
	HeaderSecWebSocketVersion             = "Sec-WebSocket-Version"
	HeaderAcceptPatch                     = "Accept-Patch"
	HeaderAcceptPushPolicy                = "Accept-Push-Policy"
	HeaderAcceptSignature                 = "Accept-Signature"
	HeaderAltSvc                          = "Alt-Svc"
	HeaderDate                            = "Date"
	HeaderIndex                           = "Index"
	HeaderLargeAllocation                 = "Large-Allocation"
	HeaderLink                            = "Link"
	HeaderPushPolicy                      = "Push-Policy"
	HeaderRetryAfter                      = "Retry-After"
	HeaderServerTiming                    = "Server-Timing"
	HeaderSignature                       = "Signature"
	HeaderSignedHeaders                   = "Signed-Headers"
	HeaderSourceMap                       = "SourceMap"
	HeaderUpgrade                         = "Upgrade"
	HeaderXDNSPrefetchControl             = "X-DNS-Prefetch-Control"
	HeaderXPingback                       = "X-Pingback"
	HeaderXRequestID                      = "X-Request-ID"
	HeaderXRequestedWith                  = "X-Requested-With"
	HeaderXRobotsTag                      = "X-Robots-Tag"
	HeaderXUACompatible                   = "X-UA-Compatible"
)
```