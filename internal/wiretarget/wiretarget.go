// Package wiretarget spells the path the router matched the way a request
// line carries it, for middleware that hands the request on to something that
// reads request lines: a proxy upstream, a net/http or fasthttp handler.
package wiretarget

import (
	"bytes"
	"strings"

	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Ctx is what this package reads of a fiber.Ctx, App being *fiber.App. Naming
// either would be an import cycle: package fiber serves net/http handlers here.
type Ctx[App any] interface {
	App() App
	OriginalURL() string
	Path(override ...string) string
	Request() *fasthttp.Request
	Response() *fasthttp.Response
	RequestCtx() *fasthttp.RequestCtx
	IsAbandoned() bool
}

// Allowance names what SegmentsAsRouted lets through that it refuses
// otherwise, for a caller that was told the reader keeps it as sent.
type Allowance uint8

const (
	// AllowSlashes lets an escaped slash and an empty segment through.
	AllowSlashes Allowance = 1 << iota
	// AllowParameters lets a path parameter through on a segment that has a
	// name other than "." and "..".
	AllowParameters
)

// Routed returns the request target to hand on for c: the path the router
// matched, spelled as a request line has to carry it, then the query.
//
// Under the default configuration c.Path() is nearly that spelling already:
// escapes of unreserved characters decoded, every other escape kept, dot
// segments resolved and empty segments kept. What it may still hold raw is
// written as an escape, the valid spelling of the same name: a stray "%" that
// begins no escape as "%25", and every byte RFC 3986 does not allow raw in a
// path, such as a space, "\", "|", "{", "}", '"', "^", "<", ">", a control
// byte or a byte of UTF-8, as its escape, as fasthttp's normalizing writer did
// before. Under UnescapePath c.Path() is fully decoded, so it is escaped again
// segment by segment, and a "/" that came from "%2F" stays the separator the
// router took it for.
//
// The raw request line is deliberately not the source. fasthttp's own
// normalization of it decodes "%2F" into a separator, merges "//" and then
// resolves "..", so "/public/..%2Fadmin" and "//admin" reached a proxy
// upstream or a net/http handler as "/admin" although no middleware mounted
// on "/admin" had run for them. The routed path keeps them the distinct names
// the router matched.
//
// The query is the one the client sent, even after a handler read it through
// c.Query(): fasthttp would serialize parsed arguments again, "%20" as "+" and
// ";" as "%3B", and a signed URL would no longer verify. Only when a handler
// changed the arguments through QueryArgs() are they forwarded as fasthttp
// serializes them, since that is the change it made.
//
// The target may be the request line itself, a view of it rather than a
// copy, when that already spells the routed path, so read it before the
// request line is set to something else.
//
// The target always begins with a slash. A path override that dropped it, a
// rewrite of "/go/*" to "$1" say, would otherwise hand on
// "http://u:pw@evil.example/x", which a request line reads as an absolute URL
// and so as a host and a credential; rooted, it is the path
// "/http://u:pw@evil.example/x".
func Routed[App any](c Ctx[App]) string {
	return routedFor(c, c.Path())
}

// routedFor is Routed for path, which is c.Path(), for a caller that has read it already.
func routedFor[App any](c Ctx[App], path string) string {
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	query := queryOf(c.Request())
	if appconfig.Of(c.App()).UnescapePath {
		target := make([]byte, 0, len(path)+len(query)+9)
		target = utils.AppendPathSegmentsEscape(target, path)
		return utils.UnsafeString(appendQuery(target, query))
	}
	if !needsEscape(path) {
		if len(query) == 0 {
			return path
		}
		// Most often the target is the request line as it arrived, which
		// is then handed back rather than built again.
		if line := c.OriginalURL(); len(line) == len(path)+1+len(query) && line[len(path)] == '?' &&
			line[:len(path)] == path && line[len(path)+1:] == string(query) {
			return line
		}
	}
	target := make([]byte, 0, len(path)+len(query)+9)
	target = appendPathEscaped(target, path)
	return utils.UnsafeString(appendQuery(target, query))
}

// Serve runs handler, a net/http handler adapted by fasthttpadaptor, on a
// request line that spells the path the router matched (Routed) rather than
// the one that arrived, and puts the original back afterwards. The two differ
// for an escaped or non-canonical request, and net/http reads the raw one its
// own way: "/public/..%2Fadmin" decodes into a URL.Path that http.FileServer
// cleans to "/admin", although no middleware mounted on "/admin" has run. A
// routed path whose segments would not survive that reading, one with an
// empty segment or an escaped slash, is not handed on, nor is one that a
// handler passing the request on, such as httputil.ReverseProxy, would hand an
// upstream that reads it as another path: a backslash, or a segment a servlet
// container strips to a dot or an empty segment ("..;", "/;x/admin"). A path
// parameter on a named segment is handed on, since net/http reads it as part
// of the name. Unless UnescapePath makes the router decode every escape itself,
// a routed path that is not in the one spelling SingleSpelling accepts is not
// handed on either: the router keeps "/R&D" and "/R%26D", "/user@corp" and
// "/user%40corp", "/caf%C3%A9" and a raw "/café" apart, but the handler decodes
// each pair into one name. Serve reports false, without running handler, for a
// routed path it does not hand on.
func Serve[App any](c Ctx[App], handler fasthttp.RequestHandler) bool {
	return serveWith(c, handler, func() bool { return c.Response().IsBodyStream() || c.RequestCtx().Hijacked() })
}

// ServeNative is Serve for a fasthttp handler registered as a route handler. Nothing of it keeps reading
// the request line once it returned: fasthttp forbids a stream writer and a hijack handler to access the
// RequestCtx. The original line is therefore put back whatever the response is, unless the handler timed
// out (fasthttp.TimeoutHandler, the timeout middleware), which leaves it running on the request.
func ServeNative[App any](c Ctx[App], handler fasthttp.RequestHandler) bool {
	// IsAbandoned goes first, as in the limiter: its atomic load orders the read of the timeout response
	// after the write of a timeout.New that gave up on the request on another goroutine.
	return serveWith(c, handler, func() bool { return c.IsAbandoned() || c.RequestCtx().LastTimeoutErrorResponse() != nil })
}

// serveWith runs handler on the routed request line and puts the original back afterwards, unless stillReading
// reports that the handler goes on reading the request.
func serveWith[App any](c Ctx[App], handler fasthttp.RequestHandler, stillReading func() bool) bool {
	// Read once: with Immutable every read of either copies the string.
	path, line := c.Path(), c.OriginalURL()
	target := routedFor(c, path)
	if !SegmentsAsRouted(target, AllowParameters) || (!appconfig.Of(c.App()).UnescapePath && !SingleSpelling(path)) {
		return false
	}
	if target == line {
		// the request line already spells the routed path
		handler(c.RequestCtx())
		return true
	}
	original := utils.CopyString(line)
	SetTarget(c, target)
	// Buffered, the handler has returned, or panicked, in which case
	// fasthttpadaptor re-panics here and the recover middleware or error
	// handler that catches it still has to see the original line. Streaming
	// or hijacked, the handler is still running with an http.Request whose
	// URL aliases the request line's storage, so the line is left as the
	// handler read it.
	defer func() {
		if !stillReading() {
			RestoreTarget(c.RequestCtx(), original)
		}
	}()
	handler(c.RequestCtx())
	return true
}

// SetTarget sets target, the routed request target, as c's request line.
// A target that is the line as it arrived is left alone, which spares a second
// parse of it. Otherwise the host fasthttp parsed is kept: a target that is a
// path makes fasthttp take the host from the Host header again, while an
// absolute-form request line names its own, which RFC 9112 Section 3.2.2 says
// the server uses and which c.Hostname() reported. Kept, the adapted handler
// sees in r.Host the host Fiber saw.
func SetTarget[App any](c Ctx[App], target string) {
	if target == c.OriginalURL() {
		return
	}
	req := c.Request()
	var buf [64]byte
	host := append(buf[:0], req.URI().Host()...)
	req.SetRequestURI(target)
	req.URI().SetHostBytes(host)
}

// ReadAsOriginForm reads target, the request target of fctx's request when it begins with "//", as the path it is:
// RFC 9112 Section 3.2.1 makes a target that begins with "/" origin-form, whose host is the Host header, and
// only an absolute-form target (it begins with a scheme) carries its own. fasthttp's parser takes
// the authority out of such a target when it holds "://" anywhere, query and fragment included, or
// when the request has no Host header, and then c.Host() is the first path segment and c.Path()
// lacks it. The cause is in that parser, which this module cannot change. DefaultCtx.Reset calls this for
// the request line as it arrived, RestoreTarget for the one a handler put back.
func ReadAsOriginForm(fctx *fasthttp.RequestCtx, target []byte) {
	req := &fctx.Request
	host := req.Header.Host()
	if ParsesAsPath(utils.UnsafeString(target), host) {
		return
	}
	uri := req.URI()
	var buf [64]byte
	host = append(buf[:0], host...)
	// Origin-form with the Host header, as fasthttp reads any other path; the target follows. Parsed apart:
	// URI.Parse resets its receiver before it validates the host, so a Host header it rejects would leave uri
	// empty and the request would be routed as "/". Such a header names no host: the request has none, as
	// without the header, and the first segment of its path is not taken for one (fasthttp answers 400 to a
	// plain target with that header, and cannot reject the request here, where it took the host from the path).
	scratch := fasthttp.AcquireURI()
	defer fasthttp.ReleaseURI(scratch)
	if err := scratch.Parse(host, []byte{'/'}); err != nil {
		if err = scratch.Parse(nil, []byte{'/'}); err != nil {
			return
		}
	}
	disableNormalizing := uri.DisablePathNormalizing
	scratch.CopyTo(uri)
	uri.DisablePathNormalizing = disableNormalizing
	if fctx.IsTLS() {
		uri.SetScheme("https")
	}
	rest := target
	if i := bytes.IndexByte(rest, '#'); i >= 0 {
		uri.SetHashBytes(rest[i+1:])
		rest = rest[:i]
	}
	if i := bytes.IndexByte(rest, '?'); i >= 0 {
		uri.SetQueryStringBytes(rest[i+1:])
		rest = rest[:i]
	}
	uri.SetPathBytes(rest)
}

// RestoreTarget sets original, the request line as it arrived, back as fctx's request target. fasthttp parses it
// again, and takes the authority out of a "//" target again, so the origin-form reading Reset gave it is applied
// again. The Host header has to be back in place by then.
func RestoreTarget(fctx *fasthttp.RequestCtx, original string) {
	fctx.Request.SetRequestURI(original)
	if len(original) > 1 && original[0] == '/' && original[1] == '/' {
		ReadAsOriginForm(fctx, utils.UnsafeBytes(original))
	}
}

// appendQuery appends query to dst behind a "?", or nothing for an empty one.
func appendQuery(dst, query []byte) []byte {
	if len(query) == 0 {
		return dst
	}
	return append(append(dst, '?'), query...)
}

// SegmentsAsRouted reports whether a reader that decodes target's path and
// cleans it still sees the segments the router matched. It is false when the
// path holds an empty segment, an escaped slash or a forged escape (see
// HasForgedEscape): net/http's URL.Path reads "%2F" as a separator and a
// forged "%2e%2e" as "..", and its handlers clean "//" and ".." away, so
// "/public/..%2Fadmin", "//admin" and "/public/%2e%2e/admin" would be served
// as "/admin" by a handler no middleware mounted on "/admin" guarded. A
// proxied upstream that decodes "%2F" or merges "//" before it matches routes
// reads a target the same way, so the proxy handlers refuse one for which
// this is false, as the net/http handler adapters do.
//
// It is false as well for a backslash, raw or escaped as "%5C", which WHATWG
// URL parsers and IIS read as a separator, and for a path parameter, a ";" in
// any segment. Servlet containers such as Tomcat and Jetty strip the
// parameters from every segment before they match routes, so "/admin;x/secret"
// and "/admin;/secret" are "/admin/secret" there, "/;/admin/secret" is
// "//admin/secret" and so "/admin/secret" once the empty segment is merged,
// and "/public/..;/admin" is "/admin".
//
// allow lets some of this through. A "." or ".." segment that carries
// parameters, such as "..;x", is refused whatever it says, behind a raw slash
// or, with AllowSlashes, behind an escaped one: a reader that strips the
// parameters resolves it as a dot segment, and none has a use for it as a
// name. A segment made of parameters alone, such as ";x", is an empty segment
// once they are stripped, and is refused as one unless AllowSlashes is set
// too; as the last segment it spells the trailing slash and passes.
func SegmentsAsRouted(target string, allow Allowance) bool {
	path := pathOf(target)
	slashes := allow&AllowSlashes != 0
	segment := 0
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '/':
			// An empty segment follows, or ends here: one that began with ";"
			// is empty once its parameters are stripped.
			if !slashes && ((i+1 < len(path) && path[i+1] == '/') || (segment < i && path[segment] == ';')) {
				return false
			}
			segment = i + 1
		case '\\':
			return false
		case ';':
			if allow&AllowParameters == 0 {
				return false
			}
			if name := path[segment:i]; name == "." || name == ".." {
				return false
			}
		case '%':
			if i+2 < len(path) && isHexDigit(path[i+1]) && isHexDigit(path[i+2]) {
				v := unhex(path[i+1])<<4 | unhex(path[i+2])
				if (v == '/' && !slashes) || v == '\\' || isUnreserved(v) {
					return false
				}
				i += 2
				if v == '/' {
					// Let through for a reader that keeps it as sent, but a
					// dot segment behind it is judged as one behind a raw slash.
					segment = i + 1
				}
			}
		}
	}
	return true
}

