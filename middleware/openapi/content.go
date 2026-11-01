package openapi

import (
	"maps"
	"slices"
	"strconv"

	"github.com/gofiber/fiber/v3/internal/schemautil"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"
)

func contentEntry(mt fiber.RouteMediaType, reg *schemaRegistry) map[string]any {
	entry := map[string]any{}
	if mt.SchemaRef != "" {
		entry["schema"] = map[string]any{schemaKeyRef: mt.SchemaRef}
	} else if resolved := reg.resolve(mt.Schema); len(resolved) > 0 {
		entry["schema"] = resolved
	}
	if ex := exampleObjects(mt.Examples); len(ex) > 0 {
		entry["examples"] = ex
	} else if mt.Example != nil {
		entry["example"] = mt.Example
	}
	return entry
}

// exampleObjects makes every entry of an examples map an Example Object, wrapping bare values.
func exampleObjects(examples map[string]any) map[string]any {
	if len(examples) == 0 {
		return nil
	}
	out := make(map[string]any, len(examples))
	for name, example := range examples {
		if object, ok := schemautil.AsMap(example); ok && isExampleObject(object) {
			out[name] = maps.Clone(object)
			continue
		}
		out[name] = map[string]any{"value": example}
	}
	return out
}

func isExampleObject(object map[string]any) bool {
	hasContent := false
	for key := range object {
		switch key {
		case "value", "externalValue", schemaKeyRef:
			hasContent = true
		case "summary", "description":
		default:
			return false
		}
	}
	return hasContent
}

// sortedKeys returns the keys of m in order, so component names do not change between generations.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func routeMediaTypeContent(content map[string]fiber.RouteMediaType, reg *schemaRegistry) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(content))
	for _, mediaType := range sortedKeys(content) {
		if mediaType == "" {
			continue
		}
		mt := content[mediaType]
		entry := contentEntryFor(mediaType, mt, reg)
		if len(mt.Encoding) > 0 {
			entry["encoding"] = mt.Encoding
		}
		out[mediaType] = entry
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// defaultResponseDescription names a response given no description: the status text, or a generic label.
func defaultResponseDescription(code string) string {
	if status, err := strconv.Atoi(code); err == nil {
		if text := utils.StatusMessage(status); text != "" {
			return text
		}
	}
	return "Response"
}

// convertRouteResponses converts response metadata, falling back to the route's Produces for a media type.
func convertRouteResponses(routeResponses map[string]fiber.RouteResponse, fallbackMediaType string, reg *schemaRegistry) map[string]response {
	if len(routeResponses) == 0 {
		return nil
	}
	merged := make(map[string]response, len(routeResponses))
	for _, code := range sortedKeys(routeResponses) {
		resp := routeResponses[code]
		content := routeMediaTypeContent(resp.Content, reg)
		if content == nil {
			mediaTypes := resp.MediaTypes
			if len(mediaTypes) == 0 {
				switch {
				case resp.Schema != nil || resp.SchemaRef != "" || resp.Example != nil || len(resp.Examples) > 0:
					if fallbackMediaType != "" {
						mediaTypes = []string{fallbackMediaType}
					} else {
						mediaTypes = []string{fiber.MIMEApplicationJSON}
					}
				case fallbackMediaType != "" && !statusHasNoBody(code):
					mediaTypes = []string{fallbackMediaType}
				}
			}
			content = mediaTypesToContent(mediaTypes, fiber.RouteMediaType{
				Schema: resp.Schema, SchemaRef: resp.SchemaRef, Example: resp.Example, Examples: resp.Examples,
			}, reg)
		}
		if statusHasNoBody(code) {
			content = nil
		}
		description := resp.Description
		if description == "" {
			description = defaultResponseDescription(code)
		}
		merged[code] = response{
			Description: description,
			Content:     content,
			Headers:     resolveHeaderSchemas(resp.Headers, reg),
			Links:       resp.Links,
		}
	}
	return merged
}

func mediaTypesToContent(mediaTypes []string, mt fiber.RouteMediaType, reg *schemaRegistry) map[string]map[string]any {
	if len(mediaTypes) == 0 {
		return nil
	}
	content := make(map[string]map[string]any, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		if mediaType == "" {
			continue
		}
		content[mediaType] = contentEntryFor(mediaType, mt, reg)
	}
	if len(content) == 0 {
		return nil
	}
	return content
}

func buildRequestBody(routeBody *fiber.RouteRequestBody, defaultMediaType string, reg *schemaRegistry) *requestBody {
	if routeBody == nil {
		return nil
	}
	content := routeMediaTypeContent(routeBody.Content, reg)
	if content == nil {
		mediaTypes := routeBody.MediaTypes
		if len(mediaTypes) == 0 && defaultMediaType != "" {
			mediaTypes = []string{defaultMediaType}
		}
		content = mediaTypesToContent(mediaTypes, fiber.RouteMediaType{
			Schema: routeBody.Schema, SchemaRef: routeBody.SchemaRef, Example: routeBody.Example, Examples: routeBody.Examples,
		}, reg)
	}
	merged := &requestBody{
		Description: routeBody.Description,
		Required:    routeBody.Required,
		Content:     content,
	}
	if len(merged.Content) == 0 {
		return nil
	}
	return merged
}

func defaultResponseForMethod(method, mediaType string) (string, response) {
	status := "200"
	description := "OK"

	if method == fiber.MethodDelete {
		status = "204"
		description = "No Content"
	}

	resp := response{Description: description}
	if mediaType != "" && status != "204" {
		resp.Content = map[string]map[string]any{
			mediaType: {},
		}
	}
	return status, resp
}

func resolveHeaderSchemas(headers map[string]any, reg *schemaRegistry) map[string]any {
	if len(headers) == 0 {
		return headers
	}
	resolved := make(map[string]any, len(headers))
	for _, name := range sortedKeys(headers) {
		raw := headers[name]
		header, ok := schemautil.AsMap(raw)
		if !ok {
			resolved[name] = raw
			continue
		}
		if schema, ok := header["schema"]; ok {
			if _, isMap := schemautil.AsMap(schema); !isMap {
				header = maps.Clone(header)
				header["schema"] = reg.resolve(schema)
			}
		}
		resolved[name] = header
	}
	return resolved
}
