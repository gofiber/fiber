// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

// Document elke line gelijk even
func setETag(ctx *Ctx, weak bool) {
	body := ctx.Fasthttp.Response.Body()
	// Skips ETag if no response body is present
	if len(body) <= 0 {
		return
	}
	// Get ETag header from request
	clientEtag := ctx.Get("If-None-Match")

	// Generate ETag for response
	crc32q := crc32.MakeTable(0xD5828281)
	etag := fmt.Sprintf("\"%d-%v\"", len(body), crc32.Checksum(body, crc32q))

	// Enable weak tag
	if weak {
		etag = "W/" + "\"" + etag + "\""
	}

	// Check if client's ETag is weak
	if strings.HasPrefix(clientEtag, "W/") {
		// Check if server's ETag is weak
		if clientEtag[2:] == etag || clientEtag[2:] == etag[2:] {
			// W/1 == 1 || W/1 == W/1
			ctx.SendStatus(304)
			ctx.Fasthttp.ResetBody()
			return
		}
		// W/1 != W/2 || W/1 != 2
		ctx.Set("ETag", etag)
		return
	}
	if strings.Contains(clientEtag, etag) {
		// 1 == 1
		ctx.SendStatus(304)
		ctx.Fasthttp.ResetBody()
		return
	}
	// 1 != 2
	ctx.Set("ETag", etag)
}

func groupPaths(prefix, path string) string {
	if path == "/" {
		path = ""
	}
	path = prefix + path
	path = strings.Replace(path, "//", "/", -1)
	return path
}

func getFiles(root string) (files []string, dir bool, err error) {
	root = filepath.Clean(root)
	if _, err := os.Lstat(root); err != nil {
		return files, dir, fmt.Errorf("%s", err)
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		} else {
			dir = true
		}
		return err
	})
	return
}

func getMIME(extension string) (mime string) {
	if extension == "" {
		return mime
	}
	mime = extensionMIME[extension]
	if mime == "" {
		return MIMEOctetStream
	}
	return mime
}

// #nosec G103
// getString converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
var getString = func(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
var getStringImmutable = func(b []byte) string {
	return string(b)
}

// #nosec G103
// getBytes converts string to a byte slice without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
var getBytes = func(s string) (b []byte) {
	return *(*[]byte)(unsafe.Pointer(&s))
}
var getBytesImmutable = func(s string) (b []byte) {
	return []byte(s)
}

// Check if -prefork is in arguments
func isPrefork() bool {
	for i := range os.Args[1:] {
		if os.Args[1:][i] == "-prefork" {
			return true
		}
	}
	return false
}

// Check if -child is in arguments
func isChild() bool {
	for i := range os.Args[1:] {
		if os.Args[1:][i] == "-child" {
			return true
		}
	}
	return false
}

// https://golang.org/src/net/net.go#L113
// Helper methods for application#test
type testConn struct {
	net.Conn
	r bytes.Buffer
	w bytes.Buffer
}

func (c *testConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP: net.IPv4(0, 0, 0, 0),
	}
}
func (c *testConn) LocalAddr() net.Addr                { return c.RemoteAddr() }
func (c *testConn) Read(b []byte) (int, error)         { return c.r.Read(b) }
func (c *testConn) Write(b []byte) (int, error)        { return c.w.Write(b) }
func (c *testConn) Close() error                       { return nil }
func (c *testConn) SetDeadline(t time.Time) error      { return nil }
func (c *testConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *testConn) SetWriteDeadline(t time.Time) error { return nil }

// Adapted from:
// https://github.com/jshttp/fresh/blob/10e0471669dbbfbfd8de65bc6efac2ddd0bfa057/index.js#L110
func parseTokenList(noneMatchBytes []byte) []string {
	var (
		start int
		end   int
		list  []string
	)
	for i := range noneMatchBytes {
		switch noneMatchBytes[i] {
		case 0x20:
			if start == end {
				start = i + 1
				end = i + 1
			}
		case 0x2c:
			list = append(list, getString(noneMatchBytes[start:end]))
			start = i + 1
			end = i + 1
		default:
			end = i + 1
		}
	}

	list = append(list, getString(noneMatchBytes[start:end]))
	return list
}