// SingleSpelling reports whether path, the path the router matched, is the one spelling of the
// path net/http decodes it to that a middleware registration can name. The router keeps "/R&D"
// and "/R%26D", "/user@corp" and "/user%40corp", "/caf%C3%A9" and a raw "/café" apart, so a
// middleware mounted on one of them does not run for the others, but net/http decodes them all
// into one URL.Path and serves the same directory for each. The spelling is the one a path
// has on the wire when its sub-delimiters, ":" and "@" are raw, an unreserved character is
// decoded (the router does that) and every other byte, a space or a byte of UTF-8 say,
// is an escape with uppercase hex digits. A "%" that begins no escape is let through, as it
// always was; the client cannot have meant an escape by it. Under UnescapePath the
// router decodes every escape itself, so the middleware sees one path and the caller skips this.
func SingleSpelling(path string) bool {
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '%' {
			if i+2 >= len(path) || !isHexDigit(path[i+1]) || !isHexDigit(path[i+2]) {
				continue
			}
			v := unhex(path[i+1])<<4 | unhex(path[i+2])
			if isHexLower(path[i+1]) || isHexLower(path[i+2]) || (pathRaw[v] && v != '/') {
				return false
			}
			i += 2
			continue
		}
		if !pathRaw[c] {
			return false
		}
	}
	return true
}

