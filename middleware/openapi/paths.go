package openapi

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/gofiber/utils/v2"
)

const wildcardParamName = "wildcard"

// maxPathVariants bounds the optional-parameter expansion, which is otherwise
// exponential in the number of optional parameters on one route.
const maxPathVariants = 64

type pathVariant struct {
	PathParamAliases map[string]string
	ParamConstraints map[string]string // Parameter name -> raw "<...>" constraint text
	Path             string
	ParamNames       []string
}

type resolvedParamName struct {
	openAPI string
	raw     string
}

// normalizePathHierarchy blanks template names ("/files/{dir}" → "/files/{}") so
// paths identical up to parameter names share one key.
func normalizePathHierarchy(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	inTemplate := false
	for i := 0; i < len(path); i++ {
		switch ch := path[i]; {
		case inTemplate:
			if ch == '}' {
				inTemplate = false
				_, _ = b.WriteString("{}") //nolint:errcheck // strings.Builder.WriteString never returns an error
			}
		case ch == '{':
			inTemplate = true
		default:
			_ = b.WriteByte(ch) //nolint:errcheck // strings.Builder.WriteByte never returns an error
		}
	}
	return b.String()
}

// canonicalPathItem is the path template already published for a hierarchy,
// together with the parameter names it declares.
type canonicalPathItem struct {
	path   string
	params []string
}

// pathState carries the in-progress OpenAPI path while walking a Fiber route
// pattern; optional parameters fork the walk into include/exclude branches.
type pathState struct {
	aliases     map[string]string
	constraints map[string]string // Parameter name -> raw "<...>" constraint text
	path        string
	params      []string
	paramIdx    int
}

// addParam appends the resolved parameter to the state in place.
func (s *pathState) addParam(resolved resolvedParamName, tokenName, rawConstraints string) {
	name := uniquePathParamName(resolved.openAPI, s.params)
	s.path += "{" + name + "}"
	s.params = append(s.params, name)
	s.aliases[resolved.raw] = name
	if tokenName != "" {
		s.aliases[tokenName] = name
	}
	if rawConstraints != "" {
		if s.constraints == nil {
			s.constraints = make(map[string]string, 1)
		}
		s.constraints[name] = rawConstraints
	}
	s.paramIdx++
}

func (s pathState) clone() pathState {
	return pathState{
		path:        s.path,
		params:      append([]string(nil), s.params...),
		aliases:     maps.Clone(s.aliases),
		constraints: maps.Clone(s.constraints),
		paramIdx:    s.paramIdx,
	}
}

func buildOpenAPIPathVariants(fiberPath string, params []string) []pathVariant {
	var (
		length   = len(fiberPath)
		variants []pathVariant
	)

	var walk func(i int, current pathState)
	walk = func(i int, current pathState) {
		for i < length {
			var (
				resolved                  resolvedParamName
				tokenName, rawConstraints string
				isOptional                bool
			)
			switch fiberPath[i] {
			case ':':
				tokenStart := i + 1
				i = tokenStart
				for i < length {
					c := fiberPath[i]
					// '<' opens a constraint; the rest mirrors path.go's
					// parameterEndChars, ':' and '\\' included.
					if c == '<' || c == '?' || c == '/' || c == '-' || c == '.' || c == ':' || c == '\\' {
						break
					}
					i++
				}
				tokenName = fiberPath[tokenStart:i]

				if i < length && fiberPath[i] == '<' {
					rawConstraints, i = scanConstraintSpan(fiberPath, i)
				}

				isOptional = i < length && fiberPath[i] == '?'
				if isOptional {
					i++
				}
				resolved = resolveOpenAPIPathParamName(current.paramIdx, tokenName, params)

			case '*', '+':
				// "*" also matches no segment at all, so the route serves the
				// path without it; "+" needs at least one.
				isOptional = fiberPath[i] == '*'
				resolved = resolveOpenAPIWildcardParamName(current.paramIdx, params)
				i++

			case '\\':
				// The route grammar escapes the next character, matching it
				// literally (see path.go escapeChar).
				if i+1 < length {
					// Slice the byte; string(byte) would re-encode it as a
					// code point and corrupt multi-byte UTF-8.
					current.path += fiberPath[i+1 : i+2]
				}
				i += 2
				continue

			default:
				// Append the whole literal run rather than one byte at a time.
				runStart := i
				for i < length {
					c := fiberPath[i]
					if c == ':' || c == '*' || c == '+' || c == '\\' {
						break
					}
					i++
				}
				current.path += encodeLiteralBraces(fiberPath[runStart:i])
				continue
			}

			// Each walk owns its state, so a parameter is appended in place; only
			// an optional one forks, and the exclude branch is copied off before
			// the include branch is written.
			if isOptional {
				exclude := current.clone()
				exclude.paramIdx++
				current.addParam(resolved, tokenName, rawConstraints)
				walk(i, current)
				// Each optional parameter doubles the walk, so forking stops
				// at the cap; the fully-populated variant is always emitted.
				if len(variants) < maxPathVariants {
					walk(i, exclude)
				}
				return
			}
			current.addParam(resolved, tokenName, rawConstraints)
		}

		finalPath := current.path
		if finalPath == "" {
			finalPath = "/"
		}
		variants = append(variants, pathVariant{
			Path:             finalPath,
			ParamNames:       current.params,
			PathParamAliases: current.aliases,
			ParamConstraints: current.constraints,
		})
	}

	walk(0, pathState{
		path:     "",
		params:   nil,
		aliases:  map[string]string{},
		paramIdx: 0,
	})

	seen := make(map[string]struct{}, len(variants))
	unique := make([]pathVariant, 0, len(variants))
	for _, variant := range variants {
		path := variant.Path
		if path != "" {
			// A path of only slashes is the root.
			if path = utils.TrimRight(path, '/'); path == "" {
				path = "/"
			}
		}
		// An interior empty segment only matches a literal "//" request, so
		// publishing either form would document a nonexistent endpoint.
		if strings.Contains(path, "//") {
			continue
		}
		variant.Path = path

		key := variant.Path + "|" + strings.Join(variant.ParamNames, ",")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, variant)
	}

	return unique
}