// HTTP status codes were copied from net/http.
var statusMessages = map[int]string{
	100: "Continue",
	101: "Switching Protocols",
	102: "Processing",
	103: "Early Hints",
	200: "OK",
	201: "Created",
	202: "Accepted",
	203: "Non-Authoritative Information",
	204: "No Content",
	205: "Reset Content",
	206: "Partial Content",
	207: "Multi-Status",
	208: "Already Reported",
	226: "IM Used",
	300: "Multiple Choices",
	301: "Moved Permanently",
	302: "Found",
	303: "See Other",
	304: "Not Modified",
	305: "Use Proxy",
	306: "Switch Proxy",
	307: "Temporary Redirect",
	308: "Permanent Redirect",
	400: "Bad Request",
	401: "Unauthorized",
	402: "Payment Required",
	403: "Forbidden",
	404: "Not Found",
	405: "Method Not Allowed",
	406: "Not Acceptable",
	407: "Proxy Authentication Required",
	408: "Request Timeout",
	409: "Conflict",
	410: "Gone",
	411: "Length Required",
	412: "Precondition Failed",
	413: "Request Entity Too Large",
	414: "Request URI Too Long",
	415: "Unsupported Media Type",
	416: "Requested Range Not Satisfiable",
	417: "Expectation Failed",
	418: "I'm a teapot",
	421: "Misdirected Request",
	422: "Unprocessable Entity",
	423: "Locked",
	424: "Failed Dependency",
	426: "Upgrade Required",
	428: "Precondition Required",
	429: "Too Many Requests",
	431: "Request Header Fields Too Large",
	451: "Unavailable For Legal Reasons",
	500: "Internal Server Error",
	501: "Not Implemented",
	502: "Bad Gateway",
	503: "Service Unavailable",
	504: "Gateway Timeout",
	505: "HTTP Version Not Supported",
	506: "Variant Also Negotiates",
	507: "Insufficient Storage",
	508: "Loop Detected",
	510: "Not Extended",
	511: "Network Authentication Required",
}

// MIME types were copied from labstack/echo
const (
	MIMETextXML   = "text/xml"
	MIMETextHTML  = "text/html"
	MIMETextPlain = "text/plain"

	MIMEApplicationJSON       = "application/json"
	MIMEApplicationJavaScript = "application/javascript"
	MIMEApplicationXML        = "application/xml"
	MIMEApplicationForm       = "application/x-www-form-urlencoded"

	MIMEMultipartForm = "multipart/form-data"

	MIMEOctetStream = "application/octet-stream"
)