func isHexLower(b byte) bool { return b >= 'a' && b <= 'f' }

// HasForgedEscape reports whether target's path holds an escape of an
// unreserved character (RFC 3986 Section 2.3: a letter, a digit, "-", ".",
// "_" or "~"). The router decodes every such escape when it normalizes a
// path, so one still in c.Path() was forged by a stray "%" lining up with the
// escape after it: "/%%32e" reaches the router as "/%2e". A reader that
// decodes the path once more then sees a character the router never matched,
// "%2e%2e" becoming a ".." segment or "%70rivate" becoming "private". Under
// UnescapePath the routed path is escaped again segment by segment, which
// never escapes an unreserved character, so nothing is reported there.
func HasForgedEscape(target string) bool {
	path := pathOf(target)
	for {
		i := strings.IndexByte(path, '%')
		if i < 0 || i+2 >= len(path) {
			return false
		}
		if isHexDigit(path[i+1]) && isHexDigit(path[i+2]) {
			if isUnreserved(unhex(path[i+1])<<4 | unhex(path[i+2])) {
				return true
			}
			path = path[i+3:]
			continue
		}
		path = path[i+1:]
	}
}

// SurvivesNormalization reports whether fasthttp's path normalization would
// leave target's path naming what it names. Normalization merges every empty
// segment, decodes every escape and writes the path again with only some bytes
// escaped, so "//" and an escape that does not come back (see escapeSurvives)
// leave a normalizing client as a path the router never matched:
// "/public/..%2Fadmin" as "/admin" and "/..%3B/" as "/..;/". Any other escape,
// such as the "%C3%A9" Routed writes for "é", comes out as it went in, up to
// the case of its hex digits. The client also escapes a raw "!", "'", "(", ")"
// or "*", which changes the spelling but cannot add a separator.
func SurvivesNormalization(target string) bool {
	path := pathOf(target)
	if strings.Contains(path, "//") {
		return false
	}
	for {
		i := strings.IndexByte(path, '%')
		if i < 0 {
			return true
		}
		if i+2 >= len(path) || !isHexDigit(path[i+1]) || !isHexDigit(path[i+2]) ||
			!escapeSurvives(unhex(path[i+1])<<4|unhex(path[i+2])) {
			return false
		}
		path = path[i+3:]
	}
}

