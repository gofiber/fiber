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
	for i := 0; i < len(seg); i++ {
		switch ch := seg[i]; ch {
		case '\\':
			i++ // the escaped character is a literal
		case '<':
			_, next := scanConstraintSpan(seg, i)
			i = next - 1
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

	// Each target fixes its own split: the request path must end with it, and what is left in front is the
	// mount prefix, whose segment count is its number of slashes. The shortest valid prefix wins, the spec
	// target on a tie.
	best, found := "", false
	for _, target := range [...]string{specPath, uiPath} {
		if target == "" || !hasSuffix(requestPath, target, equal) {
			continue
		}
		candidate := requestPath[:len(requestPath)-len(target)]
		n := strings.Count(candidate, "/")
		if n < minSegments || (maxSegments >= 0 && n > maxSegments) {
			continue
		}
		if !found || len(candidate) < len(best) {
			best, found = candidate, true
		}
	}
	return best, found
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
