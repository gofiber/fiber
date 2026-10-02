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
	"github.com/valyala/fasthttp"
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
func Routed(c fiber.Ctx) string {
	path := c.Path()
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	query := queryOf(c)
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
// URL parsers and IIS read as a separator, and for a "." or ".." segment that
// carries path parameters, such as "..;x", which servlet containers such as
// Tomcat and Jetty resolve as a dot segment once they strip the parameters:
// "/public/..;/admin" is "/admin" there.
func SegmentsAsRouted(target string) bool {
	return segmentsAsRouted(target, false)
}

// SegmentsAsRoutedSlashesAside is SegmentsAsRouted without its objection to
// an escaped slash and an empty segment, for a caller that was told the
// reader keeps those as sent. A backslash, a forged escape and a dot segment
// carrying parameters are still refused.
func SegmentsAsRoutedSlashesAside(target string) bool {
	return segmentsAsRouted(target, true)
}

// segmentsAsRouted scans target's path once for what SegmentsAsRouted
// refuses, leaving out escaped slashes and empty segments when slashes is
// set.
func segmentsAsRouted(target string, slashes bool) bool { //nolint:revive // flag-parameter: the two exported wrappers name the cases
	path := pathOf(target)
	segment := 0
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '/':
			if !slashes && i+1 < len(path) && path[i+1] == '/' {
				return false
			}
			segment = i + 1
		case '\\':
			return false
		case ';':
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
			}
		}
	}
	return true
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

// queryOf returns the query of c's request target without its "?": the one
// the client sent, unless a handler changed the arguments through QueryArgs(),
// in which case it is the changed arguments as fasthttp serializes them. A
// handler that only read them left them parsed, and their serialization then
// differs from what was sent in spelling alone, which parsing what was sent
// and serializing that tells apart. Unlike Request.RequestURI, URI.RequestURI
// leaves the request itself alone.
func queryOf(c fiber.Ctx) []byte {
	uri := c.Request().URI()
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
