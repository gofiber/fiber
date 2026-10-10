package openapi

import (
	"strings"
)

// Route patterns are read in two places. This file classifies a pattern one
// segment at a time and keeps only its routing tokens, which is all that
// deciding what a mount or a middleware prefix covers needs. paths.go reads
// the same grammar character by character, because turning a pattern into
// OpenAPI path templates also needs each parameter's name and constraint. The
// router does not expose its parsed segments, so both stay here, and
// Test_RouteLexers_Agree keeps them from drifting apart.

const (
	// segmentLiteral matches exactly one path segment, with no parameter in it.
	segmentLiteral segmentKind = iota
	// segmentParam is a mixed or whole-segment parameter: one path segment.
	segmentParam
	// segmentOptional matches zero or one segment.
	segmentOptional
	// segmentGreedy ("*" or "+") matches any number of segments.
	segmentGreedy
)

// routeTokens returns a segment's routing-relevant characters, dropping escapes
// and <constraint> spans so neither is mistaken for a routing token.
//
// See the note at the top of this file on why patterns are lexed twice.
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

// resolveDynamicMountPrefix picks the mount prefix when the pattern has optional
// or greedy segments. Every split the segment bounds allow is tried, shortest
// first, and the one whose remainder is a target wins.
func resolveDynamicMountPrefix(pattern, requestPath, specPath, uiPath string, equal segmentEqual) (string, bool) {
	minSegments, maxSegments, dynamic := prefixSegmentBounds(pattern)
	if !dynamic {
		// A static mount is its own prefix, and its escaped form still needs
		// unescaping, which routePrefix handles.
		return "", false
	}

	if maxSegments < 0 || maxSegments > countPathSegments(requestPath) {
		// A greedy segment is unbounded; no split can consume more segments
		// than the request has.
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

// segmentKind classifies a route pattern segment by its routing tokens only,
// so a constraint or an escaped character never reads as a parameter.
type segmentKind uint8

// classifySegment returns the kind of a pattern segment and its routing tokens.
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
