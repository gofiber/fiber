// Package escapedpath resolves URL references through net/url on the path as
// written, so an escape such as "%2F" is never decoded into a separator. The
// client and the proxy follow redirects each with a loop of their own and
// resolve a Location here.
package escapedpath

import (
	"net/url"
	"strings"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// Resolve resolves location against base on the escaped path (RFC 3986 Section 5.2), not
// through fasthttp.URI, which decodes "%2F" and resolves the ".." it uncovers: a Location that
// repeats the request URI, as a canonical-host redirect does, would otherwise name another path
// than the one requested. The result carries no user info and has a host.
func Resolve(base string, location []byte) (*url.URL, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fasthttp.ErrorInvalidURI
	}
	ref, err := url.Parse(escapeStrayPercents(utils.UnsafeString(location)))
	if err != nil {
		return nil, fasthttp.ErrorInvalidURI
	}
	if !RawPathValid(ref.RawPath) {
		ref.RawPath = RepairRawPath(ref.RawPath, ref.Path)
	}
	next := baseURL.ResolveReference(ref)
	// A hop has never carried the user info of a Location: it would write an Authorization the
	// request did not have.
	next.User = nil
	if next.Host == "" {
		return nil, fasthttp.ErrorInvalidURI
	}
	return next, nil
}

// RepairRawPath escapes the bytes of raw that a path cannot hold raw, as "%XX", and leaves the
// escapes it has alone. url.Parse made sure raw decodes to path, and each byte is written as its own
// escape, so the result does too. It returns "" when that is what String writes for path anyway.
func RepairRawPath(raw, path string) string {
	const upperHex = "0123456789ABCDEF"
	b := make([]byte, 0, len(raw)+8)
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if RawPathValid(raw[i : i+1]) {
			b = append(b, c)
			continue
		}
		b = append(b, '%', upperHex[c>>4], upperHex[c&15])
	}
	repaired := string(b)
	if repaired == (&url.URL{Path: path}).EscapedPath() {
		return ""
	}
	return repaired
}

// RawPathValid reports whether raw holds only the bytes net/url's validEncoded accepts in a path.
func RawPathValid(raw string) bool {
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && strings.IndexByte("-._~!$&'()*+,;=:@[]%/", c) < 0 {
			return false
		}
	}
	return true
}

// escapeStrayPercents returns location with every "%" of its path that does not begin an escape written
// as "%25", which is how browsers, curl and fasthttp.URI take "/discount/100%"; net/url refuses it. Only the
// path is touched: the scheme and the authority are cut off as net/url does, so the host a Location names
// does not depend on this. It returns location itself, without allocating, when there is nothing to write.
func escapeStrayPercents(location string) string {
	end := len(location)
	if i := utils.IndexAny2(location, '?', '#'); i >= 0 {
		end = i
	}
	var b []byte
	last := 0
	for i := pathStart(location[:end]); i < end; i++ {
		if location[i] != '%' || i+2 < end && isHexDigit(location[i+1]) && isHexDigit(location[i+2]) {
			continue
		}
		if b == nil {
			b = make([]byte, 0, len(location)+8)
		}
		b = append(b, location[last:i+1]...)
		b = append(b, "25"...)
		last = i + 1
	}
	if b == nil {
		return location
	}
	return string(append(b, location[last:]...))
}

// pathStart returns where the path of ref, which ends where its query or fragment begins, starts, as net/url cuts
// it: after the "scheme:" and the "//authority" it opens with.
func pathStart(ref string) int {
	start := SchemeEnd(ref)
	if rest := ref[start:]; strings.HasPrefix(rest, "//") && (start > 0 || !strings.HasPrefix(rest, "///")) {
		if i := strings.IndexByte(rest[2:], '/'); i >= 0 {
			return start + 2 + i
		}
		return len(ref)
	}
	return start
}

// SchemeEnd returns the index after the "scheme:" ref opens with, or 0 when it opens with none, as net/url's getScheme does.
func SchemeEnd(ref string) int {
	for i := 0; i < len(ref); i++ {
		switch c := ref[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
			if i == 0 {
				return 0
			}
		case c == ':' && i > 0:
			return i + 1
		default:
			return 0
		}
	}
	return 0
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
