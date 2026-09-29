// Package wiretarget spells the path the router matched the way a request
// line carries it, for middleware that hands the request on to something that
// reads request lines: a proxy upstream, a net/http handler.
package wiretarget

import (
	"bytes"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/gofiber/utils/v2"
)

// Routed returns the request target to hand on for c: the path the router
// matched, spelled as a request line has to carry it, then the query.
//
// Under the default configuration c.Path() is that spelling already: escapes
// of unreserved characters decoded, every other escape kept, dot segments
// resolved and empty segments kept. Only a stray "%" that begins no escape,
// which the router matches as sent, is written as "%25", the valid spelling
// of the same name. Under UnescapePath c.Path() is fully decoded, so it is
// escaped again segment by segment, and a "/" that came from "%2F" stays the
// separator the router took it for.
//
// The raw request line is deliberately not the source. fasthttp's own
// normalization of it decodes "%2F" into a separator, merges "//" and then
// resolves "..", so "/public/..%2Fadmin" and "//admin" reached a proxy
// upstream or a net/http handler as "/admin" although no middleware mounted
// on "/admin" had run for them. The routed path keeps them the distinct names
// the router matched.
//
// The query is the one fasthttp writes: the arguments serialized again when a
// handler read or changed them through QueryArgs(), else the string as sent.
//
// The target always begins with a slash. A path override that dropped it, a
// rewrite of "/go/*" to "$1" say, would otherwise hand on
// "http://u:pw@evil.example/x", which a request line reads as an absolute URL
// and so as a host and a credential; rooted, it is the path
// "/http://u:pw@evil.example/x".
func Routed(c fiber.Ctx) string {
	path := c.Path()
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	query := queryOf(c)
	if appconfig.Of(c.App()).UnescapePath {
		target := make([]byte, 0, len(path)+len(query)+8)
		target = utils.AppendPathSegmentsEscape(target, path)
		return utils.UnsafeString(append(target, query...))
	}
	if len(query) == 0 && strings.IndexByte(path, '%') < 0 {
		return path
	}
	target := make([]byte, 0, len(path)+len(query)+8)
	target = appendStrayPercentEscaped(target, path)
	return utils.UnsafeString(append(target, query...))
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
func SegmentsAsRouted(target string) bool {
	path := pathOf(target)
	return !strings.Contains(path, "//") && !strings.Contains(path, "%2F") && !strings.Contains(path, "%2f") && !HasForgedEscape(path)
}

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
// leave target's path as it is. It decodes every escape and merges every
// empty segment, so a target holding either comes out of a normalizing client
// as a path the router never matched: "/public/..%2Fadmin" as "/admin".
func SurvivesNormalization(target string) bool {
	path := pathOf(target)
	return strings.IndexByte(path, '%') < 0 && !strings.Contains(path, "//")
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

// queryOf returns the query of c's request target, "?" included, or nil.
// Unlike Request.RequestURI, URI.RequestURI leaves the request itself alone.
func queryOf(c fiber.Ctx) []byte {
	uri := c.Request().URI().RequestURI()
	i := bytes.IndexByte(uri, '?')
	if i < 0 {
		return nil
	}
	return uri[i:]
}

// appendStrayPercentEscaped appends path to dst with every "%" that begins no
// escape written as "%25".
func appendStrayPercentEscaped(dst []byte, path string) []byte {
	for {
		i := strings.IndexByte(path, '%')
		if i < 0 {
			return append(dst, path...)
		}
		if i+2 < len(path) && isHexDigit(path[i+1]) && isHexDigit(path[i+2]) {
			dst = append(dst, path[:i+3]...)
			path = path[i+3:]
			continue
		}
		dst = append(dst, path[:i]...)
		dst = append(dst, "%25"...)
		path = path[i+1:]
	}
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