// MIME types were copied from nginx/mime.types.
var extensionMIME = map[string]string{
	// without dot
	"html":    "text/html",
	"htm":     "text/html",
	"shtml":   "text/html",
	"css":     "text/css",
	"gif":     "image/gif",
	"jpeg":    "image/jpeg",
	"jpg":     "image/jpeg",
	"xml":     "application/xml",
	"js":      "application/javascript",
	"atom":    "application/atom+xml",
	"rss":     "application/rss+xml",
	"mml":     "text/mathml",
	"txt":     "text/plain",
	"jad":     "text/vnd.sun.j2me.app-descriptor",
	"wml":     "text/vnd.wap.wml",
	"htc":     "text/x-component",
	"png":     "image/png",
	"svg":     "image/svg+xml",
	"svgz":    "image/svg+xml",
	"tif":     "image/tiff",
	"tiff":    "image/tiff",
	"wbmp":    "image/vnd.wap.wbmp",
	"webp":    "image/webp",
	"ico":     "image/x-icon",
	"jng":     "image/x-jng",
	"bmp":     "image/x-ms-bmp",
	"woff":    "font/woff",
	"woff2":   "font/woff2",
	"jar":     "application/java-archive",
	"war":     "application/java-archive",
	"ear":     "application/java-archive",
	"json":    "application/json",
	"hqx":     "application/mac-binhex40",
	"doc":     "application/msword",
	"pdf":     "application/pdf",
	"ps":      "application/postscript",
	"eps":     "application/postscript",
	"ai":      "application/postscript",
	"rtf":     "application/rtf",
	"m3u8":    "application/vnd.apple.mpegurl",
	"kml":     "application/vnd.google-earth.kml+xml",
	"kmz":     "application/vnd.google-earth.kmz",
	"xls":     "application/vnd.ms-excel",
	"eot":     "application/vnd.ms-fontobject",
	"ppt":     "application/vnd.ms-powerpoint",
	"odg":     "application/vnd.oasis.opendocument.graphics",
	"odp":     "application/vnd.oasis.opendocument.presentation",
	"ods":     "application/vnd.oasis.opendocument.spreadsheet",
	"odt":     "application/vnd.oasis.opendocument.text",
	"wmlc":    "application/vnd.wap.wmlc",
	"7z":      "application/x-7z-compressed",
	"cco":     "application/x-cocoa",
	"jardiff": "application/x-java-archive-diff",
	"jnlp":    "application/x-java-jnlp-file",
	"run":     "application/x-makeself",
	"pl":      "application/x-perl",
	"pm":      "application/x-perl",
	"prc":     "application/x-pilot",
	"pdb":     "application/x-pilot",
	"rar":     "application/x-rar-compressed",
	"rpm":     "application/x-redhat-package-manager",
	"sea":     "application/x-sea",
	"swf":     "application/x-shockwave-flash",
	"sit":     "application/x-stuffit",
	"tcl":     "application/x-tcl",
	"tk":      "application/x-tcl",
	"der":     "application/x-x509-ca-cert",
	"pem":     "application/x-x509-ca-cert",
	"crt":     "application/x-x509-ca-cert",
	"xpi":     "application/x-xpinstall",
	"xhtml":   "application/xhtml+xml",
	"xspf":    "application/xspf+xml",
	"zip":     "application/zip",
	"bin":     "application/octet-stream",
	"exe":     "application/octet-stream",
	"dll":     "application/octet-stream",
	"deb":     "application/octet-stream",
	"dmg":     "application/octet-stream",
	"iso":     "application/octet-stream",
	"img":     "application/octet-stream",
	"msi":     "application/octet-stream",
	"msp":     "application/octet-stream",
	"msm":     "application/octet-stream",
	"mid":     "audio/midi",
	"midi":    "audio/midi",
	"kar":     "audio/midi",
	"mp3":     "audio/mpeg",
	"ogg":     "audio/ogg",
	"m4a":     "audio/x-m4a",
	"ra":      "audio/x-realaudio",
	"3gpp":    "video/3gpp",
	"3gp":     "video/3gpp",
	"ts":      "video/mp2t",
	"mp4":     "video/mp4",
	"mpeg":    "video/mpeg",
	"mpg":     "video/mpeg",
	"mov":     "video/quicktime",
	"webm":    "video/webm",
	"flv":     "video/x-flv",
	"m4v":     "video/x-m4v",
	"mng":     "video/x-mng",
	"asx":     "video/x-ms-asf",
	"asf":     "video/x-ms-asf",
	"wmv":     "video/x-ms-wmv",
	"avi":     "video/x-msvideo",

	// with dot
	".html":    "text/html",
	".htm":     "text/html",
	".shtml":   "text/html",
	".css":     "text/css",
	".gif":     "image/gif",
	".jpeg":    "image/jpeg",
	".jpg":     "image/jpeg",
	".xml":     "application/xml",
	".js":      "application/javascript",
	".atom":    "application/atom+xml",
	".rss":     "application/rss+xml",
	".mml":     "text/mathml",
	".txt":     "text/plain",
	".jad":     "text/vnd.sun.j2me.app-descriptor",
	".wml":     "text/vnd.wap.wml",
	".htc":     "text/x-component",
	".png":     "image/png",
	".svg":     "image/svg+xml",
	".svgz":    "image/svg+xml",
	".tif":     "image/tiff",
	".tiff":    "image/tiff",
	".wbmp":    "image/vnd.wap.wbmp",
	".webp":    "image/webp",
	".ico":     "image/x-icon",
	".jng":     "image/x-jng",
	".bmp":     "image/x-ms-bmp",
	".woff":    "font/woff",
	".woff2":   "font/woff2",
	".jar":     "application/java-archive",
	".war":     "application/java-archive",
	".ear":     "application/java-archive",
	".json":    "application/json",
	".hqx":     "application/mac-binhex40",
	".doc":     "application/msword",
	".pdf":     "application/pdf",
	".ps":      "application/postscript",
	".eps":     "application/postscript",
	".ai":      "application/postscript",
	".rtf":     "application/rtf",
	".m3u8":    "application/vnd.apple.mpegurl",
	".kml":     "application/vnd.google-earth.kml+xml",
	".kmz":     "application/vnd.google-earth.kmz",
	".xls":     "application/vnd.ms-excel",
	".eot":     "application/vnd.ms-fontobject",
	".ppt":     "application/vnd.ms-powerpoint",
	".odg":     "application/vnd.oasis.opendocument.graphics",
	".odp":     "application/vnd.oasis.opendocument.presentation",
	".ods":     "application/vnd.oasis.opendocument.spreadsheet",
	".odt":     "application/vnd.oasis.opendocument.text",
	".wmlc":    "application/vnd.wap.wmlc",
	".7z":      "application/x-7z-compressed",
	".cco":     "application/x-cocoa",
	".jardiff": "application/x-java-archive-diff",
	".jnlp":    "application/x-java-jnlp-file",
	".run":     "application/x-makeself",
	".pl":      "application/x-perl",
	".pm":      "application/x-perl",
	".prc":     "application/x-pilot",
	".pdb":     "application/x-pilot",
	".rar":     "application/x-rar-compressed",
	".rpm":     "application/x-redhat-package-manager",
	".sea":     "application/x-sea",
	".swf":     "application/x-shockwave-flash",
	".sit":     "application/x-stuffit",
	".tcl":     "application/x-tcl",
	".tk":      "application/x-tcl",
	".der":     "application/x-x509-ca-cert",
	".pem":     "application/x-x509-ca-cert",
	".crt":     "application/x-x509-ca-cert",
	".xpi":     "application/x-xpinstall",
	".xhtml":   "application/xhtml+xml",
	".xspf":    "application/xspf+xml",
	".zip":     "application/zip",
	".bin":     "application/octet-stream",
	".exe":     "application/octet-stream",
	".dll":     "application/octet-stream",
	".deb":     "application/octet-stream",
	".dmg":     "application/octet-stream",
	".iso":     "application/octet-stream",
	".img":     "application/octet-stream",
	".msi":     "application/octet-stream",
	".msp":     "application/octet-stream",
	".msm":     "application/octet-stream",
	".mid":     "audio/midi",
	".midi":    "audio/midi",
	".kar":     "audio/midi",
	".mp3":     "audio/mpeg",
	".ogg":     "audio/ogg",
	".m4a":     "audio/x-m4a",
	".ra":      "audio/x-realaudio",
	".3gpp":    "video/3gpp",
	".3gp":     "video/3gpp",
	".ts":      "video/mp2t",
	".mp4":     "video/mp4",
	".mpeg":    "video/mpeg",
	".mpg":     "video/mpeg",
	".mov":     "video/quicktime",
	".webm":    "video/webm",
	".flv":     "video/x-flv",
	".m4v":     "video/x-m4v",
	".mng":     "video/x-mng",
	".asx":     "video/x-ms-asf",
	".asf":     "video/x-ms-asf",
	".wmv":     "video/x-ms-wmv",
	".avi":     "video/x-msvideo",
}

