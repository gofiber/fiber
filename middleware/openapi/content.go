package openapi

import (
	"maps"
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
	// "example" and "examples" are mutually exclusive in OpenAPI; prefer "examples".
	if ex := exampleObjects(mt.Examples); len(ex) > 0 {
		entry["examples"] = ex
	} else if mt.Example != nil {
		entry["example"] = mt.Example
	}
	return entry
}

// exampleObjects makes every entry of an examples map an Example Object, as OpenAPI requires. An entry that
// already is one (only summary, description, value, externalValue or $ref keys, and a value, an external value
// or a reference) is kept; any other value becomes the value of a new Example Object.
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

// routeMediaTypeContent builds a content map with a schema, example and encoding per media type.
func routeMediaTypeContent(content map[string]fiber.RouteMediaType, reg *schemaRegistry) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(content))
	for mediaType, mt := range content {
		if mediaType == "" {
			continue
		}
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

// defaultResponseDescription names a response that was given no description, as the Response Object
// requires one: the status text for a code, and a generic label for "default" and ranges like "4XX".
func defaultResponseDescription(code string) string {
	if status, err := strconv.Atoi(code); err == nil {
		if text := utils.StatusMessage(status); text != "" {
			return text
		}
	}
	return "Response"
}

// convertRouteResponses converts response metadata, falling back to the route's
// Produces when a schema or example names no media type.
func convertRouteResponses(routeResponses map[string]fiber.RouteResponse, fallbackMediaType string, reg *schemaRegistry) map[string]response {
	if len(routeResponses) == 0 {
		return nil
	}
	merged := make(map[string]response, len(routeResponses))
	for code, resp := range routeResponses {
		content := routeMediaTypeContent(resp.Content, reg)
		if content == nil {
			mediaTypes := resp.MediaTypes
			if len(mediaTypes) == 0 {
				switch {
				case resp.Schema != nil || resp.SchemaRef != "" || resp.Example != nil || len(resp.Examples) > 0:
					// Without a media type the schema or example would be dropped.
					if fallbackMediaType != "" {
						mediaTypes = []string{fallbackMediaType}
					} else {
						mediaTypes = []string{fiber.MIMEApplicationJSON}
					}
				case fallbackMediaType != "" && !statusHasNoBody(code):
					// A description-only response still has a body on the wire.
					mediaTypes = []string{fallbackMediaType}
				}
			}
			content = mediaTypesToContent(mediaTypes, fiber.RouteMediaType{
				Schema: resp.Schema, SchemaRef: resp.SchemaRef, Example: resp.Example, Examples: resp.Examples,
			}, reg)
		}
		// A 1xx, 204, 205 or 304 response has no body, whatever media types were given.
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
		// A body without a media type still has one on the wire.
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
	// OpenAPI requires at least one media type in content.
	if len(merged.Content) == 0 {
		return nil
	}
	return merged
}

// defaultResponseForMethod is the response an undocumented route gets.
func defaultResponseForMethod(method, mediaType string) (string, response) {
	status := "200"
	description := "OK"

	// RFC 9110 names 204 among the statuses a successful DELETE should return.
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

// resolveHeaderSchemas reflects any Go value used as a header's schema.
func resolveHeaderSchemas(headers map[string]any, reg *schemaRegistry) map[string]any {
	if len(headers) == 0 {
		return headers
	}
	resolved := make(map[string]any, len(headers))
	for name, raw := range headers {
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
