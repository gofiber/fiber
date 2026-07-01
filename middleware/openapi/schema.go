package openapi

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

// SchemaOf generates an OpenAPI JSON Schema from a Go value using reflection.
// It inspects struct fields, their types, and json tags to produce a schema
// suitable for use with route helpers like ResponseWithExample, RequestBodyWithExample,
// and ParameterWithExample, or for inclusion in Config.Components.
//
// Supported types:
//   - Primitives: string, bool, int*, uint*, float*
//   - time.Time → {"type": "string", "format": "date-time"}
//   - []byte → {"type": "string", "format": "byte"} (Go marshals it as base64)
//   - Slices/arrays → {"type": "array", "items": {...}}
//   - Maps with string keys → {"type": "object", "additionalProperties": {...}}
//   - Structs → {"type": "object", "properties": {...}, "required": [...]}
//   - Pointers → schema of the pointed-to type (nullable fields are not required)
//   - interface{}/any → {} (accepts any value)
//
// Embedded structs and embedded pointers to structs are flattened into the
// parent object (matching encoding/json). Self-referential or mutually
// recursive structs are handled by emitting a bare {"type": "object"} at the
// point the cycle repeats, so reflection never recurses forever. Fields whose
// type has no JSON representation (chan, func, complex, ...) are skipped.
//
// Struct field tags:
//   - `json:"name"` sets the property name; `json:"-"` skips the field
//   - `json:",omitempty"` makes the field optional (not added to required)
//   - `openapi:"description:text"` sets the property description
//   - `openapi:"example:value"` sets the property example
//   - `openapi:"format:fmt"` overrides the format (e.g., "email", "uuid")
//   - `openapi:"enum:a|b|c"` sets the enum values
//
// openapi directives are comma-separated and a value may contain commas and
// colons; the only limitation is that a value cannot contain a comma immediately
// followed by another directive key (e.g. ",description:").
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
	return typeSchema(t, nil)
}

var timeType = reflect.TypeFor[time.Time]()

// typeSchema builds the schema for a single type. visited tracks the struct
// types currently on the recursion stack so that cyclic types terminate.
func typeSchema(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == timeType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{schemaKeyType: schemaTypeString}
	case reflect.Bool:
		return map[string]any{schemaKeyType: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{schemaKeyType: "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{schemaKeyType: "number"}
	case reflect.Slice, reflect.Array:
		// Go marshals []byte (a slice of uint8) as a base64-encoded string.
		// Fixed-size byte arrays are still marshaled as arrays of numbers.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "byte"}
		}
		items := typeSchema(t.Elem(), visited)
		if items == nil {
			items = map[string]any{}
		}
		return map[string]any{schemaKeyType: "array", "items": items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		additional := typeSchema(t.Elem(), visited)
		if additional == nil {
			additional = map[string]any{}
		}
		return map[string]any{schemaKeyType: schemaTypeObject, "additionalProperties": additional}
	case reflect.Struct:
		return structSchema(t, visited)
	case reflect.Interface:
		// An interface value (e.g. any) accepts any JSON value.
		return map[string]any{}
	default:
		// Unsupported kinds (chan, func, complex, uintptr, unsafe.Pointer) have
		// no JSON representation.
		return nil
	}
}

func structSchema(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	// Break reference cycles: if this struct type is already being expanded
	// further up the stack, emit a bare object instead of recursing forever.
	if visited[t] {
		return map[string]any{schemaKeyType: schemaTypeObject}
	}
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	visited[t] = true
	defer delete(visited, t)

	properties := make(map[string]any)
	var required []string
	requiredSet := make(map[string]struct{})

	addRequired := func(name string) {
		if _, ok := requiredSet[name]; ok {
			return
		}
		requiredSet[name] = struct{}{}
		required = append(required, name)
	}

	// Embedded structs are flattened the way encoding/json promotes their
	// fields: a field declared on the parent shadows a promoted field of the
	// same name regardless of declaration order, so embedded fields are merged
	// in a second pass and never overwrite parent properties.
	type embeddedField struct {
		field reflect.StructField
		omit  bool
	}
	var embeds []embeddedField

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		name, omit, skip := parseJSONTag(&field)
		if skip {
			continue
		}

		embeddedType := field.Type
		for embeddedType.Kind() == reflect.Pointer {
			embeddedType = embeddedType.Elem()
		}
		if field.Anonymous && embeddedType.Kind() == reflect.Struct && embeddedType != timeType && name == "" {
			embeds = append(embeds, embeddedField{field: field, omit: omit})
			continue
		}

		if name == "" {
			name = field.Name
		}

		fieldSchema := typeSchema(field.Type, visited)
		if fieldSchema == nil {
			// The field type has no JSON representation; skip it entirely
			// rather than emitting a meaningless empty schema.
			continue
		}

		applyOpenAPITag(&field, fieldSchema)

		properties[name] = fieldSchema

		isPointer := field.Type.Kind() == reflect.Pointer
		if !omit && !isPointer {
			addRequired(name)
		}
	}

	for _, embed := range embeds {
		embeddedType := embed.field.Type
		for embeddedType.Kind() == reflect.Pointer {
			embeddedType = embeddedType.Elem()
		}
		embedded := structSchema(embeddedType, visited)

		promotedRequired := make(map[string]struct{})
		// An embedded pointer can be nil, so its fields are not guaranteed
		// to be present and must not be marked required on the parent.
		isPtrEmbed := embed.field.Type.Kind() == reflect.Pointer
		if reqs, ok := embedded["required"].([]string); ok && !embed.omit && !isPtrEmbed {
			for _, name := range reqs {
				promotedRequired[name] = struct{}{}
			}
		}

		if props, ok := embedded["properties"].(map[string]any); ok {
			for name, prop := range props {
				if _, exists := properties[name]; exists {
					continue
				}
				properties[name] = prop
				if _, ok := promotedRequired[name]; ok {
					addRequired(name)
				}
			}
		}
	}

	schema := map[string]any{
		schemaKeyType: schemaTypeObject,
		"properties":  properties,
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

// openapiDirectiveRe locates the start of each recognized openapi tag directive.
// A directive begins at the start of the tag or after a comma. Everything from a
// directive's colon up to the next directive (or the end of the tag) is its
// value, so values may freely contain commas and colons.
var openapiDirectiveRe = regexp.MustCompile(`(?:^|,)\s*(description|example|format|enum):`)

func applyOpenAPITag(field *reflect.StructField, schema map[string]any) {
	tag := field.Tag.Get("openapi")
	if tag == "" {
		return
	}

	locs := openapiDirectiveRe.FindAllStringSubmatchIndex(tag, -1)
	for i, loc := range locs {
		key := tag[loc[2]:loc[3]]
		valStart := loc[1]
		valEnd := len(tag)
		if i+1 < len(locs) {
			valEnd = locs[i+1][0]
		}
		val := utils.TrimSpace(tag[valStart:valEnd])

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
			for j, v := range values {
				enumSlice[j] = utils.TrimSpace(v)
			}
			schema["enum"] = enumSlice
		default:
			// Unreachable: the regexp only matches the keys handled above.
		}
	}
}

func inferExampleValue(val string, schema map[string]any) any {
	schemaType, ok := schema[schemaKeyType].(string)
	if !ok {
		return val
	}
	switch schemaType {
	case "integer":
		if n, err := utils.ParseInt(val); err == nil {
			return n
		}
	case "number":
		if f, err := utils.ParseFloat64(val); err == nil {
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
