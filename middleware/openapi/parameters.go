package openapi

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// dropQuerystringParameters filters out parameters using the OpenAPI 3.2-only
// "querystring" location, returning the input slice unchanged when none match.
func dropQuerystringParameters(extras []fiber.RouteParameter) []fiber.RouteParameter {
	isQuerystring := func(in string) bool {
		return utils.EqualFold(utils.TrimSpace(in), paramLocationQuerystring)
	}
	for i := range extras {
		if !isQuerystring(extras[i].In) {
			continue
		}
		filtered := append([]fiber.RouteParameter(nil), extras[:i]...)
		for j := i + 1; j < len(extras); j++ {
			if isQuerystring(extras[j].In) {
				continue
			}
			filtered = append(filtered, extras[j])
		}
		return filtered
	}
	return extras
}

func mergeRouteParameters(params []parameter, index map[string]int, extras []fiber.RouteParameter, reg *schemaRegistry) []parameter {
	if len(extras) == 0 {
		return params
	}
	for i := range extras {
		extra := &extras[i]
		if utils.TrimSpace(extra.Name) == "" {
			continue
		}
		location := utilsstrings.ToLower(utils.TrimSpace(extra.In))
		if location == "" {
			location = "query"
		}
		// OpenAPI spec: "example" and "examples" are mutually exclusive.
		// Prefer "examples" when both are provided.
		var paramExample any
		var paramExamples map[string]any
		if len(extra.Examples) > 0 {
			paramExamples = extra.Examples
		} else {
			paramExample = extra.Example
		}
		param := parameter{
			Name:            extra.Name,
			In:              location,
			Description:     extra.Description,
			Required:        extra.Required,
			Example:         paramExample,
			Examples:        paramExamples,
			Deprecated:      extra.Deprecated,
			Style:           extra.Style,
			AllowEmptyValue: extra.AllowEmptyValue,
			AllowReserved:   extra.AllowReserved,
		}
		// A Parameter Object describes its value either with "schema" or with
		// "content", never both and never neither.
		switch content := routeMediaTypeContent(extra.Content, reg); {
		case content != nil:
			param.Content = content
			// "example"/"examples" belong to the media type object when content
			// is used, so they are not repeated at the parameter level.
			param.Example = nil
			param.Examples = nil
		case location == paramLocationQuerystring:
			// The 3.2 "querystring" location must use content, so any supplied
			// schema is wrapped rather than emitting neither key.
			param.Content = map[string]map[string]any{
				querystringMediaType: contentEntry(fiber.RouteMediaType{
					Schema:   schemaFrom(extra.Schema, extra.SchemaRef, schemaTypeString, reg),
					Example:  paramExample,
					Examples: paramExamples,
				}, reg),
			}
			param.Example = nil
			param.Examples = nil
		default:
			param.Schema = schemaFrom(extra.Schema, extra.SchemaRef, schemaTypeString, reg)
		}
		if extra.Explode != nil {
			explode := *extra.Explode
			param.Explode = &explode
		}
		if param.In == paramLocationPath {
			param.Required = true
			// AddParameter injects {"type": "string"} when no schema is given, so
			// a description-only call would otherwise drop the schema the route
			// constraint derived (":id<int>" documented as a string).
			if idx, ok := index[param.In+":"+param.Name]; ok && extra.SchemaRef == "" && param.Content == nil && isDefaultStringSchema(extra.Schema) {
				param.Schema = params[idx].Schema
			}
		}
		params = appendOrReplaceParameter(params, index, &param)
	}
	return params
}

// isDefaultStringSchema reports whether a schema says nothing beyond the string
// default the route helpers inject.
func isDefaultStringSchema(schema any) bool {
	if schema == nil {
		return true
	}
	values, ok := schema.(map[string]any)
	if !ok {
		return false
	}
	return isDefaultStringSchemaMap(values)
}

func isDefaultStringSchemaMap(schema map[string]any) bool {
	switch len(schema) {
	case 0:
		return true
	case 1:
		typ, ok := schema[schemaKeyType].(string)
		return ok && typ == schemaTypeString
	default:
		return false
	}
}

func appendOrReplaceParameter(params []parameter, index map[string]int, p *parameter) []parameter {
	if p == nil || p.Name == "" || p.In == "" {
		return params
	}
	key := p.In + ":" + p.Name
	if idx, ok := index[key]; ok {
		params[idx] = *p
		return params
	}
	index[key] = len(params)
	return append(params, *p)
}

func schemaFrom(schema any, schemaRef, defaultType string, reg *schemaRegistry) map[string]any {
	if schemaRef != "" {
		return map[string]any{schemaKeyRef: schemaRef}
	}

	copied := reg.resolve(schema)
	if copied == nil {
		copied = map[string]any{}
	}
	// A reference describes its type elsewhere; only a bare schema takes the
	// default.
	if _, isRef := copied[schemaKeyRef]; !isRef {
		if _, ok := copied[schemaKeyType]; !ok && defaultType != "" {
			copied[schemaKeyType] = defaultType
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// adoptCanonicalParamNames renames a variant's path parameters to the canonical
// template's. Sharing a hierarchy, they correspond position by position.
func adoptCanonicalParamNames(variant *pathVariant, canonical []string) {
	if len(canonical) != len(variant.ParamNames) {
		// Defensive: a hierarchy match implies equal counts. Renaming on a
		// mismatch would be worse than leaving the names alone.
		return
	}

	renamed := make(map[string]string, len(canonical))
	for i, old := range variant.ParamNames {
		renamed[old] = canonical[i] //nolint:gosec // G602: the guard above makes the two slices the same length
	}

	if len(variant.ParamConstraints) > 0 {
		constraints := make(map[string]string, len(variant.ParamConstraints))
		for name, raw := range variant.ParamConstraints {
			if newName, ok := renamed[name]; ok {
				name = newName
			}
			constraints[name] = raw
		}
		variant.ParamConstraints = constraints
	}

	// Aliases map pattern names onto emitted ones, so they must follow the move
	// for AddParameter(in: "path") to keep matching.
	if len(variant.PathParamAliases) > 0 {
		aliases := make(map[string]string, len(variant.PathParamAliases))
		for raw, emitted := range variant.PathParamAliases {
			if newName, ok := renamed[emitted]; ok {
				emitted = newName
			}
			aliases[raw] = emitted
		}
		variant.PathParamAliases = aliases
	}

	variant.ParamNames = append([]string(nil), canonical...)
}

func remapRouteParameters(extras []fiber.RouteParameter, aliases map[string]string, pathParams []string) []fiber.RouteParameter {
	if len(extras) == 0 {
		return nil
	}
	pathSet := make(map[string]struct{}, len(pathParams))
	for _, name := range pathParams {
		pathSet[name] = struct{}{}
	}
	out := make([]fiber.RouteParameter, 0, len(extras))
	for i := range extras {
		copyExtra := extras[i]
		if utils.EqualFold(utils.TrimSpace(copyExtra.In), paramLocationPath) {
			if mapped, ok := aliases[copyExtra.Name]; ok {
				copyExtra.Name = mapped
			}
			if _, ok := pathSet[copyExtra.Name]; !ok {
				continue
			}
		}
		out = append(out, copyExtra)
	}
	return out
}
