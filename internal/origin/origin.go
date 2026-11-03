// Package origin validates and matches the serialized origins that the CORS and
// CSRF middleware compare against their configured allow lists.
//
// Both middleware carried their own copy of this. `normalizeOrigin` differed by
// one scheme check, and the subdomain type, its matcher and the wildcard walk
// were character for character the same in `middleware/cors/utils.go` and
// `middleware/csrf/helpers.go`. The one real difference is named here as
// [SchemePolicy] rather than left to be inferred from which copy you are
// reading:
//
//   - CORS accepts any scheme, because Access-Control-Allow-Origin is echoed
//     for schemes beyond http(s) and the browser is the one enforcing.
//   - CSRF accepts only http and https, because it decides whether a request
//     may mutate state and a non-web scheme has no business doing so.
//
// This package decides only whether two serialized origins are the same string
// after normalization. It does not fold default ports, so "https://a.com" and
// "https://a.com:443" are different origins here. That is what an Origin header
// carries (RFC 6454 Section 6.1 serializes without the default port), and it is
// deliberately stricter than internal/schemehost, which folds ports because it
// compares URLs rather than origins.
package origin

import (
	"net/url"
	"strings"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// SchemePolicy says which URL schemes a caller is willing to treat as an origin.
type SchemePolicy uint8

const (
	// AnyScheme accepts whatever scheme url.Parse accepts. Used by CORS.
	AnyScheme SchemePolicy = iota
	// WebSchemesOnly accepts only http and https. Used by CSRF.
	WebSchemesOnly
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
	schemeSep   = "://"
	// wildcardSep introduces a subdomain wildcard, as in "https://*.example.com".
	wildcardSep = schemeSep + "*."
)

// Normalize reports whether raw is a well formed origin and returns it as
// lowercase "scheme://host".
//
// An origin is scheme and host and nothing else, so userinfo, a query, a
// fragment, or a path beyond a bare "/" all make it invalid rather than being
// stripped. A "*" anywhere in the host is rejected outright: a wildcard is only
// meaningful as a whole configured pattern, never inside a value, so
// "https://*" is not an origin (see the note on Access-Control-Allow-Origin in
// RFC 6454 Section 7.1).
func Normalize(raw string, policy SchemePolicy) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	if policy == WebSchemesOnly && parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
		return "", false
	}

	if strings.IndexByte(parsed.Host, '*') >= 0 {
		return "", false
	}

	if parsed.User != nil ||
		parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return "", false
	}

	return FromURL(parsed), true
}

// FromURL serializes the origin of an already parsed URL as lowercase
// "scheme://host", dropping userinfo, path, query and fragment. It is the one
// place that spells an origin string, shared by [Normalize] and by callers that
// derive an origin from a full URL, such as the Referer check in CSRF.
//
// Unlike Normalize it validates nothing: a URL with an empty host or a scheme
// outside the caller's policy still yields a string. That string is only safe
// to compare against an [AllowList], whose entries are all valid origins, so an
// invalid one can match nothing there.
func FromURL(u *url.URL) string {
	return utilsstrings.ToLower(u.Scheme) + schemeSep + utilsstrings.ToLower(u.Host)
}

// Subdomain is a configured wildcard pattern such as "https://*.example.com",
// split around the "*" into the part before it and the part after.
type Subdomain struct {
	Prefix string
	Suffix string
}

// Pattern is one parsed entry of an allow list, which is either a plain origin
// or a subdomain wildcard.
type Pattern struct {
	// Subdomain is set when Wildcard, and zero otherwise.
	Subdomain Subdomain
	// Origin is the normalized value when not Wildcard, and empty otherwise.
	Origin string
	// Wildcard reports which of the two fields above carries the entry.
	Wildcard bool
}

