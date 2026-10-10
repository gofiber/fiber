package openapi

import (
	"strings"
)

// Patterns are lexed twice: here per segment for routing tokens only (mount and prefix coverage), and
// in paths.go per character for names and constraints. The router does not expose its parsed segments;
// Test_RouteLexers_Agree keeps the two from drifting.

const (
	segmentLiteral segmentKind = iota
	segmentParam
	segmentOptional
	segmentGreedy
)

// routeTokens returns a segment's routing characters, dropping escapes and <constraint> spans.
func routeTokens(seg string) string {
	var b strings.Builder
	inConstraint := false
	for i := 0; i < len(seg); i++ {
		switch ch := seg[i]; {
		case ch == '\\':
			i++ // the escaped character is a literal
		case inConstraint:
			if ch == '>' {
				inConstraint = false
			}
		case ch == '<':
			inConstraint = true
		default:
			_ = b.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// resolveDynamicMountPrefix picks the mount prefix of an optional/greedy pattern: the shortest split whose remainder is a target.
func resolveDynamicMountPrefix(pattern, requestPath, specPath, uiPath string, equal segmentEqual) (string, bool) {
	minSegments, maxSegments, dynamic := prefixSegmentBounds(pattern)
	if !dynamic {
		// A static mount is its own prefix; routePrefix handles unescaping.
		return "", false
	}

	if maxSegments < 0 || maxSegments > countPathSegments(requestPath) {
		// A greedy segment is unbounded; cap splits at the request's segment count.
		maxSegments = countPathSegments(requestPath)
	}

	for n := minSegments; n <= maxSegments; n++ {
		candidate, ok := pathPrefixSegments(requestPath, n)
		if !ok {
			break
		}
		if specPath != "" && equal(candidate+specPath, requestPath) {
			return candidate, true
		}
		if uiPath != "" && equal(candidate+uiPath, requestPath) {
			return candidate, true
		}
	}
	return "", false
}

type segmentKind uint8

func classifySegment(segment string) (segmentKind, string) { //nolint:gocritic // unnamedResult: named returns conflict with nonamedreturns linter
	tokens := routeTokens(segment)
	switch {
	case strings.ContainsAny(tokens, "*+"):
		return segmentGreedy, tokens
	case strings.HasSuffix(tokens, "?"):
		return segmentOptional, tokens
	case strings.Contains(tokens, ":"):
		return segmentParam, tokens
	default:
		return segmentLiteral, tokens
	}
}
