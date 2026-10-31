package openapi

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/gofiber/utils/v2"
)

const wildcardParamName = "wildcard"

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

type canonicalPathItem struct {
	path   string
	params []string
}

type pathState struct {
	aliases     map[string]string
	constraints map[string]string // Parameter name -> raw "<...>" constraint text
	path        string
	params      []string
	paramIdx    int
}

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

func buildOpenAPIPathVariants(fiberPath string, params []string, strict bool) []pathVariant {
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
				isOptional = fiberPath[i] == '*'
				resolved = resolveOpenAPIWildcardParamName(current.paramIdx, params)
				i++

			case '\\':
				if i+1 < length {
					current.path += encodeLiteralBraces(fiberPath[i+1 : i+2])
				}
				i += 2
				continue

			default:
				runStart := i
				for i < length {
					c := fiberPath[i]
					if c == ':' || c == '*' || c == '+' || c == '\\' {
						break
					}
					i++
				}
				// A run of parameter starters ("::", ":*") begins the parameter at its last byte; the others are literal.
				if i < length && fiberPath[i] != '\\' && fiberPath[i] != '*' {
					end := i + 1
					for end < length && (fiberPath[end] == ':' || fiberPath[end] == '*' || fiberPath[end] == '+') {
						end++
					}
					i = end - 1
				}
				current.path += encodeLiteralBraces(fiberPath[runStart:i])
				continue
			}

			if isOptional {
				exclude := current.clone()
				exclude.paramIdx++
				current.addParam(resolved, tokenName, rawConstraints)
				walk(i, current)
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
	keepSlash := strict && strings.HasSuffix(fiberPath, "/")
	for _, variant := range variants {
		path := variant.Path
		// A path of only slashes is the root; under StrictRouting a trailing slash is kept, so a variant dropping an optional segment ends in "//" and is discarded.
		if path != "" && !keepSlash {
			if path = utils.TrimRight(path, '/'); path == "" {
				path = "/"
			}
		}
		// An interior empty segment only matches a literal "//" request; documenting it would invent an endpoint.
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

func encodeLiteralBraces(literal string) string {
	if !strings.ContainsAny(literal, "{}") {
		return literal
	}
	replaced := strings.ReplaceAll(literal, "{", "%7B")
	return strings.ReplaceAll(replaced, "}", "%7D")
}

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