// HTTP methods were copied from net/http.
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
)

// HTTP Headers were copied from net/http.
const (
	// Authentication
	HeaderAuthorization      = "Authorization"
	HeaderProxyAuthenticate  = "Proxy-Authenticate"
	HeaderProxyAuthorization = "Proxy-Authorization"
	HeaderWWWAuthenticate    = "WWW-Authenticate"

	// Caching
	HeaderAge           = "Age"
	HeaderCacheControl  = "Cache-Control"
	HeaderClearSiteData = "Clear-Site-Data"
	HeaderExpires       = "Expires"
	HeaderPragma        = "Pragma"
	HeaderWarning       = "Warning"

	// Client hints
	HeaderAcceptCH         = "Accept-CH"
	HeaderAcceptCHLifetime = "Accept-CH-Lifetime"
	HeaderContentDPR       = "Content-DPR"
	HeaderDPR              = "DPR"
	HeaderEarlyData        = "Early-Data"
	HeaderSaveData         = "Save-Data"
	HeaderViewportWidth    = "Viewport-Width"
	HeaderWidth            = "Width"

	// Conditionals
	HeaderETag              = "ETag"
	HeaderIfMatch           = "If-Match"
	HeaderIfModifiedSince   = "If-Modified-Since"
	HeaderIfNoneMatch       = "If-None-Match"
	HeaderIfUnmodifiedSince = "If-Unmodified-Since"
	HeaderLastModified      = "Last-Modified"
	HeaderVary              = "Vary"

	// Connection management
	HeaderConnection = "Connection"
	HeaderKeepAlive  = "Keep-Alive"

	// Content negotiation
	HeaderAccept         = "Accept"
	HeaderAcceptCharset  = "Accept-Charset"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAcceptLanguage = "Accept-Language"

	// Controls
	HeaderCookie      = "Cookie"
	HeaderExpect      = "Expect"
	HeaderMaxForwards = "Max-Forwards"
	HeaderSetCookie   = "Set-Cookie"

	// CORS
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderAccessControlRequestHeaders   = "Access-Control-Request-Headers"
	HeaderAccessControlRequestMethod    = "Access-Control-Request-Method"
	HeaderOrigin                        = "Origin"
	HeaderTimingAllowOrigin             = "Timing-Allow-Origin"
	HeaderXPermittedCrossDomainPolicies = "X-Permitted-Cross-Domain-Policies"

	// Do Not Track
	HeaderDNT = "DNT"
	HeaderTk  = "Tk"

	// Downloads
	HeaderContentDisposition = "Content-Disposition"

	// Message body information
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentLanguage = "Content-Language"
	HeaderContentLength   = "Content-Length"
	HeaderContentLocation = "Content-Location"
	HeaderContentType     = "Content-Type"

	// Proxies
	HeaderForwarded       = "Forwarded"
	HeaderVia             = "Via"
	HeaderXForwardedFor   = "X-Forwarded-For"
	HeaderXForwardedHost  = "X-Forwarded-Host"
	HeaderXForwardedProto = "X-Forwarded-Proto"

	// Redirects
	HeaderLocation = "Location"

	// Request context
	HeaderFrom           = "From"
	HeaderHost           = "Host"
	HeaderReferer        = "Referer"
	HeaderReferrerPolicy = "Referrer-Policy"
	HeaderUserAgent      = "User-Agent"

	// Response context
	HeaderAllow  = "Allow"
	HeaderServer = "Server"

	// Range requests
	HeaderAcceptRanges = "Accept-Ranges"
	HeaderContentRange = "Content-Range"
	HeaderIfRange      = "If-Range"
	HeaderRange        = "Range"

	// Security
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

	// Server-sent event
	HeaderLastEventID = "Last-Event-ID"
	HeaderNEL         = "NEL"
	HeaderPingFrom    = "Ping-From"
	HeaderPingTo      = "Ping-To"
	HeaderReportTo    = "Report-To"

	// Transfer coding
	HeaderTE               = "TE"
	HeaderTrailer          = "Trailer"
	HeaderTransferEncoding = "Transfer-Encoding"

	// WebSockets
	HeaderSecWebSocketAccept     = "Sec-WebSocket-Accept"
	HeaderSecWebSocketExtensions = "Sec-WebSocket-Extensions"
	HeaderSecWebSocketKey        = "Sec-WebSocket-Key"
	HeaderSecWebSocketProtocol   = "Sec-WebSocket-Protocol"
	HeaderSecWebSocketVersion    = "Sec-WebSocket-Version"

	// Other
	HeaderAcceptPatch         = "Accept-Patch"
	HeaderAcceptPushPolicy    = "Accept-Push-Policy"
	HeaderAcceptSignature     = "Accept-Signature"
	HeaderAltSvc              = "Alt-Svc"
	HeaderDate                = "Date"
	HeaderIndex               = "Index"
	HeaderLargeAllocation     = "Large-Allocation"
	HeaderLink                = "Link"
	HeaderPushPolicy          = "Push-Policy"
	HeaderRetryAfter          = "Retry-After"
	HeaderServerTiming        = "Server-Timing"
	HeaderSignature           = "Signature"
	HeaderSignedHeaders       = "Signed-Headers"
	HeaderSourceMap           = "SourceMap"
	HeaderUpgrade             = "Upgrade"
	HeaderXDNSPrefetchControl = "X-DNS-Prefetch-Control"
	HeaderXPingback           = "X-Pingback"
	HeaderXRequestID          = "X-Request-ID"
	HeaderXRequestedWith      = "X-Requested-With"
	HeaderXRobotsTag          = "X-Robots-Tag"
	HeaderXUACompatible       = "X-UA-Compatible"
)