// escapeSurvives reports whether an escape of b comes out of fasthttp's path
// normalization as it went in. Normalization decodes it and then escapes only
// the bytes net/url escapes in a path, so b must not be an unreserved
// character or one of "$&+,/:;=@", which are written raw. Nor may it be a
// backslash, which normalization on Windows reads as a separator when it
// resolves dot segments.
func escapeSurvives(b byte) bool {
	return !isUnreserved(b) && strings.IndexByte("$&+,/:;=@\\", b) < 0
}

// ParsesAsPath reports whether fasthttp reads target as a path when it is set
// as the request line of a request whose Host header is host. It is false for
// a target without a leading slash, which a request line reads as an absolute
// URL or an authority, for one that begins with "//" and holds "://", and for
// one that begins with "//" on a request without a Host header: fasthttp then
// reads the target as "[scheme:]//authority/path" and takes the request's
// host, and a Basic credential, from the authority, over the ones the
// application set. The router never sees such a target as an authority, and
// it need not have arrived as one: ".." resolution builds
// "//u:pw@evil.example/a://b" out of "//u:pw@evil.example/a:/x/..//b", and
// under UnescapePath a decoded "%3A%2F%2F" spells the "://".
func ParsesAsPath(target string, host []byte) bool {
	if target == "" || target[0] != '/' {
		return false
	}
	if !strings.HasPrefix(target, "//") {
		return true
	}
	return len(host) != 0 && !strings.Contains(target, "://")
}