// encodeLiteralBraces percent-encodes braces coming from the route's literal
// text, which OpenAPI would otherwise read as an undeclared template expression.
func encodeLiteralBraces(literal string) string {
	if !strings.ContainsAny(literal, "{}") {
		return literal
	}
	replaced := strings.ReplaceAll(literal, "{", "%7B")
	return strings.ReplaceAll(replaced, "}", "%7D")
}

// uniquePathParamName suffixes a name already used in this variant: templates
// must not repeat names, but distinct Fiber parameters can sanitize alike.
func uniquePathParamName(name string, used []string) string {
	candidate := name
	for i := 2; slices.Contains(used, candidate); i++ {
		candidate = fmt.Sprintf("%s_%d", name, i)
	}
	return candidate
}

func resolveOpenAPIPathParamName(paramIdx int, extracted string, params []string) resolvedParamName {
	raw := extracted
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIParamName(raw, paramIdx+1),
	}
}

func resolveOpenAPIWildcardParamName(paramIdx int, params []string) resolvedParamName {
	raw := wildcardParamName
	if paramIdx < len(params) && params[paramIdx] != "" {
		raw = params[paramIdx]
	}
	return resolvedParamName{
		raw:     raw,
		openAPI: sanitizeOpenAPIWildcardParamName(raw, paramIdx+1),
	}
}

// trimWildcardMarkers drops the leading "*" or "+" of a wildcard's name,
// falling back when nothing else is left.
func trimWildcardMarkers(name, fallback string) string {
	if trimmed := strings.TrimLeft(name, "*+"); trimmed != "" {
		return trimmed
	}
	return fallback
}

func sanitizeOpenAPIWildcardParamName(name string, idx int) string {
	trimmed := trimWildcardMarkers(name, wildcardParamName)
	trimmed = strings.TrimLeft(trimmed, "_.-")
	if trimmed == "" {
		trimmed = wildcardParamName
	}
	if !strings.HasPrefix(trimmed, wildcardParamName) {
		trimmed = wildcardParamName + trimmed
	}
	return sanitizeOpenAPIParamName(trimmed, idx)
}

func sanitizeOpenAPIParamName(name string, idx int) string {
	sanitized := keyName(trimWildcardMarkers(name, name))
	if sanitized == "" {
		return fmt.Sprintf("param%d", idx)
	}
	return sanitized
}

// keyName replaces every character outside [a-zA-Z0-9._-], one underscore per
// character, which is what a component key and a path parameter name allow.
func keyName(name string) string {
	var builder strings.Builder
	builder.Grow(len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			_, _ = builder.WriteRune(r) //nolint:errcheck // strings.Builder.WriteRune never returns an error
			continue
		}
		_ = builder.WriteByte('_') //nolint:errcheck // strings.Builder.WriteByte never returns an error
	}
	return builder.String()
}