// HTTP status codes were copied from net/http.
const (
	StatusContinue           = 100 // RFC 7231, 6.2.1
	StatusSwitchingProtocols = 101 // RFC 7231, 6.2.2
	StatusProcessing         = 102 // RFC 2518, 10.1
	StatusEarlyHints         = 103 // RFC 8297

	StatusOK                   = 200 // RFC 7231, 6.3.1
	StatusCreated              = 201 // RFC 7231, 6.3.2
	StatusAccepted             = 202 // RFC 7231, 6.3.3
	StatusNonAuthoritativeInfo = 203 // RFC 7231, 6.3.4
	StatusNoContent            = 204 // RFC 7231, 6.3.5
	StatusResetContent         = 205 // RFC 7231, 6.3.6
	StatusPartialContent       = 206 // RFC 7233, 4.1
	StatusMultiStatus          = 207 // RFC 4918, 11.1
	StatusAlreadyReported      = 208 // RFC 5842, 7.1
	StatusIMUsed               = 226 // RFC 3229, 10.4.1

	StatusMultipleChoices  = 300 // RFC 7231, 6.4.1
	StatusMovedPermanently = 301 // RFC 7231, 6.4.2
	StatusFound            = 302 // RFC 7231, 6.4.3
	StatusSeeOther         = 303 // RFC 7231, 6.4.4
	StatusNotModified      = 304 // RFC 7232, 4.1
	StatusUseProxy         = 305 // RFC 7231, 6.4.5

	StatusTemporaryRedirect = 307 // RFC 7231, 6.4.7
	StatusPermanentRedirect = 308 // RFC 7538, 3

	StatusBadRequest                   = 400 // RFC 7231, 6.5.1
	StatusUnauthorized                 = 401 // RFC 7235, 3.1
	StatusPaymentRequired              = 402 // RFC 7231, 6.5.2
	StatusForbidden                    = 403 // RFC 7231, 6.5.3
	StatusNotFound                     = 404 // RFC 7231, 6.5.4
	StatusMethodNotAllowed             = 405 // RFC 7231, 6.5.5
	StatusNotAcceptable                = 406 // RFC 7231, 6.5.6
	StatusProxyAuthRequired            = 407 // RFC 7235, 3.2
	StatusRequestTimeout               = 408 // RFC 7231, 6.5.7
	StatusConflict                     = 409 // RFC 7231, 6.5.8
	StatusGone                         = 410 // RFC 7231, 6.5.9
	StatusLengthRequired               = 411 // RFC 7231, 6.5.10
	StatusPreconditionFailed           = 412 // RFC 7232, 4.2
	StatusRequestEntityTooLarge        = 413 // RFC 7231, 6.5.11
	StatusRequestURITooLong            = 414 // RFC 7231, 6.5.12
	StatusUnsupportedMediaType         = 415 // RFC 7231, 6.5.13
	StatusRequestedRangeNotSatisfiable = 416 // RFC 7233, 4.4
	StatusExpectationFailed            = 417 // RFC 7231, 6.5.14
	StatusTeapot                       = 418 // RFC 7168, 2.3.3
	StatusMisdirectedRequest           = 421 // RFC 7540, 9.1.2
	StatusUnprocessableEntity          = 422 // RFC 4918, 11.2
	StatusLocked                       = 423 // RFC 4918, 11.3
	StatusFailedDependency             = 424 // RFC 4918, 11.4
	StatusUpgradeRequired              = 426 // RFC 7231, 6.5.15
	StatusPreconditionRequired         = 428 // RFC 6585, 3
	StatusTooManyRequests              = 429 // RFC 6585, 4
	StatusRequestHeaderFieldsTooLarge  = 431 // RFC 6585, 5
	StatusUnavailableForLegalReasons   = 451 // RFC 7725, 3

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
