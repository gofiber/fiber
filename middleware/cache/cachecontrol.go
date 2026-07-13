package cache

import (
	"github.com/gofiber/utils/v2"
	"github.com/valyala/fasthttp"
)

// hasDirective checks if a cache directive header value contains a directive (case-insensitive).
// A directive is considered matched when followed by end-of-string, ',', ' ', '\t', or '='
// per RFC 9111 §5.2.
func hasDirective(cc, directive string) bool {
	pos := 0
	for {
		i := utils.IndexFold(cc[pos:], directive)
		if i == -1 {
			return false
		}
		i += pos
		pos = i + 1
		if i > 0 {
			prev := cc[i-1]
			if prev != ' ' && prev != ',' && prev != '\t' {
				continue
			}
		}
		end := i + len(directive)
		if end == len(cc) {
			return true
		}
		next := cc[end]
		if next == ',' || next == ' ' || next == '\t' || next == '=' {
			return true
		}
	}
}

func parseUintDirective(val []byte) (uint64, bool) {
	if len(val) == 0 {
		return 0, false
	}
	parsed, err := fasthttp.ParseUint(val)
	if err != nil || parsed < 0 {
		return 0, false
	}
	return uint64(parsed), true
}

func parseCacheControlDirectives(cc []byte, fn func(key, value []byte)) {
	for i := 0; i < len(cc); {
		// skip leading separators and OWS (space/tab per RFC 9110 §5.6.3)
		for i < len(cc) && (cc[i] == ' ' || cc[i] == '\t' || cc[i] == ',') {
			i++
		}
		if i >= len(cc) {
			break
		}

		start := i
		for i < len(cc) && cc[i] != ',' {
			i++
		}
		partEnd := i
		for partEnd > start && (cc[partEnd-1] == ' ' || cc[partEnd-1] == '\t') {
			partEnd--
		}

		keyStart := start
		for keyStart < partEnd && (cc[keyStart] == ' ' || cc[keyStart] == '\t') {
			keyStart++
		}
		if keyStart >= partEnd {
			continue
		}

		keyEnd := keyStart
		for keyEnd < partEnd && cc[keyEnd] != '=' {
			keyEnd++
		}
		// Trim trailing OWS from key
		keyEndTrimmed := keyEnd
		for keyEndTrimmed > keyStart && (cc[keyEndTrimmed-1] == ' ' || cc[keyEndTrimmed-1] == '\t') {
			keyEndTrimmed--
		}
		key := cc[keyStart:keyEndTrimmed]

		var value []byte
		if keyEnd < partEnd && cc[keyEnd] == '=' {
			valueStart := keyEnd + 1
			for valueStart < partEnd && (cc[valueStart] == ' ' || cc[valueStart] == '\t') {
				valueStart++
			}
			valueEnd := partEnd
			for valueEnd > valueStart && (cc[valueEnd-1] == ' ' || cc[valueEnd-1] == '\t') {
				valueEnd--
			}
			if valueStart <= valueEnd {
				value = cc[valueStart:valueEnd]
				// Handle quoted-string values per RFC 9111 Section 5.2
				if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
					value = unquoteCacheDirective(value)
				}
			}
		}

		fn(key, value)
		i++ // skip comma
	}
}

// unquoteCacheDirective removes quotes and handles escaped characters in quoted-string values.
// Per RFC 9111 Section 5.2, quoted-string values follow RFC 9110 Section 5.6.4.
func unquoteCacheDirective(quoted []byte) []byte {
	if len(quoted) < 2 {
		return quoted
	}

	// Remove surrounding quotes
	inner := quoted[1 : len(quoted)-1]

	// Check if there are any escaped characters (backslash followed by another character)
	hasEscapes := false
	for i := 0; i < len(inner)-1; i++ {
		if inner[i] == '\\' {
			hasEscapes = true
			break
		}
	}

	// If no escapes, return the inner content directly
	if !hasEscapes {
		return inner
	}

	// Process escaped characters
	result := make([]byte, 0, len(inner))
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			// Skip the backslash and take the next character
			i++
			result = append(result, inner[i])
		} else {
			result = append(result, inner[i])
		}
	}

	return result
}

type responseCacheControl struct {
	maxAge          uint64
	sMaxAge         uint64
	maxAgeSet       bool
	sMaxAgeSet      bool
	hasNoCache      bool
	hasNoStore      bool
	hasPrivate      bool
	hasPublic       bool
	mustRevalidate  bool
	proxyRevalidate bool
}

func parseResponseCacheControl(cc []byte) responseCacheControl {
	parsed := responseCacheControl{}
	parseCacheControlDirectives(cc, func(key, value []byte) {
		switch {
		case utils.EqualFold(utils.UnsafeString(key), noStore):
			parsed.hasNoStore = true
		case utils.EqualFold(utils.UnsafeString(key), noCache):
			parsed.hasNoCache = true
		case utils.EqualFold(utils.UnsafeString(key), privateDirective):
			parsed.hasPrivate = true
		case utils.EqualFold(utils.UnsafeString(key), "public"):
			parsed.hasPublic = true
		case utils.EqualFold(utils.UnsafeString(key), "max-age"):
			if v, ok := parseUintDirective(value); ok {
				parsed.maxAgeSet = true
				parsed.maxAge = v
			}
		case utils.EqualFold(utils.UnsafeString(key), "s-maxage"):
			if v, ok := parseUintDirective(value); ok {
				parsed.sMaxAgeSet = true
				parsed.sMaxAge = v
			}
		case utils.EqualFold(utils.UnsafeString(key), "must-revalidate"):
			parsed.mustRevalidate = true
		case utils.EqualFold(utils.UnsafeString(key), "proxy-revalidate"):
			parsed.proxyRevalidate = true
		default:
			// ignore unknown directives
		}
	})
	return parsed
}

func parseRequestCacheControl(cc []byte) requestCacheDirectives {
	directives := requestCacheDirectives{}
	parseCacheControlDirectives(cc, func(key, value []byte) {
		switch {
		case utils.EqualFold(utils.UnsafeString(key), noStore):
			directives.noStore = true
		case utils.EqualFold(utils.UnsafeString(key), noCache):
			directives.noCache = true
		case utils.EqualFold(utils.UnsafeString(key), "only-if-cached"):
			directives.onlyIfCached = true
		case utils.EqualFold(utils.UnsafeString(key), "max-age"):
			if sec, ok := parseUintDirective(value); ok {
				directives.maxAgeSet = true
				directives.maxAge = sec
			}
		case utils.EqualFold(utils.UnsafeString(key), "max-stale"):
			directives.maxStaleSet = true
			directives.maxStaleAny = len(value) == 0
			if !directives.maxStaleAny {
				if sec, ok := parseUintDirective(value); ok {
					directives.maxStale = sec
				}
			}
		case utils.EqualFold(utils.UnsafeString(key), "min-fresh"):
			if sec, ok := parseUintDirective(value); ok {
				directives.minFreshSet = true
				directives.minFresh = sec
			}
		default:
			// ignore unknown directives
		}
	})
	return directives
}

func allowsSharedCacheDirectives(cc responseCacheControl) bool {
	if cc.hasPrivate {
		return false
	}
	if cc.hasPublic || cc.sMaxAgeSet || cc.mustRevalidate || cc.proxyRevalidate {
		return true
	}

	// RFC 9111 §4.2.2 permits Expires as an absolute expiry for cacheable responses, but for
	// authenticated requests §3.6 requires an explicit shared-cache directive. Therefore,
	// an Expires header alone MUST NOT allow sharing when Authorization is present.
	return false
}