// pathOf returns target without its query.
func pathOf(target string) string {
	path, _, _ := strings.Cut(target, "?")
	return path
}

// queryOf returns the query of req's request target without its "?": the one
// the client sent, unless a handler changed the arguments through QueryArgs(),
// in which case it is the changed arguments as fasthttp serializes them. A
// handler that only read them left them parsed, and their serialization then
// differs from what was sent in spelling alone, which parsing what was sent
// and serializing that tells apart. Unlike Request.RequestURI, URI.RequestURI
// leaves the request itself alone.
func queryOf(req *fasthttp.Request) []byte {
	uri := req.URI()
	sent := uri.QueryString()
	var current []byte
	requestURI := uri.RequestURI()
	if _, query, ok := bytes.Cut(requestURI, []byte{'?'}); ok {
		current = query
	}
	if bytes.Equal(current, sent) {
		return sent
	}
	args := fasthttp.AcquireArgs()
	defer fasthttp.ReleaseArgs(args)
	args.ParseBytes(sent)
	if bytes.Equal(args.QueryString(), current) {
		return sent
	}
	return current
}

// pathRaw marks the bytes a path may carry unescaped (RFC 3986 Section 3.3):
// the unreserved characters, the sub-delims, ":", "@" and the "/" between
// segments. A "%" is handled apart, since it is raw only where it begins an
// escape.
var pathRaw = func() [256]bool {
	var t [256]bool
	for c := range 256 {
		t[c] = isUnreserved(byte(c)) || strings.IndexByte("!$&'()*+,;=:@/", byte(c)) >= 0
	}
	return t
}()

// needsEscape reports whether appendPathEscaped would change path.
func needsEscape(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '%' && i+2 < len(path) && isHexDigit(path[i+1]) && isHexDigit(path[i+2]) {
			i += 2 // kept as it is
			continue
		}
		if !pathRaw[path[i]] {
			return true
		}
	}
	return false
}

// appendPathEscaped appends path to dst with every escape kept as it is, every
// "%" that begins no escape written as "%25" and every other byte a path may
// not carry raw written as its escape, with uppercase hex digits.
func appendPathEscaped(dst []byte, path string) []byte {
	const upperhex = "0123456789ABCDEF"
	for i := 0; i < len(path); {
		j := i
		for j < len(path) && pathRaw[path[j]] {
			j++
		}
		dst = append(dst, path[i:j]...)
		if j == len(path) {
			break
		}
		if c := path[j]; c == '%' && j+2 < len(path) && isHexDigit(path[j+1]) && isHexDigit(path[j+2]) {
			dst = append(dst, path[j:j+3]...)
			i = j + 3
			continue
		}
		dst = append(dst, '%', upperhex[path[j]>>4], upperhex[path[j]&0x0F])
		i = j + 1
	}
	return dst
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// unhex returns the value of the hex digit b.
func unhex(b byte) byte {
	switch {
	case b >= 'a':
		return b - 'a' + 10
	case b >= 'A':
		return b - 'A' + 10
	default:
		return b - '0'
	}
}

// isUnreserved reports whether b is an unreserved character (RFC 3986
// Section 2.3), one an escape never needs to spell.
func isUnreserved(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '.' || b == '_' || b == '~'
}