// ParsePattern reads one allow-list entry and reports whether it is usable.
//
// An entry that does not normalize is rejected, so callers get one place to
// fail rather than one per shape.
func ParsePattern(pattern string, policy SchemePolicy) (Pattern, bool) {
	before, after, found := strings.Cut(pattern, wildcardSep)
	if !found {
		normalized, valid := Normalize(pattern, policy)
		if !valid {
			return Pattern{}, false
		}
		return Pattern{Origin: normalized}, true
	}

	// Validate the pattern with the wildcard label removed, so that everything
	// Normalize rejects is rejected here too.
	normalized, valid := Normalize(before+schemeSep+after, policy)
	if !valid {
		return Pattern{Wildcard: true}, false
	}

	scheme, host, split := strings.Cut(normalized, schemeSep)
	if !split {
		return Pattern{Wildcard: true}, false
	}
	return Pattern{Subdomain: Subdomain{Prefix: scheme + schemeSep, Suffix: host}, Wildcard: true}, true
}

// AllowList is a parsed allow list, such as CORS AllowOrigins or CSRF
// TrustedOrigins: the exact origins, the subdomain wildcards, and the scheme
// policy both were parsed under. The zero value is an empty list.
type AllowList struct {
	exact      map[string]struct{}
	subdomains []Subdomain
	policy     SchemePolicy
}

// ParseAllowList parses every entry of a configured allow list with [ParsePattern],
// after trimming the whitespace around it.
//
// It stops at the first entry that does not parse and returns that entry as
// written, untrimmed, with ok false, so each middleware can name it in its own
// configuration error. The list is only built here, never per request.
func ParseAllowList(entries []string, policy SchemePolicy) (allowList AllowList, invalid string, ok bool) { //nolint:nonamedreturns // names tell the offending entry apart from the allow list
	allowList = AllowList{exact: make(map[string]struct{}, len(entries)), policy: policy}
	for _, entry := range entries {
		pattern, valid := ParsePattern(utils.TrimSpace(entry), policy)
		if !valid {
			return AllowList{}, entry, false
		}
		if pattern.Wildcard {
			allowList.subdomains = append(allowList.subdomains, pattern.Subdomain)
		} else {
			allowList.exact[pattern.Origin] = struct{}{}
		}
	}
	return allowList, "", true
}

// Contains reports whether o is on the list, either as an exact entry or under
// one of its wildcards.
//
// o must be in normalized form, as [Normalize] or [FromURL] produce it; it is
// compared, not normalized. A value that is not normalized fails the exact
// lookup, and [MatchAny] rejects it for the wildcards.
func (l AllowList) Contains(o string) bool {
	if _, ok := l.exact[o]; ok {
		return true
	}
	return MatchAny(l.subdomains, o, l.policy)
}

// MatchNormalized reports whether an already normalized origin sits under this
// wildcard. The caller is trusted to have normalized it; use [MatchAny] to
// match a raw header value.
func (s Subdomain) MatchNormalized(o string) bool {
	// Not a subdomain if not long enough for a dot separator.
	if len(o) < len(s.Prefix)+len(s.Suffix)+1 {
		return false
	}

	if !strings.HasPrefix(o, s.Prefix) || !strings.HasSuffix(o, s.Suffix) {
		return false
	}

	// Require the dot separator and at least one non-empty label between the
	// prefix and the suffix, so "https://.example.com" does not match.
	suffixStart := len(o) - len(s.Suffix)
	if suffixStart <= len(s.Prefix) || o[suffixStart-1] != '.' {
		return false
	}

	// The labels between the two must themselves be non-empty.
	sub := o[len(s.Prefix) : suffixStart-1]
	if sub == "" || strings.HasPrefix(sub, ".") || strings.HasSuffix(sub, ".") || strings.Contains(sub, "..") {
		return false
	}

	return true
}

// MatchAny reports whether raw sits under any of the wildcards.
//
// raw must already be in normalized form: it is normalized again and rejected
// if that changes it, so a request carrying "https://EXAMPLE.com" or a trailing
// slash cannot match a pattern that the same value in canonical form would not.
func MatchAny(subs []Subdomain, raw string, policy SchemePolicy) bool {
	if len(subs) == 0 {
		return false
	}

	normalized, ok := Normalize(raw, policy)
	if !ok || normalized != raw {
		return false
	}

	for _, sub := range subs {
		if sub.MatchNormalized(raw) {
			return true
		}
	}
	return false
}
