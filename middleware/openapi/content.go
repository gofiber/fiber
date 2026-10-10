package openapi

import (
	"maps"

	"github.com/gofiber/fiber/v3"
)

func contentEntry(mt fiber.RouteMediaType, reg *schemaRegistry) map[string]any {
	entry := map[string]any{}
	if mt.SchemaRef != "" {
		entry["schema"] = map[string]any{schemaKeyRef: mt.SchemaRef}
	} else if resolved := reg.resolve(mt.Schema); len(resolved) > 0 {
		entry["schema"] = resolved
	}
	// OpenAPI spec: "example" and "examples" are mutually exclusive.
	// Prefer "examples" when both are provided.
	if ex := maps.Clone(mt.Examples); len(ex) > 0 {
		entry["examples"] = ex
	} else if mt.Example != nil {
		entry["example"] = mt.Example
	}
	return entry
}

// routeMediaTypeContent builds an OpenAPI content map from per-media-type
// entries, allowing a different schema/example/encoding per content type.
func routeMediaTypeContent(content map[string]fiber.RouteMediaType, reg *schemaRegistry) map[string]map[string]any {
	if len(content) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(content))
	for mediaType, mt := range content {
		if mediaType == "" {
			continue
		}
		entry := contentEntry(mt, reg)
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
					// A schema or example with no media type would be discarded,
					// so fall back to Produces, then to JSON.
					if fallbackMediaType != "" {
						mediaTypes = []string{fallbackMediaType}
					} else {
						mediaTypes = []string{fiber.MIMEApplicationJSON}
					}
				case fallbackMediaType != "" && !statusHasNoBody(code):
					// A response declared with a description alone still has a
					// body on the wire, so it documents the app-wide media type.
					mediaTypes = []string{fallbackMediaType}
				}
			}
			content = mediaTypesToContent(mediaTypes, fiber.RouteMediaType{
				Schema: resp.Schema, SchemaRef: resp.SchemaRef, Example: resp.Example, Examples: resp.Examples,
			}, reg)
		}
		merged[code] = response{
			Description: resp.Description,
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
		content[mediaType] = contentEntry(mt, reg)
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
		// A body declared without a media type is still a body on the wire,
		// so it documents the app-wide request media type.
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
	// Omit requestBody entirely when content could not be built, as the
	// OpenAPI specification requires at least one media type in content.
	if len(merged.Content) == 0 {
		return nil
	}
	return merged
}

// defaultResponseForMethod is the response an undocumented route gets. HEAD
// mirrors GET, since RFC 9110 has it answer as GET would minus the content.
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

// resolveHeaderSchemas reflects any Go value used as a header's schema, leaving
// every other header field as documented.
func resolveHeaderSchemas(headers map[string]any, reg *schemaRegistry) map[string]any {
	if len(headers) == 0 {
		return headers
	}
	resolved := make(map[string]any, len(headers))
	for name, raw := range headers {
		header, ok := raw.(map[string]any)
		if !ok {
			resolved[name] = raw
			continue
		}
		if schema, ok := header["schema"]; ok {
			if _, isMap := schema.(map[string]any); !isMap {
				header = maps.Clone(header)
				header["schema"] = reg.resolve(schema)
			}
		}
		resolved[name] = header
	}
	return resolved
}
