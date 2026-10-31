package openapi

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/schemautil"
	"github.com/gofiber/utils/v2"
)

type parameterKey struct {
	in   fiber.ParamLocation
	name string
}

// filterParams returns extras without the parameters drop reports true for.
func filterParams(extras []fiber.RouteParameter, drop func(in fiber.ParamLocation) bool) []fiber.RouteParameter {
	for i := range extras {
		if !drop(extras[i].In.Normalize()) {
			continue
		}
		filtered := append([]fiber.RouteParameter(nil), extras[:i]...)
		for j := i + 1; j < len(extras); j++ {
			if !drop(extras[j].In.Normalize()) {
				filtered = append(filtered, extras[j])
			}
		}
		return filtered
	}
	return extras
}

func dropQuerystringParameters(extras []fiber.RouteParameter) []fiber.RouteParameter {
	return filterParams(extras, func(in fiber.ParamLocation) bool { return in == fiber.ParamInQuerystring })
}

func isReservedHeader(name string) bool {
	name = utils.TrimSpace(name)
	return utils.EqualFold(name, fiber.HeaderAccept) ||
		utils.EqualFold(name, fiber.HeaderContentType) ||
		utils.EqualFold(name, fiber.HeaderAuthorization)
}

// dropQueryAlongsideQuerystring removes every "query" parameter when a "querystring" one is present (3.2 forbids both).
func dropQueryAlongsideQuerystring(extras []fiber.RouteParameter) []fiber.RouteParameter {
	hasQuerystring := false
	for i := range extras {
		if extras[i].In.Normalize() == fiber.ParamInQuerystring {
			hasQuerystring = true
			break
		}
	}
	if !hasQuerystring {
		return extras
	}
	return filterParams(extras, func(in fiber.ParamLocation) bool { return in == fiber.ParamInQuery })
}

func mergeRouteParameters(params []parameter, index map[parameterKey]int, extras []fiber.RouteParameter, reg *schemaRegistry) []parameter {
	if len(extras) == 0 {
		return params
	}
	for i := range extras {
		extra := &extras[i]
		if utils.TrimSpace(extra.Name) == "" {
			continue
		}
		location := extra.In.Normalize()
		if location == "" {
			location = fiber.ParamInQuery
		}
		if location == fiber.ParamInHeader && isReservedHeader(extra.Name) {
			continue
		}
		var paramExample any
		var paramExamples map[string]any
		if len(extra.Examples) > 0 {
			paramExamples = exampleObjects(extra.Examples)
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
		switch content := routeMediaTypeContent(extra.Content, reg); {
		case content != nil:
			param.Content = content
			param.Example = nil
			param.Examples = nil
		case location == fiber.ParamInQuerystring:
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
		if param.In == fiber.ParamInPath {
			param.Required = true
			// AddParameter's default string schema must not replace one derived from the route constraint.
			if idx, ok := index[parameterKey{param.In, param.Name}]; ok && extra.SchemaRef == "" && param.Content == nil && isDefaultStringSchema(extra.Schema) {
				param.Schema = params[idx].Schema
			}
		}
		params = appendOrReplaceParameter(params, index, &param)
	}
	return params
}

func isDefaultStringSchema(schema any) bool {
	if schema == nil {
		return true
	}
	values, ok := schemautil.AsMap(schema)
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

func appendOrReplaceParameter(params []parameter, index map[parameterKey]int, p *parameter) []parameter {
	if p == nil || p.Name == "" || p.In == "" {
		return params
	}
	key := parameterKey{p.In, p.Name}
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
	// A reference, composition or const describes its type elsewhere; an enum takes its values' type.
	if defaultType != "" {
		if typ, ok := schemautil.DefaultType(copied, defaultType); ok {
			copied[schemaKeyType] = typ
		}
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// adoptCanonicalParamNames renames a variant's path parameters to the canonical template's.
func adoptCanonicalParamNames(variant *pathVariant, canonical []string) {
	if len(canonical) != len(variant.ParamNames) {
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
		if copyExtra.In.Normalize() == fiber.ParamInPath {
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
