package openapi

import (
	"maps"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// SchemaOf generates an OpenAPI JSON Schema from a Go value using reflection.
// It inspects struct fields, their types, and json tags to produce a schema
// suitable for use with route helpers like ResponseWithExample, RequestBodyWithExample,
// and ParameterWithExample, or for inclusion in Config.Components.
//
// Supported types:
//   - Primitives: string, bool, int*, uint*, float*
//   - time.Time → {"type": "string", "format": "date-time"}
//   - Slices/arrays → {"type": "array", "items": {...}}
//   - Maps → {"type": "object", "additionalProperties": {...}}
//   - Structs → {"type": "object", "properties": {...}, "required": [...]}
//   - Pointers → schema of the pointed-to type (nullable fields are not required)
//
// Struct field tags:
//   - `json:"name"` sets the property name; `json:"-"` skips the field
//   - `json:",omitempty"` makes the field optional (not added to required)
//   - `openapi:"description:text"` sets the property description
//   - `openapi:"example:value"` sets the property example
//   - `openapi:"format:fmt"` overrides the format (e.g., "email", "uuid")
//   - `openapi:"enum:a|b|c"` sets the enum values
//
// Example:
//
//	type User struct {
//	    ID    int    `json:"id"`
//	    Name  string `json:"name"`
//	    Email string `json:"email" openapi:"format:email,description:User email"`
//	}
//	schema := openapi.SchemaOf(User{})
//	// Returns: map[string]any{
//	//   "type": "object",
//	//   "properties": map[string]any{
//	//     "id":    map[string]any{"type": "integer"},
//	//     "name":  map[string]any{"type": "string"},
//	//     "email": map[string]any{"type": "string", "format": "email", "description": "User email"},
//	//   },
//	//   "required": []string{"id", "name", "email"},
//	// }
func SchemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	return typeSchema(t)
}

var timeType = reflect.TypeFor[time.Time]()

func typeSchema(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == timeType {
		return map[string]any{"type": "string", "format": "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		items := typeSchema(t.Elem())
		if items == nil {
			items = map[string]any{}
		}
		return map[string]any{"type": "array", "items": items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{"type": "object"}
		}
		additional := typeSchema(t.Elem())
		if additional == nil {
			additional = map[string]any{}
		}
		return map[string]any{"type": "object", "additionalProperties": additional}
	case reflect.Struct:
		return structSchema(t)
	default:
		return nil
	}
}

func structSchema(t reflect.Type) map[string]any {
	properties := make(map[string]any)
	var required []string

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		name, omit, skip := parseJSONTag(&field)
		if skip {
			continue
		}

		// Handle embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct && name == "" {
			embedded := structSchema(field.Type)
			if props, ok := embedded["properties"].(map[string]any); ok {
				maps.Copy(properties, props)
			}
			if reqs, ok := embedded["required"].([]string); ok && !omit {
				required = append(required, reqs...)
			}
			continue
		}

		if name == "" {
			name = field.Name
		}

		fieldSchema := typeSchema(field.Type)
		if fieldSchema == nil {
			fieldSchema = map[string]any{}
		}

		applyOpenAPITag(&field, fieldSchema)

		properties[name] = fieldSchema

		isPointer := field.Type.Kind() == reflect.Pointer
		if !omit && !isPointer {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func parseJSONTag(field *reflect.StructField) (string, bool, bool) { //nolint:gocritic // nonamedreturns forbids naming these
	tag := field.Tag.Get("json")
	if tag == "" {
		return "", false, false
	}
	if tag == "-" {
		return "", false, true
	}
	parts := strings.SplitN(tag, ",", 2)
	return parts[0], len(parts) > 1 && strings.Contains(parts[1], "omitempty"), false
}

func applyOpenAPITag(field *reflect.StructField, schema map[string]any) {
	tag := field.Tag.Get("openapi")
	if tag == "" {
		return
	}
	for part := range strings.SplitSeq(tag, ",") {
		key, val, found := strings.Cut(part, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "description":
			schema["description"] = val
		case "example":
			schema["example"] = inferExampleValue(val, schema)
		case "format":
			schema["format"] = val
		case "enum":
			values := strings.Split(val, "|")
			enumSlice := make([]any, len(values))
			for i, v := range values {
				enumSlice[i] = strings.TrimSpace(v)
			}
			schema["enum"] = enumSlice
		default:
			// Unrecognized openapi tag directives are ignored.
		}
	}
}

func inferExampleValue(val string, schema map[string]any) any {
	schemaType, ok := schema["type"].(string)
	if !ok {
		return val
	}
	switch schemaType {
	case "integer":
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n
		}
	case "number":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	case "boolean":
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	default:
		// String and other types use the raw string value.
	}
	return val
}
